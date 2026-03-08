package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/faiface/beep/speaker"
	"github.com/gizak/termui/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"

	"github.com/Depado/parakeet/cmd"
	"github.com/Depado/parakeet/player"
	"github.com/Depado/parakeet/soundcloud"
	"github.com/Depado/parakeet/ui"
)

func init() {
	// Disable logrus logging to avoid UI clutter
	logrus.SetLevel(logrus.PanicLevel)
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())
}

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
		
		// Check for OAuth token
		if c.AuthToken == "" {
			l.Fatal().Msg("OAuth token required. Set PARAKEET_AUTH_TOKEN or use --auth_token flag")
		}
		
		// Soundcloud client mit OAuth
		scc := soundcloud.NewClient(c.AuthToken)
		run(c, l, scc)
	},
}

// shuffleTracks shuffles a slice of tracks in place
func shuffleTracks(tracks soundcloud.Tracks) {
	for i := len(tracks) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		tracks[i], tracks[j] = tracks[j], tracks[i]
	}
}

func run(c *cmd.Conf, l zerolog.Logger, scc *soundcloud.Client) {
	l.Debug().Str("build", cmd.Build).Str("version", cmd.Version).Msg("starting parakeet")
	if c.URL == "" {
		l.Fatal().Msg("no playlist URL provided, nothing to do")
	}

	l.Info().Str("url", c.URL).Msg("fetching playlist")
	pls, err := scc.Playlist().FromURL(c.URL)
	if err != nil {
		l.Fatal().Err(err).Msg("unable to get playlists")
	}

	pl, err := pls.Get()
	if err != nil {
		l.Fatal().Err(err).Msg("unable to retrieve tracks")
	}

	// Shuffle tracks if requested
	if c.Shuffle {
		shuffleTracks(pl.Tracks)
		l.Info().Int("tracks", len(pl.Tracks)).Msg("playlist loaded and shuffled successfully")
	} else {
		l.Info().Int("tracks", len(pl.Tracks)).Msg("playlist loaded successfully")
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

	l.Info().Str("track", playing.Title).Msg("starting player with first track")
	go func() {
		if err = player.Start(playing); err != nil {
			l.Fatal().Err(err).Msg("unable to start player")
		}
	}()
	current = <-streamerchan

	if current == nil {
		l.Fatal().Msg("failed to start initial track")
	}

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

	// Update help widget to show shuffle status
	if c.Shuffle {
		helpwidget.Text = "     [[↑]](fg:blue,mod:bold)/[[↓]](fg:blue,mod:bold) Browse Tracklist\n" +
			"    [[Return]](fg:blue,mod:bold) Play Selected Track\n" +
			"     [[Space]](fg:blue,mod:bold) Pause/Play\n" +
			"[[q]](fg:blue,mod:bold)/[[Ctrl+C]](fg:blue,mod:bold) Exit\n\n" +
			"[🔀 SHUFFLED](fg:yellow,mod:bold)"
	}

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
				// Removed the log message that was cluttering the UI
				trackchan <- playing
				current = <-streamerchan
				if current == nil {
					l.Warn().Str("track", playing.Title).Msg("failed to load selected track, trying next")
					nextchan <- true
				}
			case "<Down>":
				tracklist.ScrollDown()
			case "<Up>":
				tracklist.ScrollUp()
			case "<Space>":
				togglechan <- true
			case "q", "<C-c>":
				l.Info().Msg("user requested exit")
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
			// Removed excessive logging for auto-advance
			trackchan <- playing
			current = <-streamerchan
			if current == nil {
				// Will trigger another next automatically
			}
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
