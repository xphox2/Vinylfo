// Playlist management JavaScript
(function() {
    console.log('Playlist management script executing');
    
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
    `;

    item.querySelector('.remove-btn').addEventListener('click', function(e) {
        e.stopPropagation();
        const trackId = this.dataset.trackId;
        if (confirm('Are you sure you want to remove this track from the playlist?')) {
            removeTrackFromPlaylist(sessionId, trackId);
        }
    });

    // Click to view track details
    item.addEventListener('click', function(e) {
        if (e.target.classList.contains('remove-btn') || e.target.classList.contains('drag-handle')) return;
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

function loadAllTracks() {
    console.log('Loading all tracks...');
    let url;
    if (availableTrackPagination.query) {
        url = `/tracks/search?q=${encodeURIComponent(availableTrackPagination.query)}&page=${availableTrackPagination.page}&limit=${availableTrackPagination.limit}`;
    } else {
        url = `/tracks?page=${availableTrackPagination.page}&limit=${availableTrackPagination.limit}`;
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
    console.log('Rendering available tracks, total loaded:', tracks.length);
    const container = document.getElementById('available-tracks');
    
    if (tracks.length === 0) {
        container.innerHTML = '<p class="empty-message">No tracks available. Add some albums first.</p>';
        return;
    }
    
    fetch(`/sessions/playlist/${window.currentPlaylistId}`)
        .then(response => {
            if (!response.ok) {
                throw new Error('Failed to load current playlist');
            }
            return response.json();
        })
        .then(data => {
            const currentTrackIds = (data.tracks || []).map(t => t.id);
            const availableTracks = tracks.filter(t => !currentTrackIds.includes(t.id));
            
            if (availableTracks.length === 0) {
                container.innerHTML = '<p class="empty-message">All tracks are already in this playlist.</p>';
                return;
            }
            
            container.innerHTML = '';
            
            availableTracks.forEach(track => {
                const item = document.createElement('div');
                item.className = 'available-track-item';
                
                let displayAlbumTitle = cleanAlbumTitle(track.album_title, track.title);
                
                item.innerHTML = `
                    <div class="track-info">
                        <div class="track-title">${escapeHtml(track.title)}</div>
                        <div class="track-artist">${escapeHtml(displayAlbumTitle)}</div>
                    </div>
                    <button class="add-btn" data-track-id="${track.id}">Add</button>
                `;
                
                item.querySelector('.add-btn').addEventListener('click', function() {
                    addTrackToPlaylist(window.currentPlaylistId, track.id);
                });
                
                container.appendChild(item);
            });
        })
        .catch(error => {
            console.error('Error loading current playlist:', error);
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
    })
    .catch(error => {
        console.error('Error removing track:', error);
        alert('Error removing track. Please try again.');
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
            window.location.href = '/dashboard';
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
            throw new Error('Failed to delete playlist');
        }
        return response.json();
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

// Initialize event listeners when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    // Back to playlists button
    document.getElementById('back-to-playlists').addEventListener('click', function() {
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
        loadAllTracks().then(renderAvailableTracks);
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
            loadAllTracks().then(renderAvailableTracks);
        }, 300);
    });

    document.querySelector('#add-tracks-view .search-clear').addEventListener('click', function() {
        const searchInput = document.getElementById('available-track-search');
        searchInput.value = '';
        availableTrackPagination.page = 1;
        availableTrackPagination.query = '';
        loadAllTracks().then(renderAvailableTracks);
    });

    // Available track pagination
    document.getElementById('available-track-prev').addEventListener('click', function() {
        if (availableTrackPagination.page > 1) {
            availableTrackPagination.page--;
            loadAllTracks().then(renderAvailableTracks);
        }
    });

    document.getElementById('available-track-next').addEventListener('click', function() {
        if (availableTrackPagination.page < availableTrackPagination.totalPages) {
            availableTrackPagination.page++;
            loadAllTracks().then(renderAvailableTracks);
        }
    });

    document.getElementById('available-track-limit').addEventListener('change', function() {
        availableTrackPagination.limit = parseInt(this.value);
        availableTrackPagination.page = 1;
        loadAllTracks().then(renderAvailableTracks);
    });

    // Load playlists on page load
    loadPlaylists();
})();
})();
