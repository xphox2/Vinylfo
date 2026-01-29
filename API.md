# Vinylfo API Documentation

A comprehensive REST API for managing music collections, playback, and YouTube integration.

## Base URL
```
http://localhost:8080
```

## Table of Contents

1. [System & Health](#system--health)
2. [Web Pages](#web-pages)
3. [Albums](#albums)
4. [Tracks](#tracks)
5. [Playback Control](#playback-control)
6. [Playback History](#playback-history)
7. [Video Feed (OBS Integration)](#video-feed-obs-integration)
8. [Album Art Feed (OBS)](#album-art-feed-obs)
9. [Track Info Feed (OBS)](#track-info-feed-obs)
10. [Sessions & Playlists](#sessions--playlists)
11. [Session Sharing](#session-sharing)
12. [Session Notes](#session-notes)
13. [Discogs Integration](#discogs-integration)
14. [Settings & Configuration](#settings--configuration)
15. [Log Management](#log-management)
16. [Audit Logs](#audit-logs)
17. [Database Backup](#database-backup)
18. [Duration Resolution](#duration-resolution)
19. [Duration Review](#duration-review)
20. [YouTube Integration](#youtube-integration)

---

## System & Health

### Health Check
- **GET** `/health`
- **Description:** Health check endpoint for monitoring
- **Response:**
```json
{
  "status": "healthy",
  "database": "connected",
  "timestamp": 1705776000
}
```
- **Status Codes:**
  - `200` - Service healthy
  - `503` - Service unhealthy

### Get Version
- **GET** `/version`
- **Description:** Get application version and database info
- **Response:** Version information with timestamp

### Frontend Configuration
- **GET** `/api/config`
- **Description:** Get frontend configuration for footer display

### Favicon
- **GET** `/favicon.ico`
- **Description:** Serve application favicon

---

## Web Pages

These endpoints serve HTML templates for the web interface:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Main application page (albums/tracks list) |
| GET | `/player` | Playback dashboard with video player |
| GET | `/playlist` | Playlist management page |
| GET | `/track/:id` | Track details page |
| GET | `/album/:id` | Album details page |
| GET | `/settings` | Settings and configuration page |
| GET | `/sync` | Discogs sync management page |
| GET | `/search` | Search page for adding albums |
| GET | `/resolution-center` | Duration resolution center |
| GET | `/youtube` | YouTube integration page |

---

## Albums

### List Albums
- **GET** `/albums`
- **Description:** Get all albums with pagination
- **Query Parameters:**
  - `page` (optional): Page number (default: 1)
  - `limit` (optional): Items per page (default: 20)
- **Response:**
```json
{
  "data": [
    {
      "id": 1,
      "title": "Abbey Road",
      "artist": "The Beatles",
      "release_year": 1969,
      "genre": "Rock",
      "cover_image_url": "https://...",
      "discogs_id": "12345"
    }
  ],
  "totalPages": 10,
  "page": 1,
  "limit": 20
}
```

### Search Albums
- **GET** `/albums/search`
- **Description:** Search albums by title or artist
- **Query Parameters:**
  - `q`: Search query string
  - `page` (optional): Page number
  - `limit` (optional): Items per page

### Get Album
- **GET** `/albums/:id`
- **Description:** Get album by ID with tracks
- **Response:** Album details including track list

### Get Album Image
- **GET** `/albums/:id/image`
- **Description:** Get album cover image
- **Response:** Image binary data

### Get Album Tracks
- **GET** `/albums/:id/tracks`
- **Description:** Get all tracks for a specific album
- **Response:** Array of track objects

### Create Album
- **POST** `/albums`
- **Description:** Create a new album
- **Request Body:**
```json
{
  "title": "New Album",
  "artist": "Artist Name",
  "release_year": 2024,
  "genre": "Rock",
  "discogs_id": "12345"
}
```

### Update Album
- **PUT** `/albums/:id`
- **Description:** Update an existing album
- **Request Body:** Album fields to update

### Delete Album
- **DELETE** `/albums/:id`
- **Description:** Delete an album and its tracks

---

## Tracks

### List Tracks
- **GET** `/tracks`
- **Description:** Get all tracks with pagination
- **Query Parameters:**
  - `page` (optional): Page number
  - `limit` (optional): Items per page
- **Response:**
```json
{
  "data": [
    {
      "id": 1,
      "title": "Come Together",
      "album_id": 1,
      "album_title": "Abbey Road",
      "album_artist": "The Beatles",
      "duration": 259,
      "track_number": 1,
      "youtube_video_id": "dQw4w9WgXcQ"
    }
  ],
  "totalPages": 10,
  "page": 1,
  "limit": 20
}
```

### Search Tracks
- **GET** `/tracks/search`
- **Description:** Search tracks by title or artist
- **Query Parameters:**
  - `q`: Search query string
  - `page` (optional): Page number
  - `limit` (optional): Items per page

### Get Track
- **GET** `/tracks/:id`
- **Description:** Get track by ID
- **Response:** Track details

### Create Track
- **POST** `/tracks`
- **Description:** Create a new track
- **Request Body:** Track data

### Update Track
- **PUT** `/tracks/:id`
- **Description:** Update a track
- **Request Body:** Track fields to update

### Delete Track
- **DELETE** `/tracks/:id`
- **Description:** Delete a track

### Set YouTube Video
- **PUT** `/tracks/:id/youtube`
- **Description:** Associate a YouTube video with a track
- **Request Body:**
```json
{
  "youtube_url": "https://youtube.com/watch?v=dQw4w9WgXcQ"
}
```

### Remove YouTube Video
- **DELETE** `/tracks/:id/youtube`
- **Description:** Remove YouTube video association from track

### Debug YouTube Matches
- **GET** `/api/debug/youtube-matches`
- **Description:** Debug endpoint for YouTube track matching

---

## Playback Control

### Get Current Playback
- **GET** `/playback`
- **Description:** Get basic current playback state

### Get Current Playback Info
- **GET** `/playback/current`
- **Description:** Get detailed current playback information

### Get Playback State
- **GET** `/playback/state`
- **Description:** Get comprehensive playback state with queue

### Playback Events Stream
- **GET** `/playback/events`
- **Description:** Server-sent events stream for real-time playback updates
- **Content-Type:** `text/event-stream`

### Start Playback
- **POST** `/playback/start`
- **Description:** Start playback of a track
- **Request Body:**
```json
{
  "track_id": 1,
  "album_id": 1,
  "queue": [],
  "queue_index": 0
}
```

### Start Playlist Playback
- **POST** `/playback/start-playlist`
- **Description:** Start playback of a playlist
- **Request Body:**
```json
{
  "playlist_id": "playlist_123",
  "shuffle": false
}
```

### Pause Playback
- **POST** `/playback/pause`
- **Description:** Pause current playback

### Resume Playback
- **POST** `/playback/resume`
- **Description:** Resume paused playback

### Skip Track
- **POST** `/playback/skip`
- **Description:** Skip to next track in queue

### Play Specific Index
- **POST** `/playback/play-index`
- **Description:** Play track at specific queue index
- **Request Body:**
```json
{
  "index": 5
}
```

### Previous Track
- **POST** `/playback/previous`
- **Description:** Go to previous track in queue

### Stop Playback
- **POST** `/playback/stop`
- **Description:** Stop playback and clear state

### Restore Session
- **POST** `/playback/restore`
- **Description:** Restore a previous playback session
- **Request Body:**
```json
{
  "playlist_id": "playlist_123"
}
```

### Clear Playback
- **POST** `/playback/clear`
- **Description:** Clear current playback state

### Update Progress
- **POST** `/playback/update-progress`
- **Description:** Update playback progress position
- **Request Body:**
```json
{
  "position": 120.5
}
```

### Seek
- **POST** `/playback/seek`
- **Description:** Seek to position in current track
- **Request Body:**
```json
{
  "seconds": 60
}
```

---

## Playback History

### Get Full History
- **GET** `/playback/history`
- **Description:** Get complete playback history

### Get Most Played
- **GET** `/playback/history/most-played`
- **Description:** Get most frequently played tracks

### Get Recent History
- **GET** `/playback/history/recent`
- **Description:** Get recently played tracks

### Get Track History
- **GET** `/playback/history/:track_id`
- **Description:** Get playback history for specific track

### Update History
- **POST** `/playback/update-history`
- **Description:** Manually update playback history

---

## Video Feed (OBS Integration)

### Video Feed Page
- **GET** `/feeds/video`
- **Description:** Video feed page optimized for OBS browser source

### Video Feed Events
- **GET** `/feeds/video/events`
- **Description:** Server-sent events for video feed updates
- **Content-Type:** `text/event-stream`

### Current YouTube Video
- **GET** `/playback/current-youtube`
- **Description:** Get current YouTube video information

### Next Track Preload
- **GET** `/playback/next-preload`
- **Description:** Get next track info for preloading

### Video Play
- **POST** `/playback/video/play`
- **Description:** Play video (feed controller)

### Video Pause
- **POST** `/playback/video/pause`
- **Description:** Pause video (feed controller)

### Video Stop
- **POST** `/playback/video/stop`
- **Description:** Stop video (feed controller)

### Video Next
- **POST** `/playback/video/next`
- **Description:** Next video (feed controller)

### Video Previous
- **POST** `/playback/video/previous`
- **Description:** Previous video (feed controller)

### Video Seek
- **POST** `/playback/video/seek`
- **Description:** Seek in video (feed controller)

### Get YouTube Duration
- **GET** `/playback/video/youtube-duration`
- **Description:** Get YouTube video duration

### Refresh Duration
- **POST** `/playback/video/refresh-duration`
- **Description:** Refresh YouTube video duration

### Refresh All Durations
- **POST** `/playback/video/refresh-all-durations`
- **Description:** Refresh all YouTube video durations

---

## Album Art Feed (OBS)

### Album Art Feed
- **GET** `/feeds/art`
- **Description:** Album art feed page for OBS integration

---

## Track Info Feed (OBS)

### Track Info Feed
- **GET** `/feeds/track`
- **Description:** Track information feed page for OBS integration

---

## Sessions & Playlists

### List Sessions
- **GET** `/sessions`
- **Description:** Get all playback sessions

### Get Session
- **GET** `/playback-sessions/:id`
- **Description:** Get session by ID

### Create Session
- **POST** `/sessions`
- **Description:** Create a new playback session
- **Request Body:** Session data

### Update Session
- **PUT** `/playback-sessions/:id`
- **Description:** Update a session
- **Request Body:** Session fields to update

### Delete Session
- **DELETE** `/playback-sessions/:id`
- **Description:** Delete a session

### Create Playlist Session
- **POST** `/sessions/playlist`
- **Description:** Create a playlist-based session

### List Playlists
- **GET** `/sessions/playlist`
- **Description:** Get all playlists

### Create New Playlist
- **POST** `/sessions/playlist/new`
- **Description:** Create a new playlist
- **Request Body:**
```json
{
  "name": "My Playlist"
}
```

### Get Playlist
- **GET** `/sessions/playlist/:id`
- **Description:** Get playlist by ID with tracks

### Update Playlist
- **PUT** `/sessions/playlist/:id`
- **Description:** Update playlist

### Delete Playlist
- **DELETE** `/sessions/playlist/:id`
- **Description:** Delete playlist

### Delete Playlist (All Sessions)
- **DELETE** `/sessions/playlist/:id/delete-all`
- **Description:** Delete playlist and all associated sessions

### Add Track to Playlist
- **POST** `/sessions/playlist/:id/tracks`
- **Description:** Add track to playlist
- **Request Body:**
```json
{
  "track_id": 1
}
```

### Remove Track from Playlist
- **DELETE** `/sessions/playlist/:id/tracks/:track_id`
- **Description:** Remove track from playlist

### Shuffle Playlist
- **POST** `/sessions/playlist/:id/shuffle`
- **Description:** Shuffle playlist tracks

---

## Session Sharing

### Create Share Link
- **POST** `/sessions/:session_id/share`
- **Description:** Create sharing link for session

### Get Share Info
- **GET** `/sessions/:session_id/share`
- **Description:** Get sharing information for session

### Update Share Settings
- **PUT** `/sessions/:session_id/share`
- **Description:** Update sharing settings

### Remove Sharing
- **DELETE** `/sessions/:session_id/share`
- **Description:** Remove sharing from session

### Access Shared Session
- **GET** `/share/:token`
- **Description:** Access shared session via public token

---

## Session Notes

### Add Note
- **POST** `/sessions/:session_id/notes`
- **Description:** Add note to session
- **Request Body:**
```json
{
  "content": "Note text"
}
```

### Get Notes
- **GET** `/sessions/:session_id/notes`
- **Description:** Get all notes for session

### Get Note
- **GET** `/notes/:id`
- **Description:** Get specific note by ID

### Update Note
- **PUT** `/notes/:id`
- **Description:** Update a note

### Delete Note
- **DELETE** `/notes/:id`
- **Description:** Delete a note

---

## Discogs Integration

### OAuth URL
- **GET** `/api/discogs/oauth/url`
- **Description:** Get Discogs OAuth authorization URL
- **Response:**
```json
{
  "auth_url": "https://..."
}
```

### OAuth Callback
- **GET** `/api/discogs/oauth/callback`
- **Description:** Discogs OAuth callback handler
- **Query Parameters:**
  - `oauth_token`: OAuth token
  - `oauth_verifier`: OAuth verifier

### Disconnect
- **POST** `/api/discogs/disconnect`
- **Description:** Disconnect Discogs account

### Connection Status
- **GET** `/api/discogs/status`
- **Description:** Check Discogs connection status

### Get Folders
- **GET** `/api/discogs/folders`
- **Description:** Get Discogs collection folders

### Search Discogs
- **GET** `/api/discogs/search`
- **Description:** Search Discogs database
- **Query Parameters:**
  - `q`: Search query
  - `type` (optional): Search type (release, master, artist)

### Preview Album
- **GET** `/api/discogs/albums/:id`
- **Description:** Preview album from Discogs

### Create Album from Discogs
- **POST** `/api/discogs/albums`
- **Description:** Create album from Discogs data

### Start Sync
- **POST** `/api/discogs/sync/start`
- **Description:** Start collection sync

### Get Sync Progress
- **GET** `/api/discogs/sync/progress`
- **Description:** Get sync progress

### Get Sync History
- **GET** `/api/discogs/sync/history`
- **Description:** Get sync history

### Resume Sync
- **GET** `/api/discogs/sync/resume`
- **Description:** Resume sync after interruption

### Pause Sync
- **POST** `/api/discogs/sync/pause`
- **Description:** Pause sync

### Resume from Pause
- **POST** `/api/discogs/sync/resume-pause`
- **Description:** Resume from paused state

### Get Batch
- **GET** `/api/discogs/sync/batch/:id`
- **Description:** Get batch details

### Confirm Batch
- **POST** `/api/discogs/sync/batch/:id/confirm`
- **Description:** Confirm batch processing

### Skip Batch
- **POST** `/api/discogs/sync/batch/:id/skip`
- **Description:** Skip batch

### Cancel Sync
- **POST** `/api/discogs/sync/cancel`
- **Description:** Cancel sync

### Fetch Username
- **POST** `/api/discogs/fetch-username`
- **Description:** Fetch Discogs username

### Refresh Tracks
- **POST** `/api/discogs/refresh-tracks`
- **Description:** Refresh tracks from Discogs

### Find Unlinked Albums
- **GET** `/api/discogs/unlinked-albums`
- **Description:** Find unlinked albums

### Delete Unlinked Albums
- **POST** `/api/discogs/unlinked-albums/delete`
- **Description:** Delete unlinked albums

### Cleanup Orphaned Tracks
- **POST** `/api/discogs/cleanup-orphaned-tracks`
- **Description:** Cleanup orphaned tracks

---

## Settings & Configuration

### Get Settings
- **GET** `/api/settings`
- **Description:** Get current settings
- **Response:**
```json
{
  "discogs_connected": true,
  "discogs_username": "username",
  "youtube_connected": true,
  "youtube_is_configured": true,
  "items_per_page": 20,
  "sync_mode": "all"
}
```

### Update Settings
- **PUT** `/api/settings`
- **Description:** Update settings
- **Request Body:** Settings to update

### Reset Database
- **POST** `/api/database/reset`
- **Description:** Reset database (destructive)

### Seed Database
- **POST** `/api/database/seed`
- **Description:** Seed database with sample data

### Get Log Settings
- **GET** `/api/settings/logs`
- **Description:** Get log settings

### Update Log Settings
- **PUT** `/api/settings/logs`
- **Description:** Update log settings

### Cleanup Logs
- **POST** `/api/settings/logs/cleanup`
- **Description:** Cleanup old log files

---

## Log Management

### Export Logs
- **GET** `/api/logs/export`
- **Description:** Export logs as ZIP for bug reports
- **Response:** ZIP file download

### List Log Files
- **GET** `/api/logs/list`
- **Description:** List available log files

---

## Audit Logs

### Get Audit Logs
- **GET** `/api/audit/logs`
- **Description:** Get audit logs
- **Query Parameters:**
  - `event_type` (optional): Filter by event type (oauth, auth, api, sync, security)
  - `limit` (optional): Max records (default: 50, max: 100)
  - `offset` (optional): Pagination offset
- **Response:**
```json
{
  "logs": [
    {
      "id": 1,
      "event_type": "oauth",
      "event_action": "connect",
      "user_id": 1,
      "ip_address": "192.168.1.1",
      "resource": "youtube_oauth",
      "status": "success",
      "created_at": "2024-01-20T12:00:00Z"
    }
  ],
  "total": 100,
  "limit": 50,
  "offset": 0
}
```

### Cleanup Audit Logs
- **POST** `/api/audit/cleanup`
- **Description:** Clean up old audit logs
- **Request Body:**
```json
{
  "days_retained": 90
}
```

---

## Database Backup

### Create Backup
- **POST** `/api/database/backup`
- **Description:** Create database backup (SQLite only)

### List Backups
- **GET** `/api/database/backups`
- **Description:** List all backups

### Cleanup Backups
- **POST** `/api/database/backups/cleanup`
- **Description:** Cleanup old backups

---

## Duration Resolution

### Get Tracks Needing Resolution
- **GET** `/api/duration/tracks`
- **Description:** Get tracks needing duration resolution

### Get Resolution Stats
- **GET** `/api/duration/stats`
- **Description:** Get resolution statistics

### Set Manual Duration
- **POST** `/api/duration/track/:id/manual`
- **Description:** Set manual duration for track

### Resolve Single Track
- **POST** `/api/duration/resolve/track/:id`
- **Description:** Resolve single track duration

### Retry Failed Track
- **POST** `/api/duration/resolve/track/:id/retry`
- **Description:** Retry failed track resolution

### Resolve Album
- **POST** `/api/duration/resolve/album/:id`
- **Description:** Resolve all tracks in album

### Get Track Resolution Status
- **GET** `/api/duration/resolve/track/:id`
- **Description:** Get resolution status for track

### Start Bulk Resolution
- **POST** `/api/duration/resolve/start`
- **Description:** Start bulk resolution

### Pause Bulk Resolution
- **POST** `/api/duration/resolve/pause`
- **Description:** Pause bulk resolution

### Resume Bulk Resolution
- **POST** `/api/duration/resolve/resume`
- **Description:** Resume bulk resolution

### Cancel Bulk Resolution
- **POST** `/api/duration/resolve/cancel`
- **Description:** Cancel bulk resolution

### Get Resolution Progress
- **GET** `/api/duration/resolve/progress`
- **Description:** Get bulk resolution progress

---

## Duration Review

### Get Review Queue
- **GET** `/api/duration/review`
- **Description:** Get review queue

### Get Resolved Queue
- **GET** `/api/duration/review/resolved`
- **Description:** Get resolved queue

### Get Review Details
- **GET** `/api/duration/review/:id`
- **Description:** Get review details

### Submit Review
- **POST** `/api/duration/review/:id`
- **Description:** Submit review decision

### Bulk Review
- **POST** `/api/duration/review/bulk`
- **Description:** Bulk review operations

---

## YouTube Integration

### OAuth & Connection

#### Get OAuth URL
- **GET** `/api/youtube/oauth/url`
- **Description:** Get YouTube OAuth authorization URL
- **Response:**
```json
{
  "auth_url": "https://accounts.google.com/o/oauth2/v2/auth?..."
}
```

#### OAuth Callback
- **GET** `/api/youtube/oauth/callback`
- **Description:** YouTube OAuth callback handler
- **Query Parameters:**
  - `code`: Authorization code
  - `state`: State parameter
  - `error`: Error message (if auth failed)

#### Disconnect
- **POST** `/api/youtube/disconnect`
- **Description:** Disconnect YouTube account and revoke tokens
- **Response:**
```json
{
  "message": "Successfully disconnected from YouTube",
  "connected": false
}
```

#### Connection Status
- **GET** `/api/youtube/status`
- **Description:** Check YouTube connection status
- **Response:**
```json
{
  "connected": true,
  "is_configured": true,
  "db_connected": true,
  "has_token": true
}
```

### YouTube Playlists

#### List YouTube Playlists
- **GET** `/api/youtube/playlists`
- **Description:** Get user's YouTube playlists

#### Create YouTube Playlist
- **POST** `/api/youtube/playlists`
- **Description:** Create new YouTube playlist
- **Request Body:**
```json
{
  "title": "My Vinyl Collection",
  "description": "A playlist of my favorite vinyl tracks",
  "privacy_status": "private"
}
```

#### Update YouTube Playlist
- **PUT** `/api/youtube/playlists/:id`
- **Description:** Update YouTube playlist

#### Delete YouTube Playlist
- **DELETE** `/api/youtube/playlists/:id`
- **Description:** Delete YouTube playlist

#### Get Playlist Items
- **GET** `/api/youtube/playlists/:id`
- **Description:** Get playlist items

#### Add Video to Playlist
- **POST** `/api/youtube/playlists/:id/videos`
- **Description:** Add video to playlist
- **Request Body:**
```json
{
  "video_id": "dQw4w9WgXcQ",
  "position": 0,
  "track_id": 1,
  "album_id": 1
}
```

#### Remove Video from Playlist
- **DELETE** `/api/youtube/playlists/:id/videos/:item_id`
- **Description:** Remove video from playlist

### YouTube Search

#### Search YouTube
- **POST** `/api/youtube/search`
- **Description:** Search for YouTube videos
- **Request Body:**
```json
{
  "query": "Come Together Beatles",
  "max_results": 5
}
```
- **Response:**
```json
{
  "videos": [
    {
      "video_id": "dQw4w9WgXcQ",
      "title": "Come Together - The Beatles",
      "channel": "The Beatles",
      "thumbnail": "https://..."
    }
  ],
  "total_results": 1
}
```

### Export to YouTube

#### Export Playlist
- **POST** `/api/youtube/export-playlist`
- **Description:** Export playback session to YouTube playlist
- **Request Body:**
```json
{
  "session_id": "session_123",
  "title": "Vinylfo Playlist",
  "description": "Exported from Vinylfo",
  "privacy_status": "private"
}
```
- **Response:**
```json
{
  "message": "Playlist exported successfully",
  "playlist_id": "PLxxxxx",
  "playlist_url": "https://www.youtube.com/playlist?list=PLxxxxx",
  "total_tracks": 10,
  "success_count": 8,
  "fail_count": 2
}
```

### Track Matching & Sync

#### Match Single Track
- **POST** `/api/youtube/match-track/:track_id`
- **Description:** Match single track to YouTube video

#### Match Playlist
- **POST** `/api/youtube/match-playlist/:playlist_id`
- **Description:** Match all tracks in playlist

#### Get Playlist Matches
- **GET** `/api/youtube/matches/:playlist_id`
- **Description:** Get all matches for playlist

#### Get Track Match
- **GET** `/api/youtube/match/:track_id`
- **Description:** Get match for specific track

#### Update Match
- **PUT** `/api/youtube/matches/:track_id`
- **Description:** Update track match

#### Delete Match
- **DELETE** `/api/youtube/matches/:track_id`
- **Description:** Delete track match

#### Sync Playlist to YouTube
- **POST** `/api/youtube/sync-playlist/:playlist_id`
- **Description:** Sync playlist to YouTube

#### Get Sync Status
- **GET** `/api/youtube/sync-status/:playlist_id`
- **Description:** Get playlist sync status

#### Get Match Candidates
- **GET** `/api/youtube/candidates/:track_id`
- **Description:** Get candidate videos for track

#### Select Candidate
- **POST** `/api/youtube/candidates/:track_id/select/:candidate_id`
- **Description:** Select candidate for track

#### Clear Web Cache
- **POST** `/api/youtube/clear-cache`
- **Description:** Clear YouTube web cache

---

## Error Responses

### 400 Bad Request
```json
{
  "error": "Invalid request parameters"
}
```

### 401 Unauthorized
```json
{
  "error": "Authentication required"
}
```

### 403 Forbidden
```json
{
  "error": "quotaExceeded",
  "reason": "The request cannot be completed because the daily limit has been exceeded"
}
```

### 404 Not Found
```json
{
  "error": "Resource not found"
}
```

### 500 Internal Server Error
```json
{
  "error": "Internal server error"
}
```

---

## Rate Limiting

YouTube API calls are rate-limited:
- **Authenticated requests:** 10,000 units/day
- **Unauthenticated requests:** 1,000 units/day

---

## Authentication

OAuth 2.0 with PKCE is used for:
- **YouTube integration** - Google OAuth
- **Discogs integration** - Discogs OAuth

Tokens are:
- Stored encrypted in the database
- Automatically refreshed before expiry
- Revoked on disconnect

---

## Security Headers

All responses include the following security headers:
- `Content-Security-Policy`: Restricts resource loading
- `X-Content-Type-Options`: nosniff
- `X-Frame-Options`: DENY
- `X-XSS-Protection`: 1; mode=block
- `Referrer-Policy`: strict-origin-when-cross-origin
- `Permissions-Policy`: geolocation=(), microphone=(), camera=()

---

## Statistics

- **Total API Endpoints:** 168+
- **GET Endpoints:** 78+
- **POST Endpoints:** 62+
- **PUT Endpoints:** 16+
- **DELETE Endpoints:** 18+

### Categories:
1. System & Health (4 endpoints)
2. Web Pages (9 endpoints)
3. Albums (8 endpoints)
4. Tracks (9 endpoints)
5. Playback Control (14 endpoints)
6. Playback History (5 endpoints)
7. Video Feed/OBS (12 endpoints)
8. Album Art Feed (1 endpoint)
9. Track Info Feed (1 endpoint)
10. Sessions/Playlists (14 endpoints)
11. Session Sharing (5 endpoints)
12. Session Notes (5 endpoints)
13. Discogs Integration (22 endpoints)
14. Settings & Config (7 endpoints)
15. Log Management (2 endpoints)
16. Audit Logs (2 endpoints)
17. Database Backup (3 endpoints)
18. Duration Resolution (11 endpoints)
19. Duration Review (5 endpoints)
20. YouTube Integration (21 endpoints)
