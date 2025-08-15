package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// SoundCloudDebugClient for testing API calls
type SoundCloudDebugClient struct {
	client    *http.Client
	authToken string
	baseURL   string
}

// NewDebugClient creates a debug client
func NewDebugClient(authToken string) *SoundCloudDebugClient {
	return &SoundCloudDebugClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		authToken: strings.TrimSpace(authToken),
		baseURL:   "https://api-v2.soundcloud.com",
	}
}

// makeRequest creates an authenticated HTTP request
func (c *SoundCloudDebugClient) makeRequest(method, endpoint string) (*http.Request, error) {
	url := c.baseURL + endpoint
	fmt.Printf("🌐 Making request to: %s\n", url)

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "OAuth "+c.authToken)
	req.Header.Set("User-Agent", "SoundCloud-Terminal-Player/1.0")
	req.Header.Set("Accept", "application/json")

	fmt.Printf("🔑 Authorization: OAuth %s...\n", c.authToken[:20])
	return req, nil
}

// testAuth tests if the OAuth token works
func (c *SoundCloudDebugClient) testAuth() error {
	fmt.Println("🔍 Testing OAuth token...")

	req, err := c.makeRequest("GET", "/me")
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("📋 Response Status: %d\n", resp.StatusCode)
	fmt.Printf("📋 Response Body: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid token: status %d", resp.StatusCode)
	}

	var user map[string]interface{}
	if err := json.Unmarshal(body, &user); err == nil {
		fmt.Printf("✅ Authenticated as: %v (ID: %.0f)\n", user["username"], user["id"])
	}

	return nil
}

// testResolve tests the resolve endpoint and returns playlist ID if found
func (c *SoundCloudDebugClient) testResolve(url string) (string, error) {
	fmt.Printf("🔍 Testing resolve endpoint with URL: %s\n", url)

	endpoint := "/resolve?url=" + url
	req, err := c.makeRequest("GET", endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("📋 Resolve Response Status: %d\n", resp.StatusCode)
	fmt.Printf("📋 Resolve Response Body: %s\n", string(body))

	// Playlist-ID extrahieren
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		if id, ok := result["id"].(float64); ok {
			return fmt.Sprintf("%.0f", id), nil
		}
	}

	return "", nil
}

// testDirectPlaylist tests direct playlist access
func (c *SoundCloudDebugClient) testDirectPlaylist(playlistID string) error {
	fmt.Printf("🔍 Testing direct playlist access with ID: %s\n", playlistID)

	endpoint := "/playlists/" + playlistID
	req, err := c.makeRequest("GET", endpoint)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("📋 Playlist Response Status: %d\n", resp.StatusCode)
	fmt.Printf("📋 Playlist Response Body: %s\n", string(body))

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run debug.go <soundcloud-playlist-url>")
		fmt.Println("Make sure PARAKEET_AUTH_TOKEN is set")
		os.Exit(1)
	}

	authToken := os.Getenv("PARAKEET_AUTH_TOKEN")
	if authToken == "" {
		fmt.Println("❌ PARAKEET_AUTH_TOKEN environment variable not set")
		os.Exit(1)
	}

	playlistURL := os.Args[1]
	client := NewDebugClient(authToken)

	fmt.Println("🚀 SoundCloud API Debug Tool")
	fmt.Println("============================")

	// Test 1: Check OAuth token
	if err := client.testAuth(); err != nil {
		fmt.Printf("❌ Auth test failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()

	// Test 2: Try resolve endpoint
	playlistID, err := client.testResolve(playlistURL)
	if err != nil {
		fmt.Printf("❌ Resolve test failed: %v\n", err)
	}

	fmt.Println()

	// Test 3: Try direct playlist access (only if ID found)
	if playlistID != "" {
		if err := client.testDirectPlaylist(playlistID); err != nil {
			fmt.Printf("❌ Direct playlist test failed: %v\n", err)
		}
	} else {
		fmt.Println("⚠️ No playlist ID found in resolve response.")
	}

	fmt.Println("\n🏁 Debug completed!")
}

