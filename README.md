# Discord Music Bot (Go)

A feature-rich Discord music bot written in Go, supporting YouTube video playback and playlist queuing with high-quality audio streaming.

## Features

- Play music from YouTube
- Queue management
- Simple and intuitive commands

## Requirements

- [Go](https://go.dev) 1.24 or higher
- [FFmpeg](https://ffmpeg.org) - for audio processing
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) - for YouTube video extraction
- Discord Bot Token from [here](https://discord.com/developers/applications)
- Google Cloud API (YouTube Data API v3) from [here](https://console.cloud.google.com/)

## Project Structure

```
/
├── cmd/bot/              # Application entry point
├── internal/
│   ├── bot/              # Bot initialization & message handling
│   ├── commands/         # Command implementations (play, skip, etc.)
│   ├── config/           # Configuration management
│   └── player/           # Audio player & voice channel management
├── pkg/youtube/          # YouTube API integration
├── assets/               # Bot assets (logo, banner)
├── Dockerfile            # Multi-stage Docker build
├── Makefile              # Build automation
└── README.md
```

## Installation

### Using Make (Recommended)

```bash
# Clone the repository
git clone https://github.com/nuraziz0404/go-discord-music.git
cd go-discord-music

# Install dependencies
go mod download

# Run the bot
go run ./cmd/bot
```

### Manual Build

```bash
# Build the bot
go build -o bin/bot ./cmd/bot

# Run the bot
./bin/bot
```

### Using Docker

```bash
# Build Docker image
docker build -t discord-music-bot .

# Run in Docker
docker run --rm --env-file .env discord-music-bot
```

## Configuration

Create a `.env` file in the project root with the following variables:

```env
BOT_TOKEN=your_discord_bot_token
PREFIX=!
YT_APIKEY=your_youtube_api_key
DEBUG=false
```

## Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `!ping` | - | Check if the bot is responsive |
| `!play <url\|title>` | `!p` | Play a song or add to queue |
| `!skip` | `!s` | Skip the current song |
| `!queue` | `!q` | Show the current queue |
| `!clear` | `!dc` | Clear queue and disconnect |

## Usage Examples

```
# Play a song by search
!play never gonna give you up

# Play from URL
!play https://www.youtube.com/watch?v=dQw4w9WgXcQ

# Play entire playlist
!play https://www.youtube.com/playlist?list=PLxxxxxx

# Check queue
!queue

# Skip current song
!skip

# Disconnect bot
!clear
```

## License

ISC License

## Acknowledgments

- [discordgo](https://github.com/bwmarrin/discordgo) - Discord API wrapper
- [opus](https://github.com/hraban/opus) - Opus audio codec bindings
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) - YouTube video extraction
