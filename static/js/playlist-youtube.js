// YouTube integration for playlist management
// This file is loaded after playlist.js and uses its shared functions

import { normalizeArtistName } from './modules/utils.js';

function cleanArtistNameYT(artistName) {
    if (!artistName) return 'Unknown Artist';
    return normalizeArtistName(artistName) || 'Unknown Artist';
}

// Review state variables
let reviewTracks = [];
let currentReviewIndex = 0;
let currentReviewPlaylistId = '';

function loadYouTubeMatchStatus(playlistId) {
    fetch(`/api/youtube/matches/${playlistId}`)
        .then(response => {
            if (!response.ok) {
                if (response.status === 404) return null;
                throw new Error('Failed to load YouTube match status');
            }
            return response.json();
        })
        .then(data => {
            if (!data || !data.tracks) {
                document.getElementById('youtube-sync-status').style.display = 'none';
                return;
            }

            const tracks = data.tracks;
            const matched = tracks.filter(t => t.status === 'matched' || t.status === 'reviewed').length;
            const needsReview = tracks.filter(t => t.status === 'needs_review').length;
            const unavailable = tracks.filter(t => t.status === 'unavailable').length;
            const pending = tracks.filter(t => t.status === 'pending').length;

            updateSyncStatusDisplay(matched, needsReview, unavailable, pending);

            if (matched > 0 || needsReview > 0) {
                document.getElementById('sync-youtube-section').style.display = 'block';
            } else {
                document.getElementById('sync-youtube-section').style.display = 'none';
            }

            // Pre-fill playlist name and update sync button state
            if (data.youtube_sync && data.youtube_sync.youtube_playlist_id) {
                // Already synced - update button and pre-fill
                document.getElementById('new-playlist-name-yt').value = data.youtube_sync.youtube_playlist_name || data.playlist_name;
                updateSyncButtonState(true, data.youtube_sync.youtube_playlist_id, data.youtube_sync.youtube_playlist_name || data.playlist_name);
            } else {
                // Not synced - pre-fill with playlist name
                document.getElementById('new-playlist-name-yt').value = data.playlist_name || '';
                updateSyncButtonState(false, null, null);
            }
        })
        .catch(error => {
            console.error('Error loading sync status:', error);
            showNotification('Error loading sync status', 'error');
        });
}

// Alias for backward compatibility (loadYouTubeSyncStatus was being called but never defined)
function loadYouTubeSyncStatus() {
    if (window.currentPlaylistId) {
        loadYouTubeMatchStatus(window.currentPlaylistId);
    }
}

function openYouTubeSyncModal() {
    if (!window.currentPlaylistId) {
        showNotification('No playlist selected', 'error');
        return;
    }

    document.getElementById('youtube-sync-modal').style.display = 'flex';
    loadYouTubeSyncStatus();
}

function updateSyncButtonState(isSynced, youtubePlaylistId, youtubePlaylistName) {
    const syncBtn = document.getElementById('sync-youtube-btn');
    if (!syncBtn) return;
    
    if (isSynced) {
        syncBtn.textContent = 'Synced';
        syncBtn.style.backgroundColor = '#28a745';
        syncBtn.onclick = function() {
            if (youtubePlaylistId) {
                loadYouTubePlaylist(youtubePlaylistId, youtubePlaylistName);
            }
        };
    } else {
        syncBtn.textContent = 'YouTube Sync';
        syncBtn.style.backgroundColor = '#ff0000';
        syncBtn.onclick = function() {
            openYouTubeSyncModal();
        };
    }
}

function loadYouTubePlaylist(playlistId, playlistName) {
    if (playlistName) {
        window.location.href = `/youtube?playlist_id=${playlistId}&playlist_title=${encodeURIComponent(playlistName)}`;
    } else {
        window.location.href = `/youtube?playlist_id=${playlistId}`;
    }
}

function updateSyncStatusDisplay(matched, needsReview, unavailable, pending) {
    document.getElementById('yt-matched').textContent = matched;
    document.getElementById('yt-review').textContent = needsReview;
    document.getElementById('yt-unavailable').textContent = unavailable;
    document.getElementById('yt-pending').textContent = pending;
}

function matchPlaylistTracks() {
    const playlistId = window.currentPlaylistId;
    const includeReview = document.getElementById('include-needs-review').checked;
    const youtubeApiFallback = document.getElementById('youtube-api-fallback').checked;
    const forceRescan = document.getElementById('force-rescan') ? document.getElementById('force-rescan').checked : false;

    document.getElementById('match-playlist-btn').disabled = true;
    document.getElementById('sync-progress').style.display = 'block';
    document.getElementById('match-progress-fill').style.width = '0%';
    document.getElementById('match-progress-text').textContent = forceRescan ? 'Force re-scanning...' : 'Starting match...';
    document.getElementById('youtube-status-msg').textContent = '';

    const url = forceRescan
        ? `/api/youtube/match-playlist/${playlistId}?force=true`
        : `/api/youtube/match-playlist/${playlistId}`;

    fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            include_review: includeReview,
            youtube_api_fallback: youtubeApiFallback
        })
    })
    .then(response => {
        if (!response.ok) {
            return response.text().then(text => {
                throw new Error(text || 'Failed to match tracks');
            });
        }
        return response.json();
    })
    .then(data => {
        document.getElementById('match-progress-fill').style.width = '100%';
        document.getElementById('match-progress-text').textContent = 'Match complete!';

        setTimeout(() => {
            document.getElementById('sync-progress').style.display = 'none';
            document.getElementById('match-playlist-btn').disabled = false;
        }, 1000);

        loadYouTubeSyncStatus();
        loadYouTubeMatchStatus(playlistId);

        const synced = data.matched || 0;
        const review = data.needs_review || 0;
        const unavailable = data.unavailable || 0;

        showNotification(`Matched: ${synced}, Needs Review: ${review}, Unavailable: ${unavailable}`, 'success');

        if (review > 0) {
            document.getElementById('youtube-status-msg').innerHTML =
                `<p class="review-notice">${review} track(s) need review. <button id="review-tracks-btn" class="youtube-action-btn" style="margin-left: 0.5rem;">Review Now</button></p>`;
            document.getElementById('review-tracks-btn').addEventListener('click', () => {
                openReviewModal(playlistId);
            });
        }
    })
    .catch(error => {
        console.error('Error matching tracks:', error);
        document.getElementById('match-playlist-btn').disabled = false;
        document.getElementById('youtube-status-msg').textContent = 'Error: ' + error.message;
        showNotification('Error matching tracks: ' + error.message, 'error');
    });
}

function syncToYouTube() {
    const playlistId = window.currentPlaylistId;
    const exportOption = document.querySelector('input[name="youtube-export"]:checked').value;
    let playlistName = document.getElementById('new-playlist-name-yt').value.trim();
    let youtubePlaylistId = '';

    if (exportOption === 'new') {
        if (!playlistName) {
            playlistName = playlistId;
        }
    } else {
        youtubePlaylistId = document.getElementById('existing-playlist-id').value.trim();
        if (!youtubePlaylistId) {
            showNotification('Please enter a YouTube Playlist ID', 'error');
            return;
        }
    }

    const includeReview = document.getElementById('include-needs-review').checked;

    document.getElementById('sync-to-youtube-btn').disabled = true;
    document.getElementById('youtube-status-msg').textContent = 'Syncing to YouTube...';

    const body = {
        include_needs_review: includeReview
    };

    if (exportOption === 'new') {
        body.playlist_name = playlistName;
    } else {
        body.youtube_playlist_id = youtubePlaylistId;
    }

    fetch(`/api/youtube/sync-playlist/${playlistId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    })
    .then(response => {
        if (!response.ok) {
            return response.text().then(text => {
                throw new Error(text || 'Failed to sync to YouTube');
            });
        }
        return response.json();
    })
    .then(data => {
        document.getElementById('sync-to-youtube-btn').disabled = false;
        document.getElementById('youtube-status-msg').textContent =
            `Synced ${data.synced_count} tracks to YouTube. Skipped: ${data.skipped_count}`;

        if (data.youtube_playlist_url) {
            document.getElementById('youtube-status-msg').innerHTML +=
                ` <a href="${data.youtube_playlist_url}" target="_blank">View on YouTube</a>`;
        }

        showNotification(`Synced ${data.synced_count} tracks to YouTube`, 'success');

        // Reload sync status to update button state
        loadYouTubeSyncStatus();
    })
    .catch(error => {
        console.error('Error syncing to YouTube:', error);
        document.getElementById('sync-to-youtube-btn').disabled = false;
        document.getElementById('youtube-status-msg').textContent = 'Error: ' + error.message;
        showNotification('Error syncing to YouTube: ' + error.message, 'error');
    });
}

function clearWebCache() {
    if (!confirm('Clear the YouTube web search cache? This will force fresh searches for all tracks.')) {
        return;
    }

    fetch('/api/youtube/clear-cache', { method: 'POST' })
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to clear cache');
            }
            return response.json();
        })
        .then(data => {
            showNotification('Web cache cleared successfully', 'success');
        })
        .catch(error => {
            console.error('Error clearing cache:', error);
            showNotification('Error clearing cache: ' + error.message, 'error');
        });
}

function openReviewModal(playlistId) {
    fetch(`/api/youtube/matches/${playlistId}`)
        .then(response => {
            if (!response.ok) throw new Error('Failed to load matches');
            return response.json();
        })
        .then(data => {
            const needsReview = data.tracks.filter(t => t.status === 'needs_review');

            if (needsReview.length === 0) {
                showNotification('No tracks need review', 'info');
                return;
            }

            document.getElementById('youtube-review-modal').style.display = 'flex';
            renderReviewTrack(needsReview, 0, playlistId);
        })
        .catch(error => {
            console.error('Error loading review tracks:', error);
            showNotification('Error loading review tracks', 'error');
        });
}

function renderReviewTrack(tracks, index, playlistId) {
    reviewTracks = tracks;
    currentReviewIndex = index;
    currentReviewPlaylistId = playlistId;

    if (index >= tracks.length) {
        showNotification('All reviews complete!', 'success');
        document.getElementById('youtube-review-modal').style.display = 'none';
        loadYouTubeSyncStatus();
        return;
    }

    const track = tracks[index];
    const trackInfo = document.getElementById('review-track-info');
    trackInfo.innerHTML = `
        <p><strong>Track ${index + 1} of ${tracks.length}</strong></p>
        <p><strong>Page:</strong> ${escapeHtml(track.album_title)}</p>
        <p>${escapeHtml(track.track_title)} - ${escapeHtml(cleanArtistNameYT(track.artist))}</p>
        <p>Duration: ${formatDuration(track.duration)}</p>
    `;

    fetch(`/api/youtube/candidates/${track.track_id}`)
        .then(response => {
            if (!response.ok) throw new Error('Failed to load candidates');
            return response.json();
        })
        .then(data => {
            renderCandidates(data.candidates || [], track.track_id);
        })
        .catch(error => {
            console.error('Error loading candidates:', error);
            document.getElementById('review-candidates').innerHTML = '<p>Error loading candidates</p>';
        });
}

function renderCandidates(candidates, trackId) {
    const container = document.getElementById('review-candidates');

    if (!candidates || candidates.length === 0) {
        container.innerHTML = '<p>No candidates found.</p>';
        return;
    }

    container.innerHTML = '';

    candidates.forEach(candidate => {
        const div = document.createElement('div');
        div.className = 'candidate-item';

        const thumbnailUrl = candidate.thumbnail_url ?
            `<img src="${escapeHtml(candidate.thumbnail_url)}" alt="" class="candidate-thumb" onerror="this.style.display='none'">` :
            '<div class="candidate-thumb-placeholder">▶</div>';

        div.innerHTML = `
            ${thumbnailUrl}
            <div class="candidate-info">
                <p class="candidate-title">${escapeHtml(candidate.title)}</p>
                <p class="candidate-channel">${escapeHtml(candidate.channel_name)}</p>
            </div>
            <img src="/icons/yt_icon_red_digital.png" alt="YouTube" class="youtube-candidate-logo">
            <button class="select-candidate-btn" data-candidate-id="${candidate.id}">Select</button>
        `;

        div.querySelector('.select-candidate-btn').addEventListener('click', () => {
            selectCandidate(trackId, candidate.id);
        });

        container.appendChild(div);
    });
}

function selectCandidate(trackId, candidateId) {
    fetch(`/api/youtube/candidates/${trackId}/select/${candidateId}`, {
        method: 'POST'
    })
    .then(response => {
        if (!response.ok) throw new Error('Failed to select candidate');
        return response.json();
    })
    .then(() => {
        showNotification('Candidate selected!', 'success');
        renderReviewTrack(reviewTracks, currentReviewIndex + 1, currentReviewPlaylistId);
    })
    .catch(error => {
        console.error('Error selecting candidate:', error);
        showNotification('Error selecting candidate', 'error');
    });
}

function closeYouTubeSyncModal() {
    document.getElementById('youtube-sync-modal').style.display = 'none';
}

function closeYouTubeReviewModal() {
    document.getElementById('youtube-review-modal').style.display = 'none';
}

// Expose functions globally for non-module scripts
window.openYouTubeSyncModal = openYouTubeSyncModal;
window.closeYouTubeSyncModal = closeYouTubeSyncModal;
window.closeYouTubeReviewModal = closeYouTubeReviewModal;
window.matchPlaylistTracks = matchPlaylistTracks;
window.syncToYouTube = syncToYouTube;
window.clearWebCache = clearWebCache;
window.loadYouTubeSyncStatus = loadYouTubeSyncStatus;
window.loadYouTubePlaylist = loadYouTubePlaylist;
