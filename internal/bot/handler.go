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

	content := m.Content
	args := strings.Split(content, " ")
	
	// Check for mention at the start
	mentionPrefix := "<@" + b.BotID + ">"
	mentionPrefixNick := "<@!" + b.BotID + ">"
	
	if strings.HasPrefix(content, mentionPrefix) {
		// Remove mention and parse remaining args
		content = strings.TrimPrefix(content, mentionPrefix)
		content = strings.TrimSpace(content)
		args = strings.Split(content, " ")
		if len(args) == 0 || args[0] == "" {
			return
		}
	} else if strings.HasPrefix(content, mentionPrefixNick) {
		// Handle nickname mention format
		content = strings.TrimPrefix(content, mentionPrefixNick)
		content = strings.TrimSpace(content)
		args = strings.Split(content, " ")
		if len(args) == 0 || args[0] == "" {
			return
		}
	} else if len(args) == 0 || len(args[0]) == 0 || args[0][:1] != b.Config.Prefix {
		// Check for prefix
		return
	} else {
		// Remove prefix from command
		args[0] = args[0][1:]
	}

	cmd := args[0]
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
