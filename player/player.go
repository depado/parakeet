package player

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Depado/soundcloud"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/sirupsen/logrus"
)

// Player holds the necessary data and structs to control the player
type Player struct {
	c         *soundcloud.Client
	tc        <-chan soundcloud.Track
	toggle    <-chan bool
	next      chan<- bool
	streamerc chan<- *StreamerFormat
	positionc chan<- time.Duration
	source    io.ReadCloser
}

// StreamerFormat is a struct holding both a streamer and a format
type StreamerFormat struct {
	Streamer      beep.StreamSeekCloser
	Format        beep.Format
	TotalDuration time.Duration
	Track         soundcloud.Track
}

// NewPlayer will return a new player
func NewPlayer(c *soundcloud.Client, tc <-chan soundcloud.Track, toggle <-chan bool, next chan<- bool, streamerc chan<- *StreamerFormat, positionc chan<- time.Duration) *Player {
	return &Player{
		c:         c,
		tc:        tc,
		toggle:    toggle,
		streamerc: streamerc,
		positionc: positionc,
		next:      next,
	}
}

// StreamerFromTrack will retrieve a stream from the track and return the
// streamer, format and total duration, as well as the source
func (p *Player) StreamerFromTrack(t soundcloud.Track) (*StreamerFormat, io.ReadCloser, error) {
	var (
		err  error
		resp *http.Response
	)

	ts, track, err := p.c.Track().FromTrack(&t, false)
	if err != nil {
		return nil, nil, fmt.Errorf("from track: %w", err)
	}

	url, err := ts.Stream(soundcloud.ProgressiveMP3)
	if err != nil {
		return nil, nil, fmt.Errorf("get stream url: %w", err)
	}

	if resp, err = http.Get(url); err != nil { // nolint: bodyclose
		return nil, nil, fmt.Errorf("http request for mp3 failed: %w", err)
	}

	streamer, format, err := mp3.Decode(resp.Body)
	if err != nil {
		logrus.WithError(err).Fatal("Unable to decode MP3")
	}

	return &StreamerFormat{
		Streamer:      streamer,
		Format:        format,
		TotalDuration: time.Duration(int64(track.Duration)) * time.Millisecond,
		Track:         t,
	}, resp.Body, nil
}

// Start will start the player, starting with the given track
func (p *Player) Start(t soundcloud.Track) error {
	var sf *StreamerFormat
	var s io.ReadCloser
	var err error
	var startTime time.Time
	var pausedDuration time.Duration

	if sf, s, err = p.StreamerFromTrack(t); err != nil {
		return fmt.Errorf("unable to get streamer from track: %w", err)
	}
	p.source = s
	defer p.source.Close()

	if err = speaker.Init(sf.Format.SampleRate, sf.Format.SampleRate.N(100*time.Millisecond)); err != nil {
		return err
	}

	ctrl := &beep.Ctrl{Streamer: sf.Streamer, Paused: false}
	startTime = time.Now()

	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		p.next <- true
	})))
	p.streamerc <- sf

	// Position ticker for sending position updates
	positionTicker := time.NewTicker(100 * time.Millisecond)
	defer positionTicker.Stop()

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
			startTime = time.Now()
			pausedDuration = 0
			speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
				p.next <- true
			})))
			p.streamerc <- sf
		case <-p.toggle:
			speaker.Lock()
			wasPaused := ctrl.Paused
			ctrl.Paused = !ctrl.Paused
			if wasPaused {
				// Resuming - reset start time accounting for paused duration
				startTime = time.Now().Add(-pausedDuration)
			} else {
				// Pausing - calculate total paused duration so far
				pausedDuration = time.Since(startTime)
			}
			speaker.Unlock()
		case <-positionTicker.C:
			// Send current position if not paused
			if !ctrl.Paused {
				currentPos := time.Since(startTime)
				if currentPos < sf.TotalDuration {
					p.positionc <- currentPos
				}
			}
		}
	}
}
