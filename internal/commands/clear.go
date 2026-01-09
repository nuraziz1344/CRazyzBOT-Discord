package commands

import (
	"go-tube/internal/player"

	"github.com/bwmarrin/discordgo"
)

func Clear(s *discordgo.Session, m *discordgo.MessageCreate) {
	player.Clear(m.GuildID)
	s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
}
