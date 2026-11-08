package bot

import (
	"errors"
	"fmt"
	"go-tube/internal/commands"
	"go-tube/internal/player"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || m.Author.ID == b.BotID || m.GuildID == "" {
		return
	}

	content := strings.TrimSpace(m.Content)
	if content == "" {
		return
	}

	mentionPrefix := "<@" + b.BotID + ">"
	mentionPrefixNick := "<@!" + b.BotID + ">"

	var cmdLine string
	switch {
	case strings.HasPrefix(content, mentionPrefix):
		cmdLine = strings.TrimSpace(strings.TrimPrefix(content, mentionPrefix))
	case strings.HasPrefix(content, mentionPrefixNick):
		cmdLine = strings.TrimSpace(strings.TrimPrefix(content, mentionPrefixNick))
	case strings.HasPrefix(content, b.Config.Prefix):
		cmdLine = strings.TrimSpace(strings.TrimPrefix(content, b.Config.Prefix))
	default:
		return
	}

	if cmdLine == "" {
		return
	}

	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return
	}

	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	if !isKnownCommand(cmd) {
		return
	}

	// Enforce Voice Channel Guard:
	// 1. User must be in a voice channel.
	// 2. If bot is already in a voice channel, user must be in the same voice channel.
	voiceChannelID, err := player.ValidateVoiceAccess(s, m.GuildID, m.Author.ID)
	if err != nil {
		ref := &discordgo.MessageReference{
			MessageID: m.ID,
			ChannelID: m.ChannelID,
			GuildID:   m.GuildID,
		}
		if errors.Is(err, player.ErrUserInDifferentVoice) {
			_, _ = s.ChannelMessageSendReply(m.ChannelID,
				"⚠️ Bot sedang aktif di voice channel lain. Kamu harus berada di voice channel yang sama dengan bot untuk menjalankan command.", ref)
		} else {
			_, _ = s.ChannelMessageSendReply(m.ChannelID,
				"⚠️ Kamu harus bergabung ke voice channel terlebih dahulu untuk menjalankan command.", ref)
		}
		return
	}

	switch cmd {
	case "ping":
		commands.Ping(s, m)

	case "help", "h":
		b.sendHelp(s, m)

	case "play", "p":
		commands.Play(s, m, args, b.Config.Prefix, voiceChannelID, b.Config.Debug)

	case "search", "find", "sc":
		commands.Search(s, m, args, b.Config.Prefix)

	case "pause":
		commands.Pause(s, m)

	case "resume", "unpause":
		commands.Resume(s, m)

	case "skip", "s", "next":
		commands.Skip(s, m)

	case "shuffle", "sh", "acak":
		commands.Shuffle(s, m)

	case "repeat", "loop", "rp":
		commands.Repeat(s, m, args, b.Config.Prefix)

	case "remove", "rm", "delete", "del":
		commands.Remove(s, m, args, b.Config.Prefix)

	case "clear", "dc", "leave", "stop":
		commands.Clear(s, m)

	case "queue", "q":
		commands.Queue(s, m)
	}
}

func (b *Bot) interactionHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent || i.GuildID == "" {
		return
	}

	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}
	if userID == "" {
		return
	}

	voiceChannelID, err := player.ValidateVoiceAccess(s, i.GuildID, userID)
	if err != nil {
		msg := "⚠️ Kamu harus bergabung ke voice channel terlebih dahulu."
		if errors.Is(err, player.ErrUserInDifferentVoice) {
			msg = "⚠️ Bot sedang digunakan di voice channel lain. Kamu harus berada di voice channel yang sama dengan bot."
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: msg,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	customID := i.MessageComponentData().CustomID
	if customID == commands.SelectSearch {
		commands.HandleSearchSelect(s, i, voiceChannelID)
		return
	}

	commands.HandlePlayerButton(s, i)
}

func (b *Bot) voiceStateHandler(s *discordgo.Session, vsu *discordgo.VoiceStateUpdate) {
	player.HandleVoiceStateUpdate(s, vsu, b.BotID)
}

func isKnownCommand(cmd string) bool {
	switch cmd {
	case "ping", "help", "h",
		"play", "p",
		"search", "find", "sc",
		"pause", "resume", "unpause",
		"skip", "s", "next",
		"shuffle", "sh", "acak",
		"repeat", "loop", "rp",
		"remove", "rm", "delete", "del",
		"clear", "dc", "leave", "stop",
		"queue", "q":
		return true
	default:
		return false
	}
}

func (b *Bot) sendHelp(s *discordgo.Session, m *discordgo.MessageCreate) {
	p := b.Config.Prefix
	desc := fmt.Sprintf(
		"**Sumber Musik:** YouTube, YouTube Music, Spotify (Track/Playlist/Album)\n\n"+
			"• `%splay <keyword/url>` (`%sp`) — Putar musik langsung dari kata kunci atau URL\n"+
			"• `%ssearch <keyword>` (`%ssc`) — Cari lagu & pilih melalui menu dropdown\n"+
			"• `%spause` / `%sresume` — Jeda atau lanjutkan pemutaran\n"+
			"• `%sskip` (`%ss`, `%snext`) — Lewati lagu yang sedang diputar\n"+
			"• `%squeue` (`%sq`) — Tampilkan daftar antrean musik\n"+
			"• `%sshuffle` (`%ssh`) — Acak urutan antrean musik\n"+
			"• `%srepeat [off|all|1]` (`%sloop`) — Atur mode pengulangan lagu\n"+
			"• `%sremove <nomor>` (`%srm`, `%sdel`) — Hapus 1 lagu tertentu dari antrean\n"+
			"• `%sclear` (`%sdc`, `%sleave`) — Hapus seluruh antrean & keluar dari voice channel\n"+
			"• `%sping` — Cek status respons bot",
		p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p,
	)

	embed := &discordgo.MessageEmbed{
		Title:       "📖 Panduan Perintah Music Bot",
		Description: desc,
		Color:       0x9B59B6,
	}

	ref := &discordgo.MessageReference{
		MessageID: m.ID,
		ChannelID: m.ChannelID,
		GuildID:   m.GuildID,
	}
	_, _ = s.ChannelMessageSendEmbedReply(m.ChannelID, embed, ref)
}
