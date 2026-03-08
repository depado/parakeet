package ui

import (
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/Depado/parakeet/player"
	"github.com/Depado/parakeet/soundcloud"
	"github.com/Depado/parakeet/utils"
)

// PlayerWidget is a set of widget that represents
// the player in the app
type PlayerWidget struct {
	Gauge  *widgets.Gauge
	Cursor *widgets.Paragraph
	Total  *widgets.Paragraph
}

// NewPlayerWidget will return a new player widget
func NewPlayerWidget(height, width int) *PlayerWidget {
	gauge := widgets.NewGauge()
	gauge.Border = false
	gauge.Percent = 0
	gauge.BarColor = ui.ColorBlue
	gauge.LabelStyle = ui.NewStyle(ui.ColorWhite)
	gauge.SetRect(10, height-2, width-10, height+1)

	cursor := widgets.NewParagraph()
	cursor.Border = false
	cursor.SetRect(-1, height-2, 11, height+1)

	total := widgets.NewParagraph()
	total.Border = false
	total.SetRect(width-11, height-2, width+1, height+1)

	return &PlayerWidget{
		Gauge:  gauge,
		Cursor: cursor,
		Total:  total,
	}
}

// Update will update the widget with the given information
func (p *PlayerWidget) Update(playing soundcloud.Track, stream *player.StreamerFormat, cursor time.Duration) {
	p.Gauge.Label = " "
	p.Gauge.Percent = int(float64(cursor) / float64(stream.TotalDuration.Round(time.Second)) * 100)
	p.Total.Text = "] " + utils.FormatDuration(stream.TotalDuration)
	p.Cursor.Text = utils.FormatDuration(cursor) + " [["
}
