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
- **Search** (`/search`) - Search Discogs database
- **Settings** (`/settings`) - Configure Discogs connection

## Project Structure

```
vinylfo/
├── main.go                # Application entry point
├── controllers/           # HTTP request handlers
│   ├── album.go           # Album CRUD operations
│   ├── discogs.go         # Discogs sync & OAuth (~1,674 lines)
│   ├── discogs_helpers.go # Utility functions
│   ├── playback.go        # Playback session management
│   ├── playlist.go        # Playlist management
│   ├── settings.go        # Settings API
│   └── track.go           # Track CRUD operations
├── services/              # Business logic layer
│   ├── album_import.go    # Album import from Discogs
│   ├── sync_progress.go   # Sync progress persistence
│   └── sync_worker.go     # Sync processing engine
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
- **AppConfig**: Application settings and OAuth credentials

## License

This project is licensed under the MIT License.

## Acknowledgments

- [Discogs](https://www.discogs.com/) for their comprehensive music database and API
- [Gin](https://gin-gonic.com/) web framework
- [GORM](https://gorm.io/) ORM library
