const API_BASE = '/api';

class SettingsManager {
    constructor() {
        this.settings = null;
        this.init();
    }

    async init() {
        await this.loadSettings();
        this.bindEvents();
    }

    async loadSettings() {
        try {
            const [settingsRes, discogsRes] = await Promise.all([
                fetch(`${API_BASE}/settings`),
                fetch(`${API_BASE}/discogs/status`)
            ]);

            this.settings = await settingsRes.json();
            const discogsStatus = await discogsRes.json();

            this.renderSettings(this.settings);
            this.renderDiscogsStatus(discogsStatus);
        } catch (error) {
            console.error('Failed to load settings:', error);
            this.showNotification('Failed to load settings', 'error');
        }
    }

    renderSettings(settings) {
        document.getElementById('sync-confirm-batches').checked = settings.sync_confirm_batches;
        document.getElementById('sync-batch-size').value = settings.sync_batch_size;
        document.getElementById('auto-apply-safe').checked = settings.auto_apply_safe;
        document.getElementById('auto-sync-new').checked = settings.auto_sync_new;
        document.getElementById('items-per-page').value = settings.items_per_page || 25;
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

        document.getElementById('sync-settings-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.saveSyncSettings();
        });

        document.getElementById('app-settings-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.saveAppSettings();
        });
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

    async saveSyncSettings() {
        const data = {
            sync_confirm_batches: document.getElementById('sync-confirm-batches').checked,
            sync_batch_size: parseInt(document.getElementById('sync-batch-size').value),
            auto_apply_safe_updates: document.getElementById('auto-apply-safe').checked,
            auto_sync_new_albums: document.getElementById('auto-sync-new').checked
        };

        try {
            const response = await fetch(`${API_BASE}/settings`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (response.ok) {
                this.showNotification('Sync settings saved', 'success');
            } else {
                const error = await response.json();
                this.showNotification(error.error || 'Failed to save settings', 'error');
            }
        } catch (error) {
            console.error('Failed to save sync settings:', error);
            this.showNotification('Failed to save settings', 'error');
        }
    }

    async saveAppSettings() {
        const data = {
            items_per_page: parseInt(document.getElementById('items-per-page').value)
        };

        try {
            const response = await fetch(`${API_BASE}/settings`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (response.ok) {
                this.showNotification('App settings saved', 'success');
            } else {
                const error = await response.json();
                this.showNotification(error.error || 'Failed to save settings', 'error');
            }
        } catch (error) {
            console.error('Failed to save app settings:', error);
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

document.addEventListener('DOMContentLoaded', () => {
    new SettingsManager();
});
