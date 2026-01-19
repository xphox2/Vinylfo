const API_BASE = '/api';

class SettingsManager {
    constructor() {
        this.init();
    }

    async init() {
        await this.loadSettings();
        this.bindEvents();
    }

    async loadSettings() {
        try {
            const discogsRes = await fetch(`${API_BASE}/discogs/status`);
            const discogsStatus = await discogsRes.json();
            this.renderDiscogsStatus(discogsStatus);

            const settingsRes = await fetch(`${API_BASE}/settings`);
            const settings = await settingsRes.json();
            this.renderYouTubeAPIKey(settings.youtube_api_key);
            this.renderLastFMAPIKey(settings.lastfm_api_key);
        } catch (error) {
            console.error('Failed to load settings:', error);
            this.showNotification('Failed to load settings', 'error');
        }
    }

    renderYouTubeAPIKey(apiKey) {
        const input = document.getElementById('youtube-api-key');
        const status = document.getElementById('youtube-key-status');
        if (input && apiKey) {
            input.value = apiKey;
            status.textContent = 'API key is set';
            status.className = 'status-message success';
        } else if (status) {
            status.textContent = '';
            status.className = 'status-message';
        }
    }

    renderLastFMAPIKey(apiKey) {
        const input = document.getElementById('lastfm-api-key');
        const status = document.getElementById('lastfm-key-status');
        if (input && apiKey) {
            input.value = apiKey;
            status.textContent = 'API key is set';
            status.className = 'status-message success';
        } else if (status) {
            status.textContent = '';
            status.className = 'status-message';
        }
    }

    renderDiscogsStatus(status) {
        const statusCard = document.getElementById('discogs-status');
        const indicator = statusCard.querySelector('.status-indicator');
        const text = statusCard.querySelector('.status-text');
        const username = statusCard.querySelector('.status-username');
        const connectBtn = document.getElementById('connect-discogs');
        const disconnectBtn = document.getElementById('disconnect-discogs');

        if (status.is_connected) {
            indicator.classList.remove('disconnected');
            indicator.classList.add('connected');
            text.textContent = 'Connected to Discogs';
            username.textContent = status.username ? `@${status.username}` : '';
            connectBtn.classList.add('hidden');
            disconnectBtn.classList.remove('hidden');
        } else {
            indicator.classList.remove('connected');
            indicator.classList.add('disconnected');
            text.textContent = 'Not Connected';
            username.textContent = '';
            connectBtn.classList.remove('hidden');
            disconnectBtn.classList.add('hidden');
        }
    }

    bindEvents() {
        document.getElementById('connect-discogs').addEventListener('click', () => this.connectDiscogs());
        document.getElementById('disconnect-discogs').addEventListener('click', () => this.disconnectDiscogs());
        document.getElementById('reset-database').addEventListener('click', () => this.resetDatabase());
        document.getElementById('seed-database').addEventListener('click', () => this.seedDatabase());
        document.getElementById('save-youtube-key').addEventListener('click', () => this.saveYouTubeAPIKey());
        document.getElementById('save-lastfm-key').addEventListener('click', () => this.saveLastFMAPIKey());
    }

    async saveYouTubeAPIKey() {
        const input = document.getElementById('youtube-api-key');
        const status = document.getElementById('youtube-key-status');
        const apiKey = input.value.trim();

        if (!apiKey) {
            status.textContent = 'Please enter an API key';
            status.className = 'status-message error';
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/settings`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ youtube_api_key: apiKey })
            });

            if (response.ok) {
                status.textContent = 'API key saved successfully';
                status.className = 'status-message success';
                this.showNotification('YouTube API key saved', 'success');
            } else {
                const data = await response.json();
                status.textContent = data.error || 'Failed to save API key';
                status.className = 'status-message error';
            }
        } catch (error) {
            console.error('Failed to save YouTube API key:', error);
            status.textContent = 'Failed to save API key';
            status.className = 'status-message error';
        }
    }

    async saveLastFMAPIKey() {
        const input = document.getElementById('lastfm-api-key');
        const status = document.getElementById('lastfm-key-status');
        const apiKey = input.value.trim();

        if (!apiKey) {
            status.textContent = 'Please enter an API key';
            status.className = 'status-message error';
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/settings`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ lastfm_api_key: apiKey })
            });

            if (response.ok) {
                status.textContent = 'API key saved successfully';
                status.className = 'status-message success';
                this.showNotification('Last.fm API key saved', 'success');
            } else {
                const data = await response.json();
                status.textContent = data.error || 'Failed to save API key';
                status.className = 'status-message error';
            }
        } catch (error) {
            console.error('Failed to save Last.fm API key:', error);
            status.textContent = 'Failed to save API key';
            status.className = 'status-message error';
        }
    }

    async connectDiscogs() {
        try {
            const response = await fetch(`${API_BASE}/discogs/oauth/url`);
            const data = await response.json();

            if (data.auth_url) {
                window.location.href = data.auth_url;
            } else {
                this.showNotification('Failed to get authorization URL', 'error');
            }
        } catch (error) {
            console.error('Failed to connect Discogs:', error);
            this.showNotification('Failed to connect to Discogs', 'error');
        }
    }

    async disconnectDiscogs() {
        if (!confirm('Are you sure you want to disconnect your Discogs account?')) {
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/discogs/disconnect`, { method: 'POST' });
            if (response.ok) {
                this.showNotification('Disconnected from Discogs', 'success');
                this.loadSettings();
            } else {
                this.showNotification('Failed to disconnect', 'error');
            }
        } catch (error) {
            console.error('Failed to disconnect Discogs:', error);
            this.showNotification('Failed to disconnect from Discogs', 'error');
        }
    }

    async resetDatabase() {
        const confirmed = confirm(
            'Are you sure you want to reset the database?\n\n' +
            'This will delete:\n' +
            '- All albums and tracks\n' +
            '- All playback sessions\n' +
            '- All playlists\n' +
            '- All listening history\n\n' +
            'This will NOT affect:\n' +
            '- Your Discogs OAuth connection\n' +
            '- Your application settings\n\n' +
            'This action cannot be undone!'
        );

        if (!confirmed) {
            return;
        }

        const doubleConfirm = prompt('Type "RESET" to confirm this action:');
        if (doubleConfirm !== 'RESET') {
            this.showNotification('Reset cancelled - confirmation did not match', 'info');
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/database/reset`, { method: 'POST' });
            const data = await response.json();

            if (response.ok) {
                this.showNotification('Database has been reset successfully', 'success');
                setTimeout(() => {
                    window.location.href = '/';
                }, 2000);
            } else {
                this.showNotification(data.error || 'Failed to reset database', 'error');
            }
        } catch (error) {
            console.error('Failed to reset database:', error);
            this.showNotification('Failed to reset database', 'error');
        }
    }

    async seedDatabase() {
        const confirmed = confirm(
            'Seed sample data?\n\n' +
            'This will add 4 sample albums with tracks:\n' +
            '- Abbey Road - The Beatles\n' +
            '- Rumours - Fleetwood Mac\n' +
            '- Dark Side of the Moon - Pink Floyd\n' +
            '- Thriller - Michael Jackson\n\n' +
            'Note: This will only work if your database is empty.'
        );

        if (!confirmed) {
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/database/seed`, { method: 'POST' });
            const data = await response.json();

            if (response.ok) {
                this.showNotification(data.message, 'success');
                setTimeout(() => {
                    window.location.href = '/';
                }, 1500);
            } else {
                this.showNotification(data.error || data.message || 'Failed to seed database', 'error');
            }
        } catch (error) {
            console.error('Failed to seed database:', error);
            this.showNotification('Failed to seed database', 'error');
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

document.addEventListener('DOMContentLoaded', () => {
    new SettingsManager();
});
