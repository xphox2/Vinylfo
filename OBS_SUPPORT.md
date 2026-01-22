# Vinylfo OBS Support Guide

This document provides comprehensive documentation for using Vinylfo's video feed with OBS Studio for live streaming.

## Table of Contents

1. [Quick Start](#quick-start)
2. [URL Configuration Options](#url-configuration-options)
3. [Common OBS Setups](#common-obs-setups)
4. [OBS Setup Instructions](#obs-setup-instructions)
5. [Features Overview](#features-overview)
6. [Troubleshooting](#troubleshooting)

---

## Quick Start

The simplest way to add Vinylfo's video feed to OBS:

1. Add a **Browser Source** in OBS
2. Set the URL: `http://localhost:8080/feeds/video?overlay=bottom&theme=dark`
3. Set dimensions: `1920` width × `1080` height
4. Click **OK**

The video feed will automatically display YouTube videos synced with your Vinylfo playback.

---

## URL Configuration Options

All options are passed as URL query parameters.

### Base URL

```
http://localhost:8080/feeds/video
```

### Parameter Reference

| Parameter | Options | Default | Description |
|-----------|---------|---------|-------------|
| `overlay` | `bottom`, `top`, `none` | `bottom` | Track info overlay position |
| `theme` | `dark`, `light`, `transparent` | `dark` | Color scheme for text and backgrounds |
| `transition` | `fade`, `slide`, `none` | `fade` | Animation when track changes |
| `showVisualizer` | `true`, `false` | `true` | Show/hide audio visualizer |
| `quality` | `auto`, `high`, `medium`, `small` | `auto` | YouTube video quality preference |
| `overlayDuration` | `0`-`30` | `5` | Seconds before overlay auto-hides (0 = never) |

### Parameter Combinations

**Example URLs:**

```bash
# Full options example
http://localhost:8080/feeds/video?overlay=bottom&theme=dark&transition=fade&showVisualizer=true&quality=auto&overlayDuration=5

# Minimal setup
http://localhost:8080/feeds/video

# No overlay
http://localhost:8080/feeds/video?overlay=none

# Top overlay with light theme
http://localhost:8080/feeds/video?overlay=top&theme=light

# Transparent theme for keying
http://localhost:8080/feeds/video?theme=transparent&overlay=bottom&overlayDuration=0
```

---

## Common OBS Setups

### Setup 1: Full-Featured Stream Overlay

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

### Setup 2: Clean Album Art Focus

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

### Setup 3: Minimal Information

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

### Setup 4: Transparent Keying (Green Screen)

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

### Setup 5: Slide Transition (Dynamic Movement)

**URL:**
```
http://localhost:8080/feeds/video?overlay=bottom&theme=dark&transition=slide&showVisualizer=true
```

**Settings:**
- Transition: Slide animation- Visualizer:
 On

**Best for:** High-energy streams with dynamic visuals

---

### Setup 6: Audio Visualizer Only

**URL:**
```
http://localhost:8080/feeds/video?overlay=none&showVisualizer=true
```

**Settings:**
- Overlay: None
- Visualizer: On

**Best for:** Adding an audio visualizer to your stream without video

---

### Setup 7: Top Overlay (Lower Third)

**URL:**
```
http://localhost:8080/feeds/video?overlay=top&theme=light&transition=fade
```

**Settings:**
- Overlay: Top position
- Theme: Light for bright backgrounds

**Best for:** Overlaying on video content where bottom is covered

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

### Video Layer

- Displays YouTube videos matched to tracks
- Automatically plays/pauses with Vinylfo playback
- Quality adapts based on network (configurable)
- No YouTube controls visible (clean overlay)

### Album Art Fallback

When a track has no YouTube video match:
- Displays album art with animated "Ken Burns" effect
- Smooth crossfade between tracks
- Visualizer continues on top

### Track Info Overlay

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

### Audio Visualizer

Real-time frequency visualization:
- Animated bar graph
- Responds to audio levels
- Can be toggled on/off
- Optional on all themes

### Transitions

Animations when changing tracks:
- `fade` - Smooth opacity crossfade (recommended)
- `slide` - Horizontal slide animation
- `none` - Instant cut

### Connection Status

Automatic reconnection handling:
- Shows "Reconnecting..." indicator if SSE disconnects
- Auto-reconnects within ~10 seconds
- Minimal visual disruption

---

## Troubleshooting

### Video Not Playing

**Problem:** Black screen or "No track playing" message

**Solutions:**
1. Ensure Vinylfo server is running
2. Start playback in Vinylfo main interface
3. Check that the track has a YouTube match

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

---

## Technical Notes

### How It Works

1. **SSE Connection:** Video feed maintains a Server-Sent Events connection to receive real-time updates
2. **TabSync:** Cross-tab communication uses BroadcastChannel API for instant updates
3. **YouTube IFrame:** Videos embed via YouTube's iframe API with no external controls
4. **Auto-Sync:** Video position syncs with Vinylfo's playback position

### Port Requirements

- Default port: `8080`
- SSE uses: `/feeds/video/events` endpoint
- All traffic: Localhost only (no external servers)

### Browser Compatibility

Tested and working on:
- Chrome/Edge (recommended)
- Firefox
- OBS built-in browser (Chromium-based)

---

## Advanced Configuration

### Custom CSS in OBS

While not required, you can add custom CSS for advanced customization:

```css
/* Example: Change track title font */
#track-title {
    font-family: 'Your Font Name', sans-serif;
    font-size: 32px;
}

/* Example: Hide connection status */
#connection-status {
    display: none !important;
}
```

Add this in the Browser Source properties under **Custom CSS**.

### Multiple Video Feeds

You can add multiple browser sources with different URLs:

```bash
# Main video feed
http://localhost:8080/feeds/video?overlay=bottom

# Visualizer only
http://localhost:8080/feeds/video?overlay=none&showVisualizer=true
```

### Integrating with Other Sources

The video feed is designed to work alongside other OBS sources:
- Use **Crop/Pad** to resize
- Use **Color Correction** filters for matching
- Use **Blend Modes** for creative effects

---

## Support

For issues or questions:

1. Check browser console for error messages (F12 → Console)
2. Verify Vinylfo server is running without errors
3. Check GitHub issues for known problems

---

*Last updated: January 2026*
