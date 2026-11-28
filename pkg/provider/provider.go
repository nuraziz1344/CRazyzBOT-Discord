package provider

import (
	"errors"
	"go-tube/pkg/music"
	"go-tube/pkg/spotify"
	"go-tube/pkg/youtube"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Configure sets global scraper options (debug, cookies.txt path, proxy) across providers.
func Configure(debug bool, cookieFile, proxy string) {
	youtube.Debug = debug
	youtube.CookieFile = cookieFile
	youtube.Proxy = proxy

	spotify.CookieFile = cookieFile
	spotify.Proxy = proxy
}

// ResolveInput resolves a keyword, single track URL, or playlist/album URL from
// YouTube, YouTube Music, or Spotify into one or more *music.Track items.
func ResolveInput(input string, requestedBy string) ([]*music.Track, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, errors.New("empty_input")
	}

	if err := music.ValidateURL(input); err != nil {
		return nil, err
	}

	var (
		tracks []*music.Track
		err    error
	)

	switch music.ClassifyInput(input) {
	case music.InputSpotify:
		tracks, err = spotify.Resolve(input)
	case music.InputYouTubePlaylist, music.InputYouTubeMusicPlaylist:
		tracks, err = youtube.ParsePlaylist(input)
	case music.InputYouTubeVideo, music.InputYouTubeMusicVideo:
		var t *music.Track
		t, err = youtube.GetVideoInfo(input)
		if err == nil && t != nil {
			tracks = []*music.Track{t}
		}
	default:
		tracks, err = youtube.SearchTracks(input, 1, false)
	}

	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, errors.New("no_tracks_found")
	}

	for _, t := range tracks {
		t.RequestedBy = requestedBy
	}
	return tracks, nil
}

// Search queries YouTube/YouTube Music for up to `limit` tracks for interactive selection.
func Search(keyword string, limit int, requestedBy string) ([]*music.Track, error) {
	tracks, err := youtube.SearchTracks(keyword, limit, false)
	if err != nil {
		return nil, err
	}
	for _, t := range tracks {
		t.RequestedBy = requestedBy
	}
	return tracks, nil
}

// EnsureYouTubeURL lazily resolves a track's YouTubeURL (e.g. for Spotify tracks) just before playback.
func EnsureYouTubeURL(track *music.Track) error {
	if track == nil {
		return errors.New("nil track")
	}
	if track.YouTubeURL != "" {
		return nil
	}

	query := strings.TrimSpace(track.StreamQuery)
	if query == "" {
		query = track.DisplayTitle() + " audio"
	}

	results, err := youtube.SearchTracks(query, 1, true)
	if err != nil || len(results) == 0 {
		return errors.New("failed to resolve YouTube stream for track")
	}

	track.YouTubeURL = results[0].YouTubeURL
	if track.Duration == 0 && results[0].Duration > 0 {
		track.Duration = results[0].Duration
	}
	if track.Thumbnail == "" && results[0].Thumbnail != "" {
		track.Thumbnail = results[0].Thumbnail
	}
	return nil
}

var titleNoiseRegex = regexp.MustCompile(`(?i)[\(\[][^)\]]*(official(\s+music)?\s+(video|audio)|lyric(\s+video)?|lyrics|visualizer|remastered|\b4k\b|\bhd\b|\baudio\b)[^)\]]*[\)\]]`)

// NormalizeTitleKey produces a canonical key for deduplicating tracks across YouTube,
// YouTube Music, and Spotify variants.
func NormalizeTitleKey(artist, title string) string {
	normTitle := cleanTitleComponent(titleNoiseRegex.ReplaceAllString(title, " "))
	normArtist := cleanTitleComponent(artist)

	if normArtist == "" {
		return normTitle
	}
	if normTitle == "" {
		return normArtist
	}
	if normTitle == normArtist || strings.HasPrefix(normTitle, normArtist+" ") {
		return normTitle
	}
	return normArtist + " " + normTitle
}

func cleanTitleComponent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
		} else if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// WarmRelatedTracks pre-warms the related-tracks cache for a seed track without mutating it.
func WarmRelatedTracks(seed *music.Track) {
	if seed == nil {
		return
	}
	cp := seed.Clone()
	vID := resolveSeedVideoID(cp)
	if vID != "" {
		_, _ = youtube.GetRelatedTracks(vID)
	}
}

// ResolveRelatedTrack finds a related track for autoplay, excluding previously played IDs and titles.
func ResolveRelatedTrack(seed *music.Track, excludeIDs map[string]bool, excludeTitles map[string]bool) (*music.Track, error) {
	if seed == nil {
		return nil, errors.New("nil seed track")
	}
	cp := seed.Clone()
	vID := resolveSeedVideoID(cp)

	var (
		candidates []*music.Track
		err        error
	)
	if vID != "" {
		candidates, err = youtube.GetRelatedTracks(vID)
	}
	if err != nil || len(candidates) == 0 {
		query := strings.TrimSpace(cp.Artist)
		if query != "" {
			query += " mix"
		} else if title := strings.TrimSpace(cp.Title); title != "" {
			query = title + " mix"
		}
		if query != "" {
			candidates, err = youtube.SearchTracks(query, 10, true)
		}
	}
	if len(candidates) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("no related tracks found")
	}

	seedNorm := NormalizeTitleKey(cp.Artist, cp.Title)
	chosen := selectRelatedCandidate(candidates, vID, seedNorm, excludeIDs, excludeTitles)
	if chosen == nil {
		return nil, errors.New("no suitable related track found")
	}

	res := chosen.Clone()
	res.Autoplay = true
	res.RequestedBy = ""
	return res, nil
}

func resolveSeedVideoID(cp *music.Track) string {
	vID := youtube.ExtractVideoID(cp.YouTubeURL)
	if vID == "" && cp.Source != music.SourceSpotify {
		vID = strings.TrimSpace(cp.ID)
	}
	if vID == "" {
		if err := EnsureYouTubeURL(cp); err == nil {
			vID = youtube.ExtractVideoID(cp.YouTubeURL)
		}
	}
	return vID
}

func selectRelatedCandidate(candidates []*music.Track, seedID, seedNorm string, excludeIDs, excludeTitles map[string]bool) *music.Track {
	matchesSeed := func(c *music.Track, norm string) bool {
		if c.ID == seedID && seedID != "" {
			return true
		}
		return seedNorm != "" && norm == seedNorm
	}

	// Pass 1: strict exclusions + duration <= 15m
	for _, c := range candidates {
		if c == nil || c.ID == "" {
			continue
		}
		norm := NormalizeTitleKey(c.Artist, c.Title)
		if matchesSeed(c, norm) || excludeIDs[c.ID] || (norm != "" && excludeTitles[norm]) {
			continue
		}
		if c.Duration > 0 && c.Duration <= 15*time.Minute {
			return c
		}
	}

	// Pass 2: strict exclusions without duration cap
	for _, c := range candidates {
		if c == nil || c.ID == "" {
			continue
		}
		norm := NormalizeTitleKey(c.Artist, c.Title)
		if matchesSeed(c, norm) || excludeIDs[c.ID] || (norm != "" && excludeTitles[norm]) {
			continue
		}
		return c
	}

	// Pass 3: fallback if session history exhausted every candidate
	for _, c := range candidates {
		if c == nil || c.ID == "" {
			continue
		}
		norm := NormalizeTitleKey(c.Artist, c.Title)
		if matchesSeed(c, norm) {
			continue
		}
		return c
	}

	return nil
}
