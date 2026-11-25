package music

import (
	"testing"
	"time"
)

func TestClassifyInput(t *testing.T) {
	tests := []struct {
		input    string
		expected InputKind
	}{
		{"never gonna give you up", InputKeyword},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", InputYouTubeVideo},
		{"https://youtu.be/dQw4w9WgXcQ", InputYouTubeVideo},
		{"https://www.youtube.com/playlist?list=PL123456", InputYouTubePlaylist},
		{"https://music.youtube.com/watch?v=dQw4w9WgXcQ", InputYouTubeMusicVideo},
		{"https://music.youtube.com/playlist?list=PL123456", InputYouTubeMusicPlaylist},
		{"https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT", InputSpotify},
		{"https://open.spotify.com/intl-id/playlist/37i9dQZF1DXcBWIGoYBM5M", InputSpotify},
		{"https://open.spotify.com/album/1ATL5GLyefJaxhQzSPVrLX", InputSpotify},
		{"https://spotify.link/abc123xyz", InputSpotify},
	}

	for _, tc := range tests {
		got := ClassifyInput(tc.input)
		if got != tc.expected {
			t.Errorf("ClassifyInput(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestTrackFormatting(t *testing.T) {
	tr := &Track{
		ID:       "123",
		Title:    "Bohemian Rhapsody",
		Artist:   "Queen",
		Duration: 355 * time.Second,
		Source:   SourceSpotify,
	}

	if got := tr.DisplayTitle(); got != "Queen - Bohemian Rhapsody" {
		t.Errorf("DisplayTitle() = %q, want %q", got, "Queen - Bohemian Rhapsody")
	}
	if got := tr.FormattedDuration(); got != "05:55" {
		t.Errorf("FormattedDuration() = %q, want %q", got, "05:55")
	}
	if got := tr.SourceBadge(); got != "[Spotify]" {
		t.Errorf("SourceBadge() = %q, want %q", got, "[Spotify]")
	}
}
