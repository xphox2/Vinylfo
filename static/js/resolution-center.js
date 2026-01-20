import { api, durationAPI } from './modules/api.js';
import { escapeHtml, formatDuration, showNotification } from './modules/utils.js';

const API_BASE = '/api';

class ResolutionCenterManager {
    constructor() {
        this.currentPage = 1;
        this.resolvedPage = 1;
        this.unprocessedPage = 1;
        this.pageSize = 20;
        this.totalPages = 1;
        this.resolvedTotalPages = 1;
        this.unprocessedTotalPages = 1;
        this.reviewItems = [];
        this.resolvedItems = [];
        this.unprocessedItems = [];
        this.selectedSources = {};
        this.wasRunning = false;
        this.currentTab = 'review';
        this.init();
    }

    async init() {
        this.bindEvents();
        await this.loadStats();
        await this.loadReviewQueue();
        this.startProgressPolling();
    }

    bindEvents() {
        document.getElementById('prev-page').addEventListener('click', () => this.changePage(-1));
        document.getElementById('next-page').addEventListener('click', () => this.changePage(1));
        document.getElementById('resolved-prev-page').addEventListener('click', () => this.changeResolvedPage(-1));
        document.getElementById('resolved-next-page').addEventListener('click', () => this.changeResolvedPage(1));
        document.getElementById('unprocessed-prev-page').addEventListener('click', () => this.changeUnprocessedPage(-1));
        document.getElementById('unprocessed-next-page').addEventListener('click', () => this.changeUnprocessedPage(1));

        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.addEventListener('click', (e) => this.switchTab(e.target.dataset.tab));
        });

        document.getElementById('start-bulk').addEventListener('click', () => this.startBulkResolution());
        document.getElementById('pause-bulk').addEventListener('click', () => this.pauseBulkResolution());
        document.getElementById('resume-bulk').addEventListener('click', () => this.resumeBulkResolution());
        document.getElementById('cancel-bulk').addEventListener('click', () => this.cancelBulkResolution());
        document.getElementById('refresh-btn').addEventListener('click', () => this.refreshAll());

        document.getElementById('apply-all').addEventListener('click', () => this.applyAllHighestConfidence());
        document.getElementById('reject-all').addEventListener('click', () => this.rejectAll());

        document.getElementById('modal-close').addEventListener('click', () => this.closeModal());
        document.getElementById('review-modal').addEventListener('click', (e) => {
            if (e.target.id === 'review-modal') this.closeModal();
        });

        document.addEventListener('click', (e) => this.handleDelegatedClick(e));
    }

    handleDelegatedClick(e) {
        const sourceBadge = e.target.closest('.source-badge[data-resolution-id][data-source-id]');
        if (sourceBadge) {
            const resolutionId = parseInt(sourceBadge.dataset.resolutionId, 10);
            const sourceId = parseInt(sourceBadge.dataset.sourceId, 10);
            this.selectSource(resolutionId, sourceId);
            return;
        }

        const sourceDetail = e.target.closest('.source-detail[data-source-id]');
        if (sourceDetail) {
            const resolutionId = this.currentReviewId;
            const sourceId = parseInt(sourceDetail.dataset.sourceId, 10);
            this.selectSource(resolutionId, sourceId);
            return;
        }
    }

    switchTab(tab) {
        this.currentTab = tab;
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.tab === tab);
        });

        if (tab === 'review') {
            document.getElementById('review-section').classList.remove('hidden');
            document.getElementById('unprocessed-section').classList.add('hidden');
            document.getElementById('resolved-section').classList.add('hidden');
            document.getElementById('queue-title').textContent = 'Needs Review';
            document.getElementById('review-pagination').classList.remove('hidden');
        } else if (tab === 'unprocessed') {
            document.getElementById('review-section').classList.add('hidden');
            document.getElementById('unprocessed-section').classList.remove('hidden');
            document.getElementById('resolved-section').classList.add('hidden');
            document.getElementById('unprocessed-queue-title').textContent = 'Unprocessed Tracks';
            document.getElementById('unprocessed-pagination').classList.remove('hidden');
            if (this.unprocessedItems.length === 0) {
                this.loadUnprocessedQueue();
            }
        } else {
            document.getElementById('review-section').classList.add('hidden');
            document.getElementById('unprocessed-section').classList.add('hidden');
            document.getElementById('resolved-section').classList.remove('hidden');
            document.getElementById('resolved-queue-title').textContent = 'Resolved Queue';
            document.getElementById('resolved-pagination').classList.remove('hidden');
            if (this.resolvedItems.length === 0) {
                this.loadResolvedQueue();
            }
        }
    }

    async refreshAll() {
        await this.loadStats();
        if (this.currentTab === 'review') {
            await this.loadReviewQueue();
        } else if (this.currentTab === 'unprocessed') {
            await this.loadUnprocessedQueue();
        } else {
            await this.loadResolvedQueue();
        }
        showNotification('Refreshed', 'info');
    }

    async loadStats() {
        try {
            const stats = await durationAPI.getStats();

            document.getElementById('total-count').textContent = stats.missing_duration || 0;
            document.getElementById('unprocessed-count').textContent = stats.unprocessed || 0;
            document.getElementById('review-count').textContent = stats.needs_review || 0;
            document.getElementById('resolved-count').textContent = stats.resolved || 0;
        } catch (error) {
            console.error('Failed to load stats:', error);
        }
    }

    async loadReviewQueue() {
        try {
            const data = await durationAPI.getReviewQueue(this.currentPage, this.pageSize);

            this.totalPages = data.total_pages || 1;
            this.reviewItems = data.items || [];

            this.renderReviewQueue();
            this.updatePagination();
        } catch (error) {
            console.error('Failed to load review queue:', error);
            document.getElementById('review-list').innerHTML =
                '<div class="loading">Failed to load review queue</div>';
        }
    }

    renderReviewQueue() {
        const container = document.getElementById('review-list');

        if (this.reviewItems.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <h3>No items to review</h3>
                    <p>All tracks have been resolved or no tracks need duration resolution.</p>
                </div>
            `;
            return;
        }

        container.innerHTML = this.reviewItems.map(item => this.renderReviewItem(item)).join('');
        this.bindReviewItemEvents();
    }

    renderReviewItem(item) {
        const sources = item.sources || [];
        const resolutionId = item.resolution.id;
        const selectedSourceId = this.selectedSources[resolutionId];

        const sourceBadges = sources.map(src => {
            let className = 'no-result';

            if (src.error_message) {
                className = 'error';
            } else if (src.duration_value > 0) {
                className = src.source_name.toLowerCase().replace(' ', '-');
            }

            const isSelected = selectedSourceId === src.id;
            if (isSelected) {
                className += ' selected-source';
            }

            const mins = Math.floor(src.duration_value / 60);
            const secs = src.duration_value % 60;
            const timeStr = src.duration_value > 0 ? `${mins}:${secs.toString().padStart(2, '0')}` : 'N/A';

            const clickable = src.duration_value > 0 && !src.error_message;
            const dataAttrs = clickable ? `data-resolution-id="${resolutionId}" data-source-id="${src.id}"` : '';

            return `<span class="source-badge ${className}" ${dataAttrs}>
                <span class="source-name">${escapeHtml(src.source_name)}</span>
                <span class="duration-time">${src.duration_value > 0 ? timeStr : '--:--'}</span>
            </span>`;
        }).join('');

        return `
            <div class="review-item" data-resolution-id="${resolutionId}">
                <div class="track-info">
                    <div class="track-title">${escapeHtml(item.track.title)}</div>
                    <div class="track-meta">
                        ${escapeHtml(item.album.artist)} - ${escapeHtml(item.album.title)}
                    </div>
                </div>
                <div class="sources-summary" id="sources-${resolutionId}">
                    ${sourceBadges}
                </div>
                <div class="review-actions-item">
                    <button class="btn btn-primary btn-small" onclick="reviewManager.applySelected(${resolutionId})">Apply</button>
                    <button class="btn btn-secondary btn-small" onclick="reviewManager.openReviewModal(${resolutionId}, 'manual')">Manual</button>
                </div>
            </div>
        `;
    }

    bindReviewItemEvents() {
        const buttons = document.querySelectorAll('.review-btn');
        buttons.forEach(btn => {
            btn.onclick = (e) => {
                const reviewItem = e.target.closest('.review-item');
                const resolutionId = reviewItem ? reviewItem.dataset.resolutionId : null;
                const action = e.target.dataset.action;
                this.openReviewModal(resolutionId, action);
            };
        });
    }

    async loadUnprocessedQueue() {
        try {
            const data = await durationAPI.getUnprocessed(this.unprocessedPage, this.pageSize);

            this.unprocessedTotalPages = data.total_pages || 1;
            this.unprocessedItems = data.tracks || [];

            this.renderUnprocessedQueue();
            this.updateUnprocessedPagination();
        } catch (error) {
            console.error('Failed to load unprocessed tracks:', error);
            document.getElementById('unprocessed-list').innerHTML =
                '<div class="loading">Failed to load unprocessed tracks</div>';
        }
    }

    renderUnprocessedQueue() {
        const container = document.getElementById('unprocessed-list');

        if (this.unprocessedItems.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <h3>No unprocessed tracks</h3>
                    <p>All tracks with missing durations have been scanned.</p>
                </div>
            `;
            return;
        }

        container.innerHTML = this.unprocessedItems.map(item => this.renderUnprocessedItem(item)).join('');
        this.bindUnprocessedItemEvents();
    }

    renderUnprocessedItem(item) {
        return `
            <div class="unprocessed-item" data-track-id="${item.id}">
                <div class="unprocessed-track-info">
                    <div class="unprocessed-track-title">${escapeHtml(item.title)}</div>
                    <div class="unprocessed-track-meta">
                        ${escapeHtml(item.artist)} - ${escapeHtml(item.album_title)}
                    </div>
                </div>
                <div>
                    <button class="btn btn-secondary btn-small unprocessed-manual-btn" data-track-id="${item.id}" data-title="${escapeHtml(item.title)}">Manual</button>
                </div>
            </div>
        `;
    }

    bindUnprocessedItemEvents() {
        document.querySelectorAll('.unprocessed-manual-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const trackId = e.target.dataset.trackId;
                const title = e.target.dataset.title;
                this.openUnprocessedManualModal(trackId, title);
            });
        });
    }

    openUnprocessedManualModal(trackId, title) {
        this.currentReviewId = null;
        document.getElementById('modal-title').textContent = `Manual Duration: ${escapeHtml(title)}`;
        document.getElementById('modal-body').innerHTML = `
            <div class="manual-input">
                <label>Enter Duration:</label>
                <div class="duration-inputs">
                    <input type="number" id="manual-minutes" min="0" max="999" placeholder="Min" style="width: 70px;">
                    <span>:</span>
                    <input type="number" id="manual-seconds" min="0" max="59" placeholder="Sec" style="width: 70px;">
                </div>
            </div>
            <div class="review-notes">
                <label>Notes (optional):</label>
                <textarea id="review-notes" placeholder="Add notes..."></textarea>
            </div>
            <div class="modal-actions">
                <button class="btn btn-secondary" onclick="reviewManager.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="reviewManager.submitUnprocessedManual(${trackId})">Save</button>
            </div>
        `;
        document.getElementById('review-modal').classList.remove('hidden');
    }

    async submitUnprocessedManual(trackId) {
        const minutes = parseInt(document.getElementById('manual-minutes').value, 10) || 0;
        const seconds = parseInt(document.getElementById('manual-seconds').value, 10) || 0;
        const notes = document.getElementById('review-notes').value;

        const totalSeconds = (minutes * 60) + seconds;
        if (totalSeconds <= 0) {
            showNotification('Please enter a valid duration', 'error');
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/duration/track/${trackId}/manual`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    duration: totalSeconds,
                    notes: notes
                })
            });

            const data = await response.json();
            if (response.ok) {
                this.closeModal();
                showNotification('Duration saved', 'success');
                this.loadStats();
                this.loadUnprocessedQueue();
            } else {
                showNotification(data.error || 'Failed to save duration', 'error');
            }
        } catch (error) {
            console.error('Failed to save manual duration:', error);
            showNotification('Failed to save duration', 'error');
        }
    }

    updateUnprocessedPagination() {
        document.getElementById('unprocessed-page-info').textContent =
            `Page ${this.unprocessedPage} of ${this.unprocessedTotalPages}`;
        document.getElementById('unprocessed-prev-page').disabled = this.unprocessedPage <= 1;
        document.getElementById('unprocessed-next-page').disabled = this.unprocessedPage >= this.unprocessedTotalPages;
    }

    changeUnprocessedPage(delta) {
        const newPage = this.unprocessedPage + delta;
        if (newPage >= 1 && newPage <= this.unprocessedTotalPages) {
            this.unprocessedPage = newPage;
            this.loadUnprocessedQueue();
        }
    }

    async loadResolvedQueue() {
        try {
            const data = await durationAPI.getResolved(this.resolvedPage, this.pageSize);

            this.resolvedTotalPages = data.total_pages || 1;
            this.resolvedItems = data.items || [];

            this.renderResolvedQueue();
            this.updateResolvedPagination();
        } catch (error) {
            console.error('Failed to load resolved queue:', error);
            document.getElementById('resolved-list').innerHTML =
                '<div class="loading">Failed to load resolved queue</div>';
        }
    }

    renderResolvedQueue() {
        const container = document.getElementById('resolved-list');

        if (this.resolvedItems.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <h3>No resolved tracks</h3>
                    <p>No tracks have been automatically resolved yet.</p>
                </div>
            `;
            return;
        }

        container.innerHTML = this.resolvedItems.map(item => this.renderResolvedItem(item)).join('');
        this.bindResolvedItemEvents();
    }

    renderResolvedItem(item) {
        const sources = item.sources || [];
        const sourceBadges = sources.map(src => {
            const className = src.source_name.toLowerCase().replace(' ', '-');
            const mins = Math.floor(src.duration_value / 60);
            const secs = src.duration_value % 60;
            const timeStr = `${mins}:${secs.toString().padStart(2, '0')}`;
            const matchClass = src.caused_match ? ' caused-match' : ' not-caused-match';

            return `<span class="source-badge ${className}${matchClass}">
                <span class="source-name">${escapeHtml(src.source_name)}</span>
                <span class="duration-time">${timeStr}</span>
            </span>`;
        }).join('');

        return `
            <div class="review-item" data-resolution-id="${item.resolution.id}">
                <div class="track-info">
                    <div class="track-title">${escapeHtml(item.track.title)}</div>
                    <div class="track-meta">
                        ${escapeHtml(item.album.artist)} - ${escapeHtml(item.album.title)}
                    </div>
                </div>
                <div class="sources-summary">
                    ${sourceBadges}
                </div>
                <div class="review-actions-item">
                    <button class="btn btn-warning btn-small resolved-reject-btn" data-resolution-id="${item.resolution.id}">Reject</button>
                </div>
            </div>
        `;
    }

    bindResolvedItemEvents() {
        document.querySelectorAll('.resolved-reject-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const resolutionId = e.target.dataset.resolutionId;
                this.rejectResolved(resolutionId);
            });
        });
    }

    async rejectResolved(resolutionId) {
        if (!confirm('Reject this resolved track? It will move back to the needs review queue.')) {
            return;
        }
        try {
            const response = await fetch(`${API_BASE}/duration/review/${resolutionId}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    action: 'reject'
                })
            });

            if (response.ok) {
                showNotification('Track moved to needs review', 'success');
                this.loadResolvedQueue();
            } else {
                const data = await response.json();
                showNotification(data.error || 'Failed to reject', 'error');
            }
        } catch (error) {
            console.error('Failed to reject resolved track:', error);
            showNotification('Failed to reject', 'error');
        }
    }

    updateResolvedPagination() {
        document.getElementById('resolved-page-info').textContent = `Page ${this.resolvedPage} of ${this.resolvedTotalPages}`;
        document.getElementById('resolved-prev-page').disabled = this.resolvedPage <= 1;
        document.getElementById('resolved-next-page').disabled = this.resolvedPage >= this.resolvedTotalPages;
    }

    changeResolvedPage(delta) {
        const newPage = this.resolvedPage + delta;
        if (newPage >= 1 && newPage <= this.resolvedTotalPages) {
            this.resolvedPage = newPage;
            this.loadResolvedQueue();
        }
    }

    async openReviewModal(resolutionId, defaultAction) {
        this.currentReviewId = resolutionId;

        try {
            const data = await durationAPI.getReviewDetails(resolutionId);

            if (!data.resolution) {
                showNotification('Resolution not found', 'error');
                return;
            }

            const resolution = data.resolution;
            const sources = data.sources || [];
            const track = data.track;
            const album = data.album;
            const selectedSourceId = this.selectedSources[resolutionId];

            document.getElementById('modal-title').textContent = `Review: ${escapeHtml(track.title)}`;

            let selectedSourceInfo = '';
            if (selectedSourceId) {
                const selectedSource = sources.find(s => s.id === selectedSourceId);
                if (selectedSource) {
                    const formatted = formatDuration(selectedSource.duration_value);
                    selectedSourceInfo = `
                        <div class="selected-source-info">
                            <span class="selected-label">Selected:</span>
                            <span class="source-badge ${selectedSource.source_name.toLowerCase().replace(' ', '-')} selected">
                                ${escapeHtml(selectedSource.source_name)} - ${formatted}
                            </span>
                        </div>
                    `;
                }
            }

            let sourcesHtml = sources.map(src => {
                const formatted = formatDuration(src.duration);
                const errorDisplay = src.error_message
                    ? `<div style="color: #c62828; font-size: 13px;">Error: ${escapeHtml(src.error_message)}</div>`
                    : `<div class="source-scores">
                        <span>Match: ${(src.match_score * 100).toFixed(0)}%</span>
                        <span>Confidence: ${(src.confidence * 100).toFixed(0)}%</span>
                       </div>`;

                const isSelected = selectedSourceId === src.id;
                const dataAttrs = src.duration_value > 0
                    ? `data-source-id="${src.id}"`
                    : '';

                return `
                    <div class="source-detail ${isSelected ? 'selected' : ''}" id="source-${src.id}" data-source-id="${src.id}" data-duration="${src.duration_value}" ${dataAttrs}>
                        <div class="source-header">
                            <span class="source-name">${escapeHtml(src.source_name)}</span>
                            <span class="source-duration">${src.duration_value > 0 ? formatted : 'N/A'}</span>
                        </div>
                        ${errorDisplay}
                        ${src.external_url && !src.error_message
                            ? `<div style="margin-top: 8px;"><a href="${escapeHtml(src.external_url)}" target="_blank" rel="noopener">View on ${escapeHtml(src.source_name)}</a></div>`
                            : ''
                        }
                    </div>
                `;
            }).join('');

            document.getElementById('modal-body').innerHTML = `
                ${selectedSourceInfo}
                <div style="margin-bottom: 16px;">
                    <strong>${escapeHtml(album.artist)}</strong> - ${escapeHtml(album.title)}
                </div>
                ${sourcesHtml}
                <div class="manual-input">
                    <label>Or enter manual duration:</label>
                    <div class="duration-inputs">
                        <input type="number" id="manual-minutes" min="0" max="999" placeholder="Min" style="width: 70px;">
                        <span>:</span>
                        <input type="number" id="manual-seconds" min="0" max="59" placeholder="Sec" style="width: 70px;">
                    </div>
                </div>
                <div class="review-notes">
                    <label>Notes (optional):</label>
                    <textarea id="review-notes" placeholder="Add notes about this review..."></textarea>
                </div>
                <div class="modal-actions">
                    <button class="btn btn-secondary" onclick="reviewManager.closeModal()">Cancel</button>
                    <button class="btn btn-warning" onclick="reviewManager.submitReview(${resolutionId}, 'reject', 0, document.getElementById('review-notes').value)">Reject</button>
                    <button class="btn btn-primary" onclick="reviewManager.submitSelectedOrManual(${resolutionId})">Apply Selected</button>
                </div>
            `;

            document.getElementById('review-modal').classList.remove('hidden');
        } catch (error) {
            console.error('Failed to load review details:', error);
            showNotification('Failed to load review details', 'error');
        }
    }

    selectSource(resolutionId, sourceId) {
        this.selectedSources[resolutionId] = sourceId;
        this.renderReviewQueue();
    }

    async applySelected(resolutionId) {
        const selectedSourceId = this.selectedSources[resolutionId];
        if (!selectedSourceId) {
            showNotification('Please select a source first', 'error');
            return;
        }

        const reviewItem = this.reviewItems.find(item => item.resolution.id === resolutionId);
        if (!reviewItem) {
            showNotification('Review item not found', 'error');
            return;
        }

        const source = reviewItem.sources.find(s => s.id === selectedSourceId);
        if (!source) {
            showNotification('Source not found', 'error');
            return;
        }

        try {
            await durationAPI.submitReview(resolutionId, 'apply', source.duration_value, '');

            delete this.selectedSources[resolutionId];
            await this.loadStats();
            await this.loadReviewQueue();
            showNotification('Duration applied successfully', 'success');
        } catch (error) {
            console.error('Failed to apply:', error);
            showNotification(error.message, 'error');
        }
    }

    async submitSelectedOrManual(resolutionId) {
        const minutes = parseInt(document.getElementById('manual-minutes').value, 10) || 0;
        const seconds = parseInt(document.getElementById('manual-seconds').value, 10) || 0;
        const manualDuration = (minutes * 60) + seconds;
        const notes = document.getElementById('review-notes').value;

        const selectedSourceId = this.selectedSources[resolutionId];
        if (selectedSourceId) {
            const reviewItem = this.reviewItems.find(item => item.resolution.id === resolutionId);
            if (reviewItem) {
                const source = reviewItem.sources.find(s => s.id === selectedSourceId);
                if (source) {
                    await this.submitReview(resolutionId, 'apply', source.duration_value, notes);
                    delete this.selectedSources[resolutionId];
                    return;
                }
            }
        } else if (manualDuration > 0) {
            await this.submitManual(resolutionId, manualDuration, notes);
        } else {
            showNotification('Please select a source or enter a valid duration', 'error');
        }
    }

    closeModal() {
        this.currentReviewId = null;
        document.getElementById('review-modal').classList.add('hidden');
    }

    async submitReview(resolutionId, action, duration, notes) {
        try {
            await durationAPI.submitReview(resolutionId, action, duration, notes);

            this.closeModal();
            await this.loadStats();
            await this.loadReviewQueue();
            showNotification('Review submitted successfully', 'success');
        } catch (error) {
            console.error('Failed to submit review:', error);
            showNotification(error.message, 'error');
        }
    }

    async submitManual(resolutionId, duration, notes) {
        if (!duration || duration <= 0) {
            showNotification('Please enter a valid duration', 'error');
            return;
        }
        await this.submitReview(resolutionId, 'manual', parseInt(duration), notes);
    }

    async submitReview(resolutionId, action, duration, notes) {
        try {
            await durationAPI.submitReview(resolutionId, action, duration, notes);

            this.closeModal();
            await this.loadStats();
            await this.loadReviewQueue();
            showNotification('Review submitted successfully', 'success');
        } catch (error) {
            console.error('Failed to submit review:', error);
            showNotification(error.message, 'error');
        }
    }

    async submitManual(resolutionId, duration, notes) {
        if (!duration || duration <= 0) {
            showNotification('Please enter a valid duration', 'error');
            return;
        }
        await this.submitReview(resolutionId, 'manual', parseInt(duration), notes);
    }

    async startBulkResolution() {
        try {
            await durationAPI.startBulkResolution();

            this.updateBulkButtons('running');
            showNotification('Bulk resolution started', 'success');
        } catch (error) {
            console.error('Failed to start bulk resolution:', error);
            showNotification(error.message, 'error');
        }
    }

    async pauseBulkResolution() {
        try {
            await durationAPI.pauseBulkResolution();

            this.updateBulkButtons('paused');
            showNotification('Bulk resolution paused', 'success');
        } catch (error) {
            console.error('Failed to pause bulk resolution:', error);
            showNotification(error.message, 'error');
        }
    }

    async resumeBulkResolution() {
        try {
            await durationAPI.resumeBulkResolution();

            this.updateBulkButtons('running');
            showNotification('Bulk resolution resumed', 'success');
        } catch (error) {
            console.error('Failed to resume bulk resolution:', error);
            showNotification(error.message, 'error');
        }
    }

    async cancelBulkResolution() {
        if (!confirm('Are you sure you want to cancel the bulk resolution?')) {
            return;
        }

        try {
            await durationAPI.cancelBulkResolution();

            this.updateBulkButtons('idle');
            showNotification('Bulk resolution cancelled', 'success');
        } catch (error) {
            console.error('Failed to cancel bulk resolution:', error);
            showNotification(error.message, 'error');
        }
    }

    updateBulkButtons(status) {
        const startBtn = document.getElementById('start-bulk');
        const pauseBtn = document.getElementById('pause-bulk');
        const resumeBtn = document.getElementById('resume-bulk');
        const cancelBtn = document.getElementById('cancel-bulk');

        startBtn.classList.add('hidden');
        pauseBtn.classList.add('hidden');
        resumeBtn.classList.add('hidden');
        cancelBtn.classList.add('hidden');

        if (status === 'running') {
            pauseBtn.classList.remove('hidden');
            cancelBtn.classList.remove('hidden');
        } else if (status === 'paused') {
            resumeBtn.classList.remove('hidden');
            cancelBtn.classList.remove('hidden');
        } else {
            startBtn.classList.remove('hidden');
        }
    }

    startProgressPolling() {
        this.updateProgress();
        this.progressInterval = setInterval(() => this.updateProgress(), 2000);
    }

    async updateProgress() {
        try {
            const progress = await durationAPI.getProgress();

            const container = document.getElementById('progress-container');
            const isRunning = progress.status === 'running' || progress.status === 'paused';

            // Track if resolution was running to detect completion
            if (isRunning) {
                this.wasRunning = true;
            }

            if (progress.status === 'idle' || progress.status === 'completed') {
                container.classList.add('hidden');
                this.updateBulkButtons('idle');

                // Reload stats and review queue when resolution completes
                if (this.wasRunning && progress.processed_tracks > 0) {
                    await this.loadStats();
                    await this.loadReviewQueue();
                    showNotification(
                        `Resolution complete: ${progress.resolved_count} resolved, ${progress.needs_review_count} need review, ${progress.failed_count} failed`,
                        'success'
                    );
                    this.wasRunning = false;
                }
                return;
            }

            container.classList.remove('hidden');

            const percent = progress.percent_complete || 0;
            document.getElementById('progress-fill').style.width = `${percent}%`;
            document.getElementById('progress-text').textContent =
                `Processing: ${progress.processed_tracks}/${progress.total_tracks} (${percent.toFixed(1)}%) - ${progress.resolved_count} resolved, ${progress.needs_review_count} need review`;

            if (progress.status === 'running') {
                this.updateBulkButtons('running');
            } else if (progress.status === 'paused') {
                this.updateBulkButtons('paused');
            }
        } catch (error) {
            console.error('Failed to update progress:', error);
        }
    }

    async applyAllHighestConfidence() {
        if (!confirm('Apply highest confidence source for all pending reviews?')) {
            return;
        }

        try {
            const data = await durationAPI.getReviewQueue(1, 1000);

            const items = data.items || [];
            const resolutionIds = items.map(item => item.resolution.id);

            if (resolutionIds.length === 0) {
                showNotification('No items to apply', 'info');
                return;
            }

            await api.post('/duration/review/bulk', {
                action: 'apply_all',
                resolution_ids: resolutionIds
            });

            await this.loadStats();
            await this.loadReviewQueue();
            showNotification('Applied all reviews successfully', 'success');
        } catch (error) {
            console.error('Failed to apply all:', error);
            showNotification(error.message, 'error');
        }
    }

    async rejectAll() {
        if (!confirm('Reject all pending reviews? This will mark them as reviewed without applying durations.')) {
            return;
        }

        try {
            const data = await durationAPI.getReviewQueue(1, 1000);

            const items = data.items || [];
            const resolutionIds = items.map(item => item.resolution.id);

            if (resolutionIds.length === 0) {
                showNotification('No items to reject', 'info');
                return;
            }

            await api.post('/duration/review/bulk', {
                action: 'reject_all',
                resolution_ids: resolutionIds
            });

            await this.loadStats();
            await this.loadReviewQueue();
            showNotification('Rejected all reviews', 'success');
        } catch (error) {
            console.error('Failed to reject all:', error);
            showNotification(error.message, 'error');
        }
    }

    changePage(delta) {
        const newPage = this.currentPage + delta;
        if (newPage >= 1 && newPage <= this.totalPages) {
            this.currentPage = newPage;
            this.loadReviewQueue();
        }
    }

    updatePagination() {
        document.getElementById('page-info').textContent = `Page ${this.currentPage} of ${this.totalPages}`;
        document.getElementById('prev-page').disabled = this.currentPage <= 1;
        document.getElementById('next-page').disabled = this.currentPage >= this.totalPages;
    }

    formatDuration(seconds) {
        if (!seconds || seconds <= 0) return 'N/A';
        const mins = Math.floor(seconds / 60);
        const secs = seconds % 60;
        return `${mins}:${secs.toString().padStart(2, '0')}`;
    }
}

let reviewManager;

document.addEventListener('DOMContentLoaded', () => {
    reviewManager = new ResolutionCenterManager();
    window.reviewManager = reviewManager; // Make available globally for inline onclick handlers
});
