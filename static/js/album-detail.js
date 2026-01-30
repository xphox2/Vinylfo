// Album detail page JavaScript

import { normalizeArtistName, normalizeTitle } from './modules/utils.js';

function cleanArtistName(artistName) {
    if (!artistName) return 'Unknown Artist';
    return normalizeArtistName(artistName) || 'Unknown Artist';
}

function cleanTrackTitle(trackTitle) {
    if (!trackTitle) return 'Unknown Track';
    return normalizeTitle(trackTitle) || 'Unknown Track';
}

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
                            <div class="track-title">${escapeHtml(cleanTrackTitle(track.title) || 'Unknown Title')}</div>
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
                    <button class="btn-update-cover" id="btn-update-cover">Change Cover</button>
                    <div id="cover-update-form" class="cover-update-form" style="display: none;">
                        <input type="text" id="cover-url-input" placeholder="Enter image URL" class="cover-url-input">
                        <div class="cover-update-buttons">
                            <button class="btn-save-cover" id="btn-save-cover">Update</button>
                            <button class="btn-cancel-cover" id="btn-cancel-cover">Cancel</button>
                        </div>
                        <div id="cover-update-error" class="cover-update-error"></div>
                    </div>
                </div>
                <div class="album-detail-info">
                    <h2>${escapeHtml(album.title || 'Unknown Title')}</h2>
                    <div class="album-detail-info-grid">
                        <div class="album-detail-info-item">
                            <strong>Artist:</strong>
                            <span>${escapeHtml(cleanArtistName(album.artist))}</span>
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
        
        // Attach event listeners for cover update UI
        attachCoverUpdateListeners(album.id);
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

// Cover image update functions
function attachCoverUpdateListeners(albumId) {
    const updateBtn = document.getElementById('btn-update-cover');
    const saveBtn = document.getElementById('btn-save-cover');
    const cancelBtn = document.getElementById('btn-cancel-cover');
    
    if (updateBtn) {
        updateBtn.addEventListener('click', () => showUpdateCoverForm(albumId));
    }
    if (saveBtn) {
        saveBtn.addEventListener('click', () => updateAlbumCover(albumId));
    }
    if (cancelBtn) {
        cancelBtn.addEventListener('click', hideUpdateCoverForm);
    }
}

function showUpdateCoverForm(albumId) {
    const form = document.getElementById('cover-update-form');
    const errorDiv = document.getElementById('cover-update-error');
    const input = document.getElementById('cover-url-input');
    if (form) {
        form.style.display = 'block';
        input.value = '';
        input.focus();
        if (errorDiv) errorDiv.textContent = '';
    }
}

function hideUpdateCoverForm() {
    const form = document.getElementById('cover-update-form');
    const errorDiv = document.getElementById('cover-update-error');
    if (form) {
        form.style.display = 'none';
    }
    if (errorDiv) {
        errorDiv.textContent = '';
    }
}

function updateAlbumCover(albumId) {
    const input = document.getElementById('cover-url-input');
    const errorDiv = document.getElementById('cover-update-error');
    const imageUrl = input.value.trim();
    
    if (!imageUrl) {
        if (errorDiv) errorDiv.textContent = 'Please enter an image URL';
        return;
    }
    
    // Show loading state
    const saveButton = document.querySelector('.btn-save-cover');
    const originalText = saveButton ? saveButton.textContent : 'Update';
    if (saveButton) {
        saveButton.textContent = 'Updating...';
        saveButton.disabled = true;
    }
    if (errorDiv) errorDiv.textContent = '';
    
    fetch('/albums/' + albumId + '/image', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({ image_url: imageUrl })
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(data => {
                throw new Error(data.error || 'Failed to update cover image');
            });
        }
        return response.json();
    })
    .then(data => {
        // Success - reload the album detail to show new image
        hideUpdateCoverForm();
        loadAlbumDetail(albumId);
    })
    .catch(error => {
        console.error('Error updating cover:', error);
        if (errorDiv) errorDiv.textContent = error.message;
        if (saveButton) {
            saveButton.textContent = originalText;
            saveButton.disabled = false;
        }
    });
}
