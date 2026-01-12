package commands

import (
	"fmt"
	"go-tube/internal/player"
	"go-tube/pkg/youtube"
	"net/url"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func Play(s *discordgo.Session, m *discordgo.MessageCreate, args []string, prefix string, debug bool) {
	if len(args) == 0 {
		s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("type `%splay <url|title>`", prefix), &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
		return
	}

	// Check if user is in a voice channel
	vc, err := s.State.VoiceState(m.GuildID, m.Author.ID)
	if err != nil || vc == nil {
		s.ChannelMessageSendReply(m.ChannelID, "please join a voice channel first", &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
		return
	}

	// Check if bot is already in another voice channel
	ch := player.Channels[m.GuildID]
	if ch != nil && ch.ChannelID != vc.ChannelID {
		s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("bot is already in another voice channel\nto force, type `%sdc` first", prefix), &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
		return
	}

	_m, err := s.ChannelMessageSendReply(m.ChannelID, "searching...", &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}

	var q *url.URL
	query := strings.Join(args, " ")
	if len(query) >= 4 && query[:4] == "http" {
		q, err = url.ParseRequestURI(query)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
			return
		}
	}

	if len(query) >= 4 && query[:4] == "http" && q.Path == "/playlist" {
		handlePlaylist(s, m, _m, q.String(), debug)
	} else {
		handleSingleVideo(s, m, _m, query, debug)
	}
}

func handlePlaylist(s *discordgo.Session, m *discordgo.MessageCreate, _m *discordgo.Message, playlistURL string, debug bool) {
	if debug {
		fmt.Println("Parsing playlist")
	}

	lists, err := youtube.ParsePlaylist(playlistURL)
	if err != nil {
		if err.Error() == "playlist_not_found" {
			s.ChannelMessageEdit(m.ChannelID, _m.ID, "playlist not found")
		} else if err.Error() == "invalid_playlist_url" {
			s.ChannelMessageEdit(m.ChannelID, _m.ID, "invalid playlist url")
		} else {
			fmt.Printf("Error: %s\n", err.Error())
			s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
		}
		return
	}

	for _, v := range lists {
		res, err := youtube.GetVideoInfo(v)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
			return
		}

		if debug {
			fmt.Printf("Playing audio: %s\n", res.YouTubeURL)
		}
		_, err = player.AddQueue(m.GuildID, res, s, m.Author.ID)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
			return
		}
	}
	s.ChannelMessageEdit(m.ChannelID, _m.ID, fmt.Sprintf("Queue:\n%s", strings.Join(player.ListQueue(m.GuildID), "\n")))
}

func handleSingleVideo(s *discordgo.Session, m *discordgo.MessageCreate, _m *discordgo.Message, query string, debug bool) {
	res, err := youtube.SearchVideo(query)
	if err != nil {
		if err.Error() == "ivalid_url" {
			s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, maybe invalid url")
		} else {
			fmt.Printf("Error: %s\n", err.Error())
			s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
		}
		return
	}

	if debug {
		fmt.Printf("Adding to queue: %s\n", res.Title)
	}
	_, err = player.AddQueue(m.GuildID, res, s, m.Author.ID)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
		return
	}
	s.ChannelMessageEdit(m.ChannelID, _m.ID, fmt.Sprintf("Queue:\n%s", strings.Join(player.ListQueue(m.GuildID), "\n")))
}
