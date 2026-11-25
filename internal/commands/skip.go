package commands

import (
	"go-tube/internal/player"

	"github.com/bwmarrin/discordgo"
)

func Skip(s *discordgo.Session, m *discordgo.MessageCreate) {
	ref := &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID}
	gp := player.GetPlayer(m.GuildID)
	if gp != nil {
		gp.SetTextChannelID(m.ChannelID)
	}
	if !player.Skip(m.GuildID) {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, "❌ Tidak ada lagu yang sedang diputar.", ref)
		return
	}
	_ = s.MessageReactionAdd(m.ChannelID, m.ID, "⏭️")
}
