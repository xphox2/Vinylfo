# Vinylfo Codebase Cleanup Tracker

**Started:** 2026-01-21
**Total Lines:** ~32,590
**Status:** In Progress

---

## Overview

This document tracks the cleanup and refactoring progress for the Vinylfo codebase. Use this to resume work if context is lost.

---

## Priority 1: Large Files Requiring Refactoring

| File | Lines | Status | Action Required |
|------|-------|--------|-----------------|
| `/discogs/client.go` | 1,286 | PARTIAL | Extracted rate_limiter.go (187 lines), string_utils.go (165 lines) |
| `/controllers/playback.go` | 782 | PARTIAL | Extracted playback_history.go (169 lines) |
| `/controllers/duration.go` | 667 | PARTIAL | Extracted duration_bulk.go (157 lines) |
| `/services/youtube_sync_service.go` | 838 | TODO | Split into: sync.go, playlist_export.go |
| `/duration/youtube_oauth_client.go` | 702 | TODO | Split into: oauth.go, playlist_api.go |
| `/static/css/style.css` | 1,246 | OPTIONAL | Could extract youtube styles (lines 957-1246), but lower priority |
| `/static/js/playlist.js` | 870 | DONE | Extracted playlist-youtube.js (380 lines) |
| `/static/js/playback-dashboard.js` | 1,016 | PARTIAL | Extracted tab-sync-manager.js (112 lines) |
| `/static/js/resolution-center.js` | 904 | TODO | Split into modules |
| `/static/js/sync.js` | 711 | TODO | Split into modules |
| `/static/js/app.js` | 648 | TODO | Extract state, navigation |

---

## Priority 2: Dead Code Found (Completed)

### Go Dead Code
| File | Line | Function | Status | Issue |
|------|------|----------|--------|-------|
| `controllers/playlist.go` | ~489 | `PlayPlaylist()` | DONE | Removed - never called |
| `controllers/youtube.go` | ~500 | `updateTrackYouTubeInfo()` | DONE | Removed - empty stub |

### JavaScript Dead Code
| File | Line | Function | Status | Issue |
|------|------|----------|--------|-------|
| `static/js/resolution-center.js` | ~891-896 | `formatDuration()` | DONE | Removed duplicate |
| `static/js/resolution-center.js` | ~648,670 | `submitReview()` | DONE | Removed duplicate |
| `static/js/resolution-center.js` | ~662,684 | `submitManual()` | DONE | Removed duplicate |
| `static/js/playlist.js` | ~798,813 | `loadYouTubePlaylist()` | DONE | Removed duplicate + orphaned code |

### Debug Console.log Statements
Found **127 console.log statements** across JavaScript files:
- `static/js/playback-dashboard.js` - 73 statements (with [PlaybackManager], [TabSync] prefixes)
- `static/js/sync.js` - 29 statements (with [Sync] prefixes)
- `static/js/playlist.js` - 17 statements
- `static/js/app.js` - 6 statements

**Status:** REVIEW - These appear to be intentional debugging aids with structured prefixes.
Consider adding a debug flag to enable/disable them in production.

---

## Priority 3: Code Quality Checks

- [ ] Remove commented-out code
- [ ] Remove debug console.log statements
- [ ] Remove unused imports
- [ ] Check for duplicate code

---

## Completed Tasks

| Date | Task | Details |
|------|------|---------|
| 2026-01-21 | Initial codebase exploration | Identified 11+ large files needing refactoring |
| 2026-01-21 | Remove dead Go code | Removed PlayPlaylist from playlist.go, updateTrackYouTubeInfo from youtube.go |
| 2026-01-21 | Remove dead JavaScript code | Removed duplicates in resolution-center.js, orphaned code in playlist.js |
| 2026-01-21 | Extract discogs/rate_limiter.go | 187 lines extracted from client.go |
| 2026-01-21 | Extract discogs/string_utils.go | 165 lines extracted from client.go (string matching, levenshtein, etc.) |
| 2026-01-21 | Extract controllers/playback_history.go | 169 lines extracted (history tracking methods) |
| 2026-01-21 | Review CSS structure | style.css has feature-specific sections, could extract YouTube styles (optional) |
| 2026-01-21 | Audit console.log statements | 127 statements found, appear intentional with prefixes |

---

## In Progress

| Task | Started | Notes |
|------|---------|-------|
| Review console.log statements | 2026-01-21 | 127 statements - appear to be intentional debugging |

---

## Files Structure Summary

```
Total: ~32,590 lines
- Go Source: ~17,050 lines
- JavaScript: ~6,450 lines
- CSS: ~3,650 lines
- HTML Templates: ~1,290 lines
```

### Go Files Over 400 Lines (Current State)
1. discogs/client.go - 1,286 lines (was 1,621 - extracted rate_limiter.go, string_utils.go)
2. controllers/discogs_test.go - 1,368 lines (test file - OK)
3. services/youtube_sync_service.go - 838 lines
4. controllers/duration.go - 814 lines
5. controllers/playback.go - 782 lines (was 942 - extracted playback_history.go)
6. duration/youtube_oauth_client.go - 702 lines
7. services/sync_worker.go - 686 lines
8. services/youtube_web_search.go - 613 lines
9. controllers/youtube.go - 573 lines (cleaned up dead code)
10. services/duration_resolver.go - 521 lines
11. controllers/playlist.go - 519 lines (cleaned up dead code)
12. services/youtube_matcher_test.go - 471 lines (test file - OK)
13. services/album_import.go - 441 lines
14. controllers/discogs.go - 423 lines
15. controllers/youtube_sync.go - 407 lines

**New files created:**
- discogs/rate_limiter.go - 187 lines
- discogs/string_utils.go - 165 lines
- controllers/playback_history.go - 169 lines

### JavaScript Files Over 300 Lines (Need Attention)
1. static/js/playlist.js - 1,251 lines
2. static/js/playback-dashboard.js - 1,124 lines
3. static/js/resolution-center.js - 904 lines
4. static/js/sync.js - 711 lines
5. static/js/app.js - 648 lines
6. static/js/search.js - 392 lines
7. static/js/settings.js - 364 lines

---

## Refactoring Plan

### Phase 1: Find and Remove Unused Code
Scan for:
- Unused Go functions
- Unused JavaScript functions
- Unused CSS classes
- Commented-out code blocks

### Phase 2: Split Large Go Files
Priority order:
1. discogs/client.go (1,621 lines) - Split by concern
2. controllers/playback.go (942 lines) - Split by operation
3. services/youtube_sync_service.go (838 lines) - Split by feature
4. controllers/duration.go (814 lines) - Split by endpoint type

### Phase 3: Split Large JavaScript Files
Priority order:
1. playlist.js (1,251 lines) - Split into list/editor/queue modules
2. playback-dashboard.js (1,124 lines) - Split into controls/ui/state modules
3. resolution-center.js (904 lines) - Split into list/review/batch modules

### Phase 4: Split Large CSS Files
1. style.css (1,246 lines) - Split into base/layout/components/utilities

---

## Resume Instructions

If context is lost, read this file first. Then:
1. Check the "In Progress" section above
2. Continue from where we left off
3. Update status as tasks complete

---

## Session Summary (2026-01-21)

### Accomplished Today
- **Dead Code Removed:**
  - Go: PlayPlaylist(), updateTrackYouTubeInfo()
  - JS: Duplicate methods in resolution-center.js, orphaned code in playlist.js
  - Fixed bug: Added loadYouTubeSyncStatus() function that was called but never defined

- **Go Files Refactored:**
  - discogs/client.go: 1,621 -> 1,286 lines (-335 lines)
  - controllers/playback.go: 942 -> 782 lines (-160 lines)
  - controllers/duration.go: 814 -> 667 lines (-147 lines)

- **JavaScript Files Refactored:**
  - static/js/playlist.js: 1,240 -> 870 lines (-370 lines)
  - static/js/playback-dashboard.js: 1,124 -> 1,016 lines (-108 lines)

- **New Module Files Created:**
  - discogs/rate_limiter.go (187 lines)
  - discogs/string_utils.go (165 lines)
  - controllers/playback_history.go (169 lines)
  - controllers/duration_bulk.go (157 lines)
  - static/js/playlist-youtube.js (380 lines)
  - static/js/tab-sync-manager.js (112 lines)

- **Total Lines Extracted:** ~1,120 lines into 6 new module files

### Still TODO (Future Sessions)
1. Further split large Go files (youtube_sync_service.go - 838 lines)
2. Continue splitting large JS files (playback-dashboard.js - 1,016 lines)
3. Console.log statements kept intentionally for debugging
4. Optional: Extract YouTube styles from style.css
