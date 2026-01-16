# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 2026-01-15

### Fixed
- Album count showing incorrect values (Processed and Total both incrementing)
- Sync progress not updating during sync (added saveSyncProgress after each album)
- Pause not actually pausing the sync job (added IsPaused state)
- UI not showing completion when sync finishes
- Leaving sync page and returning not showing resume option

### Added
- Pause/Resume functionality for sync operations
- Manual refresh button for status updates
- Heartbeat to prevent stale sync detection
- Estimated time remaining display
- Current folder display during multi-folder syncs
- Retry logic for connection errors (3 retries)

### Changed
- Removed API rate limit display from sync screen
- Changed poll interval from 1000ms to 500ms
- Total is now set once at start and remains constant

### Backend
- Added PauseSync endpoint (POST /api/discogs/sync/pause)
- Added ResumeSyncFromPause endpoint (POST /api/discogs/sync/resume-pause)
- GetSyncProgress now includes is_paused in response
- **Atomic Album+Track Sync**: Albums now only save to database if all tracks import successfully
  - Uses database transactions with rollback on track failure
  - Creates sync log entry for failed albums/tracks
  - Prevents partial album imports
- **Folder-Based Sync**: Users can now sync by specific Discogs folders or all folders
  - New sync mode options: "Sync All Folders" and "Sync Specific Folder"
  - Added `/api/discogs/folders` endpoint to fetch user's Discogs collection folders
  - Added folder selection UI in sync page
- **API Usage Visibility**: Users can now see remaining API requests during sync
  - Visual API usage bar shows 0-60 requests consumed
  - Color-coded warnings at 70% (yellow) and 90% (red)
  - Updates in real-time during sync progress polling
- **Sync Resume Capability**: Long syncs can now be paused, cancelled, and resumed
  - Sync progress saved to `sync_progress` table in database
  - 30-minute timeout detection marks stale syncs as "paused"
  - "Resume Sync" button appears when incomplete sync detected
  - "Start New Sync" button to discard previous progress
- **Sync Log Tracking**: All sync errors are now logged for troubleshooting
  - New `SyncLog` model tracks failed album/track imports
  - Logs include Discogs ID, album title, artist, error type, and error message

### New API Endpoints
- `GET /api/discogs/folders` - Get user's Discogs collection folders
- `GET /api/discogs/sync/resume` - Check if there's sync progress to resume
- `POST /api/database/seed` - Seed database with sample data (moved from auto-seed)
- `GET /api/discogs/sync/progress` - Now returns saved progress and API remaining

### Database Changes
- Added `SyncLog` model for tracking sync errors
- Added `SyncProgress` model for tracking sync state across requests
- Added `DiscogsFolderID` field to `Album` model to track which folder album came from
- Added `SyncMode` and `SyncFolderID` fields to `AppConfig` model

### Fixed
- **Rate Limiting**: Fixed rate limiter not preventing 429 Too Many Requests errors
  - Added `X-Discogs-Ratelimit-*` header parsing to track actual remaining requests
  - Added 429 response handling with Retry-After header support
  - Rate limiter now waits for window reset when exhausted
  - Extensive debug logging in `sync_debug.log` file
- **Sync Race Conditions**: Fixed potential race conditions in sync state management
  - Added `sync.RWMutex` for thread-safe access to sync state
  - Protected all `syncState` reads/writes with mutex
  - Consistent use of getter/setter/update helper functions

### Changed
- **Database Seeding**: Removed automatic database seeding on startup
  - "Seed Sample Data" button now available on Settings page
  - Prevents accidental data loss on server restart
- **Sync Flow Improvements**:
  - Sync now processes albums automatically without requiring batch review confirmation
  - Removed batch review UI from sync flow (still available but not used by default)
  - Sync saves progress every batch and updates last_activity timestamp
  - Backend uses `gin.New()` with explicit `gin.Recovery()` and `gin.Logger()` middleware
- **API Rate Limits**: Rate limiter starts with 60 authenticated requests, 25 anonymous

### UI Improvements
- Sync page now shows folder selection when "Sync Specific Folder" is chosen
- API usage bar appears during sync with request count and progress bar
- Paused sync dialog shows folder name, progress, and last activity time
- Cancel sync button now prompts for confirmation
- Sync mode info displays current sync operation (all folders or specific folder)

### Removed
- Automatic database seeding on application startup (now manual via Settings page)

### New Files
- `progress_tracking.go` - Sync progress save/load functions with stale sync detection

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
