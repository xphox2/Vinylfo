/**
 * Vinylfo Video Feed Manager
 * Handles YouTube video playback synced with Vinylfo playback queue
 * Uses SSE for real-time updates and TabSync for tab-to-tab communication
 */

import { normalizeArtistName, normalizeTitle } from './modules/utils.js';

function cleanArtistName(artistName) {
    if (!artistName) return 'Unknown Artist';
    return normalizeArtistName(artistName) || 'Unknown Artist';
}

function cleanTrackTitle(trackTitle) {
    if (!trackTitle) return 'Unknown Track';
    return normalizeTitle(trackTitle) || 'Unknown Track';
}

class VideoFeedManager {
    constructor() {
        // Configuration from data attributes
        this.config = {
            overlay: document.body.dataset.overlay || 'bottom',
            theme: document.body.dataset.theme || 'dark',
            transition: document.body.dataset.transition || 'fade',
            showVisualizer: document.body.dataset.showVisualizer !== 'false',
            quality: document.body.dataset.quality || 'auto',
            overlayDuration: parseInt(document.body.dataset.overlayDuration) || 5,
            showBackground: document.body.dataset.showBackground !== 'false',
            enableAudio: document.body.dataset.enableAudio === 'true'
        };

        // State
        this.currentTrack = null;
        this.currentVideoId = null;
        this.isPlaying = false;
        this.isPaused = false;
        this.player = null;
        this.playerReady = false;
        this.preloadedVideoId = null;
        this.overlayTimeout = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.eventSource = null;
        this.visualizer = null;
        this.tabSync = null;

        // Operation queue to prevent race conditions
        this.pendingOperation = null;
        this.operationTimeout = null;

        // DOM Elements
        this.elements = {
            container: document.getElementById('video-feed-container'),
            videoLayer: document.getElementById('video-layer'),
            albumArtLayer: document.getElementById('album-art-layer'),
            visualizerLayer: document.getElementById('visualizer-layer'),
            loadingOverlay: document.getElementById('loading-overlay'),
            noTrackOverlay: document.getElementById('no-track-overlay'),
            trackOverlay: document.getElementById('track-overlay'),
            connectionStatus: document.getElementById('connection-status'),
            albumArt: document.getElementById('album-art'),
            overlayAlbumArt: document.getElementById('overlay-album-art'),
            trackTitle: document.getElementById('track-title'),
            trackArtist: document.getElementById('track-artist'),
            trackAlbum: document.getElementById('track-album'),
            visualizerCanvas: document.getElementById('visualizer-canvas')
        };

        // Apply theme
        this.applyTheme();

        // Initialize
        this.init();
    }

    async init() {
        console.log('[VideoFeed] Initializing with config:', JSON.stringify(this.config));

        // Initialize TabSync for tab-to-tab communication
        if (typeof TabSyncManager !== 'undefined') {
            this.tabSync = new TabSyncManager();
            window.addEventListener('vinylfo_seek', (e) => {
                const position = e.detail?.position;
                if (typeof position === 'number') {
                    console.log('[VideoFeed] Received seek from TabSync:', position);
                    this.handleTabSyncSeek(position);
                }
            });
        }

        // Initialize visualizer if enabled
        if (this.config.showVisualizer && typeof AudioVisualizer !== 'undefined') {
            this.visualizer = new AudioVisualizer(this.elements.visualizerCanvas, {
                theme: this.config.theme
            });
        }

        // Wait for YouTube API to load
        await this.waitForYouTubeAPI();

        // Connect to SSE
        this.connectSSE();

        // Fetch initial state
        await this.fetchInitialState();
    }

    handleTabSyncSeek(position) {
        if (typeof position !== 'number' || position < 0) {
            return;
        }

        if (!this.player || !this.playerReady) {
            console.log('[VideoFeed] Player not ready for seek');
            return;
        }

        this.seekVideo(position);
    }

    applyTheme() {
        document.body.classList.add(`theme-${this.config.theme}`);

        // Apply overlay position
        if (this.config.overlay !== 'none') {
            this.elements.trackOverlay.classList.add(`position-${this.config.overlay}`);
        }

        // Apply transition type
        this.elements.container.classList.add(`transition-${this.config.transition}`);
    }

    waitForYouTubeAPI() {
        return new Promise((resolve) => {
            if (window.YT && window.YT.Player) {
                resolve();
                return;
            }

            window.onYouTubeIframeAPIReady = () => {
                console.log('[VideoFeed] YouTube API ready');
                resolve();
            };
        });
    }

    initYouTubePlayer(videoId) {
        if (this.player) {
            if (videoId && this.currentVideoId !== videoId) {
                if (this.isPaused || !this.isPlaying) {
                    this.player.cueVideoById({
                        videoId: videoId,
                        suggestedQuality: this.getQualitySetting()
                    });
                } else {
                    this.player.loadVideoById({
                        videoId: videoId,
                        suggestedQuality: this.getQualitySetting()
                    });
                }
                this.currentVideoId = videoId;
            }
            return;
        }

        console.log('[VideoFeed] Initializing YouTube player with video:', videoId, 'isPaused:', this.isPaused);

        this.player = new YT.Player('youtube-player', {
            height: '100%',
            width: '100%',
            videoId: videoId || '',
            playerVars: {
                autoplay: 0,
                controls: 0,
                disablekb: 1,
                enablejsapi: 1,
                fs: 0,
                iv_load_policy: 3,
                loop: 0,
                modestbranding: 1,
                playsinline: 1,
                rel: 0,
                showinfo: 0,
                origin: window.location.origin,
                cc_load_policy: 0,
                egm: 0,
                ptl: 0,
                wmode: 'transparent'
            },
            events: {
                onReady: (event) => this.onPlayerReady(event),
                onStateChange: (event) => this.onPlayerStateChange(event),
                onError: (event) => this.onPlayerError(event)
            }
        });

        this.currentVideoId = videoId;
    }

    getQualitySetting() {
        const qualityMap = {
            '1080': 'hd1080',
            '720': 'hd720',
            '480': 'large',
            '360': 'medium',
            'auto': 'default'
        };
        return qualityMap[this.config.quality] || 'default';
    }

    onPlayerReady(event) {
        console.log('[VideoFeed] Player ready');
        this.playerReady = true;
        
        // Set volume based on enableAudio config (default muted for OBS)
        const volume = this.config.enableAudio ? 100 : 0;
        event.target.setVolume(volume);
        console.log('[VideoFeed] Audio enabled:', this.config.enableAudio, 'Volume set to:', volume);
        
        event.target.setPlaybackQuality(this.getQualitySetting());

        // Don't auto-play on initial load - user must click play
        // Only play if this is a track change, not initial load
        if (this.isPlaying && this.currentVideoId && this.currentTrack) {
            event.target.playVideo();
        }

        this.hideLoading();
    }

    onPlayerStateChange(event) {
        console.log('[VideoFeed] Player state changed:', event.data);

        switch (event.data) {
            case YT.PlayerState.PLAYING:
                this.showVideoLayer();
                break;
            case YT.PlayerState.ENDED:
                // Video ended - wait for next track from SSE
                break;
            case YT.PlayerState.BUFFERING:
                // Could show loading indicator
                break;
        }
    }

    onPlayerError(event) {
        console.error('[VideoFeed] Player error:', event.data);
        // Error codes: 2 (invalid param), 5 (HTML5 error), 100 (not found), 101/150 (embedding not allowed)
        this.showAlbumArtFallback();
    }

    connectSSE() {
        if (this.eventSource) {
            this.eventSource.close();
        }

        console.log('[VideoFeed] Connecting to SSE...');
        this.eventSource = new EventSource('/feeds/video/events');

        this.eventSource.onopen = () => {
            console.log('[VideoFeed] SSE connected');
            this.reconnectAttempts = 0;
            this.hideConnectionStatus();
        };

        this.eventSource.onmessage = (event) => {
            console.log('[VideoFeed] SSE raw message received:', event.data);
            try {
                const data = JSON.parse(event.data);
                console.log('[VideoFeed] SSE parsed event type:', data.type);
                this.handleSSEEvent(data);
            } catch (e) {
                console.error('[VideoFeed] Error parsing SSE event:', e);
            }
        };

        this.eventSource.onerror = () => {
            console.error('[VideoFeed] SSE connection error');
            this.eventSource.close();
            this.showConnectionStatus();
            this.scheduleReconnect();
        };
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('[VideoFeed] Max reconnect attempts reached');
            return;
        }

        const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
        this.reconnectAttempts++;

        console.log(`[VideoFeed] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
        setTimeout(() => this.connectSSE(), delay);
    }

    handleSSEEvent(event) {
        console.log('[VideoFeed] SSE event:', event.type, event.data);

        switch (event.type) {
            case 'initial_state':
            case 'track_changed':
                this.handleTrackUpdate(event.data);
                break;
            case 'playback_state':
                this.handlePlaybackState(event.data);
                break;
            case 'position_update':
                this.handlePositionUpdate(event.data);
                break;
            case 'no_track':
                this.showNoTrack();
                break;
        }
    }

    handleTrackUpdate(data) {
        if (!data.track) {
            this.showNoTrack();
            return;
        }

        const track = data.track;
        const trackChanged = !this.currentTrack || this.currentTrack.track_id !== track.track_id;

        this.currentTrack = track;
        this.isPlaying = data.is_playing;
        this.isPaused = data.is_paused;

        // Update overlay info
        this.updateTrackOverlay(track);

        if (trackChanged) {
            console.log('[VideoFeed] Track changed:', track.track_title);

            // Perform transition
            this.transitionToTrack(track);

            // Show overlay for new track
            if (this.config.overlay !== 'none') {
                this.showTrackOverlay();
            }

            // Preload next track
            this.preloadNextTrack();
        }

        // Queue play state operation - cancels any pending operation
        this.queuePlayStateOperation(trackChanged);
    }

    // Centralized method to apply play state - prevents race conditions
    queuePlayStateOperation(isTrackChange = false) {
        // Cancel any pending operation
        if (this.operationTimeout) {
            clearTimeout(this.operationTimeout);
            this.operationTimeout = null;
        }

        const applyPlayState = (attempt = 1) => {
            if (this.playerReady && this.player) {
                console.log('[VideoFeed] Applying play state - isPlaying:', this.isPlaying, 'isPaused:', this.isPaused);

                if (this.isPlaying && !this.isPaused) {
                    this.player.playVideo();
                } else {
                    this.player.pauseVideo();
                }
                this.operationTimeout = null;
            } else if (isTrackChange && attempt < 10) {
                // Only retry on track change, max 10 attempts
                console.log('[VideoFeed] Player not ready, retry attempt', attempt);
                this.operationTimeout = setTimeout(() => applyPlayState(attempt + 1), 150);
            } else {
                console.log('[VideoFeed] Player not ready, skipping play state update');
                this.operationTimeout = null;
            }
        };

        applyPlayState();
    }

    handlePlaybackState(data) {
        console.log('[VideoFeed] handlePlaybackState called with:', data);
        this.isPlaying = data.is_playing;
        this.isPaused = data.is_paused;

        if (data.stopped) {
            console.log('[VideoFeed] Stopped - pausing and seeking to 0');
            // Cancel any pending operation
            if (this.operationTimeout) {
                clearTimeout(this.operationTimeout);
                this.operationTimeout = null;
            }
            if (this.playerReady && this.player) {
                this.player.pauseVideo();
                this.player.seekTo(0, true);
            } else {
                console.log('[VideoFeed] Cannot stop - player not ready');
            }
            return;
        }

        // Use centralized queue to prevent race conditions
        this.queuePlayStateOperation(false);
    }

    handlePositionUpdate(data) {
        if (!this.player || !this.playerReady) {
            return;
        }

        const position = data.position;
        if (typeof position !== 'number' || position < 0) {
            return;
        }

        this.seekVideo(position);
    }

    seekVideo(position) {
        const currentTime = this.player.getCurrentTime();

        if (Math.abs(currentTime - position) <= 2) {
            return;
        }

        console.log('[VideoFeed] Seeking from', currentTime, 'to', position);

        const playerState = this.player.getPlayerState();
        const isPaused = playerState === YT.PlayerState.PAUSED;
        const isYouTube = this.currentVideoId && this.currentVideoId.length > 0;

        this.player.seekTo(position, true);

        if (!isYouTube && isPaused) {
            this.player.playVideo();
            setTimeout(() => this.player.pauseVideo(), 50);
        }
    }

    transitionToTrack(track) {
        const hasVideo = track.has_video && track.youtube_video_id;
        console.log('[VideoFeed] transitionToTrack - hasVideo:', hasVideo, 'youtube_video_id:', track.youtube_video_id);

        if (hasVideo) {
            this.transitionTo('video', () => {
                if (!this.player) {
                    this.initYouTubePlayer(track.youtube_video_id);
                } else if (this.currentVideoId !== track.youtube_video_id) {
                    this.currentVideoId = track.youtube_video_id;
                    
                    if (this.isPaused || !this.isPlaying) {
                        this.player.cueVideoById({
                            videoId: track.youtube_video_id,
                            suggestedQuality: this.getQualitySetting()
                        });
                    } else {
                        this.player.loadVideoById({
                            videoId: track.youtube_video_id,
                            suggestedQuality: this.getQualitySetting()
                        });
                    }
                }
            });
        } else {
            console.log('[VideoFeed] No YouTube video, showing album art fallback');
            this.transitionTo('album-art', () => {
                this.showAlbumArtFallback(track);
            });
        }
    }

    transitionTo(target, callback) {
        const layers = ['video', 'album-art', 'visualizer'];
        const targetLayer = this.elements[`${target}Layer`] || this.elements.albumArtLayer;

        if (this.config.transition === 'none') {
            layers.forEach(layer => {
                const el = this.elements[`${layer}Layer`];
                if (el) {
                    if (layer === target || (target === 'album-art' && layer === 'visualizer' && this.config.showVisualizer)) {
                        el.classList.remove('hidden');
                    } else {
                        el.classList.add('hidden');
                    }
                }
            });
            if (callback) callback();
            this.hideLoading();
            return;
        }

        // Fade transition
        const duration = this.config.transition === 'fade' ? 500 : 300;

        // Fade out current
        layers.forEach(layer => {
            const el = this.elements[`${layer}Layer`];
            if (el && !el.classList.contains('hidden')) {
                el.classList.add('fading-out');
            }
        });

        setTimeout(() => {
            // Hide all layers
            layers.forEach(layer => {
                const el = this.elements[`${layer}Layer`];
                if (el) {
                    el.classList.add('hidden');
                    el.classList.remove('fading-out');
                }
            });

            // Show target layer
            targetLayer.classList.remove('hidden');
            targetLayer.classList.add('fading-in');

            // Show visualizer layer if album art mode and visualizer enabled
            if (target === 'album-art' && this.config.showVisualizer) {
                this.elements.visualizerLayer.classList.remove('hidden');
                this.elements.visualizerLayer.classList.add('fading-in');
                if (this.visualizer) {
                    this.visualizer.start();
                }
            } else if (this.visualizer) {
                this.visualizer.stop();
            }

            if (callback) callback();

            setTimeout(() => {
                targetLayer.classList.remove('fading-in');
                if (this.elements.visualizerLayer) {
                    this.elements.visualizerLayer.classList.remove('fading-in');
                }
                this.hideLoading();
            }, duration);
        }, duration);
    }

    showVideoLayer() {
        this.elements.videoLayer.classList.remove('hidden');
        this.elements.albumArtLayer.classList.add('hidden');
        if (this.visualizer) {
            this.visualizer.stop();
        }
        this.hideLoading();
    }

    showAlbumArtFallback(track = null) {
        const displayTrack = track || this.currentTrack;

        // Handle different field names for album art URL
        const albumArtUrl = displayTrack?.album_art_url || displayTrack?.AlbumArtURL || displayTrack?.album_cover;
        
        if (albumArtUrl) {
            this.elements.albumArt.src = albumArtUrl;
            this.elements.albumArt.onerror = () => {
                console.log('[VideoFeed] Album art failed to load, using placeholder');
                this.elements.albumArt.src = '/icons/vinyl-icon.png';
            };
        } else {
            // No album art URL, use placeholder
            this.elements.albumArt.src = '/icons/vinyl-icon.png';
        }

        this.elements.videoLayer.classList.add('hidden');
        this.elements.albumArtLayer.classList.remove('hidden');

        if (this.config.showVisualizer) {
            this.elements.visualizerLayer.classList.remove('hidden');
            if (this.visualizer) {
                this.visualizer.start();
            }
        }

        this.hideLoading();
    }

    showNoTrack() {
        this.currentTrack = null;
        this.currentVideoId = null;

        if (this.player && this.playerReady) {
            this.player.stopVideo();
        }

        // Show placeholder vinyl icon instead of empty screen
        this.elements.albumArt.src = '/icons/vinyl-icon.png';
        
        this.elements.videoLayer.classList.add('hidden');
        this.elements.albumArtLayer.classList.remove('hidden');
        this.elements.visualizerLayer.classList.add('hidden');
        this.elements.trackOverlay.classList.add('hidden');
        
        if (this.config.showBackground) {
            this.elements.noTrackOverlay.classList.remove('hidden');
        }

        if (this.visualizer) {
            this.visualizer.stop();
        }

        this.hideLoading();
    }

    updateTrackOverlay(track) {
        this.elements.trackTitle.textContent = track.track_title || 'Unknown Track';
        this.elements.trackArtist.textContent = cleanArtistName(track.artist);
        this.elements.trackAlbum.textContent = track.album_title || '';

        if (track.album_art_url) {
            this.elements.overlayAlbumArt.src = track.album_art_url;
            this.elements.overlayAlbumArt.classList.remove('hidden');
        } else {
            this.elements.overlayAlbumArt.classList.add('hidden');
        }
    }

    showTrackOverlay() {
        if (this.config.overlay === 'none') return;

        this.elements.noTrackOverlay.classList.add('hidden');
        this.elements.trackOverlay.classList.remove('hidden');
        this.elements.trackOverlay.classList.add('visible');

        // Auto-hide after duration (unless duration is 0 = always visible)
        if (this.config.overlayDuration > 0) {
            if (this.overlayTimeout) {
                clearTimeout(this.overlayTimeout);
            }

            this.overlayTimeout = setTimeout(() => {
                this.hideTrackOverlay();
            }, this.config.overlayDuration * 1000);
        }
    }

    hideTrackOverlay() {
        this.elements.trackOverlay.classList.remove('visible');

        setTimeout(() => {
            if (!this.elements.trackOverlay.classList.contains('visible')) {
                this.elements.trackOverlay.classList.add('hidden');
            }
        }, 500);
    }

    showLoading() {
        this.elements.loadingOverlay.classList.remove('hidden');
    }

    hideLoading() {
        this.elements.loadingOverlay.classList.add('hidden');
    }

    showConnectionStatus() {
        this.elements.connectionStatus.classList.remove('hidden');
    }

    hideConnectionStatus() {
        this.elements.connectionStatus.classList.add('hidden');
    }

    async fetchInitialState() {
        // Check for demo track data in data attribute
        const demoTrackData = document.body.dataset.demoTrack;
        if (demoTrackData) {
            console.log('[VideoFeed] Demo track data found:', demoTrackData);
            try {
                const track = JSON.parse(demoTrackData);
                this.handleTrackUpdate({
                    track: track,
                    is_playing: false,
                    is_paused: true
                });
                return;
            } catch (e) {
                console.error('[VideoFeed] Error parsing demo track data:', e);
            }
        }

        // Check for demo track ID
        const demoTrackId = document.body.dataset.demoTrackId;
        if (demoTrackId) {
            console.log('[VideoFeed] Demo track ID found:', demoTrackId);
            await this.loadDemoTrack(demoTrackId);
            return;
        }

        // Normal operation - fetch current playback state
        try {
            const response = await fetch('/playback/current-youtube');
            const data = await response.json();

            if (data.has_track && data.track) {
                this.handleTrackUpdate({
                    track: data.track,
                    is_playing: data.is_playing,
                    is_paused: data.is_paused
                });
            } else {
                this.showNoTrack();
            }
        } catch (error) {
            console.error('[VideoFeed] Error fetching initial state:', error);
            this.showNoTrack();
        }
    }

    async loadDemoTrack(trackId) {
        try {
            const response = await fetch(`/tracks/${trackId}`);
            if (response.ok) {
                const track = await response.json();
                console.log('[VideoFeed] Loaded demo track:', track);
                console.log('[VideoFeed] YouTube video ID:', track.youtube_video_id);
                
                // Create track data in the format expected by handleTrackUpdate
                const trackData = {
                    track_id: track.id,
                    track_title: track.title,
                    artist: track.album_artist || 'Unknown Artist',
                    album_title: track.album_title || 'Unknown Album',
                    album_art_url: track.album_cover || track.album_art_url || '/icons/vinyl-icon.png',
                    duration: track.duration,
                    has_video: track.youtube_video_id ? true : false,
                    youtube_video_id: track.youtube_video_id,
                    video_title: track.video_title,
                    video_duration: track.video_duration
                };
                
                console.log('[VideoFeed] Demo track data:', trackData);
                
                this.handleTrackUpdate({
                    track: trackData,
                    is_playing: false,
                    is_paused: true
                });
            } else {
                console.error('[VideoFeed] Failed to load demo track:', response.status);
                this.showNoTrack();
            }
        } catch (error) {
            console.error('[VideoFeed] Error loading demo track:', error);
            this.showNoTrack();
        }
    }

    async preloadNextTrack() {
        try {
            const response = await fetch('/playback/next-preload');
            const data = await response.json();

            if (data.has_next && data.track && data.track.youtube_video_id) {
                this.preloadedVideoId = data.track.youtube_video_id;
                console.log('[VideoFeed] Preloaded next video:', this.preloadedVideoId);
            }
        } catch (error) {
            console.error('[VideoFeed] Error preloading next track:', error);
        }
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.videoFeedManager = new VideoFeedManager();
});
