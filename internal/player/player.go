package player

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"go-tube/pkg/music"
	"go-tube/pkg/provider"
	"go-tube/pkg/youtube"
	"io"
	"time"

	"gopkg.in/hraban/opus.v2"
)

const (
	sampleRate    = 48000
	channels      = 2
	frameSize     = 960 // 20ms @ 48kHz
	framesPerSec  = 50  // 1000ms / 20ms = 50 frames per second
	defaultBufSec = 30

	prebufferFrames  = 25 // ~500ms warmup before starting playback
	pipeReaderBuffer = 64 * 1024
)

var (
	Debug         = false
	BufferSeconds = defaultBufSec
)

// SetBufferSeconds sets the read-ahead Opus buffer duration in seconds (clamped between 2s and 300s).
func SetBufferSeconds(seconds int) {
	if seconds < 2 {
		seconds = 2
	} else if seconds > 300 {
		seconds = 300
	}
	BufferSeconds = seconds
}

func (gp *GuildPlayer) runPlayerLoop() {
	if err := gp.JoinVoice(); err != nil {
		if Debug {
			fmt.Printf("[Player] Error joining voice channel in guild %s: %v\n", gp.GuildID, err)
		}
		ClearAndLeave(gp.GuildID, "Gagal bergabung ke voice channel.")
		return
	}

	for {
		gp.mu.Lock()
		running := gp.isRunning
		gp.mu.Unlock()
		if !running {
			return
		}

		currentTrack := gp.Queue.Current()
		if currentTrack == nil {
			break
		}

		// Trigger background prefetch for the next track in queue
		go gp.prefetchNextTrack()

		skipped, forceRemove := gp.playSingleTrack(currentTrack)

		gp.mu.Lock()
		stillRunning := gp.isRunning
		gp.mu.Unlock()
		if !stillRunning {
			return
		}

		next := gp.Queue.Advance(skipped, forceRemove)
		if next == nil {
			break
		}
	}

	// Queue finished naturally -> auto-leave voice channel
	ClearAndLeave(gp.GuildID, "")
}

func (gp *GuildPlayer) prefetchNextTrack() {
	next := gp.Queue.PeekNext()
	if next == nil || next.YouTubeURL != "" {
		return
	}
	if Debug {
		fmt.Printf("[Prefetch] Resolving next track in background: %s\n", next.DisplayTitle())
	}
	_ = provider.EnsureYouTubeURL(next)
}

func (gp *GuildPlayer) playSingleTrack(track *music.Track) (skipped bool, forceRemove bool) {
	ctx, cancel := context.WithCancel(context.Background())

	gp.mu.Lock()
	gp.currentCancel = cancel
	gp.skipRequested = false
	gp.forceRemoveCurrent = false
	gp.isPaused = false
	s := gp.session
	gp.mu.Unlock()

	defer func() {
		cancel()
		gp.mu.Lock()
		if gp.currentCmd != nil {
			_ = gp.currentCmd.Kill()
			gp.currentCmd = nil
		}
		gp.currentCancel = nil
		skipped = gp.skipRequested
		forceRemove = gp.forceRemoveCurrent
		gp.mu.Unlock()
	}()

	// Lazily resolve YouTubeURL if this track came from Spotify
	if track.YouTubeURL == "" {
		if Debug {
			fmt.Printf("[Player] Resolving YouTube stream URL for: %s\n", track.DisplayTitle())
		}
		if err := provider.EnsureYouTubeURL(track); err != nil {
			fmt.Printf("[Player] Failed to resolve track %s: %v\n", track.DisplayTitle(), err)
			return false, false
		}
	}

	if ctx.Err() != nil {
		return true, false
	}

	cmd, pcm, err := youtube.GetAudioStream(track.YouTubeURL)
	if err != nil {
		fmt.Printf("[Player] Error getting audio stream for %s (%s): %v\n", track.DisplayTitle(), track.YouTubeURL, err)
		return false, false
	}

	encoder, err := opus.NewEncoder(sampleRate, channels, opus.Application(opus.AppAudio))
	if err != nil {
		fmt.Printf("[Player] Error creating opus encoder: %v\n", err)
		_ = cmd.Kill()
		return false, false
	}

	gp.mu.Lock()
	gp.currentCmd = cmd
	dgv := gp.dgv
	gp.mu.Unlock()

	if dgv == nil {
		_ = cmd.Kill()
		return false, false
	}

	// Notify UI that a new track is now playing
	if OnPlayerStateChange != nil && s != nil {
		OnPlayerStateChange(s, gp, EventTrackStart, "")
	}

	// Configurable Read-Ahead Opus Frame Buffer (default 30s = 1500 frames @ 20ms)
	bufFrames := BufferSeconds * framesPerSec
	if bufFrames < prebufferFrames*2 {
		bufFrames = prebufferFrames * 2
	}
	frameChan := make(chan []byte, bufFrames)
	prebuffered := make(chan struct{})

	// Producer Goroutine: reads PCM from FFmpeg ahead of time, encodes to independent Opus packets,
	// and buffers up to 30 seconds in memory (~450 KB).
	go func() {
		defer close(frameChan)
		defer pcm.Close()

		reader := bufio.NewReaderSize(pcm, pipeReaderBuffer)
		pcmBuf := make([]int16, frameSize*channels)
		encScratch := make([]byte, 1275) // Max valid Opus packet size
		count := 0
		signaledPrebuffer := false

		for {
			select {
			case <-ctx.Done():
				if !signaledPrebuffer {
					close(prebuffered)
				}
				return
			default:
			}

			if err := binary.Read(reader, binary.LittleEndian, pcmBuf); err != nil {
				if err != io.EOF && Debug {
					fmt.Printf("[Player] PCM stream ended: %v\n", err)
				}
				if !signaledPrebuffer {
					close(prebuffered)
				}
				return
			}

			n, err := encoder.Encode(pcmBuf, encScratch)
			if err != nil || n <= 0 {
				continue
			}

			// Allocate an exact-sized copy per Opus frame to eliminate slice aliasing data races
			frame := make([]byte, n)
			copy(frame, encScratch[:n])

			select {
			case frameChan <- frame:
				count++
				if !signaledPrebuffer && count >= prebufferFrames {
					signaledPrebuffer = true
					close(prebuffered)
				}
			case <-ctx.Done():
				if !signaledPrebuffer {
					close(prebuffered)
				}
				return
			}
		}
	}()

	// Wait briefly for initial pre-buffer (~500ms) or stream completion/cancellation
	select {
	case <-prebuffered:
	case <-ctx.Done():
		return true, false
	case <-time.After(3 * time.Second):
	}

	_ = dgv.Speaking(true)
	defer func() {
		_ = dgv.Speaking(false)
	}()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	wasPaused := false

	// Consumer Loop: paces pre-encoded Opus frames every 20ms without blocking on network/FFmpeg I/O
	for {
		select {
		case <-ctx.Done():
			return true, false

		case <-ticker.C:
			gp.mu.Lock()
			paused := gp.isPaused
			activeDGV := gp.dgv
			gp.mu.Unlock()

			if activeDGV == nil {
				return false, false
			}

			if paused {
				if !wasPaused {
					_ = activeDGV.Speaking(false)
					wasPaused = true
				}
				continue
			}
			if wasPaused {
				_ = activeDGV.Speaking(true)
				wasPaused = false
			}

			frame, ok := <-frameChan
			if !ok {
				// Stream finished all buffered frames
				return false, false
			}

			select {
			case activeDGV.OpusSend <- frame:
			case <-ctx.Done():
				return true, false
			case <-time.After(60 * time.Millisecond):
				if Debug {
					fmt.Println("[Player] Voice OpusSend congested, dropping frame")
				}
			}
		}
	}
}
