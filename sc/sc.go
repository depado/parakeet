package sc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dghubble/sling"
	"github.com/yanatan16/golang-soundcloud/soundcloud"
)

// StreamParam is a simple struct containing the data to send as query params
type StreamParam struct {
	ClientID string `url:"client_id"`
}

// StreamOutput is a simple response
type StreamOutput struct {
	HTTPMp3128URL    string `json:"http_mp3_128_url"`
	HlsOpus64URL     string `json:"hls_opus_64_url"`
	HlsMp3128URL     string `json:"hls_mp3_128_url"`
	PreviewMp3128URL string `json:"preview_mp3_128_url"`
}

// Client is the struct holding our data and client to fetch data on Soundcloud
type Client struct {
	*soundcloud.Api
	clientID string
	hc       *http.Client
	Tracks   []*soundcloud.Track
}

// Stream will retrieve a streamable content respecting SoundCloud's workflow
func (c *Client) Stream(t *soundcloud.Track) (*StreamOutput, error) {
	var (
		err  error
		resp *http.Response
		req  *http.Request
		out  StreamOutput
	)

	if req, err = sling.New().
		Get(fmt.Sprintf("https://api.soundcloud.com/tracks/%d/streams", t.Id)).
		QueryStruct(StreamParam{ClientID: c.clientID}).
		Request(); err != nil {
		return nil, fmt.Errorf("create query: %w", err)
	}

	if resp, err = c.hc.Do(req); err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer resp.Body.Close()

	o, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if err = json.Unmarshal(o, &out); err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}

	return &out, nil
}

// NewClient returns a new Clinet
func NewClient(clientID string) *Client {
	return &Client{
		Api: &soundcloud.Api{
			ClientId: clientID,
		},
		clientID: clientID,
		hc: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}
