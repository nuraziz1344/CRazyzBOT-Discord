package main

import (
	"errors"
	"fmt"
	"go-tube/youtube"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var BotId string
var Bot *discordgo.Session
var debug = false

func Start() error {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		return errors.New("BOT_TOKEN is not set")
	}

	Bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return err
	}

	u, err := Bot.User("@me")
	if err != nil {
		return err
	}

	BotId = u.ID
	Bot.AddHandler(Handler)

	err = Bot.Open()
	if err != nil {
		return err
	}

	if debug {
		fmt.Println("Debug mode enabled")
		youtube.Debug = true
	}

	fmt.Println("Bot started successfully")
	return nil
}

func Handler(s *discordgo.Session, m *discordgo.MessageCreate) {
	prefix := os.Getenv("PREFIX")
	if m.Author.ID == BotId {
		return
	}
	args := strings.Split(m.Content, " ")
	if len(args) == 0 || args[0][:1] != prefix {
		return
	}

	cmd := args[0][1:]
	args = args[1:]

	if cmd == "ping" {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, "pong...", &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
	}

	if cmd == "play" || cmd == "p" {
		if len(args) == 0 {
			_, _ = s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("type `%splay <url|title>`", prefix), &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
			return
		}

		// Check if user is in a voice channel
		vc, err := s.State.VoiceState(m.GuildID, m.Author.ID)
		if err != nil || vc == nil {
			_, _ = s.ChannelMessageSendReply(m.ChannelID, "please join a voice channel first", &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
			return
		}

		// Check if bot is already in another voice channel
		if Channels[m.GuildID] != nil && Channels[m.GuildID].ChannelID != vc.ChannelID {
			_, _ = s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("bot is already in another voice channel\nto force, type `%sdc` first", prefix), &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
			return
		}

		_m, _ := s.ChannelMessageSendReply(m.ChannelID, "searching...", &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})

		var q *url.URL
		query := strings.Join(args, " ")
		if query[:4] == "http" {
			q, err = url.ParseRequestURI(query)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
				_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
				return
			}
		}

		if query[:4] == "http" && q.Path == "/playlist" {
			if debug {
				fmt.Println("Parsing playlist")
			}

			lists, err := youtube.ParsePlaylist(q.String())
			if err != nil {
				if err.Error() == "playlist_not_found" {
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "playlist not found")
				} else if err.Error() == "invalid_playlist_url" {
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "invalid playlist url")
				} else {
					fmt.Printf("Error: %s\n", err.Error())
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
				}
				return
			}

			for _, v := range lists {
				res, err := youtube.GetVideoInfo(v)
				if err != nil {
					fmt.Printf("Error: %s\n", err.Error())
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
					return
				}

				if debug {
					fmt.Printf("Playing audio: %s\n", res.YouTubeURL)
				}
				_, err = AddQueue(m.GuildID, res, s, m.Author.ID)
				if err != nil {
					fmt.Printf("Error: %s\n", err.Error())
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
					return
				}
			}
		} else {
			res, err := youtube.SearchVideo(query)
			if err != nil {
				if err.Error() == "ivalid_url" {
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, maybe invalid url")
				} else {
					fmt.Printf("Error: %s\n", err.Error())
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
				}
				return
			}

			if debug {
				fmt.Printf("Playing %s\n", res.Title)
			}
			_, err = AddQueue(m.GuildID, res, s, m.Author.ID)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
				_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "server error, try again")
				return
			}
		}
		_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, fmt.Sprintf("Queue:\n%s", strings.Join(listQueue(m.GuildID), "\n")))
	}

	if cmd == "skip" || cmd == "s" {
		Skip(m.GuildID)
		_ = s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
	}

	if cmd == "clear" || cmd == "dc" {
		Clear(m.GuildID)
		_ = s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
	}

	if cmd == "queue" || cmd == "q" {
		_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Queue:\n%s", strings.Join(listQueue(m.GuildID), "\n")))
	}
}

func main() {
	fmt.Println("Starting...")
	fmt.Println("Loading environment variables...")

	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading environment variables:", err)
		return
	}

	fmt.Println("Environment variables loaded successfully")
	debug = os.Getenv("DEBUG") == "true"

	fmt.Printf("Bot token: %s...\n", os.Getenv("BOT_TOKEN")[:5])
	if err := Start(); err != nil {
		fmt.Println("Error starting bot:", err)
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	fmt.Println("Shutting down...")
	for gid := range Channels {
		Clear(gid)
	}
	if Bot != nil {
		Bot.Close()
	}
}
