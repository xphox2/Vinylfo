const API_BASE = '/api';

class SearchManager {
    constructor() {
        this.currentPage = 1;
        this.init();
    }

    init() {
        this.bindEvents();
    }

    bindEvents() {
        document.getElementById('discogs-search-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.searchDiscogs();
        });

        document.getElementById('tab-discogs').addEventListener('click', () => this.showDiscogsTab());
        document.getElementById('tab-local').addEventListener('click', () => this.showLocalTab());

        document.getElementById('manual-album-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.saveLocalAlbum();
        });

        document.getElementById('add-track').addEventListener('click', () => this.addTrackField());

        document.getElementById('modal-close').addEventListener('click', () => this.closeModal());
        document.getElementById('modal-cancel').addEventListener('click', () => this.closeModal());
        document.getElementById('modal-confirm').addEventListener('click', () => this.confirmAddAlbum());
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

        this.showLoading(true);

        try {
            const response = await fetch(`${API_BASE}/discogs/search?q=${encodeURIComponent(query)}&page=${this.currentPage}`);
            const data = await response.json();

            this.renderResults(data.results);
        } catch (error) {
            console.error('Search failed:', error);
            this.showNotification('Search failed', 'error');
        } finally {
            this.showLoading(false);
        }
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
                    <span class="result-title">${this.escapeHtml(album.title)}</span>
                    <span class="result-artist">${this.escapeHtml(album.artist)}</span>
                    <span class="result-year">${album.year || 'Unknown year'}</span>
                </div>
                <button class="btn btn-primary btn-sm add-album-btn" data-id="${album.discogs_id}">Add</button>
            `;
            container.appendChild(item);
        });

        container.querySelectorAll('.add-album-btn').forEach(btn => {
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
            const response = await fetch(`${API_BASE}/discogs/albums`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    discogs_id: parseInt(discogsId),
                    from_discogs: true
                })
            });

            if (!response.ok) {
                throw new Error('Failed to fetch album details');
            }

            const album = await response.json();

            this.currentAlbumData = {
                discogs_id: discogsId,
                title: album.title,
                artist: album.artist,
                release_year: album.release_year,
                genre: album.genre,
                cover_image: album.cover_image_url,
                tracks: album.Tracks || []
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

        let tracksHtml = '';
        if (album.Tracks && album.Tracks.length > 0) {
            tracksHtml = '<ul class="track-list">';
            album.Tracks.forEach((track, i) => {
                tracksHtml += `<li>${i + 1}. ${this.escapeHtml(track.title)}</li>`;
            });
            tracksHtml += '</ul>';
        }

        document.getElementById('modal-body').innerHTML = `
            <div class="modal-album-info">
                <img src="${album.cover_image_url || '/static/images/no-cover.png'}" alt="${this.escapeHtml(album.title)}" class="modal-cover">
                <div class="modal-details">
                    <p><strong>Artist:</strong> ${this.escapeHtml(album.artist)}</p>
                    <p><strong>Year:</strong> ${album.release_year || 'Unknown'}</p>
                    <p><strong>Genre:</strong> ${album.genre || 'Unknown'}</p>
                </div>
            </div>
            <div class="modal-tracks">
                <h4>Tracks</h4>
                ${tracksHtml || '<p>No track information available</p>'}
            </div>
        `;
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
