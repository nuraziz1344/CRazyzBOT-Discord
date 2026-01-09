package bot

import (
	"go-tube/internal/commands"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == b.BotID {
		return
	}

	args := strings.Split(m.Content, " ")
	if len(args) == 0 || len(args[0]) == 0 || args[0][:1] != b.Config.Prefix {
		return
	}

	cmd := args[0][1:]
	args = args[1:]

	switch cmd {
	case "ping":
		commands.Ping(s, m)

	case "play", "p":
		commands.Play(s, m, args, b.Config.Prefix, b.Config.Debug)

	case "skip", "s":
		commands.Skip(s, m)

	case "clear", "dc":
		commands.Clear(s, m)

	case "queue", "q":
		commands.Queue(s, m)
	}
}
