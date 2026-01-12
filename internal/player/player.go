package player

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"go-tube/pkg/youtube"
	"time"

	"gopkg.in/hraban/opus.v2"
)

const (
	sampleRate = 48000
	channels   = 2
	frameSize  = 960 // 20ms @ 48kHz
)

var Debug = false

func StartPlayer(gid string) {
	Channels[gid].isRunning = len(Channels[gid].queue) > 0

	// Join voice channel if not already connected
	if Channels[gid].dgv == nil {
		if Debug {
			fmt.Println("Joining voice channel")
		}

		err := JoinVoice(gid, Channels[gid].userID, Channels[gid].session)
		if err != nil {
			fmt.Printf("Error joining voice channel: %s\n", err.Error())
			Channels[gid].isRunning = false
			delete(Channels, gid)
			return
		}
	}

	for Channels[gid].isRunning && len(Channels[gid].queue) > 0 {
		queueItem := Channels[gid].queue[0]

		// Verify voice connection is still ready
		if !Channels[gid].dgv.Ready {
			fmt.Printf("Voice connection not ready, waiting...\n")
			for !Channels[gid].dgv.Ready {
				time.Sleep(100 * time.Millisecond)
			}
		}

		if Debug {
			fmt.Printf("Playing: %s\n", queueItem.Title)
		}

		audioUrl, err := youtube.GetAudioURL(queueItem.YouTubeURL)
		if err != nil {
			fmt.Printf("Error getting audio URL: %s\n", err.Error())
			if Debug {
				fmt.Println(queueItem.YouTubeURL)
			}

			Channels[gid].queue = Channels[gid].queue[1:]
			continue
		}

		cmd, pcm, err := youtube.StartFFmpeg(audioUrl)
		if err != nil {
			fmt.Printf("Error starting ffmpeg: %s\n", err.Error())
			fmt.Println(audioUrl)
			Channels[gid].queue = Channels[gid].queue[1:]
			continue
		}

		Channels[gid].currentCmd = cmd
		defer func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()

		encoder, err := opus.NewEncoder(sampleRate, channels, opus.Application(opus.AppAudio))
		if err != nil {
			fmt.Printf("Error creating opus encoder: %s\n", err.Error())
			Channels[gid].queue = Channels[gid].queue[1:]
			continue
		}

		reader := bufio.NewReader(pcm)
		pcmBuf := make([]int16, frameSize*channels)
		opusBuf := make([]byte, 4000)

		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()

		startTime := time.Now()

		for range ticker.C {
			if err := binary.Read(reader, binary.LittleEndian, pcmBuf); err != nil {
				elapsed := time.Since(startTime)
				if Debug && elapsed < 10*time.Second {
					fmt.Printf("Player finished too fast (%v) - possible stream error\n", elapsed)
					fmt.Println(audioUrl)
				} else if Debug {
					fmt.Println("Player finished")
				}
				
				// Clean up and move to next item
				if len(Channels[gid].queue) > 0 {
					Channels[gid].queue = Channels[gid].queue[1:]
				}
				if len(Channels[gid].queue) == 0 {
					Channels[gid].isRunning = false
					Channels[gid].dgv.Disconnect()
				}
				break
			}

			n, err := encoder.Encode(pcmBuf, opusBuf)
			if err != nil {
				continue
			}

			select {
			case Channels[gid].dgv.OpusSend <- opusBuf[:n]:
			default:
				if Debug {
					fmt.Println("Player is congested, dropping frame")
				}
				continue
			}
		}
	}
	delete(Channels, gid)
}

// Kill method for exec.Cmd to satisfy interface
func (c *Channel) Kill() error {
	if c.currentCmd != nil && c.currentCmd.Process != nil {
		return c.currentCmd.Process.Kill()
	}
	return nil
}
