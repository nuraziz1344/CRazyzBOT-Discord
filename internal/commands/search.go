package commands

import (
	"fmt"
	"go-tube/internal/player"
	"go-tube/pkg/music"
	"go-tube/pkg/provider"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

type searchSession struct {
	GuildID     string
	ChannelID   string
	RequesterID string
	Query       string
	Tracks      []*music.Track
	ExpiresAt   time.Time
}

var (
	searchMu       sync.RWMutex
	searchSessions = make(map[string]*searchSession)
)

const searchSessionTTL = 2 * time.Minute

func saveSearchSession(messageID string, sess *searchSession) {
	searchMu.Lock()
	defer searchMu.Unlock()

	now := time.Now()
	for k, v := range searchSessions {
		if now.After(v.ExpiresAt) {
			delete(searchSessions, k)
		}
	}
	searchSessions[messageID] = sess
}

func popSearchSession(messageID string) *searchSession {
	searchMu.Lock()
	defer searchMu.Unlock()
	sess, ok := searchSessions[messageID]
	if !ok {
		return nil
	}
	delete(searchSessions, messageID)
	if time.Now().After(sess.ExpiresAt) {
		return nil
	}
	return sess
}

// Search handles the `!search <keyword>` command and renders a dropdown SelectMenu of results.
func Search(s *discordgo.Session, m *discordgo.MessageCreate, args []string, prefix string) {
	ref := &discordgo.MessageReference{
		MessageID: m.ID,
		ChannelID: m.ChannelID,
		GuildID:   m.GuildID,
	}

	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		_, _ = s.ChannelMessageSendReply(m.ChannelID,
			fmt.Sprintf("Gunakan: `%ssearch <kata kunci lagu>`", prefix), ref)
		return
	}

	statusMsg, err := s.ChannelMessageSendReply(m.ChannelID, fmt.Sprintf("🔎 Mencari daftar lagu untuk **`%s`**...", query), ref)
	if err != nil {
		return
	}

	results, err := provider.Search(query, 10, m.Author.ID)
	if err != nil || len(results) == 0 {
		_, _ = s.ChannelMessageEdit(m.ChannelID, statusMsg.ID, "❌ Tidak ditemukan hasil pencarian untuk kata kunci tersebut.")
		return
	}

	options := make([]discordgo.SelectMenuOption, 0, len(results))
	var listBuilder strings.Builder
	for i, t := range results {
		label := fmt.Sprintf("%d. %s", i+1, t.Title)
		label = truncateRunes(label, 95)

		desc := fmt.Sprintf("%s • %s", t.Artist, t.FormattedDuration())
		if strings.TrimSpace(t.Artist) == "" {
			desc = t.FormattedDuration()
		}
		desc = truncateRunes(desc, 95)

		options = append(options, discordgo.SelectMenuOption{
			Label:       label,
			Value:       strconv.Itoa(i),
			Description: desc,
			Emoji:       &discordgo.ComponentEmoji{Name: "🎵"},
		})

		listBuilder.WriteString(fmt.Sprintf("`%d.` **%s** — %s `(%s)`\n",
			i+1, t.DisplayTitle(), string(t.Source), t.FormattedDuration()))
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🔎 Hasil Pencarian: %s", truncateRunes(query, 200)),
		Description: listBuilder.String(),
		Color:       0x3498DB,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Pilih lagu melalui menu dropdown di bawah (berlaku 2 menit)",
		},
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    SelectSearch,
					Placeholder: "Pilih lagu untuk diputar...",
					Options:     options,
				},
			},
		},
	}

	emptyContent := ""
	_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         statusMsg.ID,
		Channel:    m.ChannelID,
		Content:    &emptyContent,
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	})
	if err != nil {
		return
	}

	saveSearchSession(statusMsg.ID, &searchSession{
		GuildID:     m.GuildID,
		ChannelID:   m.ChannelID,
		RequesterID: m.Author.ID,
		Query:       query,
		Tracks:      results,
		ExpiresAt:   time.Now().Add(searchSessionTTL),
	})
}

// HandleSearchSelect processes the SelectMenu interaction when a user chooses a song from `!search`.
func HandleSearchSelect(s *discordgo.Session, i *discordgo.InteractionCreate, voiceChannelID string) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		respondEphemeral(s, i, "❌ Pilihan tidak valid.")
		return
	}

	sess := popSearchSession(i.Message.ID)
	if sess == nil {
		emptyComps := []discordgo.MessageComponent{}
		expiredText := "⌛ Sesi pencarian ini telah berakhir. Silakan jalankan perintah `search` kembali."
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    expiredText,
				Embeds:     []*discordgo.MessageEmbed{},
				Components: emptyComps,
			},
		})
		return
	}

	idx, err := strconv.Atoi(data.Values[0])
	if err != nil || idx < 0 || idx >= len(sess.Tracks) {
		respondEphemeral(s, i, "❌ Lagu yang dipilih tidak valid.")
		return
	}

	selected := sess.Tracks[idx].Clone()
	selected.RequestedBy = interactionUserID(i)

	wasRunning := false
	if existing := player.GetPlayer(i.GuildID); existing != nil && existing.IsRunning() && existing.Queue.Len() > 0 {
		wasRunning = true
	}

	gp, totalInQueue, err := player.EnqueueTracks(s, i.GuildID, voiceChannelID, i.ChannelID, selected)
	if err != nil {
		respondEphemeral(s, i, "❌ Gagal menambahkan lagu ke antrean.")
		return
	}

	summary := fmt.Sprintf("✅ Dipilih & ditambahkan ke antrean: **%s** %s `(%s)` — Posisi `#%d`",
		selected.DisplayTitle(), selected.SourceBadge(), selected.FormattedDuration(), totalInQueue)

	if wasRunning {
		embed := BuildNowPlayingEmbed(gp, summary)
		comps := BuildPlayerComponents(gp)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "",
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: comps,
			},
		})
		gp.SetTextChannelID(i.ChannelID)
		oldChanID, oldMsgID := gp.SetLastPlayerMessage(i.ChannelID, i.Message.ID)
		if oldMsgID != "" && oldMsgID != i.Message.ID {
			stripButtonsFromMessage(s, oldChanID, oldMsgID)
		}
	} else {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    summary,
				Embeds:     []*discordgo.MessageEmbed{},
				Components: []discordgo.MessageComponent{},
			},
		})
	}
}

func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if maxRunes > 3 {
		return string(runes[:maxRunes-3]) + "..."
	}
	return string(runes[:maxRunes])
}
