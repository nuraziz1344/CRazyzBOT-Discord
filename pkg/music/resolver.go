package music

import (
	"errors"
	"net/url"
	"strings"
)

type InputKind int

const (
	InputKeyword InputKind = iota
	InputYouTubeVideo
	InputYouTubePlaylist
	InputYouTubeMusicVideo
	InputYouTubeMusicPlaylist
	InputSpotify
)

// ClassifyInput determines whether a user input string is a keyword or a specific URL type.
func ClassifyInput(input string) InputKind {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		return InputKeyword
	}

	u, err := url.ParseRequestURI(input)
	if err != nil {
		return InputKeyword
	}

	host := strings.ToLower(u.Hostname())
	switch host {
	case "open.spotify.com", "play.spotify.com", "spotify.link", "spoti.fi":
		return InputSpotify
	case "music.youtube.com":
		if u.Path == "/playlist" || (u.Query().Get("list") != "" && u.Query().Get("v") == "") {
			return InputYouTubeMusicPlaylist
		}
		return InputYouTubeMusicVideo
	case "youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be":
		if u.Path == "/playlist" || (u.Query().Get("list") != "" && u.Query().Get("v") == "") {
			return InputYouTubePlaylist
		}
		return InputYouTubeVideo
	default:
		return InputKeyword
	}
}

// ValidateURL checks if an input that starts with http(s) is a supported music URL.
func ValidateURL(input string) error {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		if ClassifyInput(input) == InputKeyword {
			return errors.New("unsupported_url")
		}
	}
	return nil
}
