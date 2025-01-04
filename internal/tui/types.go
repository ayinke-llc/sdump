package tui

import (
	"fmt"
	"time"

	"github.com/ayinke-llc/sdump"
	"github.com/dustin/go-humanize"
)

type ErrorMsg struct {
	err error
}

type DumpURLMsg struct {
	URL        string `json:"url,omitempty"`
	SSEChannel string `json:"sse_channel,omitempty"`
}

type ItemMsg struct {
	item item
}

type item struct {
	Request   sdump.RequestDefinition   `json:"request,omitempty"`
	Response  *sdump.ResponseDefinition `json:"response,omitempty"`
	ID        string                    `json:"id,omitempty"`
	CreatedAt time.Time                 `json:"created_at,omitempty"`
}

func (i item) Title() string { return fmt.Sprintf("%s    %s", i.ID, i.Request.IPAddress) }
func (i item) Description() string {
	return fmt.Sprintf("%s   %s    %s",
		defaultTextStyle.Copy().Foreground(faintBuleColor).
			Render(i.Request.Method), humanize.Bytes(uint64(i.Request.Size)), i.CreatedAt.Format("02/01/2006 15:04:05"))
}
func (i item) FilterValue() string { return i.ID }

// type items []item
//
// func (a items) Len() int            { return len(a) }
// func (a items) Less(i, j int) bool  { return a[i].CreatedAt.Before(a[j].CreatedAt) }
// func (a items) Swap(i, j int)       { a[i], a[j] = a[j], a[i] }
// func (i items) FilterValue() string { return "" }
