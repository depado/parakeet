package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Depado/soundcloud"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/depado/parakeet/player"
	"github.com/depado/parakeet/utils"
)

// Color palette and styles
var (
	// Colors
	primaryColor   = lipgloss.Color("#FBBF24") // Yellow/amber
	secondaryColor = lipgloss.Color("#10B981") // Green
	accentColor    = lipgloss.Color("#3B82F6") // Blue
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	borderColor    = lipgloss.Color("#374151") // Border gray

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB"))

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 2).
			Margin(0, 1)

	// Header styles
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1)

	// Track list styles
	trackStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Padding(0, 1)

	selectedTrackStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(accentColor).
				Bold(true).
				Padding(0, 1)

	currentTrackStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Bold(true).
				Padding(0, 1)

	// Info panel styles
	infoLabelStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	infoValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F3F4F6"))

	// Player styles
	progressBarStyle = lipgloss.NewStyle().
				Foreground(primaryColor)

	progressEmptyStyle = lipgloss.NewStyle().
				Foreground(borderColor)

	timeStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	// Help text styles
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true).
			Padding(0, 1)

	keyStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)
)

// BubbleTeaModel holds the state for the bubbletea UI
type BubbleTeaModel struct {
	// Terminal dimensions
	width  int
	height int

	// Music data
	tracks       soundcloud.Tracks
	currentTrack *soundcloud.Track
	streamer     *player.StreamerFormat

	// UI state
	selectedTrack int
	cursor        time.Duration
	scrollOffset  int // For track list scrolling

	// View state
	currentView ViewMode

	// Animation state
	animationFrame int

	// Channels for communication with the player
	trackChan  chan<- soundcloud.Track
	toggleChan chan<- bool
	nextChan   <-chan bool

	// State flags
	ready             bool
	isPlaying         bool
	currentTrackIndex int // Track the index of the currently playing track
}

// ViewMode represents different view modes
type ViewMode int

const (
	ViewMain ViewMode = iota
	ViewHelp
)

// TickMsg represents a periodic update message
type TickMsg time.Time

// StreamerUpdateMsg represents an update to the current streamer
type StreamerUpdateMsg struct {
	Track    soundcloud.Track
	Streamer *player.StreamerFormat
}

// TrackPositionMsg represents the current position in the track
type TrackPositionMsg time.Duration

// NextTrackMsg indicates the player moved to the next track automatically
type NextTrackMsg struct{}

// NewBubbleTeaModel creates a new bubbletea model
func NewBubbleTeaModel(tracks soundcloud.Tracks, trackChan chan<- soundcloud.Track, toggleChan chan<- bool, nextChan <-chan bool) *BubbleTeaModel {
	return &BubbleTeaModel{
		tracks:            tracks,
		selectedTrack:     0,
		scrollOffset:      0,
		currentView:       ViewMain,
		animationFrame:    0,
		trackChan:         trackChan,
		toggleChan:        toggleChan,
		nextChan:          nextChan,
		ready:             false,
		isPlaying:         false,
		currentTrackIndex: -1, // Initialize to -1 (no track playing yet)
	}
}

// Init implements tea.Model
func (m *BubbleTeaModel) Init() tea.Cmd {
	return tea.Batch(
		tea.WindowSize(),
		tickCmd(),
		nextTrackCmd(m.nextChan),
	)
}

// Update implements tea.Model
func (m *BubbleTeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		if m.currentView == ViewHelp {
			// In help view, any key returns to main
			m.currentView = ViewMain
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?", "h":
			// Toggle help view
			if m.currentView == ViewMain {
				m.currentView = ViewHelp
			} else {
				m.currentView = ViewMain
			}
		case "up", "k":
			if m.selectedTrack > 0 {
				m.selectedTrack--
			}
		case "down", "j":
			if m.selectedTrack < len(m.tracks)-1 {
				m.selectedTrack++
			}
		case "g":
			// Go to top
			m.selectedTrack = 0
		case "G":
			// Go to bottom
			m.selectedTrack = len(m.tracks) - 1
		case "pageup":
			// Page up (10 tracks)
			m.selectedTrack -= 10
			if m.selectedTrack < 0 {
				m.selectedTrack = 0
			}
		case "pagedown":
			// Page down (10 tracks)
			m.selectedTrack += 10
			if m.selectedTrack >= len(m.tracks) {
				m.selectedTrack = len(m.tracks) - 1
			}
		case "enter":
			// Send selected track to player
			if m.selectedTrack < len(m.tracks) {
				selectedTrack := m.tracks[m.selectedTrack]

				// Send to player
				m.trackChan <- selectedTrack
				m.isPlaying = true
				return m, waitForStreamerCmd()
			}
		case " ": // spacebar
			// Toggle play/pause
			if m.toggleChan != nil {
				m.toggleChan <- true
				m.isPlaying = !m.isPlaying
			}
		}

	case TickMsg:
		// Increment animation frame for any animated elements
		m.animationFrame++
		return m, tickCmd()

	case StreamerUpdateMsg:
		// Update streamer and track
		m.streamer = msg.Streamer
		m.currentTrack = &msg.Track
		m.isPlaying = true
		m.ready = true

		// Find the index of this track in the playlist
		for i, track := range m.tracks {
			if track.ID == msg.Track.ID {
				m.currentTrackIndex = i
				m.selectedTrack = i // Also update the selected track to match
				break
			}
		}

		return m, nil

	case TrackPositionMsg:
		m.cursor = time.Duration(msg)
		return m, nil

	case NextTrackMsg:
		// Player automatically moved to next track
		if m.selectedTrack < len(m.tracks)-1 {
			m.selectedTrack++
		} else {
			m.selectedTrack = 0 // Loop back to beginning
		}
		// Update current track index to match
		m.currentTrackIndex = m.selectedTrack

		// Send the next track to the player
		if m.selectedTrack < len(m.tracks) {
			nextTrack := m.tracks[m.selectedTrack]
			m.trackChan <- nextTrack
		}

		return m, tea.Batch(nextTrackCmd(m.nextChan), waitForStreamerCmd())

	default:
		return m, nil
	}

	return m, nil
}

// View implements tea.Model
func (m *BubbleTeaModel) View() string {
	if !m.ready {
		return baseStyle.Render("Loading...")
	}

	switch m.currentView {
	case ViewHelp:
		return m.renderHelpView()
	default:
		return m.renderMainView()
	}
}

// renderMainView renders the main interface
func (m *BubbleTeaModel) renderMainView() string {
	// Create the main layout
	header := m.renderStyledHeader()
	content := m.renderStyledContent()
	playerCard := m.renderPlayerCard()
	footer := m.renderStyledFooter()

	// Join all sections
	return baseStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			header,
			content,
			playerCard,
			footer,
		),
	)
}

// renderHelpView renders the help screen
func (m *BubbleTeaModel) renderHelpView() string {
	helpTitle := titleStyle.Render("🆘 Help & Controls")

	helpContent := []string{
		"",
		keyStyle.Render("Navigation:"),
		"  ↑/k         - Move up",
		"  ↓/j         - Move down",
		"  g           - Go to top",
		"  G           - Go to bottom",
		"  PageUp      - Move up 10 tracks",
		"  PageDown    - Move down 10 tracks",
		"",
		keyStyle.Render("Playback:"),
		"  Enter       - Play selected track",
		"  Space       - Pause/Resume",
		"",
		keyStyle.Render("Interface:"),
		"  h/?         - Show this help",
		"  q/Ctrl+C    - Quit",
		"",
		helpStyle.Render("Press any key to return to the main interface"),
	}

	content := strings.Join(helpContent, "\n")

	// Center the help content
	helpBox := panelStyle.
		Width(m.width-4).
		Height(m.height-6).
		Align(lipgloss.Center, lipgloss.Center).
		Render(lipgloss.JoinVertical(lipgloss.Center, helpTitle, content))

	return baseStyle.Render(helpBox)
}

// renderStyledHeader renders the logo and title
func (m *BubbleTeaModel) renderStyledHeader() string {
	// Simple title without status (status is shown in player card)
	title := titleStyle.Render("🦜 Parakeet")

	// Clean header without extra spacing
	headerBox := lipgloss.NewStyle().
		Width(m.width).
		Margin(1, 0, 1, 0).
		Padding(0, 2).
		Render(title)

	return headerBox
}

// renderPlayerCard renders the combined currently playing track and player controls
func (m *BubbleTeaModel) renderPlayerCard() string {
	if m.streamer == nil || m.currentTrack == nil {
		return ""
	}

	var cardContent strings.Builder

	// Currently playing track info
	trackInfo := fmt.Sprintf("♪ %s - %s", m.currentTrack.Title, m.currentTrack.User.Username)

	// Create a more elegant and realistic visualizer
	var statusText string
	if m.isPlaying {
		// Simple static triangle for playing state
		statusText = lipgloss.NewStyle().Foreground(secondaryColor).Render("▶ PLAYING")
	} else {
		statusText = lipgloss.NewStyle().Foreground(mutedColor).Render("⏸ PAUSED")
	}

	// Style the track title
	styledTrack := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render(trackInfo)

	// Combine track and status
	nowPlayingLine := fmt.Sprintf("%s  %s", styledTrack, statusText)
	cardContent.WriteString(nowPlayingLine)
	cardContent.WriteString("\n\n")

	// Calculate progress
	progress := float64(m.cursor) / float64(m.streamer.TotalDuration)
	if progress > 1 {
		progress = 1
	}
	if progress < 0 {
		progress = 0
	}

	// Time displays
	currentTime := timeStyle.Render(utils.FormatDuration(m.cursor))
	totalTime := timeStyle.Render(utils.FormatDuration(m.streamer.TotalDuration))

	// Progress bar
	barWidth := m.width - len(currentTime) - len(totalTime) - 12 // Account for spacing and borders
	if barWidth < 10 {
		barWidth = 10
	}

	filledWidth := int(float64(barWidth) * progress)
	emptyWidth := barWidth - filledWidth

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("▒", emptyWidth)

	progressBar := progressBarStyle.Render(filled) +
		progressEmptyStyle.Render(empty)

	// Combine time and progress bar
	playerLine := fmt.Sprintf("%s ║%s║ %s", currentTime, progressBar, totalTime)
	cardContent.WriteString(playerLine)

	// Style the entire player card
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Margin(1, 1, 0, 1).
		Width(m.width - 4).
		Align(lipgloss.Center).
		Render(cardContent.String())
}

// renderStyledContent renders the main content area with track list and info
func (m *BubbleTeaModel) renderStyledContent() string {
	trackList := m.renderStyledTrackList()
	trackInfo := m.renderStyledTrackInfo()

	// Create two columns
	contentWidth := m.width - 6                // Account for margins and borders
	leftWidth := contentWidth * 3 / 5          // 60% for track list
	rightWidth := contentWidth - leftWidth - 4 // Remaining for info, with proper border spacing

	// Ensure minimum widths
	if leftWidth < 30 {
		leftWidth = 30
	}
	if rightWidth < 25 {
		rightWidth = 25
	}

	// Ensure the total width doesn't exceed available space
	if leftWidth+rightWidth+8 > m.width {
		rightWidth = m.width - leftWidth - 8
		if rightWidth < 20 {
			rightWidth = 20
			leftWidth = m.width - rightWidth - 8
		}
	}

	leftPanel := panelStyle.
		Width(leftWidth).
		Height(m.height - 15). // More conservative reduction to ensure header is visible
		Render(trackList)

	rightPanel := panelStyle.
		Width(rightWidth).
		Height(m.height - 15). // More conservative reduction to ensure header is visible
		Render(trackInfo)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

// renderStyledTrackList renders the track list with better styling
func (m *BubbleTeaModel) renderStyledTrackList() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Tracklist"))
	b.WriteString("\n\n")

	// Calculate visible tracks based on available height
	maxTracks := m.height - 15 // Conservative reduction to ensure all UI elements are visible
	if maxTracks < 1 {
		maxTracks = 1
	}

	// Calculate scroll offset to keep selected track visible
	if m.selectedTrack < m.scrollOffset {
		m.scrollOffset = m.selectedTrack
	} else if m.selectedTrack >= m.scrollOffset+maxTracks {
		m.scrollOffset = m.selectedTrack - maxTracks + 1
	}

	startIdx := m.scrollOffset
	endIdx := startIdx + maxTracks
	if endIdx > len(m.tracks) {
		endIdx = len(m.tracks)
	}

	// Render visible tracks
	for i := startIdx; i < endIdx; i++ {
		track := m.tracks[i]
		trackText := fmt.Sprintf("%s - %s", track.Title, track.User.Username)

		// Truncate if too long
		maxWidth := (m.width * 3 / 5) - 10 // Account for panel padding and borders
		if len(trackText) > maxWidth {
			trackText = trackText[:maxWidth-3] + "..."
		}

		// Add track number
		prefix := fmt.Sprintf("%3d. ", i+1)

		// Style based on state
		var styledLine string
		if i == m.selectedTrack {
			// Selected track (cursor position)
			styledLine = selectedTrackStyle.Render(prefix + "▶ " + trackText)
		} else if i == m.currentTrackIndex && m.currentTrack != nil {
			// Currently playing track (use index instead of ID to handle duplicates)
			styledLine = currentTrackStyle.Render(prefix + "♪ " + trackText)
		} else {
			// Regular track
			styledLine = trackStyle.Render(prefix + "  " + trackText)
		}

		b.WriteString(styledLine)
		b.WriteString("\n")
	}

	// Add scroll indicator if needed
	if len(m.tracks) > maxTracks {
		scrollInfo := fmt.Sprintf("\n%s %d-%d of %d tracks",
			lipgloss.NewStyle().Foreground(mutedColor).Render("Showing"),
			startIdx+1, endIdx, len(m.tracks))
		b.WriteString(scrollInfo)
	}

	return b.String()
}

// renderStyledTrackInfo renders the track information panel
func (m *BubbleTeaModel) renderStyledTrackInfo() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Track Information"))
	b.WriteString("\n\n")

	if m.currentTrack == nil || m.streamer == nil {
		b.WriteString(helpStyle.Render("No track currently playing"))
		return b.String()
	}

	// Track details
	details := []struct {
		label string
		value string
	}{
		{"Title", m.currentTrack.Title},
		{"Artist", m.currentTrack.User.Username},
		{"Duration", utils.FormatDuration(m.streamer.TotalDuration)},
		{"Progress", fmt.Sprintf("%s / %s",
			utils.FormatDuration(m.cursor),
			utils.FormatDuration(m.streamer.TotalDuration))},
	}

	for _, detail := range details {
		label := infoLabelStyle.Render(fmt.Sprintf("%-10s", detail.label+":"))
		value := infoValueStyle.Render(detail.value)
		b.WriteString(fmt.Sprintf("%s %s\n", label, value))
	}

	b.WriteString("\n")

	// Stats
	stats := fmt.Sprintf("❤️  %s   💬 %s",
		infoValueStyle.Render(fmt.Sprintf("%d likes", m.currentTrack.LikesCount)),
		infoValueStyle.Render(fmt.Sprintf("%d comments", m.currentTrack.CommentCount)))
	b.WriteString(stats)
	b.WriteString("\n\n")

	// URL (truncated)
	url := m.currentTrack.PermalinkURL
	maxUrlWidth := (m.width * 2 / 5) - 10
	if len(url) > maxUrlWidth {
		url = url[:maxUrlWidth-3] + "..."
	}
	b.WriteString(infoLabelStyle.Render("URL: "))
	b.WriteString(infoValueStyle.Render(url))

	return b.String()
}

// renderStyledPlayer renders the progress bar and controls
// renderStyledFooter renders the help/controls footer
func (m *BubbleTeaModel) renderStyledFooter() string {
	controls := []string{
		keyStyle.Render("↑/↓") + " navigate",
		keyStyle.Render("Enter") + " play",
		keyStyle.Render("Space") + " pause/play",
		keyStyle.Render("h/?") + " help",
		keyStyle.Render("q") + " quit",
	}

	controlsText := strings.Join(controls, " • ")

	// Add track count info
	var trackInfo string
	if len(m.tracks) > 0 {
		trackInfo = fmt.Sprintf("Track %d of %d", m.selectedTrack+1, len(m.tracks))
	}

	// Combine controls and track info
	footerContent := controlsText
	if trackInfo != "" {
		footerContent = fmt.Sprintf("%s | %s", controlsText,
			lipgloss.NewStyle().Foreground(mutedColor).Render(trackInfo))
	}

	return lipgloss.NewStyle().
		Foreground(mutedColor).
		Italic(true).
		Width(m.width).
		Align(lipgloss.Center).
		Margin(1, 0, 0, 0).
		Render(footerContent)
} // Commands

// tickCmd returns a command that sends periodic tick messages
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// waitForStreamerCmd waits for streamer updates
func waitForStreamerCmd() tea.Cmd {
	return func() tea.Msg {
		// This would be implemented to wait for streamer updates
		// For now, return nil
		return nil
	}
}

// nextTrackCmd listens for automatic track changes
func nextTrackCmd(nextChan <-chan bool) tea.Cmd {
	return func() tea.Msg {
		<-nextChan // Block until we receive a signal
		return NextTrackMsg{}
	}
}

// UpdateStreamer updates the current streamer information
func (m *BubbleTeaModel) UpdateStreamer(track soundcloud.Track, streamer *player.StreamerFormat) {
	m.currentTrack = &track
	m.streamer = streamer
	// For initialization, find the track and update both selected and current index
	for i, t := range m.tracks {
		if t.ID == track.ID {
			m.selectedTrack = i
			m.currentTrackIndex = i
			break
		}
	}
	// Don't set isPlaying here - let the actual playback state determine this
}

// UpdatePosition updates the current playback position
func (m *BubbleTeaModel) UpdatePosition(position time.Duration) {
	m.cursor = position
}

// SetPlaying updates the playing state
func (m *BubbleTeaModel) SetPlaying(playing bool) {
	m.isPlaying = playing
}
