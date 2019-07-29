package main

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/faiface/beep/speaker"
	"github.com/gizak/termui/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yanatan16/golang-soundcloud/soundcloud"

	"github.com/Depado/parakeet/cmd"
	"github.com/Depado/parakeet/player"
	"github.com/Depado/parakeet/sc"
	"github.com/Depado/parakeet/ui"
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

func run() {
	c := sc.NewClient(viper.GetString("client_id"))
	u := c.User(viper.GetUint64("user_id"))
	tt, err := u.Favorites(url.Values{})
	if err != nil {
		logrus.WithError(err).Fatal("Unable to get favorite tracks")
	}
	playingindex := 0
	playing := tt[playingindex]
	var current *player.StreamerFormat

	// Player setup and start
	streamerchan := make(chan *player.StreamerFormat)
	trackchan := make(chan *soundcloud.Track)
	togglechan := make(chan bool)
	nextchan := make(chan bool)
	player := player.NewPlayer(c, trackchan, togglechan, nextchan, streamerchan)
	go player.Start(playing)
	current = <-streamerchan

	// Initialize UI
	if err := termui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer termui.Close()

	// Widgets setup and placement
	w, h := termui.TerminalDimensions()
	logowidget := ui.NewLogoWidget(h, w)
	helpwidget := ui.NewHelpWidget(h, w)
	infowidget := ui.NewInfoWidget(h, w)
	tracklist := ui.NewTracklistWidget(h, w, tt)
	playerwidget := ui.NewPlayerWidget(h, w)

	// Draw function executed periodically or on event
	draw := func() {
		if current != nil {
			speaker.Lock()
			c := current.Format.SampleRate.D(current.Streamer.Position()).Round(time.Second)
			speaker.Unlock()
			playerwidget.Update(playing, current, c)
			infowidget.Update(playing, current.TotalDuration)
			termui.Render(playerwidget.Gauge,
				playerwidget.Cursor,
				playerwidget.Total,
				tracklist,
				infowidget,
				logowidget,
				helpwidget,
			)
		}
	}
	draw()

	// Main control loop
	uiEvents := termui.PollEvents()
	ticker := time.NewTicker(100 * time.Millisecond).C
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "<Enter>":
				tracklist.Rows[playingindex] = fmt.Sprintf("[%s - %s](fg:green)", playing.Title, playing.User.Username)
				playingindex = tracklist.SelectedRow
				playing = tt[playingindex]
				tracklist.Rows[playingindex] = fmt.Sprintf("[%s - %s](fg:blue,mod:bold)", playing.Title, playing.User.Username)
				trackchan <- playing
				current = <-streamerchan
			case "<Down>":
				tracklist.ScrollDown()
			case "<Up>":
				tracklist.ScrollUp()
			case "<Space>":
				togglechan <- true
			case "q", "<C-c>":
				return
			case "<Resize>":
				payload := e.Payload.(termui.Resize)
				playerwidget.Cursor.SetRect(-1, payload.Height-2, 11, payload.Height+1)
				playerwidget.Gauge.SetRect(10, payload.Height-2, payload.Width-10, payload.Height+1)
				playerwidget.Total.SetRect(payload.Width-11, payload.Height-2, payload.Width+1, payload.Height+1)
				tracklist.SetRect(0, 9, payload.Width/2, payload.Height-2)
				infowidget.SetRect(payload.Width/2, 9, payload.Width, payload.Height-2)
				logowidget.SetRect(0, -1, payload.Width-35, 9)
				helpwidget.SetRect(payload.Width-35, 2, payload.Width, 9)
				termui.Clear()
				draw()
			}
		case <-ticker:
			draw()
		case <-nextchan:
			if playingindex < len(tt) {
				tracklist.Rows[playingindex] = fmt.Sprintf("[%s - %s](fg:green)", playing.Title, playing.User.Username)
				playingindex++
				playing = tt[playingindex]
				tracklist.Rows[playingindex] = fmt.Sprintf("[%s - %s](fg:blue,mod:bold)", playing.Title, playing.User.Username)
				trackchan <- playing
				current = <-streamerchan
			}
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
