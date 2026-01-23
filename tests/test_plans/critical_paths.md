# Critical Path Tests

This document describes critical user paths that must work correctly before alpha release.

## Path 1: Application Startup

### Test Steps
1. Launch `vinylfo.exe`
2. Verify system tray icon appears
3. Verify web interface accessible at `http://localhost:8080`
4. Check logs for successful startup messages

### Expected Results
- Application starts without errors
- No crash dialogs
- Server responds within 5 seconds

### Automated Verification
```bash
go test ./tests/syntax/... -v
```

## Path 2: Discogs OAuth Connection

### Test Steps
1. Navigate to Settings > Discogs
2. Click "Connect to Discogs"
3. Complete OAuth flow in browser
4. Verify connection status shows "Connected"
5. Check API rate limits display correctly

### Expected Results
- OAuth redirect works
- Tokens stored securely
- API calls use authenticated endpoints

### Automated Verification
```bash
go test ./controllers/... -run TestOAuth -v
go test ./discogs/... -v
```

## Path 3: Album Sync Process

### Test Steps
1. Start sync from a Discogs folder
2. Monitor progress indicator
3. Pause sync midway
4. Resume sync
5. Verify completed albums appear in library

### Expected Results
- Progress updates in real-time
- Pause/Resume works reliably
- No duplicate albums created
- Rate limiting respected

### Automated Verification
```bash
go test ./controllers/... -run TestSync -v
```

## Path 4: Playback Control

### Test Steps
1. Select a playlist
2. Start playback
3. Test pause/resume
4. Test skip track
5. Test volume control
6. Verify playback timer advances

### Expected Results
- Playback starts within 2 seconds
- Controls respond immediately
- No audio glitches

### Automated Verification
```bash
go test ./controllers/... -run TestPlayback -v
```

## Path 5: YouTube Link Resolution

### Test Steps
1. Open any track detail
2. Add YouTube URL
3. Verify video ID extracted correctly
4. Check match score calculated
5. Save link

### Expected Results
- URL parsing handles all YouTube formats
- Match scores between 0-1
- Links saved to database

### Automated Verification
```bash
go test ./services/... -run TestYouTube -v
```

## Path 6: Settings Management

### Test Steps
1. Change sync interval
2. Change theme
3. Update rate limiting settings
4. Verify settings persist after restart

### Expected Results
- All settings save correctly
- No validation errors
- Changes apply immediately

## Critical Bugs to Avoid

### Blocker Issues
1. [ ] Application crashes on startup
2. [ ] OAuth flow fails to complete
3. [ ] Sync process creates duplicates
4. [ ] Playback causes audio glitches
5. [ ] Settings not persisted

### High Priority
1. [ ] Progress indicator inaccurate
2. [ ] Pause/Resume cycle fails
3. [ ] YouTube URL parsing errors
4. [ ] Database locking issues

## Test Data Required

- Discogs test account credentials
- Sample YouTube video URLs
- Test playlists (5-10 albums each)
- Various album types (single, compilation, various artists)
