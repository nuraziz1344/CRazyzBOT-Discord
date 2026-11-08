package spotify

import (
	"testing"
)

func TestParseSpotifyURL(t *testing.T) {
	tests := []struct {
		url      string
		wantType string
		wantID   string
		wantErr  bool
	}{
		{"https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT?si=123", "track", "4cOdK2wGLETKBW3PvgPWqT", false},
		{"https://open.spotify.com/intl-id/track/4cOdK2wGLETKBW3PvgPWqT", "track", "4cOdK2wGLETKBW3PvgPWqT", false},
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M", "playlist", "37i9dQZF1DXcBWIGoYBM5M", false},
		{"https://open.spotify.com/album/1ATL5GLyefJaxhQzSPVrLX", "album", "1ATL5GLyefJaxhQzSPVrLX", false},
		{"https://open.spotify.com/user/someone", "", "", true},
	}

	for _, tc := range tests {
		resType, resID, err := ParseSpotifyURL(tc.url)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseSpotifyURL(%q) error = %v, wantErr %v", tc.url, err, tc.wantErr)
			continue
		}
		if resType != tc.wantType || resID != tc.wantID {
			t.Errorf("ParseSpotifyURL(%q) = (%q, %q), want (%q, %q)", tc.url, resType, resID, tc.wantType, tc.wantID)
		}
	}
}
