package youtube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go-tube/pkg/cookies"
	"go-tube/pkg/music"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

type cacheItem struct {
	tracks    []*music.Track
	expiresAt time.Time
}

var (
	searchCacheMu sync.RWMutex
	searchCache   = make(map[string]cacheItem)
)

const cacheTTL = 30 * time.Minute

func getCachedTracks(key string) []*music.Track {
	searchCacheMu.RLock()
	item, ok := searchCache[key]
	searchCacheMu.RUnlock()
	if !ok || time.Now().After(item.expiresAt) {
		return nil
	}
	res := make([]*music.Track, len(item.tracks))
	for i, t := range item.tracks {
		res[i] = t.Clone()
	}
	return res
}

func setCachedTracks(key string, tracks []*music.Track) {
	if len(tracks) == 0 {
		return
	}
	searchCacheMu.Lock()
	if len(searchCache) > 500 {
		// Simple eviction when cache exceeds 500 keys
		now := time.Now()
		for k, v := range searchCache {
			if now.After(v.expiresAt) {
				delete(searchCache, k)
			}
		}
		if len(searchCache) > 500 {
			searchCache = make(map[string]cacheItem)
		}
	}
	cloned := make([]*music.Track, len(tracks))
	for i, t := range tracks {
		cloned[i] = t.Clone()
	}
	searchCache[key] = cacheItem{
		tracks:    cloned,
		expiresAt: time.Now().Add(cacheTTL),
	}
	searchCacheMu.Unlock()
}

// IsYouTubeMusicURL checks if the URL belongs to music.youtube.com
func IsYouTubeMusicURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "music.youtube.com"
}

// GetVideoInfo fetches metadata for a YouTube or YouTube Music video URL.
func GetVideoInfo(videoURL string) (*music.Track, error) {
	vID := extractYouTubeVideoID(videoURL)
	if vID == "" {
		return nil, errors.New("invalid_video_url")
	}

	source := music.SourceYouTube
	webURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID)
	if IsYouTubeMusicURL(videoURL) {
		source = music.SourceYouTubeMusic
		webURL = fmt.Sprintf("https://music.youtube.com/watch?v=%s", vID)
	}

	cacheKey := "video:" + string(source) + ":" + vID
	if cached := getCachedTracks(cacheKey); len(cached) > 0 {
		return cached[0], nil
	}

	// 1. Try InnerTube player endpoint (fast, no API key required, returns duration & author)
	track, err := fetchInnerTubeVideoInfo(vID, source, webURL)
	if err == nil && track != nil && track.Title != "" {
		setCachedTracks(cacheKey, []*music.Track{track})
		return track, nil
	}

	// 2. Fallback to YouTube oEmbed (no API key required)
	track, err = fetchOEmbedVideoInfo(vID, source, webURL)
	if err == nil && track != nil && track.Title != "" {
		setCachedTracks(cacheKey, []*music.Track{track})
		return track, nil
	}

	// 3. Fallback to YouTube Data API v3 if key is configured
	apiKey := strings.TrimSpace(os.Getenv("YT_APIKEY"))
	if apiKey != "" && apiKey != "Abc123" {
		track, err = fetchDataAPIVideoInfo(vID, apiKey, source, webURL)
		if err == nil && track != nil && track.Title != "" {
			setCachedTracks(cacheKey, []*music.Track{track})
			return track, nil
		}
	}

	return nil, errors.New("failed to fetch video info")
}

func fetchInnerTubeVideoInfo(vID string, source music.SourceType, webURL string) (*music.Track, error) {
	payload := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20250101.00.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"videoId": vID,
	}
	bodyBytes, _ := json.Marshal(payload)

	resBody, err := doYouTubeRequest(http.MethodPost, "https://www.youtube.com/youtubei/v1/player?prettyPrint=false", bytes.NewReader(bodyBytes), map[string]string{
		"Content-Type": "application/json",
		"Origin":       "https://www.youtube.com",
		"Referer":      "https://www.youtube.com/",
	})
	if err != nil {
		return nil, err
	}

	data := string(resBody)
	title := gjson.Get(data, "videoDetails.title").String()
	if title == "" {
		return nil, errors.New("empty videoDetails title")
	}
	author := gjson.Get(data, "videoDetails.author").String()
	lengthSec, _ := strconv.Atoi(gjson.Get(data, "videoDetails.lengthSeconds").String())

	return &music.Track{
		ID:         vID,
		Title:      title,
		Artist:     strings.TrimSuffix(author, " - Topic"),
		Duration:   time.Duration(lengthSec) * time.Second,
		Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vID),
		WebURL:     webURL,
		YouTubeURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID),
		Source:     source,
	}, nil
}

func fetchOEmbedVideoInfo(vID string, source music.SourceType, webURL string) (*music.Track, error) {
	oembedURL := fmt.Sprintf("https://www.youtube.com/oembed?url=%s&format=json", url.QueryEscape(fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID)))
	resBody, err := doYouTubeRequest(http.MethodGet, oembedURL, nil, nil)
	if err != nil {
		return nil, err
	}
	data := string(resBody)
	title := gjson.Get(data, "title").String()
	if title == "" {
		return nil, errors.New("empty oembed title")
	}
	author := gjson.Get(data, "author_name").String()
	return &music.Track{
		ID:         vID,
		Title:      title,
		Artist:     strings.TrimSuffix(author, " - Topic"),
		Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vID),
		WebURL:     webURL,
		YouTubeURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID),
		Source:     source,
	}, nil
}

func fetchDataAPIVideoInfo(vID, apiKey string, source music.SourceType, webURL string) (*music.Track, error) {
	reqURL := fmt.Sprintf("https://youtube.googleapis.com/youtube/v3/videos?part=snippet&id=%s&key=%s", vID, apiKey)
	resBody, err := doYouTubeRequest(http.MethodGet, reqURL, nil, nil)
	if err != nil {
		return nil, err
	}
	data := string(resBody)
	title := gjson.Get(data, "items.0.snippet.title").String()
	if title == "" {
		return nil, errors.New("empty API video title")
	}
	author := gjson.Get(data, "items.0.snippet.channelTitle").String()
	return &music.Track{
		ID:         vID,
		Title:      title,
		Artist:     strings.TrimSuffix(author, " - Topic"),
		Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vID),
		WebURL:     webURL,
		YouTubeURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID),
		Source:     source,
	}, nil
}

// SearchTracks searches YouTube / YouTube Music and returns up to `limit` tracks.
func SearchTracks(keyword string, limit int, preferMusic bool) ([]*music.Track, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("empty search keyword")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	if strings.HasPrefix(keyword, "http://") || strings.HasPrefix(keyword, "https://") {
		t, err := GetVideoInfo(keyword)
		if err != nil {
			return nil, err
		}
		return []*music.Track{t}, nil
	}

	cacheKey := fmt.Sprintf("search:%v:%d:%s", preferMusic, limit, strings.ToLower(keyword))
	if cached := getCachedTracks(cacheKey); len(cached) > 0 {
		return cached, nil
	}

	// 1. Primary: InnerTube Search (fast, supports multiple results, duration, channel name, no quota)
	tracks, err := searchInnerTube(keyword, limit, preferMusic)
	if err == nil && len(tracks) > 0 {
		setCachedTracks(cacheKey, tracks)
		return tracks, nil
	}

	// 2. Fallback: YouTube Data API v3 if key is available
	apiKey := strings.TrimSpace(os.Getenv("YT_APIKEY"))
	if apiKey != "" && apiKey != "Abc123" {
		tracks, err = searchDataAPI(keyword, limit, apiKey, preferMusic)
		if err == nil && len(tracks) > 0 {
			setCachedTracks(cacheKey, tracks)
			return tracks, nil
		}
	}

	return nil, errors.New("no search results found")
}

// SearchVideo returns the single top track for a keyword or URL.
func SearchVideo(keyword string) (*music.Track, error) {
	tracks, err := SearchTracks(keyword, 1, false)
	if err != nil || len(tracks) == 0 {
		return nil, err
	}
	return tracks[0], nil
}

func searchInnerTube(keyword string, limit int, preferMusic bool) ([]*music.Track, error) {
	payload := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20250101.00.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"query": keyword,
	}
	bodyBytes, _ := json.Marshal(payload)

	resBody, err := doYouTubeRequest(http.MethodPost, "https://www.youtube.com/youtubei/v1/search?prettyPrint=false", bytes.NewReader(bodyBytes), map[string]string{
		"Content-Type": "application/json",
		"Origin":       "https://www.youtube.com",
		"Referer":      "https://www.youtube.com/",
	})
	if err != nil {
		return nil, err
	}

	data := string(resBody)
	sections := gjson.Get(data, "contents.twoColumnSearchResultsRenderer.primaryContents.sectionListRenderer.contents")
	if !sections.Exists() {
		return nil, errors.New("unexpected innertube search response")
	}

	source := music.SourceYouTube
	if preferMusic {
		source = music.SourceYouTubeMusic
	}

	var results []*music.Track
	sections.ForEach(func(_, section gjson.Result) bool {
		items := section.Get("itemSectionRenderer.contents")
		items.ForEach(func(_, item gjson.Result) bool {
			vr := item.Get("videoRenderer")
			if !vr.Exists() {
				return true
			}
			vID := vr.Get("videoId").String()
			if vID == "" {
				return true
			}

			title := vr.Get("title.runs.0.text").String()
			if title == "" {
				title = vr.Get("title.simpleText").String()
			}
			if title == "" {
				return true
			}

			artist := vr.Get("ownerText.runs.0.text").String()
			if artist == "" {
				artist = vr.Get("longBylineText.runs.0.text").String()
			}
			artist = strings.TrimSuffix(artist, " - Topic")

			lengthStr := vr.Get("lengthText.simpleText").String()
			duration := parseDurationString(lengthStr)

			webURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID)
			if preferMusic {
				webURL = fmt.Sprintf("https://music.youtube.com/watch?v=%s", vID)
			}

			results = append(results, &music.Track{
				ID:         vID,
				Title:      title,
				Artist:     artist,
				Duration:   duration,
				Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vID),
				WebURL:     webURL,
				YouTubeURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID),
				Source:     source,
			})

			return len(results) < limit
		})
		return len(results) < limit
	})

	if len(results) == 0 {
		return nil, errors.New("no video results found")
	}
	return results, nil
}

func searchDataAPI(keyword string, limit int, apiKey string, preferMusic bool) ([]*music.Track, error) {
	reqURL := fmt.Sprintf("https://youtube.googleapis.com/youtube/v3/search?part=snippet&type=video&maxResults=%d&q=%s&key=%s",
		limit, url.QueryEscape(keyword), apiKey)
	resBody, err := doYouTubeRequest(http.MethodGet, reqURL, nil, nil)
	if err != nil {
		return nil, err
	}

	data := string(resBody)
	items := gjson.Get(data, "items")
	if !items.Exists() || !items.IsArray() {
		return nil, errors.New("empty search items")
	}

	source := music.SourceYouTube
	if preferMusic {
		source = music.SourceYouTubeMusic
	}

	var results []*music.Track
	items.ForEach(func(_, item gjson.Result) bool {
		vID := item.Get("id.videoId").String()
		title := item.Get("snippet.title").String()
		artist := strings.TrimSuffix(item.Get("snippet.channelTitle").String(), " - Topic")
		if vID == "" || title == "" {
			return true
		}

		webURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID)
		if preferMusic {
			webURL = fmt.Sprintf("https://music.youtube.com/watch?v=%s", vID)
		}

		results = append(results, &music.Track{
			ID:         vID,
			Title:      title,
			Artist:     artist,
			Thumbnail:  fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", vID),
			WebURL:     webURL,
			YouTubeURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", vID),
			Source:     source,
		})
		return len(results) < limit
	})

	if len(results) == 0 {
		return nil, errors.New("no results from Data API")
	}
	return results, nil
}

// parseDurationString parses "3:45" or "1:02:30" into time.Duration.
func parseDurationString(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parts := strings.Split(s, ":")
	var totalSec int
	for _, p := range parts {
		val, err := strconv.Atoi(p)
		if err != nil {
			return 0
		}
		totalSec = totalSec*60 + val
	}
	return time.Duration(totalSec) * time.Second
}

var videoIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:youtube\.com/watch\?.*v=|music\.youtube\.com/watch\?.*v=|youtu\.be/)([a-zA-Z0-9_-]{11})`),
	regexp.MustCompile(`youtube\.com/embed/([a-zA-Z0-9_-]{11})`),
	regexp.MustCompile(`youtube\.com/v/([a-zA-Z0-9_-]{11})`),
	regexp.MustCompile(`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`),
}

func extractYouTubeVideoID(rawURL string) string {
	for _, re := range videoIDPatterns {
		if match := re.FindStringSubmatch(rawURL); len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

func doYouTubeRequest(method, rawURL string, body io.Reader, headers map[string]string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	if Proxy != "" {
		if proxyURL, err := url.Parse(Proxy); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	if CookieFile != "" {
		cookies.Apply(req, CookieFile)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}
