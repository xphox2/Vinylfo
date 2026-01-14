const API_BASE = '/api';

class SyncManager {
    constructor() {
        this.isRunning = false;
        this.currentBatch = null;
        this.pollInterval = null;
        this.init();
    }

    init() {
        this.checkConnection();
        this.bindEvents();
    }

    async checkConnection() {
        try {
            const response = await fetch(`${API_BASE}/discogs/status`);
            const status = await response.json();

            if (status.is_connected) {
                this.showSyncReady();
            } else {
                this.showNotConnected();
            }
        } catch (error) {
            console.error('Failed to check connection:', error);
            this.showNotConnected();
        }
    }

    bindEvents() {
        document.getElementById('start-sync').addEventListener('click', () => this.startSync());
        document.getElementById('cancel-sync').addEventListener('click', () => this.cancelSync());
        document.getElementById('skip-batch').addEventListener('click', () => this.skipBatch());
        document.getElementById('confirm-batch').addEventListener('click', () => this.confirmBatch());
        document.getElementById('start-new-sync').addEventListener('click', () => {
            this.hideAll();
            this.showSyncReady();
        });
    }

    showNotConnected() {
        this.hideAll();
        document.getElementById('not-connected').classList.remove('hidden');
    }

    showSyncReady() {
        this.hideAll();
        document.getElementById('sync-ready').classList.remove('hidden');
    }

    showSyncRunning() {
        this.hideAll();
        document.getElementById('sync-running').classList.remove('hidden');
        this.startPolling();
    }

    showSyncComplete(processed) {
        this.hideAll();
        this.stopPolling();
        document.getElementById('sync-complete').classList.remove('hidden');
        document.getElementById('sync-summary').textContent = `Processed ${processed} albums.`;
    }

    showBatchReview(batch) {
        this.hideAll();
        this.currentBatch = batch;
        document.getElementById('batch-review').classList.remove('hidden');

        const container = document.getElementById('batch-albums');
        container.innerHTML = '';

        batch.albums.forEach(album => {
            const item = document.createElement('div');
            item.className = 'batch-album-item';
            item.innerHTML = `
                <div class="album-info">
                    <span class="album-title">${this.escapeHtml(album.title)}</span>
                    <span class="album-artist">${this.escapeHtml(album.artist)}</span>
                    <span class="album-year">${album.year || 'Unknown year'}</span>
                </div>
            `;
            container.appendChild(item);
        });
    }

    hideAll() {
        document.getElementById('not-connected').classList.add('hidden');
        document.getElementById('sync-ready').classList.add('hidden');
        document.getElementById('sync-running').classList.add('hidden');
        document.getElementById('sync-complete').classList.add('hidden');
        document.getElementById('batch-review').classList.add('hidden');
    }

    async startSync() {
        try {
            const response = await fetch(`${API_BASE}/discogs/sync/start`, { method: 'POST' });
            if (response.ok) {
                this.isRunning = true;
                this.showSyncRunning();
            } else {
                const error = await response.json();
                this.showNotification(error.error || 'Failed to start sync', 'error');
            }
        } catch (error) {
            console.error('Failed to start sync:', error);
            this.showNotification('Failed to start sync', 'error');
        }
    }

    async cancelSync() {
        if (!confirm('Are you sure you want to cancel the sync?')) {
            return;
        }

        try {
            await fetch(`${API_BASE}/discogs/sync/cancel`, { method: 'POST' });
            this.isRunning = false;
            this.stopPolling();
            this.showSyncReady();
            this.showNotification('Sync cancelled', 'info');
        } catch (error) {
            console.error('Failed to cancel sync:', error);
            this.showNotification('Failed to cancel sync', 'error');
        }
    }

    async skipBatch() {
        if (!this.currentBatch) return;

        try {
            const response = await fetch(`${API_BASE}/discogs/sync/batch/${this.currentBatch.id}/skip`, {
                method: 'POST'
            });

            if (response.ok) {
                this.showNotification('Batch skipped', 'info');
                this.checkForBatch();
            }
        } catch (error) {
            console.error('Failed to skip batch:', error);
            this.showNotification('Failed to skip batch', 'error');
        }
    }

    async confirmBatch() {
        if (!this.currentBatch) return;

        try {
            const response = await fetch(`${API_BASE}/discogs/sync/batch/${this.currentBatch.id}/confirm`, {
                method: 'POST'
            });

            if (response.ok) {
                this.showNotification('Batch confirmed and synced', 'success');
                this.currentBatch = null;
                this.checkForBatch();
            }
        } catch (error) {
            console.error('Failed to confirm batch:', error);
            this.showNotification('Failed to confirm batch', 'error');
        }
    }

    startPolling() {
        this.pollInterval = setInterval(() => this.pollProgress(), 2000);
        this.pollProgress();
    }

    stopPolling() {
        if (this.pollInterval) {
            clearInterval(this.pollInterval);
            this.pollInterval = null;
        }
    }

    async pollProgress() {
        try {
            const response = await fetch(`${API_BASE}/discogs/sync/progress`);
            const progress = await response.json();

            const progressPercent = progress.total > 0
                ? (progress.processed / progress.total) * 100
                : 0;

            document.getElementById('sync-progress').style.width = `${progressPercent}%`;
            document.getElementById('sync-progress-text').textContent =
                `Processing batch ${progress.current_page} of ${progress.total_pages}... (${progress.processed}/${progress.total} albums)`;

            if (!progress.is_running) {
                this.isRunning = false;
                this.showSyncComplete(progress.processed);
            } else {
                this.checkForBatch();
            }
        } catch (error) {
            console.error('Failed to poll progress:', error);
        }
    }

    async checkForBatch() {
        try {
            const response = await fetch(`${API_BASE}/discogs/sync/progress`);
            const progress = await response.json();

            if (progress.is_running && progress.last_batch && progress.last_batch.albums) {
                this.showBatchReview(progress.last_batch);
            }
        } catch (error) {
            console.error('Failed to check for batch:', error);
        }
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
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
    new SyncManager();
});
