package soundcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client represents the new OAuth-based SoundCloud client
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

// Track represents a SoundCloud track
type Track struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Duration     int    `json:"duration"`
	PermalinkURL string `json:"permalink_url"`
	LikesCount   int    `json:"likes_count"`
	CommentCount int    `json:"comment_count"`
	User         User   `json:"user"`
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
		baseURL:   "https://api-v2.soundcloud.com",
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

// FromURL gets a playlist from URL
func (ps *PlaylistService) FromURL(url string) (*PlaylistWrapper, error) {
	// Extract playlist ID from URL
	playlistID := extractPlaylistID(url)
	
	req, err := ps.client.makeRequest("GET", "/playlists/"+playlistID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ps.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed: status %d", resp.StatusCode)
	}

	var playlist Playlist
	if err := json.NewDecoder(resp.Body).Decode(&playlist); err != nil {
		return nil, fmt.Errorf("failed to decode playlist: %w", err)
	}

	return &PlaylistWrapper{playlist: &playlist}, nil
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

// FromTrack creates a track wrapper from a track
func (ts *TrackService) FromTrack(t *Track, progressive bool) (*TrackWrapper, *Track, error) {
	return &TrackWrapper{track: t, client: ts.client}, t, nil
}

// StreamQuality represents stream quality
type StreamQuality string

const (
	ProgressiveMP3 StreamQuality = "progressive_mp3"
)

// Stream gets the streaming URL for a track
func (tw *TrackWrapper) Stream(quality StreamQuality) (string, error) {
	req, err := tw.client.makeRequest("GET", fmt.Sprintf("/tracks/%d/stream", tw.track.ID), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := tw.client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed: status %d", resp.StatusCode)
	}

	var streamResp struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&streamResp); err != nil {
		return "", fmt.Errorf("failed to decode stream response: %w", err)
	}

	return streamResp.URL, nil
}

// extractPlaylistID extracts playlist ID from URL
func extractPlaylistID(url string) string {
	// Simple extraction logic - enhance as needed
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "sets" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return parts[len(parts)-1]
}

