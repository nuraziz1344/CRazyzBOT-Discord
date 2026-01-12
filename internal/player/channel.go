package player

import (
	"errors"
	"fmt"
	"go-tube/pkg/youtube"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Channel struct {
	ChannelID  string
	dgv        *discordgo.VoiceConnection
	queue      []*youtube.VideoInfo
	isRunning  bool
	session    *discordgo.Session
	userID     string
	currentCmd *youtube.PipelineCmd
}

var Channels = make(map[string]*Channel)

func JoinVoice(gid string, uid string, s *discordgo.Session) error {
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
		Channels[gid] = &Channel{
			ChannelID: vc.ChannelID,
			queue:     []*youtube.VideoInfo{},
			session:   s,
			userID:    uid,
		}
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

func ListQueue(gid string) []string {
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

	queue := ListQueue(gid)
	return queue, nil
}

func Skip(gid string) {
	if Channels[gid] == nil || len(Channels[gid].queue) == 0 {
		return
	}

	Channels[gid].Kill()
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

func ClearAll() {
	for gid := range Channels {
		Clear(gid)
	}
}
