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

func GetAudioURL(youtubeURL string) (string, error) {
	// get audio url from yt-dlp with JSON output
	// Fall back to best format with audio if no audio-only stream available
	args := []string{
		"-f", "bestaudio[protocol=https]/bestaudio[protocol=http]/bestaudio/best[acodec!=none]/best",
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
		return "", err
	}

	jsonStr := string(output)
	// When using -f with --dump-json, yt-dlp returns the selected format directly
	audioURL := gjson.Get(jsonStr, "url").String()
	if audioURL == "" {
		return "", errors.New("YT-DLP: unable to find audio URL")
	}
	return audioURL, nil
}

func StartFFmpeg(videoURL string) (*exec.Cmd, io.ReadCloser, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "panic",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-i", videoURL,
		"-vn", // ignore video, audio only
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	return cmd, stdout, nil
}
