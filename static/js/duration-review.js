const API_BASE = '/api';

class DurationReviewManager {
    constructor() {
        this.currentPage = 1;
        this.pageSize = 20;
        this.totalPages = 1;
        this.reviewItems = [];
        this.wasRunning = false;
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
    }

    async refreshAll() {
        await this.loadStats();
        await this.loadReviewQueue();
        this.showNotification('Refreshed', 'info');
    }

    async loadStats() {
        try {
            const response = await fetch(`${API_BASE}/duration/stats`);
            const stats = await response.json();

            document.getElementById('total-count').textContent = stats.missing_duration || 0;
            document.getElementById('resolved-count').textContent = stats.resolved || 0;
            document.getElementById('review-count').textContent = stats.needs_review || 0;
        } catch (error) {
            console.error('Failed to load stats:', error);
        }
    }

    async loadReviewQueue() {
        try {
            const response = await fetch(
                `${API_BASE}/duration/review?page=${this.currentPage}&limit=${this.pageSize}`
            );
            const data = await response.json();

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
        const sourceBadges = sources.map(src => {
            let className = 'no-result';
            let text = src.source_name;

            if (src.error_message) {
                className = 'error';
                text = `${src.source_name} (error)`;
            } else if (src.duration_value > 0) {
                className = src.source_name.toLowerCase().replace(' ', '-');
                const mins = Math.floor(src.duration_value / 60);
                const secs = src.duration_value % 60;
                text = `${src.source_name}: ${mins}:${secs.toString().padStart(2, '0')}`;
            }

            return `<span class="source-badge ${className}">${text}</span>`;
        }).join('');

        return `
            <div class="review-item" data-resolution-id="${item.resolution.id}">
                <div class="track-info">
                    <div class="track-title">${this.escapeHtml(item.track.title)}</div>
                    <div class="track-meta">
                        ${this.escapeHtml(item.album.artist)} - ${this.escapeHtml(item.album.title)}
                    </div>
                </div>
                <div class="sources-summary">
                    ${sourceBadges}
                </div>
                <div class="review-actions-item">
                    <button class="btn btn-primary btn-small review-btn" data-action="apply">Apply</button>
                    <button class="btn btn-secondary btn-small review-btn" data-action="manual">Manual</button>
                    <button class="btn btn-warning btn-small review-btn" data-action="reject">Reject</button>
                </div>
            </div>
        `;
    }

    bindReviewItemEvents() {
        document.querySelectorAll('.review-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const resolutionId = e.target.closest('.review-item').dataset.resolutionId;
                const action = e.target.dataset.action;
                this.openReviewModal(resolutionId, action);
            });
        });
    }

    async openReviewModal(resolutionId, defaultAction) {
        try {
            const response = await fetch(`${API_BASE}/duration/review/${resolutionId}`);
            const data = await response.json();

            if (!data.resolution) {
                alert('Resolution not found');
                return;
            }

            const resolution = data.resolution;
            const sources = data.sources || [];
            const track = data.track;
            const album = data.album;

            document.getElementById('modal-title').textContent = `Review: ${this.escapeHtml(track.title)}`;

            let sourcesHtml = sources.map(src => {
                const formatted = this.formatDuration(src.duration);
                const actionBtn = src.duration_value > 0 && !src.error_message
                    ? `<button class="btn btn-primary btn-small" onclick="reviewManager.submitReview(${resolutionId}, 'apply', ${src.duration_value}, '')">Apply ${formatted}</button>`
                    : '';

                return `
                    <div class="source-detail">
                        <div class="source-header">
                            <span class="source-name">${this.escapeHtml(src.source_name)}</span>
                            <span class="source-duration">${formatted}</span>
                        </div>
                        ${src.error_message
                            ? `<div style="color: #c62828; font-size: 13px;">Error: ${this.escapeHtml(src.error_message)}</div>`
                            : `<div class="source-scores">
                                <span>Match: ${(src.match_score * 100).toFixed(0)}%</span>
                                <span>Confidence: ${(src.confidence * 100).toFixed(0)}%</span>
                               </div>`
                        }
                        ${src.external_url
                            ? `<div style="margin-top: 8px;"><a href="${this.escapeHtml(src.external_url)}" target="_blank" rel="noopener">View on ${this.escapeHtml(src.source_name)}</a></div>`
                            : ''
                        }
                        <div style="margin-top: 8px;">${actionBtn}</div>
                    </div>
                `;
            }).join('');

            document.getElementById('modal-body').innerHTML = `
                <div style="margin-bottom: 16px;">
                    <strong>${this.escapeHtml(album.artist)}</strong> - ${this.escapeHtml(album.title)}
                </div>
                ${sourcesHtml}
                <div class="manual-input">
                    <label>Enter Manual Duration:</label>
                    <input type="number" id="manual-duration" min="1" max="9999" placeholder="Seconds">
                    <span>seconds</span>
                </div>
                <div class="review-notes">
                    <label>Notes (optional):</label>
                    <textarea id="review-notes" placeholder="Add notes about this review..."></textarea>
                </div>
                <div class="modal-actions">
                    <button class="btn btn-secondary" onclick="reviewManager.closeModal()">Cancel</button>
                    <button class="btn btn-warning" onclick="reviewManager.submitReview(${resolutionId}, 'reject', 0, document.getElementById('review-notes').value)">Reject</button>
                    <button class="btn btn-primary" onclick="reviewManager.submitManual(${resolutionId}, document.getElementById('manual-duration').value, document.getElementById('review-notes').value)">Apply Manual</button>
                </div>
            `;

            document.getElementById('review-modal').classList.remove('hidden');
        } catch (error) {
            console.error('Failed to load review details:', error);
            alert('Failed to load review details');
        }
    }

    closeModal() {
        document.getElementById('review-modal').classList.add('hidden');
    }

    async submitReview(resolutionId, action, duration, notes) {
        try {
            const response = await fetch(`${API_BASE}/duration/review/${resolutionId}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    action: action,
                    duration: duration,
                    notes: notes
                })
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || 'Failed to submit review');
            }

            this.closeModal();
            await this.loadStats();
            await this.loadReviewQueue();
            this.showNotification('Review submitted successfully', 'success');
        } catch (error) {
            console.error('Failed to submit review:', error);
            this.showNotification(error.message, 'error');
        }
    }

    async submitManual(resolutionId, duration, notes) {
        if (!duration || duration <= 0) {
            alert('Please enter a valid duration');
            return;
        }
        await this.submitReview(resolutionId, 'manual', parseInt(duration), notes);
    }

    async startBulkResolution() {
        try {
            const response = await fetch(`${API_BASE}/duration/resolve/start`, { method: 'POST' });

            if (!response.ok) {
                throw new Error('Failed to start bulk resolution');
            }

            this.updateBulkButtons('running');
            this.showNotification('Bulk resolution started', 'success');
        } catch (error) {
            console.error('Failed to start bulk resolution:', error);
            this.showNotification(error.message, 'error');
        }
    }

    async pauseBulkResolution() {
        try {
            const response = await fetch(`${API_BASE}/duration/resolve/pause`, { method: 'POST' });

            if (!response.ok) {
                throw new Error('Failed to pause bulk resolution');
            }

            this.updateBulkButtons('paused');
            this.showNotification('Bulk resolution paused', 'success');
        } catch (error) {
            console.error('Failed to pause bulk resolution:', error);
            this.showNotification(error.message, 'error');
        }
    }

    async resumeBulkResolution() {
        try {
            const response = await fetch(`${API_BASE}/duration/resolve/resume`, { method: 'POST' });

            if (!response.ok) {
                throw new Error('Failed to resume bulk resolution');
            }

            this.updateBulkButtons('running');
            this.showNotification('Bulk resolution resumed', 'success');
        } catch (error) {
            console.error('Failed to resume bulk resolution:', error);
            this.showNotification(error.message, 'error');
        }
    }

    async cancelBulkResolution() {
        if (!confirm('Are you sure you want to cancel the bulk resolution?')) {
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/duration/resolve/cancel`, { method: 'POST' });

            if (!response.ok) {
                throw new Error('Failed to cancel bulk resolution');
            }

            this.updateBulkButtons('idle');
            this.showNotification('Bulk resolution cancelled', 'success');
        } catch (error) {
            console.error('Failed to cancel bulk resolution:', error);
            this.showNotification(error.message, 'error');
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
            const response = await fetch(`${API_BASE}/duration/resolve/progress`);
            const progress = await response.json();

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
                    this.showNotification(
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
            const response = await fetch(`${API_BASE}/duration/review`, { method: 'GET' });
            const data = await response.json();

            const items = data.items || [];
            const resolutionIds = items.map(item => item.resolution.id);

            if (resolutionIds.length === 0) {
                this.showNotification('No items to apply', 'info');
                return;
            }

            const applyResponse = await fetch(`${API_BASE}/duration/review/bulk`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    action: 'apply_all',
                    resolution_ids: resolutionIds
                })
            });

            if (!applyResponse.ok) {
                throw new Error('Failed to apply all');
            }

            await this.loadStats();
            await this.loadReviewQueue();
            this.showNotification('Applied all reviews successfully', 'success');
        } catch (error) {
            console.error('Failed to apply all:', error);
            this.showNotification(error.message, 'error');
        }
    }

    async rejectAll() {
        if (!confirm('Reject all pending reviews? This will mark them as reviewed without applying durations.')) {
            return;
        }

        try {
            const response = await fetch(`${API_BASE}/duration/review`, { method: 'GET' });
            const data = await response.json();

            const items = data.items || [];
            const resolutionIds = items.map(item => item.resolution.id);

            if (resolutionIds.length === 0) {
                this.showNotification('No items to reject', 'info');
                return;
            }

            const rejectResponse = await fetch(`${API_BASE}/duration/review/bulk`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    action: 'reject_all',
                    resolution_ids: resolutionIds
                })
            });

            if (!rejectResponse.ok) {
                throw new Error('Failed to reject all');
            }

            await this.loadStats();
            await this.loadReviewQueue();
            this.showNotification('Rejected all reviews', 'success');
        } catch (error) {
            console.error('Failed to reject all:', error);
            this.showNotification(error.message, 'error');
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

    escapeHtml(text) {
        if (!text) return '';
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
            notification.style.animation = 'slideOut 0.3s ease';
            setTimeout(() => notification.remove(), 300);
        }, 4000);
    }
}

let reviewManager;

document.addEventListener('DOMContentLoaded', () => {
    reviewManager = new DurationReviewManager();
});
