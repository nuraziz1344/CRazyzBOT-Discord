package commands

import (
	"fmt"
	"go-tube/internal/player"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Pause pauses current playback.
func Pause(s *discordgo.Session, m *discordgo.MessageCreate) {
	ref := &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID}
	gp := player.GetPlayer(m.GuildID)
	if gp == nil || !gp.IsRunning() || gp.Queue.Len() == 0 {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, "❌ Tidak ada musik yang sedang diputar.", ref)
		return
	}

	gp.SetPaused(true)
	embed := BuildNowPlayingEmbed(gp, "⏸️ Musik dijeda.")
	_, _ = SendLatestPlayerMessage(s, gp, m.ChannelID, "", embed, ref)
}

// Resume resumes paused playback.
func Resume(s *discordgo.Session, m *discordgo.MessageCreate) {
	ref := &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID}
	gp := player.GetPlayer(m.GuildID)
	if gp == nil || !gp.IsRunning() || gp.Queue.Len() == 0 {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, "❌ Tidak ada musik yang sedang diputar.", ref)
		return
	}

	gp.SetPaused(false)
	embed := BuildNowPlayingEmbed(gp, "▶️ Musik dilanjutkan.")
	_, _ = SendLatestPlayerMessage(s, gp, m.ChannelID, "", embed, ref)
}

// Shuffle randomizes the upcoming tracks in the queue.
func Shuffle(s *discordgo.Session, m *discordgo.MessageCreate) {
	ref := &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID}
	gp := player.GetPlayer(m.GuildID)
	if gp == nil || gp.Queue.Len() == 0 {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, "❌ Antrean kosong.", ref)
		return
	}

	count := gp.Queue.Shuffle()
	notice := fmt.Sprintf("🔀 Berhasil mengacak %d lagu berikutnya di antrean.", count)
	if count <= 1 {
		notice = "ℹ️ Jumlah lagu di antrean belum cukup untuk diacak."
	}
	embed := BuildNowPlayingEmbed(gp, notice)
	_, _ = SendLatestPlayerMessage(s, gp, m.ChannelID, "", embed, ref)
}

// Repeat sets or cycles the repeat mode (Off, All, 1).
func Repeat(s *discordgo.Session, m *discordgo.MessageCreate, args []string, prefix string) {
	ref := &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID}
	gp := player.GetPlayer(m.GuildID)
	if gp == nil || gp.Queue.Len() == 0 {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, "❌ Tidak ada antrean musik yang aktif.", ref)
		return
	}

	var mode player.RepeatMode
	if len(args) == 0 {
		mode = gp.Queue.CycleRepeat()
	} else {
		parsed, ok := player.ParseRepeatMode(args[0])
		if !ok {
			_, _ = s.ChannelMessageSendReply(m.ChannelID,
				fmt.Sprintf("Gunakan: `%srepeat <off|all|1>`", prefix), ref)
			return
		}
		mode = gp.Queue.SetRepeat(parsed)
	}

	embed := BuildNowPlayingEmbed(gp, fmt.Sprintf("🔁 Mode Repeat diubah ke: **%s**", mode.String()))
	_, _ = SendLatestPlayerMessage(s, gp, m.ChannelID, "", embed, ref)
}

// Remove deletes 1 track from the queue by its 1-based number.
func Remove(s *discordgo.Session, m *discordgo.MessageCreate, args []string, prefix string) {
	ref := &discordgo.MessageReference{MessageID: m.ID, ChannelID: m.ChannelID, GuildID: m.GuildID}
	if len(args) == 0 {
		_, _ = s.ChannelMessageSendReply(m.ChannelID,
			fmt.Sprintf("Gunakan: `%sremove <nomor urut di queue>` (contoh: `%sremove 2`)", prefix, prefix), ref)
		return
	}

	idx, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || idx < 1 {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, "❌ Nomor urut tidak valid.", ref)
		return
	}

	removed, err := player.RemoveFromQueue(m.GuildID, idx)
	if err != nil {
		if err == player.ErrQueueEmpty {
			_, _ = s.ChannelMessageSendReply(m.ChannelID, "❌ Antrean kosong.", ref)
		} else {
			_, _ = s.ChannelMessageSendReply(m.ChannelID, "❌ Nomor urut tidak ditemukan di dalam antrean.", ref)
		}
		return
	}

	gp := player.GetPlayer(m.GuildID)
	notice := fmt.Sprintf("🗑️ Menghapus lagu `#%d`: **%s**", idx, removed.DisplayTitle())
	if gp != nil && gp.IsRunning() && gp.Queue.Len() > 0 && idx > 1 {
		embed := BuildNowPlayingEmbed(gp, notice)
		_, _ = SendLatestPlayerMessage(s, gp, m.ChannelID, "", embed, ref)
	} else {
		_, _ = s.ChannelMessageSendReply(m.ChannelID, notice, ref)
	}
}

// HandlePlayerButton processes clicks on the 5 navigation buttons on the latest bot message.
func HandlePlayerButton(s *discordgo.Session, i *discordgo.InteractionCreate) {
	gp := player.GetPlayer(i.GuildID)
	customID := i.MessageComponentData().CustomID

	if gp == nil || !gp.IsRunning() || gp.Queue.Len() == 0 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Components: []discordgo.MessageComponent{},
			},
		})
		return
	}

	gp.SetTextChannelID(i.ChannelID)

	switch customID {
	case BtnTogglePause:
		paused := gp.TogglePause()
		notice := "▶️ Musik dilanjutkan."
		if paused {
			notice = "⏸️ Musik dijeda."
		}
		embed := BuildNowPlayingEmbed(gp, notice)
		comps := BuildPlayerComponents(gp)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: comps,
			},
		})

	case BtnSkip:
		current := gp.Queue.Current()
		title := "lagu saat ini"
		if current != nil {
			title = current.DisplayTitle()
		}
		// Remove buttons from the skipped track's message; EventTrackStart or EventPlayerStopped will handle next UI
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("⏭️ Melewati: **%s**", title),
				Components: []discordgo.MessageComponent{},
			},
		})
		player.Skip(i.GuildID)

	case BtnShuffle:
		count := gp.Queue.Shuffle()
		notice := fmt.Sprintf("🔀 Mengacak %d lagu berikutnya.", count)
		if count <= 1 {
			notice = "ℹ️ Lagu di antrean belum cukup untuk diacak."
		}
		embed := BuildNowPlayingEmbed(gp, notice)
		comps := BuildPlayerComponents(gp)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: comps,
			},
		})

	case BtnRepeat:
		mode := gp.Queue.CycleRepeat()
		notice := fmt.Sprintf("🔁 Mode Repeat: **%s**", mode.String())
		embed := BuildNowPlayingEmbed(gp, notice)
		comps := BuildPlayerComponents(gp)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: comps,
			},
		})

	case BtnClear:
		_, _ = gp.SetLastPlayerMessage("", "")
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "🗑️ Antrean dibersihkan dan bot keluar dari voice channel.",
				Embeds:     []*discordgo.MessageEmbed{},
				Components: []discordgo.MessageComponent{},
			},
		})
		player.Clear(i.GuildID)
	}
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}
