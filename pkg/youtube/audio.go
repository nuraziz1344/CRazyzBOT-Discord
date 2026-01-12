package youtube

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/tidwall/gjson"
)

var Debug = false
var CookieFile = ""
var UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

// GetAudioStream fetches the audio URL from yt-dlp and starts FFmpeg to stream it.
// Returns the FFmpeg command and stdout pipe for reading PCM audio.
func GetAudioStream(youtubeURL string) (*PipelineCmd, io.ReadCloser, error) {
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
	isHLS := strings.Contains(audioURL, ".m3u8")
	if isHLS {
		if Debug {
			fmt.Println("Detected m3u8 stream, using yt-dlp piped download")
		}
		// For HLS, pipe yt-dlp output directly to FFmpeg
		return startYtdlpPipedStream(youtubeURL)
	}

	// For direct URLs, use FFmpeg directly
	if Debug {
		fmt.Println("Using direct FFmpeg stream")
	}
	return startDirectFFmpeg(audioURL)
}

// startYtdlpPipedStream uses yt-dlp to download and pipe to FFmpeg for m3u8 streams.
// This handles cookies and authentication properly.
func startYtdlpPipedStream(youtubeURL string) (*PipelineCmd, io.ReadCloser, error) {
	// Build yt-dlp args to output audio to stdout
	ytArgs := []string{
		"-f", "bestaudio/best[acodec!=none]",
		"--no-warnings",
		"--geo-bypass",
		"-o", "-", // output to stdout
	}
	if CookieFile != "" {
		ytArgs = append(ytArgs, "--cookies", CookieFile)
	}
	ytArgs = append(ytArgs, youtubeURL)

	// FFmpeg reads from stdin and outputs PCM
	ffArgs := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0", // read from stdin
		"-vn",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"pipe:1", // output to stdout
	}

	ytdlp := exec.Command("yt-dlp", ytArgs...)
	ffmpeg := exec.Command("ffmpeg", ffArgs...)

	// Pipe yt-dlp stdout to FFmpeg stdin
	ytdlpOut, err := ytdlp.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("yt-dlp stdout pipe: %w", err)
	}
	ffmpeg.Stdin = ytdlpOut

	// Get FFmpeg stdout for PCM output
	ffmpegOut, err := ffmpeg.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	// Start both processes
	if err := ytdlp.Start(); err != nil {
		return nil, nil, fmt.Errorf("yt-dlp start: %w", err)
	}
	if err := ffmpeg.Start(); err != nil {
		ytdlp.Process.Kill()
		return nil, nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	// Create a wrapper that kills both processes
	wrapper := &PipelineCmd{
		ffmpeg: ffmpeg,
		ytdlp:  ytdlp,
	}

	return wrapper, ffmpegOut, nil
}

// startDirectFFmpeg starts FFmpeg for direct audio URLs (m4a, webm, etc.)
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

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	wrapper := &PipelineCmd{
		ffmpeg: cmd,
	}
	return wrapper, stdout, nil
}

// PipelineCmd wraps both yt-dlp and ffmpeg processes for cleanup
type PipelineCmd struct {
	ffmpeg *exec.Cmd
	ytdlp  *exec.Cmd
}

// Kill terminates both processes in the pipeline
func (p *PipelineCmd) Kill() error {
	if p.ytdlp != nil && p.ytdlp.Process != nil {
		p.ytdlp.Process.Kill()
	}
	if p.ffmpeg != nil && p.ffmpeg.Process != nil {
		p.ffmpeg.Process.Kill()
	}
	return nil
}
