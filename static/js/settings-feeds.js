const API_BASE = '/api';

class FeedSettingsManager {
    constructor() {
        this.currentSettings = {
            video: {},
            art: {},
            track: {}
        };
        this.tracks = [];
        this.baseUrl = window.location.origin;
        this.init();
    }

    async init() {
        await this.loadFeedSettings();
        await this.loadTracks();
        this.bindEvents();
        this.updateAllPreviews();
        this.updateAllUrls();
    }

    async loadFeedSettings() {
        try {
            const response = await fetch(`${API_BASE}/settings/feeds`);
            const data = await response.json();
            
            if (response.ok) {
                this.currentSettings = data;
                this.applySettingsToUI(data);
            } else {
                console.error('Failed to load feed settings:', data.error);
                this.showNotification('Failed to load feed settings', 'error');
            }
        } catch (error) {
            console.error('Error loading feed settings:', error);
            this.showNotification('Failed to load feed settings', 'error');
        }
    }

    async loadTracks() {
        try {
            const response = await fetch('/tracks?limit=100');
            const data = await response.json();
            
            // API returns tracks in 'data' property, not 'tracks'
            if (response.ok && data.data && Array.isArray(data.data)) {
                this.tracks = data.data;
                console.log('Loaded', this.tracks.length, 'tracks');
                this.populateTrackSelects();
            } else {
                console.error('Failed to load tracks:', data);
                this.showTrackLoadError('Failed to load tracks');
            }
        } catch (error) {
            console.error('Error loading tracks:', error);
            this.showTrackLoadError('Error loading tracks: ' + error.message);
        }
    }

    showTrackLoadError(message) {
        const selects = document.querySelectorAll('.sample-track-select');
        selects.forEach(select => {
            select.innerHTML = `<option value="">${message}</option>`;
        });
    }

    async getCurrentlyPlaying() {
        try {
            const response = await fetch('/playback/current');
            const data = await response.json();
            
            if (response.ok && data.track) {
                return data.track;
            }
        } catch (error) {
            console.error('Error getting currently playing:', error);
        }
        return null;
    }

    populateTrackSelects() {
        const selects = document.querySelectorAll('.sample-track-select');
        
        if (this.tracks.length === 0) {
            // No tracks in database
            selects.forEach(select => {
                select.innerHTML = '<option value="">No tracks in library</option>';
            });
            return;
        }
        
        const options = this.tracks.map(track => 
            `<option value="${track.id}">${track.album_artist || 'Unknown'} - ${track.title}</option>`
        ).join('');
        
        selects.forEach(select => {
            select.innerHTML = `<option value="">-- Select a track --</option>${options}`;
            // Auto-select first track if available
            if (this.tracks.length > 0) {
                select.value = this.tracks[0].id;
            }
        });
        
        // Trigger preview updates after populating and selecting default
        this.updateAllPreviews();
        this.updateAllUrls();
    }

    applySettingsToUI(settings) {
        // Video feed settings
        if (settings.video) {
            this.setSelectValue('video-theme', settings.video.theme);
            this.setSelectValue('video-overlay', settings.video.overlay);
            this.setSelectValue('video-transition', settings.video.transition);
            this.setSelectValue('video-quality', settings.video.quality);
            this.setCheckboxValue('video-show-visualizer', settings.video.show_visualizer);
            this.setRangeValue('video-overlay-duration', settings.video.overlay_duration);
            this.setCheckboxValue('video-show-background', settings.video.show_background);
            this.setCheckboxValue('video-enable-audio', settings.video.enable_audio);
        }

        // Art feed settings
        if (settings.art) {
            this.setSelectValue('art-theme', settings.art.theme);
            this.setCheckboxValue('art-animation', settings.art.animation);
            this.setRangeValue('art-anim-duration', settings.art.anim_duration);
            this.setSelectValue('art-fit', settings.art.fit);
            this.setCheckboxValue('art-show-background', settings.art.show_background);
        }

        // Track feed settings
        if (settings.track) {
            this.setSelectValue('track-theme', settings.track.theme);
            this.setRangeValue('track-speed', settings.track.speed);
            this.setSelectValue('track-direction', settings.track.direction);
            document.getElementById('track-separator').value = settings.track.separator || '*';
            document.getElementById('track-prefix').value = settings.track.prefix || 'Now Playing:';
            document.getElementById('track-suffix').value = settings.track.suffix || '';
            this.setCheckboxValue('track-show-artist', settings.track.show_artist);
            this.setCheckboxValue('track-show-album', settings.track.show_album);
            this.setCheckboxValue('track-show-duration', settings.track.show_duration);
            this.setCheckboxValue('track-show-background', settings.track.show_background);
        }
    }

    setSelectValue(id, value) {
        const element = document.getElementById(id);
        if (element && value) {
            element.value = value;
        }
    }

    setCheckboxValue(id, value) {
        const element = document.getElementById(id);
        if (element) {
            if (element.type === 'checkbox') {
                element.checked = value;
            } else if (element.tagName === 'LABEL' && element.classList.contains('toggle-label')) {
                element.setAttribute('data-checked', value.toString());
            }
        }
    }

    setRangeValue(id, value) {
        const element = document.getElementById(id);
        if (element && value) {
            element.value = value;
            const valueDisplay = document.getElementById(`${id}-value`);
            if (valueDisplay) {
                valueDisplay.textContent = element.dataset.param === 'speed' ? value : `${value}s`;
            }
        }
    }

    bindEvents() {
        // Accordion toggle
        window.toggleAccordion = (feed) => {
            const section = document.getElementById(`${feed}-feed-section`);
            section.classList.toggle('collapsed');
        };

        // Input change events for all feeds
        ['video', 'art', 'track'].forEach(feed => {
            const inputs = document.querySelectorAll(`[data-feed="${feed}"]`);
            inputs.forEach(input => {
                input.addEventListener('change', () => {
                    this.updatePreview(feed);
                    this.updateUrl(feed);
                    this.updateCurrentSettings(feed);
                });

                // For range inputs, update the display value
                if (input.type === 'range') {
                    input.addEventListener('input', () => {
                        const valueDisplay = document.getElementById(`${input.id}-value`);
                        if (valueDisplay) {
                            const suffix = input.dataset.param === 'speed' ? '' : 's';
                            valueDisplay.textContent = `${input.value}${suffix}`;
                        }
                    });
                }
            });
        });

        // Initialize toggle switches based on data-default
        document.querySelectorAll('.toggle-label[data-toggle]').forEach(label => {
            const defaultValue = label.dataset.default === 'true';
            label.setAttribute('data-checked', defaultValue.toString());
            
            // Add click handler for toggle
            label.addEventListener('click', (e) => {
                const current = label.getAttribute('data-checked') === 'true';
                label.setAttribute('data-checked', (!current).toString());
                
                // Update preview and URL
                const toggleId = label.dataset.toggle;
                const feed = toggleId.split('-')[0];
                this.updatePreview(feed);
                this.updateUrl(feed);
            });
        });

        // Copy buttons
        document.querySelectorAll('.copy-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const feed = e.target.dataset.feed;
                this.copyUrl(feed);
            });
        });

        // Use currently playing buttons
        document.querySelectorAll('[id$="-use-current"]').forEach(btn => {
            btn.addEventListener('click', async (e) => {
                const feed = e.target.id.replace('-use-current', '');
                const currentTrack = await this.getCurrentlyPlaying();
                if (currentTrack) {
                    const select = document.getElementById(`${feed}-sample-track`);
                    select.value = currentTrack.id;
                    this.updatePreview(feed);
                } else {
                    this.showNotification('No track currently playing', 'info');
                }
            });
        });

        // Sample track changes
        document.querySelectorAll('.sample-track-select').forEach(select => {
            select.addEventListener('change', (e) => {
                const feed = e.target.id.replace('-sample-track', '');
                this.updatePreview(feed);
            });
        });

        // Save button
        document.getElementById('save-feed-settings').addEventListener('click', () => this.saveSettings());

        // Collapse all sections by default except the first one
        document.getElementById('art-feed-section').classList.add('collapsed');
        document.getElementById('track-feed-section').classList.add('collapsed');
    }

    updateCurrentSettings(feed) {
        const settings = {};
        const inputs = document.querySelectorAll(`[data-feed="${feed}"]`);
        
        inputs.forEach(input => {
            const param = input.dataset.param;
            if (input.type === 'checkbox') {
                settings[param] = input.checked;
            } else {
                settings[param] = input.value;
            }
        });

        this.currentSettings[feed] = settings;
    }

    getFeedParams(feed) {
        const params = {};
        const inputs = document.querySelectorAll(`[data-feed="${feed}"]`);
        
        inputs.forEach(input => {
            const param = input.dataset.param;
            if (input.type === 'checkbox') {
                params[param] = input.checked;
            } else {
                params[param] = input.value;
            }
        });
        
        // Also check toggle labels with data-toggle attribute
        document.querySelectorAll(`.toggle-label[data-toggle^="${feed}-"]`).forEach(label => {
            const param = label.dataset.param;
            if (param) {
                params[param] = label.getAttribute('data-checked') === 'true';
            }
        });

        return params;
    }

    buildUrl(feed, params) {
        const basePath = {
            video: '/feeds/video',
            art: '/feeds/art',
            track: '/feeds/track'
        }[feed];

        const queryParams = new URLSearchParams();
        
        Object.entries(params).forEach(([key, value]) => {
            if (typeof value === 'boolean') {
                queryParams.append(key, value.toString());
            } else if (value && value !== '') {
                queryParams.append(key, value);
            }
        });

        const queryString = queryParams.toString();
        return `${this.baseUrl}${basePath}${queryString ? '?' + queryString : ''}`;
    }

    updatePreview(feed) {
        const params = this.getFeedParams(feed);
        const url = this.buildUrl(feed, params);
        
        // Add demoTrack parameter for iframe preview only
        const sampleTrack = document.getElementById(`${feed}-sample-track`);
        let previewUrl = url;
        if (!(sampleTrack && sampleTrack.value)) {
            const iframe = document.getElementById(`${feed}-preview`);
            if (iframe && iframe.src !== 'about:blank') {
                iframe.src = 'about:blank';
            }
            return;
        }

        const separator = url.includes('?') ? '&' : '?';
        previewUrl = `${url}${separator}demoTrack=${sampleTrack.value}`;
         
        const iframe = document.getElementById(`${feed}-preview`);
        
        if (iframe) {
            // Only update if URL has changed to avoid flickering
            const currentSrc = iframe.src;
            if (currentSrc !== previewUrl) {
                iframe.src = previewUrl;
            }
        }
    }

    updateAllPreviews() {
        ['video', 'art', 'track'].forEach(feed => {
            this.updatePreview(feed);
        });
    }

    updateUrl(feed) {
        const params = this.getFeedParams(feed);
        const url = this.buildUrl(feed, params);
        const urlInput = document.getElementById(`${feed}-url`);
        
        if (urlInput) {
            urlInput.value = url;
        }
    }

    updateAllUrls() {
        ['video', 'art', 'track'].forEach(feed => {
            this.updateUrl(feed);
        });
    }

    async copyUrl(feed) {
        const urlInput = document.getElementById(`${feed}-url`);
        
        if (urlInput) {
            try {
                await navigator.clipboard.writeText(urlInput.value);
                this.showNotification('URL copied to clipboard!', 'success');
            } catch (err) {
                // Fallback for older browsers
                urlInput.select();
                document.execCommand('copy');
                this.showNotification('URL copied to clipboard!', 'success');
            }
        }
    }

    async saveSettings() {
        const statusSpan = document.getElementById('save-status');
        statusSpan.textContent = 'Saving...';
        statusSpan.className = 'status-message';

        // Helper function to get toggle state from data-checked attribute
        const getToggleState = (toggleId) => {
            const label = document.querySelector(`[data-toggle="${toggleId}"]`);
            return label && label.getAttribute('data-checked') === 'true';
        };

        try {
            const settings = {
                video: {
                    theme: document.getElementById('video-theme').value,
                    overlay: document.getElementById('video-overlay').value,
                    transition: document.getElementById('video-transition').value,
                    quality: document.getElementById('video-quality').value,
                    show_visualizer: getToggleState('video-show-visualizer'),
                    overlay_duration: parseInt(document.getElementById('video-overlay-duration').value),
                    show_background: getToggleState('video-show-background'),
                    enable_audio: getToggleState('video-enable-audio')
                },
                art: {
                    theme: document.getElementById('art-theme').value,
                    animation: getToggleState('art-animation'),
                    anim_duration: parseInt(document.getElementById('art-anim-duration').value),
                    fit: document.getElementById('art-fit').value,
                    show_background: getToggleState('art-show-background')
                },
                track: {
                    theme: document.getElementById('track-theme').value,
                    speed: parseInt(document.getElementById('track-speed').value),
                    direction: document.getElementById('track-direction').value,
                    separator: document.getElementById('track-separator').value,
                    prefix: document.getElementById('track-prefix').value,
                    suffix: document.getElementById('track-suffix').value,
                    show_artist: getToggleState('track-show-artist'),
                    show_album: getToggleState('track-show-album'),
                    show_duration: getToggleState('track-show-duration'),
                    show_background: getToggleState('track-show-background')
                }
            };

            const response = await fetch(`${API_BASE}/settings/feeds`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(settings)
            });

            if (response.ok) {
                statusSpan.textContent = 'Saved!';
                statusSpan.className = 'status-message success';
                this.showNotification('Feed settings saved successfully', 'success');
                
                setTimeout(() => {
                    statusSpan.textContent = '';
                }, 3000);
            } else {
                const data = await response.json();
                statusSpan.textContent = 'Failed to save';
                statusSpan.className = 'status-message error';
                this.showNotification(data.error || 'Failed to save settings', 'error');
            }
        } catch (error) {
            console.error('Error saving settings:', error);
            statusSpan.textContent = 'Failed to save';
            statusSpan.className = 'status-message error';
            this.showNotification('Failed to save settings', 'error');
        }
    }

    showNotification(message, type = 'info') {
        const notification = document.createElement('div');
        notification.className = `notification ${type}`;
        notification.textContent = message;
        document.body.appendChild(notification);

        setTimeout(() => {
            notification.classList.add('fade-out');
            setTimeout(() => notification.remove(), 300);
        }, 3000);
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    new FeedSettingsManager();
});
