# Vinylfo Project Task List

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

---

## Feature: Fix Discogs Album Import - COMPLETED

### Overview
Fix Discogs album import to capture all available metadata (label, country, release date, genre, style) and track information including disc/side tracking.

### User Stories
- [x] As a user, when I search and add an album from Discogs, I want all metadata captured (label, country, release date, style)
- [x] As a user, I want track information properly imported with disc and side indicators
- [x] As a user, I want duration stored as seconds (not strings)
- [x] As a user, I want to see all available metadata in the album detail modal before adding
- [x] As a user, I want to preview albums without adding them automatically
- [x] As a user, I want validation to prevent adding albums without tracks

---

### Phase 1: Database Schema Updates

#### models/models.go
- [x] Add fields to Album struct:
  - [x] `Label` (string) - Primary label name
  - [x] `Country` (string) - Release country
  - [x] `ReleaseDate` (string) - Full release date
  - [x] `Style` (string) - Comma-separated styles from Discogs
  - [x] `DiscogsID` (int) - Original Discogs release ID

- [x] Add fields to Track struct:
  - [x] `DiscNumber` (int) - Which disc (1, 2, 3...)
  - [x] `Side` (string) - Side position code (A1, B2, C1, etc.)
  - [x] `Position` (string) - Full position code for reference

---

### Phase 2: Discogs API Response Parsing

#### discogs/client.go

- [x] Update `parseAlbumResponse()` function:
  - [x] Extract `label` field from Discogs response
  - [x] Include `country` in returned album map
  - [x] Extract `released` or `date Released` field for release date
  - [x] Parse all `styles` as comma-separated string

- [x] Update `parseTracklist()` function:
  - [x] Keep position as-is (A1, B2, C1, etc.)
  - [x] Add duration string to seconds conversion helper:
    - [x] Parse "3:45" format (MM:SS)
    - [x] Parse "1:30:00" format (HH:MM:SS)
  - [x] Return duration as int (seconds)

---

### Phase 3: Controller Integration

#### controllers/discogs.go

- [x] Update `CreateAlbum()` function:
  - [x] Extract `label`, `country`, `release_date`, `style` from Discogs data
  - [x] Update album creation with new fields
  - [x] Fix track extraction to handle duration conversion (string to int)
  - [x] Update track creation to include `disc_number` and `side` fields

- [x] Add `PreviewAlbum()` endpoint:
  - [x] GET /api/discogs/albums/:id - Fetches album without saving
  - [x] Validates that album has tracks before returning

---

### Phase 4: Frontend Updates

#### static/js/search.js

- [x] Update album search results:
  - [x] Change "Add" button to "View"
  - [x] Call preview endpoint on click

- [x] Update album detail modal:
  - [x] Display Label (below title)
  - [x] Display Country and Release Date
  - [x] Display Style(s)
  - [x] Update track list display to show disc/side indicators
  - [x] Show "No track information available" only if tracklist is empty
  - [x] Show track count in header

#### static/search.html
- [x] Modal already has "Add to Collection" button text

---

### Phase 5: Testing

- [x] Test search and add album flow
- [x] Verify all metadata captured (label, country, date, style)
- [x] Verify tracks have correct disc/side assignments
- [x] Verify position conversion (1-1 → A1, 2-1 → B1, etc.)
- [x] Verify duration conversion works (3:45 -> 225 seconds)
- [x] Test preview endpoint without saving
- [x] Test validation for albums with no tracks

---

### Files Modified

| File | Changes |
|------|---------|
| `models/models.go` | Add 5 fields to Album (Label, Country, ReleaseDate, Style, DiscogsID), 3 fields to Track (DiscNumber, Side, Position) |
| `discogs/client.go` | Enhance parsing to extract all Discogs fields, add duration string to seconds conversion, add position format conversion (1-1 → A1) |
| `controllers/discogs.go` | Add PreviewAlbum endpoint, update CreateAlbum, add no-tracks validation |
| `routes/routes.go` | Add GET /api/discogs/albums/:id route |
| `static/js/search.js` | Change button to "View", use preview endpoint, add parseInt for discogs_id, event delegation for modal buttons |
| `static/search.html` | Modal buttons: X, Cancel, Add to Collection |
| `static/css/search.css` | Modal styling with centered layout, larger cover image (180px), wrapped text for long styles, aligned search results |

---

### Completed Features

#### Feature: Fix Discogs Album Import ✓
- All metadata captured (label, country, release date, style)
- Track positions standardized (A1, A2, B1, B2, etc.)
- Duration stored as seconds
- Preview before add workflow
- No-tracks validation

#### Feature: UI/UX Improvements ✓
- Search results properly centered and aligned
- Modal centered with larger cover image
- Long text wraps properly
- Modal buttons work correctly

---

### Estimated Effort
- Phase 1 (Database): ~15 minutes - DONE
- Phase 2 (API Parsing): ~45 minutes - DONE
- Phase 3 (Controller): ~30 minutes - DONE
- Phase 4 (Frontend): ~30 minutes - DONE
- Phase 5 (Testing): ~30 minutes - DONE

**Total: ~2.5 hours - COMPLETED**