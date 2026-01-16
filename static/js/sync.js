const API_BASE = '/api';

class SyncManager {
    constructor() {
        this.isRunning = false;
        this.isPaused = false;
        this.pollInterval = null;
        this.folders = [];
        this.processedCount = 0;
        this.lastProcessedAt = null;
        this.albumsPerMinute = 0;
        this.retryCount = 0;
        this.maxRetries = 3;
        this.init();
    }

    init() {
        this.checkConnection();
        this.bindEvents();
    }

    async checkConnection() {
        try {
            const statusResponse = await fetch(`${API_BASE}/discogs/status`);
            const status = await statusResponse.json();

            if (status.is_connected) {
                const progressResponse = await fetch(`${API_BASE}/discogs/sync/progress`);
                const progress = await progressResponse.json();

                if (progress.is_running) {
                    this.isRunning = true;
                    this.isPaused = progress.is_paused || false;
                    if (this.isPaused) {
                        this.showSyncPaused();
                    } else {
                        this.showSyncRunning();
                    }
                    this.pollProgress();
                } else {
                    this.showSyncReady();
                }
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
        document.getElementById('refresh-status').addEventListener('click', () => this.refreshStatus());
        document.getElementById('pause-sync').addEventListener('click', () => this.togglePause());
        document.getElementById('start-new-sync').addEventListener('click', () => {
            this.hideAll();
            this.showSyncReady();
        });

        document.querySelectorAll('input[name="sync_mode"]').forEach(radio => {
            radio.addEventListener('change', (e) => this.handleSyncModeChange(e.target.value));
        });
    }

    handleSyncModeChange(mode) {
        const folderSelector = document.getElementById('folder-selector');
        if (mode === 'specific') {
            folderSelector.classList.remove('hidden');
            this.loadFolders();
        } else {
            folderSelector.classList.add('hidden');
        }
    }

    async loadFolders() {
        const select = document.getElementById('selected-folder');
        select.innerHTML = '<option value="">Loading folders...</option>';

        try {
            const response = await fetch(`${API_BASE}/discogs/folders`);
            const data = await response.json();

            if (response.ok && data.folders) {
                this.folders = data.folders;
                select.innerHTML = '';

                if (this.folders.length === 0) {
                    select.innerHTML = '<option value="">No folders found</option>';
                    return;
                }

                this.folders.forEach(folder => {
                    const option = document.createElement('option');
                    option.value = folder.id;
                    option.textContent = `${folder.name} (${folder.count} items)`;
                    select.appendChild(option);
                });
            } else {
                select.innerHTML = '<option value="">Failed to load folders</option>';
            }
        } catch (error) {
            console.error('Failed to load folders:', error);
            select.innerHTML = '<option value="">Failed to load folders</option>';
        }
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
        document.getElementById('pause-sync').textContent = 'Pause';
        document.getElementById('pause-sync').classList.remove('btn-success');
        document.getElementById('pause-sync').classList.add('btn-warning');
        this.isRunning = true;
        this.isPaused = false;
        this.startPolling();
    }

    showSyncPaused() {
        this.hideAll();
        document.getElementById('sync-running').classList.remove('hidden');
        document.getElementById('pause-sync').textContent = 'Resume';
        document.getElementById('pause-sync').classList.remove('btn-warning');
        document.getElementById('pause-sync').classList.add('btn-success');
        this.isPaused = true;
    }

    showSyncComplete(processed) {
        this.hideAll();
        this.stopPolling();
        this.isRunning = false;
        this.isPaused = false;
        document.getElementById('sync-complete').classList.remove('hidden');
        document.getElementById('sync-summary').textContent = `${processed} albums synced.`;
    }

    hideAll() {
        document.getElementById('not-connected').classList.add('hidden');
        document.getElementById('sync-ready').classList.add('hidden');
        document.getElementById('sync-running').classList.add('hidden');
        document.getElementById('sync-complete').classList.add('hidden');
    }

    async startSync(forceNew = false) {
        const syncMode = document.querySelector('input[name="sync_mode"]:checked').value;
        let folderId = 0;

        if (syncMode === 'specific') {
            folderId = parseInt(document.getElementById('selected-folder').value, 10);
            if (!folderId || isNaN(folderId)) {
                alert('Please select a folder');
                return;
            }
        }

        this.processedCount = 0;
        this.albumsPerMinute = 0;
        this.lastProcessedAt = null;
        this.retryCount = 0;

        try {
            const response = await fetch(`${API_BASE}/discogs/sync/start`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    sync_mode: syncMode,
                    folder_id: folderId,
                    force_new: forceNew
                })
            });

            if (response.ok) {
                const data = await response.json();
                if (data.has_progress) {
                    if (data.total_albums === 0 && data.processed === 0) {
                        if (confirm('There is sync progress saved, but no albums have been synced yet. Would you like to start a fresh sync?')) {
                            await this.startSync(syncMode, folderId, true);
                        } else {
                            this.isRunning = true;
                            this.isPaused = false;
                            this.showSyncRunning();
                            this.pollProgress();
                        }
                    } else {
                        this.isRunning = true;
                        this.isPaused = false;
                        this.showSyncRunning();
                        this.pollProgress();
                    }
                } else {
                    this.isRunning = true;
                    this.isPaused = false;
                    this.showSyncRunning();
                }
            } else {
                const error = await response.json();
                if (error.error === 'Sync already in progress') {
                    this.isRunning = true;
                    this.isPaused = false;
                    this.showSyncRunning();
                    this.pollProgress();
                } else {
                    alert(error.error || 'Failed to start sync');
                }
            }
        } catch (error) {
            console.error('Failed to start sync:', error);
            alert('Failed to start sync');
        }
    }

    async cancelSync() {
        if (!confirm('Cancel sync?')) return;

        try {
            await fetch(`${API_BASE}/discogs/sync/cancel`, { method: 'POST' });
            this.isRunning = false;
            this.isPaused = false;
            this.stopPolling();
            this.processedCount = 0;
            this.albumsPerMinute = 0;
            document.getElementById('pause-sync').textContent = 'Pause';
            document.getElementById('pause-sync').classList.remove('btn-success');
            document.getElementById('pause-sync').classList.add('btn-warning');
            this.showSyncReady();
        } catch (error) {
            console.error('Failed to cancel sync:', error);
        }
    }

    async togglePause() {
        if (this.isPaused) {
            try {
                const response = await fetch(`${API_BASE}/discogs/sync/resume-pause`, { method: 'POST' });
                if (response.ok) {
                    this.isPaused = false;
                    document.getElementById('pause-sync').textContent = 'Pause';
                    document.getElementById('pause-sync').classList.remove('btn-success');
                    document.getElementById('pause-sync').classList.add('btn-warning');
                } else {
                    const error = await response.json();
                    alert(error.error || 'Failed to resume sync');
                }
            } catch (error) {
                console.error('Failed to resume sync:', error);
            }
        } else {
            try {
                const response = await fetch(`${API_BASE}/discogs/sync/pause`, { method: 'POST' });
                if (response.ok) {
                    this.isPaused = true;
                    document.getElementById('pause-sync').textContent = 'Resume';
                    document.getElementById('pause-sync').classList.remove('btn-warning');
                    document.getElementById('pause-sync').classList.add('btn-success');
                } else {
                    const error = await response.json();
                    alert(error.error || 'Failed to pause sync');
                }
            } catch (error) {
                console.error('Failed to pause sync:', error);
            }
        }
    }

    refreshStatus() {
        this.pollProgress();
    }

    startPolling() {
        this.pollInterval = setInterval(() => this.pollProgress(), 500);
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
            if (!response.ok) {
                throw new Error('Failed to fetch progress');
            }
            this.retryCount = 0;

            const progress = await response.json();

            const processed = progress.processed || 0;
            const total = progress.total || 0;

            let progressPercent = 0;
            let progressText = '';

            if (total > 0) {
                progressPercent = Math.min((processed / total) * 100, 100);
                progressText = `${processed} / ${total} albums (${Math.round(progressPercent)}%)`;
            } else {
                progressText = `${processed} albums synced`;
            }

            document.getElementById('sync-progress').style.width = `${progressPercent}%`;
            document.getElementById('sync-progress-text').textContent = progressText;

            if (progress.is_paused && !this.isPaused) {
                this.isPaused = true;
                document.getElementById('pause-sync').textContent = 'Resume';
                document.getElementById('pause-sync').classList.remove('btn-warning');
                document.getElementById('pause-sync').classList.add('btn-success');
            } else if (!progress.is_paused && this.isPaused && progress.is_running) {
                this.isPaused = false;
                document.getElementById('pause-sync').textContent = 'Pause';
                document.getElementById('pause-sync').classList.remove('btn-success');
                document.getElementById('pause-sync').classList.add('btn-warning');
            }

            if (!progress.is_running && !this.isPaused) {
                this.isRunning = false;
                this.isPaused = false;
                if (processed > 0 && processed >= total) {
                    this.showSyncComplete(processed);
                } else if (processed === 0 && total === 0) {
                    document.getElementById('sync-progress-text').textContent = 'No albums to sync';
                } else if (processed < total) {
                    document.getElementById('sync-progress-text').textContent = `Sync stopped at ${processed}/${total} albums`;
                }
            }

            if (progress.is_running) {
                this.isRunning = true;
                document.getElementById('sync-running').classList.remove('hidden');

                if (progress.folder_name && progress.total_folders > 1) {
                    const folderInfo = document.getElementById('sync-folder-info');
                    if (folderInfo) {
                        folderInfo.textContent = `Syncing folder ${progress.folder_index + 1} of ${progress.total_folders}: "${progress.folder_name}"`;
                        folderInfo.style.display = 'block';
                    }
                }
            }

            // Rate limit display disabled
            // this.updateRateLimitDisplay(progress.api_remaining, progress.anon_remaining);
            this.updateEstimatedTime(processed, total);

        } catch (error) {
            console.error('Failed to poll progress:', error);
            this.retryCount++;
            if (this.retryCount <= this.maxRetries) {
                console.log(`Retrying progress fetch (${this.retryCount}/${this.maxRetries})...`);
                setTimeout(() => this.pollProgress(), 1000);
            } else {
                document.getElementById('sync-progress-text').textContent = 'Connection error - please refresh';
                this.retryCount = 0;
            }
        }
    }

    updateEstimatedTime(processed, total) {
        if (processed > this.processedCount) {
            const now = Date.now();
            if (this.lastProcessedAt) {
                const timeDiff = (now - this.lastProcessedAt) / 1000 / 60;
                const albumDiff = processed - this.processedCount;
                const rate = albumDiff / timeDiff;
                this.albumsPerMinute = Math.round(rate * 10) / 10;
            }
            this.lastProcessedAt = now;
            this.processedCount = processed;
        }

        if (this.albumsPerMinute > 0 && total > processed) {
            const remaining = total - processed;
            const minutesLeft = Math.ceil(remaining / this.albumsPerMinute);
            let timeText = '';
            if (minutesLeft < 60) {
                timeText = `${minutesLeft} min remaining`;
            } else {
                const hours = Math.floor(minutesLeft / 60);
                const mins = minutesLeft % 60;
                timeText = `${hours}h ${mins}m remaining`;
            }
            let timeEl = document.getElementById('sync-time-remaining');
            if (!timeEl) {
                timeEl = document.createElement('p');
                timeEl.id = 'sync-time-remaining';
                timeEl.className = 'time-remaining';
                const rateLimitEl = document.getElementById('sync-rate-limit');
                if (rateLimitEl) {
                    rateLimitEl.parentNode.insertBefore(timeEl, rateLimitEl.nextSibling);
                } else {
                    const progressText = document.getElementById('sync-progress-text');
                    progressText.parentNode.insertBefore(timeEl, progressText.nextSibling);
                }
            }
            timeEl.textContent = timeText;
            timeEl.style.fontSize = '0.8rem';
            timeEl.style.color = '#28a745';
            timeEl.style.marginTop = '0.25rem';
        }
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new SyncManager();
});
