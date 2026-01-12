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

func GetAudioURL(youtubeURL string) (string, error) {
	// get audio url from yt-dlp with JSON output
	// Fall back to best format with audio if no audio-only stream available
	args := []string{
		"-f", "bestaudio/best[acodec!=none]/91/92/93/best",
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

	// if audioUrl ends with ".m3u8"
	if Debug && len(audioURL) >= 5 && audioURL[len(audioURL)-5:] == ".m3u8" {
		fmt.Println("Warning: audio URL is an m3u8 stream, which may not be supported")
	}
	
	return audioURL, nil
}

func StartFFmpeg(videoURL string, referer string) (*exec.Cmd, io.ReadCloser, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-user_agent", UserAgent,
		"-headers", fmt.Sprintf("Referer: %s\r\nOrigin: https://www.youtube.com", referer),
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-i", videoURL,
		"-vn",
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
