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
        console.log('[AlbumArtFeed] Initializing with config:', this.config);

        document.body.classList.add('theme-' + this.config.theme);

        document.body.style.setProperty('--anim-duration', this.config.animDuration + 's');

        if (!this.config.animation) {
            document.body.setAttribute('data-animation', 'false');
        }

        this.connectSSE();
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
        console.log('[AlbumArtFeed] Showing album art for track:', track.track_title);

        this.elements.noTrackOverlay.classList.add('hidden');

        const albumArtUrl = track.album_art_url;
        const cacheBustedUrl = albumArtUrl + '?v=' + track.track_id;

        const img = new Image();
        img.onload = () => {
            this.elements.albumArt.src = cacheBustedUrl;
            this.elements.albumArt.classList.add('loaded');
        };
        img.onerror = () => {
            console.error('[AlbumArtFeed] Failed to load album art:', cacheBustedUrl);
            this.handleNoTrack();
        };
        img.src = cacheBustedUrl;
    }

    handleNoTrack() {
        console.log('[AlbumArtFeed] No track playing');
        this.currentTrackId = null;
        this.elements.albumArt.classList.remove('loaded');
        this.elements.albumArt.src = '';
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
