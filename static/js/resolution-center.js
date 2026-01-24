import { api, durationAPI } from './modules/api.js';
import { escapeHtml, formatDuration, showNotification, normalizeArtistName, normalizeTitle } from './modules/utils.js';

const API_BASE = '/api';

class ResolutionQueueManager {
    constructor(manager) {
        this.manager = manager;
    }

    async loadReviewQueue() {
        try {
            const data = await durationAPI.getReviewQueue(this.manager.currentPage, this.manager.pageSize, this.manager.reviewSearchQuery);

            this.manager.totalPages = data.total_pages || 1;
            this.manager.reviewItems = data.items || [];

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

        if (this.manager.reviewItems.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <h3>No items to review</h3>
                    <p>All tracks have been resolved or no tracks need duration resolution.</p>
                </div>
            `;
            return;
        }

        container.innerHTML = this.manager.reviewItems.map(item => this.renderReviewItem(item)).join('');
    }

    renderReviewItem(item) {
        const sources = item.sources || [];
        const resolutionId = item.resolution.id;
        const selectedSourceId = this.manager.selectedSources[resolutionId];

        const sourceBadges = sources.map(src => {
            let className;

            if (src.duration_value > 0) {
                className = src.source_name.toLowerCase().replace(/[.\s]/g, '');
            } else {
                className = 'error';
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
                    <div class="track-title">${escapeHtml(normalizeTitle(item.track.title))}</div>
                    <div class="track-meta">
                        ${escapeHtml(normalizeArtistName(item.album.artist))} - ${escapeHtml(normalizeTitle(item.album.title))}
                    </div>
                </div>
                <div class="sources-summary" id="sources-${resolutionId}">
                    ${sourceBadges}
                </div>
                <div class="review-actions-item">
                    <button class="btn btn-primary btn-small" onclick="reviewManager.handleReviewClick(${resolutionId})">Apply</button>
                </div>
            </div>
        `;
    }

    async loadUnprocessedQueue() {
        try {
            const data = await durationAPI.getUnprocessed(this.manager.unprocessedPage, this.manager.pageSize, this.manager.unprocessedSearchQuery);

            this.manager.unprocessedTotalPages = data.total_pages || 1;
            this.manager.unprocessedItems = data.tracks || [];

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

        if (this.manager.unprocessedItems.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <h3>No unprocessed tracks</h3>
                    <p>All tracks with missing durations have been scanned.</p>
                </div>
            `;
            return;
        }

        container.innerHTML = this.manager.unprocessedItems.map(item => this.renderUnprocessedItem(item)).join('');
        this.bindUnprocessedItemEvents();
    }

    renderUnprocessedItem(item) {
        return `
            <div class="unprocessed-item" data-track-id="${item.id}">
                <div class="unprocessed-track-info">
                    <div class="unprocessed-track-title">${escapeHtml(normalizeTitle(item.title))}</div>
                    <div class="unprocessed-track-meta">
                        ${escapeHtml(normalizeArtistName(item.artist || ''))} - ${escapeHtml(normalizeTitle(item.album_title || ''))}
                    </div>
                </div>
                <div>
                    <button class="btn btn-secondary btn-small unprocessed-manual-btn" data-track-id="${item.id}" data-title="${escapeHtml(normalizeTitle(item.title))}">Manual</button>
                </div>
            </div>
        `;
    }

    bindUnprocessedItemEvents() {
        document.querySelectorAll('.unprocessed-manual-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const trackId = e.target.dataset.trackId;
                const title = e.target.dataset.title;
                this.manager.openUnprocessedManualModal(trackId, title);
            });
        });
    }

    async loadResolvedQueue() {
        try {
            const data = await durationAPI.getResolved(this.manager.resolvedPage, this.manager.pageSize, this.manager.resolvedSearchQuery);

            this.manager.resolvedTotalPages = data.total_pages || 1;
            this.manager.resolvedItems = data.items || [];

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

        if (this.manager.resolvedItems.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <h3>No resolved tracks</h3>
                    <p>No tracks have been automatically resolved yet.</p>
                </div>
            `;
            return;
        }

        container.innerHTML = this.manager.resolvedItems.map(item => this.renderResolvedItem(item)).join('');
        this.bindResolvedItemEvents();
    }

    renderResolvedItem(item) {
        let sourceBadges = '';
        
        const isManual = item.resolution.review_action === 'manual';
        
        if (isManual && item.resolution.resolved_duration) {
            const manualTime = formatDuration(item.resolution.resolved_duration);
            sourceBadges = `<span class="source-badge manual caused-match">
                <span class="source-name">Manual</span>
                <span class="duration-time">${manualTime}</span>
            </span>`;
        }
        
        const sources = item.sources || [];
        const autoSourceBadges = sources
            .filter(src => src.duration_value > 0)
            .map(src => {
                const className = src.source_name.toLowerCase().replace(/[.\s]/g, '');
                const mins = Math.floor(src.duration_value / 60);
                const secs = src.duration_value % 60;
                const timeStr = `${mins}:${secs.toString().padStart(2, '0')}`;
                const matchClass = src.caused_match ? ' caused-match' : ' not-caused-match';

                return `<span class="source-badge ${className}${matchClass}">
                    <span class="source-name">${escapeHtml(src.source_name)}</span>
                    <span class="duration-time">${timeStr}</span>
                </span>`;
            }).join('');

        sourceBadges += autoSourceBadges;

        return `
            <div class="review-item" data-resolution-id="${item.resolution.id}">
                <div class="track-info">
                    <div class="track-title">${escapeHtml(normalizeTitle(item.track.title))}</div>
                    <div class="track-meta">
                        ${escapeHtml(normalizeArtistName(item.album.artist))} - ${escapeHtml(normalizeTitle(item.album.title))}
                    </div>
                </div>
                <div class="sources-summary">
                    ${sourceBadges}
                </div>
                <div class="review-actions-item">
                    <span class="resolved-duration">${formatDuration(item.resolution.resolved_duration)}</span>
                    <button class="btn btn-warning btn-small resolved-reject-btn" data-resolution-id="${item.resolution.id}">Reject</button>
                </div>
            </div>
        `;
    }

    bindResolvedItemEvents() {
        document.querySelectorAll('.resolved-reject-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const resolutionId = e.target.dataset.resolutionId;
                this.manager.rejectResolved(resolutionId);
            });
        });

        document.querySelectorAll('.unprocessed-manual-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const trackId = e.target.dataset.trackId;
                const title = e.target.dataset.title;
                this.manager.openUnprocessedManualModal(trackId, title);
            });
        });
    }

    updatePagination() {
        document.getElementById('page-info').textContent = `Page ${this.manager.currentPage} of ${this.manager.totalPages}`;
        document.getElementById('prev-page').disabled = this.manager.currentPage <= 1;
        document.getElementById('next-page').disabled = this.manager.currentPage >= this.manager.totalPages;
    }

    updateUnprocessedPagination() {
        document.getElementById('unprocessed-page-info').textContent =
            `Page ${this.manager.unprocessedPage} of ${this.manager.unprocessedTotalPages}`;
        document.getElementById('unprocessed-prev-page').disabled = this.manager.unprocessedPage <= 1;
        document.getElementById('unprocessed-next-page').disabled = this.manager.unprocessedPage >= this.manager.unprocessedTotalPages;
    }

    changeUnprocessedPage(delta) {
        const newPage = this.manager.unprocessedPage + delta;
        if (newPage >= 1 && newPage <= this.manager.unprocessedTotalPages) {
            this.manager.unprocessedPage = newPage;
            this.loadUnprocessedQueue();
        }
    }

    updateResolvedPagination() {
        document.getElementById('resolved-page-info').textContent = `Page ${this.manager.resolvedPage} of ${this.manager.resolvedTotalPages}`;
        document.getElementById('resolved-prev-page').disabled = this.manager.resolvedPage <= 1;
        document.getElementById('resolved-next-page').disabled = this.manager.resolvedPage >= this.manager.resolvedTotalPages;
    }

    changeResolvedPage(delta) {
        const newPage = this.manager.resolvedPage + delta;
        if (newPage >= 1 && newPage <= this.manager.resolvedTotalPages) {
            this.manager.resolvedPage = newPage;
            this.loadResolvedQueue();
        }
    }
}

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
        this.queueManager = null;
        this.reviewSearchQuery = '';
        this.unprocessedSearchQuery = '';
        this.resolvedSearchQuery = '';
        this.reviewSearchTimeout = null;
        this.unprocessedSearchTimeout = null;
        this.resolvedSearchTimeout = null;
        this.init();
    }

    async init() {
        this.queueManager = new ResolutionQueueManager(this);
        this.bindEvents();
        await this.loadStats();
        await this.queueManager.loadReviewQueue();
        this.startProgressPolling();
    }

    bindEvents() {
        document.getElementById('prev-page').addEventListener('click', () => this.changePage(-1));
        document.getElementById('next-page').addEventListener('click', () => this.changePage(1));
        document.getElementById('resolved-prev-page').addEventListener('click', () => this.queueManager.changeResolvedPage(-1));
        document.getElementById('resolved-next-page').addEventListener('click', () => this.queueManager.changeResolvedPage(1));
        document.getElementById('unprocessed-prev-page').addEventListener('click', () => this.queueManager.changeUnprocessedPage(-1));
        document.getElementById('unprocessed-next-page').addEventListener('click', () => this.queueManager.changeUnprocessedPage(1));

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

        this.bindSearchEvents();
    }

    bindSearchEvents() {
        const reviewSearchInput = document.getElementById('review-search');
        const unprocessedSearchInput = document.getElementById('unprocessed-search');
        const resolvedSearchInput = document.getElementById('resolved-search');

        if (reviewSearchInput) {
            reviewSearchInput.addEventListener('input', (e) => {
                clearTimeout(this.reviewSearchTimeout);
                const query = e.target.value.trim();
                this.reviewSearchTimeout = setTimeout(() => {
                    this.reviewSearchQuery = query;
                    this.currentPage = 1;
                    this.queueManager.loadReviewQueue();
                }, 300);
            });

            document.getElementById('review-search-clear')?.addEventListener('click', () => {
                reviewSearchInput.value = '';
                this.reviewSearchQuery = '';
                this.currentPage = 1;
                this.queueManager.loadReviewQueue();
                reviewSearchInput.focus();
            });
        }

        if (unprocessedSearchInput) {
            unprocessedSearchInput.addEventListener('input', (e) => {
                clearTimeout(this.unprocessedSearchTimeout);
                const query = e.target.value.trim();
                this.unprocessedSearchTimeout = setTimeout(() => {
                    this.unprocessedSearchQuery = query;
                    this.unprocessedPage = 1;
                    this.queueManager.loadUnprocessedQueue();
                }, 300);
            });

            document.getElementById('unprocessed-search-clear')?.addEventListener('click', () => {
                unprocessedSearchInput.value = '';
                this.unprocessedSearchQuery = '';
                this.unprocessedPage = 1;
                this.queueManager.loadUnprocessedQueue();
                unprocessedSearchInput.focus();
            });
        }

        if (resolvedSearchInput) {
            resolvedSearchInput.addEventListener('input', (e) => {
                clearTimeout(this.resolvedSearchTimeout);
                const query = e.target.value.trim();
                this.resolvedSearchTimeout = setTimeout(() => {
                    this.resolvedSearchQuery = query;
                    this.resolvedPage = 1;
                    this.queueManager.loadResolvedQueue();
                }, 300);
            });

            document.getElementById('resolved-search-clear')?.addEventListener('click', () => {
                resolvedSearchInput.value = '';
                this.resolvedSearchQuery = '';
                this.resolvedPage = 1;
                this.queueManager.loadResolvedQueue();
                resolvedSearchInput.focus();
            });
        }
    }

    handleDelegatedClick(e) {
        const sourceBadge = e.target.closest('.source-badge[data-resolution-id][data-source-id]');
        if (sourceBadge) {
            const resolutionId = parseInt(sourceBadge.dataset.resolutionId, 10);
            const sourceId = parseInt(sourceBadge.dataset.sourceId, 10);
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
                this.queueManager.loadUnprocessedQueue();
            }
        } else {
            document.getElementById('review-section').classList.add('hidden');
            document.getElementById('unprocessed-section').classList.add('hidden');
            document.getElementById('resolved-section').classList.remove('hidden');
            document.getElementById('resolved-queue-title').textContent = 'Resolved Queue';
            document.getElementById('resolved-pagination').classList.remove('hidden');
            if (this.resolvedItems.length === 0) {
                this.queueManager.loadResolvedQueue();
            }
        }
    }

    async refreshAll() {
        await this.loadStats();
        if (this.currentTab === 'review') {
            await this.queueManager.loadReviewQueue();
        } else if (this.currentTab === 'unprocessed') {
            await this.queueManager.loadUnprocessedQueue();
        } else {
            await this.queueManager.loadResolvedQueue();
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

    selectSource(resolutionId, sourceId) {
        if (this.selectedSources[resolutionId] === sourceId) {
            delete this.selectedSources[resolutionId];
        } else {
            this.selectedSources[resolutionId] = sourceId;
        }
        this.queueManager.renderReviewQueue();
    }

    handleReviewClick(resolutionId) {
        const selectedSourceId = this.selectedSources[resolutionId];
        if (selectedSourceId) {
            this.applySelected(resolutionId);
        } else {
            this.openReviewModal(resolutionId, 'apply');
        }
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
            await this.queueManager.loadReviewQueue();
            showNotification('Duration applied successfully', 'success');
        } catch (error) {
            console.error('Failed to apply:', error);
            showNotification(error.message, 'error');
        }
    }

    async openReviewModal(resolutionId, action) {
        const reviewItem = this.reviewItems.find(item => item.resolution.id === resolutionId);
        if (!reviewItem) {
            showNotification('Review item not found', 'error');
            return;
        }

        this.currentReviewId = resolutionId;
        const modal = document.getElementById('review-modal');
        const modalTitle = document.getElementById('modal-title');
        const modalBody = document.getElementById('modal-body');

        modalTitle.textContent = action === 'manual' ? 'Enter Manual Duration' : 'Review Track';

        const sources = reviewItem.sources || [];
        const sourceDetails = sources.map(src => {
            const mins = Math.floor(src.duration_value / 60);
            const secs = src.duration_value % 60;
            const timeStr = src.duration_value > 0 ? `${mins}:${secs.toString().padStart(2, '0')}` : 'N/A';
            const isSelected = this.selectedSources[resolutionId] === src.id;
            const clickable = src.duration_value > 0 && !src.error_message;

            return `
                <div class="source-detail ${isSelected ? 'selected' : ''}"
                     data-source-id="${src.id}"
                     data-duration="${src.duration_value}"
                     style="${!clickable ? 'cursor: default;' : 'cursor: pointer;'}">
                    <div class="source-header">
                        <span class="source-name">${escapeHtml(src.source_name)}</span>
                        <span class="source-duration">${timeStr}</span>
                    </div>
                    <div class="source-scores">
                        <span>Confidence: ${(src.confidence * 100).toFixed(0)}%</span>
                        ${src.match_score > 0 ? `<span>Match: ${(src.match_score * 100).toFixed(0)}%</span>` : ''}
                    </div>
                    ${src.error_message ? `<p class="error-message">${escapeHtml(src.error_message)}</p>` : ''}
                </div>
            `;
        }).join('');

        modalBody.innerHTML = `
            <div class="track-info" style="margin-bottom: 20px;">
                <div class="track-title" style="font-size: 18px; font-weight: 600;">
                    ${escapeHtml(normalizeTitle(reviewItem.track.title))}
                </div>
                <div class="track-meta">
                    ${escapeHtml(normalizeArtistName(reviewItem.album.artist))} - ${escapeHtml(normalizeTitle(reviewItem.album.title))}
                </div>
            </div>
            <div class="sources-list">
                <h4>Available Sources</h4>
                ${sourceDetails}
            </div>
            <div class="manual-input">
                <label>Or enter duration manually:</label>
                <div class="duration-inputs">
                    <input type="number" id="manual-minutes" min="0" max="999" placeholder="MM" value="0">
                    <span>:</span>
                    <input type="number" id="manual-seconds" min="0" max="59" placeholder="SS" value="0">
                </div>
            </div>
            <div class="review-notes">
                <label>Notes (optional):</label>
                <textarea id="review-notes" placeholder="Add any notes about this review..."></textarea>
            </div>
            <div class="modal-actions">
                <button class="btn btn-primary" onclick="reviewManager.submitSelectedOrManual(${resolutionId})">Apply</button>
                <button class="btn btn-warning" onclick="reviewManager.rejectSelected(${resolutionId})">Reject</button>
                <button class="btn btn-secondary" onclick="reviewManager.closeModal()">Cancel</button>
            </div>
        `;

        document.querySelectorAll('.source-detail[data-duration="0"]').forEach(el => {
            el.style.cursor = 'default';
        });

        document.querySelectorAll('.source-detail[data-duration]').forEach(el => {
            if (el.dataset.duration !== '0') {
                el.addEventListener('click', (e) => {
                    const sourceId = parseInt(el.dataset.sourceId, 10);
                    this.selectSource(resolutionId, sourceId);
                    this.openReviewModal(resolutionId, action);
                });
            }
        });

        modal.classList.remove('hidden');
    }

    async openUnprocessedManualModal(trackId, title) {
        this.currentTrackId = trackId;
        const modal = document.getElementById('review-modal');
        const modalTitle = document.getElementById('modal-title');
        const modalBody = document.getElementById('modal-body');

        modalTitle.textContent = 'Enter Manual Duration';
        modalBody.innerHTML = `
            <div class="track-info" style="margin-bottom: 20px;">
                <div class="track-title" style="font-size: 18px; font-weight: 600;">
                    ${escapeHtml(title)}
                </div>
            </div>
            <div class="manual-input">
                <label>Enter duration:</label>
                <div class="duration-inputs">
                    <input type="number" id="manual-minutes" min="0" max="999" placeholder="MM" value="0">
                    <span>:</span>
                    <input type="number" id="manual-seconds" min="0" max="59" placeholder="SS" value="0">
                </div>
            </div>
            <div class="review-notes">
                <label>Notes (optional):</label>
                <textarea id="review-notes" placeholder="Add any notes..."></textarea>
            </div>
            <div class="modal-actions">
                <button class="btn btn-secondary" onclick="reviewManager.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="reviewManager.submitUnprocessedManual()">Save Duration</button>
            </div>
        `;

        modal.classList.remove('hidden');
    }

    async submitUnprocessedManual() {
        const minutes = parseInt(document.getElementById('manual-minutes').value, 10) || 0;
        const seconds = parseInt(document.getElementById('manual-seconds').value, 10) || 0;
        const duration = (minutes * 60) + seconds;
        const notes = document.getElementById('review-notes').value;

        if (duration <= 0) {
            showNotification('Please enter a valid duration', 'error');
            return;
        }

        try {
            await durationAPI.manualDuration(this.currentTrackId, duration, notes);
            this.closeModal();
            await this.loadStats();
            await this.queueManager.loadUnprocessedQueue();
            showNotification('Duration saved successfully', 'success');
        } catch (error) {
            console.error('Failed to save manual duration:', error);
            showNotification(error.message, 'error');
        }
    }

    async rejectSelected(resolutionId) {
        const notes = document.getElementById('review-notes')?.value || '';
        try {
            await durationAPI.reject(resolutionId, notes);
            this.closeModal();
            await this.loadStats();
            await this.queueManager.loadReviewQueue();
            showNotification('Track rejected', 'success');
        } catch (error) {
            console.error('Failed to reject:', error);
            showNotification(error.message, 'error');
        }
    }

    async rejectResolved(resolutionId) {
        if (!confirm('Reject this resolved track? This will mark it as needing review.')) {
            return;
        }

        try {
            await durationAPI.reject(resolutionId, 'Rejected from resolved queue');
            await this.loadStats();
            await this.queueManager.loadResolvedQueue();
            showNotification('Track rejected', 'success');
        } catch (error) {
            console.error('Failed to reject resolved:', error);
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
            await this.submitReview(resolutionId, 'manual', manualDuration, notes);
        } else {
            showNotification('Please select a source or enter a valid duration', 'error');
        }
    }

    closeModal() {
        this.currentReviewId = null;
        this.currentTrackId = null;
        document.getElementById('review-modal').classList.add('hidden');
    }

    async submitReview(resolutionId, action, duration, notes) {
        try {
            await durationAPI.submitReview(resolutionId, action, duration, notes);

            this.closeModal();
            await this.loadStats();
            await this.queueManager.loadReviewQueue();
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

            if (isRunning) {
                this.wasRunning = true;
            }

            if (progress.status === 'idle' || progress.status === 'completed') {
                container.classList.add('hidden');
                this.updateBulkButtons('idle');

                if (this.wasRunning && progress.processed_tracks > 0) {
                    await this.loadStats();
                    await this.queueManager.loadReviewQueue();
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
            await this.queueManager.loadReviewQueue();
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
            await this.queueManager.loadReviewQueue();
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
            this.queueManager.loadReviewQueue();
        }
    }

    updatePagination() {
        this.queueManager.updatePagination();
    }
}

let reviewManager;

document.addEventListener('DOMContentLoaded', () => {
    reviewManager = new ResolutionCenterManager();
    window.reviewManager = reviewManager;
});
