package bot

import (
	"fmt"
	"go-tube/internal/commands"
	"go-tube/internal/config"
	"go-tube/internal/player"
	"go-tube/pkg/provider"

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

	// Enable intents required for guild state, text channels, voice channel open-chat, and voice states
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsMessageContent

	user, err := session.User("@me")
	if err != nil {
		return nil, err
	}

	b := &Bot{
		Session: session,
		Config:  cfg,
		BotID:   user.ID,
	}

	provider.Configure(cfg.Debug, cfg.CookieFile, cfg.Proxy)
	player.Debug = cfg.Debug
	player.SetBufferSeconds(cfg.BufferSeconds)
	player.OnPlayerStateChange = commands.HandlePlayerStateChange

	session.AddHandler(b.messageHandler)
	session.AddHandler(b.interactionHandler)
	session.AddHandler(b.voiceStateHandler)

	if cfg.Debug {
		session.LogLevel = discordgo.LogInformational
		fmt.Println("Debug mode enabled")
	}

	return b, nil
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
		_ = b.Session.Close()
	}
}
