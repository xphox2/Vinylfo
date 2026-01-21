// Playlist management JavaScript

window.currentPlaylistId = null;
let playlists = [];
let tracks = [];

// Available tracks pagination state
let availableTrackPagination = {
    page: 1,
    limit: 25,
    query: '',
    totalPages: 1,
    total: 0
};

// Load saved playlist ID from localStorage
const savedId = localStorage.getItem('vinylfo_currentPlaylistId');
if (savedId) {
    console.log('Loaded saved playlist ID:', savedId);
    window.currentPlaylistId = savedId;
}

function savePlaylistId(id) {
    localStorage.setItem('vinylfo_currentPlaylistId', id);
    window.currentPlaylistId = id;
    console.log('Saved playlist ID:', id);
}

function clearPlaylistId() {
    localStorage.removeItem('vinylfo_currentPlaylistId');
    window.currentPlaylistId = null;
    console.log('Cleared playlist ID');
}

function cleanAlbumTitle(albumTitle, trackTitle) {
    if (!albumTitle) return 'Unknown Album';
    
    if (albumTitle.includes(' / ') && albumTitle.includes(trackTitle)) {
        const parts = albumTitle.split(' / ');
        return parts[parts.length - 1].trim();
    }
    
    return albumTitle;
}

function loadPlaylists() {
    fetch('/sessions/playlist')
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load playlists');
            }
            return response.json();
        })
        .then(data => {
            playlists = data || [];
            renderPlaylists();
        })
        .catch(error => {
            console.error('Error loading playlists:', error);
            document.getElementById('playlists-container').innerHTML =
                '<p class="empty-message">Error loading playlists. Please try again.</p>';
        });
}

function renderPlaylists() {
    const container = document.getElementById('playlists-container');
    
    if (!playlists || playlists.length === 0) {
        container.innerHTML = '<p class="empty-message">No playlists yet. Create one to get started!</p>';
        return;
    }
    
    container.innerHTML = '';
    
    playlists.forEach(playlist => {
        const card = document.createElement('div');
        card.className = 'playlist-card';
        card.innerHTML = `
            <h3>${escapeHtml(playlist.session_id || 'Untitled Playlist')}</h3>
            <p>Created: ${formatDate(playlist.created_at)}</p>
            <p class="track-count">Loading tracks...</p>
        `;
        
        card.addEventListener('click', function() {
            console.log('Playlist card clicked, session_id:', playlist.session_id);
            showPlaylistDetail(playlist.session_id);
        });
        
        container.appendChild(card);
        
        loadPlaylistTracks(playlist.session_id, card);
    });
}

function loadPlaylistTracks(sessionId, cardElement) {
    fetch(`/sessions/playlist/${sessionId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load playlist tracks');
            }
            return response.json();
        })
        .then(data => {
            const trackCount = data.tracks ? data.tracks.length : 0;
            const trackCountEl = cardElement.querySelector('.track-count');
            if (trackCountEl) {
                trackCountEl.textContent = `${trackCount} track${trackCount !== 1 ? 's' : ''}`;
            }
        })
        .catch(error => {
            console.error('Error loading playlist tracks:', error);
        });
}

function showPlaylistDetail(sessionId) {
    console.log('showPlaylistDetail called with sessionId:', sessionId);
    savePlaylistId(sessionId);

    document.getElementById('playlist-view').style.display = 'none';
    document.getElementById('playlist-detail-view').style.display = 'block';
    document.getElementById('playlist-name').textContent = sessionId || 'Untitled Playlist';

    document.getElementById('delete-playlist-btn').onclick = function() {
        if (confirm('Are you sure you want to delete this playlist?')) {
            deletePlaylist(sessionId);
        }
    };

    document.getElementById('play-playlist-btn').onclick = function() {
        playPlaylist(sessionId);
    };

    loadPlaylistTracksForDetail(sessionId);
}

function loadPlaylistTracksForDetail(sessionId) {
    fetch(`/sessions/playlist/${sessionId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load playlist tracks: ' + response.status);
            }
            return response.json();
        })
        .then(data => {
            renderPlaylistTracks(data.tracks || [], sessionId);
        })
        .catch(error => {
            console.error('Error loading playlist tracks:', error);
            document.getElementById('playlist-tracks').innerHTML =
                '<p class="empty-message">Error loading tracks. Please try again.</p>';
        });
}

function createTrackListItem(track, index, sessionId) {
    const item = document.createElement('div');
    item.className = 'track-list-item';
    item.draggable = true;
    item.dataset.trackId = track.id;
    item.dataset.index = index;

    let displayAlbumTitle = cleanAlbumTitle(track.album_title, track.title);
    
    item.innerHTML = `
        <span class="drag-handle">☰</span>
        <div class="track-info">
            <div class="track-title">${escapeHtml(track.title || 'Unknown Track')}</div>
            <div class="track-artist">${escapeHtml(displayAlbumTitle)}</div>
        </div>
        <span class="track-duration">${formatDuration(track.duration)}</span>
        <button class="remove-btn" data-track-id="${track.id}">Remove</button>
        <button class="remove-album-btn" data-album-id="${track.album_id}">Album</button>
    `;

    item.querySelector('.remove-btn').addEventListener('click', function(e) {
        e.stopPropagation();
        const trackId = this.dataset.trackId;
        removeTrackFromPlaylist(sessionId, trackId);
    });

    item.querySelector('.remove-album-btn').addEventListener('click', function(e) {
        e.stopPropagation();
        const albumId = this.dataset.albumId;
        removeAlbumFromPlaylist(sessionId, albumId);
    });

    // Click to view track details
    item.addEventListener('click', function(e) {
        if (e.target.classList.contains('remove-btn') || e.target.classList.contains('drag-handle') || e.target.classList.contains('remove-album-btn')) return;
        window.location.href = '/track/' + track.id;
    });

    return item;
}

function renderPlaylistTracks(tracks, sessionId) {
    const container = document.getElementById('playlist-tracks');

    if (tracks.length === 0) {
        container.innerHTML = '<p class="empty-message">No tracks in this playlist. Click "Add Tracks" to add some!</p>';
        return;
    }

    container.innerHTML = '';

    tracks.forEach((track, index) => {
        const item = createTrackListItem(track, index, sessionId);
        item.dataset.trackId = track.id;
        item.dataset.index = index;
        container.appendChild(item);
    });

    initDragAndDrop(container);
}

function initDragAndDrop(container) {
    let draggedItem = null;

    container.querySelectorAll('.track-list-item').forEach(item => {
        item.addEventListener('dragstart', function(e) {
            draggedItem = this;
            this.classList.add('dragging');
            e.dataTransfer.effectAllowed = 'move';
            e.dataTransfer.setData('text/plain', this.dataset.trackId);
        });

        item.addEventListener('dragend', function() {
            this.classList.remove('dragging');
            draggedItem = null;
            container.querySelectorAll('.track-list-item').forEach(el => el.classList.remove('drag-over'));
        });

        item.addEventListener('dragover', function(e) {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';
        });

        item.addEventListener('dragenter', function(e) {
            e.preventDefault();
            if (this !== draggedItem) {
                this.classList.add('drag-over');
            }
        });

        item.addEventListener('dragleave', function() {
            this.classList.remove('drag-over');
        });

        item.addEventListener('drop', function(e) {
            e.preventDefault();
            this.classList.remove('drag-over');

            if (draggedItem && this !== draggedItem) {
                const draggedId = parseInt(draggedItem.dataset.trackId);
                const targetId = parseInt(this.dataset.trackId);

                if (draggedId && targetId && draggedId !== targetId) {
                    reorderPlaylistTracks(window.currentPlaylistId, draggedId, targetId);
                }
            }
        });
    });
}

function showPlaylistsList() {
    clearPlaylistId();
    document.getElementById('playlist-detail-view').style.display = 'none';
    document.getElementById('add-tracks-view').style.display = 'none';
    document.getElementById('playlist-view').style.display = 'block';
    loadPlaylists();
}

function loadAllTracks(excludeTrackIds) {
    console.log('Loading all tracks...');
    let url;
    let excludeParam = excludeTrackIds ? `&exclude_track_ids=${excludeTrackIds}` : '';

    if (availableTrackPagination.query) {
        url = `/tracks/search?q=${encodeURIComponent(availableTrackPagination.query)}&page=${availableTrackPagination.page}&limit=${availableTrackPagination.limit}${excludeParam}`;
    } else {
        url = `/tracks?page=${availableTrackPagination.page}&limit=${availableTrackPagination.limit}${excludeParam}`;
    }
    return fetch(url)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load tracks, status: ' + response.status);
            }
            return response.json();
        })
        .then(data => {
            console.log('Tracks response:', data);
            tracks = data.data || [];
            availableTrackPagination.totalPages = data.totalPages || 1;
            availableTrackPagination.total = data.total || 0;
            console.log('Loaded tracks count:', tracks.length);
            updateAvailableTrackPaginationControls();
        })
        .catch(error => {
            console.error('Error loading tracks:', error);
            tracks = [];
        });
}

function updateAvailableTrackPaginationControls() {
    document.getElementById('available-track-prev').disabled = availableTrackPagination.page <= 1;
    document.getElementById('available-track-next').disabled = availableTrackPagination.page >= availableTrackPagination.totalPages;
    document.getElementById('available-track-page-info').textContent = `Page ${availableTrackPagination.page} of ${availableTrackPagination.totalPages}`;
}

function renderAvailableTracks() {
    console.log('Rendering available tracks...');
    const container = document.getElementById('available-tracks');

    fetch(`/sessions/playlist/${window.currentPlaylistId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load current playlist');
            }
            return response.json();
        })
        .then(data => {
            const currentTrackIds = (data.tracks || []).map(t => t.id);
            const excludeIds = currentTrackIds.join(',');

            return loadAllTracks(excludeIds).then(() => {
                if (tracks.length === 0) {
                    container.innerHTML = '<p class="empty-message">No tracks available. Add some albums first.</p>';
                    return;
                }

                if (tracks.length === 0 && availableTrackPagination.total > 0) {
                    if (availableTrackPagination.page > 1) {
                        availableTrackPagination.page--;
                        return loadAllTracks(excludeIds).then(renderAvailableTracks);
                    }
                }

                container.innerHTML = '';

                tracks.forEach(track => {
                    const item = document.createElement('div');
                    item.className = 'available-track-item';

                    let displayAlbumTitle = cleanAlbumTitle(track.album_title, track.title);

                    item.innerHTML = `
                        <div class="track-cover-small">
                            <img src="/albums/${track.album_id}/image" alt="" class="track-cover-img" onerror="this.style.display='none';this.parentElement.innerHTML='<div class=\\'track-cover-placeholder-small\\'>♪</div>';">
                        </div>
                        <div class="track-info">
                            <h3>${track.title || 'Unknown Title'}</h3>
                            <p>${track.album_artist || 'Unknown Artist'}</p>
                        </div>
                        <div class="track-meta">
                            <p class="track-album-title">${displayAlbumTitle}</p>
                            <p class="track-duration">${formatDuration(track.duration) || ''}</p>
                        </div>
                        <button class="add-btn" data-track-id="${track.id}">Add</button>
                        <button class="add-album-btn" data-album-id="${track.album_id}">Album</button>
                    `;

                    item.querySelector('.add-btn').addEventListener('click', function() {
                        addTrackToPlaylist(window.currentPlaylistId, track.id);
                    });

                    item.querySelector('.add-album-btn').addEventListener('click', function() {
                        addAlbumToPlaylist(window.currentPlaylistId, track.album_id);
                    });

                    container.appendChild(item);
                });
            });
        })
        .catch(error => {
            console.error('Error rendering available tracks:', error);
            container.innerHTML = '<p class="empty-message">Error loading tracks.</p>';
        });
}

function createPlaylist(name) {
    fetch('/sessions/playlist/new', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            session_id: name
        })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Failed to create playlist');
        }
        return response.json();
    })
    .then(data => {
        document.getElementById('create-playlist-modal').style.display = 'none';
        document.getElementById('new-playlist-name').value = '';
        loadPlaylists();
    })
    .catch(error => {
        console.error('Error creating playlist:', error);
        alert('Error creating playlist. Please try again.');
    });
}

function addTrackToPlaylist(sessionId, trackId) {
    fetch(`/sessions/playlist/${sessionId}/tracks`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            track_id: trackId
        })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Failed to add track');
        }
        return response.json();
    })
    .then(data => {
        loadPlaylistTracksForDetail(sessionId);
        renderAvailableTracks();
    })
    .catch(error => {
        console.error('Error adding track:', error);
        alert('Error adding track. Please try again.');
    });
}

function addAlbumToPlaylist(sessionId, albumId) {
    fetch(`/albums/${albumId}/tracks`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to fetch album tracks');
            }
            return response.json();
        })
        .then(tracks => {
            if (!tracks || tracks.length === 0) {
                alert('No tracks found for this album.');
                return;
            }
            fetch(`/sessions/playlist/${sessionId}`)
                .then(response => response.json())
                .then(data => {
                    const currentTrackIds = (data.tracks || []).map(t => t.id);
                    const tracksToAdd = tracks.filter(t => !currentTrackIds.includes(t.id));

                    if (tracksToAdd.length === 0) {
                        return;
                    }

                    const addPromises = tracksToAdd.map(track => {
                        return fetch(`/sessions/playlist/${sessionId}/tracks`, {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json'
                            },
                            body: JSON.stringify({
                                track_id: track.id
                            })
                        });
                    });

                    return Promise.all(addPromises).then(() => {
                        loadPlaylistTracksForDetail(sessionId);
                        renderAvailableTracks();
                    });
                });
        })
        .catch(error => {
            console.error('Error adding album tracks:', error);
            alert('Error adding album tracks. Please try again.');
        });
}

function removeTrackFromPlaylist(sessionId, trackId) {
    fetch(`/sessions/playlist/${sessionId}/tracks/${trackId}`, {
        method: 'DELETE'
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Failed to remove track');
        }
        return response.json();
    })
    .then(data => {
        loadPlaylistTracksForDetail(sessionId);
        renderAvailableTracks();
    })
    .catch(error => {
        console.error('Error removing track:', error);
        alert('Error removing track. Please try again.');
    });
}

function removeAlbumFromPlaylist(sessionId, albumId) {
    fetch(`/sessions/playlist/${sessionId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load playlist');
            }
            return response.json();
        })
        .then(data => {
            const tracks = data.tracks || [];
            const tracksToRemove = tracks.filter(t => t.album_id === parseInt(albumId));

            if (tracksToRemove.length === 0) {
                return;
            }

            const removePromises = tracksToRemove.map(track => {
                return fetch(`/sessions/playlist/${sessionId}/tracks/${track.id}`, {
                    method: 'DELETE'
                });
            });

            return Promise.all(removePromises).then(() => {
                loadPlaylistTracksForDetail(sessionId);
                renderAvailableTracks();
            });
        })
        .catch(error => {
            console.error('Error removing album tracks:', error);
            alert('Error removing album tracks. Please try again.');
        });
}

function reorderPlaylistTracks(sessionId, draggedTrackId, targetTrackId) {
    fetch(`/sessions/playlist/${sessionId}`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            dragged_track_id: parseInt(draggedTrackId),
            target_track_id: parseInt(targetTrackId)
        })
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Failed to reorder tracks');
        }
        return response.json();
    })
    .then(data => {
        loadPlaylistTracksForDetail(sessionId);
    })
    .catch(error => {
        console.error('Error reordering tracks:', error);
        alert('Error reordering tracks. Please try again.');
    });
}

function playPlaylist(sessionId) {
    console.log('Play clicked for playlist:', sessionId);
    
    // First get the playlist details with tracks
    fetch(`/sessions/playlist/${sessionId}`)
        .then(response => {
            console.log('Playlist response status:', response.status);
            if (!response.ok) {
                throw new Error('Failed to load playlist, status: ' + response.status);
            }
            return response.json();
        })
        .then(data => {
            console.log('Playlist data:', data);
            const tracks = data.tracks || [];
            if (tracks.length === 0) {
                alert('This playlist has no tracks.');
                return;
            }

            // Extract track IDs
            const trackIds = tracks.map(t => t.id);
            console.log('Track IDs:', trackIds);

            // Start playlist playback with queue
            return fetch('/playback/start-playlist', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    playlist_id: sessionId,
                    playlist_name: sessionId,
                    track_ids: trackIds
                })
            });
        })
        .then(response => {
            console.log('Start playlist response status:', response.status);
            if (!response.ok) {
                const errorText = response.text();
                throw new Error('Failed to start playback, status: ' + response.status + ', error: ' + errorText);
            }
            return response.json();
        })
        .then(data => {
            console.log('Playback started:', data);
            window.location.href = '/player';
        })
        .catch(error => {
            console.error('Error starting playback:', error);
            alert('Error starting playback: ' + error.message);
        });
}

function deletePlaylist(sessionId) {
    fetch(`/sessions/playlist/${sessionId}`, {
        method: 'DELETE'
    })
    .then(response => {
        if (!response.ok) {
            return response.text().then(text => {
                throw new Error('Failed to delete playlist: ' + text);
            });
        }
        return response.json().catch(() => ({}));
    })
    .then(data => {
        showPlaylistsList();
    })
    .catch(error => {
        console.error('Error deleting playlist:', error);
        alert('Error deleting playlist. Please try again.');
    });
}

function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return '0:00';
    
    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes}:${secs < 10 ? '0' : ''}${secs}`;
}

function formatDate(dateString) {
    if (!dateString) return 'Unknown date';
    
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric'
    });
}

function escapeHtml(text) {
    if (!text) return '';
    
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function showNotification(message, type) {
    const existing = document.querySelector('.notification');
    if (existing) existing.remove();

    const notification = document.createElement('div');
    notification.className = `notification ${type || 'info'}`;
    notification.textContent = message;
    document.body.appendChild(notification);

    setTimeout(() => {
        notification.classList.add('fade-out');
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

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
            const matched = tracks.filter(t => t.status === 'matched').length;
            const needsReview = tracks.filter(t => t.status === 'needs_review').length;
            const unavailable = tracks.filter(t => t.status === 'unavailable').length;
            const pending = tracks.filter(t => t.status === 'pending').length;

            document.getElementById('yt-matched-count').textContent = matched;
            document.getElementById('yt-review-count').textContent = needsReview;
            document.getElementById('yt-unavailable-count').textContent = unavailable;
            document.getElementById('yt-pending-count').textContent = pending;

            if (matched + needsReview + unavailable + pending > 0) {
                document.getElementById('youtube-sync-status').style.display = 'block';
            } else {
                document.getElementById('youtube-sync-status').style.display = 'none';
            }
        })
        .catch(error => {
            console.error('Error loading YouTube match status:', error);
        });
}

function openYouTubeSyncModal() {
    if (!window.currentPlaylistId) {
        showNotification('No playlist selected', 'error');
        return;
    }

    document.getElementById('youtube-sync-modal').style.display = 'flex';
    loadYouTubeSyncStatus();
}

function loadYouTubeSyncStatus() {
    const playlistId = window.currentPlaylistId;

    fetch(`/api/youtube/matches/${playlistId}`)
        .then(response => {
            if (!response.ok) {
                if (response.status === 404) {
                    updateSyncStatusDisplay(0, 0, 0, 0);
                    return null;
                }
                throw new Error('Failed to load sync status');
            }
            return response.json();
        })
        .then(data => {
            if (!data || !data.tracks) {
                updateSyncStatusDisplay(0, 0, 0, 0);
                document.getElementById('sync-youtube-section').style.display = 'none';
                return;
            }

            const tracks = data.tracks;
            const matched = tracks.filter(t => t.status === 'matched').length;
            const needsReview = tracks.filter(t => t.status === 'needs_review').length;
            const unavailable = tracks.filter(t => t.status === 'unavailable').length;
            const pending = tracks.filter(t => t.status === 'pending').length;

            updateSyncStatusDisplay(matched, needsReview, unavailable, pending);

            if (matched > 0 || needsReview > 0) {
                document.getElementById('sync-youtube-section').style.display = 'block';
            } else {
                document.getElementById('sync-youtube-section').style.display = 'none';
            }
        })
        .catch(error => {
            console.error('Error loading sync status:', error);
            showNotification('Error loading sync status', 'error');
        });
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
    })
    .catch(error => {
        console.error('Error syncing to YouTube:', error);
        document.getElementById('sync-to-youtube-btn').disabled = false;
        document.getElementById('youtube-status-msg').textContent = 'Error: ' + error.message;
        showNotification('Error syncing to YouTube: ' + error.message, 'error');
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

let reviewTracks = [];
let currentReviewIndex = 0;
let currentReviewPlaylistId = '';

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
        <p>${escapeHtml(track.track_title)} - ${escapeHtml(track.artist)}</p>
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
                <p class="candidate-duration">Duration: ${formatDuration(candidate.duration)}</p>
                <p class="candidate-score">Match Score: ${(candidate.match_score * 100).toFixed(0)}%</p>
            </div>
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

// Initialize event listeners when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    console.log('Playlist management script executing');
    
    // Back to playlists button
    document.getElementById('back-to-playlists').addEventListener('click', function() {
        console.log('Back to playlists clicked');
        showPlaylistsList();
    });

    // Back to playlist button (from add-tracks view)
    document.getElementById('back-to-playlist').addEventListener('click', function() {
        document.getElementById('add-tracks-view').style.display = 'none';
        document.getElementById('playlist-detail-view').style.display = 'block';
    });

    // Add tracks button
    document.getElementById('add-tracks-btn').addEventListener('click', function() {
        if (!window.currentPlaylistId) {
            alert('No playlist selected');
            return;
        }
        document.getElementById('playlist-detail-view').style.display = 'none';
        document.getElementById('add-tracks-view').style.display = 'block';
        // Reset pagination and search
        availableTrackPagination = {
            page: 1,
            limit: 25,
            query: '',
            totalPages: 1,
            total: 0
        };
        document.getElementById('available-track-search').value = '';
        renderAvailableTracks();
    });

    // Create playlist button
    document.getElementById('create-playlist-btn').addEventListener('click', function() {
        document.getElementById('create-playlist-modal').style.display = 'flex';
        document.getElementById('new-playlist-name').focus();
    });

    // Close modal
    document.querySelector('#create-playlist-modal .close-modal').addEventListener('click', function() {
        document.getElementById('create-playlist-modal').style.display = 'none';
    });

    // Save playlist button
    document.getElementById('save-playlist-btn').addEventListener('click', function() {
        const name = document.getElementById('new-playlist-name').value.trim();
        if (!name) {
            alert('Please enter a playlist name');
            return;
        }
        createPlaylist(name);
    });

    // Close modal on outside click
    document.getElementById('create-playlist-modal').addEventListener('click', function(e) {
        if (e.target === this) {
            this.style.display = 'none';
        }
    });

    // Available track search
    let availableTrackSearchTimeout;
    document.getElementById('available-track-search').addEventListener('input', function(e) {
        clearTimeout(availableTrackSearchTimeout);
        const query = e.target.value.trim();
        availableTrackSearchTimeout = setTimeout(() => {
            availableTrackPagination.page = 1;
            availableTrackPagination.query = query;
            renderAvailableTracks();
        }, 300);
    });

    document.querySelector('#add-tracks-view .search-clear').addEventListener('click', function() {
        const searchInput = document.getElementById('available-track-search');
        searchInput.value = '';
        availableTrackPagination.page = 1;
        availableTrackPagination.query = '';
        renderAvailableTracks();
    });

    // Available track pagination
    document.getElementById('available-track-prev').addEventListener('click', function() {
        if (availableTrackPagination.page > 1) {
            availableTrackPagination.page--;
            renderAvailableTracks();
        }
    });

    document.getElementById('available-track-next').addEventListener('click', function() {
        if (availableTrackPagination.page < availableTrackPagination.totalPages) {
            availableTrackPagination.page++;
            renderAvailableTracks();
        }
    });

    document.getElementById('available-track-limit').addEventListener('change', function() {
        availableTrackPagination.limit = parseInt(this.value);
        availableTrackPagination.page = 1;
        renderAvailableTracks();
    });

    document.getElementById('sync-youtube-btn').addEventListener('click', function() {
        if (!window.currentPlaylistId) {
            showNotification('No playlist selected', 'error');
            return;
        }
        openYouTubeSyncModal();
    });

    document.getElementById('close-youtube-sync').addEventListener('click', closeYouTubeSyncModal);
    document.getElementById('close-youtube-review').addEventListener('click', closeYouTubeReviewModal);

    document.getElementById('match-playlist-btn').addEventListener('click', matchPlaylistTracks);
    document.getElementById('sync-to-youtube-btn').addEventListener('click', syncToYouTube);

    document.querySelectorAll('input[name="youtube-export"]').forEach(radio => {
        radio.addEventListener('change', function() {
            const isNew = this.value === 'new';
            document.getElementById('new-playlist-name-yt').disabled = !isNew;
            document.getElementById('existing-playlist-id').disabled = isNew;
        });
    });

    document.getElementById('create-playlist-modal').addEventListener('click', function(e) {
        if (e.target === this) {
            this.style.display = 'none';
        }
    });

    document.getElementById('youtube-sync-modal').addEventListener('click', function(e) {
        if (e.target === this) {
            closeYouTubeSyncModal();
        }
    });

    document.getElementById('youtube-review-modal').addEventListener('click', function(e) {
        if (e.target === this) {
            closeYouTubeReviewModal();
        }
    });

    loadPlaylists();
});
