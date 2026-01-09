package youtube

import (
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/tidwall/gjson"
)

var Debug = os.Getenv("DEBUG") == "true"

func GetAudioURL(youtubeURL string) (string, error) {
	// get audio url from yt-dlp with JSON output
	infoCmd := exec.Command("yt-dlp", "-f", "bestaudio[ext=m4a]/bestaudio/best", "--dump-json", youtubeURL)
	output, err := infoCmd.Output()
	if err != nil {
		return "", errors.New("failed_to_get_audio_url")
	}

	jsonStr := string(output)
	// When using -f with --dump-json, yt-dlp returns the selected format directly
	audioURL := gjson.Get(jsonStr, "url").String()
	if audioURL == "" {
		return "", errors.New("audio_url_not_found")
	}
	return audioURL, nil
}

func StartFFmpeg(videoURL string) (*exec.Cmd, io.ReadCloser, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-re", // real-time pacing
		"-hide_banner",
		"-loglevel", "panic",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-i", videoURL,
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
