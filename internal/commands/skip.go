package commands

import (
	"go-tube/internal/player"

	"github.com/bwmarrin/discordgo"
)

func Skip(s *discordgo.Session, m *discordgo.MessageCreate) {
	player.Skip(m.GuildID)
	s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
}
