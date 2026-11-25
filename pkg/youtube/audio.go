package youtube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

const ytdlpAudioFormat = "bestaudio[ext=m4a]/bestaudio[ext=webm]/bestaudio/best[acodec!=none]/best"

// GetAudioStream resolves a YouTube audio stream using yt-dlp (-g -> direct FFmpeg)
// with automatic cookieless retry if cookies fail, and falls back to yt-dlp piped FFmpeg.
func GetAudioStream(youtubeURL string) (*PipelineCmd, io.ReadCloser, error) {
	if CookieFile != "" {
		if cmd, pipe, err := tryYtDlpDirectOrPiped(youtubeURL, true); err == nil {
			return cmd, pipe, nil
		} else if Debug {
			fmt.Printf("[Stream] yt-dlp with cookies failed (%v), retrying without cookies...\n", err)
		}
	}

	return tryYtDlpDirectOrPiped(youtubeURL, false)
}

func tryYtDlpDirectOrPiped(youtubeURL string, useCookies bool) (*PipelineCmd, io.ReadCloser, error) {
	directURL, err := resolveYtDlpURL(youtubeURL, useCookies)
	if err != nil {
		return nil, nil, err
	}

	if !strings.Contains(directURL, ".m3u8") {
		if Debug {
			fmt.Printf("[Stream] Using yt-dlp resolved direct FFmpeg stream (cookies=%v)\n", useCookies)
		}
		if cmd, pipe, ffErr := startDirectFFmpeg(directURL); ffErr == nil {
			return cmd, pipe, nil
		} else if Debug {
			fmt.Printf("[Stream] Direct FFmpeg failed (%v), falling back to piped stream...\n", ffErr)
		}
	}

	if Debug {
		fmt.Printf("[Stream] Using yt-dlp piped stream to FFmpeg (cookies=%v)\n", useCookies)
	}
	return startYtdlpPipedStream(youtubeURL, useCookies)
}

func buildYtDlpArgs(youtubeURL string, useCookies bool, extraArgs ...string) []string {
	args := []string{
		"-f", ytdlpAudioFormat,
		"--no-playlist",
		"--no-warnings",
	}
	if useCookies && CookieFile != "" {
		args = append(args, "--cookies", CookieFile, "--remote-components", "ejs:github")
	}
	if Proxy != "" {
		args = append(args, "--proxy", Proxy)
	}
	args = append(args, extraArgs...)
	args = append(args, youtubeURL)
	return args
}

func resolveYtDlpURL(youtubeURL string, useCookies bool) (string, error) {
	args := buildYtDlpArgs(youtubeURL, useCookies, "-g")
	cmd := exec.Command("yt-dlp", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("yt-dlp -g failed: %s", errMsg)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http") {
			return line, nil
		}
	}
	return "", errors.New("yt-dlp returned no stream URL")
}

func startYtdlpPipedStream(youtubeURL string, useCookies bool) (*PipelineCmd, io.ReadCloser, error) {
	ytArgs := buildYtDlpArgs(youtubeURL, useCookies, "-o", "-")

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

// StartAutoUpdater starts a background ticker that periodically updates yt-dlp every intervalHours.
func StartAutoUpdater(ctx context.Context, intervalHours int) {
	if intervalHours < 1 {
		intervalHours = 12
	}
	interval := time.Duration(intervalHours) * time.Hour

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updateCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				if err := UpdateYtDlp(updateCtx); err != nil {
					fmt.Printf("[yt-dlp] Auto-update failed: %v\n", err)
				} else if Debug {
					fmt.Println("[yt-dlp] Auto-update check completed")
				}
				cancel()
			}
		}
	}()
}

// UpdateYtDlp updates yt-dlp via pip3 (matching Dockerfile user-space install) or falls back to `yt-dlp -U`.
func UpdateYtDlp(ctx context.Context) error {
	if _, err := exec.LookPath("pip3"); err == nil {
		cmd := exec.CommandContext(ctx, "pip3", "install", "--no-cache-dir", "--user", "--upgrade", "--break-system-packages", "yt-dlp")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", "-U")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(out.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
