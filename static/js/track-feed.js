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
            noTrackOverlay: document.getElementById('no-track-overlay'),
            connectionStatus: document.getElementById('connection-status')
        };

        this.init();
    }

    init() {
        console.log('[TrackFeed] Initializing with config:', JSON.stringify(this.config));

        document.body.classList.add('theme-' + this.config.theme);
        document.body.setAttribute('data-direction', this.config.direction);

        // Default gap between repeats.
        document.body.style.setProperty('--marquee-gap', '50px');

        // Check for demo track
        const demoTrackId = document.body.dataset.demoTrack;
        console.log('[TrackFeed] Checking for demo track, dataset:', JSON.stringify(document.body.dataset));
        if (demoTrackId) {
            console.log('[TrackFeed] Demo track ID found:', demoTrackId);
            this.loadDemoTrack(demoTrackId);

            // Settings preview/demo: don't open SSE connections.
            return;
        } else {
            console.log('[TrackFeed] No demo track ID found in dataset');
        }

        this.connectSSE();

        window.addEventListener('resize', () => {
            if (this.currentTrackId) {
                this.rebuildMarquee();
            }
        });
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

    getPixelsPerSecond() {
        // Map speed 1..10 to a reasonable pixel velocity.
        const min = 40;
        const max = 260;
        const t = (Math.max(1, Math.min(10, this.config.speed)) - 1) / 9;
        return min + t * (max - min);
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

        this.renderMarqueeText(text);
    }

    renderMarqueeText(text) {
        this.stopAnimation();

        // Build enough repeated segments to always scroll, even if the text is short.
        const wrapperWidth = this.elements.marqueeWrapper?.offsetWidth || window.innerWidth;

        // Render one segment to measure its width.
        this.elements.marqueeContent.innerHTML = '';
        const seg = document.createElement('span');
        seg.className = 'marquee-segment';
        seg.textContent = text;
        this.elements.marqueeContent.appendChild(seg);

        // Force layout.
        const segmentWidth = seg.offsetWidth || 1;

        const styles = window.getComputedStyle(this.elements.marqueeContent);
        const gapPx = parseFloat(styles.columnGap || styles.gap) || 50;
        const stepPx = segmentWidth + gapPx;

        const repeatCount = Math.max(3, Math.ceil((wrapperWidth * 2) / stepPx) + 1);

        this.elements.marqueeContent.innerHTML = '';
        const frag = document.createDocumentFragment();
        for (let i = 0; i < repeatCount; i++) {
            const s = document.createElement('span');
            s.className = 'marquee-segment';
            s.textContent = text;
            frag.appendChild(s);
        }
        this.elements.marqueeContent.appendChild(frag);

        document.body.style.setProperty('--marquee-shift', stepPx + 'px');
        const pxPerSec = this.getPixelsPerSecond();
        const durationSeconds = Math.max(2, stepPx / pxPerSec);
        document.body.style.setProperty('--marquee-duration', durationSeconds.toFixed(3) + 's');

        requestAnimationFrame(() => this.startAnimation());
    }

    rebuildMarquee() {
        const first = this.elements.marqueeContent?.querySelector('.marquee-segment');
        const text = first ? first.textContent : '';
        if (text) {
            this.renderMarqueeText(text);
        }
    }

    startAnimation() {
        this.elements.marqueeContent.classList.add('animating');
        this.isAnimating = true;
    }

    stopAnimation() {
        this.elements.marqueeContent.classList.remove('animating');
        this.isAnimating = false;
    }

    handleNoTrack() {
        console.log('[TrackFeed] No track playing');
        this.currentTrackId = null;
        this.stopAnimation();
        this.elements.marqueeContent.innerHTML = '';
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
