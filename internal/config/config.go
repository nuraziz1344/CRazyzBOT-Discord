package config

import (
	"errors"
	"fmt"
	"go-tube/pkg/cookies"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken      string
	Prefix        string
	YTAPIKey      string
	Debug         bool
	CookieFile    string
	Proxy         string
	BufferSeconds int
}

func Load() (*Config, error) {
	// Load .env if present; ignore error if running with OS environment variables (e.g. Docker)
	_ = godotenv.Load()

	botToken := strings.TrimSpace(os.Getenv("BOT_TOKEN"))
	if botToken == "" {
		return nil, errors.New("BOT_TOKEN is not set")
	}

	prefix := strings.TrimSpace(os.Getenv("PREFIX"))
	if prefix == "" {
		prefix = "!"
	}

	bufferSeconds := 30
	if rawBuf := strings.TrimSpace(os.Getenv("BUFFER_SECONDS")); rawBuf != "" {
		if parsed, err := strconv.Atoi(rawBuf); err == nil && parsed > 0 {
			bufferSeconds = parsed
		}
	}
	if bufferSeconds < 2 {
		bufferSeconds = 2
	} else if bufferSeconds > 300 {
		bufferSeconds = 300
	}

	return &Config{
		BotToken:      botToken,
		Prefix:        prefix,
		YTAPIKey:      strings.TrimSpace(os.Getenv("YT_APIKEY")),
		Debug:         strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG")), "true"),
		CookieFile:    strings.TrimSpace(os.Getenv("COOKIE_FILE")),
		Proxy:         strings.TrimSpace(os.Getenv("PROXY")),
		BufferSeconds: bufferSeconds,
	}, nil
}

// PrintStartupLog prints the loaded configuration and estimated RAM impact to stdout.
func (c *Config) PrintStartupLog() {
	const (
		framesPerSec        = 50  // 20ms per Opus frame
		avgBytesPerFrame    = 324 // ~300B Opus payload + 24B Go slice header in channel
		pipeAndEncoderBytes = 96 * 1024 // 64KB bufio reader + ~32KB Opus encoder & PCM scratch
		maxSearchCacheKB    = 250 // Max 500 cached metadata entries (TTL 30m)
	)

	totalFrames := c.BufferSeconds * framesPerSec
	opusBufferBytes := totalFrames * avgBytesPerFrame
	totalPerGuildBytes := opusBufferBytes + pipeAndEncoderBytes

	opusBufferKB := float64(opusBufferBytes) / 1024.0
	totalPerGuildKB := float64(totalPerGuildBytes) / 1024.0
	totalPerGuildMB := totalPerGuildKB / 1024.0

	ytAPIStatus := "Disabled (Using InnerTube Scraper)"
	if c.YTAPIKey != "" && c.YTAPIKey != "Abc123" {
		ytAPIStatus = fmt.Sprintf("Enabled (%s...)", maskSecret(c.YTAPIKey))
	}

	cookieStatus := "Not configured"
	if c.CookieFile != "" {
		if entries, err := cookies.LoadNetscape(c.CookieFile); err == nil {
			cookieStatus = fmt.Sprintf("%s (Loaded %d cookies)", c.CookieFile, len(entries))
		} else {
			cookieStatus = fmt.Sprintf("%s (File not found / unreadable)", c.CookieFile)
		}
	}

	proxyStatus := "Direct connection (No proxy)"
	if c.Proxy != "" {
		proxyStatus = c.Proxy
	}

	fmt.Println("============================================================")
	fmt.Println("               DISCORD MUSIC BOT CONFIGURATION              ")
	fmt.Println("============================================================")
	fmt.Printf("  Bot Token          : %s...\n", maskSecret(c.BotToken))
	fmt.Printf("  Command Prefix     : %q (or @Bot mention)\n", c.Prefix)
	fmt.Printf("  Debug Mode         : %v\n", c.Debug)
	fmt.Printf("  Music Sources      : YouTube, YouTube Music, Spotify\n")
	fmt.Printf("  Audio Pipeline     : Tier 1 (dlsrv Scraper) -> Tier 2 (yt-dlp)\n")
	fmt.Printf("  YouTube Data API   : %s\n", ytAPIStatus)
	fmt.Printf("  Cookie File        : %s\n", cookieStatus)
	fmt.Printf("  Network Proxy      : %s\n", proxyStatus)
	fmt.Println("------------------------------------------------------------")
	fmt.Println("                 MEMORY & BUFFER ESTIMATION                 ")
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  Read-Ahead Buffer  : %d seconds (%d Opus frames @ 20ms)\n", c.BufferSeconds, totalFrames)
	fmt.Printf("  Est. Buffer RAM    : ~%.0f KB per active voice channel\n", opusBufferKB)
	fmt.Printf("  Pipe & Encoder RAM : ~96 KB per active voice channel\n")
	fmt.Printf("  Total Audio RAM    : ~%.0f KB (~%.2f MB) / active server\n", totalPerGuildKB, totalPerGuildMB)
	fmt.Printf("  Metadata Cache RAM : Up to ~%d KB global (max 500 items, TTL 30m)\n", maxSearchCacheKB)
	fmt.Println("============================================================")
}

func maskSecret(s string) string {
	if len(s) <= 5 {
		return "***"
	}
	return s[:5]
}
