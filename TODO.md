# VinylFO Project Task List

## Feature: Playlist Playback with Resume Tracking

### Overview
Enable playing playlists from the dashboard with continuous progress tracking and resume capability even after browser close.

### User Stories
- [x] As a user, I want to click "Play" on a playlist and have it start playing the first track
- [x] As a user, I want my playback position to be saved automatically so I can resume later
- [x] As a user, I want to see a queue of upcoming tracks and toggle its visibility
- [x] As a user, I want my listening history tracked across sessions

---

### Phase 1: Backend Foundation

#### Database Schema Changes
- [x] Add columns to PlaybackSession model:
  - [x] `playlist_id` (string) - which playlist is playing
  - [x] `playlist_name` (string) - display name of playlist
  - [x] `queue` (string/JSON) - ordered array of track IDs
  - [x] `queue_index` (int) - current position in queue
  - [x] `queue_position` (int) - saved position in current track (seconds)

- [x] Create TrackHistory model:
  - [x] `track_id` (uint) - reference to track
  - [x] `playlist_id` (string) - which playlist played from
  - [x] `listen_count` (int) - number of times played
  - [x] `last_played` (time) - when last played
  - [x] `progress` (int) - last saved position

#### API Endpoints
- [x] POST `/playback/start-playlist` - Start playlist, return first track + queue
  - [x] Input: `playlist_id`, `playlist_name`, `track_ids[]`
  - [x] Output: First track details, queue info, saves state to DB

- [x] POST `/playback/update-progress` - Save current position
  - [x] Input: `track_id`, `position_seconds`, `queue_index`
  - [x] Updates `queue_position` and `queue_index` in DB

- [x] GET `/playback/state` - Get current playback state for resume
  - [x] Returns: current track, position, queue, playlist info

- [x] GET `/playback/history` - Get track listening history
  - [x] Returns: list of tracks with listen counts

- [x] GET `/playback/history/:track_id` - Get history for specific track

- [x] Modify GET `/playback/current` - Return queue info + queue index with full track objects

- [x] Modify POST `/playback/start` - Also update TrackHistory table

- [x] Implement POST `/playback/pause` - Pause playback, save state
- [x] Implement POST `/playback/resume` - Resume playback
- [x] Implement POST `/playback/skip` - Skip to next track in queue
- [x] Implement POST `/playback/previous` - Skip to previous track in queue
- [x] Implement POST `/playback/stop` - Stop playback, clear state

---

### Phase 2: Playlist Integration

#### playlist.js Changes
- [x] Update "Play" button to call `POST /playback/start-playlist`
- [x] Pass playlist ID, name, and ordered track IDs
- [x] Navigate to `/dashboard` after API call
- [x] Handle errors gracefully
- [x] Fix currentPlaylistId persistence using localStorage

---

### Phase 3: Dashboard Enhancement

#### playback-dashboard.js Rewrite
- [x] Create PlaybackManager class with:
  - [x] `queue` array - ordered list of tracks
  - [x] `queueIndex` int - current position
  - [x] `saveInterval` - timer for saving progress
  - [x] `isQueueVisible` bool - toggle state

- [x] On dashboard load:
  - [x] Call `GET /playback/state`
  - [x] If state exists, restore playback directly (removed modal)
  - [x] If no state, load default view

- [x] Progress saving:
  - [x] Save every 5 seconds while playing
  - [x] Save on `beforeunload` event
  - [x] Call `POST /playback/update-progress`

- [x] Queue toggle:
  - [x] "Show Queue" / "Hide Queue" button
  - [x] Display queue in panel when visible
  - [x] Highlight current track in queue
  - [x] Allow clicking queue items to jump

- [x] Playback controls:
  - [x] Play button - calls `/playback/resume`
  - [x] Pause button - calls `/playback/pause`
  - [x] Previous button - calls `/playback/previous`
  - [x] Next button - calls `/playback/skip`
  - [x] Stop button - calls `/playback/stop`

- [x] Clickable progress bar for seeking

---

### Phase 4: UI Components

#### playback-dashboard.html
- [x] Add resume modal (hidden by default, removed in later update)
- [x] Add queue toggle button
- [x] Add queue panel (hidden by default)
- [x] Style modal and queue appropriately

#### playback-dashboard.css
- [x] Style resume modal
- [x] Style queue panel
- [x] Style queue list items
- [x] Highlight current track in queue
- [x] Add cursor:pointer to progress bar for seek indication

---

### Phase 5: Testing & Polish

- [x] Test full flow: playlist → play → close browser → reopen → resume
- [x] Test queue navigation (next/previous)
- [x] Test progress saving interval
- [x] Test clickable progress bar
- [ ] Test edge cases:
  - [x] Track deleted from database
  - [x] Queue empty
  - [x] User explicitly stops playback
  - [ ] Multiple tabs open

---

### Files Modified

| File | Changes |
|------|---------|
| `models/models.go` | Extend PlaybackSession, add TrackHistory |
| `main.go` | Add 10+ new endpoints, modify 2 existing |
| `static/js/playlist.js` | Update Play button, fix localStorage persistence |
| `static/js/playback-dashboard.js` | Rewrite with PlaybackManager class, add seek functionality |
| `static/playback-dashboard.html` | Add modal, queue panel, toggle button |
| `static/css/playback-dashboard.css` | Style modal, queue panel, progress bar |

---

### Remaining Tasks

1. **Listening History Display** - DONE
   - [x] Show most played tracks on dashboard
   - [x] Show recently played
   - [x] Display track history in separate page

2. **Multiple Tabs Support** - DONE
   - [x] Sync playback state across tabs
   - [x] Handle concurrent playback requests
   - [x] Sync playback time across computers using server timestamps

---

### Estimated Effort
- Phase 1 (Backend): ~2 hours - DONE
- Phase 2 (Playlist): ~30 minutes - DONE
- Phase 3 (Dashboard): ~3 hours - DONE
- Phase 4 (UI): ~1 hour - DONE
- Phase 5 (Testing): ~1 hour - IN PROGRESS
- Remaining Tasks: ~5-6 hours

**Total: ~12+ hours invested**

---

## Feature: Discogs Data Sync - COMPLETED

### Overview
Sync vinyl collection from Discogs API with rate limiting, batch processing, and data review capabilities. Supports both OAuth collection sync and anonymous search/manual add.

### User Stories
- [x] As a user, I want to connect my Discogs account using OAuth
- [x] As a user, I want to sync my Discogs collection in batches with confirmation
- [x] As a user, I want to search Discogs database without authentication
- [x] As a user, I want to add albums to my collection that aren't in Discogs
- [x] As a user, I want to review additional data from Discogs before merging
- [x] As a user, I want to configure sync preferences (batch size, auto-apply, etc.)

---

## Feature: Playlist Playback with Resume Tracking - COMPLETED

### Overview
Enable playing playlists from the dashboard with continuous progress tracking and resume capability even after browser close.

### User Stories
- [x] As a user, I want to click "Play" on a playlist and have it start playing the first track
- [x] As a user, I want my playback position to be saved automatically so I can resume later
- [x] As a user, I want to see a queue of upcoming tracks and toggle its visibility
- [x] As a user, I want my listening history tracked across sessions

---

## Feature: Listening History Display - COMPLETED

- [x] Show most played tracks on dashboard
- [x] Show recently played
- [x] Display track history in separate page

## Feature: Multiple Tabs Support - COMPLETED

- [x] Sync playback state across tabs
- [x] Handle concurrent playback requests
- [x] Sync playback time across computers using server timestamps