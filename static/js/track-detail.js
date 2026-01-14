// Track detail page JavaScript
document.addEventListener('DOMContentLoaded', function() {
    console.log('Track detail page loaded');

    // Get track ID from URL path
    const pathParts = window.location.pathname.split('/');
    const trackId = pathParts[pathParts.length - 1];

    if (!trackId || isNaN(trackId)) {
        document.getElementById('track-detail').innerHTML = '<p>Invalid track ID</p>';
        return;
    }

    // Load track details
    loadTrackDetail(trackId);
});

function loadTrackDetail(trackId) {
    fetch('/tracks/' + trackId)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load track');
            }
            return response.json();
        })
        .then(track => {
            const detail = document.getElementById('track-detail');
            detail.innerHTML = `
                <div class="track-detail-content">
                    <div class="track-header">
                        <h3>${escapeHtml(track.title || 'Unknown Title')}</h3>
                    </div>
                    <div class="track-info-grid">
                        <div class="track-info-item">
                            <strong>Album:</strong>
                            <span>${escapeHtml(track.album_title || 'Unknown Album')}</span>
                        </div>
                        <div class="track-info-item">
                            <strong>Duration:</strong>
                            <span>${formatDuration(track.duration)}</span>
                        </div>
                        <div class="track-info-item">
                            <strong>Genre:</strong>
                            <span>${track.album_genre || track.genre || 'Unknown'}</span>
                        </div>
                        <div class="track-info-item">
                            <strong>Release Year:</strong>
                            <span>${track.release_year || 'Unknown'}</span>
                        </div>
                    </div>
                </div>
            `;
        })
        .catch(error => {
            console.error('Error loading track detail:', error);
            document.getElementById('track-detail').innerHTML = '<p>Error loading track details</p>';
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
