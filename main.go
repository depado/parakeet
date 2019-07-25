package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/Depado/parakeet/cmd"
	"github.com/Depado/parakeet/sc"
)

// Build number and versions injected at compile time, set yours
var (
	Version = "unknown"
	Build   = "unknown"
)

// Main command that will be run when no other command is provided on the
// command-line
var rootCmd = &cobra.Command{
	Use: "parakeet",
	Run: func(cmd *cobra.Command, args []string) { run() }, // nolint: unparam
}

// Version command that will display the build number and version (if any)
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show build and version",
	Run:   func(cmd *cobra.Command, args []string) { fmt.Printf("Build: %s\nVersion: %s\n", Build, Version) }, // nolint: unparam
}

func formatDuration(t time.Duration) string {
	h := int64(math.Mod(t.Hours(), 24))
	m := int64(math.Mod(t.Minutes(), 60))
	s := int64(math.Mod(t.Seconds(), 60))
	return fmt.Sprintf("%02d:%02d:%02d", int(h), int(m), int(s))
}

func run() {
	c := sc.NewClient(viper.GetString("client_id"))
	u := c.User(viper.GetUint64("user_id"))
	tt, err := u.Favorites(url.Values{})
	if err != nil {
		logrus.WithError(err).Fatal("Unable to get favorite tracks")
	}
	playing := tt[0]
	o, err := c.Stream(playing)
	if err != nil {
		logrus.WithError(err).Fatal("Unable to get stream")
	}
	tot := time.Duration(int64(playing.Duration)) * time.Millisecond

	resp, err := http.Get(o.HTTPMp3128URL)
	if err != nil {
		logrus.WithError(err).Fatal("Unable to get stream")
	}
	defer resp.Body.Close()

	streamer, format, err := mp3.Decode(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	done := make(chan bool)

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second))
	ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		done <- true
	})))

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	w, h := ui.TerminalDimensions()

	player := widgets.NewGauge()
	player.Border = false
	player.Percent = 0
	player.BarColor = ui.ColorBlue
	player.LabelStyle = ui.NewStyle(ui.ColorWhite)

	current := widgets.NewParagraph()
	current.Border = false
	current.Text = "00:00:00"

	total := widgets.NewParagraph()
	total.Border = false
	total.Text = formatDuration(tot)

	current.SetRect(-1, h-2, 9, h+1)
	player.SetRect(8, h-2, w-9, h+1)
	total.SetRect(w-9, h-2, w+1, h+1)

	tracklist := widgets.NewList()
	tracklist.SetRect(-1, 0, w+1, h-1)
	tracklist.Title = "Tracklist"
	tracklist.Rows = make([]string, 0, len(tt))
	for _, t := range tt {
		tracklist.Rows = append(tracklist.Rows, fmt.Sprintf("%s - %s", t.Title, t.User.Username))
	}

	draw := func() {
		speaker.Lock()
		cursor := format.SampleRate.D(streamer.Position()).Round(time.Second)
		speaker.Unlock()
		player.Label = fmt.Sprintf("%s - %s", playing.Title, playing.User.Username)
		player.Percent = int(float64(cursor) / float64(tot.Round(time.Second)) * 100)
		current.Text = formatDuration(cursor)
		ui.Render(player, current, total, tracklist)
	}
	draw()

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(time.Second).C
	for {
		select {
		case <-done:
			return
		case e := <-uiEvents:
			switch e.ID {
			case "<Space>":
				speaker.Lock()
				ctrl.Paused = !ctrl.Paused
				speaker.Unlock()
			case "q", "<C-c>":
				return
			case "<Resize>":
				payload := e.Payload.(ui.Resize)
				current.SetRect(-1, payload.Height-2, 9, payload.Height+1)
				player.SetRect(8, payload.Height-2, payload.Width-9, payload.Height+1)
				total.SetRect(payload.Width-9, payload.Height-2, payload.Width+1, payload.Height+1)
				tracklist.SetRect(-1, 0, payload.Width+1, payload.Height-1)
				ui.Clear()
				draw()
			}
			switch e.Type {
			case ui.KeyboardEvent: // handle all key presses
				// eventID := e.ID // keypress string
				// fmt.Println(eventID)
			}
		case <-ticker:
			draw()
		}
	}
}

func main() {
	// Initialize Cobra and Viper
	cobra.OnInitialize(cmd.Initialize)
	cmd.AddFlags(rootCmd)
	rootCmd.AddCommand(versionCmd)

	// Run the command
	if err := rootCmd.Execute(); err != nil {
		logrus.WithError(err).Fatal("Couldn't start")
	}
}
