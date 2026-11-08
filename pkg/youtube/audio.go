package youtube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go-tube/pkg/cookies"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	Debug      = false
	UserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	CookieFile = ""
	Proxy      = ""
)

type dlsrvResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

// GetAudioStream resolves a YouTube audio stream using Tier 1 (dlsrv scraper -> direct FFmpeg)
// and automatically falls back to Tier 2 (yt-dlp -> direct/piped FFmpeg) if Tier 1 fails.
func GetAudioStream(youtubeURL string) (*PipelineCmd, io.ReadCloser, error) {
	// Tier 1: Fast HTTP scraper (dlsrv) -> Direct FFmpeg (minimal CPU & RAM)
	audioURL, err := resolveYouTubeAudioURL(youtubeURL)
	if err == nil && audioURL != "" {
		if Debug {
			fmt.Println("[Stream] Tier 1: Using dlsrv scraper-resolved FFmpeg stream")
		}
		cmd, pipe, ffErr := startDirectFFmpeg(audioURL)
		if ffErr == nil {
			return cmd, pipe, nil
		}
		if Debug {
			fmt.Printf("[Stream] Tier 1 FFmpeg failed (%v), falling back to yt-dlp...\n", ffErr)
		}
	} else if Debug {
		fmt.Printf("[Stream] Tier 1 dlsrv failed (%v), falling back to yt-dlp...\n", err)
	}

	// Tier 2: Fallback to yt-dlp
	return startYtDlpFallback(youtubeURL)
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
	client := &http.Client{Timeout: 25 * time.Second}
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

// startYtDlpFallback extracts the direct audio URL via `yt-dlp -g` so yt-dlp exits immediately
// (saving RAM/CPU), or uses piped streaming if the format is an HLS (.m3u8) manifest.
func startYtDlpFallback(youtubeURL string) (*PipelineCmd, io.ReadCloser, error) {
	args := []string{
		"-f", "bestaudio[ext=m4a]/bestaudio[ext=webm]/bestaudio/best[acodec!=none]/best",
		"--no-playlist",
		"--no-warnings",
		"-g",
	}
	if CookieFile != "" {
		args = append(args, "--cookies", CookieFile)
	}
	if Proxy != "" {
		args = append(args, "--proxy", Proxy)
	}
	args = append(args, youtubeURL)

	infoCmd := exec.Command("yt-dlp", args...)
	output, err := infoCmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "http") {
			directURL := strings.TrimSpace(lines[0])
			if !strings.Contains(directURL, ".m3u8") {
				if Debug {
					fmt.Println("[Stream] Tier 2: Using yt-dlp resolved direct FFmpeg stream")
				}
				return startDirectFFmpeg(directURL)
			}
		}
	}

	if Debug {
		fmt.Println("[Stream] Tier 2: Using yt-dlp piped stream to FFmpeg")
	}
	return startYtdlpPipedStream(youtubeURL)
}

func startYtdlpPipedStream(youtubeURL string) (*PipelineCmd, io.ReadCloser, error) {
	ytArgs := []string{
		"-f", "bestaudio[ext=m4a]/bestaudio[ext=webm]/bestaudio/best[acodec!=none]/best",
		"--no-playlist",
		"--no-warnings",
		"-o", "-",
	}
	if CookieFile != "" {
		ytArgs = append(ytArgs, "--cookies", CookieFile)
	}
	if Proxy != "" {
		ytArgs = append(ytArgs, "--proxy", Proxy)
	}
	ytArgs = append(ytArgs, youtubeURL)

	ffArgs := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-vn",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"pipe:1",
	}

	ytdlp := exec.Command("yt-dlp", ytArgs...)
	ffmpeg := exec.Command("ffmpeg", ffArgs...)

	ytdlpOut, err := ytdlp.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("yt-dlp stdout pipe: %w", err)
	}
	ffmpeg.Stdin = ytdlpOut

	ffmpegOut, err := ffmpeg.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	if err := ytdlp.Start(); err != nil {
		return nil, nil, fmt.Errorf("yt-dlp start: %w", err)
	}
	if err := ffmpeg.Start(); err != nil {
		_ = ytdlp.Process.Kill()
		_ = ytdlp.Wait()
		return nil, nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	wrapper := &PipelineCmd{
		ffmpeg: ffmpeg,
		ytdlp:  ytdlp,
	}
	return wrapper, ffmpegOut, nil
}

// startDirectFFmpeg starts FFmpeg for direct audio URLs (mp3, m4a, webm, etc.)
func startDirectFFmpeg(audioURL string) (*PipelineCmd, io.ReadCloser, error) {
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
		return nil, nil, err
	}

	if Proxy != "" {
		cmd.Env = append(cmd.Env, "http_proxy="+Proxy, "https_proxy="+Proxy)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	wrapper := &PipelineCmd{
		ffmpeg: cmd,
	}
	return wrapper, stdout, nil
}

// PipelineCmd wraps FFmpeg (and optional yt-dlp) subprocesses for safe, zombie-free cleanup.
type PipelineCmd struct {
	once   sync.Once
	ffmpeg *exec.Cmd
	ytdlp  *exec.Cmd
}

// Kill terminates running subprocesses and reaps them via Wait() to prevent zombie processes.
func (p *PipelineCmd) Kill() error {
	if p == nil {
		return nil
	}
	p.once.Do(func() {
		if p.ytdlp != nil && p.ytdlp.Process != nil {
			_ = p.ytdlp.Process.Kill()
			go func(c *exec.Cmd) { _ = c.Wait() }(p.ytdlp)
		}
		if p.ffmpeg != nil && p.ffmpeg.Process != nil {
			_ = p.ffmpeg.Process.Kill()
			go func(c *exec.Cmd) { _ = c.Wait() }(p.ffmpeg)
		}
	})
	return nil
}
