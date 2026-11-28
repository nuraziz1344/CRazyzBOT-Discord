package youtube

import (
	"go-tube/pkg/music"
	"testing"
	"time"

	"github.com/tidwall/gjson"
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

func TestExtractVideoID(t *testing.T) {
	if got := ExtractVideoID("https://youtu.be/dQw4w9WgXcQ"); got != "dQw4w9WgXcQ" {
		t.Errorf("ExtractVideoID() = %q, want dQw4w9WgXcQ", got)
	}
	if got := ExtractVideoID("not-a-url"); got != "" {
		t.Errorf("ExtractVideoID(invalid) = %q, want empty", got)
	}
}

func TestParsePlaylistPanelItemsWebRemix(t *testing.T) {
	raw := `{
		"contents": {
			"singleColumnMusicWatchNextResultsRenderer": {
				"tabbedRenderer": {
					"watchNextTabbedResultsRenderer": {
						"tabs": [{
							"tabRenderer": {
								"content": {
									"musicQueueRenderer": {
										"content": {
											"playlistPanelRenderer": {
												"contents": [
													{
														"playlistPanelVideoRenderer": {
															"videoId": "abc12345678",
															"title": {"runs": [{"text": "First Song"}]},
															"shortBylineText": {"runs": [{"text": "Artist One - Topic"}]},
															"lengthText": {"runs": [{"text": "3:21"}]}
														}
													},
													{
														"playlistPanelVideoRenderer": {
															"videoId": "del12345678",
															"title": {"simpleText": "[Deleted video]"}
														}
													}
												]
											}
										}
									}
								}
							}
						}]
					}
				}
			}
		}
	}`

	items := gjson.Get(raw, "contents.singleColumnMusicWatchNextResultsRenderer.tabbedRenderer.watchNextTabbedResultsRenderer.tabs.0.tabRenderer.content.musicQueueRenderer.content.playlistPanelRenderer.contents")
	tracks := parsePlaylistPanelItems(items, music.SourceYouTubeMusic, true)
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	got := tracks[0]
	if got.ID != "abc12345678" || got.Title != "First Song" || got.Artist != "Artist One" {
		t.Fatalf("unexpected track metadata: %+v", got)
	}
	if got.Duration != (3*60+21)*time.Second {
		t.Fatalf("expected duration 3m21s, got %v", got.Duration)
	}
	if got.Source != music.SourceYouTubeMusic || got.WebURL != "https://music.youtube.com/watch?v=abc12345678" {
		t.Fatalf("unexpected source/webURL: %s / %s", got.Source, got.WebURL)
	}
}

func TestParsePlaylistPanelItemsWeb(t *testing.T) {
	raw := `{
		"contents": {
			"twoColumnWatchNextResults": {
				"playlist": {
					"playlist": {
						"contents": [
							{
								"playlistPanelVideoRenderer": {
									"videoId": "xyz12345678",
									"title": {"simpleText": "Web Mix Track"},
									"longBylineText": {"runs": [{"text": "Band Two"}]},
									"lengthText": {"simpleText": "4:05"}
								}
							},
							{
								"playlistPanelVideoRenderer": {
									"videoId": "prv12345678",
									"title": {"runs": [{"text": "[Private video]"}]}
								}
							}
						]
					}
				}
			}
		}
	}`

	items := gjson.Get(raw, "contents.twoColumnWatchNextResults.playlist.playlist.contents")
	tracks := parsePlaylistPanelItems(items, music.SourceYouTube, false)
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	got := tracks[0]
	if got.ID != "xyz12345678" || got.Title != "Web Mix Track" || got.Artist != "Band Two" {
		t.Fatalf("unexpected track metadata: %+v", got)
	}
	if got.Duration != (4*60+5)*time.Second {
		t.Fatalf("expected duration 4m5s, got %v", got.Duration)
	}
	if got.Source != music.SourceYouTube || got.WebURL != "https://www.youtube.com/watch?v=xyz12345678" {
		t.Fatalf("unexpected source/webURL: %s / %s", got.Source, got.WebURL)
	}
}
