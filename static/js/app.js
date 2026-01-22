import { pagination, formatDuration, formatTime, escapeHtml, cleanAlbumTitle, cleanArtistName, updatePaginationControls } from './app-state.js';

window.pagination = pagination;
window.formatDuration = formatDuration;
window.formatTime = formatTime;
window.escapeHtml = escapeHtml;
window.cleanAlbumTitle = cleanAlbumTitle;
window.cleanArtistName = cleanArtistName;
window.updatePaginationControls = updatePaginationControls;

document.addEventListener('DOMContentLoaded', function() {
    console.log('Vinylfo frontend loaded');

    const albumSearchInput = document.getElementById('album-search');
    const trackSearchInput = document.getElementById('track-search');
    
    let albumSearchTimeout;
    let trackSearchTimeout;

    document.querySelectorAll('.album-limit').forEach(el => {
        el.addEventListener('change', function() {
            pagination.album.limit = parseInt(this.value);
            pagination.album.page = 1;
            if (pagination.album.query) {
                searchAlbums(pagination.album.query);
            } else {
                loadAlbums();
            }
        });
    });

    document.querySelectorAll('.album-prev').forEach(el => {
        el.addEventListener('click', function() {
            if (pagination.album.page > 1) {
                pagination.album.page--;
                if (pagination.album.query) {
                    searchAlbums(pagination.album.query);
                } else {
                    loadAlbums();
                }
            }
        });
    });

    document.querySelectorAll('.album-next').forEach(el => {
        el.addEventListener('click', function() {
            if (pagination.album.page < pagination.album.totalPages) {
                pagination.album.page++;
                if (pagination.album.query) {
                    searchAlbums(pagination.album.query);
                } else {
                    loadAlbums();
                }
            }
        });
    });

    document.querySelectorAll('.track-limit').forEach(el => {
        el.addEventListener('change', function() {
            pagination.track.limit = parseInt(this.value);
            pagination.track.page = 1;
            if (pagination.track.query) {
                searchTracks(pagination.track.query);
            } else {
                loadTracks();
            }
        });
    });

    document.querySelectorAll('.track-prev').forEach(el => {
        el.addEventListener('click', function() {
            if (pagination.track.page > 1) {
                pagination.track.page--;
                if (pagination.track.query) {
                    searchTracks(pagination.track.query);
                } else {
                    loadTracks();
                }
            }
        });
    });

    document.querySelectorAll('.track-next').forEach(el => {
        el.addEventListener('click', function() {
            if (pagination.track.page < pagination.track.totalPages) {
                pagination.track.page++;
                if (pagination.track.query) {
                    searchTracks(pagination.track.query);
                } else {
                    loadTracks();
                }
            }
        });
    });

    const navLinks = document.querySelectorAll('nav a');
    navLinks.forEach(link => {
        link.addEventListener('click', function(e) {
            const href = this.getAttribute('href');

            if (href.startsWith('#')) {
                const targetHash = href.substring(1);
                const viewElement = document.getElementById(`${targetHash}-view`);
                if (viewElement) {
                    e.preventDefault();
                    window.location.hash = targetHash;
                }
                return;
            }

            if (href.startsWith('/') && href !== '/') {
                e.preventDefault();
                window.location.href = href;
                return;
            }
        });
    });

    window.addEventListener('hashchange', function() {
        const hash = window.location.hash.substring(1);
        if (hash) {
            document.querySelectorAll('.view').forEach(view => {
                view.style.display = 'none';
            });
            const viewElement = document.getElementById(`${hash}-view`);
            if (viewElement) {
                viewElement.style.display = 'block';

                if (hash === 'albums') {
                    loadAlbums();
                } else if (hash === 'tracks') {
                    loadTracks();
                } else if (hash === 'track' || hash.startsWith('track-')) {
                    const trackId = hash.replace('track-', '');
                    loadTrackDetail(trackId);
                } else if (hash === 'sessions') {
                    loadSessions();
                }
            }
        }
    });

    if (window.location.hash) {
        const hash = window.location.hash.substring(1);
        document.querySelectorAll('.view').forEach(view => {
            view.style.display = 'none';
        });
        const viewElement = document.getElementById(`${hash}-view`);
        if (viewElement) {
            viewElement.style.display = 'block';
            if (hash === 'tracks') {
                loadTracks();
            } else if (hash === 'albums') {
                loadAlbums();
            } else if (hash === 'sessions') {
                loadSessions();
            }
        } else if (hash.startsWith('track-')) {
            document.getElementById('track-detail-view').style.display = 'block';
            const trackId = hash.replace('track-', '');
            loadTrackDetail(trackId);
        }
    } else {
        document.querySelectorAll('.view').forEach(view => {
            view.style.display = 'none';
        });
        document.getElementById('albums-view').style.display = 'block';
        loadAlbums();
    }

    const backToTracksBtn = document.getElementById('back-to-tracks');
    if (backToTracksBtn) {
        backToTracksBtn.addEventListener('click', function() {
            document.querySelectorAll('.view').forEach(view => {
                view.style.display = 'none';
            });
            document.getElementById('tracks-view').style.display = 'block';
            loadTracks();
        });
    }

    if (albumSearchInput) {
        albumSearchInput.addEventListener('input', function(e) {
            clearTimeout(albumSearchTimeout);
            const query = e.target.value.trim();
            albumSearchTimeout = setTimeout(() => {
                pagination.album.page = 1;
                if (query) {
                    searchAlbums(query);
                } else {
                    loadAlbums();
                }
            }, 300);
        });
    }

    const albumSearchClear = document.querySelector('#albums-view .search-clear');
    if (albumSearchClear) {
        albumSearchClear.addEventListener('click', function() {
            const searchInput = document.getElementById('album-search');
            searchInput.value = '';
            pagination.album.page = 1;
            loadAlbums();
            searchInput.focus();
        });
    }

    if (trackSearchInput) {
        trackSearchInput.addEventListener('input', function(e) {
            clearTimeout(trackSearchTimeout);
            const query = e.target.value.trim();
            trackSearchTimeout = setTimeout(() => {
                pagination.track.page = 1;
                if (query) {
                    searchTracks(query);
                } else {
                    loadTracks();
                }
            }, 300);
        });
    }

    const trackSearchClear = document.querySelector('#tracks-view .search-clear');
    if (trackSearchClear) {
        trackSearchClear.addEventListener('click', function() {
            const searchInput = document.getElementById('track-search');
            searchInput.value = '';
            pagination.track.page = 1;
            loadTracks();
            searchInput.focus();
        });
    }

    loadAlbums();
});

function renderAlbums(albums) {
    const list = document.getElementById('albums-list');
    list.innerHTML = '';

    if (!albums || albums.length === 0) {
        list.innerHTML = '<p>No albums found.</p>';
        return;
    }

    albums.forEach(album => {
        const item = document.createElement('div');
        item.className = 'album-item';
        
        const imageContainer = document.createElement('div');
        imageContainer.className = 'album-cover-container';
        
        if (album.cover_image_url || album.discogs_cover_image_type || album.cover_image_failed) {
            const img = document.createElement('img');
            img.src = '/albums/' + album.id + '/image';
            img.alt = album.title || '';
            img.className = 'album-cover';
            img.onerror = function() {
                this.style.display = 'none';
                imageContainer.innerHTML = '<div class="album-cover-placeholder">No Cover</div>';
            };
            imageContainer.appendChild(img);
        } else {
            imageContainer.innerHTML = '<div class="album-cover-placeholder">No Cover</div>';
        }
        
        const infoDiv = document.createElement('div');
        infoDiv.className = 'album-info';
        infoDiv.innerHTML = '<h3>' + (album.title || 'Unknown Title') + '</h3><p>Artist: ' + cleanArtistName(album.artist) + '</p><p>Year: ' + (album.release_year || 'Unknown Year') + '</p>';
        
        item.appendChild(imageContainer);
        item.appendChild(infoDiv);
        
        item.addEventListener('click', function() {
            window.location.href = '/album/' + album.id;
        });
        list.appendChild(item);
    });
}

function renderTracks(tracks) {
    const list = document.getElementById('tracks-list');
    list.innerHTML = '';

    if (!tracks || tracks.length === 0) {
        list.innerHTML = '<p>No tracks found.</p>';
        return;
    }

    tracks.forEach(track => {
        let displayAlbumTitle = cleanAlbumTitle(track.album_title, track.title);
        
        const item = document.createElement('div');
        item.className = 'track-item';
        item.innerHTML = `
            <div class="track-cover-small">
                <img src="/albums/${track.album_id}/image" alt="" class="track-cover-img" onerror="this.style.display='none';this.parentElement.innerHTML='<div class=\\'track-cover-placeholder-small\\'>♪</div>';">
            </div>
            <div class="track-info">
                <h3>${track.title || 'Unknown Title'}</h3>
                <p>${cleanArtistName(track.album_artist)}</p>
            </div>
            <div class="track-meta">
                <p class="track-album-title">${displayAlbumTitle}</p>
                <p class="track-duration">${formatDuration(track.duration) || ''}</p>
            </div>
        `;
        item.addEventListener('click', function() {
            window.location.href = '/track/' + track.id;
        });
        list.appendChild(item);
    });
}

function searchAlbums(query) {
    pagination.album.query = query;
    const url = `/albums/search?q=${encodeURIComponent(query)}&page=${pagination.album.page}&limit=${pagination.album.limit}`;
    fetch(url)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                console.error('API error:', data.error);
                document.getElementById('albums-list').innerHTML = '<p>Error: ' + data.error + '</p>';
                return;
            }
            const albums = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
            pagination.album.totalPages = data.totalPages || 1;
            updatePaginationControls('album');
            renderAlbums(albums);
        })
        .catch(error => {
            console.error('Error searching albums:', error);
            document.getElementById('albums-list').innerHTML = '<p>Error searching albums</p>';
        });
}

function searchTracks(query) {
    pagination.track.query = query;
    const url = `/tracks/search?q=${encodeURIComponent(query)}&page=${pagination.track.page}&limit=${pagination.track.limit}`;
    fetch(url)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                console.error('API error:', data.error);
                document.getElementById('tracks-list').innerHTML = '<p>Error: ' + data.error + '</p>';
                return;
            }
            const tracks = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
            pagination.track.totalPages = data.totalPages || 1;
            updatePaginationControls('track');
            renderTracks(tracks);
        })
        .catch(error => {
            console.error('Error searching tracks:', error);
            document.getElementById('tracks-list').innerHTML = '<p>Error searching tracks</p>';
        });
}

function loadAlbums() {
    pagination.album.query = '';
    const url = `/albums?page=${pagination.album.page}&limit=${pagination.album.limit}`;
    fetch(url)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                console.error('API error:', data.error);
                document.getElementById('albums-list').innerHTML = '<p>Error: ' + data.error + '</p>';
                return;
            }
            const albums = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
            pagination.album.totalPages = data.totalPages || 1;
            updatePaginationControls('album');
            renderAlbums(albums);
        })
        .catch(error => {
            console.error('Error loading albums:', error);
            document.getElementById('albums-list').innerHTML = '<p>Error loading albums</p>';
        });
}

function loadTracks() {
    pagination.track.query = '';
    const url = `/tracks?page=${pagination.track.page}&limit=${pagination.track.limit}`;
    console.log('Loading tracks from:', url);
    fetch(url)
        .then(response => {
            console.log('Response status:', response.status);
            return response.json();
        })
        .then(data => {
            console.log('Tracks data:', data);
            if (data.error) {
                console.error('API error:', data.error);
                document.getElementById('tracks-list').innerHTML = '<p>Error: ' + data.error + '</p>';
                return;
            }
            const tracks = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
            pagination.track.totalPages = data.totalPages || 1;
            updatePaginationControls('track');
            renderTracks(tracks);
        })
        .catch(error => {
            console.error('Error loading tracks:', error);
            document.getElementById('tracks-list').innerHTML = '<p>Error loading tracks</p>';
        });
}

function showTracksForAlbum(albumID) {
    fetch(`/albums/${albumID}`)
        .then(response => response.json())
        .then(album => {
            fetch(`/albums/${albumID}/tracks`)
                .then(response => response.json())
                .then(data => {
                    const list = document.getElementById('tracks-list');
                    list.innerHTML = '';
                    
                    let coverHtml = '<div class="album-cover-placeholder">No Cover</div>';
                    if (album.cover_image_url || album.cover_image_type) {
                        coverHtml = `<img src="/albums/${album.id}/image" alt="${album.title}" class="album-cover" onerror="this.style.display='none';this.parentElement.innerHTML='<div class=\\'album-cover-placeholder\\'>No Cover</div>';">`;
                    }
                    
                    const headerHtml = `
                        <div class="album-header" style="display: flex; align-items: center; margin-bottom: 1rem; padding-bottom: 1rem; border-bottom: 1px solid #eee;">
                            ${coverHtml}
                            <div>
                                <h2 style="margin: 0 0 0.25rem 0;">${escapeHtml(album.title || 'Unknown Title')}</h2>
                                <p style="margin: 0; color: #666;">${escapeHtml(cleanArtistName(album.artist))}</p>
                                <p style="margin: 0; color: #666; font-size: 0.85rem;">${album.release_year || ''}</p>
                            </div>
                        </div>
                    `;
                    
                    if (!data || data.length === 0) {
                        list.innerHTML = headerHtml + '<p>No tracks found for this album.</p>';
                        return;
                    }
                    
                    list.innerHTML = headerHtml;
                    
                    data.forEach(track => {
                        let displayAlbumTitle = cleanAlbumTitle(track.album_title, track.title);
                        
                        const item = document.createElement('div');
                        item.className = 'track-item';
                        item.innerHTML = `
                            <div class="track-cover-small">
                                <img src="/albums/${track.album_id}/image" alt="" class="track-cover-img" onerror="this.style.display='none';this.parentElement.innerHTML='<div class=\\'track-cover-placeholder-small\\'>♪</div>';">
                            </div>
                            <div class="track-info">
                                <h3>${track.title || 'Unknown Title'}</h3>
                                <p>${cleanArtistName(track.album_artist)}</p>
                            </div>
                            <div class="track-meta">
                                <p class="track-duration">${formatDuration(track.duration) || ''}</p>
                            </div>
                        `;
                        item.addEventListener('click', function() {
                            window.location.href = '/track/' + track.id;
                        });
                        list.appendChild(item);
                    });
                })
                .catch(error => {
                    console.error('Error loading tracks:', error);
                    const list = document.getElementById('tracks-list');
                    list.innerHTML = '<p>Error loading tracks</p>';
                });
        })
        .catch(error => {
            console.error('Error loading album:', error);
            const list = document.getElementById('tracks-list');
            list.innerHTML = '<p>Error loading album</p>';
        });
    
    document.querySelectorAll('.view').forEach(view => {
        view.style.display = 'none';
    });
    document.getElementById('tracks-view').style.display = 'block';
}

function loadTrackDetail(trackId) {
    fetch('/tracks/' + trackId)
        .then(response => response.json())
        .then(track => {
            const detail = document.getElementById('track-detail');
            
            let displayAlbumTitle = cleanAlbumTitle(track.album_title, track.title);
            
            detail.innerHTML = `
                <div class="track-detail-item">
                    <h3>${track.title || 'Unknown Title'}</h3>
                    <p><strong>Artist:</strong> ${cleanArtistName(track.album_artist)}</p>
                    <p><strong>Album:</strong> ${displayAlbumTitle}</p>
                    <p><strong>Duration:</strong> ${formatDuration(track.duration) || 'Unknown Duration'}</p>
                    <p><strong>Track Number:</strong> ${track.track_number || 'Unknown'}</p>
                </div>
            `;
        })
        .catch(error => {
            console.error('Error loading track detail:', error);
            const detail = document.getElementById('track-detail');
            detail.innerHTML = '<p>Error loading track details</p>';
        });
}

function loadSessions() {
    fetch('/sessions')
        .then(response => response.json())
        .then(data => {
            const list = document.getElementById('sessions-list');
            list.innerHTML = '';
            
            if (!data || data.length === 0) {
                list.innerHTML = '<p>No sessions found.</p>';
                return;
            }
            
            data.forEach(session => {
                const item = document.createElement('div');
                item.className = 'session-item';
                item.style.cursor = 'pointer';
                item.title = 'Click to restore this session';
                
                const statusBadge = session.status === 'playing' ? '🔊 Playing' : 
                                   session.status === 'paused' ? '⏸️ Paused' : 
                                   '⏹️ Stopped';
                
                const startedDate = session.started_at ? new Date(session.started_at).toLocaleString() : 'Unknown';
                const lastPlayed = session.last_played_at ? new Date(session.last_played_at).toLocaleString() : 'Unknown';
                
                const playlistId = session.playlist_id || 'Untitled';
                const playlistName = session.playlist_name || playlistId;
                
                let queueInfo = '';
                if (session.queue_count !== undefined) {
                    queueInfo = `<p>Tracks in queue: ${session.queue_count}</p>`;
                } else {
                    try {
                        const queue = JSON.parse(session.queue || '[]');
                        queueInfo = `<p>Tracks in queue: ${queue.length}</p>`;
                    } catch (e) {
                        queueInfo = '';
                    }
                }
                
                item.innerHTML = `
                    <h3>${escapeHtml(playlistName)} <span style="font-size: 0.6em;">${statusBadge}</span></h3>
                    <p><strong>Playlist ID:</strong> ${escapeHtml(playlistId)}</p>
                    <p><strong>Status:</strong> ${statusBadge}</p>
                    <p><strong>Started:</strong> ${startedDate}</p>
                    <p><strong>Last Played:</strong> ${lastPlayed}</p>
                    <p><strong>Queue Position:</strong> ${session.queue_position || 0}s in track ${session.queue_index + 1 || 1}</p>
                    ${queueInfo}
                `;
                
                item.addEventListener('click', () => restoreSession(session.playlist_id));
                list.appendChild(item);
            });
        })
        .catch(error => {
            console.error('Error loading sessions:', error);
            const list = document.getElementById('sessions-list');
            list.innerHTML = '<p>Error loading sessions</p>';
        });
}

function restoreSession(playlistId) {
    console.log('Restoring session for playlist:', playlistId, typeof playlistId);
    const body = { playlist_id: playlistId };
    console.log('Sending request body:', JSON.stringify(body));
    console.log('Content-Type:', 'application/json');
    fetch('/playback/restore', {
        method: 'POST',
        headers: { 
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        },
        body: JSON.stringify(body)
    })
    .then(response => {
        console.log('Restore response status:', response.status);
        if (!response.ok) {
            return response.text().then(text => {
                console.error('Error response body:', text);
                throw new Error(`HTTP ${response.status}: ${text}`);
            });
        }
        return response.json();
    })
    .then(data => {
        console.log('Session restored:', data);
        if (data.track) {
            if (data.queue_position !== undefined) {
                localStorage.setItem('vinylfo_queuePosition', data.queue_position.toString());
                console.log('Saved queue position:', data.queue_position);
            }
            if (data.queue && data.queue.length > 0) {
                localStorage.setItem('vinylfo_queue', JSON.stringify(data.queue));
                localStorage.setItem('vinylfo_queueIndex', data.queue_index.toString());
                console.log('Saved queue with', data.queue.length, 'tracks');
            }
            window.location.href = '/player';
        }
    })
    .catch(error => {
        console.error('Error restoring session:', error);
        alert('Failed to restore session: ' + error.message);
    });
}

window.loadAlbums = loadAlbums;
window.loadTracks = loadTracks;
window.showTracksForAlbum = showTracksForAlbum;
window.loadTrackDetail = loadTrackDetail;
window.loadSessions = loadSessions;
window.restoreSession = restoreSession;
window.searchAlbums = searchAlbums;
window.searchTracks = searchTracks;
