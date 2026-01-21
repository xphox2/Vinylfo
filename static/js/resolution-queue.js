import { api, durationAPI } from './modules/api.js';
import { escapeHtml, formatDuration, showNotification, normalizeArtistName, normalizeTitle } from './modules/utils.js';

export class ResolutionQueueManager {
    constructor(manager) {
        this.manager = manager;
    }

    async loadReviewQueue() {
        try {
            const data = await durationAPI.getReviewQueue(this.manager.currentPage, this.manager.pageSize);

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
        this.bindReviewItemEvents();
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
                this.manager.openReviewModal(resolutionId, action);
            };
        });
    }

    async loadUnprocessedQueue() {
        try {
            const data = await durationAPI.getUnprocessed(this.manager.unprocessedPage, this.manager.pageSize);

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
                this.manager.openUnprocessedManualModal(trackId, title);
            });
        });
    }

    async loadResolvedQueue() {
        try {
            const data = await durationAPI.getResolved(this.manager.resolvedPage, this.manager.pageSize);

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
        const sources = item.sources || [];
        const sourceBadges = sources.map(src => {
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
