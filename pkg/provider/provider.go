package provider

import (
	"errors"
	"go-tube/pkg/music"
	"go-tube/pkg/spotify"
	"go-tube/pkg/youtube"
	"strings"
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
