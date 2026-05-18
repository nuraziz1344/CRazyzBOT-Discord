package youtube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	Debug      = false
	UserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	CookieFile = ""
	Proxy      = ""
)

type dlsrvResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

// GetAudioStream resolves a YouTube audio URL through a scraper endpoint and starts FFmpeg.
// Returns the FFmpeg command and stdout pipe for reading PCM audio.
func GetAudioStream(youtubeURL string) (*PipelineCmd, io.ReadCloser, error) {
	audioURL, err := resolveYouTubeAudioURL(youtubeURL)
	if err != nil {
		return nil, nil, err
	}

	if Debug {
		fmt.Println("Using scraper-resolved FFmpeg stream")
	}
	return startDirectFFmpeg(audioURL)
}

func resolveYouTubeAudioURL(youtubeURL string) (string, error) {
	videoID := extractYouTubeVideoID(youtubeURL)
	if videoID == "" {
		return "", errors.New("invalid YouTube URL")
	}

	requestBody, _ := json.Marshal(map[string]string{
		"videoId": videoID,
		"format":  "mp3",
		"quality": "128",
	})
	resolved, err := resolveDlsrv("https://embed.dlsrv.online/api/download/mp3", requestBody, videoID)
	if err != nil {
		return "", fmt.Errorf("YouTube download link failed: %w", err)
	}
	return resolved.URL, nil
}

func resolveDlsrv(endpoint string, body []byte, videoID string) (*dlsrvResponse, error) {
	resBody, err := doScraperRequest(http.MethodPost, endpoint, bytes.NewReader(body), map[string]string{
		"Accept":       "*/*",
		"Content-Type": "application/json",
		"Referer":      "https://embed.dlsrv.online/v1/full?videoId=" + videoID,
	})
	if err != nil {
		return nil, err
	}

	var resolved dlsrvResponse
	if err := json.Unmarshal(resBody, &resolved); err != nil {
		return nil, err
	}
	if resolved.URL == "" {
		return nil, errors.New("empty download URL")
	}
	return &resolved, nil
}

func doScraperRequest(method string, rawURL string, body io.Reader, headers map[string]string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	if Proxy != "" {
		proxyURL, err := url.Parse(Proxy)
		if err != nil {
			return nil, err
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	for key, value := range headers {
		if value != "" {
			req.Header.Set(key, value)
		}
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

func extractYouTubeVideoID(rawURL string) string {
	patterns := []string{
		`(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]{11})`,
		`youtube\.com/embed/([a-zA-Z0-9_-]{11})`,
		`youtube\.com/v/([a-zA-Z0-9_-]{11})`,
		`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(rawURL); len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

// startDirectFFmpeg starts FFmpeg for direct audio URLs (m4a, webm, etc.)
func startDirectFFmpeg(audioURL string) (*PipelineCmd, io.ReadCloser, error) {
	if Debug {
		fmt.Println("FFMPEG: ", audioURL)
	}

	args := []string{
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

	cmd := exec.Command("ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if Debug {
			fmt.Println("FFMPEG stdout: ", err.Error())
		}
		return nil, nil, err
	}

	if Proxy != "" {
		cmd.Env = append(cmd.Env, "http_proxy="+Proxy, "https_proxy="+Proxy)
	}

	if Debug {
		fmt.Println("FFMPEG command: ", strings.Join(cmd.Env, " "), strings.Join(cmd.Args, " "))
	}

	if err := cmd.Start(); err != nil {
		if Debug {
			fmt.Println("FFMPEG start: ", err.Error())
		}
		return nil, nil, err
	}

	if Debug {
		fmt.Println("FFMPEG started")
	}

	wrapper := &PipelineCmd{
		ffmpeg: cmd,
	}
	return wrapper, stdout, nil
}

// PipelineCmd wraps FFmpeg for cleanup.
type PipelineCmd struct {
	ffmpeg *exec.Cmd
}

// Kill terminates the running process.
func (p *PipelineCmd) Kill() error {
	if p.ffmpeg != nil && p.ffmpeg.Process != nil {
		p.ffmpeg.Process.Kill()
	}
	return nil
}
