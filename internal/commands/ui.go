package commands

import (
	"fmt"
	"go-tube/internal/player"
	"go-tube/pkg/music"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	BtnTogglePause = "player:toggle_pause"
	BtnSkip        = "player:skip"
	BtnShuffle     = "player:shuffle"
	BtnRepeat      = "player:repeat"
	BtnClear       = "player:clear"
	BtnAutoplay    = "player:autoplay"
	SelectSearch   = "search:select"
)

// BuildPlayerComponents creates the navigation buttons across two ActionsRows.
func BuildPlayerComponents(gp *player.GuildPlayer) []discordgo.MessageComponent {
	if gp == nil || !gp.IsRunning() || gp.Queue.Len() == 0 {
		return []discordgo.MessageComponent{}
	}

	return buildPlayerComponents(gp.IsPaused(), gp.Queue.Repeat(), gp.Queue.Autoplay())
}

func buildPlayerComponents(paused bool, repeatMode player.RepeatMode, autoplay bool) []discordgo.MessageComponent {
	pauseLabel := "Pause"
	pauseEmoji := "⏸️"
	pauseStyle := discordgo.PrimaryButton
	if paused {
		pauseLabel = "Play"
		pauseEmoji = "▶️"
		pauseStyle = discordgo.SuccessButton
	}

	repeatStyle := discordgo.SecondaryButton
	if repeatMode != player.RepeatOff {
		repeatStyle = discordgo.PrimaryButton
	}

	autoplayStyle := discordgo.SecondaryButton
	if autoplay {
		autoplayStyle = discordgo.SuccessButton
	}

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					CustomID: BtnTogglePause,
					Label:    pauseLabel,
					Style:    pauseStyle,
					Emoji:    &discordgo.ComponentEmoji{Name: pauseEmoji},
				},
				discordgo.Button{
					CustomID: BtnSkip,
					Label:    "Next",
					Style:    discordgo.SecondaryButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "⏭️"},
				},
				discordgo.Button{
					CustomID: BtnShuffle,
					Label:    "Shuffle",
					Style:    discordgo.SecondaryButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "🔀"},
				},
				discordgo.Button{
					CustomID: BtnRepeat,
					Label:    fmt.Sprintf("Repeat: %s", repeatMode.String()),
					Style:    repeatStyle,
					Emoji:    &discordgo.ComponentEmoji{Name: "🔁"},
				},
				discordgo.Button{
					CustomID: BtnClear,
					Label:    "Clear",
					Style:    discordgo.DangerButton,
					Emoji:    &discordgo.ComponentEmoji{Name: "🗑️"},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					CustomID: BtnAutoplay,
					Label:    fmt.Sprintf("Autoplay: %s", player.FormatAutoplay(autoplay)),
					Style:    autoplayStyle,
					Emoji:    &discordgo.ComponentEmoji{Name: "📻"},
				},
			},
		},
	}
}

// BuildNowPlayingEmbed constructs the rich embed for the currently playing track and queue summary.
func BuildNowPlayingEmbed(gp *player.GuildPlayer, statusNotice string) *discordgo.MessageEmbed {
	if gp == nil {
		return nil
	}

	tracks, repeatMode := gp.Queue.Snapshot()
	autoplay := gp.Queue.Autoplay()
	if len(tracks) == 0 {
		return &discordgo.MessageEmbed{
			Title:       "⏹️ Pemutaran Berhenti",
			Description: "Antrean musik kosong.",
			Color:       0x95A5A6,
		}
	}

	current := tracks[0]
	statusText := "▶️ Playing"
	color := 0x2ECC71 // Green
	if gp.IsPaused() {
		statusText = "⏸️ Paused"
		color = 0xF39C12 // Orange
	}

	var descBuilder strings.Builder
	if current.WebURL != "" {
		descBuilder.WriteString(fmt.Sprintf("**[%s](%s)**\n", current.DisplayTitle(), current.WebURL))
	} else {
		descBuilder.WriteString(fmt.Sprintf("**%s**\n", current.DisplayTitle()))
	}

	if statusNotice != "" {
		descBuilder.WriteString(fmt.Sprintf("\n> 💡 %s\n", statusNotice))
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Status",
			Value:  statusText,
			Inline: true,
		},
		{
			Name:   "Sumber",
			Value:  string(current.Source),
			Inline: true,
		},
		{
			Name:   "Durasi",
			Value:  current.FormattedDuration(),
			Inline: true,
		},
		{
			Name:   "Repeat",
			Value:  repeatMode.String(),
			Inline: true,
		},
		{
			Name:   "Autoplay",
			Value:  player.FormatAutoplay(autoplay),
			Inline: true,
		},
		{
			Name:   "Total Queue",
			Value:  fmt.Sprintf("%d lagu", len(tracks)),
			Inline: true,
		},
	}

	if current.Autoplay {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Diminta Oleh",
			Value:  "📻 Autoplay",
			Inline: true,
		})
	} else if current.RequestedBy != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Diminta Oleh",
			Value:  fmt.Sprintf("<@%s>", current.RequestedBy),
			Inline: true,
		})
	}

	if len(tracks) > 1 {
		var upNext []string
		maxPreview := 5
		for i := 1; i < len(tracks) && i <= maxPreview; i++ {
			t := tracks[i]
			upNext = append(upNext, fmt.Sprintf("`%d.` %s %s `(%s)`", i+1, t.SourceBadge(), t.DisplayTitle(), t.FormattedDuration()))
		}
		if len(tracks)-1 > maxPreview {
			upNext = append(upNext, fmt.Sprintf("*...dan %d lagu lainnya*", len(tracks)-1-maxPreview))
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Lagu Berikutnya (Up Next)",
			Value:  strings.Join(upNext, "\n"),
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🎵 Music Player",
		Description: descBuilder.String(),
		Color:       color,
		Fields:      fields,
	}

	if current.Thumbnail != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: current.Thumbnail,
		}
	}

	return embed
}

// SendLatestPlayerMessage sends a new bot message with the player navigation buttons attached,
// and strips the navigation buttons from the previous player message so buttons only exist on the latest message.
func SendLatestPlayerMessage(s *discordgo.Session, gp *player.GuildPlayer, channelID string, content string, embed *discordgo.MessageEmbed, ref *discordgo.MessageReference) (*discordgo.Message, error) {
	if channelID == "" && gp != nil {
		channelID = gp.GetTextChannelID()
	}
	if channelID == "" {
		return nil, nil
	}

	var comps []discordgo.MessageComponent
	if gp != nil && gp.IsRunning() && gp.Queue.Len() > 0 {
		comps = BuildPlayerComponents(gp)
	}

	sendData := &discordgo.MessageSend{
		Content:    content,
		 Components: comps,
		Reference:  ref,
	}
	if embed != nil {
		sendData.Embeds = []*discordgo.MessageEmbed{embed}
	}

	msg, err := s.ChannelMessageSendComplex(channelID, sendData)
	if err != nil {
		return nil, err
	}

	if gp != nil && len(comps) > 0 {
		gp.SetTextChannelID(channelID)
		oldChanID, oldMsgID := gp.SetLastPlayerMessage(channelID, msg.ID)
		if oldMsgID != "" && oldMsgID != msg.ID {
			stripButtonsFromMessage(s, oldChanID, oldMsgID)
		}
	}

	return msg, nil
}

// EditToPlayerMessage updates an existing bot status message (such as "searching...") into the latest player message,
// stripping buttons from any older player message.
func EditToPlayerMessage(s *discordgo.Session, gp *player.GuildPlayer, channelID, messageID, content string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	var comps []discordgo.MessageComponent
	if gp != nil && gp.IsRunning() && gp.Queue.Len() > 0 {
		comps = BuildPlayerComponents(gp)
	}

	editData := &discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Content:    &content,
		Components: &comps,
	}
	if embed != nil {
		editData.Embeds = &[]*discordgo.MessageEmbed{embed}
	} else {
		emptyEmbeds := []*discordgo.MessageEmbed{}
		editData.Embeds = &emptyEmbeds
	}

	msg, err := s.ChannelMessageEditComplex(editData)
	if err != nil {
		return nil, err
	}

	if gp != nil && len(comps) > 0 {
		gp.SetTextChannelID(channelID)
		oldChanID, oldMsgID := gp.SetLastPlayerMessage(channelID, messageID)
		if oldMsgID != "" && oldMsgID != messageID {
			stripButtonsFromMessage(s, oldChanID, oldMsgID)
		}
	}

	return msg, nil
}

func stripButtonsFromMessage(s *discordgo.Session, channelID, messageID string) {
	if s == nil || channelID == "" || messageID == "" {
		return
	}
	emptyComps := []discordgo.MessageComponent{}
	_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Components: &emptyComps,
	})
}

// HandlePlayerStateChange is registered as player.OnPlayerStateChange to automatically manage
// the player card and navigation buttons when tracks start, update, or stop.
func HandlePlayerStateChange(s *discordgo.Session, gp *player.GuildPlayer, event player.PlayerEvent, detail string) {
	if s == nil || gp == nil {
		return
	}

	switch event {
	case player.EventTrackStart:
		embed := BuildNowPlayingEmbed(gp, "")
		_, _ = SendLatestPlayerMessage(s, gp, gp.GetTextChannelID(), "", embed, nil)

	case player.EventStateUpdate:
		chanID, msgID := gp.GetLastPlayerMessage()
		if chanID != "" && msgID != "" {
			embed := BuildNowPlayingEmbed(gp, detail)
			comps := BuildPlayerComponents(gp)
			_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         msgID,
				Channel:    chanID,
				Embeds:     &[]*discordgo.MessageEmbed{embed},
				Components: &comps,
			})
		}

	case player.EventPlayerStopped:
		chanID, msgID := gp.SetLastPlayerMessage("", "")
		if chanID != "" && msgID != "" {
			stripButtonsFromMessage(s, chanID, msgID)
		}
		if detail != "" {
			textChan := gp.GetTextChannelID()
			if textChan != "" {
				_, _ = s.ChannelMessageSend(textChan, "ℹ️ "+detail)
			}
		}
	}
}

// FormatTrackList formats a slice of tracks for queue display.
func FormatTrackList(tracks []*music.Track, repeatMode player.RepeatMode, autoplay bool, isPaused bool) string {
	if len(tracks) == 0 {
		return "Antrean kosong."
	}

	var sb strings.Builder
	stateLabel := "Playing"
	if isPaused {
		stateLabel = "Paused"
	}

	sb.WriteString(fmt.Sprintf("**Repeat:** `%s` | **Autoplay:** `%s` | **Total:** `%d lagu`\n\n", repeatMode.String(), player.FormatAutoplay(autoplay), len(tracks)))
	maxDisplay := 20
	for i, t := range tracks {
		if i >= maxDisplay {
			sb.WriteString(fmt.Sprintf("\n*...dan %d lagu lainnya*", len(tracks)-maxDisplay))
			break
		}
		badge := t.SourceBadge()
		if t.Autoplay {
			badge += " 📻"
		}
		if i == 0 {
			sb.WriteString(fmt.Sprintf("`%d.` 🎵 **%s** %s `(%s)` — *(%s)*\n",
				i+1, t.DisplayTitle(), badge, t.FormattedDuration(), stateLabel))
		} else {
			sb.WriteString(fmt.Sprintf("`%d.` %s %s `(%s)`\n",
				i+1, t.DisplayTitle(), badge, t.FormattedDuration()))
		}
	}
	return sb.String()
}
