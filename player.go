package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"go-tube/youtube"
	"io"
	"os/exec"
	"time"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/hraban/opus.v2"
)

type Channel struct {
	ChannelID  string
	dgv        *discordgo.VoiceConnection
	queue      []*youtube.VideoInfo
	isRunning  bool
	session    *discordgo.Session
	userID     string
	currentCmd *exec.Cmd
}

const (
	sampleRate = 48000
	channels   = 2
	frameSize  = 960 // 20ms @ 48kHz
)

var Channels = make(map[string]*Channel)

func JoinAudio(gid string, uid string, s *discordgo.Session) error {
	vc, err := s.State.VoiceState(gid, uid)
	if err != nil {
		return err
	} else if vc == nil {
		return errors.New("MISSING_VOICE_CHANNEL")
	}

	// If channel exists and is already connected to the same voice channel, return
	if Channels[gid] != nil && Channels[gid].dgv != nil && Channels[gid].ChannelID == vc.ChannelID {
		return nil
	}

	// If channel exists but is in a different voice channel, disconnect first
	if Channels[gid] != nil && Channels[gid].ChannelID != vc.ChannelID {
		if Channels[gid].dgv != nil {
			Channels[gid].dgv.Disconnect()
		}
		Channels[gid].ChannelID = vc.ChannelID
	}

	// Initialize channel if it doesn't exist
	if Channels[gid] == nil {
		Channels[gid] = &Channel{ChannelID: vc.ChannelID, queue: []*youtube.VideoInfo{}, session: s, userID: uid}
	}

	// Update session and userID if needed
	if Channels[gid].session == nil {
		Channels[gid].session = s
	}
	if Channels[gid].userID == "" {
		Channels[gid].userID = uid
	}

	// Disconnect existing connection if it exists and is not ready
	if Channels[gid].dgv != nil {
		if !Channels[gid].dgv.Ready {
			Channels[gid].dgv.Disconnect()
			Channels[gid].dgv = nil
		}
	}

	// Join the voice channel
	Channels[gid].dgv, err = s.ChannelVoiceJoin(gid, vc.ChannelID, false, true)
	if err != nil {
		return err
	}

	// Wait for voice connection to be ready (with timeout)
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for !Channels[gid].dgv.Ready {
		select {
		case <-timeout:
			if Channels[gid].dgv != nil {
				Channels[gid].dgv.Disconnect()
			}
			return errors.New("timeout waiting for voice connection to be ready")
		case <-ticker.C:
			if Channels[gid].dgv == nil {
				return errors.New("voice connection was closed")
			}
		}
	}

	return nil
}

func listQueue(gid string) []string {
	if _, ex := Channels[gid]; !ex {
		return []string{"<empty>"}
	}
	queue := []string{}
	for i, v := range Channels[gid].queue {
		if i == 0 {
			queue = append(queue, fmt.Sprintf("%d. %s (playing)", i+1, v.Title))
		} else {
			queue = append(queue, fmt.Sprintf("%d. %s", i+1, v.Title))
		}
	}
	return queue
}

func AddQueue(gid string, video *youtube.VideoInfo, session *discordgo.Session, userID string) ([]string, error) {
	// Initialize channel if it doesn't exist
	if Channels[gid] == nil {
		vc, err := session.State.VoiceState(gid, userID)
		if err != nil || vc == nil {
			return []string{}, errors.New("MISSING_VOICE_CHANNEL")
		}
		Channels[gid] = &Channel{
			ChannelID: vc.ChannelID,
			queue:     []*youtube.VideoInfo{},
			session:   session,
			userID:    userID,
		}
	}

	Channels[gid].queue = append(Channels[gid].queue, video)
	if !Channels[gid].isRunning {
		go StartPlayer(gid)
	}

	queue := listQueue(gid)
	return queue, nil
}

func startFFmpegFromURL(url string) (*exec.Cmd, io.ReadCloser, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-re", // real-time pacing
		"-hide_banner",
		"-loglevel", "panic",
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-i", url,
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

func StartPlayer(gid string) {
	Channels[gid].isRunning = len(Channels[gid].queue) > 0

	// Join voice channel if not already connected
	if Channels[gid].dgv == nil {
		if debug {
			fmt.Println("Joining voice channel")
		}

		err := JoinAudio(gid, Channels[gid].userID, Channels[gid].session)
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

		if debug {
			fmt.Printf("Getting audio URL for: %s\n", queueItem.Title)
		}

		audioUrl, err := youtube.GetAudioURL(queueItem.YouTubeURL)
		if err != nil {
			fmt.Printf("Error getting audio URL: %s\n", err.Error())
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
			cmd.Process.Kill()
			Channels[gid].currentCmd = nil
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

		for range ticker.C {
			if err := binary.Read(reader, binary.LittleEndian, pcmBuf); err != nil {
				if debug {
					fmt.Println("Audio finished")
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
				if debug {
					fmt.Println("Channel is congested, dropping frame")
				}
				continue
			}
		}
	}
	delete(Channels, gid)
}

func Skip(gid string) {
	if Channels[gid] == nil || len(Channels[gid].queue) == 0 {
		return
	}

	if Channels[gid].currentCmd != nil && Channels[gid].currentCmd.Process != nil {
		Channels[gid].currentCmd.Process.Kill()
	}
}

func Clear(gid string) {
	if Channels[gid] != nil {
		Channels[gid].queue = []*youtube.VideoInfo{}
		Channels[gid].isRunning = false
		if Channels[gid].dgv != nil {
			Channels[gid].dgv.Disconnect()
		}
	}
}
