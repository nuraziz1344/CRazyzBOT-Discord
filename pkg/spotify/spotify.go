package spotify

import (
	"errors"
	"fmt"
	"go-tube/pkg/cookies"
	"go-tube/pkg/music"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

var (
	UserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	CookieFile = ""
	Proxy      = ""
)

var (
	spotifyPathRegex = regexp.MustCompile(`/(?:intl-[a-zA-Z-]+/)?(track|playlist|album)/([a-zA-Z0-9]+)`)
	nextDataRegex    = regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)
	ogTitleRegex     = regexp.MustCompile(`<meta property="og:title" content="([^"]+)"`)
	ogDescRegex      = regexp.MustCompile(`<meta property="og:description" content="([^"]+)"`)
	ogImageRegex     = regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
)

// IsSpotifyURL checks if rawURL points to Spotify (open.spotify.com, spotify.link, spoti.fi).
func IsSpotifyURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "open.spotify.com" || host == "play.spotify.com" || host == "spotify.link" || host == "spoti.fi"
}

// ParseSpotifyURL extracts resource type ("track", "playlist", "album") and ID from a Spotify URL.
func ParseSpotifyURL(rawURL string) (resourceType string, resourceID string, err error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", errors.New("invalid_spotify_url")
	}

	host := strings.ToLower(u.Hostname())
	if host == "spotify.link" || host == "spoti.fi" {
		resolved, rErr := resolveRedirect(rawURL)
		if rErr != nil {
			return "", "", rErr
		}
		u, err = url.Parse(resolved)
		if err != nil {
			return "", "", errors.New("invalid_spotify_url")
		}
	}

	matches := spotifyPathRegex.FindStringSubmatch(u.Path)
	if len(matches) < 3 {
		return "", "", errors.New("unsupported_spotify_url")
	}
	return strings.ToLower(matches[1]), matches[2], nil
}

// Resolve fetches one or more tracks from a Spotify track, playlist, or album URL.
// Tracks returned have StreamQuery populated ("Artist - Title") for lazy YouTube resolution.
func Resolve(rawURL string) ([]*music.Track, error) {
	resType, resID, err := ParseSpotifyURL(rawURL)
	if err != nil {
		return nil, err
	}

	tracks, err := scrapeEmbed(resType, resID)
	if err == nil && len(tracks) > 0 {
		return tracks, nil
	}

	if resType == "track" {
		track, fbErr := scrapeTrackFallback(resID)
		if fbErr == nil && track != nil {
			return []*music.Track{track}, nil
		}
	}

	if err != nil {
		return nil, err
	}
	return nil, errors.New("spotify_not_found")
}

func scrapeEmbed(resType, resID string) ([]*music.Track, error) {
	embedURL := fmt.Sprintf("https://open.spotify.com/embed/%s/%s", resType, resID)
	bodyBytes, err := doSpotifyRequest(http.MethodGet, embedURL)
	if err != nil {
		return nil, err
	}

	matches := nextDataRegex.FindSubmatch(bodyBytes)
	if len(matches) < 2 {
		return nil, errors.New("spotify embed __NEXT_DATA__ not found")
	}

	entity := gjson.GetBytes(matches[1], "props.pageProps.state.data.entity")
	if !entity.Exists() {
		return nil, errors.New("spotify entity not found in embed")
	}

	thumbnail := entity.Get("visualIdentity.image.0.url").String()
	if thumbnail == "" {
		thumbnail = entity.Get("coverArt.sources.0.url").String()
	}

	if resType == "track" {
		title := cleanText(entity.Get("title").String())
		if title == "" {
			title = cleanText(entity.Get("name").String())
		}
		if title == "" {
			return nil, errors.New("empty spotify track title")
		}

		var artists []string
		entity.Get("artists").ForEach(func(_, a gjson.Result) bool {
			if name := cleanText(a.Get("name").String()); name != "" {
				artists = append(artists, name)
			}
			return true
		})
		artistStr := strings.Join(artists, ", ")
		if artistStr == "" {
			artistStr = cleanText(entity.Get("subtitle").String())
		}

		durationMs := entity.Get("duration").Int()
		webURL := fmt.Sprintf("https://open.spotify.com/track/%s", resID)
		query := title
		if artistStr != "" {
			query = fmt.Sprintf("%s - %s audio", artistStr, title)
		}

		return []*music.Track{
			{
				ID:          resID,
				Title:       title,
				Artist:      artistStr,
				Duration:    time.Duration(durationMs) * time.Millisecond,
				Thumbnail:   thumbnail,
				WebURL:      webURL,
				StreamQuery: query,
				Source:      music.SourceSpotify,
			},
		}, nil
	}

	// Playlist or Album
	trackList := entity.Get("trackList")
	if !trackList.Exists() || !trackList.IsArray() {
		return nil, errors.New("spotify trackList not found")
	}

	var results []*music.Track
	trackList.ForEach(func(_, item gjson.Result) bool {
		title := cleanText(item.Get("title").String())
		if title == "" {
			title = cleanText(item.Get("name").String())
		}
		if title == "" {
			return true
		}

		artistStr := cleanText(item.Get("subtitle").String())
		durationMs := item.Get("duration").Int()
		uri := item.Get("uri").String()
		trackID := resID
		webURL := fmt.Sprintf("https://open.spotify.com/%s/%s", resType, resID)
		if strings.HasPrefix(uri, "spotify:track:") {
			trackID = strings.TrimPrefix(uri, "spotify:track:")
			webURL = fmt.Sprintf("https://open.spotify.com/track/%s", trackID)
		}

		query := title
		if artistStr != "" {
			query = fmt.Sprintf("%s - %s audio", artistStr, title)
		}

		results = append(results, &music.Track{
			ID:          trackID,
			Title:       title,
			Artist:      artistStr,
			Duration:    time.Duration(durationMs) * time.Millisecond,
			Thumbnail:   thumbnail,
			WebURL:      webURL,
			StreamQuery: query,
			Source:      music.SourceSpotify,
		})
		return true
	})

	if len(results) == 0 {
		return nil, errors.New("empty spotify playlist/album")
	}
	return results, nil
}

func scrapeTrackFallback(trackID string) (*music.Track, error) {
	trackURL := fmt.Sprintf("https://open.spotify.com/track/%s", trackID)

	// 1. Try page meta tags
	if bodyBytes, err := doSpotifyRequest(http.MethodGet, trackURL); err == nil {
		htmlStr := string(bodyBytes)
		var title, artist, thumb string
		if m := ogTitleRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
			title = cleanText(html.UnescapeString(m[1]))
		}
		if m := ogDescRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
			// Format is usually "Artist · Song · Year" or "Listen to X on Spotify. Artist · Song · Year."
			desc := cleanText(html.UnescapeString(m[1]))
			parts := strings.Split(desc, "·")
			if len(parts) > 0 {
				artist = strings.TrimSpace(parts[0])
			}
		}
		if m := ogImageRegex.FindStringSubmatch(htmlStr); len(m) > 1 {
			thumb = strings.TrimSpace(m[1])
		}
		if title != "" {
			query := title
			if artist != "" {
				query = fmt.Sprintf("%s - %s audio", artist, title)
			}
			return &music.Track{
				ID:          trackID,
				Title:       title,
				Artist:      artist,
				Thumbnail:   thumb,
				WebURL:      trackURL,
				StreamQuery: query,
				Source:      music.SourceSpotify,
			}, nil
		}
	}

	// 2. Try oEmbed endpoint
	oembedURL := fmt.Sprintf("https://open.spotify.com/oembed?url=%s", url.QueryEscape(trackURL))
	bodyBytes, err := doSpotifyRequest(http.MethodGet, oembedURL)
	if err != nil {
		return nil, err
	}
	title := cleanText(gjson.GetBytes(bodyBytes, "title").String())
	if title == "" {
		return nil, errors.New("empty spotify oembed title")
	}
	thumb := gjson.GetBytes(bodyBytes, "thumbnail_url").String()
	return &music.Track{
		ID:          trackID,
		Title:       title,
		Thumbnail:   thumb,
		WebURL:      trackURL,
		StreamQuery: title + " audio",
		Source:      music.SourceSpotify,
	}, nil
}

func cleanText(s string) string {
	// Replace non-breaking spaces (\u00a0) often used in Spotify subtitles
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}

func resolveRedirect(shortURL string) (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	if Proxy != "" {
		if proxyURL, err := url.Parse(Proxy); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	req, err := http.NewRequest(http.MethodGet, shortURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UserAgent)

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	return res.Request.URL.String(), nil
}

func doSpotifyRequest(method, rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	if Proxy != "" {
		if proxyURL, err := url.Parse(Proxy); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

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
