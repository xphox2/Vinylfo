const API_BASE = '/api';

import { normalizeArtistName } from './modules/utils.js';

class SyncManager {
    constructor() {
        this.isRunning = false;
        this.isPaused = false;
        this.pollInterval = null;
        this.pollingActive = false;
        this.folders = [];
        this.processedCount = 0;
        this.lastProcessedAt = null;
        this.albumsPerMinute = 0;
        this.retryCount = 0;
        this.maxRetries = 5;
        this.basePollInterval = 5000;
        this.stalledPollInterval = 10000;
        this.stalePollTimeout = 45000;
        this.pollRequestTimeout = 12000;
        this.stallDetectionCount = 0;
        this.wasRateLimited = false;
        this.pollInProgress = false;
        this.lastPollStart = null;
        this.rateLimitSecondsLeft = 0;
        this.countdownInterval = null;
        this.init();
    }

    init() {
        this.checkConnection();
        this.bindEvents();
    }

    async checkConnection() {
        try {
            console.log('checkConnection: fetching /api/discogs/status...');
            const statusResponse = await fetch(`${API_BASE}/discogs/status`);
            console.log('checkConnection: statusResponse.ok:', statusResponse.ok, 'status:', statusResponse.status);
            
            const status = await statusResponse.json();
            console.log('checkConnection: status.is_connected:', status.is_connected);
            
            if (status.is_connected) {
                console.log('checkConnection: connected, fetching progress...');
                try {
                    const progressResponse = await fetch(`${API_BASE}/discogs/sync/progress`);
                    console.log('checkConnection: progressResponse.ok:', progressResponse.ok);
                    
                    const progress = await progressResponse.json();
                    console.log('checkConnection: progress.is_running:', progress.is_running, 'is_paused:', progress.is_paused);

                    if (progress.is_running) {
                        this.isRunning = true;
                        this.isPaused = progress.is_paused || false;
                        if (this.isPaused) {
                            // If rate-limited, start polling to detect when it clears
                            const shouldPoll = progress.is_rate_limited || false;
                            if (progress.is_rate_limited) {
                                this.wasRateLimited = true;
                                this.rateLimitSecondsLeft = progress.rate_limit_seconds_left || 60;
                            }
                            this.showSyncPaused(shouldPoll);
                            // If rate-limited, update the UI to show countdown
                            if (progress.is_rate_limited) {
                                this.startRateLimitCountdown(progress.processed || 0, progress.total || 0);
                                document.getElementById('pause-sync').textContent = 'Cancel';
                                document.getElementById('pause-sync').classList.remove('btn-success');
                                document.getElementById('pause-sync').classList.add('btn-danger');
                            }
                        } else {
                            this.showSyncRunning();
                        }
                    } else if (progress.is_paused && progress.is_rate_limited) {
                        // Backend paused due to rate limit but is_running is false - sync will auto-resume
                        console.log('checkConnection: rate-limited pause detected, starting polling');
                        this.isRunning = true;
                        this.isPaused = true;
                        this.wasRateLimited = true;
                        this.rateLimitSecondsLeft = progress.rate_limit_seconds_left || 60;
                        this.showSyncPaused(true); // Start polling to detect resume
                        this.startRateLimitCountdown(progress.processed || 0, progress.total || 0);
                        document.getElementById('pause-sync').textContent = 'Cancel';
                        document.getElementById('pause-sync').classList.remove('btn-success');
                        document.getElementById('pause-sync').classList.add('btn-danger');
                    } else if (progress.has_saved_progress) {
                        console.log('checkConnection: has saved progress, showing paused state');
                        this.isRunning = true;
                        this.isPaused = true;
                        this.showSyncPaused();
                    } else {
                        this.showSyncReady();
                    }
                } catch (progressError) {
                    console.error('checkConnection: progress fetch failed:', progressError.message);
                    console.log('checkConnection: progress fetch failed but connected=true, checking saved progress...');
                    
                    try {
                        const progressResponse = await fetch(`${API_BASE}/discogs/sync/progress`);
                        if (progressResponse.ok) {
                            const progress = await progressResponse.json();
                            if (progress.has_saved_progress || progress.is_running) {
                                console.log('checkConnection: found saved progress, showing paused state');
                                this.isRunning = true;
                                this.isPaused = true;
                                this.showSyncPaused();
                                return;
                            }
                        }
                    } catch (fallbackError) {
                        console.error('checkConnection: fallback progress fetch failed:', fallbackError.message);
                    }
                    
                    console.log('checkConnection: showing sync ready (connected but no active sync)');
                    this.showSyncReady();
                }
            } else {
                console.log('checkConnection: NOT connected, checking for saved progress...');
                const progressResponse = await fetch(`${API_BASE}/discogs/sync/progress`);
                
                if (progressResponse.ok) {
                    const progress = await progressResponse.json();
                    console.log('checkConnection: has_saved_progress:', progress.has_saved_progress);
                    
                    if (progress.has_saved_progress || progress.is_running) {
                        this.showSyncPaused();
                        this.isRunning = true;
                        this.isPaused = true;
                    } else {
                        this.showNotConnected();
                    }
                } else {
                    this.showNotConnected();
                }
            }
        } catch (error) {
            console.error('checkConnection: error:', error.message, error.stack);
            console.log('checkConnection: trying to fetch progress as fallback...');
            
            try {
                const progressResponse = await fetch(`${API_BASE}/discogs/sync/progress`);
                if (progressResponse.ok) {
                    const progress = await progressResponse.json();
                    console.log('checkConnection: fallback progress:', progress);
                    
                    if (progress.has_saved_progress || progress.is_running) {
                        this.showSyncPaused();
                        this.isRunning = true;
                        this.isPaused = true;
                        return;
                    }
                }
            } catch (e) {
                console.error('checkConnection: fallback error:', e.message);
            }
            
            console.log('checkConnection: showing not connected');
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
        document.getElementById('refresh-tracks').addEventListener('click', () => this.refreshTracks());
        document.getElementById('cleanup-albums').addEventListener('click', () => this.showCleanupModal());
        document.getElementById('close-cleanup-modal').addEventListener('click', () => this.hideCleanupModal());
        document.getElementById('cancel-cleanup').addEventListener('click', () => this.hideCleanupModal());
        document.getElementById('delete-selected').addEventListener('click', () => this.deleteSelectedAlbums());

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

    cleanArtistName(artistName) {
        if (!artistName) return 'Unknown Artist';
        return normalizeArtistName(artistName) || 'Unknown Artist';
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

    showSyncPaused(startPolling = false) {
        this.hideAll();
        document.getElementById('sync-running').classList.remove('hidden');
        document.getElementById('pause-sync').textContent = 'Resume';
        document.getElementById('pause-sync').classList.remove('btn-warning');
        document.getElementById('pause-sync').classList.add('btn-success');
        this.isPaused = true;
        this.isRunning = true;
        // Start polling if requested (e.g., for rate-limited pauses that will auto-resume)
        if (startPolling && !this.pollingActive) {
            console.log('showSyncPaused: starting polling for auto-resume detection');
            this.startPolling();
        }
    }

    showSyncComplete(processed) {
        this.hideAll();
        this.stopPolling();
        this.isRunning = false;
        this.isPaused = false;
        document.getElementById('sync-complete').classList.remove('hidden');
        document.getElementById('sync-summary').textContent = `${processed} albums synced.`;
        setTimeout(() => location.reload(), 5000);
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
        this.stallDetectionCount = 0;
        this.currentPollInterval = this.basePollInterval;

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
                                await this.startSync(true);
                            } else {
                                this.isRunning = true;
                                this.isPaused = false;
                                this.showSyncRunning();
                            }
                        } else {
                            this.isRunning = true;
                            this.isPaused = false;
                            this.showSyncRunning();
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
                } else {
                    alert(error.error || 'Failed to start sync');
                }
            }
        } catch (error) {
            console.error('Failed to start sync:', error);
            alert('Failed to start sync: ' + error.message);
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
            this.stallDetectionCount = 0;
            this.retryCount = 0;
            document.getElementById('pause-sync').textContent = 'Pause';
            document.getElementById('pause-sync').classList.remove('btn-success');
            document.getElementById('pause-sync').classList.add('btn-warning');
            this.showSyncReady();
        } catch (error) {
            console.error('Failed to cancel sync:', error);
        }
    }

    async togglePause() {
        console.log('togglePause: START, this.isPaused:', this.isPaused);
        
        if (this.isPaused) {
            console.log('togglePause: Calling resume API...');
            try {
                console.log('togglePause: Fetching /api/discogs/sync/resume-pause...');
                const response = await fetch(`${API_BASE}/discogs/sync/resume-pause`, { method: 'POST' });
                console.log('togglePause: Resume API response status:', response.status);
                
                if (response.ok) {
                    const data = await response.json();
                    console.log('togglePause: Resume succeeded:', data);
                    this.isPaused = false;
                    this.stallDetectionCount = 0;
                    this.currentPollInterval = this.basePollInterval;
                    document.getElementById('pause-sync').textContent = 'Pause';
                    document.getElementById('pause-sync').classList.remove('btn-success');
                    document.getElementById('pause-sync').classList.add('btn-warning');
                    this.stopPolling();
                    this.isRunning = true;
                    this.startPolling();
                } else {
                    const error = await response.json();
                    console.error('togglePause: Resume failed:', error);
                    alert(error.error || 'Failed to resume sync');
                }
            } catch (error) {
                console.error('togglePause: Error calling resume API:', error);
            }
        } else {
            console.log('togglePause: Calling pause API...');
            try {
                console.log('togglePause: Fetching /api/discogs/sync/pause...');
                const response = await fetch(`${API_BASE}/discogs/sync/pause`, { method: 'POST' });
                console.log('togglePause: Pause API response status:', response.status);
                
                if (response.ok) {
                    const data = await response.json();
                    console.log('togglePause: Pause succeeded:', data);
                    this.isPaused = true;
                    document.getElementById('pause-sync').textContent = 'Resume';
                    document.getElementById('pause-sync').classList.remove('btn-warning');
                    document.getElementById('pause-sync').classList.add('btn-success');
                    this.stopPolling();
                } else {
                    const error = await response.json();
                    console.error('togglePause: Pause failed:', error);
                    alert(error.error || 'Failed to pause sync');
                }
            } catch (error) {
                console.error('togglePause: Error calling pause API:', error);
            }
        }
        console.log('togglePause: END, this.isPaused:', this.isPaused);
    }

    async resumeSync() {
        console.log('resumeSync: START');
        try {
            console.log('resumeSync: Fetching /api/discogs/sync/resume-pause...');
            const response = await fetch(`${API_BASE}/discogs/sync/resume-pause`, { method: 'POST' });
            console.log('resumeSync: Response status:', response.status);

            if (response.ok) {
                const data = await response.json();
                console.log('resumeSync: Success:', data);
                this.isPaused = false;
                this.wasRateLimited = false;
                document.getElementById('pause-sync').textContent = 'Pause';
                document.getElementById('pause-sync').classList.remove('btn-success');
                document.getElementById('pause-sync').classList.add('btn-warning');
                this.stopPolling();
                this.isRunning = true;
                this.startPolling();
            } else {
                const error = await response.json();
                console.error('resumeSync: Failed:', error);
            }
        } catch (error) {
            console.error('resumeSync: Error:', error);
        }
        console.log('resumeSync: END');
    }

    refreshStatus() {
        this.pollProgress();
    }

    startPolling() {
        if (this.pollingActive) {
            console.log('startPolling: polling already active, not starting again');
            return;
        }
        this.pollingActive = true;
        this.currentPollInterval = this.basePollInterval;
        this.stallDetectionCount = 0;
        this.pollInProgress = false;
        this.scheduleNextPoll(0);
        console.log('startPolling: scheduled polling at', this.currentPollInterval, 'ms');
    }

    stopPolling() {
        this.pollingActive = false;
        if (this.pollInterval) {
            clearTimeout(this.pollInterval);
            this.pollInterval = null;
        }
        this.stallDetectionCount = 0;
        this.pollInProgress = false;
        this.stopRateLimitCountdown();
        console.log('stopPolling: stopped polling');
    }

    scheduleNextPoll(delay) {
        if (!this.pollingActive) {
            return;
        }
        if (this.pollInterval) {
            clearTimeout(this.pollInterval);
        }
        this.pollInterval = setTimeout(() => {
            this.pollInterval = null;
            this.pollProgress();
        }, delay);
    }

    startRateLimitCountdown(processed, total) {
        this.stopRateLimitCountdown();
        // Store the values for the countdown display
        this.countdownProcessed = processed;
        this.countdownTotal = total;
        this.updateRateLimitDisplay(processed, total);

        this.countdownInterval = setInterval(() => {
            this.rateLimitSecondsLeft--;
            if (this.rateLimitSecondsLeft <= 0) {
                this.stopRateLimitCountdown();
                document.getElementById('sync-progress-text').textContent = `Rate limit clearing... (${this.countdownProcessed}/${this.countdownTotal})`;
            } else {
                this.updateRateLimitDisplay(this.countdownProcessed, this.countdownTotal);
            }
        }, 1000);
    }

    stopRateLimitCountdown() {
        if (this.countdownInterval) {
            clearInterval(this.countdownInterval);
            this.countdownInterval = null;
        }
    }

    updateRateLimitDisplay(processed, total) {
        const seconds = this.rateLimitSecondsLeft;
        const minutes = Math.floor(seconds / 60);
        const secs = seconds % 60;
        const timeStr = minutes > 0
            ? `${minutes}m ${secs}s`
            : `${secs}s`;
        document.getElementById('sync-progress-text').textContent =
            `API rate limit - resuming in ${timeStr} (${processed}/${total} albums)`;
    }

    async pollProgress() {
        const now = Date.now();
        if (this.pollInProgress && this.lastPollStart && (now - this.lastPollStart > this.stalePollTimeout)) {
            console.log('pollProgress: stale poll detected, resetting flag');
            this.pollInProgress = false;
        }
        if (this.pollInProgress) {
            console.log('pollProgress: skipping, request already in progress');
            // Schedule next poll anyway to prevent getting stuck
            if (this.pollingActive) {
                this.scheduleNextPoll(this.currentPollInterval);
            }
            return;
        }
        this.pollInProgress = true;
        this.lastPollStart = now;
        console.log('pollProgress called, isRunning:', this.isRunning, 'isPaused:', this.isPaused);
        
        let response;
        let progress;
        try {
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), this.pollRequestTimeout);
            try {
                response = await fetch(`${API_BASE}/discogs/sync/progress`, { signal: controller.signal });
            } finally {
                clearTimeout(timeoutId);
            }
            if (!response.ok) {
                throw new Error('Failed to fetch progress: HTTP ' + response.status);
            }
            progress = await response.json();
            console.log('pollProgress received:', { is_running: progress.is_running, is_paused: progress.is_paused, processed: progress.processed, total: progress.total, is_stalled: progress.is_stalled, is_rate_limited: progress.is_rate_limited });
            
            // Reset retry count on successful poll
            this.retryCount = 0;

            if (this.isRunning && !progress.is_running && !this.isPaused && !progress.is_rate_limited) {
                // Sync stopped and NOT rate-limited - this is a real stop
                const processed = progress.processed || 0;
                const total = progress.total || 0;

                if (processed === 0 && total === 0) {
                    document.getElementById('sync-progress-text').textContent = 'No albums found in collection';
                    this.isRunning = false;
                    this.stopPolling();
                    this.showSyncReady();
                    this.pollInProgress = false;
                    return;
                }

                if (processed < total) {
                    document.getElementById('sync-progress-text').textContent = `Sync stopped at ${processed}/${total} albums`;
                    this.isRunning = false;
                    this.stopPolling();
                    this.pollInProgress = false;
                    return;
                }
            }

            let processed = progress.processed || progress.saved_processed || 0;
            let total = progress.total || progress.saved_total_albums || 0;

            let progressPercent = 0;
            let progressText = '';

            if (total > 0) {
                progressPercent = Math.min((processed / total) * 100, 100);
                const displayProcessed = Math.min(processed, total);
                progressText = `${displayProcessed} / ${total} albums (${Math.round(progressPercent)}%)`;
            } else {
                progressText = `${processed} albums synced`;
            }

            document.getElementById('sync-progress').style.width = `${progressPercent}%`;

            if (progress.is_stalled && this.isRunning) {
                this.stallDetectionCount++;
                if (this.stallDetectionCount >= 6) {
                    console.log('pollProgress: sync is stalled, increasing poll interval to', this.stalledPollInterval, 'ms');
                    this.currentPollInterval = this.stalledPollInterval;
                }
                console.log('pollProgress: sync appears stalled, continuing to poll (count:', this.stallDetectionCount, ')');
                document.getElementById('sync-progress-text').textContent = `Waiting for API... (${processed}/${total})`;
            } else {
                this.stallDetectionCount = 0;
                if (this.currentPollInterval !== this.basePollInterval && !progress.is_stalled) {
                    console.log('pollProgress: sync resumed, decreasing poll interval to', this.basePollInterval, 'ms');
                    this.currentPollInterval = this.basePollInterval;
                }
                document.getElementById('sync-progress-text').textContent = progressText;
            }

            if (progress.is_paused && !this.isPaused) {
                // Backend just paused (either user-initiated or rate-limited)
                this.isPaused = true;
                this.wasRateLimited = progress.is_rate_limited || false;
                if (progress.is_rate_limited) {
                    this.rateLimitSecondsLeft = progress.rate_limit_seconds_left || 60;
                    this.startRateLimitCountdown(processed, total);
                    document.getElementById('pause-sync').textContent = 'Cancel';
                    document.getElementById('pause-sync').classList.remove('btn-warning');
                    document.getElementById('pause-sync').classList.add('btn-danger');
                    // CRITICAL: Keep polling active during rate limit to detect when it clears
                    if (!this.pollingActive) {
                        console.log('pollProgress: rate limited - ensuring polling stays active');
                        this.pollingActive = true;
                    }
                } else {
                    document.getElementById('pause-sync').textContent = 'Resume';
                    document.getElementById('pause-sync').classList.remove('btn-warning');
                    document.getElementById('pause-sync').classList.add('btn-success');
                    document.getElementById('sync-progress-text').textContent = `Paused at ${processed}/${total} albums`;
                }
            } else if (progress.is_paused && this.isPaused && progress.is_rate_limited) {
                // Still rate limited - update countdown with fresh values from server
                this.wasRateLimited = true;
                const serverSecondsLeft = progress.rate_limit_seconds_left || 0;
                // Only restart countdown if server has a significantly different value
                // This prevents countdown jitter from server/client time differences
                if (Math.abs(serverSecondsLeft - this.rateLimitSecondsLeft) > 3) {
                    this.rateLimitSecondsLeft = serverSecondsLeft;
                }
                this.updateRateLimitDisplay(processed, total);
                // Ensure polling stays active
                if (!this.pollingActive) {
                    console.log('pollProgress: rate limited - restarting polling');
                    this.pollingActive = true;
                }
            } else if (!progress.is_paused && this.isPaused && progress.is_running) {
                // Backend resumed (either user-initiated or rate limit cleared)
                this.isPaused = false;
                this.wasRateLimited = false;
                this.stopRateLimitCountdown();
                document.getElementById('pause-sync').textContent = 'Pause';
                document.getElementById('pause-sync').classList.remove('btn-success', 'btn-danger');
                document.getElementById('pause-sync').classList.add('btn-warning');
            }

            // Check for rate limit cleared state - multiple conditions for robustness
            if (this.wasRateLimited && this.isPaused) {
                // Rate limit cleared if: not rate limited anymore, OR seconds left is 0 or negative
                const rateLimitCleared = !progress.is_rate_limited || 
                    (progress.rate_limit_seconds_left !== undefined && progress.rate_limit_seconds_left <= 0);
                
                if (rateLimitCleared) {
                    console.log('pollProgress: rate limit cleared detected, is_running:', progress.is_running, 'is_paused:', progress.is_paused);
                    this.stopRateLimitCountdown();
                    
                    if (progress.is_running) {
                        // Backend already resumed automatically, just update UI
                        console.log('pollProgress: backend already resumed, updating UI');
                        this.isPaused = false;
                        this.wasRateLimited = false;
                        document.getElementById('pause-sync').textContent = 'Pause';
                        document.getElementById('pause-sync').classList.remove('btn-success', 'btn-danger');
                        document.getElementById('pause-sync').classList.add('btn-warning');
                    } else if (progress.is_paused) {
                        // Backend is still paused but rate limit cleared - call resume API
                        console.log('pollProgress: rate limit cleared but still paused, calling resumeSync');
                        this.wasRateLimited = false;
                        this.resumeSync();
                    }
                }
            }
            
            // Also handle case where backend resumed but we missed the transition
            if (progress.is_running && this.isPaused && !progress.is_paused) {
                console.log('pollProgress: detected running state while frontend thought paused, updating UI');
                this.isPaused = false;
                this.wasRateLimited = false;
                this.stopRateLimitCountdown();
                document.getElementById('pause-sync').textContent = 'Pause';
                document.getElementById('pause-sync').classList.remove('btn-success', 'btn-danger');
                document.getElementById('pause-sync').classList.add('btn-warning');
            }

            if (!progress.is_running && !this.isPaused && !progress.is_rate_limited) {
                // Sync truly stopped (not just rate-limited pause)
                this.isRunning = false;
                this.isPaused = false;
                this.stopPolling();
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

                if (!this.pollingActive) {
                    this.startPolling();
                }

                if (progress.folder_name && progress.total_folders > 1) {
                    const folderInfo = document.getElementById('sync-folder-info');
                    if (folderInfo) {
                        folderInfo.textContent = `Syncing folder ${progress.folder_index + 1} of ${progress.total_folders}: "${progress.folder_name}"`;
                        folderInfo.style.display = 'block';
                    }
                }
            }

            this.updateEstimatedTime(processed, total);
            this.pollInProgress = false;
            this.scheduleNextPoll(this.currentPollInterval);

        } catch (error) {
            if (error.name === 'AbortError') {
                console.warn('Failed to poll progress: request timed out');
            } else {
                console.error('Failed to poll progress:', error);
            }
            // Always reset pollInProgress to prevent getting stuck
            this.pollInProgress = false;
            this.retryCount++;
            
            if (this.retryCount <= this.maxRetries) {
                if (!this.pollingActive) {
                    return;
                }
                const retryDelay = Math.min(1000 * Math.pow(2, this.retryCount - 1), 10000);
                console.log(`Retrying progress fetch (${this.retryCount}/${this.maxRetries}) in ${retryDelay}ms...`);
                this.scheduleNextPoll(retryDelay);
            } else {
                document.getElementById('sync-progress-text').textContent = 'Connection error - sync may have stopped. Click Refresh Status to check.';
                this.retryCount = 0;
                // Don't stop polling completely - just slow it down significantly
                // This allows recovery if the server comes back
                this.currentPollInterval = 15000; // 15 second interval
                this.scheduleNextPoll(this.currentPollInterval);
            }
            return;
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

    showCleanupModal() {
        const modal = document.getElementById('cleanup-modal');
        modal.classList.remove('hidden');
        
        document.getElementById('cleanup-loading').classList.remove('hidden');
        document.getElementById('cleanup-results').classList.add('hidden');
        document.getElementById('cleanup-empty').classList.add('hidden');
        document.getElementById('delete-selected').classList.add('hidden');
        
        this.unlinkedAlbums = [];
        this.findUnlinkedAlbums();
    }

    hideCleanupModal() {
        document.getElementById('cleanup-modal').classList.add('hidden');
    }

    async findUnlinkedAlbums() {
        try {
            const response = await fetch(`${API_BASE}/discogs/unlinked-albums`);
            const result = await response.json();

            document.getElementById('cleanup-loading').classList.add('hidden');

            if (!response.ok) {
                alert(`Failed to scan: ${result.error || 'Unknown error'}`);
                this.hideCleanupModal();
                return;
            }

            this.unlinkedAlbums = result.unlinked_albums || [];

            if (this.unlinkedAlbums.length === 0) {
                document.getElementById('cleanup-empty').classList.remove('hidden');
            } else {
                document.getElementById('cleanup-results').classList.remove('hidden');
                document.getElementById('delete-selected').classList.remove('hidden');
                document.getElementById('cleanup-summary').textContent = 
                    `Found ${this.unlinkedAlbums.length} album(s) not in your Discogs collection (checked ${result.total_checked} local albums against ${result.discogs_total} Discogs releases):`;
                
                this.renderUnlinkedAlbums();
            }
        } catch (error) {
            console.error('Failed to find unlinked albums:', error);
            alert('Failed to scan for unlinked albums. Please try again.');
            this.hideCleanupModal();
        }
    }

    renderUnlinkedAlbums() {
        const list = document.getElementById('unlinked-albums-list');
        list.innerHTML = '';

        this.unlinkedAlbums.forEach(album => {
            const item = document.createElement('div');
            item.className = 'unlinked-album-item';
            item.innerHTML = `
                <label class="album-checkbox">
                    <input type="checkbox" value="${album.id}" checked>
                    <span class="album-info">
                        <span class="album-title">${album.title}</span>
                        <span class="album-artist">${this.cleanArtistName(album.artist)}</span>
                        ${album.year ? `<span class="album-year">(${album.year})</span>` : ''}
                    </span>
                </label>
            `;
            list.appendChild(item);
        });
    }

    async deleteSelectedAlbums() {
        const checkboxes = document.querySelectorAll('#unlinked-albums-list input[type="checkbox"]:checked');
        const albumIds = Array.from(checkboxes).map(cb => parseInt(cb.value));

        if (albumIds.length === 0) {
            alert('No albums selected for deletion.');
            return;
        }

        if (!confirm(`Are you sure you want to delete ${albumIds.length} album(s) and all their tracks? This cannot be undone.`)) {
            return;
        }

        const button = document.getElementById('delete-selected');
        button.disabled = true;
        button.textContent = 'Deleting...';

        try {
            const response = await fetch(`${API_BASE}/discogs/unlinked-albums/delete`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ album_ids: albumIds })
            });

            const result = await response.json();

            if (response.ok) {
                alert(`Deleted ${result.deleted} album(s)${result.failed > 0 ? `, ${result.failed} failed` : ''}.`);
                this.hideCleanupModal();
            } else {
                alert(`Deletion failed: ${result.error || 'Unknown error'}`);
            }
        } catch (error) {
            console.error('Failed to delete albums:', error);
            alert('Failed to delete albums. Please try again.');
        } finally {
            button.disabled = false;
            button.textContent = 'Delete Selected';
        }
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new SyncManager();
});
