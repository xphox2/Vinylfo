# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.2-alpha] - 2026-01-17

### Changed

#### Code Refactoring: Discogs Controller
- **Extracted utility functions to `controllers/discogs_helpers.go`** (~80 lines)
  - `intPtr()` - Helper for creating int pointers
  - `downloadImage()` - Downloads and validates images from URLs
  - `logToFile()` - Debug logging utility for sync operations
  - `isLockTimeout()` - Database lock timeout detection
  - `maskValue()` - Masks sensitive values for logging

- **Created new `services/` package** with focused, testable modules:
  - `services/album_import.go` (~280 lines) - Album import service
    - `AlbumImporter` struct for handling album creation from Discogs
    - `DownloadCoverImage()` - Image downloading with validation
    - `CreateAlbumWithTracks()` - Album creation with associated tracks
    - `FetchAndSaveTracks()` - Fetch and persist tracks from Discogs API
    - `ImportFromDiscogs()` - Full album import by Discogs ID

  - `services/sync_progress.go` (~120 lines) - Sync progress persistence
    - `SyncProgressService` for database persistence
    - `Load()` / `Save()` - Progress state management
    - `ArchiveToHistory()` - Move completed syncs to history
    - `RestoreLastBatch()` - Restore batch from database on resume
    - `Clear()` / `Delete()` - Progress cleanup operations

  - `services/sync_worker.go` (~480 lines) - Sync processing engine
    - `SyncWorker` struct replacing monolithic `processSyncBatches()` function
    - `Run()` - Main sync loop with proper error handling
    - `handlePagination()` - Pagination and folder transition logic
    - `fetchNextBatch()` - API fetch with rate limit handling
    - `processAlbum()` - Single album processing
    - `createNewAlbum()` / `updateExistingAlbum()` - Album CRUD operations
    - `checkPauseState()` - Pause detection and wait loop
    - `markComplete()` - Completion handling and cleanup

- **Slimmed down `controllers/discogs.go`** from ~2,538 to ~1,674 lines
  - Now a thin HTTP layer that delegates to services
  - Controllers handle request parsing, validation, and response formatting
  - Business logic moved to service layer
  - Improved separation of concerns and testability

### Technical Details
- No behavior changes - this is a structural refactoring only
- All existing tests continue to pass
- Backward compatible with existing database schema
- The `sync/` package (state.go, legacy.go) remains unchanged

---

## [0.1.1-alpha] - 2026-01-17

### Added

- **Player Page Renamed from Dashboard**: The playback page is now accessible at `/player` instead of `/dashboard`
  - Updated route in main.go
  - Updated navigation link in header.html
  - Updated playlist.js redirect after starting playback

- **Queue Pagination on Player Page**: Added pagination to the queue display
  - Shows 25 tracks per page
  - Prev/Next buttons with page info
  - Consistent styling with albums/tracks pages

- **Queue Track Duration**: Track duration is now displayed on the queue
  - Shown on the right side of each queue item
  - Hidden on small mobile screens

- **Bottom Pagination on Albums Page**: Added pagination controls at the bottom of albums list
  - Works identically to top pagination controls
  - Uses shared JavaScript event listeners via CSS classes

### Changed

- **Playlist Management No Longer Paginated**: Removed pagination from playlist detail view
  - All tracks now load at once for easier management
  - Album removal works correctly across all tracks
  - Backend returns all tracks (limit: 100000)

- **Add Album to Playlist**: New "Album" button next to "Add" button allows adding all tracks from an album at once
  - Skips tracks already in the playlist
  - No confirmation dialog
  - Backend endpoint: `/albums/{albumId}/tracks` (already existed)

- **Remove Album from Playlist**: New "Album" button next to "Remove" button allows removing all tracks from an album at once
  - Shows confirmation dialog with number of tracks to remove
  - Removes all tracks from that album in the playlist
  - No popup when adding/removing albums

### Changed

- **Unified Track Display Format**: Add Tracks view now matches Playlist detail view format
  - Album cover image on left
  - Bold track title with artist below
  - Album title and duration on right
  - Add button on far right

- **Removed Single Track Removal Confirmation**: Clicking "Remove" on a single track now removes it immediately without confirmation dialog

- **Simplified JavaScript Structure**: Removed nested IIFE wrapper from playlist.js
  - Single DOMContentLoaded handler instead of IIFE + DOMContentLoaded
  - Prevents syntax errors when editing event listener section

### Fixed

- **Dashboard "Unknown Artist"**: Added `album_artist` field to `buildTrackResponse()` in playback controller
  - Fixed "Unknown Artist" display when playing tracks

- **Add/Album Buttons Not Working**: Fixed playlist.js to call `savePlaylistId()` when viewing a playlist
  - This ensures `window.currentPlaylistId` is set when adding tracks
  - Previously the ID was only saved to localStorage but not to the global variable

- **Album Removal Not Working**: Removed pagination limit from playlist management
  - Was causing only first 25 tracks to be loaded, missing album tracks on other pages
  - Playlist detail now loads all tracks at once for proper management

- **Dashboard "Playlist: -"**: Added `playlist_name` to playback state responses
  - Backend returns `playlist_name` in `/playback/current` and `/playback/start-playlist`
  - Frontend stores and displays playlist name on dashboard

- **Play Button on Playlist Detail**: Added missing event listener for play button
  - `playPlaylist()` function now properly called when clicking "Play"

- **Add Tracks Page Formatting**: Fixed "Add Tracks to Playlist" page layout
  - Wrapped title and search in proper `view-header` container
  - Matches structure of albums/tracks pages for consistent styling

- **Back to Playlists Button**: Added missing `clearPlaylistId()` function definition
  - Button now navigates back to playlists list properly

- **Delete Playlist Error Handling**: Improved error handling in `deletePlaylist()`
  - Handles non-JSON responses gracefully

- **Add/Remove Album Refresh**: Available tracks list now refreshes after adding/removing albums
  - Backend filtering prevents showing tracks already in playlist
  - Pagination recalculates based on filtered results

---

## [0.1.0-alpha] - 2026-01-17

### Added

#### Sync Improvements
- **Refresh Tracks Button**: New button to re-sync track listings from Discogs for all existing albums
  - Fetches latest tracklist from Discogs API
  - Removes tracks deleted on Discogs, adds new tracks
  - Useful when track metadata changes on Discogs
  - Endpoint: `POST /api/discogs/refresh-tracks`

- **Cleanup Unlinked Albums**: New feature to find and remove albums no longer in Discogs collection
  - "Cleanup" button on Sync page opens review modal
  - Scans entire Discogs collection and compares to local albums
  - Shows list of albums with checkboxes for selective deletion
  - Deletes albums and their tracks safely
  - Endpoints: `GET /api/discogs/unlinked-albums`, `POST /api/discogs/unlinked-albums/delete`

- **Album Metadata Updates on Re-sync**: Existing albums now get updated when re-syncing
  - Updates DiscogsID if previously missing (links manual entries to Discogs)
  - Updates folder ID if changed
  - Updates cover image if different
  - Updates release year if previously missing

- **Local Database Search**: Users can now search albums and tracks directly in the local database
  - Search bar on Albums page filters by title and artist
  - Search bar on Tracks page filters by track title, album title, and artist
  - 300ms debounce for live search as you type
  - Clear (X) button to quickly reset search
  - Search persists when switching between views

- **Playlist Management Improvements**
  - Added search bar to Add Tracks view for filtering available tracks
  - Added pagination to Add Tracks view (25, 50, 100 items per page)
  - Improved playlist list display with proper date formatting
  - Fixed playlist creation and track addition functionality

- **UI Improvements**
  - Fixed radio button alignment on Sync page (changed from vertical to horizontal layout)
  - Unified Start Sync button styling to match Refresh and Cleanup buttons
  - Fixed Tracks page formatting to match Albums page style
  - Added album cover images to Tracks page with placeholder fallback
  - Fixed playlist page event listeners for Create Playlist functionality

- Pause/Resume functionality for sync operations
- Manual refresh button for status updates
- Heartbeat to prevent stale sync detection
- Estimated time remaining display
- Current folder display during multi-folder syncs
- Retry logic for connection errors (3 retries)

### Fixed

#### Sync Resume Bug Fix
- **Fixed sync re-fetching same page after resume**: Resume was clearing `LastBatch` causing the worker to re-fetch the same page from API instead of continuing with remaining albums
  - Now properly restores `LastBatch` from database using `restoreLastBatch()`
  - Prevents wasted API calls and rate limit exhaustion on resume
  - Fixed in both paused-state and stopped-state resume paths

#### Stall Detection Fix
- **Fixed false "stalled" detection during rate limit waits**: Stall timeout was 30s but API rate limit reset is 60s
  - Increased stall detection timeout from 30s to 90s
  - Frontend now shows "Waiting for API rate limit reset..." instead of stopping
  - Frontend continues polling instead of giving up when stalled

#### Album Count Display Fix
- **Fixed "142/141 albums" overcounting display**: Progress could show processed > total
  - Frontend now caps displayed count at total (e.g., shows "141/141" not "142/141")
  - Backend now updates `Total` from API response on each page fetch
  - Handles cases where collection size changes during sync

#### Discogs Sync Pause/Resume Fixes

- **PAUSE-001: LastBatch Not Persisted for Resume**
  - Added `LastBatchJSON` field to `SyncProgress` model to persist batch data
  - Sync goroutine now saves batch state to database during pause
  - Resume now restores mid-batch progress instead of losing albums
  - Added `restoreLastBatch()` function to deserialize batch data on resume
  - Backend can now resume from exact point, even mid-page

- **PAUSE-002: Pause Timeout Too Short**
  - Changed timeout from 30 minutes to 4 hours when sync is paused
  - Running sync still uses 30-minute timeout for crash detection
  - Allows overnight pauses without losing progress

- **PAUSE-003: Resume Not Working After Page Refresh**
  - Fixed `ResumeSyncFromPause()` to handle both paused and stopped states
  - Added folder fetching on resume for "all-folders" mode
  - Fixed `GetSyncProgress` to load folders from state if available
  - Added `Processed` and `Total` count restoration on resume

- **PAUSE-004: Race Conditions in Sync Loop**
  - Refactored `processSyncBatches()` to use consistent state throughout iteration
  - Removed redundant `getSyncState()` calls that could return different values
  - Added `IsPaused` check after API calls to detect pause during network operations
  - State now captured once per loop and used for all subsequent checks

#### Track Management Fixes

- **TRACK-001: Tracks Not Removed When Re-syncing Album**
  - Modified `fetchTracksForAlbum()` to delete existing tracks before creating new ones
  - Ensures tracks removed from Discogs are also removed from local database
  - Prevents orphaned tracks when album metadata changes on Discogs

- **TRACK-002: Reset Database Missing sync_progresses Table**
  - Added `sync_progresses` tableDatabase function
  - Fixed table name: to Reset GORM pluralizes `SyncProgress` to `sync_progresses`
  - All data tables now cleared except `app_configs` (OAuth preserved)

#### Frontend Improvements

- **UI-001: Pause/Resume Debug Logging**
  - Added console.log statements in sync.js for pause/resume debugging
  - Logs API calls, responses, and progress updates
  - Open browser DevTools (F12) > Console to trace issues

- **UI-002: Improved Paused State Display**
  - Shows "Paused at X/Y albums" when sync is paused
  - Ensures polling restarts properly after resume
  - Button state synced with actual sync state

- **Discogs Search with Spaces**: Fixed OAuth 1.0 signature generation for GET requests with query parameters containing spaces
  - Added custom percentEncodeValue() function to properly encode spaces as %20 instead of +
  - OAuth signature now correctly matches Discogs API requirements
  - Fixed 401 "You must authenticate to access this resource" error on search queries with spaces
- **Discogs Track Import**: Fixed tracks being imported with AlbumID=0
  - Tracks now stored temporarily and created after album is saved
  - Album ID properly assigned to all imported tracks
- Sync progress not updating during sync (added saveSyncProgress after each album)
- Pause not actually pausing the sync job (added IsPaused state)
- UI not showing completion when sync finishes
- Leaving sync page and returning not showing resume option

### Changed

#### Database Schema Changes
- **DiscogsID field changed to pointer**: `DiscogsID` is now `*int` instead of `int`
  - Allows NULL values for manual entries (no Discogs source)
  - Added unique index on DiscogsID to prevent duplicate Discogs albums
  - Migration automatically handles existing data

- **Album unique constraint fixed**: Changed from title-only to title+artist composite
  - Allows albums with same title by different artists (e.g., "Greatest Hits")
  - Old index dropped automatically on startup
  - New composite index: `uniqueIndex:idx_title_artist`

#### Duplicate Detection Improvements
- **Improved duplicate checking**: Now checks DiscogsID first, falls back to title+artist
  - More reliable matching for Discogs albums
  - Manual entries still matched by title+artist
  - Existing manual entries get linked to DiscogsID when synced

- **OAuth-Only Authentication**: Removed DISCOGS_API_TOKEN support and transitioned to OAuth-only authentication
  - Removed DISCOGS_API_TOKEN environment variable from .env
  - Updated getDiscogsClient() to create clients without API key
  - Updated PreviewAlbum and CreateAlbum endpoints to use getDiscogsClientWithOAuth()
  - OAuth URL generation now loads consumer credentials from database config first

- Removed API rate limit display from sync screen
- Changed poll interval from 1000ms to 500ms
- Total is now set once at start and remains constant

- **Database Seeding**: Removed automatic database seeding on startup
  - "Seed Sample Data" button now available on Settings page
  - Prevents accidental data loss on server restart

- **Sync Flow Improvements**:
  - Sync now processes albums automatically without requiring batch review confirmation
  - Removed batch review UI from sync flow (still available but not used by default)
  - Sync saves progress every batch and updates last_activity timestamp
  - Backend uses `gin.New()` with explicit `gin.Recovery()` and `gin.Logger()` middleware

- **API Rate Limits**: Rate limiter starts with 60 authenticated requests, 25 anonymous

- Added `LastBatchJSON` field to `SyncProgress` model for batch persistence
- Modified `saveSyncProgress()` to serialize and save LastBatch to database
- Modified `loadSyncProgress()` to restore LastBatch on resume
- Extended timeout for paused syncs from 30 minutes to 4 hours
- Added `restoreLastBatch()` function for batch deserialization
- Added folder restoration on resume for all-folders sync mode
- Added console logging for frontend debugging (visible in browser DevTools)

### New API Endpoints
- `POST /api/discogs/refresh-tracks` - Re-fetch tracks for all albums with DiscogsID
- `GET /api/discogs/unlinked-albums` - Find albums not in Discogs collection
- `POST /api/discogs/unlinked-albums/delete` - Delete selected unlinked albums
- `GET /api/discogs/folders` - Get user's Discogs collection folders
- `GET /api/discogs/sync/resume` - Check if there's sync progress to resume
- `POST /api/database/seed` - Seed database with sample data (moved from auto-seed)
- `GET /api/discogs/sync/progress` - Now returns saved progress and API remaining
- `POST /api/discogs/sync/pause` - Pause sync operation
- `POST /api/discogs/sync/resume-pause` - Resume from paused state

### UI Changes
- Added "Refresh Tracks" button on Sync page
- Added "Cleanup" button on Sync page
- Added cleanup modal with album list and checkboxes
- Added sync hint text explaining button functions
- Improved rate limit wait messaging in progress display
- Sync page now shows folder selection when "Sync Specific Folder" is chosen
- API usage bar appears during sync with request count and progress bar
- Paused sync dialog shows folder name, progress, and last activity time
- Cancel sync button now prompts for confirmation
- Sync mode info displays current sync operation (all folders or specific folder)

### Backend Changes
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

### Database Changes
- Added `last_batch_json` column to `sync_progresses` table
- Added `sync_progresses` table to ResetDatabase cleanup list
- Added `SyncLog` model for tracking sync errors
- Added `SyncProgress` model for tracking sync state across requests
- Added `DiscogsFolderID` field to `Album` model to track which folder album came from
- Added `SyncMode` and `SyncFolderID` fields to `AppConfig` model

### Removed
- Automatic database seeding on application startup (now manual via Settings page)

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

---

## [0.0.3-alpha] - 2026-01-15

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

## [0.0.2-alpha] - 2026-01-14

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

## [0.0.1-alpha] - 2026-01-13

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

## [0.0.0-alpha] - 2026-01-12

### Added
- Initial project setup
- Database schema for albums, tracks, playback sessions
- Discogs API client
- Frontend dashboard components
- Playback timer functionality
- Collection management interface

[0.1.2-alpha]: https://github.com/yourusername/vinylfo/releases/tag/v0.1.2-alpha
[0.1.1-alpha]: https://github.com/yourusername/vinylfo/releases/tag/v0.1.1-alpha
[0.1.0-alpha]: https://github.com/yourusername/vinylfo/releases/tag/v0.1.0-alpha
