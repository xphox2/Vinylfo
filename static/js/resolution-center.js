import { api, durationAPI } from './modules/api.js';
import { escapeHtml, formatDuration, showNotification, normalizeArtistName, normalizeTitle } from './modules/utils.js';

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

    selectSource(resolutionId, sourceId) {
        if (this.selectedSources[resolutionId] === sourceId) {
            delete this.selectedSources[resolutionId];
        } else {
            this.selectedSources[resolutionId] = sourceId;
        }
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
}

let reviewManager;

document.addEventListener('DOMContentLoaded', () => {
    reviewManager = new ResolutionCenterManager();
    window.reviewManager = reviewManager;
});
