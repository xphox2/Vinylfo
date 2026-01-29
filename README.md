# Vinylfo

**Vinyl Collection Manager with YouTube Integration & OBS Support**

Vinylfo is a self-hosted web application for managing your vinyl record collection. Sync with Discogs to import your collection, create playlists, track listening sessions, and stream music videos to OBS for live broadcasts.

## 🌟 Key Features

- **📀 Discogs Integration** - OAuth sync to import your vinyl collection with full metadata
- **🎵 Playlist Management** - Create, organize, and shuffle custom playlists
- **📺 YouTube Integration** - Connect your YouTube account to export playlists and match tracks to music videos
- **🎬 OBS Streaming** - Real-time video feeds for professional streaming overlays
- **⏱️ Duration Resolution** - Automatically resolve missing track durations from multiple sources
- **🔄 Playback Tracking** - Track listening sessions with progress persistence across browser tabs
- **🔒 Secure** - OAuth tokens encrypted at rest with automatic refresh

## 🚀 Quick Start

### Installation

1. **Download latest version.**

2. **Place vinylfo.exe into a new folder.**

3. **Run vinylfo.exe**

4. **Open in browser:**
   ```
   http://localhost:8080
   ```
## 📖 Documentation

- **[API.md](API.md)** - Complete REST API reference with all 168+ endpoints
- **[OBS_SUPPORT.md](OBS_SUPPORT.md)** - OBS Studio integration guide for streaming feeds
- **[CHANGELOG.md](CHANGELOG.md)** - Version history and release notes

## 🎯 Usage

### Basic Collection Management

1. **Home** (`/`) - Browse albums and tracks
2. **Player** (`/player`) - Playback dashboard with queue
3. **Playlist** (`/playlist`) - Create and manage playlists

### Discogs Sync

1. Go to **Settings** (`/settings`)
2. Click "Connect Discogs"
3. Authorize the application
4. Go to **Sync** (`/sync`) to start importing

### YouTube Integration

1. Go to **Settings** (`/settings`)
2. Click "Connect YouTube"
3. Authorize the application
4. Use **YouTube** (`/youtube`) page to:
   - Browse your YouTube playlists
   - Export vinyl playlists to YouTube
   - Match tracks to music videos

### OBS Streaming

Add browser sources to OBS Studio:

- **Video Feed** (`http://localhost:8080/feeds/video`) - YouTube video player
- **Album Art** (`http://localhost:8080/feeds/art`) - Current track artwork  
- **Track Info** (`http://localhost:8080/feeds/track`) - Artist/title/album display

See [OBS.md](OBS.md) for complete setup instructions.

### Duration Resolution

If tracks are missing duration information:

1. Go to **Resolution Center** (`/resolution-center`)
2. The system will query multiple sources (MusicBrainz, Wikipedia, Last.fm, YouTube)
3. Review tracks where sources disagree
4. Bulk process remaining tracks

## 🏗️ Architecture

### Tech Stack

- **Backend:** Go 1.24 with Gin framework
- **Database:** SQLite (default) or MySQL
- **Frontend:** Vanilla JavaScript with Server-Sent Events

### Project Structure

```
vinylfo/
├── main.go                # Application entry point
├── controllers/           # HTTP handlers
│   ├── album.go           # Album management
│   ├── playback.go        # Playback control
│   ├── discogs.go         # Discogs integration
│   ├── youtube.go         # YouTube integration
│   └── ...
├── services/              # Business logic
│   ├── album_import.go    # Import from Discogs
│   ├── duration_resolver.go  # Duration lookup
│   └── ...
├── models/                # Database models
├── routes/                # Route definitions
├── templates/             # HTML templates
├── static/                # JS/CSS assets
├── duration/              # Duration resolution clients
└── discogs/               # Discogs API client
```

### Database

SQLite:

- Automatic migrations on startup
- Relationships: Albums → Tracks → Playlists → Sessions

## 🔐 Security

- OAuth tokens encrypted with AES-256-GCM
- HTTPS support for production deployments
- Security headers on all responses (CSP, X-Frame-Options, etc.)
- Rate limiting on external API calls
- No sensitive data in logs

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices
- Add tests for new features
- Update CHANGELOG.md
- Ensure all API endpoints are documented in API.md

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Discogs API](https://www.discogs.com/developers/) for collection data
- [YouTube Data API](https://developers.google.com/youtube/v3) for video integration
- [MusicBrainz](https://musicbrainz.org/) for duration metadata
- [Last.fm](https://www.last.fm/api) for track information

---

Made with ❤️ for vinyl enthusiasts
