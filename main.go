package soundcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client represents the OAuth-based SoundCloud client
type Client struct {
	httpClient *http.Client
	authToken  string
	baseURL    string
}

// User represents a SoundCloud user
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// Media and Transcoding structures for new SoundCloud API
type Media struct {
	Transcodings []Transcoding `json:"transcodings"`
}

type Transcoding struct {
	URL     string `json:"url"`
	Preset  string `json:"preset"`
	Quality string `json:"quality"`
	Format  struct {
		Protocol string `json:"protocol"`
		MimeType string `json:"mime_type"`
	} `json:"format"`
}

// Track represents a SoundCloud track
type Track struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Duration     int    `json:"duration"`
	PermalinkURL string `json:"permalink_url"`
	LikesCount   int    `json:"likes_count"`
	CommentCount int    `json:"comment_count"`
	StreamURL    string `json:"stream_url"`   // Direct stream URL if available
	DownloadURL  string `json:"download_url"` // Download URL fallback
	User         User   `json:"user"`
	Media        Media  `json:"media"`
}

// Tracks is a slice of Track
type Tracks []Track

// Playlist represents a SoundCloud playlist
type Playlist struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Tracks Tracks `json:"tracks"`
	User   User   `json:"user"`
}

// NewClient creates a new OAuth-based SoundCloud client
func NewClient(authToken string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		authToken: strings.TrimSpace(authToken),
		baseURL:   "https://api-v2.soundcloud.com", // API v2 for OAuth
	}
}

// makeRequest creates an authenticated HTTP request
func (c *Client) makeRequest(method, endpoint string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + endpoint
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "OAuth "+c.authToken)
	req.Header.Set("User-Agent", "SoundCloud-Terminal-Player/1.0")

	return req, nil
}

// PlaylistWrapper is a wrapper around Playlist to mimic the old API
type PlaylistWrapper struct {
	playlist *Playlist
}

// Get returns the wrapped playlist
func (pw *PlaylistWrapper) Get() (*Playlist, error) {
	return pw.playlist, nil
}

// Playlist returns a playlist wrapper (mimics old API structure)
func (c *Client) Playlist() *PlaylistService {
	return &PlaylistService{client: c}
}

// PlaylistService handles playlist operations
type PlaylistService struct {
	client *Client
}

// FromURL gets a playlist from URL - handles both regular playlists and special URLs like likes
func (ps *PlaylistService) FromURL(url string) (*PlaylistWrapper, error) {
	// Handle special URLs for likes
	if strings.Contains(url, "/you/likes") || strings.Contains(url, "soundcloud.com/you/likes") {
		return ps.getUserLikes()
	}

	// Everything else -> try normal resolve (also works for private playlists & "your-playback")
	return ps.resolvePlaylistFromURL(url)
}

// getUserLikes fetches the user's liked tracks using the correct V2 API endpoint
func (ps *PlaylistService) getUserLikes() (*PlaylistWrapper, error) {
	// First get user info to get the user ID
	req, err := ps.client.makeRequest("GET", "/me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create me request: %w", err)
	}

	resp, err := ps.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("me API request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// Now get the user's likes using the correct V2 endpoint: /users/{ID}/track_likes
	endpoint := fmt.Sprintf("/users/%d/track_likes?limit=200", user.ID)
	req, err = ps.client.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create likes request: %w", err)
	}

	resp, err = ps.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get likes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("likes API request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	// The V2 track_likes endpoint returns a collection of like objects
	var likesResponse struct {
		Collection []struct {
			Track Track `json:"track"`
		} `json:"collection"`
		NextHref string `json:"next_href"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&likesResponse); err != nil {
		return nil, fmt.Errorf("failed to decode likes: %w", err)
	}

	// Extract tracks from likes response
	var tracks Tracks
	for _, item := range likesResponse.Collection {
		if item.Track.ID != 0 { // Make sure track exists
			tracks = append(tracks, item.Track)
		}
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("no liked tracks found")
	}

	// Create a virtual playlist for likes
	playlist := &Playlist{
		ID:     -1, // Virtual ID for likes playlist
		Title:  fmt.Sprintf("Your Likes (%d tracks)", len(tracks)),
		Tracks: tracks,
		User:   user,
	}

	return &PlaylistWrapper{playlist: playlist}, nil
}

// resolvePlaylistFromURL handles regular playlist URLs using the resolve endpoint
func (ps *PlaylistService) resolvePlaylistFromURL(url string) (*PlaylistWrapper, error) {
	req, err := ps.client.makeRequest("GET", "/resolve?url="+url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolve request: %w", err)
	}

	resp, err := ps.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make resolve request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("resolve API request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var resolveData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&resolveData); err != nil {
		return nil, fmt.Errorf("failed to decode resolve response: %w", err)
	}

	// Handle both playlist and track resolves
	kind, ok := resolveData["kind"].(string)
	if !ok {
		return nil, fmt.Errorf("could not determine resource kind from resolve response")
	}

	switch kind {
	case "playlist":
		idFloat, ok := resolveData["id"].(float64)
		if !ok {
			return nil, fmt.Errorf("could not extract numeric playlist ID from resolve response")
		}
		playlistID := fmt.Sprintf("%.0f", idFloat)

		req, err = ps.client.makeRequest("GET", "/playlists/"+playlistID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create playlist request: %w", err)
		}

		resp, err = ps.client.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make playlist request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("playlist API request failed: status %d, body: %s", resp.StatusCode, string(body))
		}

		var playlist Playlist
		if err := json.NewDecoder(resp.Body).Decode(&playlist); err != nil {
			return nil, fmt.Errorf("failed to decode playlist: %w", err)
		}

		return &PlaylistWrapper{playlist: &playlist}, nil

	case "track":
		// Single track - create a playlist with one track
		var track Track
		trackData, _ := json.Marshal(resolveData)
		if err := json.Unmarshal(trackData, &track); err != nil {
			return nil, fmt.Errorf("failed to decode track: %w", err)
		}

		playlist := &Playlist{
			ID:     -2, // Virtual ID for single track playlist
			Title:  "Single Track: " + track.Title,
			Tracks: Tracks{track},
			User:   track.User,
		}

		return &PlaylistWrapper{playlist: playlist}, nil

	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

// TrackService handles track operations
type TrackService struct {
	client *Client
}

// Track returns a track service
func (c *Client) Track() *TrackService {
	return &TrackService{client: c}
}

// TrackWrapper wraps track operations
type TrackWrapper struct {
	track  *Track
	client *Client
}

// FromTrack creates a track wrapper from a track - but first fetches full track details
func (ts *TrackService) FromTrack(t *Track, progressive bool) (*TrackWrapper, *Track, error) {
	// Step 1: Fetch full track details to get media.transcodings
	trackID := fmt.Sprintf("%d", t.ID)
	req, err := ts.client.makeRequest("GET", "/tracks/"+trackID, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create track request: %w", err)
	}

	resp, err := ts.client.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make track request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("track API request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var fullTrack Track
	if err := json.NewDecoder(resp.Body).Decode(&fullTrack); err != nil {
		return nil, nil, fmt.Errorf("failed to decode track: %w", err)
	}

	return &TrackWrapper{track: &fullTrack, client: ts.client}, &fullTrack, nil
}

// StreamQuality represents stream quality
type StreamQuality string

const (
	ProgressiveMP3 StreamQuality = "progressive_mp3"
)

// Stream gets the streaming URL for a track using the same logic as the debugger
func (tw *TrackWrapper) Stream(quality StreamQuality) (string, error) {
	// Look for progressive MP3 transcoding (exactly like the debugger)
	for _, t := range tw.track.Media.Transcodings {
		if t.Format.Protocol == "progressive" && strings.Contains(t.Format.MimeType, "mpeg") {
			// Second request to get the final stream URL
			endpoint := strings.TrimPrefix(t.URL, tw.client.baseURL)
			req, err := tw.client.makeRequest("GET", endpoint, nil)
			if err != nil {
				continue
			}

			resp, err := tw.client.httpClient.Do(req)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var data map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
					if finalURL, ok := data["url"].(string); ok && finalURL != "" {
						return finalURL, nil
					}
				}
			}
		}
	}

	// Fallback options (same as before)
	if tw.track.StreamURL != "" {
		return tw.track.StreamURL, nil
	}
	if tw.track.DownloadURL != "" {
		return tw.track.DownloadURL, nil
	}

	return "", fmt.Errorf("no streaming URL found for track %d", tw.track.ID)
}

