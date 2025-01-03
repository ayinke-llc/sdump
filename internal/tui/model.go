package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ayinke-llc/sdump/config"
	"github.com/ayinke-llc/sdump/internal/util"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/r3labs/sse/v2"
	"golang.design/x/clipboard"
	"golang.org/x/term"
)

type model struct {
	title   string
	spinner spinner.Model

	cfg        *config.Config
	dumpURL    *url.URL
	pubChannel string
	err        error

	requestList list.Model
	httpClient  *http.Client
	colorscheme string

	sseClient                 *sse.Client
	receiveChan               chan item
	detailedRequestView       viewport.Model
	detailedRequestViewBuffer *bytes.Buffer

	headersTable  table.Model
	width, height int

	sshFingerPrint string

	host                    string
	isHTTPForwardingEnabled bool
	portToForwardTo         int
}

func New(cfg *config.Config,
	opts ...Option,
) (tea.Model, error) {
	width, height, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return nil, err
	}

	tuiModel := newModel(cfg, width, height)

	for _, opt := range opts {
		opt(&tuiModel)
	}

	if util.IsStringEmpty(tuiModel.sshFingerPrint) {
		return nil, errors.New("SSH fingerprint must be provided")
	}

	if tuiModel.width <= 0 || tuiModel.height <= 0 {
		return nil, errors.New("width or height must be a non zero number")
	}

	return tuiModel, nil
}

func newModel(cfg *config.Config, width, height int) model {
	columns := []table.Column{
		{
			Title: "Header",
			Width: 50,
		},
		{
			Title: "Value",
			Width: 50,
		},
	}

	m := model{
		colorscheme: cfg.TUI.ColorScheme,
		width:       width,
		height:      height,
		title:       "Sdump",
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Line),
			spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))),
		),

		cfg: cfg,
		httpClient: &http.Client{
			Timeout: time.Minute,
		},

		requestList:               list.New([]list.Item{}, list.NewDefaultDelegate(), 50, height),
		detailedRequestView:       viewport.New(width, height),
		detailedRequestViewBuffer: bytes.NewBuffer(nil),
		sseClient:                 sse.NewClient(fmt.Sprintf("%s/events", cfg.HTTP.Domain)),
		receiveChan:               make(chan item),

		headersTable: table.New(table.WithColumns(columns),
			table.WithFocused(true),
			table.WithHeight(10),
			table.WithWidth(width),
			table.WithKeyMap(table.KeyMap{}),
			table.WithStyles(getTableStyles())),
	}

	m.requestList.Title = "Incoming requests"
	m.requestList.SetShowTitle(true)
	m.requestList.SetFilteringEnabled(false)
	m.requestList.DisableQuitKeybindings()

	m.headersTable.Blur()

	return m
}

func (m model) isInitialized() bool { return m.dumpURL != nil }

func (m model) Init() tea.Cmd {
	tea.SetWindowTitle(m.title)

	return tea.Batch(m.spinner.Tick,
		m.createEndpoint(false))
}

func (m model) listenForNextItem() tea.Msg {
	var knownError error

	err := m.sseClient.Subscribe(m.pubChannel, func(msg *sse.Event) {
		var i item

		if err := json.NewDecoder(bytes.NewBuffer(msg.Data)).Decode(&i); err != nil {
			knownError = err
			return
		}

		m.receiveChan <- i
	})

	if knownError != nil {
		return ErrorMsg{err: err}
	}

	if err != nil {
		return ErrorMsg{err: err}
	}

	return nil
}

func (m model) waitForNextItem() tea.Msg {
	return ItemMsg{item: <-m.receiveChan}
}

func (m model) createEndpoint(forceURLChange bool) func() tea.Msg {
	return func() tea.Msg {
		// err can be safely ignored
		req, _ := http.NewRequest(http.MethodPost,
			m.cfg.HTTP.Domain,
			strings.NewReader(fmt.Sprintf(`{"ssh_fingerprint" : "%s","force_new_endpoint" : %v}`,
				m.sshFingerPrint, forceURLChange)))

		req.Header.Add("Content-Type", "application/json")

		resp, err := m.httpClient.Do(req)
		if err != nil {
			return ErrorMsg{err: err}
		}

		defer resp.Body.Close()

		if resp.StatusCode > http.StatusCreated {
			_, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				return ErrorMsg{err: err}
			}

			return ErrorMsg{err: errors.New("an error occurred while creating ingest url")}
		}

		var response struct {
			URL struct {
				HumanReadableEndpoint string `json:"human_readable_endpoint,omitempty"`
			} `json:"url,omitempty"`
			SSE struct {
				Channel string `json:"channel,omitempty"`
			} `json:"sse,omitempty"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return ErrorMsg{err: err}
		}

		return DumpURLMsg{
			URL:        response.URL.HumanReadableEndpoint,
			SSEChannel: response.SSE.Channel,
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case DumpURLMsg:
		var err error
		if strings.Trim(msg.URL, "") == "" {
			m.err = errors.New("an error occurred while setting up URL")
			return m, cmd
		}

		m.dumpURL, err = url.Parse(msg.URL)
		if err != nil {
			m.err = err
			return m, cmd
		}

		m.pubChannel = msg.SSEChannel
		go m.listenForNextItem()
		return m, m.waitForNextItem

	case ErrorMsg:

		m.err = msg.err
		return m, cmd

	case ItemMsg:

		m.requestList.InsertItem(0, msg.item)

		return m, m.waitForNextItem

	case tea.WindowSizeMsg:

		m.requestList.SetSize(msg.Width, msg.Height-27)

		return m, cmd

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlR:

			m.dumpURL = nil
			m.requestList.SetItems([]list.Item{})

			return m, m.createEndpoint(true)

		case tea.KeyCtrlY:

			_ = clipboard.Write(clipboard.FmtText, []byte(m.dumpURL.String()))

			return m, cmd

		case tea.KeyCtrlB:

			_ = clipboard.Write(clipboard.FmtText, m.detailedRequestViewBuffer.Bytes())

			return m, cmd
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	}

	var cmds []tea.Cmd

	m.requestList, cmd = m.requestList.Update(msg)
	cmds = append(cmds, cmd)

	m.detailedRequestView, cmd = m.detailedRequestView.Update(msg)
	cmds = append(cmds, cmd)

	m.headersTable, cmd = m.headersTable.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		return showError(m.err)
	}

	if !m.isInitialized() {
		return lipgloss.Place(
			200, 3,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center,
				boldenString("Generating your URL... press CTRL+C to quit", true),
				strings.Repeat(m.spinner.View(), 20),
			))
	}

	browserHeader := lipgloss.Place(
		200, 0,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center,
			boldenString("Inspecting incoming HTTP requests", true),
			boldenString(fmt.Sprintf(`
Waiting for requests on %s .. Press Ctrl-y to copy the url. Use ctrl-b to copy the json request body in view.
				You can use j,k or arrow up and down to navigate your requests`, m.dumpURL), true),
		))

	return m.spinner.View() + browserHeader + strings.Repeat("\n", 2) + m.makeTable()
}

func (m model) buildView() string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Margin(1, 4).
			Render(m.requestList.View()),
		lipgloss.NewStyle().Padding(0, 0).
			Render(lipgloss.JoinHorizontal(lipgloss.Center,
				m.headersTable.View(), ""),
				lipgloss.NewStyle().Margin(1, 0, 0, 0).
					Render(m.detailedRequestView.View())))
}

func (m model) makeTable() string {
	selectedItem, ok := m.requestList.SelectedItem().(item)
	if !ok {
		return m.buildView()
	}

	m.detailedRequestViewBuffer.Reset()

	// Since the url is meant to take any json content ( valid or not)
	// we do not want to enforce if a JSON is valid or not. Even on the ingestion side
	// If we have a valid JSON, pretty print it. Else use the json body as is
	jsonBody, err := prettyPrintJSON(selectedItem.Request.Body)
	if err != nil {
		jsonBody = selectedItem.Request.Body
	}

	// if we have an error here, just reuse the json body as it is without adding
	// color
	if err := highlightCode(m.detailedRequestViewBuffer, jsonBody, m.cfg.TUI.ColorScheme); err != nil {
		m.detailedRequestViewBuffer.WriteString(jsonBody)
	}

	m.detailedRequestView.SetContent(m.detailedRequestViewBuffer.String())

	m.detailedRequestViewBuffer.Reset()
	m.detailedRequestViewBuffer.WriteString(jsonBody)

	var keys []string
	for key := range selectedItem.Request.Headers {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	rows := []table.Row{}

	for _, v := range keys {
		value := selectedItem.Request.Headers.Get(v)
		if value == "" {
			continue
		}

		rows = append(rows, table.Row{
			v, value,
		})
	}

	m.headersTable.SetRows(rows)

	return m.buildView()
}
