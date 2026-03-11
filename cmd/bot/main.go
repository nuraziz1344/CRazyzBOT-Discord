package main

import (
	"fmt"
	"go-tube/internal/bot"
	"go-tube/internal/config"
	"go-tube/pkg/youtube"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fmt.Println("Starting Discord Music Bot...")
	fmt.Println("Loading environment variables...")

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Error loading configuration:", err)
		return
	}

	fmt.Println("Environment variables loaded successfully")
	fmt.Printf("Bot token: %s...\n", cfg.BotToken[:5])

	// Set youtube package config
	youtube.Debug = cfg.Debug
	youtube.CookieFile = cfg.CookieFile
	youtube.Proxy = cfg.Proxy

	discordBot, err := bot.New(cfg)
	if err != nil {
		fmt.Println("Error creating bot:", err)
		return
	}

	if err := discordBot.Start(); err != nil {
		fmt.Println("Error starting bot:", err)
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	fmt.Println("Shutting down...")
	discordBot.Stop()
}
