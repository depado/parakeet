package player

import (
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Depado/parakeet/sc"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/pkg/errors"
	"github.com/yanatan16/golang-soundcloud/soundcloud"
)

// Player holds the necessary data and structs to control the player
type Player struct {
	c         *sc.Client
	tc        <-chan *soundcloud.Track
	toggle    <-chan bool
	next      chan<- bool
	streamerc chan<- *StreamerFormat
	source    io.ReadCloser
}

// StreamerFormat is a struct holding both a streamer and a format
type StreamerFormat struct {
	Streamer      beep.StreamSeekCloser
	Format        beep.Format
	TotalDuration time.Duration
}

// NewPlayer will return a new player
func NewPlayer(c *sc.Client, tc <-chan *soundcloud.Track, toggle <-chan bool, next chan<- bool, streamerc chan<- *StreamerFormat) *Player {
	return &Player{
		c:         c,
		tc:        tc,
		toggle:    toggle,
		streamerc: streamerc,
		next:      next,
	}
}

// StreamerFromTrack will retrieve a stream from the track and return the
// streamer, format and total duration, as well as the source
func (p *Player) StreamerFromTrack(t *soundcloud.Track) (*StreamerFormat, io.ReadCloser, error) {
	var err error
	var resp *http.Response
	var output *sc.StreamOutput

	if output, err = p.c.Stream(t); err != nil {
		return nil, nil, errors.Wrap(err, "get stream URLs")
	}

	if resp, err = http.Get(output.HTTPMp3128URL); err != nil { // nolint: bodyclose
		return nil, nil, errors.Wrap(err, "http request for mp3 failed")
	}

	streamer, format, err := mp3.Decode(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return &StreamerFormat{
		Streamer:      streamer,
		Format:        format,
		TotalDuration: time.Duration(int64(t.Duration)) * time.Millisecond,
	}, resp.Body, nil
}

// Start will start the player, starting with the given track
func (p *Player) Start(t *soundcloud.Track) error {
	var sf *StreamerFormat
	var s io.ReadCloser
	var err error
	if sf, s, err = p.StreamerFromTrack(t); err != nil {
		return errors.Wrap(err, "unable to get streamer from track")
	}
	p.source = s
	defer p.source.Close()

	if err = speaker.Init(sf.Format.SampleRate, sf.Format.SampleRate.N(100*time.Millisecond)); err != nil {
		return err
	}
	ctrl := &beep.Ctrl{Streamer: sf.Streamer, Paused: false}
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		p.next <- true
	})))
	p.streamerc <- sf
	for {
		select {
		case track := <-p.tc:
			if sf, s, err = p.StreamerFromTrack(track); err != nil {
				// If an error occurs, go to the next track
				p.streamerc <- nil
				p.next <- true
				break
			}
			speaker.Clear()
			if p.source != nil {
				p.source.Close()
			}
			p.source = s
			ctrl.Streamer = sf.Streamer
			ctrl.Paused = false
			speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
				p.next <- true
			})))
			p.streamerc <- sf
		case <-p.toggle:
			speaker.Lock()
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()
		}

	}
}
