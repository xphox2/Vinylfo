/**
 * Vinylfo Album Art Feed
 * Handles SSE connection and album art display for OBS streaming
 */

class AlbumArtFeedManager {
    constructor() {
        this.config = {
            theme: document.body.dataset.theme || 'dark',
            animation: document.body.dataset.animation !== 'false',
            animDuration: parseInt(document.body.dataset.animDuration) || 20,
            fit: document.body.dataset.fit || 'cover',
            showBackground: document.body.dataset.showBackground !== 'false'
        };

        this.currentTrackId = null;
        this.eventSource = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;

        this.elements = {
            container: document.getElementById('album-art-feed-container'),
            albumArtWrapper: document.getElementById('album-art-wrapper'),
            albumArt: document.getElementById('album-art'),
            noTrackOverlay: document.getElementById('no-track-overlay'),
            connectionStatus: document.getElementById('connection-status')
        };

        this.init();
    }

    init() {
        console.log('[AlbumArtFeed] Initializing with config:', JSON.stringify(this.config));

        document.body.classList.add('theme-' + this.config.theme);
        document.body.setAttribute('data-fit', this.config.fit);
        document.body.style.setProperty('--anim-duration', this.config.animDuration + 's');

        if (!this.config.animation) {
            document.body.setAttribute('data-animation', 'false');
        }

        // Check for demo track
        const demoTrackId = document.body.dataset.demoTrack;
        console.log('[AlbumArtFeed] Checking for demo track, dataset:', JSON.stringify(document.body.dataset));
        if (demoTrackId) {
            console.log('[AlbumArtFeed] Demo track ID found:', demoTrackId);
            this.loadDemoTrack(demoTrackId);
        } else {
            console.log('[AlbumArtFeed] No demo track ID found in dataset');
        }

        this.connectSSE();
    }

    async loadDemoTrack(trackId) {
        try {
            const response = await fetch(`/tracks/${trackId}`);
            if (response.ok) {
                const track = await response.json();
                console.log('[AlbumArtFeed] Loaded demo track:', track);
                
                // Create track data in the format expected by showAlbumArt
                const trackData = {
                    track_id: track.id,
                    track_title: track.title,
                    artist: track.album_artist || 'Unknown Artist',
                    album_title: track.album_title || 'Unknown Album',
                    album_art_url: track.album_cover || '/icons/vinyl-icon.png'
                };
                
                console.log('[AlbumArtFeed] Showing album art, URL:', trackData.album_art_url);
                this.showAlbumArt(trackData);
            } else {
                console.error('[AlbumArtFeed] Failed to load demo track:', response.status);
            }
        } catch (error) {
            console.error('[AlbumArtFeed] Error loading demo track:', error);
        }
    }

    connectSSE() {
        if (this.eventSource) {
            this.eventSource.close();
        }

        console.log('[AlbumArtFeed] Connecting to SSE...');
        this.eventSource = new EventSource('/feeds/video/events');

        this.eventSource.onopen = () => {
            console.log('[AlbumArtFeed] SSE connected');
            this.reconnectAttempts = 0;
            this.hideConnectionStatus();
        };

        this.eventSource.onmessage = (event) => {
            console.log('[AlbumArtFeed] SSE raw message received:', event.data);
            try {
                const data = JSON.parse(event.data);
                console.log('[AlbumArtFeed] SSE parsed event type:', data.type);
                this.handleSSEEvent(data);
            } catch (e) {
                console.error('[AlbumArtFeed] Error parsing SSE event:', e);
            }
        };

        this.eventSource.onerror = () => {
            console.error('[AlbumArtFeed] SSE connection error');
            this.showConnectionStatus();
            this.scheduleReconnect();
        };
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('[AlbumArtFeed] Max reconnect attempts reached');
            return;
        }

        const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
        this.reconnectAttempts++;
        console.log('[AlbumArtFeed] Reconnecting in ' + delay + 'ms (attempt ' + this.reconnectAttempts + ')');
        setTimeout(() => this.connectSSE(), delay);
    }

    handleSSEEvent(event) {
        console.log('[AlbumArtFeed] SSE event:', event.type, event.data);
        switch (event.type) {
            case 'initial_state':
            case 'track_changed':
                this.handleTrackUpdate(event.data);
                break;
            case 'no_track':
                this.handleNoTrack();
                break;
            case 'playback_state':
            case 'position_update':
                break;
        }
    }

    handleTrackUpdate(data) {
        if (data && data.track) {
            const track = data.track;
            const newTrackId = track.track_id;

            if (this.currentTrackId !== newTrackId) {
                this.currentTrackId = newTrackId;
                this.showAlbumArt(track);
            }
        } else {
            this.handleNoTrack();
        }
    }

    showAlbumArt(track) {
        console.log('[AlbumArtFeed] Showing album art for track:', track.track_title, 'URL:', track.album_art_url || track.AlbumArtURL || track.album_cover);

        this.elements.noTrackOverlay.classList.add('hidden');

        // Handle different field names from different sources
        const albumArtUrl = track.album_art_url || track.AlbumArtURL || track.album_cover;
        const placeholderUrl = '/icons/vinyl-icon.png';
        
        if (!albumArtUrl) {
            // No album art URL available, use placeholder immediately
            console.log('[AlbumArtFeed] No album art URL, using placeholder');
            this.elements.albumArt.src = placeholderUrl;
            this.elements.albumArt.classList.add('loaded');
            return;
        }

        const cacheBustedUrl = albumArtUrl + '?v=' + track.track_id;

        const img = new Image();
        img.onload = () => {
            console.log('[AlbumArtFeed] Album art loaded successfully');
            this.elements.albumArt.src = cacheBustedUrl;
            this.elements.albumArt.classList.add('loaded');
        };
        img.onerror = () => {
            console.error('[AlbumArtFeed] Failed to load album art:', cacheBustedUrl, 'using placeholder');
            this.elements.albumArt.src = placeholderUrl;
            this.elements.albumArt.classList.add('loaded');
        };
        img.src = cacheBustedUrl;
    }

    handleNoTrack() {
        console.log('[AlbumArtFeed] No track playing, showing placeholder');
        this.currentTrackId = null;
        this.elements.albumArt.classList.remove('loaded');
        // Show vinyl icon placeholder instead of empty
        this.elements.albumArt.src = '/icons/vinyl-icon.png';
        this.elements.albumArt.classList.add('loaded');
        if (this.config.showBackground) {
            this.elements.noTrackOverlay.classList.remove('hidden');
        }
    }

    showConnectionStatus() {
        this.elements.connectionStatus.classList.remove('hidden');
    }

    hideConnectionStatus() {
        this.elements.connectionStatus.classList.add('hidden');
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new AlbumArtFeedManager();
});
