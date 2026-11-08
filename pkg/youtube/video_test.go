package youtube

import (
	"testing"
	"time"
)

func TestExtractYouTubeVideoID(t *testing.T) {
	tests := []struct {
		url    string
		wantID string
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://music.youtube.com/watch?v=dQw4w9WgXcQ&list=RDAMVMdQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://example.com/watch?v=123", ""},
	}

	for _, tc := range tests {
		got := extractYouTubeVideoID(tc.url)
		if got != tc.wantID {
			t.Errorf("extractYouTubeVideoID(%q) = %q, want %q", tc.url, got, tc.wantID)
		}
	}
}

func TestParseDurationString(t *testing.T) {
	if got := parseDurationString("3:45"); got != (3*60+45)*time.Second {
		t.Errorf("parseDurationString(3:45) = %v", got)
	}
	if got := parseDurationString("1:02:03"); got != (3600+120+3)*time.Second {
		t.Errorf("parseDurationString(1:02:03) = %v", got)
	}
}
