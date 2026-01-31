/**
 * Vinylfo Track Feed
 * Handles SSE connection and infinite marquee display for OBS streaming
 */

class TrackFeedManager {
    constructor() {
        this.config = {
            theme: document.body.dataset.theme || 'dark',
            speed: parseInt(document.body.dataset.speed) || 5,
            separator: document.body.dataset.separator || '*',
            showDuration: document.body.dataset.showDuration !== 'false',
            showAlbum: document.body.dataset.showAlbum !== 'false',
            showArtist: document.body.dataset.showArtist !== 'false',
            direction: document.body.dataset.direction || 'rtl',
            prefix: document.body.dataset.prefix || 'Now Playing:',
            suffix: document.body.dataset.suffix || '',
            showBackground: document.body.dataset.showBackground !== 'false'
        };

        this.currentTrackId = null;
        this.eventSource = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.isAnimating = false;

        this.elements = {
            container: document.getElementById('track-feed-container'),
            marqueeWrapper: document.querySelector('.marquee-wrapper'),
            marqueeContainer: document.querySelector('.marquee-container'),
            marqueeContent: document.getElementById('marquee-content'),
            trackText1: document.getElementById('track-text-1'),
            trackText2: document.getElementById('track-text-2'),
            separator1: document.getElementById('separator-1'),
            separator2: document.getElementById('separator-2'),
            noTrackOverlay: document.getElementById('no-track-overlay'),
            connectionStatus: document.getElementById('connection-status')
        };

        this.init();
    }

    init() {
        console.log('[TrackFeed] Initializing with config:', JSON.stringify(this.config));

        document.body.classList.add('theme-' + this.config.theme);
        document.body.setAttribute('data-direction', this.config.direction);

        const animDuration = this.calculateAnimDuration();
        document.body.style.setProperty('--marquee-duration', animDuration + 's');

        // Check for demo track
        const demoTrackId = document.body.dataset.demoTrack;
        console.log('[TrackFeed] Checking for demo track, dataset:', JSON.stringify(document.body.dataset));
        if (demoTrackId) {
            console.log('[TrackFeed] Demo track ID found:', demoTrackId);
            this.loadDemoTrack(demoTrackId);
        } else {
            console.log('[TrackFeed] No demo track ID found in dataset');
        }

        this.connectSSE();
    }

    async loadDemoTrack(trackId) {
        try {
            const response = await fetch(`/tracks/${trackId}`);
            if (response.ok) {
                const track = await response.json();
                console.log('[TrackFeed] Loaded demo track:', track);
                
                // Create track data in the format expected by updateTrackText
                const trackData = {
                    track_id: track.id,
                    track_title: track.title,
                    artist: track.album_artist || 'Unknown Artist',
                    album_title: track.album_title || 'Unknown Album',
                    duration: track.duration
                };
                
                this.updateTrackText(trackData);
            } else {
                console.error('[TrackFeed] Failed to load demo track:', response.status);
            }
        } catch (error) {
            console.error('[TrackFeed] Error loading demo track:', error);
        }
    }

    calculateAnimDuration() {
        const minDuration = 10;
        const maxDuration = 60;
        const minSpeed = 1;
        const maxSpeed = 10;

        let duration;
        if (this.config.speed <= 5) {
            const t = (5 - this.config.speed) / (5 - minSpeed);
            duration = 60 - t * (60 - 30);
        } else {
            const t = (this.config.speed - 5) / (maxSpeed - 5);
            duration = 30 - t * (30 - 10);
        }

        return Math.max(minDuration, Math.min(maxDuration, duration));
    }

    connectSSE() {
        if (this.eventSource) {
            this.eventSource.close();
        }

        console.log('[TrackFeed] Connecting to SSE...');
        this.eventSource = new EventSource('/feeds/video/events');

        this.eventSource.onopen = () => {
            console.log('[TrackFeed] SSE connected');
            this.reconnectAttempts = 0;
            this.hideConnectionStatus();
        };

        this.eventSource.onmessage = (event) => {
            console.log('[TrackFeed] SSE raw message received:', event.data);
            try {
                const data = JSON.parse(event.data);
                console.log('[TrackFeed] SSE parsed event type:', data.type);
                this.handleSSEEvent(data);
            } catch (e) {
                console.error('[TrackFeed] Error parsing SSE event:', e);
            }
        };

        this.eventSource.onerror = () => {
            console.error('[TrackFeed] SSE connection error');
            this.showConnectionStatus();
            this.scheduleReconnect();
        };
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('[TrackFeed] Max reconnect attempts reached');
            return;
        }

        const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
        this.reconnectAttempts++;
        console.log('[TrackFeed] Reconnecting in ' + delay + 'ms (attempt ' + this.reconnectAttempts + ')');
        setTimeout(() => this.connectSSE(), delay);
    }

    handleSSEEvent(event) {
        console.log('[TrackFeed] SSE event:', event.type, event.data);
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
                this.updateTrackText(track);
            }
        } else {
            this.handleNoTrack();
        }
    }

    updateTrackText(track) {
        console.log('[TrackFeed] Updating track text for:', track.track_title);

        this.elements.noTrackOverlay.classList.add('hidden');

        let text = this.config.prefix + ' ';

        if (this.config.showArtist && track.artist) {
            text += track.artist + ' ' + this.config.separator + ' ';
        }

        text += track.track_title;

        if (this.config.showAlbum && track.album_title) {
            text += ' ' + this.config.separator + ' ' + track.album_title;
        }

        if (this.config.showDuration && track.duration) {
            const mins = Math.floor(track.duration / 60);
            const secs = track.duration % 60;
            text += ' (' + mins + ':' + (secs < 10 ? '0' : '') + secs + ')';
        }

        // Add suffix if provided
        if (this.config.suffix && this.config.suffix.trim() !== '') {
            text += ' ' + this.config.suffix;
        }

        this.elements.trackText1.textContent = text;
        this.elements.trackText2.textContent = text;
        this.elements.separator1.textContent = '  ';
        this.elements.separator2.textContent = '  ';

        this.stopAnimation();

        requestAnimationFrame(() => {
            this.startAnimation();
        });
    }

    startAnimation() {
        const contentWidth = this.elements.marqueeContent.offsetWidth;
        const containerWidth = this.elements.marqueeWrapper.offsetWidth;

        console.log('[TrackFeed] Content width:', contentWidth, 'Container width:', containerWidth);

        if (contentWidth > containerWidth) {
            this.elements.marqueeContent.classList.add('animating');
            this.isAnimating = true;
        } else {
            console.log('[TrackFeed] Content fits, no animation needed');
        }
    }

    stopAnimation() {
        this.elements.marqueeContent.classList.remove('animating');
        this.isAnimating = false;
    }

    handleNoTrack() {
        console.log('[TrackFeed] No track playing');
        this.currentTrackId = null;
        this.stopAnimation();
        this.elements.trackText1.textContent = '';
        this.elements.trackText2.textContent = '';
        this.elements.separator1.textContent = '';
        this.elements.separator2.textContent = '';
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
    new TrackFeedManager();
});
