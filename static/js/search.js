const API_BASE = '/api';

import { normalizeArtistName, normalizeTitle } from './modules/utils.js';

class SearchManager {
    constructor() {
        this.currentPage = 1;
        this.totalPages = 1;
        this.currentQuery = '';
        this.init();
    }

    init() {
        this.bindEvents();
    }

    bindEvents() {
        document.getElementById('discogs-search-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.currentPage = 1;
            this.searchDiscogs();
        });

        document.getElementById('tab-discogs').addEventListener('click', () => this.showDiscogsTab());
        document.getElementById('tab-local').addEventListener('click', () => this.showLocalTab());

        document.getElementById('manual-album-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.saveLocalAlbum();
        });

        document.getElementById('add-track').addEventListener('click', () => this.addTrackField());

        document.addEventListener('click', (e) => {
            if (e.target && (e.target.id === 'modal-close' || e.target.classList.contains('modal-close'))) {
                this.closeModal();
            } else if (e.target && e.target.id === 'modal-cancel') {
                this.closeModal();
            } else if (e.target && e.target.id === 'modal-confirm') {
                this.confirmAddAlbum();
            }
        });
    }

    showDiscogsTab() {
        document.getElementById('tab-discogs').classList.add('active');
        document.getElementById('tab-local').classList.remove('active');
        document.getElementById('discogs-results').classList.remove('hidden');
        document.getElementById('local-form').classList.add('hidden');
    }

    showLocalTab() {
        document.getElementById('tab-discogs').classList.remove('active');
        document.getElementById('tab-local').classList.add('active');
        document.getElementById('discogs-results').classList.add('hidden');
        document.getElementById('local-form').classList.remove('hidden');
    }

    async searchDiscogs() {
        const query = document.getElementById('search-query').value.trim();
        if (!query) return;

        this.currentQuery = query;
        this.showLoading(true);

        try {
            const response = await fetch(`${API_BASE}/discogs/search?q=${encodeURIComponent(query)}&page=${this.currentPage}`);

            if (response.status === 401) {
                const error = await response.json();
                this.showNotification(error.hint || 'Please connect your Discogs account in Settings', 'warning');
                this.renderNoConnection();
                return;
            }

            if (!response.ok) {
                const error = await response.json();
                this.showNotification(error.error || 'Search failed', 'error');
                return;
            }

            const data = await response.json();
            this.totalPages = data.totalPages || 1;
            this.renderResults(data.results);
            this.renderPagination();
        } catch (error) {
            console.error('Search failed:', error);
            this.showNotification('Search failed', 'error');
        } finally {
            this.showLoading(false);
        }
    }

    renderNoConnection() {
        const container = document.getElementById('results-list');
        container.innerHTML = '';
        document.getElementById('results-empty').classList.add('hidden');
        container.innerHTML = `
            <div class="no-connection">
                <p class="error">Discogs connection required</p>
                <p class="hint">Go to Settings to connect your Discogs account</p>
            </div>
        `;
    }

    renderPagination() {
        const container = document.getElementById('pagination');
        container.innerHTML = '';

        if (this.totalPages <= 1) {
            container.classList.add('hidden');
            return;
        }

        container.classList.remove('hidden');

        let html = '<div class="pagination-controls">';

        // Previous button
        if (this.currentPage > 1) {
            html += `<button class="btn btn-sm pagination-btn" data-page="${this.currentPage - 1}">Previous</button>`;
        }

        // Page numbers
        const startPage = Math.max(1, this.currentPage - 2);
        const endPage = Math.min(this.totalPages, this.currentPage + 2);

        if (startPage > 1) {
            html += `<button class="btn btn-sm pagination-btn" data-page="1">1</button>`;
            if (startPage > 2) {
                html += `<span class="pagination-ellipsis">...</span>`;
            }
        }

        for (let i = startPage; i <= endPage; i++) {
            if (i === this.currentPage) {
                html += `<span class="pagination-current">${i}</span>`;
            } else {
                html += `<button class="btn btn-sm pagination-btn" data-page="${i}">${i}</button>`;
            }
        }

        if (endPage < this.totalPages) {
            if (endPage < this.totalPages - 1) {
                html += `<span class="pagination-ellipsis">...</span>`;
            }
            html += `<button class="btn btn-sm pagination-btn" data-page="${this.totalPages}">${this.totalPages}</button>`;
        }

        // Next button
        if (this.currentPage < this.totalPages) {
            html += `<button class="btn btn-sm pagination-btn" data-page="${this.currentPage + 1}">Next</button>`;
        }

        html += '</div>';
        html += `<p class="pagination-info">Page ${this.currentPage} of ${this.totalPages}</p>`;

        container.innerHTML = html;

        // Bind click events
        container.querySelectorAll('.pagination-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const page = parseInt(e.target.dataset.page);
                if (page !== this.currentPage) {
                    this.currentPage = page;
                    this.searchDiscogs();
                    window.scrollTo({ top: 0, behavior: 'smooth' });
                }
            });
        });
    }

    renderResults(results) {
        const container = document.getElementById('results-list');
        container.innerHTML = '';

        if (!results || results.length === 0) {
            document.getElementById('results-empty').classList.remove('hidden');
            return;
        }

        document.getElementById('results-empty').classList.add('hidden');

        results.forEach(album => {
            const item = document.createElement('div');
            item.className = 'result-item';
            item.innerHTML = `
                <div class="result-cover">
                    ${album.cover_image
                        ? `<img src="${this.escapeHtml(album.cover_image)}" alt="${this.escapeHtml(album.title)}">`
                        : '<div class="no-cover">No Cover</div>'}
                </div>
                <div class="result-info">
                    <span class="result-title">${this.escapeHtml(this.cleanTitle(album.title))}</span>
                    <span class="result-artist">${this.escapeHtml(this.cleanArtistName(album.artist))}</span>
                    <span class="result-year">${album.year || 'Unknown year'}</span>
                </div>
                <button class="btn btn-primary btn-sm view-album-btn" data-id="${album.discogs_id}">View</button>
            `;
            container.appendChild(item);
        });

        container.querySelectorAll('.view-album-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const id = e.target.dataset.id;
                this.showAlbumDetail(id);
            });
        });
    }

    showLoading(show) {
        document.getElementById('results-loading').classList.toggle('hidden', !show);
        document.getElementById('results-list').classList.toggle('hidden', show);
    }

    async showAlbumDetail(discogsId) {
        try {
            const response = await fetch(`${API_BASE}/discogs/albums/${discogsId}`);

            if (!response.ok) {
                const error = await response.json();
                this.showNotification(error.error || 'Failed to load album details', 'error');
                return;
            }

            const album = await response.json();

            this.currentAlbumData = {
                discogs_id: parseInt(discogsId),
                title: album.title,
                artist: album.artist,
                release_year: album.year,
                genre: album.genre,
                label: album.label || '',
                country: album.country || '',
                release_date: album.release_date || '',
                style: album.style || '',
                cover_image: album.cover_image,
                from_discogs: true,
                tracks: album.tracklist || []
            };

            this.renderModal(album);
            document.getElementById('album-detail-modal').classList.remove('hidden');
        } catch (error) {
            console.error('Failed to fetch album details:', error);
            this.showNotification('Failed to load album details', 'error');
        }
    }

    renderModal(album) {
        document.getElementById('modal-title').textContent = album.title;

        const tracks = album.tracklist || [];
        let tracksHtml = '';
        if (tracks.length > 0) {
            tracksHtml = '<ul class="track-list">';
            tracks.forEach((track) => {
                const side = track.side || track.position || '';
                const duration = this.formatDuration(track.duration);
                tracksHtml += `<li><span class="track-position">${side}</span> ${this.escapeHtml(this.cleanTrackTitle(track.title))} ${duration ? `<span class="track-duration">(${duration})</span>` : ''}</li>`;
            });
            tracksHtml += '</ul>';
        }

        document.getElementById('modal-body').innerHTML = `
            <div class="modal-album-info">
                <img src="${album.cover_image || '/static/images/no-cover.png'}" alt="${this.escapeHtml(album.title)}" class="modal-cover">
                <div class="modal-details">
                    <p><strong>Artist:</strong> ${this.escapeHtml(this.cleanArtistName(album.artist))}</p>
                    <p><strong>Year:</strong> ${album.year || 'Unknown'}</p>
                    <p><strong>Genre:</strong> ${album.genre || 'Unknown'}</p>
                    ${album.label ? `<p><strong>Label:</strong> ${this.escapeHtml(album.label)}</p>` : ''}
                    ${album.country ? `<p><strong>Country:</strong> ${this.escapeHtml(album.country)}</p>` : ''}
                    ${album.release_date ? `<p><strong>Released:</strong> ${this.escapeHtml(album.release_date)}</p>` : ''}
                    ${album.style ? `<p><strong>Style:</strong> ${this.escapeHtml(album.style)}</p>` : ''}
                </div>
            </div>
            <div class="modal-tracks">
                <h4>Tracks (${tracks.length})</h4>
                ${tracksHtml || '<p>No track information available</p>'}
            </div>
        `;
    }

    formatDuration(seconds) {
        if (!seconds || seconds === 0) return '';
        const mins = Math.floor(seconds / 60);
        const secs = seconds % 60;
        return `${mins}:${secs.toString().padStart(2, '0')}`;
    }

    cleanArtistName(artistName) {
        if (!artistName) return 'Unknown Artist';
        return normalizeArtistName(artistName) || 'Unknown Artist';
    }

    cleanTrackTitle(trackTitle) {
        if (!trackTitle) return 'Unknown Track';
        return normalizeTitle(trackTitle) || 'Unknown Track';
    }

    closeModal() {
        document.getElementById('album-detail-modal').classList.add('hidden');
        this.currentAlbumData = null;
    }

    async confirmAddAlbum() {
        if (!this.currentAlbumData) return;

        try {
            const response = await fetch(`${API_BASE}/discogs/albums`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(this.currentAlbumData)
            });

            if (response.ok) {
                this.showNotification('Album added to collection!', 'success');
                this.closeModal();
            } else {
                const error = await response.json();
                this.showNotification(error.error || 'Failed to add album', 'error');
            }
        } catch (error) {
            console.error('Failed to add album:', error);
            this.showNotification('Failed to add album', 'error');
        }
    }

    addTrackField() {
        const container = document.getElementById('manual-tracks');
        const trackIndex = container.children.length;

        const trackRow = document.createElement('div');
        trackRow.className = 'track-row';
        trackRow.innerHTML = `
            <input type="text" name="track_title_${trackIndex}" placeholder="Track title" class="track-title-input">
            <input type="number" name="track_number_${trackIndex}" placeholder="#" class="track-number-input" style="width: 60px;">
            <button type="button" class="btn btn-danger btn-sm remove-track">&times;</button>
        `;

        trackRow.querySelector('.remove-track').addEventListener('click', () => {
            trackRow.remove();
        });

        container.appendChild(trackRow);
    }

    async saveLocalAlbum() {
        const title = document.getElementById('album-title').value.trim();
        const artist = document.getElementById('album-artist').value.trim();
        const releaseYear = parseInt(document.getElementById('album-year').value) || 0;
        const genre = document.getElementById('album-genre').value.trim();
        const coverImage = document.getElementById('album-cover').value.trim();

        const tracks = [];
        const trackRows = document.querySelectorAll('#manual-tracks .track-row');
        trackRows.forEach(row => {
            const trackTitle = row.querySelector('.track-title-input').value.trim();
            const trackNumber = parseInt(row.querySelector('.track-number-input').value) || 0;
            if (trackTitle) {
                tracks.push({
                    title: trackTitle,
                    track_number: trackNumber
                });
            }
        });

        try {
            const response = await fetch(`${API_BASE}/albums`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    title,
                    artist,
                    release_year: releaseYear,
                    genre,
                    cover_image_url: coverImage,
                    tracks
                })
            });

            if (response.ok) {
                this.showNotification('Album added to collection!', 'success');
                this.resetLocalForm();
            } else {
                const error = await response.json();
                this.showNotification(error.error || 'Failed to add album', 'error');
            }
        } catch (error) {
            console.error('Failed to save album:', error);
            this.showNotification('Failed to save album', 'error');
        }
    }

    resetLocalForm() {
        document.getElementById('manual-album-form').reset();
        document.getElementById('manual-tracks').innerHTML = '';
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    cleanTitle(text) {
        if (!text) return '';
        return text.replace(/\s*\(\d+\)\s*/g, ' ').replace(/\s+/g, ' ').trim();
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
    new SearchManager();
});
