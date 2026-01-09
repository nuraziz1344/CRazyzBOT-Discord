package bot

import (
	"fmt"
	"go-tube/internal/config"
	"go-tube/internal/player"
	"go-tube/pkg/youtube"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Session *discordgo.Session
	Config  *config.Config
	BotID   string
}

func New(cfg *config.Config) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}

	user, err := session.User("@me")
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		Session: session,
		Config:  cfg,
		BotID:   user.ID,
	}

	session.AddHandler(bot.messageHandler)

	if cfg.Debug {
		fmt.Println("Debug mode enabled")
		player.Debug = true
		youtube.Debug = true
	}

	return bot, nil
}

func (b *Bot) Start() error {
	if err := b.Session.Open(); err != nil {
		return err
	}
	fmt.Println("Bot started successfully")
	return nil
}

func (b *Bot) Stop() {
	player.ClearAll()
	if b.Session != nil {
		b.Session.Close()
	}
}
