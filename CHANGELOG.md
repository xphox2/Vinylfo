# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

### Fixed

#### Discogs Sync Rate Limiting and Stall Issues

- **Fixed sync not auto-resuming after rate limit**: Implemented proper rate limit pause/resume functionality
  - Added global rate limiter singleton with `IsRateLimited()` and `ClearRateLimit()` methods
  - Added callback mechanism that triggers when Discogs API returns 429 (Too Many Requests)
  - Sync worker sets up callback to pause sync state when rate limited
  - Backend now reports `is_rate_limited` flag in progress response
  - Frontend automatically resumes sync when rate limit clears
  - Files: `discogs/rate_limiter.go`, `discogs/client.go`, `services/sync_worker.go`, `controllers/discogs_sync_progress.go`, `static/js/sync.js`

- **Fixed sync slowing down due to request pileup**: Implemented request deduplication for progress polling
  - Added `pollInProgress` flag to prevent multiple concurrent polling requests
  - Polling now skips if previous request is still in flight
  - Prevents backlog of requests when server responses are slow
  - Fixes issue with multiple "pollProgress called" messages without corresponding "received" messages
  - File: `static/js/sync.js`

- **Fixed false stall detection during slow operations**: Updated activity timestamp during album processing
  - `LastActivity` now updates after each album is processed (not just per batch)
  - Prevents stall detection during slow image downloads
  - More accurate progress tracking
  - Files: `services/sync_worker.go`, `controllers/discogs_sync_progress.go`

- **Increased stall detection timeout**: Adjusted timing for better accuracy
  - Backend stall timeout increased from 90 seconds to 180 seconds
  - Frontend stall detection count increased from 3 to 6 iterations
  - Gives more time for slow operations like cover image processing
  - Files: `controllers/discogs_sync_progress.go`, `static/js/sync.js`

- **Rate limit aware stall detection**: Won't mark as stalled if rate limiter is active
  - Backend checks `discogs.GetGlobalRateLimiter().IsRateLimited()` before marking stalled
  - Prevents confusing "stalled" status when actually waiting on rate limits
  - File: `controllers/discogs_sync_progress.go`

- **Added adaptive polling intervals**: Reduces API calls during slow periods
  - Normal polling: 1 second interval
  - Stalled detection: 5 second interval
  - Automatically switches back to normal when sync resumes
  - File: `static/js/sync.js`

- **Added `resumeSync()` method**: Programmatic resume capability for auto-resume logic
  - Separate method for auto-resume functionality
  - Properly updates UI state and restarts polling
  - File: `static/js/sync.js`

- **Updated frontend validation test**: Added sync.js to JavaScript syntax validation
  - Ensures sync.js is checked for syntax errors in test suite
  - File: `tests/syntax/frontend_validation_test.go`

---

## [0.2.9-alpha] - 2026-01-22

### Fixed

#### Video Feed Race Conditions
- **Fixed duplicate `handleTabSyncSeek` function**: Removed duplicate function definition (lines 91-115 and 430-441) that was causing unpredictable behavior
- **Fixed race conditions in play state handling**: Both `handleTrackUpdate` and `handlePlaybackState` had independent retry loops (up to 20 retries, 100ms each) that could run simultaneously when SSE events arrived in quick succession
  - Created centralized `queuePlayStateOperation()` method that cancels pending operations before starting new ones
  - Reduced retry attempts from 20 to 10, only retrying on track changes
  - Prevents conflicting `playVideo()` and `pauseVideo()` calls hitting the YouTube player simultaneously
- **Added operation queuing**: New `pendingOperation` and `operationTimeout` state variables prevent concurrent state modifications
- File: `static/js/video-feed.js`

#### Auto-Play Not Loading Next Video
- **Fixed next video not loading when auto-play advances**: When a track ended and auto-play was enabled, only `saveProgress()` was called which updated the database but NOT the in-memory `PlaybackManager.currentTrack`
  - The video feed's SSE `stateMonitor` checks `pm.GetCurrentTrack()` (in-memory state), so no `track_changed` event was sent
  - `checkTrackEnd()` now calls `this.next()` which properly calls `/playback/skip` to update both database AND in-memory state
  - Added `advanceAndPause()` method for non-autoplay case (track ends but auto-play disabled)
- File: `static/js/playback-dashboard.js`

### Changed

#### Artist Name Normalization Across All Display Locations
- **Normalized artist names everywhere**: Applied `normalizeArtistName()` consistently across all UI components
  - Removes disambiguation suffixes like `(2)`, `(3)`, `(rapper)`, `(singer)`, `(band)`, `(DJ)`, etc.
  - Example: `"Eminem (2)"` → `"Eminem"`, `"Pink (singer)"` → `"Pink"`
- **Files updated**:
  - `static/js/playback-dashboard.js` - Added `cleanArtistName()` helper, updated 5 display locations
  - `static/js/video-feed.js` - Added `cleanArtistName()` helper, updated track overlay
  - `static/js/app.js` - Imported `cleanArtistName` from app-state.js, updated 5 locations
  - `static/js/app-state.js` - Added and exported `cleanArtistName()` function
  - `static/js/playlist.js` - Added `cleanArtistName()` helper, updated track list
  - `static/js/album-detail.js` - Added `cleanArtistName()` helper, updated album artist display
  - `static/js/track-detail.js` - Added `cleanArtistName()` helper, updated track artist display
  - `static/js/search.js` - Added `cleanArtistName()` method to SearchManager class, updated 2 locations
  - `static/js/playlist-youtube.js` - Added `cleanArtistNameYT()` helper, updated review track info
  - `static/js/sync.js` - Added `cleanArtistName()` method to SyncManager class, updated cleanup modal

---

## [0.2.8-alpha] - 2026-01-21

### Added

#### Video Feed for OBS Streaming
- **New Video Feed Page** (`/feeds/video`): Dedicated page for OBS browser source streaming
  - Shows YouTube music videos synced with your playback queue
  - Designed as a display-only view that mirrors the main player
  - Full viewport video display for streaming overlays

- **Real-Time Sync via Server-Sent Events (SSE)**
  - New SSE endpoint `/feeds/video/events` for instant track updates
  - No polling delay - video changes immediately when track changes
  - Automatic reconnection with exponential backoff
  - Connection status indicator

- **Smart Fallback Display**
  - Album art with Ken Burns effect when no YouTube video available
  - Canvas-based audio visualizer with animated frequency bars
  - Simulated audio visualization (works in OBS without audio input)
  - Theme support for visualizer (dark, light, transparent)

- **Track Info Overlay**
  - Configurable position (top, bottom, or none)
  - Shows track title, artist, and album name
  - Album art thumbnail in overlay
  - Auto-hide with configurable duration
  - Three themes: dark, light, transparent

- **Smooth Video Transitions**
  - Fade transition between tracks (default)
  - Slide transition option
  - No transition option for instant switching

- **URL Parameter Customization**
  - `overlay` - Position: `none`, `bottom`, `top` (default: bottom)
  - `theme` - Color theme: `dark`, `light`, `transparent` (default: dark)
  - `transition` - Effect: `fade`, `slide`, `none` (default: fade)
  - `showVisualizer` - Enable visualizer: `true`, `false` (default: true)
  - `quality` - Video quality: `auto`, `1080`, `720`, `480` (default: auto)
  - `overlayDuration` - Seconds to show overlay, 0 = always (default: 5)

- **Video Preloading**
  - Fetches next track info via `/playback/next-preload`
  - Enables seamless transitions between tracks

#### New API Endpoints
- `GET /feeds/video` - Video feed page for OBS browser source
- `GET /feeds/video/events` - SSE endpoint for real-time track updates
- `GET /playback/current-youtube` - Get current track's YouTube video info
- `GET /playback/next-preload` - Get next track info for preloading
- `POST /playback/video/play` - Play video
- `POST /playback/video/pause` - Pause video
- `POST /playback/video/stop` - Stop video (preserves session)
- `POST /playback/video/next` - Skip to next video
- `POST /playback/video/previous` - Go to previous video
- `POST /playback/video/seek` - Seek video to position

#### New Files
- `controllers/video_feed.go` - Video feed controller with SSE support
- `templates/video-feed.html` - OBS video feed page template
- `static/js/video-feed.js` - Video feed manager with YouTube IFrame API
- `static/js/audio-visualizer.js` - Canvas-based audio visualizer
- `static/css/video-feed.css` - Video feed styles with transitions

### Changed

#### CSP Updates for YouTube Embedding
- Updated Content-Security-Policy for `/feeds/video` routes
- Added YouTube domains to script-src, frame-src, and connect-src
- Allows YouTube IFrame embedding in video feed
- X-Frame-Options set to ALLOWALL for OBS browser source compatibility

### Fixed

#### Video Feed Stop Button Preserving Session
- Stop now pauses and resets position instead of deleting session
- Users can resume playback after stopping
- Session state preserved in database with status "stopped"

#### Video Feed Pause Detection
- State monitor now distinguishes between track changes and play/pause state changes
- Sends dedicated `playback_state` SSE event for pause/play changes
- Improved JavaScript handling for pause state

### Fixed

#### Playlist Playback Session Not Created
- **Fixed `is_playing` always returning false after starting playlist**: `StartPlaylist` was calling `ResumePlayback()` which only works if a session already exists
  - Changed to call `StartPlayback()` which properly creates the session with `IsPlaying: true`
  - This fix enables the video feed pause functionality to work correctly
  - File: `controllers/playback.go`

#### Video Feed Not Receiving Pause Events
- Fixed state monitor not detecting pause state changes because session was never created
- Added debug logging throughout the pause flow for troubleshooting
- Files: `controllers/video_feed.go`, `controllers/playback.go`, `static/js/video-feed.js`

---

## [0.2.7-alpha] - 2026-01-21

### Added

#### YouTube Playlist Sync Improvements

**Web Search Normalization**
- Fixed "Sublime (2)" searching instead of "Sublime" by normalizing track names before web search queries
- Now strips disambiguation suffixes like `(2)`, `(3)`, etc. just like scoring normalization does
- Uses `duration.NormalizeTitle()` and `duration.NormalizeArtistName()` before constructing search queries

**Duration Display in Match Review**
- Duration now shows on Review YouTube Match screen for candidates
- Fixed JavaScript using wrong field name (`candidate.duration` → `candidate.video_duration`)
- Changed `FetchVideoMetadata` to `FetchVideoMetadataWithDuration` to get duration via noembed.com

**Web Cache Management**
- Added "Clear Web Cache" button in YouTube Sync modal
- New API endpoint: `POST /api/youtube/clear-cache`
- Clears `.youtube_web_cache/` folder to force fresh searches
- Useful when cached results have incomplete metadata

**Synced State Button**
- After successful sync, "YouTube Sync" button changes to green "Synced" button
- Clicking "Synced" button opens YouTube Playlist Manager for that playlist
- Default playlist name pre-fills in sync modal with local playlist name

**Manual Match in Count**
- Tracks manually matched (status: "reviewed") now count towards Matched count
- Both "matched" and "reviewed" statuses included in matched count calculation

**Playlist Deletion Cleanup**
- When deleting a playlist, all related YouTube data is cleaned up:
  - `TrackYouTubeMatch` records for tracks in playlist
  - `TrackYouTubeCandidate` records for tracks in playlist
  - `PlaybackSession` record (including YouTube sync info)
  - `SessionPlaylist` entries

**YouTube Playlist Manager Enhancements**
- Click any playlist to view its videos
- Shows video title, channel name, and YouTube thumbnail
- Dynamic video counts (fetches actual count, bypasses stale cached count)
- Back button to return to playlist list
- URL parameter support: `/youtube?playlist_id=xxx&playlist_title=yyy`

### Fixed

**JavaScript Reference Errors**
- Fixed `openYouTubeSyncModal` function defined after use
- Fixed `updateSyncButtonState` function ordering issue
- Fixed `updateSyncButtonDisplay` missing playlist name parameter

**YouTube API Response Parsing**
- Fixed `resourceId` field being parsed as string when it's actually an object
- Added `resourceId` struct and `VideoID` field access via `item.Snippet.ResourceID.VideoID`
- Fixed `ChannelTitle` field access for playlist items

**GUI Bug Fixes**
- Fixed duration not showing on Review YouTube Match screen
- Fixed playlist items not loading (wrong endpoint path)
- Fixed "Untitled / Unknown" display for playlist videos

### Changed

**Updated API Responses**
- `GET /api/youtube/matches/:playlist_id` now returns `youtube_sync` info and `playlist_name`
- `GET /api/youtube/matches/:playlist_id` returns tracks with top-level fields (not nested under `snippet`)
- `PlaylistSyncStatus` now includes `YouTubePlaylistName` field

**Database Schema Changes**
- Extended `PlaybackSession` with `YouTubePlaylistID`, `YouTubePlaylistName`, `YouTubeSyncedAt` columns

### New API Endpoints
- `POST /api/youtube/clear-cache` - Clear web search cache

### Fixed

#### Playback Queue Not Displaying
- **Fixed queue showing null when playing playlists**: The `order` column in `SessionPlaylist` table is a MySQL reserved keyword causing SQL syntax errors
  - Added backtick escaping to all `ORDER BY` clauses using `order` column
  - Fixed in `controllers/playback.go`, `controllers/playlist.go`, `controllers/youtube_sync.go`, `services/youtube_sync_service.go`
  - Queue now properly displays all tracks in playlist

#### Playback Session Restore Failing
- **Fixed 400 Bad Request on session restore**: `io.ReadAll(ctx.Request.Body)` was consuming the request body before `ShouldBindJSON` could parse it
  - Replaced with manual JSON parsing using `json.Unmarshal`
  - Added debug logging to trace request body parsing
  - Session restore now works correctly

#### Playback Dashboard Queue Position Not Restored
- **Fixed queue position not being restored when resuming session**: Frontend was ignoring `queue_position` field in restore response
  - Added `vinylfo_queuePosition`, `vinylfo_queue`, and `vinylfo_queueIndex` to localStorage on restore
  - `loadCurrentPlayback()` now restores queue and position from localStorage if saved
  - Added `queue_position` to `GET /playback/current` response

#### Sessions Page Shows Zero Tracks in Queue
- **Fixed queue count showing 0 for sessions**: Frontend was parsing deprecated `session.queue` JSON field instead of querying `SessionPlaylist` table
  - Updated `GET /sessions` endpoint to return `queue_count` from `SessionPlaylist` table
  - Frontend now displays correct track count from backend

#### Playback Session Not Loading on Player Page
- **Fixed player page not restoring playback state**: `playback-dashboard.js` wasn't using saved playlist ID from localStorage
  - Added code to read `vinylfo_currentPlaylistId` on page load
  - `loadCurrentPlayback()` now includes `playlist_id` query parameter
  - Playback state now restores correctly when navigating to `/player`

#### Resolution Center Display Showing Unnormalized Names
- **Fixed junk like "(2)" and "(Remastered)" showing in resolution center**: UI was displaying raw database values instead of normalized names
  - Added `normalizeArtistName()` and `normalizeTitle()` functions to `static/js/modules/utils.js`
  - Updated resolution-center.js to normalize display of track titles, album titles, and artist names
  - Matches backend normalization logic for consistent display

#### Wikipedia Client Not Using Normalized Names
- **Fixed Wikipedia searches not finding matches due to disambiguation suffixes**: Wikipedia client was searching with raw artist/album names instead of normalized ones
  - Updated `SearchTrack()` to use `NormalizeArtistName()` and `NormalizeTitle()` for album page search
  - Updated `findMatchingTrack()` to normalize both search title and track titles before matching
  - Wikipedia now finds more matches for artists like "Sublime (2)"

---

## [0.2.6-alpha] - 2026-01-20

### Changed

#### YouTube Playlist Manager Page Redesign
- **Simplified Page Layout**: Streamlined `/youtube` page to focus on playlist management
  - Renamed page from "YouTube Integration" to "YouTube Playlist Manager"
  - Removed "Connect Your YouTube Account" section (connection now handled via Settings page)
  - Removed "Recent Uploads" tab - page now only shows user's playlists
  - Removed "Your Channel" section to reduce clutter
  - Cleaner, more focused interface for playlist management
- **Added Delete Functionality**: Each playlist now has a Delete button
  - Click Delete to remove playlist from YouTube (with confirmation prompt)
  - Confirmation dialog shows playlist name before deletion
- **Improved User Experience**:
  - Added loading indicator when refreshing playlists
  - Added retry logic with polling for slow YouTube API responses
  - Shows progress message: "Refreshing playlists (this may take a moment)..."
  - Retries up to 3 times with 1.5 second delays between attempts
  - Graceful fallback with info notification if playlist doesn't appear

### Fixed

#### Security Fixes
- **XSS Vulnerability in OAuth Error Page**: Fixed potential cross-site scripting vulnerability in `oauthErrorHTML()` where error messages were not HTML-escaped
  - Added `htmlEscapeString()` helper function to escape special HTML characters (`&`, `<`, `>`, `"`, `'`)
  - Error messages from OAuth flow (including user-controlled `error` query parameter) are now properly escaped
  - File: `controllers/youtube.go`

#### Bug Fixes
- **Request Body Not Re-readable on API Retry**: Fixed issue where retry after 401 Unauthorized would fail because the request body (an `io.Reader`) had already been consumed
  - Added `makeAuthenticatedRequestWithBytes()` helper that reads body upfront and recreates the reader on retry
  - Ensures POST/PUT requests to YouTube API can be properly retried after token refresh
  - File: `duration/youtube_oauth_client.go`

- **Rate Limiter Test Timeout**: Fixed rate limiter tests that were timing out because they expected `Wait()` to complete quickly when remaining requests were low
  - Tests now set `windowStart` to the past so the rate limit window is already expired
  - Added separate test cases for no-wait and wait scenarios
  - Added threshold boundary testing
  - File: `discogs/rate_limiter_test.go`

---

## [0.2.5-alpha] - 2026-01-20

### Added

#### YouTube OAuth Integration
- **YouTube Account Connection**: New OAuth 2.0 integration for connecting your YouTube account
  - Write vinyl playlists directly to YouTube
  - Create, update, and delete playlists on your YouTube channel
  - Add and remove videos from playlists
  - Search YouTube for videos to add to playlists
  - Export session tracks as YouTube playlists
- **New YouTube Integration Page** (`/youtube`): Dedicated page for YouTube management
  - View your YouTube channel information
  - Browse your YouTube playlists with video count
  - View recent uploads (music search results)
  - Create new YouTube playlists from the UI
  - Tabbed interface for uploads and playlists
  - Click playlists to view their videos
- **New API Endpoints**:
  - `GET /api/youtube/oauth/url` - Get YouTube authorization URL
  - `GET /api/youtube/oauth/callback` - Handle OAuth callback
  - `POST /api/youtube/disconnect` - Revoke YouTube connection
  - `GET /api/youtube/status` - Check connection status
  - `POST /api/youtube/playlists` - Create YouTube playlist
  - `PUT /api/youtube/playlists/:id` - Update playlist
  - `GET /api/youtube/playlists` - List your YouTube playlists
  - `GET /api/youtube/playlists/:id` - Get playlist items
  - `DELETE /api/youtube/playlists/:id` - Delete playlist
  - `POST /api/youtube/playlists/:id/videos` - Add video to playlist
  - `DELETE /api/youtube/playlists/:id/videos/:item_id` - Remove video
  - `POST /api/youtube/search` - Search YouTube videos
  - `POST /api/youtube/export-playlist` - Export session to YouTube playlist
- **Token Management**: OAuth tokens stored securely in database with automatic refresh
- **Navigation**: Added YouTube link to Sync dropdown menu

#### New Files
- `templates/youtube.html` - YouTube integration page template
- `duration/youtube_oauth_client.go` - YouTube OAuth client with token management and playlist operations
- `controllers/youtube.go` - YouTube API controller with all OAuth and playlist endpoints

#### Resolution Center UI Improvements
- **Toggle Source Selection**: Click a source badge to select it (green highlight), click again to unselect
- **Last.fm Display Fix**: Corrected capitalization from "Last.Fm" to "Last.fm" with proper lowercase 'fm'
- **CSS Class Fix**: Fixed source badge class generation to remove dots (was `last.fm`, now `lastfm`)

### Security

#### ENCRYPTION_KEY Moved to Environment Variables
- **Moved from codebase to .env**: The encryption key is no longer hardcoded in the application
- **Required on startup**: Application will now fail to start if ENCRYPTION_KEY is not set
- **Added to .env**: Default 32-byte key added to .env file for development
- **Generate your own key**: Users should replace the default key with a secure key for production
  - Generate with: `openssl rand -hex 32`
  - Key must be exactly 32 bytes (256 bits)

### Changed
- **Settings Page**: Added YouTube connection status and connect/disconnect buttons
- **Database Migration**: Added `youtube_access_token`, `youtube_refresh_token`, `youtube_token_expiry`, `youtube_connected` columns to `app_configs` table

---

## [0.2.4-alpha] - 2026-01-19

### Added

#### Resolution Center UI Improvements
- **Source Selection on Main Page**: Click any source badge (Wikipedia, Last.fm, etc.) on the Needs Review page to select it with visual highlighting
- **Minutes:Seconds Input**: Manual duration entry now accepts minutes and seconds separately (e.g., 3:45) instead of just seconds
- **Rejection Tracking**: Tracks manually applied from Unprocessed and rejected now properly return to Unprocessed; auto-matched tracks rejected return to Needs Review

### Changed

- **Apply Button Behavior**: Apply button now applies the selected source directly without opening the modal; Manual button still opens the modal for manual entry
- **Resolved Queue Display**: Now shows both auto-resolved and manually-applied tracks (status "resolved" or "approved")

### Fixed

- **Debug Code Cleanup**: Removed all debug alerts, console.log statements, and test buttons from the UI
- **API Endpoint**: Fixed manual duration submission for unprocessed tracks using proper endpoint `/api/duration/track/:id/manual`
- **Reject Logic**: Properly handles rejection based on track origin (auto-matched vs manual)

---

## [0.2.3-alpha] - 2026-01-19

### Added

#### Discogs Cross-Reference Timestamp Resolution
- **Cross-Reference Feature**: New fallback mechanism to find track durations from alternative Discogs releases
  - When vinyl releases have no durations (common for vinyl), searches for the same album in other formats (digital, CD)
  - Uses string similarity matching to find matching releases with durations
  - Falls back to alternative release tracks when vinyl source has no timestamps

- **Levenshtein Distance Similarity**: Implemented fuzzy string matching for release comparison
  - `stringSimilarity()` calculates similarity score (0.0 to 1.0) between normalized strings
  - `levenshteinDistance()` computes edit distance between strings
  - `normalizeStringForCompare()` cleans strings for consistent comparison
    - Converts to lowercase, trims whitespace
    - Removes common punctuation: `& - ' " ( ) [ ] : /`
    - Strips leading "the " articles
    - Collapses multiple spaces

- **Title Extraction from Discogs Format**: Handles "Artist - Title" title field format
  - When Discogs search result has empty artist field, extracts artist/title from combined title
  - Enables accurate similarity scoring when artist is embedded in title field

### Changed

- **Similarity Threshold**: Lowered high-similarity match threshold from 0.85 to 0.80
  - Allows more flexible matching for close but not exact title/artist matches
  - Helps match releases with slight title variations (e.g., "Back In Black" vs "Back In Black Tie")

- **Search Query Handling**: Special character sanitization for Discogs searches
  - Replaces "/" characters with spaces to improve search results
  - Example: "AC/DC" now searches as "AC DC" for better Discogs API compatibility

### Fixed

- **Debug Logging**: Improved traceability for cross-reference matching
  - Added match evaluation logging showing title/artist scores and match conditions
  - Logs extracted artist/title when parsing "Artist - Title" format
  - Added search query logging to debug search behavior

- **Dead Code Removal**: Removed duplicate else-if block with identical condition to main match check

---



## [0.2.2-alpha] - 2026-01-18

### Added

#### YouTube API Quota Optimization
- **Early-Exit Consensus Check**: YouTube API is now skipped when free sources (MusicBrainz, Wikipedia, Last.fm) already reach consensus
  - Saves 101 quota units per track when 2+ free sources agree on duration
  - YouTube only called as a fallback when consensus not reached
  - Logged when YouTube is skipped: "Skipping YouTube API - consensus already reached"

- **YouTube Results Caching**: File-based cache for YouTube API results
  - Cache persists in `.youtube_cache/` directory (survives database resets)
  - 30-day TTL on cached entries
  - Caches both successful results AND "not found" results to avoid repeat lookups
  - Uses SHA256 hash of (title|artist|album) as cache key
  - Useful for testing when database is frequently reset

### Changed

- **Duration Resolution Order**: Free sources (MusicBrainz, Wikipedia, Last.fm) are queried first, expensive sources (YouTube) only when needed

### New Files
- `duration/youtube_cache.go` - File-based YouTube results cache

---

## [0.2.1-alpha] - 2026-01-18

### Fixed

#### Duration Resolution Matching Improvements
- **Artist Name Normalization**: Fixed matching failures for artists with Discogs disambiguation suffixes
  - Strips suffixes like `(2)`, `(3)`, `(rapper)`, `(singer)`, `(band)`, `(DJ)`, `(musician)`, `(producer)`, `(artist)`
  - Example: "Machine Gun Kelly (2)" now correctly matches "Machine Gun Kelly" on MusicBrainz
  - Applied to both search queries and match score calculations

- **Title/Album Edition Normalization**: Fixed matching failures for remastered/special editions
  - Strips common edition suffixes that interfere with matching
  - Handles: `(Remastered)`, `(Digital)`, `(Deluxe Edition)`, `(Bonus Track Version)`, `(Anniversary Edition)`, `(Expanded Edition)`, `(Special Edition)`, `(Collector's Edition)`, `(Limited Edition)`, `(Mono Version)`, `(Stereo Mix)`, `(Original Mix)`, `(Selected Works)`, `(Greatest Hits)`, `(Best Of)`, `(Complete)`, `(Enhanced)`, `(Remix)`
  - Example: "Nena (Remastered & Selected Works)" now correctly matches "Nena"
  - Example: "99 Luftballons (Remastered)" now correctly matches "99 Luftballons"
  - Preserves non-edition suffixes like "(Part 1)", "(Live)" that are part of the actual title

- **Wikipedia Template Parsing**: Fixed track listing not being parsed on some album pages
  - Now handles both `| title1 = ...` (with space) and `|title1 = ...` (no space) formats
  - Fixed for both title and length fields
  - Example: Death Cab For Cutie albums now correctly parsed

- **Resolution Retry Logic**: Fixed rescan not retrying previously failed resolutions
  - Now retries both `"failed"` AND `"needs_review"` resolutions on rescan
  - Only skips resolutions with `"resolved"` or `"approved"` status
  - Deletes old resolution and sources before retry to get fresh results

### Changed

- **Match Score Calculation**: Now normalizes both search query and result before comparing
  - Artist names normalized on both sides
  - Track/album titles normalized on both sides
  - Results in higher match scores for equivalent content with different formatting

- **MusicBrainz Queries**: Search queries now use normalized artist/title/album
  - Removes disambiguation and edition suffixes before querying API
  - Improves search result relevance

- **Wikipedia Queries**: Search queries now use normalized artist/album names
  - Removes edition suffixes before searching for album pages
  - Improves album page matching accuracy

---

## [0.2.0-alpha] - 2026-01-18

### Added

#### Track Duration Resolution Feature
- **Automatic Duration Lookup**: New feature to resolve missing track durations (duration = 0) by querying external music databases
- **MusicBrainz Integration**: Queries MusicBrainz database for track durations
  - Uses official MusicBrainz API (no authentication required)
  - Rate limited to 50 requests per minute
  - Proper User-Agent header with project URL
  - Searches by track title, artist, and album for accurate matching
- **Wikipedia Integration**: Queries Wikipedia for track durations from album pages
  - Parses track listing templates from album Wikipedia pages
  - Supports standard {{Track listing}} template format
  - Fallback parsing for alternative table formats
- **Consensus Algorithm**: Multiple sources are queried and compared
  - Tracks require 2+ sources agreeing on duration for auto-apply
  - Tolerance of 3 seconds for duration matching
  - Sources with different durations go to review queue
- **Background Bulk Resolution**: Process all tracks with missing durations
  - Starts via API endpoint or web UI
  - Progress saved to database for resume capability
  - Can be paused, resumed, and cancelled
  - Rate limiting prevents API throttling

#### New Database Models
- **DurationSource**: Stores individual source query results
  - Links to DurationResolution via foreign key
  - Stores source name, duration, confidence, match score
  - Includes external ID and URL for verification
- **DurationResolution**: Tracks resolution attempts for each track
  - Aggregates results from multiple sources
  - Tracks status: in_progress, resolved, needs_review, failed, approved, rejected
  - Stores consensus count and auto-apply status
- **DurationResolverProgress**: Persists bulk resolution state
  - Enables resume after pause/cancel
  - Tracks total, processed, resolved, needs_review, failed counts
  - Includes timeout detection for stalled operations

#### New API Endpoints
- `GET /api/duration/stats` - Get duration resolution statistics
- `GET /api/duration/tracks` - Get tracks needing resolution with pagination
- `POST /api/duration/resolve/track/:id` - Resolve single track duration
- `POST /api/duration/resolve/album/:id` - Resolve all tracks in album
- `POST /api/duration/resolve/start` - Start bulk resolution
- `POST /api/duration/resolve/pause` - Pause running resolution
- `POST /api/duration/resolve/resume` - Resume paused resolution
- `POST /api/duration/resolve/cancel` - Cancel running resolution
- `GET /api/duration/resolve/progress` - Get bulk resolution progress
- `GET /api/duration/review` - Get review queue with sources
- `GET /api/duration/review/:id` - Get detailed resolution with all sources
- `POST /api/duration/review/:id` - Submit review decision (apply, reject, manual)
- `POST /api/duration/review/bulk` - Bulk apply/reject resolutions

#### New Web UI
- **Resolution Center Page** (`/resolution-center`): New dedicated page for managing duration resolution
  - Statistics dashboard showing missing, resolved, and needs_review counts
  - Bulk resolution controls (Start, Pause, Resume, Cancel, Refresh)
  - Progress bar with real-time status updates
  - Review queue with pagination
  - Source badges showing duration from each provider
  - Modal dialog for detailed review with source comparison
  - Apply highest confidence or reject all bulk actions

#### New Files
- `models/duration_resolution.go` - DurationResolution, DurationSource, DurationResolverProgress models
- `duration/client.go` - Base client interface and Levenshtein string similarity
- `duration/rate_limiter.go` - Sliding window rate limiter
- `duration/musicbrainz_client.go` - MusicBrainz API integration
- `duration/wikipedia_client.go` - Wikipedia API integration
- `services/duration_resolver.go` - Core resolution service with consensus logic
- `services/duration_progress.go` - Progress persistence for resume
- `services/duration_worker.go` - Background worker for bulk processing
- `controllers/duration.go` - REST API controller for duration endpoints
- `templates/resolution-center.html` - Resolution Center page template
- `static/css/resolution-center.css` - Resolution Center page styles
- `static/js/resolution-center.js` - Resolution Center page JavaScript

### Fixed

#### Wikipedia Track Matching Bug
- **Fixed Wikipedia not finding tracks due to wiki markup parsing errors**
  - Title/length values were incorrectly including `= ` prefix from wiki template syntax (e.g., `= 3:57` instead of `3:57`)
  - Added `=` sign removal when extracting title and length values from Wikipedia track listing templates
  - Duration parser now correctly parses time strings

#### Wikipedia Link Cleanup Bug
- **Fixed `cleanWikiMarkup()` not handling incomplete wiki links**
  - Wiki links like `[[Target|Display` without closing `]]` were not being cleaned
  - Rewrote function with three-phase approach: process complete links first, then handle incomplete links
  - Now correctly extracts display text from links like `[[All of You (song)|All of You]]` → `All of You`

#### Wikipedia Sources Not Displayed
- **Fixed Wikipedia sources not appearing in review queue UI**
  - Sources were only saved when results met match score threshold
  - Now saves source record for every queried API, even when no result found
  - UI now shows "No matching track found" for sources that didn't return results
  - Users can see which sources were queried and their results

#### Artist Field Bug
- Fixed `ResolveTrackDuration()` passing album title as artist parameter
- Added `getAlbumInfo()` helper to fetch both album title and artist from database
- Now properly queries APIs with artist name for accurate matching

#### Source Display in Review Queue
- Fixed `sources` field returning null in review queue API
- Added `Sources` field to `ReviewItem` struct with proper GORM associations
- Frontend now displays MusicBrainz and Wikipedia durations in badges

#### Auto-Refresh on Resolution Completion
- Fixed review page not reloading when bulk resolution completes
- Added automatic stats and queue reload on completion
- Added success notification with resolution summary
- Added Refresh button for manual reload

#### Database Reset
- Updated ResetDatabase to include new duration resolution tables
- Added `duration_sources`, `duration_resolver_progresses`, `duration_resolutions` to cleanup list
- `app_configs` table preserved (OAuth settings not touched)

### Changed

#### UI Improvements
- Added "Resolution Center" to Sync dropdown navigation
- Improved source badges with color coding (musicbrainz: blue, wikipedia: purple, error: red, no-result: gray)
- Added toast notifications for user feedback
- Action buttons grouped on right side of review page
- Progress container shows real-time resolution counts

#### Consensus Configuration
- Default consensus threshold: 2 sources must agree
- Default tolerance: 3 seconds
- Default minimum match score: 0.6 (60%)
- Auto-apply enabled when consensus is reached

#### Rate Limiting
- MusicBrainz: 50 requests per minute (sliding window)
- Uses proper User-Agent: `Vinylfo/1.0 (https://github.com/xphox2/Vinylfo)`
- Sleep between tracks in background worker: 1.2 seconds

---

## [0.1.3-alpha] - 2026-01-18

### Fixed

#### Discogs Manual Import Track Detection
- **Fixed `track_number` and `disc_number` not being detected in manual Discogs search/add**: The `parseAlbumResponse()` function was not capturing `track_number` and `disc_number` fields from the Discogs API response because the parsing struct didn't include these fields
  - Added `TrackNumber` and `DiscNumber` fields to the Tracklist parsing struct
  - Updated `parseTracklist()` function to use API-provided values when available
  - Falls back to position parsing (e.g., "A1" → disc 1, track 1) when API fields are missing
  - Both sync and manual import now use identical track parsing logic

### Changed

#### Code Cleanup
- **Removed unused `playlist_original.js`**: Deleted stale backup file that was not being used
- **Simplified settings controller update logic**: Removed unnecessary map creation for single-field updates
- **Updated `go.mod` dependencies**: Ran `go mod tidy` to ensure clean dependency tree

#### Settings Page Cleanup
- **Removed "Sync Preferences" section**: Batch size, auto-apply safe updates, and auto-sync new albums options removed from Settings page
- **Removed "Application Settings" section**: Items per page setting removed from Settings page
- **Simplified settings API**: Backend now only returns Discogs-related settings (`discogs_connected`, `discogs_username`, `last_sync_at`)
- **Streamlined settings JavaScript**: Removed unused `renderSettings()`, `saveSyncSettings()`, and `saveAppSettings()` functions

#### Player Queue UI Improvements
- **Added spacing between album name and duration**: Queue items now have proper margin between title/album and duration columns
- **Added margin between title and album**: Improved visual separation of track information in queue display

#### Pagination Styling
- **Bottom pagination now matches top pagination**: Added consistent spacing and styling to bottom pagination controls on Albums page
- **New `.bottom-pagination` CSS class**: Provides consistent `margin-top` spacing for bottom pagination controls

---



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

[0.2.9-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.9-alpha
[0.2.8-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.8-alpha
[0.2.7-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.7-alpha
[0.2.6-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.6-alpha
[0.2.5-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.5-alpha
[0.2.4-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.4-alpha
[0.2.3-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.3-alpha
[0.2.2-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.2-alpha
[0.2.1-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.1-alpha
[0.2.0-alpha]: https://github.com/xphox2/vinylfo/releases/tag/v0.2.0-alpha
[0.1.3-alpha]: https://github.com/yourusername/vinylfo/releases/tag/v0.1.3-alpha
[0.1.2-alpha]: https://github.com/yourusername/vinylfo/releases/tag/v0.1.2-alpha
[0.1.1-alpha]: https://github.com/yourusername/vinylfo/releases/tag/v0.1.1-alpha
[0.1.0-alpha]: https://github.com/yourusername/vinylfo/releases/tag/v0.1.0-alpha
