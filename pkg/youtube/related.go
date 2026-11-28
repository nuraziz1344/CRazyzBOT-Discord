package youtube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go-tube/pkg/music"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// ExtractVideoID extracts the 11-character YouTube video ID from a URL.
func ExtractVideoID(rawURL string) string {
	return extractYouTubeVideoID(rawURL)
}

// GetRelatedTracks fetches related tracks for a YouTube video ID using YouTube Music Radio
// (Tier 1) and YouTube Web Mix (Tier 2), with in-memory caching.
func GetRelatedTracks(videoID string) ([]*music.Track, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return nil, errors.New("empty video ID")
	}

	cacheKey := "related:" + videoID
	if cached := getCachedTracks(cacheKey); len(cached) > 0 {
		return cached, nil
	}

	tracks, err := fetchMusicRadioTracks(videoID)
	if err == nil && hasValidRelatedTracks(tracks, videoID) {
		setCachedTracks(cacheKey, tracks)
		return tracks, nil
	}

	webTracks, webErr := fetchWebMixTracks(videoID)
	if webErr == nil && hasValidRelatedTracks(webTracks, videoID) {
		setCachedTracks(cacheKey, webTracks)
		return webTracks, nil
	}

	if webErr != nil {
		return nil, webErr
	}
	if err != nil {
		return nil, err
	}
	return nil, errors.New("no related tracks found")
}

func hasValidRelatedTracks(tracks []*music.Track, seedVideoID string) bool {
	return len(tracks) > 1 || (len(tracks) > 0 && tracks[0].ID != seedVideoID)
}

func fetchMusicRadioTracks(videoID string) ([]*music.Track, error) {
	payload := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB_REMIX",
				"clientVersion": "1.20250101.01.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"videoId":     videoID,
		"playlistId":  "RDAMVM" + videoID,
		"isAudioOnly": true,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"Origin":       "https://music.youtube.com",
		"Referer":      "https://music.youtube.com/",
	}

	resBody, err := doYouTubeRequest(
		http.MethodPost,
		"https://music.youtube.com/youtubei/v1/next?prettyPrint=false",
		bytes.NewReader(bodyBytes),
		headers,
	)
	if err != nil {
		return nil, err
	}

	items := gjson.Get(
		string(resBody),
		"contents.singleColumnMusicWatchNextResultsRenderer.tabbedRenderer.watchNextTabbedResultsRenderer.tabs.0.tabRenderer.content.musicQueueRenderer.content.playlistPanelRenderer.contents",
	)
	tracks := parsePlaylistPanelItems(items, music.SourceYouTubeMusic, true)
	if len(tracks) == 0 {
		return nil, errors.New("no music radio tracks found")
	}
	return tracks, nil
}

func fetchWebMixTracks(videoID string) ([]*music.Track, error) {
	payload := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20250101.00.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"videoId":    videoID,
		"playlistId": "RD" + videoID,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"Origin":       "https://www.youtube.com",
		"Referer":      "https://www.youtube.com/",
	}

	resBody, err := doYouTubeRequest(
		http.MethodPost,
		"https://www.youtube.com/youtubei/v1/next?prettyPrint=false",
		bytes.NewReader(bodyBytes),
		headers,
	)
	if err != nil {
		return nil, err
	}

	items := gjson.Get(
		string(resBody),
		"contents.twoColumnWatchNextResults.playlist.playlist.contents",
	)
	tracks := parsePlaylistPanelItems(items, music.SourceYouTube, false)
	if len(tracks) == 0 {
		return nil, errors.New("no web mix tracks found")
	}
	return tracks, nil
}

func parsePlaylistPanelItems(items gjson.Result, source music.SourceType, isMusic bool) []*music.Track {
	if !items.Exists() || !items.IsArray() {
		return nil
	}

	var tracks []*music.Track
	items.ForEach(func(_, item gjson.Result) bool {
		ppvr := item.Get("playlistPanelVideoRenderer")
		if !ppvr.Exists() {
			return true
		}

		vID := strings.TrimSpace(ppvr.Get("videoId").String())
		if vID == "" {
			return true
		}

		title := ppvr.Get("title.runs.0.text").String()
		if title == "" {
			title = ppvr.Get("title.simpleText").String()
		}
		title = strings.TrimSpace(title)
		if title == "" || title == "[Deleted video]" || title == "[Private video]" {
			return true
		}

		artist := ppvr.Get("shortBylineText.runs.0.text").String()
		if artist == "" {
			artist = ppvr.Get("longBylineText.runs.0.text").String()
		}
		artist = strings.TrimSuffix(strings.TrimSpace(artist), " - Topic")

		lengthStr := ppvr.Get("lengthText.runs.0.text").String()
		if lengthStr == "" {
			lengthStr = ppvr.Get("lengthText.simpleText").String()
		}
		duration := parseDurationString(lengthStr)

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

	return tracks
}
