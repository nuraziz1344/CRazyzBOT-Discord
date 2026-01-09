package commands

import (
	"github.com/bwmarrin/discordgo"
)

func Ping(s *discordgo.Session, m *discordgo.MessageCreate) {
	s.ChannelMessageSendReply(m.ChannelID, "pong...", &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID})
}
