package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
)

type Queue struct {
	Title   string
	Session *dca.EncodeSession
}

type Channel struct {
	ChannelID string
	dgv       *discordgo.VoiceConnection
	queue     []*Queue
	isRunning bool
}

var Channels = make(map[string]*Channel)

func JoinAudio(gid string, uid string, s *discordgo.Session) error {
	vc, err := s.State.VoiceState(gid, uid)
	if err != nil {
		return err
	} else if vc == nil {
		return errors.New("MISSING_VOICE_CHANNEL")
	}

	if Channels[gid] != nil && Channels[gid].ChannelID == vc.ChannelID {
		return nil
	}

	if Channels[gid] == nil {
		Channels[gid] = &Channel{ChannelID: vc.ChannelID, queue: []*Queue{}}
	} else if Channels[gid].ChannelID != vc.ChannelID {
		return errors.New("CONFLICT")
	}

	Channels[gid].dgv, err = s.ChannelVoiceJoin(gid, vc.ChannelID, false, true)
	if err != nil {
		return err
	}
	return nil
}

func encodingOption(bitrate int) *dca.EncodeOptions {
	options := dca.StdEncodeOptions
	options.RawOutput = true
	options.Bitrate = bitrate
	options.Application = "audio"
	options.BufferedFrames = 50
	return options
}

func listQueue(gid string) []string {
	if _, ex := Channels[gid]; !ex {
		return []string{"<empty>"}
	}
	queue := []string{}
	for i, v := range Channels[gid].queue {
		_title := v.Title
		if i == 0 {
			_title = fmt.Sprintf("%s (playing)", _title)
		}
		queue = append(queue, fmt.Sprintf("%d. %s", i+1, _title))
	}
	return queue

}

func AddQueue(gid string, title string, url string, bitrate int) ([]string, error) {
	encSession, err := dca.EncodeFile(url, encodingOption(bitrate))
	if err != nil {
		return []string{}, err
	}
	Channels[gid].queue = append(Channels[gid].queue, &Queue{Title: title, Session: encSession})

	if !Channels[gid].isRunning {
		go StartPlayer(gid)
	}
	queue := listQueue(gid)
	return queue, nil
}

func StartPlayer(gid string) {
	Channels[gid].isRunning = len(Channels[gid].queue) > 0
	for Channels[gid].isRunning {
		audSession := Channels[gid].queue[0].Session
		stream := dca.NewStream(audSession, Channels[gid].dgv, make(chan error))
		isDone, _ := stream.Finished()
		for !isDone {
			var err error
			isDone, err = stream.Finished()
			if err != nil {
				fmt.Printf("Error finished stream: %s\n", err.Error())
			}
			if isDone {
				Channels[gid].queue = Channels[gid].queue[1:]
				if len(Channels[gid].queue) == 0 {
					Channels[gid].isRunning = false
					defer Channels[gid].dgv.Disconnect()
				}
			}
			time.Sleep(time.Second)
		}
		time.Sleep(time.Second)
	}
	delete(Channels, gid)
}

func Skip(gid string) {
	if Channels[gid] != nil && len(Channels[gid].queue) != 0 {
		Channels[gid].queue[0].Session.Cleanup()
	}
}

func Clear(gid string) {
	if Channels[gid] != nil && len(Channels[gid].queue) != 0 {
		for _, v := range Channels[gid].queue[1:] {
			v.Session.Cleanup()
		}
		if len(Channels[gid].queue) != 0 {
			Channels[gid].queue = Channels[gid].queue[:1]
		}
		Channels[gid].queue[0].Session.Cleanup()
	}
}
