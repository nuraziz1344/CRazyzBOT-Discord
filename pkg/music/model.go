package music

import (
	"fmt"
	"strings"
	"time"
)

type SourceType string

const (
	SourceYouTube      SourceType = "YouTube"
	SourceYouTubeMusic SourceType = "YouTube Music"
	SourceSpotify      SourceType = "Spotify"
)

// Track represents a unified music track item stored in the queue.
// Only lightweight metadata and original/external IDs are stored in the queue;
// raw audio stream URLs are resolved just-in-time before playback.
type Track struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Artist      string        `json:"artist,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Thumbnail   string        `json:"thumbnail,omitempty"`
	WebURL      string        `json:"web_url"`
	YouTubeURL  string        `json:"youtube_url,omitempty"`
	StreamQuery string        `json:"stream_query,omitempty"`
	Source      SourceType    `json:"source"`
	RequestedBy string        `json:"requested_by,omitempty"`
}

// DisplayTitle returns a clean "Artist - Title" or "Title" string.
func (t *Track) DisplayTitle() string {
	if t == nil {
		return "Unknown Track"
	}
	title := strings.TrimSpace(t.Title)
	artist := strings.TrimSpace(t.Artist)
	if artist != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(artist)) {
		return fmt.Sprintf("%s - %s", artist, title)
	}
	if title != "" {
		return title
	}
	return "Unknown Track"
}

// FormattedDuration returns duration in mm:ss or hh:mm:ss format.
func (t *Track) FormattedDuration() string {
	if t == nil || t.Duration <= 0 {
		return "Live/Unknown"
	}
	totalSeconds := int(t.Duration.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// SourceBadge returns a short badge string for the track source.
func (t *Track) SourceBadge() string {
	if t == nil {
		return "[Music]"
	}
	switch t.Source {
	case SourceYouTubeMusic:
		return "[YT Music]"
	case SourceSpotify:
		return "[Spotify]"
	default:
		return "[YouTube]"
	}
}

// Clone creates a shallow copy of the track so each queued entry is independent.
func (t *Track) Clone() *Track {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}
