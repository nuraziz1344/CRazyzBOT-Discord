package commands

import (
	"go-tube/internal/player"

	"github.com/bwmarrin/discordgo"
)

func Queue(s *discordgo.Session, m *discordgo.MessageCreate) {
	ref := &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID}
	gp := player.GetPlayer(m.GuildID)
	if gp == nil || gp.Queue.Len() == 0 {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, "📭 Antrean musik saat ini kosong.", ref)
		return
	}

	tracks, repeatMode := gp.Queue.Snapshot()
	embed := &discordgo.MessageEmbed{
		Title:       "📋 Daftar Antrean Musik (Queue)",
		Description: FormatTrackList(tracks, repeatMode, gp.IsPaused()),
		Color:       0x3498DB,
	}

	_, _ = SendLatestPlayerMessage(s, gp, m.ChannelID, "", embed, ref)
}
