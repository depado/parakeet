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

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run stream_debug.go <playlist-url> <track-number>")
		fmt.Println("Example: go run stream_debug.go 'https://soundcloud.com/user/sets/playlist' 0")
		fmt.Println("Make sure PARAKEET_AUTH_TOKEN is set")
		os.Exit(1)
	}

	authToken := os.Getenv("PARAKEET_AUTH_TOKEN")
	if authToken == "" {
		fmt.Println("❌ PARAKEET_AUTH_TOKEN environment variable not set")
		os.Exit(1)
	}

	playlistURL := os.Args[1]
	trackIndex := os.Args[2]
	
	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := "https://api-v2.soundcloud.com"

	makeRequest := func(endpoint string) (*http.Response, error) {
		req, err := http.NewRequest("GET", baseURL+endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "OAuth "+strings.TrimSpace(authToken))
		req.Header.Set("User-Agent", "SoundCloud-Terminal-Player/1.0")
		return client.Do(req)
	}

	fmt.Println("🔍 Getting playlist...")
	
	// Step 1: Resolve playlist
	resp, err := makeRequest("/resolve?url=" + playlistURL)
	if err != nil {
		fmt.Printf("❌ Resolve failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		fmt.Printf("❌ Resolve failed: status %d\n", resp.StatusCode)
		os.Exit(1)
	}
	
	var resolveData map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&resolveData)
	
	playlistID := fmt.Sprintf("%.0f", resolveData["id"].(float64))
	fmt.Printf("✅ Playlist ID: %s\n", playlistID)
	
	// Step 2: Get playlist
	resp, err = makeRequest("/playlists/" + playlistID)
	if err != nil {
		fmt.Printf("❌ Playlist fetch failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	var playlist map[string]interface{}
	json.Unmarshal(body, &playlist)
	
	tracks, ok := playlist["tracks"].([]interface{})
	if !ok {
		fmt.Println("❌ No tracks found in playlist")
		os.Exit(1)
	}
	
	fmt.Printf("✅ Found %d tracks\n", len(tracks))
	
	// Step 3: Get first track for testing
	trackIdx := 0
	if trackIndex != "0" {
		// Parse track index if provided
		fmt.Sscanf(trackIndex, "%d", &trackIdx)
	}
	
	if trackIdx >= len(tracks) {
		fmt.Printf("❌ Track index %d out of range (0-%d)\n", trackIdx, len(tracks)-1)
		os.Exit(1)
	}
	
	track := tracks[trackIdx].(map[string]interface{})
	trackID := fmt.Sprintf("%.0f", track["id"].(float64))
	trackTitle := track["title"].(string)
	
	fmt.Printf("🎵 Testing track: %s (ID: %s)\n", trackTitle, trackID)
	
	// Step 4: Try different streaming endpoints
	streamEndpoints := []string{
		"/tracks/" + trackID + "/streams",
		"/tracks/" + trackID + "/stream",
		"/i1/tracks/" + trackID + "/streams",
		"/tracks/" + trackID,
	}
	
	for _, endpoint := range streamEndpoints {
		fmt.Printf("\n🔗 Trying: %s\n", endpoint)
		
		resp, err := makeRequest(endpoint)
		if err != nil {
			fmt.Printf("❌ Request failed: %v\n", err)
			continue
		}
		defer resp.Body.Close()
		
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("📋 Status: %d\n", resp.StatusCode)
		
		if resp.StatusCode == 200 {
			fmt.Printf("📋 Response: %s\n", string(body))
			
			var data map[string]interface{}
			if json.Unmarshal(body, &data) == nil {
				// Look for streaming URLs in response
				for key, value := range data {
					if strings.Contains(strings.ToLower(key), "url") || strings.Contains(strings.ToLower(key), "stream") {
						if strVal, ok := value.(string); ok && strings.HasPrefix(strVal, "http") {
							fmt.Printf("🎯 Found stream URL in '%s': %s\n", key, strVal)
						}
					}
				}
			}
		} else {
			fmt.Printf("📋 Error: %s\n", string(body))
		}
	}
	
	fmt.Println("\n🏁 Stream debug completed!")
}
