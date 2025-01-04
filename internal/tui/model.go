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

	"github.com/ayinke-llc/sdump"
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

	sseClient                  *sse.Client
	receiveChan                chan item
	detailedRequestView        viewport.Model
	detailedRequestViewBuffer  *bytes.Buffer
	detailedResponseView       viewport.Model
	detailedResponseViewBuffer *bytes.Buffer

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

		requestList:                list.New([]list.Item{}, list.NewDefaultDelegate(), 50, height),
		detailedRequestView:        viewport.New(width, height),
		detailedRequestViewBuffer:  bytes.NewBuffer(nil),
		detailedResponseView:       viewport.New(width, height),
		detailedResponseViewBuffer: bytes.NewBuffer(nil),
		sseClient:                  sse.NewClient(fmt.Sprintf("%s/events", cfg.HTTP.Domain)),
		receiveChan:                make(chan item),

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

func (m model) forwardHTTPRequest(item *item) error {
	if !m.isHTTPForwardingEnabled {
		return nil
	}

	// Always forward to localhost since we want to forward to a local service
	forwardURL := fmt.Sprintf("http://localhost:%d%s", m.portToForwardTo, item.Request.Path)

	// Create forwarding request
	req, err := http.NewRequest(item.Request.Method, forwardURL, strings.NewReader(item.Request.Body))
	if err != nil {
		item.Response = &sdump.ResponseDefinition{
			Body:       fmt.Sprintf("Error creating forward request: %v", err),
			StatusCode: http.StatusInternalServerError,
		}
		return nil
	}

	// Copy original headers
	for k, v := range item.Request.Headers {
		for _, headerVal := range v {
			req.Header.Add(k, headerVal)
		}
	}

	// Add query parameters if any
	if item.Request.Query != "" {
		req.URL.RawQuery = item.Request.Query
	}

	// Forward the request
	resp, err := m.httpClient.Do(req)
	if err != nil {
		item.Response = &sdump.ResponseDefinition{
			Body:       fmt.Sprintf("Error forwarding request: %v", err),
			StatusCode: http.StatusInternalServerError,
		}
		return nil
	}
	defer resp.Body.Close()

	// Read response body
	respBody := new(bytes.Buffer)
	size, err := io.Copy(respBody, resp.Body)
	if err != nil {
		item.Response = &sdump.ResponseDefinition{
			Body:       fmt.Sprintf("Error reading response body: %v", err),
			StatusCode: http.StatusInternalServerError,
		}
		return nil
	}

	// Store response
	item.Response = &sdump.ResponseDefinition{
		Body:       respBody.String(),
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
		Size:       size,
	}

	return nil
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
		// Forward HTTP request if enabled
		_ = m.forwardHTTPRequest(&msg.item)
		m.requestList.InsertItem(0, msg.item)
		return m, m.waitForNextItem

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Reserve space for header (4 lines) + padding
		headerHeight := 6
		contentHeight := m.height - headerHeight - 2 // -2 for spacing

		// Adjust list and viewport sizes
		m.requestList.SetSize(m.width/4, contentHeight)
		m.detailedRequestView.Height = contentHeight
		m.detailedResponseView.Height = contentHeight
		m.detailedRequestView.Width = (m.width * 3 / 4) / 2
		m.detailedResponseView.Width = (m.width * 3 / 4) / 2

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
			m.width, m.height,
			lipgloss.Center,
			lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center,
				boldenString("Generating your URL... press CTRL+C to quit", true),
				strings.Repeat(m.spinner.View(), 20),
			))
	}

	// Style for the header section
	headerStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		BorderBottom(true).
		PaddingBottom(1)

	// Create header content
	header := lipgloss.JoinVertical(lipgloss.Center,
		boldenString("Inspecting incoming HTTP requests", true),
		boldenString(fmt.Sprintf("Waiting for requests on %s", m.dumpURL), true),
		boldenString("Press Ctrl-y to copy the url. Use ctrl-b to copy the json request body in view.", true),
		boldenString("You can use j,k or arrow up and down to navigate your requests", true),
	)

	// Render the header with style
	styledHeader := headerStyle.Render(header)

	// Main content
	content := m.makeTable()

	// Join everything together with proper spacing
	return lipgloss.JoinVertical(
		lipgloss.Top,
		styledHeader,
		"\n",
		content,
	)
}

func (m model) buildView() string {
	// Calculate available width for each section
	totalWidth := m.width
	leftWidth := totalWidth / 4              // 25% for request list
	rightWidth := totalWidth - leftWidth - 4 // Remaining space minus padding for right section
	rightHalfWidth := (rightWidth / 2) - 2   // Split right section in two, minus padding

	// Left side with request list
	leftView := lipgloss.NewStyle().
		Width(leftWidth).
		Height(m.height - 8). // Subtract header height
		Render(m.requestList.View())

	// Right side with request and response side by side
	rightView := lipgloss.JoinHorizontal(lipgloss.Top,
		// Request section (headers + body)
		lipgloss.NewStyle().
			Width(rightHalfWidth).
			Height(m.height-8). // Subtract header height
			MarginRight(2).
			Render(m.detailedRequestView.View()),
		// Response section (headers + body)
		lipgloss.NewStyle().
			Width(rightHalfWidth).
			Height(m.height-8). // Subtract header height
			Render(m.detailedResponseView.View()))

	// Join left and right sides
	return lipgloss.JoinHorizontal(lipgloss.Top,
		leftView,
		lipgloss.NewStyle().
			MarginLeft(2).
			MarginRight(2).
			Render(rightView))
}

func (m model) makeTable() string {
	selectedItem, ok := m.requestList.SelectedItem().(item)
	if !ok {
		return m.buildView()
	}

	// Reset buffers
	m.detailedRequestViewBuffer.Reset()
	m.detailedResponseViewBuffer.Reset()

	// Show URL at the top
	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	m.detailedRequestViewBuffer.WriteString(fmt.Sprintf("%s\n\n", urlStyle.Render(fmt.Sprintf("URL: %s", selectedItem.Request.Path))))

	// Request Headers Section
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#8B00FF")).
		Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	m.detailedRequestViewBuffer.WriteString(headerStyle.Render("Request Headers") + "\n\n")

	var keys []string
	for key := range selectedItem.Request.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, v := range keys {
		value := selectedItem.Request.Headers.Get(v)
		if value == "" {
			continue
		}
		m.detailedRequestViewBuffer.WriteString(fmt.Sprintf("%s %s\n",
			keyStyle.Render(v+":"),
			value))
	}

	// Request Body
	m.detailedRequestViewBuffer.WriteString("\n" + headerStyle.Render("Request Body") + "\n\n")
	jsonBody, err := prettyPrintJSON(selectedItem.Request.Body)
	if err != nil {
		jsonBody = selectedItem.Request.Body
	}
	if err := highlightCode(m.detailedRequestViewBuffer, jsonBody, m.cfg.TUI.ColorScheme); err != nil {
		m.detailedRequestViewBuffer.WriteString(jsonBody)
	}
	m.detailedRequestView.SetContent(m.detailedRequestViewBuffer.String())

	// Handle response
	if selectedItem.Response != nil {
		// Response Headers
		m.detailedResponseViewBuffer.WriteString(headerStyle.Render("Response Headers") + "\n\n")
		m.detailedResponseViewBuffer.WriteString(fmt.Sprintf("%s %d\n", keyStyle.Render("Status:"), selectedItem.Response.StatusCode))
		m.detailedResponseViewBuffer.WriteString(fmt.Sprintf("%s %d bytes\n", keyStyle.Render("Size:"), selectedItem.Response.Size))

		var respKeys []string
		for key := range selectedItem.Response.Headers {
			respKeys = append(respKeys, key)
		}
		sort.Strings(respKeys)

		for _, v := range respKeys {
			value := selectedItem.Response.Headers.Get(v)
			if value == "" {
				continue
			}
			m.detailedResponseViewBuffer.WriteString(fmt.Sprintf("%s %s\n",
				keyStyle.Render(v+":"),
				value))
		}

		// Response Body
		m.detailedResponseViewBuffer.WriteString("\n" + headerStyle.Render("Response Body") + "\n\n")
		respBody, err := prettyPrintJSON(selectedItem.Response.Body)
		if err != nil {
			respBody = selectedItem.Response.Body
		}
		if err := highlightCode(m.detailedResponseViewBuffer, respBody, m.cfg.TUI.ColorScheme); err != nil {
			m.detailedResponseViewBuffer.WriteString(respBody)
		}
	}
	m.detailedResponseView.SetContent(m.detailedResponseViewBuffer.String())

	return m.buildView()
}
