package main

import (
	"fmt"
	"log"
	"math"
	"net/url"
	"time"

	"github.com/faiface/beep/speaker"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yanatan16/golang-soundcloud/soundcloud"

	"github.com/Depado/parakeet/cmd"
	"github.com/Depado/parakeet/player"
	"github.com/Depado/parakeet/sc"
)

var img = `                       _             _   
                      | |           | |  
 _ __   __ _ _ __ __ _| | _____  ___| |_ 
| '_ \ / _` + "`" + ` | '__/ _` + "`" + ` | |/ / _ \/ _ \ __|
| |_) | (_| | | | (_| |   <  __/  __/ |_ 
| .__/ \__,_|_|  \__,_|_|\_\___|\___|\__|
| |                                      
|_|`

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
	playingindex := 0
	playing := tt[playingindex]
	var current *player.StreamerFormat

	streamerchan := make(chan *player.StreamerFormat)
	trackchan := make(chan *soundcloud.Track)
	togglechan := make(chan bool)
	nextchan := make(chan bool)
	player := player.NewPlayer(c, trackchan, togglechan, nextchan, streamerchan)

	go player.Start(playing)
	current = <-streamerchan

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	w, h := ui.TerminalDimensions()

	imagew := widgets.NewParagraph()
	imagew.Border = false
	imagew.Text = img
	imagew.TextStyle = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)
	imagew.SetRect(-1, -1, w-35, 9)

	help := widgets.NewParagraph()
	help.Text = "     [[↑]](fg:blue,mod:bold)/[[↓]](fg:blue,mod:bold) Browse Tracklist\n" +
		"    [[Return]](fg:blue,mod:bold) Play Selected Track\n" +
		"     [[Space]](fg:blue,mod:bold) Pause/Play\n" +
		"[[q]](fg:blue,mod:bold)/[[Ctrl+C]](fg:blue,mod:bold) Exit"
	help.Border = false
	help.SetRect(w-35, 2, w, 9)

	playinfo := widgets.NewParagraph()
	playinfo.Title = "Information"
	playinfo.PaddingTop = 1
	playinfo.PaddingLeft = 1
	playinfo.SetRect(w/2, 9, w, h-2)

	playergauge := widgets.NewGauge()
	playergauge.Border = false
	playergauge.Percent = 0
	playergauge.BarColor = ui.ColorBlue
	playergauge.LabelStyle = ui.NewStyle(ui.ColorWhite)

	cursor := widgets.NewParagraph()
	cursor.Border = false

	total := widgets.NewParagraph()
	total.Border = false
	total.Text = formatDuration(current.TotalDuration)

	cursor.SetRect(-1, h-2, 11, h+1)
	playergauge.SetRect(10, h-2, w-10, h+1)
	total.SetRect(w-11, h-2, w+1, h+1)

	tracklist := widgets.NewList()
	tracklist.PaddingLeft = 1
	tracklist.PaddingTop = 1
	tracklist.SetRect(0, 9, w/2, h-2)
	tracklist.Title = "Tracklist"
	tracklist.SelectedRowStyle = ui.NewStyle(ui.ColorWhite, ui.ColorBlue)
	tracklist.SelectedRow = 0
	tracklist.Rows = make([]string, 0, len(tt))
	for i, t := range tt {
		if i == 0 {
			tracklist.Rows = append(tracklist.Rows, fmt.Sprintf("[%s - %s](fg:blue,mod:bold)", playing.Title, playing.User.Username))
		} else {
			tracklist.Rows = append(tracklist.Rows, fmt.Sprintf("%s - %s", t.Title, t.User.Username))
		}
	}

	draw := func() {
		speaker.Lock()
		c := current.Format.SampleRate.D(current.Streamer.Position()).Round(time.Second)
		speaker.Unlock()
		playergauge.Label = fmt.Sprintf("%s - %s", playing.Title, playing.User.Username)
		playinfo.Text = fmt.Sprintf("   [Title:](fg:blue,mod:bold) %s\n"+
			"  [Artist:](fg:blue,mod:bold) %s\n"+
			"[Duration:](fg:blue,mod:bold) %s\n"+
			"     [URL:](fg:blue,mod:bold) %s\n\n"+
			"          [♥%d](fg:red,mod:bold) [💬%d](fg:cyan)", playing.Title,
			playing.User.Username,
			formatDuration(current.TotalDuration),
			playing.PermalinkUrl,
			playing.FavoritingsCount,
			playing.CommentCount,
		)
		playergauge.Percent = int(float64(c) / float64(current.TotalDuration.Round(time.Second)) * 100)
		total.Text = "] " + formatDuration(current.TotalDuration)
		cursor.Text = formatDuration(c) + " [["
		ui.Render(playergauge, cursor, total, tracklist, playinfo, imagew, help)
	}
	draw()

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(100 * time.Millisecond).C
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "<Enter>":
				tracklist.Rows[playingindex] = fmt.Sprintf("%s - %s", playing.Title, playing.User.Username)
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
				payload := e.Payload.(ui.Resize)
				cursor.SetRect(-1, payload.Height-2, 11, payload.Height+1)
				playergauge.SetRect(10, payload.Height-2, payload.Width-10, payload.Height+1)
				total.SetRect(payload.Width-11, payload.Height-2, payload.Width+1, payload.Height+1)
				tracklist.SetRect(0, 9, payload.Width/2, payload.Height-2)
				playinfo.SetRect(payload.Width/2, 9, payload.Width, payload.Height-2)
				imagew.SetRect(0, -1, payload.Width-35, 9)
				help.SetRect(payload.Width-35, 2, payload.Width, 9)
				ui.Clear()
				draw()
			}
		case <-ticker:
			draw()
		case <-nextchan:
			if playingindex < len(tt) {
				tracklist.Rows[playingindex] = fmt.Sprintf("%s - %s", playing.Title, playing.User.Username)
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
