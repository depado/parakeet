package main

import (
	"time"

	"github.com/Depado/soundcloud"
	tea "github.com/charmbracelet/bubbletea"
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
		runBubbleTea(c, l, scc)
	},
}

func runBubbleTea(c *cmd.Conf, l zerolog.Logger, scc *soundcloud.Client) {
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
	positionchan := make(chan time.Duration)
	togglechan := make(chan bool)
	nextchan := make(chan bool)
	player := player.NewPlayer(scc, trackchan, togglechan, nextchan, streamerchan, positionchan)

	// Start player in a goroutine
	go func() {
		if err = player.Start(playing); err != nil {
			l.Fatal().Err(err).Msg("unable to start player")
		}
	}()

	// Wait for initial streamer
	current = <-streamerchan

	// Create bubbletea model
	model := ui.NewBubbleTeaModel(pl.Tracks, trackchan, togglechan, nextchan)
	model.UpdateStreamer(playing, current)

	// The player starts playing immediately, so set the correct state
	model.SetPlaying(true)

	// Create the program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Handle player updates
	go func() {
		for streamer := range streamerchan {
			p.Send(ui.StreamerUpdateMsg{
				Track:    streamer.Track,
				Streamer: streamer,
			})
		}
	}()

	// Handle position updates
	go func() {
		for position := range positionchan {
			p.Send(ui.TrackPositionMsg(position))
		}
	}()

	// Run the program
	if _, err := p.Run(); err != nil {
		l.Fatal().Err(err).Msg("could not run program")
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
