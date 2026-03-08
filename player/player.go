package player

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"

	"github.com/Depado/parakeet/soundcloud"
)

// Player holds the necessary data and structs to control the player
type Player struct {
	c         *soundcloud.Client
	tc        <-chan soundcloud.Track
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
func NewPlayer(c *soundcloud.Client, tc <-chan soundcloud.Track, toggle <-chan bool, next chan<- bool, streamerc chan<- *StreamerFormat) *Player {
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
func (p *Player) StreamerFromTrack(t soundcloud.Track) (*StreamerFormat, io.ReadCloser, error) {
	var (
		err  error
		resp *http.Response
	)

	// Get track service and fetch full track details with media transcodings
	ts, track, err := p.c.Track().FromTrack(&t, false)
	if err != nil {
		return nil, nil, fmt.Errorf("from track: %w", err)
	}

	url, err := ts.Stream(soundcloud.ProgressiveMP3)
	if err != nil {
		return nil, nil, fmt.Errorf("get stream url: %w", err)
	}

	// Removed the debug logging that was cluttering the UI

	if resp, err = http.Get(url); err != nil { // nolint: bodyclose
		return nil, nil, fmt.Errorf("http request for mp3 failed: %w", err)
	}

	// Check if the response is valid
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("http request failed with status: %d", resp.StatusCode)
	}

	streamer, format, err := mp3.Decode(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unable to decode MP3: %w", err)
	}

	return &StreamerFormat{
		Streamer:      streamer,
		Format:        format,
		TotalDuration: time.Duration(int64(track.Duration)) * time.Millisecond,
	}, resp.Body, nil
}

// Start will start the player, starting with the given track
func (p *Player) Start(t soundcloud.Track) error {
	var sf *StreamerFormat
	var s io.ReadCloser
	var err error
	if sf, s, err = p.StreamerFromTrack(t); err != nil {
		return fmt.Errorf("unable to get streamer from track: %w", err)
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
			// Removed excessive logging for track switching
			if sf, s, err = p.StreamerFromTrack(track); err != nil {
				// If an error occurs, go to the next track (no logging to avoid UI clutter)
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
