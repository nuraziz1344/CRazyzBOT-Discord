package youtube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go-tube/pkg/music"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// ParsePlaylist fetches all tracks in a YouTube or YouTube Music playlist in a single operation
// without making N separate video info requests.
func ParsePlaylist(playlistURL string) ([]*music.Track, error) {
	parsedURL, err := url.ParseRequestURI(playlistURL)
	if err != nil {
		return nil, errors.New("invalid_playlist_url")
	}

	listID := parsedURL.Query().Get("list")
	if listID == "" {
		return nil, errors.New("invalid_playlist_url")
	}

	isMusic := IsYouTubeMusicURL(playlistURL)
	source := music.SourceYouTube
	if isMusic {
		source = music.SourceYouTubeMusic
	}

	// 1. Primary: InnerTube browse endpoint (works without API key, returns titles, authors, durations)
	tracks, err := parsePlaylistInnerTube(listID, source, isMusic)
	if err == nil && len(tracks) > 0 {
		return tracks, nil
	}

	// 2. Fallback: YouTube Data API v3 if key is configured
	apiKey := strings.TrimSpace(os.Getenv("YT_APIKEY"))
	if apiKey != "" && apiKey != "Abc123" {
		tracks, err = parsePlaylistDataAPI(listID, apiKey, source, isMusic)
		if err == nil && len(tracks) > 0 {
			return tracks, nil
		}
	}

	return nil, errors.New("playlist_not_found")
}

func parsePlaylistInnerTube(listID string, source music.SourceType, isMusic bool) ([]*music.Track, error) {
	browseID := listID
	if !strings.HasPrefix(browseID, "VL") {
		browseID = "VL" + browseID
	}

	payload := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20250101.00.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"browseId": browseID,
	}
	bodyBytes, _ := json.Marshal(payload)

	resBody, err := doYouTubeRequest(http.MethodPost, "https://www.youtube.com/youtubei/v1/browse?prettyPrint=false", bytes.NewReader(bodyBytes), map[string]string{
		"Content-Type": "application/json",
		"Origin":       "https://www.youtube.com",
		"Referer":      "https://www.youtube.com/",
	})
	if err != nil {
		return nil, err
	}

	data := string(resBody)
	items := gjson.Get(data, "contents.twoColumnBrowseResultsRenderer.tabs.0.tabRenderer.content.sectionListRenderer.contents.0.itemSectionRenderer.contents.0.playlistVideoListRenderer.contents")
	if !items.Exists() {
		return nil, errors.New("playlist items not found in innertube response")
	}

	var tracks []*music.Track
	items.ForEach(func(_, item gjson.Result) bool {
		pvr := item.Get("playlistVideoRenderer")
		if !pvr.Exists() {
			return true
		}

		vID := pvr.Get("videoId").String()
		if vID == "" {
			return true
		}

		title := pvr.Get("title.runs.0.text").String()
		if title == "" {
			title = pvr.Get("title.simpleText").String()
		}
		if title == "" || title == "[Deleted video]" || title == "[Private video]" {
			return true
		}

		artist := pvr.Get("shortBylineText.runs.0.text").String()
		artist = strings.TrimSuffix(artist, " - Topic")

		lengthSec, _ := strconv.Atoi(pvr.Get("lengthSeconds").String())
		var duration time.Duration
		if lengthSec > 0 {
			duration = time.Duration(lengthSec) * time.Second
		} else {
			duration = parseDurationString(pvr.Get("lengthText.simpleText").String())
		}

		webURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID)
		if isMusic {
			webURL = fmt.Sprintf("https://music.youtube.com/watch?v=%s", vID)
		}

		tracks = append(tracks, &music.Track{
			ID:         vID,
			Title:      title,
			Artist:     artist,
			Duration:   duration,
			Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vID),
			WebURL:     webURL,
			YouTubeURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID),
			Source:     source,
		})
		return true
	})

	if len(tracks) == 0 {
		return nil, errors.New("playlist_not_found")
	}
	return tracks, nil
}

func parsePlaylistDataAPI(listID, apiKey string, source music.SourceType, isMusic bool) ([]*music.Track, error) {
	reqURL := fmt.Sprintf("https://youtube.googleapis.com/youtube/v3/playlistItems?part=snippet,contentDetails&maxResults=50&playlistId=%s&key=%s",
		url.QueryEscape(listID), url.QueryEscape(apiKey))

	resBody, err := doYouTubeRequest(http.MethodGet, reqURL, nil, nil)
	if err != nil {
		return nil, err
	}

	data := string(resBody)
	if gjson.Get(data, "error.code").Int() != 0 {
		return nil, errors.New("playlist_not_found")
	}

	items := gjson.Get(data, "items")
	if !items.Exists() || !items.IsArray() {
		return nil, errors.New("playlist_not_found")
	}

	var tracks []*music.Track
	items.ForEach(func(_, item gjson.Result) bool {
		if item.Get("snippet.resourceId.kind").String() != "youtube#video" {
			return true
		}
		vID := item.Get("snippet.resourceId.videoId").String()
		title := item.Get("snippet.title").String()
		if vID == "" || title == "" || title == "Deleted video" || title == "Private video" {
			return true
		}

		artist := strings.TrimSuffix(item.Get("snippet.videoOwnerChannelTitle").String(), " - Topic")
		webURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID)
		if isMusic {
			webURL = fmt.Sprintf("https://music.youtube.com/watch?v=%s", vID)
		}

		tracks = append(tracks, &music.Track{
			ID:         vID,
			Title:      title,
			Artist:     artist,
			Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vID),
			WebURL:     webURL,
			YouTubeURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID),
			Source:     source,
		})
		return true
	})

	if len(tracks) == 0 {
		return nil, errors.New("playlist_not_found")
	}
	return tracks, nil
}
