package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken string
	Prefix   string
	YTAPIKey string
	Debug    bool
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		return nil, errors.New("BOT_TOKEN is not set")
	}

	return &Config{
		BotToken: botToken,
		Prefix:   os.Getenv("PREFIX"),
		YTAPIKey: os.Getenv("YT_APIKEY"),
		Debug:    os.Getenv("DEBUG") == "true",
	}, nil
}
