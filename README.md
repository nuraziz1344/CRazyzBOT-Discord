# Discord Music Bot (Go)

Bot musik Discord berbasis Go dengan arsitektur modular, *thread-safe*, hemat CPU & memori, serta dilengkapi **30-Second Read-Ahead Opus Frame Buffer** untuk pemutaran audio bebas *lag*/putus-putus.

## Fitur Utama

- **Multi-Source Music Support**:
  - **YouTube** (Video & Playlist)
  - **YouTube Music** (Track & Playlist)
  - **Spotify** (Track, Playlist, & Album — menggunakan scraper ringan + *lazy stream resolution* tanpa perlu Spotify API Key)
- **Kontrol Playback & Antrean Lengkap**:
  - Play langsung dari kata kunci (*keyword*) maupun URL (musik tunggal / playlist / album)
  - Pencarian interaktif (`!search`) dengan **SelectMenu Dropdown** untuk memilih lagu
  - Sistem Queue otomatis (mendukung append playlist secara instan)
  - Pause / Resume, Skip / Next, Shuffle (acak antrean), Repeat (`Off`, `All`, `1`)
  - Clear seluruh antrean atau hapus 1 lagu tertentu (`!remove <nomor>`)
- **Navigasi Player Interaktif**:
  - 5 tombol navigasi (`Play/Pause`, `Next`, `Shuffle`, `Repeat`, `Clear`) yang selalu menempel secara otomatis pada **pesan terakhir bot**.
- **Otomasi & Proteksi Voice Channel**:
  - Otomatis *join* ke voice channel saat `play` dan otomatis *leave* ketika antrean habis, di-clear, atau ketika **voice channel kosong (hanya tersisa bot)**.
  - Proteksi akses command: User wajib berada di voice channel yang sama dengan bot.
  - Mendukung penuh command dari **Text Channel** biasa maupun **Voice Channel Chat (Open-Chat)**.

## Struktur Proyek

```text
/
├── cmd/bot/              # Entry point aplikasi
├── internal/
│   ├── bot/              # Inisialisasi Discord session, Gateway Intents, & Voice Guard middleware
│   ├── commands/         # Implementasi command, UI Embed, Tombol Navigasi, & Search Dropdown
│   ├── config/           # Konfigurasi environment variables
│   └── player/           # GuildPlayer manager, Queue thread-safe, & 30s Read-Ahead Opus Buffer
├── pkg/
│   ├── cookies/          # Loader Netscape cookies.txt untuk sesi scraper
│   ├── music/            # Model domain Track & klasifikasi URL/keyword
│   ├── provider/         # Unified music resolver & lazy stream resolution
│   ├── spotify/          # Scraper metadata Spotify (Track, Playlist, Album)
│   └── youtube/          # Scraper InnerTube YouTube/YT Music, Data API fallback, & Audio Pipeline (dlsrv + yt-dlp)
├── Dockerfile            # Multi-stage Docker build
├── Makefile              # Otomasi build
└── README.md
```

## Konfigurasi (`.env`)

```env
BOT_TOKEN=your_discord_bot_token
PREFIX=!
DEBUG=false

# Opsional:
BUFFER_SECONDS=30
YT_APIKEY=
COOKIE_FILE=cookies.txt
PROXY=
```

## Daftar Perintah (Commands)

| Command | Alias | Deskripsi |
|---------|-------|-----------|
| `!play <keyword\|url>` | `!p` | Putar musik langsung dari kata kunci atau URL (YT / YT Music / Spotify) |
| `!search <keyword>` | `!sc`, `!find` | Cari lagu dan pilih melalui menu *dropdown* interaktif |
| `!pause` | - | Jeda lagu yang sedang diputar |
| `!resume` | `!unpause` | Lanjutkan lagu yang dijeda |
| `!skip` | `!s`, `!next` | Lewati lagu yang sedang diputar |
| `!queue` | `!q` | Tampilkan daftar antrean musik, status repeat, dan durasi |
| `!shuffle` | `!sh`, `!acak` | Acak urutan lagu berikutnya di dalam antrean |
| `!repeat [off\|all\|1]` | `!loop`, `!rp` | Ubah mode repeat (`Off`, `All`, `1`) |
| `!remove <nomor>` | `!rm`, `!del` | Hapus 1 lagu tertentu dari antrean berdasarkan nomor urut |
| `!clear` | `!dc`, `!leave`, `!stop` | Kosongkan seluruh antrean, hentikan musik, dan keluar dari voice channel |
| `!help` | `!h` | Tampilkan panduan seluruh perintah bot |
| `!ping` | - | Cek responsivitas bot |
