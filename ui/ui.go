package ui

import (
	"fmt"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/Depado/parakeet/soundcloud"
	"github.com/Depado/parakeet/utils"
)

// NewTracklistWidget will return a new tracklist widget with the appropriate
// content, as well as the currently playing track
func NewTracklistWidget(height, width int, tt soundcloud.Tracks) *widgets.List {
	tracklist := widgets.NewList()
	tracklist.PaddingLeft = 1
	tracklist.PaddingTop = 1
	tracklist.SetRect(0, 9, width/2, height-2)
	tracklist.Title = "Tracklist"
	tracklist.SelectedRowStyle = ui.NewStyle(ui.ColorWhite, ui.ColorBlue)
	tracklist.SelectedRow = 0
	tracklist.Rows = make([]string, 0, len(tt))

	for i, t := range tt {
		if i == 0 {
			tracklist.Rows = append(tracklist.Rows, fmt.Sprintf("[%s - %s](fg:blue,mod:bold)", tt[0].Title, tt[0].User.Username))
		} else {
			tracklist.Rows = append(tracklist.Rows, fmt.Sprintf("%s - %s", t.Title, t.User.Username))
		}
	}

	return tracklist
}

// NewLogoWidget will return a new paragraph widget containing the logo of the
// app
func NewLogoWidget(height, width int) *widgets.Paragraph { // nolint: unparam
	w := widgets.NewParagraph()
	w.Border = false
	w.Text = Logo
	w.TextStyle = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)
	w.SetRect(-1, -1, width-35, 9)

	return w
}

// NewHelpWidget will return a new paragraph widget containing the help text
func NewHelpWidget(height, width int) *widgets.Paragraph { // nolint: unparam
	w := widgets.NewParagraph()
	w.Text = "     [[↑]](fg:blue,mod:bold)/[[↓]](fg:blue,mod:bold) Browse Tracklist\n" +
		"    [[Return]](fg:blue,mod:bold) Play Selected Track\n" +
		"     [[Space]](fg:blue,mod:bold) Pause/Play\n" +
		"[[q]](fg:blue,mod:bold)/[[Ctrl+C]](fg:blue,mod:bold) Exit"
	w.Border = false
	w.SetRect(width-35, 2, width, 9)

	return w
}

// InfoWidget is a simple info widget
type InfoWidget struct {
	*widgets.Paragraph
}

// Update will update the information in the widget using the given parameters
func (i *InfoWidget) Update(track soundcloud.Track, duration time.Duration) {
	i.Text = fmt.Sprintf("   [Title:](fg:blue,mod:bold) %s\n"+
		"  [Artist:](fg:blue,mod:bold) %s\n"+
		"[Duration:](fg:blue,mod:bold) %s\n"+
		"     [URL:](fg:blue,mod:bold) %s\n\n"+
		"          [♥%d](fg:red,mod:bold) [💬%d](fg:cyan)", track.Title,
		track.User.Username,
		utils.FormatDuration(duration),
		track.PermalinkURL,
		track.LikesCount,
		track.CommentCount,
	)
}

// NewInfoWidget will return a new paragraph widget containing nothing at this
// moment but that will be updated as soon as it is drawn
func NewInfoWidget(height, width int) *InfoWidget {
	w := widgets.NewParagraph()
	w.Title = "Information"
	w.PaddingTop = 1
	w.PaddingLeft = 1
	w.SetRect(width/2, 9, width, height-2)

	return &InfoWidget{w}
}
