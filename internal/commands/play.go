package commands

import (
	"fmt"
	"go-tube/internal/player"
	"go-tube/pkg/provider"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func Play(s *discordgo.Session, m *discordgo.MessageCreate, args []string, prefix string, voiceChannelID string, debug bool) {
	ref := &discordgo.MessageReference{
		MessageID: m.ID,
		ChannelID: m.ChannelID,
		GuildID:   m.GuildID,
	}

	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		_, _ = s.ChannelMessageSendReply(m.ChannelID,
			fmt.Sprintf("Gunakan: `%splay <judul lagu / URL YouTube / YT Music / Spotify>`", prefix), ref)
		return
	}

	statusMsg, err := s.ChannelMessageSendReply(m.ChannelID, "🔍 Mencari musik...", ref)
	if err != nil {
		if debug {
			fmt.Printf("[Play] Error sending status message: %v\n", err)
		}
		return
	}

	tracks, err := provider.ResolveInput(query, m.Author.ID)
	if err != nil || len(tracks) == 0 {
		errMsg := "❌ Gagal menemukan musik. Pastikan kata kunci atau URL valid (YouTube, YouTube Music, atau Spotify)."
		if err != nil {
			switch err.Error() {
			case "playlist_not_found":
				errMsg = "❌ Playlist tidak ditemukan atau bersifat privat."
			case "invalid_playlist_url", "invalid_video_url", "unsupported_url", "invalid_spotify_url", "unsupported_spotify_url":
				errMsg = "❌ URL tidak valid atau tidak didukung."
			}
		}
		_, _ = s.ChannelMessageEdit(m.ChannelID, statusMsg.ID, errMsg)
		return
	}

	wasRunning := false
	if existing := player.GetPlayer(m.GuildID); existing != nil && existing.IsRunning() && existing.Queue.Len() > 0 {
		wasRunning = true
	}

	gp, totalInQueue, err := player.EnqueueTracks(s, m.GuildID, voiceChannelID, m.ChannelID, tracks...)
	if err != nil {
		_, _ = s.ChannelMessageEdit(m.ChannelID, statusMsg.ID, "❌ Gagal menambahkan lagu ke antrean.")
		return
	}

	var summary string
	if len(tracks) == 1 {
		t := tracks[0]
		summary = fmt.Sprintf("✅ Ditambahkan ke antrean: **%s** %s `(%s)` — Posisi `#%d`",
			t.DisplayTitle(), t.SourceBadge(), t.FormattedDuration(), totalInQueue)
	} else {
		summary = fmt.Sprintf("✅ Berhasil menambahkan **%d lagu** (%s) ke antrean (Total antrean: `%d lagu`).",
			len(tracks), tracks[0].SourceBadge(), totalInQueue)
	}

	if wasRunning {
		// Player is already running; attach navigation controls to this latest bot message
		embed := BuildNowPlayingEmbed(gp, summary)
		_, _ = EditToPlayerMessage(s, gp, m.ChannelID, statusMsg.ID, "", embed)
	} else {
		// Player is starting up and EventTrackStart will post the Now Playing controller shortly
		_, _ = s.ChannelMessageEdit(m.ChannelID, statusMsg.ID, summary)
	}
}
