package commands

import (
	"fmt"
	"go-tube/internal/player"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func Queue(s *discordgo.Session, m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Queue:\n%s", strings.Join(player.ListQueue(m.GuildID), "\n")))
}
