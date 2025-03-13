package main

import (
	"fmt"
	"time"

	"github.com/Depado/soundcloud"
	"github.com/faiface/beep/speaker"
	"github.com/gizak/termui/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/depado/parakeet/cmd"
	"github.com/depado/parakeet/player"
	"github.com/depado/parakeet/ui"
)

// Main command that will be run when no other command is provided on the
// command-line
var rootCmd = &cobra.Command{
	Use: "parakeet",
	Run: func(cc *cobra.Command, _ []string) {
		// Configuration
		c, err := cmd.NewConf()
		if err != nil {
			log.Fatal().Err(err).Msg("unable to load conf")
		}
		// Logger
		l := cmd.NewLogger(c)
		// Soundcloud client
		scc, err := soundcloud.NewAutoIDClient()
		if err != nil {
			l.Fatal().Err(err).Msg("unable to initialize soundcloud client")
		}
		run(c, l, scc)
	},
}

func run(c *cmd.Conf, l zerolog.Logger, scc *soundcloud.Client) {
	l.Debug().Str("build", cmd.Build).Str("version", cmd.Version).Msg("starting parakeet")
	if c.UserID == "" && c.URL == "" {
		l.Fatal().Msg("no user id or url, nothing to do")
	}

	pls, err := scc.Playlist().FromURL(c.URL)
	if err != nil {
		l.Fatal().Err(err).Msg("unable to get playlists")
	}

	pl, err := pls.Get()
	if err != nil {
		l.Fatal().Err(err).Msg("unable to retrieve tracks")
	}

	playingindex := 0
	playing := pl.Tracks[playingindex]
	var current *player.StreamerFormat

	// Player setup and start
	streamerchan := make(chan *player.StreamerFormat)
	trackchan := make(chan soundcloud.Track)
	togglechan := make(chan bool)
	nextchan := make(chan bool)
	player := player.NewPlayer(scc, trackchan, togglechan, nextchan, streamerchan)

	go func() {
		if err = player.Start(playing); err != nil {
			l.Fatal().Err(err).Msg("unable to start player")
		}
	}()
	current = <-streamerchan

	// Initialize UI
	if err := termui.Init(); err != nil {
		l.Fatal().Err(err).Msg("failed to initialize termui")
	}
	defer termui.Close()

	// Widgets setup and placement
	w, h := termui.TerminalDimensions()
	logowidget := ui.NewLogoWidget(h, w)
	helpwidget := ui.NewHelpWidget(h, w)
	infowidget := ui.NewInfoWidget(h, w)
	tracklist := ui.NewTracklistWidget(h, w, pl.Tracks)
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
				playing = pl.Tracks[playingindex]
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
			tracklist.Rows[playingindex] = fmt.Sprintf("[%s - %s](fg:green)", playing.Title, playing.User.Username)
			playingindex++
			if playingindex >= len(pl.Tracks) {
				playingindex = 0
			}
			playing = pl.Tracks[playingindex]
			tracklist.Rows[playingindex] = fmt.Sprintf("[%s - %s](fg:blue,mod:bold)", playing.Title, playing.User.Username)
			trackchan <- playing
			current = <-streamerchan
		}
	}
}

func main() {
	// Initialize Cobra and Viper
	cmd.AddAllFlags(rootCmd)
	rootCmd.AddCommand(cmd.VersionCmd)

	// Run the command
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("unable to start")
	}
}
