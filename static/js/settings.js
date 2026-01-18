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
        } catch (error) {
            console.error('Failed to load settings:', error);
            this.showNotification('Failed to load settings', 'error');
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
