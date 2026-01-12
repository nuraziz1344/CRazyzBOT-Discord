package youtube

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/tidwall/gjson"
)

var Debug = false
var CookieFile = ""
var UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

// GetAudioStream fetches the audio URL from yt-dlp and starts FFmpeg to stream it.
// Returns the FFmpeg command and stdout pipe for reading PCM audio.
func GetAudioStream(youtubeURL string) (*exec.Cmd, io.ReadCloser, error) {
	// get audio url from yt-dlp with JSON output
	// Fall back to best format with audio if no audio-only stream available
	args := []string{
		"-f", "bestaudio/best[acodec!=none]",
		"--dump-json",
		"--no-warnings",
		"--geo-bypass",
	}
	if CookieFile != "" {
		if Debug {
			fmt.Printf("Using cookie file: %s\n", CookieFile)
		}
		args = append(args, "--cookies", CookieFile)
	}
	args = append(args, youtubeURL)
	infoCmd := exec.Command("yt-dlp", args...)
	output, err := infoCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Printf("YT-DLP error: %s\n", string(exitErr.Stderr))
		}
		if Debug {
			fmt.Printf("Command: yt-dlp %v\n", args)
		}
		return nil, nil, err
	}

	jsonStr := string(output)
	// When using -f with --dump-json, yt-dlp returns the selected format directly
	audioURL := gjson.Get(jsonStr, "url").String()
	if audioURL == "" {
		return nil, nil, errors.New("YT-DLP: unable to find audio URL")
	}

	// Check if URL is m3u8 (HLS stream)
	isHLS := len(audioURL) >= 5 && audioURL[len(audioURL)-5:] == ".m3u8"
	if Debug && isHLS {
		fmt.Println("Detected m3u8 stream, using HLS-specific FFmpeg options")
	}

	// Start FFmpeg with appropriate options
	cmd := startFFmpegForURL(audioURL, youtubeURL, isHLS)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	return cmd, stdout, nil
}

// startFFmpegForURL creates an FFmpeg command with options appropriate for the URL type.
func startFFmpegForURL(audioURL, referer string, isHLS bool) *exec.Cmd {
	var args []string

	if isHLS {
		// HLS-specific options for m3u8 streams
		args = []string{
			"-hide_banner",
			"-loglevel", "error",
			"-protocol_whitelist", "file,http,https,tcp,tls,crypto,data",
			"-user_agent", UserAgent,
			"-headers", fmt.Sprintf("Referer: %s\r\nOrigin: https://www.youtube.com\r\n", referer),
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_delay_max", "5",
			"-reconnect_on_network_error", "1",
			"-multiple_requests", "1",
			"-i", audioURL,
			"-vn",
			"-f", "s16le",
			"-ar", "48000",
			"-ac", "2",
			"pipe:1",
		}
	} else {
		// Standard options for direct URLs (m4a, webm, mp4, etc.)
		args = []string{
			"-hide_banner",
			"-loglevel", "error",
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_delay_max", "5",
			"-i", audioURL,
			"-vn",
			"-f", "s16le",
			"-ar", "48000",
			"-ac", "2",
			"pipe:1",
		}
	}

	return exec.Command("ffmpeg", args...)
}
