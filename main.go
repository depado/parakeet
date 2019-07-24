package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/jroimartin/gocui"
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

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("title", -1, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		fmt.Fprintln(v, "Hello world!")
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func run() {
	c := sc.NewClient(viper.GetString("client_id"))
	u := c.User(viper.GetUint64("user_id"))
	tt, err := u.Favorites(url.Values{})
	if err != nil {
		logrus.WithError(err).Fatal("Unable to get favorite tracks")
	}
	playing := tt[0]
	fmt.Printf("Reading '%s'\n", playing.Title)
	o, err := c.Stream(playing)
	if err != nil {
		logrus.WithError(err).Fatal("Unable to get stream")
	}
	// tot := time.Duration(int64(playing.Duration)) * time.Millisecond

	resp, err := http.Get(o.HTTPMp3128URL)
	if err != nil {
		logrus.WithError(err).Fatal("Unable to get stream")
	}
	defer resp.Body.Close()

	streamer, format, err := mp3.Decode(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second))
	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.SetManagerFunc(layout)

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

	time.Sleep(2 * time.Second)
	// for {
	// 	select {
	// 	case <-done:
	// 		return
	// 	case <-time.After(time.Second):
	// 		speaker.Lock()
	// 		fmt.Printf("\033[2K\r%s / %s", format.SampleRate.D(streamer.Position()).Round(time.Second), tot.Round(time.Second))
	// 		speaker.Unlock()
	// 	}
	// }
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
