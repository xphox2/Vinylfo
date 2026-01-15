# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 2026-01-15

### Added
- **Enhanced Discogs Album Import**: Full album metadata and track information now captured when importing from Discogs
  - Label, Country, Release Date, and Style fields now imported
  - Track position (A1, A2, B1, B2, etc.) preserved with format conversion (1-1 → A1, 2-1 → B1)
  - Duration converted from string format (e.g., "3:45") to seconds for consistent storage
  - Disc and Side tracking for multi-disc albums

### New API Endpoints
- `GET /api/discogs/albums/:id` - Preview album details from Discogs without saving

### Database Changes
- Added new fields to `Album` model: `Label`, `Country`, `ReleaseDate`, `Style`, `DiscogsID`
- Added new fields to `Track` model: `DiscNumber`, `Side`, `Position`

### Changed
- **Discogs Import Flow**: Albums are now previewed first, then confirmed before saving to database
- Search results button changed from "Add" to "View" for clarity
- Album detail modal shows all metadata (label, country, release date, style) before adding
- Validation prevents adding albums with no track information

### Fixed
- **OAuth Endpoint URL**: Fixed OAuth request token endpoint from `www.discogs.com/oauth/request_token` to `api.discogs.com/oauth/request_token`
- **OAuth Signature Whitespace**: Added trimming of whitespace from consumer key, secret, and callback URL to prevent signature errors
- **Disconnect Flow**: Updated disconnect response to inform users they must also revoke access at https://www.discogs.com/settings/applications
- Modal close buttons (X, Cancel, Add to Collection) now work correctly
- Search result items properly centered with image
- Modal album info section centered with larger 180px cover image
- Long style text wraps properly to prevent modal overflow
- Fixed discogs_id type issue (string vs int) when adding albums

### UI Improvements
- Search results: Album art, text, and View button properly centered and aligned
- Modal: Larger centered album art (180px) with side-by-side metadata display
- Track list shows position codes and formatted duration (MM:SS)
- View button constrained to prevent stretching

---

## [1.2.0] - 2026-01-15

### Added
- **Discogs Data Sync Feature**: Full Discogs API integration for syncing vinyl collection
  - OAuth 1.0a authentication flow with Discogs
  - Rate limiting (60 req/min auth, 25 req/min anonymous)
  - Batch collection sync with review before import
  - Discogs search functionality
  - Manual album entry for non-Discogs items
  - Data review system with severity levels (info, warning, conflict)

### New Pages
- `/settings` - Configure Discogs connection and sync preferences
- `/sync` - Sync Discogs collection with batch review
- `/search` - Search Discogs database and add albums

### New API Endpoints
- `GET /api/discogs/oauth/url` - Get OAuth authorization URL
- `GET /api/discogs/oauth/callback` - Handle OAuth callback
- `POST /api/discogs/disconnect` - Disconnect Discogs account
- `GET /api/discogs/status` - Get connection status
- `GET /api/discogs/search?q=` - Search Discogs database
- `POST /api/discogs/albums` - Create album from Discogs or locally
- `POST /api/discogs/sync/start` - Start batch collection sync
- `GET /api/discogs/sync/progress` - Get sync progress
- `GET /api/discogs/sync/batch/:id` - Get batch details for review
- `POST /api/discogs/sync/batch/:id/confirm` - Confirm and sync batch
- `POST /api/discogs/sync/batch/:id/skip` - Skip current batch
- `POST /api/discogs/sync/cancel` - Cancel running sync
- `GET /api/settings` - Get application settings
- `PUT /api/settings` - Update application settings

### New Files
- `models/app_config.go` - AppConfig model for settings
- `discogs/client.go` - Discogs API client with OAuth and rate limiting
- `discogs/review.go` - Data review service for comparing local vs Discogs data
- `controllers/discogs.go` - Discogs API controller
- `controllers/settings.go` - Settings API controller
- `static/settings.html` - Settings page
- `static/js/settings.js` - Settings page JavaScript
- `static/css/settings.css` - Settings page styles
- `static/sync.html` - Sync dashboard page
- `static/js/sync.js` - Sync dashboard JavaScript
- `static/css/sync.css` - Sync dashboard styles
- `static/search.html` - Search page
- `static/js/search.js` - Search page JavaScript
- `static/css/search.css` - Search page styles

### Database Changes
- Added `AppConfig` table for storing settings and Discogs credentials
- Added upsert logic to ensure exactly one config row exists

### Configuration
- Added Discogs OAuth environment variables to `.env`

### Changed
- Updated main theme styling for consistency across all pages
- Unified navigation bar styling with `.main-nav` class
- Removed theme selection from settings (single light theme)

### Fixed
- CSS styling now matches main theme across all new pages

### Changed
- Simplified playback restore (removed resume modal, auto-restore from state)
- Improved button state management (Play/Pause buttons enable/disable correctly)
- Dashboard now auto-restores playback state on page load

### Fixed
- **currentPlaylistId not persisting**: Fixed by using localStorage instead of session variable
- **Queue showing "Unknown"**: Fixed backend to return full track objects with album info instead of just track IDs
- **Pause button not working**: Backend now properly saves state and returns updated status
- **Previous/Next buttons not working**: Backend now properly navigates queue and returns updated track info
- **Duplicate code issues in playlist.js**: Cleaned up and rewrote file to remove duplicate function calls
- **JavaScript scoping issues**: Made currentPlaylistId global via window object

### Backend Changes
- `GET /playback/current`: Now returns full track objects with album info instead of just track ID string
- `POST /playback/pause`: Saves state to database, returns updated status
- `POST /playback/resume`: Returns updated status
- `POST /playback/skip`: Navigates to next track, returns new track and updated queue
- `POST /playback/previous`: Navigates to previous track, returns new track and updated queue
- `GET /playback/state`: Returns full track objects with album info for queue tracks
- `POST /playback/start-playlist`: Returns full track objects with album info for queue tracks

---

## [1.1.0] - 2026-01-14

### Added
- **Playlist Playback Feature**: Users can now play playlists from the dashboard
- **Resume Playback**: Playback position is saved automatically and restored on next visit
- **Queue Display**: Shows upcoming tracks with toggle button to show/hide
- **Progress Saving**: Saves progress every 5 seconds and on page close
- **Playback Controls**: Play, Pause, Previous, Next, Stop buttons
- **Listening History**: Tracks are logged for future history display

### Features
- Start playback from any playlist
- See queue of upcoming tracks
- Navigate through queue (next/previous)
- Progress bar updates in real-time
- State persists across browser sessions

### API Endpoints Added
- `POST /playback/start-playlist` - Start playlist with queue
- `POST /playback/update-progress` - Save playback progress
- `GET /playback/state` - Get current state for resume
- `GET /playback/current` - Get current track info
- `POST /playback/pause` - Pause playback
- `POST /playback/resume` - Resume playback
- `POST /playback/skip` - Skip to next track
- `POST /playback/previous` - Go to previous track
- `POST /playback/stop` - Stop playback
- `GET /playback/history` - Get listening history
- `GET /playback/history/:track_id` - Get specific track history

### Database Changes
- Extended `PlaybackSession` model with playlist and queue fields
- Added `TrackHistory` model for tracking play history

### UI Components
- `playback-dashboard.html`: Dashboard with track info, controls, and queue
- `playback-dashboard.css`: Styles for dashboard, modal, and queue panel
- `playback-dashboard.js`: Complete playback management with PlaybackManager class

---

## [1.0.0] - 2026-01-13

### Added
- Playlist management UI with full CRUD functionality
- Create, view, delete playlists
- Add/remove tracks from playlists
- Drag-and-drop track reordering within playlists
- Shuffle playlist functionality
- Play playlist (navigates to dashboard)
- Dedicated track detail page (`/track/:id`)
- Track detail page displays: title, album, duration, genre, release year
- API endpoint for track details with album join (`GET /tracks/:id`)
- Navigation handling for hash-based routing
- Confirmation dialogs for delete operations

### Changed
- Updated getTrackByID API to include album information (album_title, release_year, album_genre)
- Removed track number from track detail display
- Unified styling across all pages (playlist, track detail)
- Improved drag-and-drop implementation for better reliability

### Fixed
- Navigation URL appending bug (hash links now properly navigate)
- Initial hash load not calling loadTracks() for #tracks view
- Track removal using correct field name (id instead of track_id)
- Drag and drop event listeners properly attached to dynamically created elements
- JavaScript function ordering (createTrackListItem defined before use)
- Tracks now preserve order after drag-and-drop reorder
- Session loading bug by adding missing API endpoints

### Removed
- Duplicate code in app.js
- Unused playlist.css file (now uses shared style.css)

---

## [0.1.0] - 2026-01-12

### Added
- Initial project setup
- Database schema for albums, tracks, playback sessions
- Discogs API client
- Frontend dashboard components
- Playback timer functionality
- Collection management interface

[Unreleased]: https://github.com/yourusername/vinylfo/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/yourusername/vinylfo/releases/tag/v1.1.0
[1.0.0]: https://github.com/yourusername/vinylfo/releases/tag/v1.0.0
[0.1.0]: https://github.com/yourusername/vinylfo/releases/tag/v0.1.0
