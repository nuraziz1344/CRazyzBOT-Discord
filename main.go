package main

import (
	"bytes"
	"fmt"
	"go-tube/youtube"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var BotId string
var Bot *discordgo.Session
var debug = false

func Start() {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		panic("Please fill the discord bot token")
	}
	Bot, err := discordgo.New("Bot " + botToken)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	u, err := Bot.User("@me")

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	BotId = u.ID

	Bot.AddHandler(Handler)

	err = Bot.Open()

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println("Bot Started....")
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

	if cmd == "debug" {
		debug = !debug
		youtube.Debug = !youtube.Debug
	}

	if cmd == "play" || cmd == "p" {
		query := strings.Join(args, " ")
		if len(args) == 0 {
			_, _ = s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("type `%splay <url|title>`", prefix), &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
			return
		}

		_m, _ := s.ChannelMessageSendReply(m.ChannelID, "```searching...```", &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})

		if debug {
			fmt.Println("Joining voice channel")
		}
		err := JoinAudio(m.GuildID, m.Author.ID, s)
		if err != nil {
			if err.Error() == "MISSING_VOICE_CHANNEL" {
				_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "please join voice channel first")
			} else if err.Error() == "CONFLICT" {
				_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, fmt.Sprintf("player is used at another voice channel\nto force, type %sdc first", prefix))
			} else {
				fmt.Printf("Error: %s\n", err.Error())
				_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```server error, try again\nmake sure you join a voice channel```")
			}
			return
		}

		if debug {
			fmt.Println("Parsing audio url")
		}

		var q *url.URL
		if query[:4] == "http" {
			q, err = url.ParseRequestURI(query)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
				_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```server error, try again```")
				return
			}
		}
		if query[:4] == "http" && q.Path == "/playlist" {
			lists, err := youtube.ParsePlaylist(q.String())
			if err != nil {
				if err.Error() == "playlist_not_found" {
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```playlist not found```")
				} else if err.Error() == "invalid_playlist_url" {
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```invalid playlist url```")
				} else {
					fmt.Printf("Error: %s\n", err.Error())
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```server error, try again```")
				}
				return
			}

			for _, v := range lists {
				res, err := youtube.Play(v)
				if err != nil {
					fmt.Printf("Error: %s\n", err.Error())
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```server error, try again```")
					return
				}

				if debug {
					fmt.Printf("Playing audio: %s\n", res.URL)
				}
				_, err = AddQueue(m.GuildID, res.Title, res.URL, res.Bitrate)
				if err != nil {
					fmt.Printf("Error: %s\n", err.Error())
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```server error, try again```")
					return
				}
			}

		} else {
			res, err := youtube.Play(query)
			if err != nil {
				if err.Error() == "ivalid_url" {
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```server error, maybe invalid url```")
				} else {
					fmt.Printf("Error: %s\n", err.Error())
					_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "```server error, try again```")
				}
				return
			}

			if debug {
				fmt.Printf("Playing audio: %s\n", res.URL)
			}
			_, err = AddQueue(m.GuildID, res.Title, res.URL, res.Bitrate)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
				_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, "``````server error, try again``````")
				return
			}
		}
		_, _ = s.ChannelMessageEdit(m.ChannelID, _m.ID, fmt.Sprintf("```Queue:\n%s```", strings.Join(listQueue(m.GuildID), "\n")))
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
		_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```Queue:\n%s```", strings.Join(listQueue(m.GuildID), "\n")))
	}
}

func main() {
	godotenv.Load()
	fmt.Printf("Starting... | [%s...]\n", os.Getenv("BOT_TOKEN")[:3])
	Start()

	r := mux.NewRouter()
	r.StrictSlash(true)
	r.HandleFunc(`/`, func(w http.ResponseWriter, r *http.Request) { w.Write(bytes.NewBufferString("hello").Bytes()) })

	http_port := os.Getenv(`PORT`)
	if http_port == `` {
		http_port = "8080"
	}

	fmt.Println(`Listening on http://localhost:` + http_port)
	http.ListenAndServe(`0.0.0.0:`+http_port, r)
	// <-make(chan struct{})
}
