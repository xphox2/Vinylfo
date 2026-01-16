// Album detail page JavaScript
document.addEventListener('DOMContentLoaded', function() {
    console.log('Album detail page loaded');

    // Get album ID from URL path
    const pathParts = window.location.pathname.split('/');
    const albumId = pathParts[pathParts.length - 1];

    if (!albumId || isNaN(albumId)) {
        document.getElementById('album-detail').innerHTML = '<p>Invalid album ID</p>';
        return;
    }

    // Load album details
    loadAlbumDetail(albumId);
});

function loadAlbumDetail(albumId) {
    // Load album info and tracks in parallel
    Promise.all([
        fetch('/albums/' + albumId).then(response => {
            if (!response.ok) throw new Error('Failed to load album');
            return response.json();
        }),
        fetch('/albums/' + albumId + '/tracks').then(response => {
            if (!response.ok) throw new Error('Failed to load tracks');
            return response.json();
        })
    ])
    .then(([album, tracks]) => {
        const detail = document.getElementById('album-detail');
        
        let coverHtml = '<div class="album-detail-cover-placeholder">No Cover</div>';
        if (album.cover_image_url || album.discogs_cover_image_type || album.cover_image_failed) {
            coverHtml = `<img src="/albums/${album.id}/image" alt="${album.title}" class="album-detail-cover" onerror="this.style.display='none';this.parentElement.innerHTML='<div class=\\'album-detail-cover-placeholder\\'>No Cover</div>';">`;
        }
        
        const tracksHtml = tracks && tracks.length > 0 ? `
            <div class="album-tracks">
                <h3>Tracks</h3>
                <div class="tracks-list">
                    ${tracks.map(track => `
                        <div class="track-item" onclick="window.location.href='/track/${track.id}'">
                            <div class="track-number">${track.track_number || ''}</div>
                            <div class="track-title">${escapeHtml(track.title || 'Unknown Title')}</div>
                            <div class="track-duration">${formatDuration(track.duration)}</div>
                        </div>
                    `).join('')}
                </div>
            </div>
        ` : '<p>No tracks found for this album.</p>';
        
        detail.innerHTML = `
            <div class="album-detail-content">
                <div class="album-detail-cover-container">
                    ${coverHtml}
                </div>
                <div class="album-detail-info">
                    <h2>${escapeHtml(album.title || 'Unknown Title')}</h2>
                    <div class="album-detail-info-grid">
                        <div class="album-detail-info-item">
                            <strong>Artist:</strong>
                            <span>${escapeHtml(album.artist || 'Unknown Artist')}</span>
                        </div>
                        <div class="album-detail-info-item">
                            <strong>Release Year:</strong>
                            <span>${album.release_year || 'Unknown'}</span>
                        </div>
                        <div class="album-detail-info-item">
                            <strong>Genre:</strong>
                            <span>${escapeHtml(album.genre || 'Unknown')}</span>
                        </div>
                        <div class="album-detail-info-item">
                            <strong>Label:</strong>
                            <span>${escapeHtml(album.label || 'Unknown')}</span>
                        </div>
                        <div class="album-detail-info-item">
                            <strong>Country:</strong>
                            <span>${escapeHtml(album.country || 'Unknown')}</span>
                        </div>
                    </div>
                </div>
            </div>
            ${tracksHtml}
        `;
    })
    .catch(error => {
        console.error('Error loading album detail:', error);
        document.getElementById('album-detail').innerHTML = '<p>Error loading album details</p>';
    });
}

function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return '0:00';

    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes}:${secs < 10 ? '0' : ''}${secs}`;
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
