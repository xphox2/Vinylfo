# Vinylfo OBS Support Guide

This document provides comprehensive documentation for using Vinylfo's OBS feeds for live streaming.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Feed Types](#feed-types)
3. [URL Configuration Options](#url-configuration-options)
4. [Common OBS Setups](#common-obs-setups)
5. [OBS Setup Instructions](#obs-setup-instructions)
6. [Features Overview](#features-overview)
7. [Troubleshooting](#troubleshooting)

---

## Quick Start

Vinylfo provides three OBS-friendly feeds. Add a **Browser Source** in OBS for each:

### Video Feed (YouTube videos)
- URL: `http://localhost:8080/feeds/video?overlay=bottom&theme=dark`
- Shows YouTube videos synced with Vinylfo playback

### Album Art Feed (Full-screen art)
- URL: `http://localhost:8080/feeds/art?theme=dark&animation=true`
- Displays full-screen album art with Ken Burns effect

### Track Info Feed (Marquee text)
- URL: `http://localhost:8080/feeds/track?theme=dark&speed=5`
- Infinite scrolling marquee with track metadata

All feeds use Server-Sent Events (SSE) for real-time synchronization.

---

## Feed Types

### Video Feed (`/feeds/video`)
Displays YouTube videos matched to tracks. Falls back to album art when no video is available.

**Use cases:**
- Full music video playback
- Lyric videos
- Visual content alongside audio

### Album Art Feed (`/feeds/art`)
Full-screen album art display with optional Ken Burns animation.

**Use cases:**
- Background visuals during tracks
- Clean, minimalist stream graphics
- Photo gallery style display

### Track Info Feed (`/feeds/track`)
Infinite marquee displaying track metadata (artist, title, album).

**Use cases:**
- Lower third scrolling text
- Song identification overlays
- Radio-style scrolling displays

---

## URL Configuration Options

All options are passed as URL query parameters.

### Feed URLs

| Feed | URL |
|------|-----|
| Video Feed | `http://localhost:8080/feeds/video` |
| Album Art Feed | `http://localhost:8080/feeds/art` |
| Track Info Feed | `http://localhost:8080/feeds/track` |

### Video Feed Parameters

| Parameter | Options | Default | Description |
|-----------|---------|---------|-------------|
| `overlay` | `bottom`, `top`, `none` | `bottom` | Track info overlay position |
| `theme` | `dark`, `light`, `transparent` | `dark` | Color scheme for text and backgrounds |
| `transition` | `fade`, `slide`, `none` | `fade` | Animation when track changes |
| `showVisualizer` | `true`, `false` | `true` | Show/hide audio visualizer |
| `quality` | `auto`, `high`, `medium`, `small` | `auto` | YouTube video quality preference |
| `overlayDuration` | `0`-`30` | `5` | Seconds before overlay auto-hides (0 = never) |

### Album Art Feed Parameters

| Parameter | Options | Default | Description |
|-----------|---------|---------|-------------|
| `theme` | `dark`, `light`, `transparent` | `dark` | Background color scheme |
| `animation` | `true`, `false` | `true` | Enable Ken Burns animation |
| `animDuration` | `5`-`120` | `20` | Animation duration in seconds |
| `fit` | `contain`, `cover` | `cover` | How image fits the viewport |

### Track Info Feed Parameters

| Parameter | Options | Default | Description |
|-----------|---------|---------|-------------|
| `theme` | `dark`, `light`, `transparent` | `dark` | Text color scheme |
| `speed` | `1`-`10` | `5` | Marquee scroll speed (1=slow, 10=fast) |
| `separator` | any string | `*` | Separator between track elements |
| `showDuration` | `true`, `false` | `true` | Show track duration |
| `showAlbum` | `true`, `false` | `true` | Show album title |
| `showArtist` | `true`, `false` | `true` | Show artist name |
| `direction` | `rtl`, `ltr` | `rtl` | Scroll direction (right-to-left or left-to-right) |
| `prefix` | any string | `Now Playing:` | Text prefix before track info |

### Example URLs

**Video Feed:**
```bash
http://localhost:8080/feeds/video?overlay=bottom&theme=dark&transition=fade
```

**Album Art Feed:**
```bash
# Dark theme with Ken Burns animation (20s default)
http://localhost:8080/feeds/art?theme=dark&animation=true

# Light theme, no animation, contain fit
http://localhost:8080/feeds/art?theme=light&animation=false&fit=contain

# Fast animation (5 seconds)
http://localhost:8080/feeds/art?theme=dark&animation=true&animDuration=5
```

**Track Info Feed:**
```bash
# Default dark theme, standard speed
http://localhost:8080/feeds/track?theme=dark

# Light theme, fast scroll, no duration
http://localhost:8080/feeds/track?theme=light&speed=10&showDuration=false

# Left-to-right scroll, custom prefix
http://localhost:8080/feeds/track?direction=ltr&prefix="Now on air:"&theme=transparent
```

---

## Common OBS Setups

### Setup 1: Full-Featured Video Stream

**URL:**
```
http://localhost:8080/feeds/video?overlay=bottom&theme=dark&transition=fade&showVisualizer=true&quality=auto&overlayDuration=5
```

**Settings:**
- Overlay: Bottom track info
- Theme: Dark with blurred background
- Transition: Smooth fade between tracks
- Visualizer: On for audio feedback
- Duration: 5 seconds before hiding info

**Best for:** General streaming with full information display

---

### Setup 2: Album Art Background

**URL:**
```
http://localhost:8080/feeds/art?theme=dark&animation=true&animDuration=20&fit=cover
```

**Settings:**
- Full-screen album art
- Ken Burns animation enabled
- 20-second animation cycle
- Cover fit (fills viewport)

**Best for:** Background visuals during playback, minimalist stream graphics

**OBS Tip:** Set the Browser Source to 1920x1080 and use **Blend Modes > Normal** over your main content.

---

### Setup 3: Scrolling Track Info (Lower Third)

**URL:**
```
http://localhost:8080/feeds/track?theme=dark&speed=5&direction=rtl&prefix=Now Playing:
```

**Settings:**
- Dark theme text
- Standard speed (30s cycle)
- Right-to-left scroll
- "Now Playing:" prefix

**Best for:** Song identification overlays, radio-style displays

**OBS Tip:** Position at the bottom of your canvas and adjust width to fit your layout.

---

### Setup 4: Clean Album Art Focus

**URL:**
```
http://localhost:8080/feeds/video?overlay=none&showVisualizer=true&quality=high
```

**Settings:**
- Overlay: None (just video/album art)
- Visualizer: On
- Quality: High

**Best for:** Music-focused streams where album art is the focus

---

### Setup 5: Transparent Track Overlay

**URL:**
```
http://localhost:8080/feeds/track?theme=transparent&direction=ltr&speed=3
```

**Settings:**
- Transparent background
- Left-to-right scroll
- Slow speed for readability

**Best for:** Keying over video backgrounds, clean overlays

**OBS Tip:** Use **Chroma Key** filter to remove any remaining background.

---

### Setup 6: Fast Scrolling Track Info

**URL:**
```
http://localhost:8080/feeds/track?theme=dark&speed=10&showDuration=true&separator=|
```

**Settings:**
- Maximum scroll speed
- Shows track duration
- Pipe separator instead of asterisk

**Best for:** High-energy streams, quick information display

---

### Setup 7: Minimal Video Info

**URL:**
```
http://localhost:8080/feeds/video?overlay=bottom&theme=dark&showVisualizer=false&overlayDuration=3
```

**Settings:**
- Overlay: Minimal bottom info
- Visualizer: Off
- Duration: Quick 3-second hide

**Best for:** Streams where you want subtle track identification

---

### Setup 8: Transparent Keying (Green Screen)

**URL:**
```
http://localhost:8080/feeds/video?theme=transparent&overlay=bottom&overlayDuration=0
```

**Settings:**
- Theme: Transparent background
- Overlay: Bottom, stays visible
- Duration: Never auto-hides

**Best for:** Using with OBS chroma key filter for custom backgrounds

**OBS Setup:**
1. Add Browser Source with the URL above
2. Right-click the source → **Filter**
3. Add **Chroma Key** filter
4. Adjust color key settings

---

### Setup 9: Slide Transition (Dynamic Movement)

**URL:**
```
http://localhost:8080/feeds/video?overlay=bottom&theme=dark&transition=slide&showVisualizer=true
```

**Settings:**
- Transition: Slide animation
- Visualizer: On

**Best for:** High-energy streams with dynamic visuals

---

### Setup 10: Light Theme for Bright Backgrounds

**URL:**
```
http://localhost:8080/feeds/art?theme=light&animation=false
```
or
```
http://localhost:8080/feeds/track?theme=light&direction=rtl
```

**Settings:**
- Light background/theme
- No animation (for art feed)

**Best for:** Overlaying on light-colored video content

---

### Setup 11: Multiple Synchronized Feeds

Combine feeds for a professional layered look:

**Layer 1 (Bottom):** Album Art
```
http://localhost:8080/feeds/art?theme=transparent&animation=false
```

**Layer 2 (Middle):** Video
```
http://localhost:8080/feeds/video?overlay=none&showVisualizer=false
```

**Layer 3 (Top):** Track Info
```
http://localhost:8080/feeds/track?theme=transparent&direction=ltr&speed=5
```

**Best for:** Professional broadcast-style streams with multiple visual elements

---

## OBS Setup Instructions

### Adding Vinylfo as a Browser Source

1. **Open OBS Studio**
2. In the **Sources** panel, click the **+** button
3. Select **Browser**
4. Enter a name (e.g., "Vinylfo Video Feed")
5. Click **OK**

### Configuration Dialog

| Setting | Value |
|---------|-------|
| **URL** | `http://localhost:8080/feeds/video?overlay=bottom&theme=dark` |
| **Width** | `1920` (or your canvas width) |
| **Height** | `1080` (or your canvas height) |
| **Control audio via OBS** | ✓ Enabled (recommended) |

### Recommended OBS Settings

1. **Custom CSS:** Leave empty (Vinylfo handles styling)
2. **Shutdown source when not visible:** Unchecked
3. **Refresh browser when scene becomes active:** Checked

### Audio Setup

To route Vinylfo audio through OBS:

1. In the Browser Source properties, check **Control audio via OBS**
2. In OBS **Audio Mixer**, you'll see a "Browser" channel
3. Adjust levels as needed
4. Use **Advanced Audio Properties** to route to specific outputs

---

## Features Overview

### Video Feed Features

#### Video Layer

- Displays YouTube videos matched to tracks
- Automatically plays/pauses with Vinylfo playback
- Quality adapts based on network (configurable)
- No YouTube controls visible (clean overlay)

#### Album Art Fallback

When a track has no YouTube video match:
- Displays album art with animated "Ken Burns" effect
- Smooth crossfade between tracks
- Visualizer continues on top

#### Track Info Overlay

Shows current track information:
- Track title
- Artist name (normalized, e.g., removes "(2)" suffixes)
- Album title
- Album art thumbnail

**Position options:**
- `bottom` - Standard lower third
- `top` - Upper overlay
- `none` - Hidden

**Themes:**
- `dark` - Black translucent background
- `light` - White translucent background
- `transparent` - No background (for chroma key)

#### Audio Visualizer

Real-time frequency visualization:
- Animated bar graph
- Responds to audio levels
- Can be toggled on/off
- Optional on all themes

#### Transitions

Animations when changing tracks:
- `fade` - Smooth opacity crossfade (recommended)
- `slide` - Horizontal slide animation
- `none` - Instant cut

---

### Album Art Feed Features

#### Full-Screen Display

- Displays album art at full viewport size
- Options for `contain` or `cover` fit
- Works with any aspect ratio

#### Ken Burns Animation

- Slow pan and zoom effect
- Configurable duration (5-120 seconds)
- Can be disabled with `animation=false`

#### Themes

- `dark` - Dark gradient background
- `light` - Light gradient background
- `transparent` - No background

#### Idle State

When no track is playing:
- Animated vinyl record icon
- "No track playing" message

---

### Track Info Feed Features

#### Infinite Marquee

- Smooth continuous scrolling text
- Duplicated content for seamless loop
- Scroll direction configurable (rtl/ltr)

#### Speed Control

- 10 speed levels (1=slowest, 10=fastest)
- Maps to animation duration (60s at speed 1, 10s at speed 10)
- Clamped to 10-60 second range

#### Customizable Content

- Toggle artist, album, duration display
- Custom separator character
- Custom prefix text

#### Themes

- `dark` - White text on dark
- `light` - Dark text on light
- `transparent` - White text with shadow (for any background)

---

### Connection Status

All feeds include automatic reconnection handling:
- Shows "Connecting..." indicator if SSE disconnects
- Auto-reconnects with exponential backoff
- Minimal visual disruption

All three feeds share the same SSE connection pattern for synchronized updates.

---

## Troubleshooting

### Video Not Playing

**Problem:** Black screen or "No track playing" message

**Solutions:**
1. Ensure Vinylfo server is running
2. Start playback in Vinylfo main interface
3. Check that the track has a YouTube match

### Album Art Not Showing

**Problem:** Album art feed shows vinyl icon or no image

**Solutions:**
1. Ensure tracks have album art assigned
2. Check album art file is valid
3. Try cache-busting: change `?v=` parameter
4. Verify album art URL is accessible

### Marquee Not Scrolling

**Problem:** Track info feed shows static text instead of scrolling

**Solutions:**
1. Check if content width exceeds container (marquee only scrolls if text is wider than viewport)
2. Try shorter prefix or disable some fields (`showAlbum=false`)
3. Verify `direction` parameter is set correctly
4. Check browser console for animation errors

### Scroll Speed Issues

**Problem:** Marquee scrolls too fast or too slow

**Solutions:**
1. Adjust `speed` parameter (1-10)
2. Lower speed value = longer animation duration
3. Speed 1 = ~60s cycle, Speed 10 = ~10s cycle

### Audio Not Working

**Problem:** No audio from video feed

**Solutions:**
1. Check "Control audio via OBS" is enabled
2. Ensure OBS audio levels are not muted
3. Check system audio output settings

### CORS Errors in Console

**Problem:** Console shows CORS errors related to googleads.g.doubleclick.net

**Explanation:** These are YouTube's internal ad tracking requests. They are blocked by browser security but do not affect playback.

**Solution:** These errors are harmless. Video and audio will work normally.

### OBS Not Showing Updates

**Problem:** Video feed doesn't update when you change tracks in Vinylfo

**Solutions:**
1. Check both Vinylfo and OBS are on the same computer
2. Refresh the browser source: Right-click → Refresh
3. Check the URL is correct with `localhost`
4. Verify SSE connection is working (check browser console)

### Chroma Key Not Working

**Problem:** Transparent theme not keying properly

**Solutions:**
1. Use `theme=transparent` option
2. Apply Chroma Key filter to the Browser Source
3. Adjust similarity and smoothness settings
4. Ensure overlay is not set to `none`

### Poor Video Quality

**Problem:** Video looks pixelated or blurry

**Solutions:**
1. Set `quality=high` in URL
2. Increase browser source resolution
3. Check network connection speed

### High CPU Usage

**Problem:** OBS running slowly with video feed

**Solutions:**
1. Set `showVisualizer=false`
2. Use `transition=none` instead of fade/slide
3. Reduce browser source resolution
4. Close other browser tabs
5. For album art feed: set `animation=false`

### Multiple Feeds Out of Sync

**Problem:** Video, album art, and track feeds show different tracks

**Solutions:**
1. Refresh all browser sources
2. All feeds connect to same SSE endpoint and should sync automatically
3. Check network connectivity between OBS and Vinylfo server

---

## Technical Notes

### How It Works

1. **SSE Connection:** All feeds maintain a Server-Sent Events connection to receive real-time updates at `/feeds/video/events`
2. **Shared Event Stream:** Video, album art, and track feeds all use the same SSE endpoint for synchronized updates
3. **YouTube IFrame:** Videos embed via YouTube's iframe API with no external controls
4. **Auto-Sync:** Video position syncs with Vinylfo's playback position

### Feed Comparison

| Feature | Video Feed | Album Art Feed | Track Feed |
|---------|------------|----------------|------------|
| Full-screen video | Yes | No | No |
| Album art display | Yes (fallback) | Yes | No |
| Scrolling text | No | No | Yes |
| YouTube playback | Yes | No | No |
| Visualizer | Yes | No | No |
| Best for | Video content | Background art | Text overlays |

### Port Requirements

- Default port: `8080`
- SSE uses: `/feeds/video/events` endpoint
- All traffic: Localhost only (no external servers)

### Browser Compatibility

Tested and working on:
- Chrome/Edge (recommended)
- Firefox
- OBS built-in browser (Chromium-based)

### Performance Considerations

- **Video feed:** Highest resource usage (YouTube video decoding)
- **Album art feed:** Low resource usage, minimal with `animation=false`
- **Track feed:** Very low resource usage (just text rendering)

For best performance, use only the feeds you need.

---

## Advanced Configuration

### Custom CSS in OBS

While not required, you can add custom CSS for advanced customization:

**Video Feed:**
```css
/* Change track title font */
#track-title {
    font-family: 'Your Font Name', sans-serif;
    font-size: 32px;
}

/* Hide connection status */
#connection-status {
    display: none !important;
}
```

**Album Art Feed:**
```css
/* Remove animation */
#album-art-wrapper {
    animation: none !important;
}

/* Custom border */
#album-art {
    border: 4px solid #fff;
    border-radius: 0 !important;
}
```

**Track Feed:**
```css
/* Change font and size */
#track-text {
    font-family: 'Roboto Mono', monospace;
    font-size: 36px;
}

/* Remove text shadow for chroma key */
.theme-transparent #track-text {
    text-shadow: none;
}
```

Add CSS in the Browser Source properties under **Custom CSS**.

### Multiple Synchronized Feeds

You can add multiple browser sources that stay synchronized:

```bash
# Main video feed
http://localhost:8080/feeds/video?overlay=bottom

# Album art background (use as layer below video)
http://localhost:8080/feeds/art?theme=transparent

# Track info scrolling (use as overlay)
http://localhost:8080/feeds/track?theme=transparent&direction=ltr
```

**Tip:** Use **Move** filter to reorder layers in OBS.

### Hiding Connection Status

For cleaner overlays, hide the connection status indicator:

```css
/* All feeds */
#connection-status {
    display: none !important;
}
```

### Customizing Album Art Animation

```css
/* Shorter, faster animation */
#album-art-wrapper {
    animation-duration: 10s !important;
}

/* Zoom effect only (no pan) */
@keyframes kenBurns {
    0% { transform: scale(1); }
    100% { transform: scale(1.15); }
}
```

### Customizing Marquee Speed

The speed parameter controls animation duration:
- Speed 1 = 60 second cycle
- Speed 5 = 30 second cycle
- Speed 10 = 10 second cycle

For more control, use CSS:

```css
/* Force specific duration */
.marquee-content.animating {
    animation-duration: 45s !important;
}
```

### Integrating with Other Sources

All feeds are designed to work alongside other OBS sources:
- Use **Crop/Pad** to resize
- Use **Color Correction** filters for matching
- Use **Blend Modes** for creative effects
- Use **Move** filter for layering

---

## Support

For issues or questions:

1. Check browser console for error messages (F12 → Console)
2. Verify Vinylfo server is running without errors
3. Check GitHub issues for known problems
4. When reporting issues, specify which feed(s) you're using

---

*Last updated: January 2026*
