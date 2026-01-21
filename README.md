# Vinylfo - Vinyl Collection Manager

Vinylfo is a self-hosted web application for managing your vinyl record collection. It syncs with your Discogs account to import albums and tracks, provides playlist management, and tracks your listening sessions.

## Features

### Collection Management
- **Discogs Integration**: Connect your Discogs account via OAuth to sync your collection
- **Automatic Sync**: Import albums from all Discogs folders or specific folders
- **Album Metadata**: Stores artist, title, year, genre, style, label, country, and cover images
- **Track Information**: Imports full tracklists with duration, position, and side information
- **Local Search**: Search albums and tracks in your local database

### Playback Tracking
- **Session Management**: Track what you're currently playing
- **Listening History**: Records play history for statistics
- **Progress Tracking**: Saves playback progress across browser sessions
- **Queue Display**: See upcoming tracks in your current session

### Playlist Features
- **Create Playlists**: Organize tracks into custom playlists
- **Add/Remove Tracks**: Add individual tracks or entire albums
- **Shuffle**: Randomize playlist order
- **Play Playlists**: Start playback from any playlist

### Resolution Center
- **Automatic Duration Lookup**: Resolves missing track durations by querying external databases
- **MusicBrainz Integration**: Queries MusicBrainz for track durations with rate limiting
- **Wikipedia Integration**: Parses Wikipedia album pages for track listings
- **Last.fm Integration**: Queries Last.fm API for track durations
- **YouTube Integration**: Searches YouTube for track videos and extracts duration (optional)
- **Consensus Algorithm**: Requires 2+ sources to agree before auto-applying durations
- **Smart Matching**: Normalizes artist names and titles for better matching
  - Handles Discogs disambiguation suffixes like "(2)", "(rapper)"
  - Handles edition suffixes like "(Remastered)", "(Deluxe Edition)"
- **Review Queue**: Manual review for tracks where sources disagree
- **Bulk Processing**: Background worker processes all tracks with missing durations
- **YouTube Quota Optimization**:
  - Skips YouTube API when free sources already reach consensus
  - File-based cache persists results across database resets
- **YouTube Playlist Export**: Export vinyl playlists directly to your YouTube account

### YouTube Integration
- **Connect YouTube Account**: OAuth 2.0 integration to link your YouTube account
- **YouTube Page** (`/youtube`): Dedicated page for managing YouTube content
  - View your channel information
  - Browse and manage your YouTube playlists
  - Search and view music videos
  - Create new playlists directly from the UI
- **Playlist Export**: Export vinyl playlists to your YouTube account
- **Secure Token Storage**: OAuth tokens encrypted and stored in database with automatic refresh
- **Smart API Usage**: 10,000 daily quota units per connected user

### Discogs Sync Features
- **Pause/Resume**: Long syncs can be paused and resumed later
- **Progress Persistence**: Sync progress saved to database (survives restarts)
- **Folder Support**: Sync all folders or select specific ones
- **Refresh Tracks**: Re-sync track listings from Discogs
- **Cleanup Tool**: Find and remove albums no longer in your Discogs collection
- **Rate Limit Handling**: Respects Discogs API limits (60 req/min authenticated)

## Screenshots

The application includes pages for:
- **Home** (`/`) - Browse albums and tracks
- **Player** (`/player`) - Playback dashboard with queue
- **Playlist** (`/playlist`) - Manage playlists
- **Sync** (`/sync`) - Discogs sync dashboard
- **YouTube** (`/youtube`) - YouTube integration and playlist management
- **Duration Review** (`/resolution-center`) - Resolve missing track durations
- **Search** (`/search`) - Search Discogs database
- **Settings** (`/settings`) - Configure Discogs and YouTube connections

## Project Structure

```
vinylfo/
├── main.go                # Application entry point
├── controllers/           # HTTP request handlers
│   ├── album.go           # Album CRUD operations
│   ├── discogs.go         # Discogs sync & OAuth
│   ├── discogs_helpers.go # Utility functions
│   ├── duration.go        # Duration resolution API
│   ├── playback.go        # Playback session management
│   ├── playlist.go        # Playlist management
│   ├── settings.go        # Settings API
│   ├── track.go           # Track CRUD operations
│   └── youtube.go         # YouTube OAuth and playlist API
├── services/              # Business logic layer
│   ├── album_import.go    # Album import from Discogs
│   ├── sync_progress.go   # Sync progress persistence
│   ├── sync_worker.go     # Sync processing engine
│   ├── duration_resolver.go  # Duration resolution service
│   ├── duration_progress.go  # Resolution progress persistence
│   └── duration_worker.go    # Bulk resolution worker
├── duration/              # Duration resolution clients
│   ├── client.go          # Base client and matching algorithms
│   ├── rate_limiter.go    # Rate limiting for APIs
│   ├── musicbrainz_client.go # MusicBrainz API integration
│   ├── wikipedia_client.go   # Wikipedia API integration
│   ├── lastfm_client.go      # Last.fm API integration
│   ├── youtube_client.go     # YouTube API integration
│   ├── youtube_cache.go      # File-based YouTube results cache
│   └── youtube_oauth_client.go # YouTube OAuth and playlist management
├── models/                # Database models
│   ├── models.go          # Album, Track, Playlist, etc.
│   └── app_config.go      # Application settings
├── discogs/               # Discogs API client
│   ├── client.go          # API client with OAuth
│   ├── rate_limiter.go    # Rate limiting
│   └── review.go          # Data review/comparison
├── sync/                  # Sync state management
│   ├── state.go           # State manager
│   └── legacy.go          # Legacy state wrapper
├── database/              # Database operations
│   ├── migrate.go         # Auto-migration
│   └── seed.go            # Sample data seeding
├── routes/                # Route definitions
│   └── routes.go          # All API routes
├── templates/             # HTML templates
│   ├── header.html        # Shared header template
│   ├── index.html         # Home page
│   ├── youtube.html       # YouTube integration page
│   ├── resolution-center.html  # Duration resolution page
│   └── ...                # Other page templates
└── static/                # Static assets (JS, CSS)
```

## API Reference

### Albums
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/albums` | List all albums (paginated) |
| GET | `/albums/search?q=` | Search albums |
| GET | `/albums/:id` | Get album details |
| GET | `/albums/:id/image` | Get album cover image |
| GET | `/albums/:id/tracks` | Get album tracks |
| POST | `/albums` | Create album |
| PUT | `/albums/:id` | Update album |
| DELETE | `/albums/:id` | Delete album |

### Tracks
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/tracks` | List all tracks (paginated) |
| GET | `/tracks/search?q=` | Search tracks |
| GET | `/tracks/:id` | Get track details |

### Playback
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/playback/current` | Get current playback state |
| POST | `/playback/start-playlist` | Start playing a playlist |
| POST | `/playback/pause` | Pause playback |
| POST | `/playback/resume` | Resume playback |
| POST | `/playback/skip` | Skip to next track |
| POST | `/playback/previous` | Go to previous track |
| GET | `/playback/history` | Get listening history |

### Playlists
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/sessions/playlist` | List all playlists |
| POST | `/sessions/playlist/new` | Create playlist |
| GET | `/sessions/playlist/:id` | Get playlist details |
| DELETE | `/sessions/playlist/:id` | Delete playlist |
| POST | `/sessions/playlist/:id/tracks` | Add track to playlist |
| DELETE | `/sessions/playlist/:id/tracks/:track_id` | Remove track |
| POST | `/sessions/playlist/:id/shuffle` | Shuffle playlist |

### Discogs Integration
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/discogs/oauth/url` | Get OAuth authorization URL |
| GET | `/api/discogs/oauth/callback` | OAuth callback handler |
| POST | `/api/discogs/disconnect` | Disconnect Discogs account |
| GET | `/api/discogs/status` | Get connection status |
| GET | `/api/discogs/folders` | Get Discogs folders |
| GET | `/api/discogs/search?q=` | Search Discogs |
| POST | `/api/discogs/sync/start` | Start collection sync |
| GET | `/api/discogs/sync/progress` | Get sync progress |
| POST | `/api/discogs/sync/pause` | Pause sync |
| POST | `/api/discogs/sync/resume-pause` | Resume sync |
| POST | `/api/discogs/sync/cancel` | Cancel sync |
| POST | `/api/discogs/refresh-tracks` | Re-sync all tracks |
| GET | `/api/discogs/unlinked-albums` | Find removed albums |
| POST | `/api/discogs/unlinked-albums/delete` | Delete unlinked albums |

### Resolution Center
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/duration/stats` | Get resolution statistics |
| GET | `/api/duration/tracks` | Get tracks needing resolution |
| POST | `/api/duration/resolve/track/:id` | Resolve single track |
| POST | `/api/duration/resolve/album/:id` | Resolve all tracks in album |
| POST | `/api/duration/resolve/start` | Start bulk resolution |
| POST | `/api/duration/resolve/pause` | Pause bulk resolution |
| POST | `/api/duration/resolve/resume` | Resume bulk resolution |
| POST | `/api/duration/resolve/cancel` | Cancel bulk resolution |
| GET | `/api/duration/resolve/progress` | Get bulk resolution progress |
| GET | `/api/duration/review` | Get review queue |
| GET | `/api/duration/review/:id` | Get resolution details |
| POST | `/api/duration/review/:id` | Submit review decision |
| POST | `/api/duration/review/bulk` | Bulk apply/reject |

### YouTube Integration
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/youtube/oauth/url` | Get YouTube authorization URL |
| GET | `/api/youtube/oauth/callback` | OAuth callback handler |
| POST | `/api/youtube/disconnect` | Disconnect YouTube account |
| GET | `/api/youtube/status` | Get connection status |
| POST | `/api/youtube/playlists` | Create YouTube playlist |
| PUT | `/api/youtube/playlists/:id` | Update playlist |
| GET | `/api/youtube/playlists` | List your YouTube playlists |
| DELETE | `/api/youtube/playlists/:id` | Delete playlist |
| POST | `/api/youtube/playlists/:id/videos` | Add video to playlist |
| DELETE | `/api/youtube/playlists/:id/videos/:item_id` | Remove video |
| POST | `/api/youtube/search` | Search YouTube videos |
| POST | `/api/youtube/export-playlist` | Export session to YouTube playlist |

### Settings
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/settings` | Get application settings |
| PUT | `/api/settings` | Update settings |
| POST | `/api/database/reset` | Reset database (keeps config) |
| POST | `/api/database/seed` | Seed sample data |

## Database Models

- **Album**: Vinyl albums with metadata and cover images
- **Track**: Individual tracks linked to albums
- **Playlist**: User-created playlists
- **PlaylistTrack**: Track-playlist associations with ordering
- **PlaybackSession**: Current playback state and queue
- **TrackHistory**: Listening history records
- **SyncProgress**: Discogs sync state for resume capability
- **SyncHistory**: Completed sync records
- **SyncLog**: Sync error logs for troubleshooting
- **DurationResolution**: Track duration resolution attempts and status
- **DurationSource**: Individual source results (MusicBrainz, Wikipedia)
- **DurationResolverProgress**: Bulk resolution progress for resume
- **AppConfig**: Application settings and OAuth credentials (Discogs, YouTube)
- **TrackYouTubeMatch**: YouTube video matches for tracks
- **TrackYouTubeCandidate**: Candidate matches awaiting review

## Configuration

### YouTube Playlist Sync Settings

| Environment Variable | Default | Description |
|----------------------|---------|-------------|
| `YOUTUBE_MATCH_SCORE_THRESHOLD` | 0.6 | Minimum score to consider a match (0.6-0.85 = needs review) |
| `YOUTUBE_AUTO_MATCH_THRESHOLD` | 0.85 | Minimum score to auto-apply match without review |
| `YOUTUBE_MAX_CANDIDATES` | 5 | Number of candidate matches stored per track |
| `YOUTUBE_WEB_SEARCH_ENABLED` | true | Use web search before falling back to YouTube API |
| `YOUTUBE_API_FALLBACK_ENABLED` | true | Fallback to YouTube API if web search insufficient |

### Scoring Weights

The YouTube matching algorithm uses weighted scoring:
- Title similarity: 40%
- Artist similarity: 30%
- Duration proximity: 20%
- Channel name match: 10%

## License

This project is licensed under the MIT License.

## Acknowledgments

- [Discogs](https://www.discogs.com/) for their comprehensive music database and API
- [Gin](https://gin-gonic.com/) web framework
- [GORM](https://gorm.io/) ORM library
