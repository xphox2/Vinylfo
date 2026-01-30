import { pagination, formatDuration, formatTime, escapeHtml, cleanAlbumTitle, cleanArtistName, cleanTrackTitle, updatePaginationControls } from './app-state.js';

window.pagination = pagination;
window.formatDuration = formatDuration;
window.formatTime = formatTime;
window.escapeHtml = escapeHtml;
window.cleanAlbumTitle = cleanAlbumTitle;
window.cleanArtistName = cleanArtistName;
window.cleanTrackTitle = cleanTrackTitle;
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
        
        // Add delete icon
        const deleteIcon = document.createElement('div');
        deleteIcon.className = 'album-delete-icon';
        deleteIcon.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">
                <path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/>
            </svg>
        `;
        deleteIcon.title = 'Delete Album';
        deleteIcon.onclick = function(e) {
            e.stopPropagation();
            openDeleteAlbumModal(album.id, album.title, album.artist);
        };
        
        item.appendChild(imageContainer);
        item.appendChild(infoDiv);
        item.appendChild(deleteIcon);
        
        item.addEventListener('click', function(e) {
            if (!e.target.closest('.album-delete-icon')) {
                window.location.href = '/album/' + album.id;
            }
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
        
        const hasYouTubeVideo = track.youtube_video_id && track.youtube_video_id.trim() !== '';
        
        const videoIconClass = hasYouTubeVideo ? 'track-video-icon--available' : 'track-video-icon--unavailable';
        const videoIconClick = hasYouTubeVideo ? `onclick="openYouTubeVideo('${track.youtube_video_id}'); return false;"` : `onclick="openYouTubeModal(${track.id}); return false;"`;
        const videoIconTitle = hasYouTubeVideo ? 'Watch on YouTube' : 'Add YouTube video';
        
        const clearIconStyle = hasYouTubeVideo ? '' : 'display: none;';
        
        item.innerHTML = `
            <div class="track-cover-small">
                <img src="/albums/${track.album_id}/image" alt="" class="track-cover-img" onerror="this.style.display='none';this.parentElement.innerHTML='<div class=\\'track-cover-placeholder-small\\'>♪</div>';">
            </div>
            <div class="track-info">
                <h3>${cleanTrackTitle(track.title) || 'Unknown Title'}</h3>
                <p>${cleanArtistName(track.album_artist)}</p>
            </div>
            <div class="track-video-clear" style="${clearIconStyle}" onclick="openClearYouTubeModal(${track.id}, '${escapeHtml(track.title)}'); return false;" title="Clear YouTube video">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
                </svg>
            </div>
            <div class="track-video-icon ${videoIconClass}" ${videoIconClick} title="${videoIconTitle}">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M17 10.5V7c0-.55-.45-1-1-1H4c-.55 0-1 .45-1 1v10c0 .55.45 1 1 1h12c.55 0 1-.45 1-1v-3.5l4 4v-11l-4 4z"/>
                </svg>
            </div>
            <div class="track-meta">
                <p class="track-album-title">${displayAlbumTitle}</p>
                <p class="track-duration">${formatDuration(track.duration) || ''}</p>
            </div>
        `;
        item.addEventListener('click', function(e) {
            if (!e.target.closest('.track-video-icon') && !e.target.closest('.track-video-clear')) {
                window.location.href = '/track/' + track.id;
            }
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
                                <h3>${cleanTrackTitle(track.title) || 'Unknown Title'}</h3>
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
                    <h3>${cleanTrackTitle(track.title) || 'Unknown Title'}</h3>
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
window.openYouTubeVideo = openYouTubeVideo;
window.openYouTubeModal = openYouTubeModal;
window.openClearYouTubeModal = openClearYouTubeModal;
window.parseYouTubeVideoId = parseYouTubeVideoId;
window.openDeleteAlbumModal = openDeleteAlbumModal;
window.closeDeleteAlbumModal = closeDeleteAlbumModal;
window.confirmDeleteAlbum = confirmDeleteAlbum;

let currentTrackIdForYouTube = null;
let currentTrackTitleForClear = null;
let currentAlbumIdForDelete = null;
let currentAlbumTitleForDelete = null;
let currentAlbumArtistForDelete = null;

function openYouTubeVideo(videoId) {
    if (videoId) {
        window.open('https://www.youtube.com/watch?v=' + videoId, '_blank');
    }
}

function openYouTubeModal(trackId) {
    currentTrackIdForYouTube = trackId;
    const modal = document.getElementById('youtube-modal');
    const input = document.getElementById('youtube-url-input');
    const errorEl = document.getElementById('youtube-modal-error');
    
    if (modal && input && errorEl) {
        input.value = '';
        errorEl.textContent = '';
        modal.style.display = 'flex';
        input.focus();
    }
}

function openClearYouTubeModal(trackId, trackTitle) {
    currentTrackIdForYouTube = trackId;
    currentTrackTitleForClear = trackTitle;
    const modal = document.getElementById('youtube-clear-modal');
    const titleEl = document.getElementById('clear-youtube-track-title');
    const errorEl = document.getElementById('youtube-clear-modal-error');
    
    if (modal && titleEl && errorEl) {
        titleEl.textContent = trackTitle || 'this track';
        errorEl.textContent = '';
        modal.style.display = 'flex';
    }
}

function closeYouTubeModal() {
    const modal = document.getElementById('youtube-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    currentTrackIdForYouTube = null;
}

function closeClearYouTubeModal() {
    const modal = document.getElementById('youtube-clear-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    currentTrackIdForYouTube = null;
    currentTrackTitleForClear = null;
}

function openDeleteAlbumModal(albumId, albumTitle, albumArtist) {
    currentAlbumIdForDelete = albumId;
    currentAlbumTitleForDelete = albumTitle;
    currentAlbumArtistForDelete = albumArtist;
    
    const modal = document.getElementById('album-delete-modal');
    const previewDiv = document.getElementById('album-delete-preview');
    const errorEl = document.getElementById('album-delete-modal-error');
    
    if (modal && previewDiv) {
        previewDiv.innerHTML = '<p>Loading preview...</p>';
        errorEl.textContent = '';
        modal.style.display = 'flex';
        
        // Fetch delete preview
        fetch('/albums/' + albumId + '/delete-preview')
            .then(response => response.json())
            .then(data => {
                if (data.error) {
                    previewDiv.innerHTML = '<p class="modal-error">Error: ' + escapeHtml(data.error) + '</p>';
                    return;
                }
                
                let playlistsHtml = '';
                if (data.impacted_playlists && data.impacted_playlists.length > 0) {
                    playlistsHtml = '<div style="margin-top: 1rem;"><p><strong>Will be removed from these playlists:</strong></p><ul style="margin: 0.5rem 0; padding-left: 1.5rem;">';
                    data.impacted_playlists.forEach(playlist => {
                        playlistsHtml += '<li>' + escapeHtml(playlist.playlist_name) + ' (' + playlist.track_count + ' tracks)</li>';
                    });
                    playlistsHtml += '</ul></div>';
                }
                
                previewDiv.innerHTML = 
                    '<p>Are you sure you want to delete <strong>' + escapeHtml(data.album.title) + '</strong> by <strong>' + escapeHtml(data.album.artist) + '</strong>?</p>' +
                    '<p style="margin-top: 0.5rem;">This will delete <strong>' + data.track_count + ' tracks</strong> and all associated data (YouTube matches, duration sources, etc.).</p>' +
                    playlistsHtml +
                    '<p style="margin-top: 1rem; color: #dc3545;"><strong>This action cannot be undone.</strong></p>';
            })
            .catch(error => {
                console.error('Error loading delete preview:', error);
                previewDiv.innerHTML = '<p class="modal-error">Failed to load preview</p>';
            });
    }
}

function closeDeleteAlbumModal() {
    const modal = document.getElementById('album-delete-modal');
    if (modal) {
        modal.style.display = 'none';
    }
    currentAlbumIdForDelete = null;
    currentAlbumTitleForDelete = null;
    currentAlbumArtistForDelete = null;
}

function confirmDeleteAlbum() {
    if (!currentAlbumIdForDelete) {
        return;
    }
    
    const errorEl = document.getElementById('album-delete-modal-error');
    const confirmBtn = document.getElementById('confirm-delete-album');
    
    confirmBtn.disabled = true;
    confirmBtn.textContent = 'Deleting...';
    
    fetch('/albums/' + currentAlbumIdForDelete + '?confirmed=true', {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json',
        }
    })
    .then(response => response.json())
    .then(data => {
        confirmBtn.disabled = false;
        confirmBtn.textContent = 'Delete Album';
        
        if (data.error) {
            errorEl.textContent = data.error;
            return;
        }
        
        closeDeleteAlbumModal();
        
        // Refresh albums list
        if (pagination.album.query) {
            searchAlbums(pagination.album.query);
        } else {
            loadAlbums();
        }
    })
    .catch(error => {
        confirmBtn.disabled = false;
        confirmBtn.textContent = 'Delete Album';
        console.error('Error deleting album:', error);
        errorEl.textContent = 'Failed to delete album';
    });
}

function parseYouTubeVideoId(url) {
    if (!url || typeof url !== 'string') {
        return null;
    }
    
    const patterns = [
        /(?:youtube\.com\/(?:[^\/]+\/.+\/|(?:v|e(?:mbed)?)\/|.*[?&]v=)|youtu\.be\/)([^"&?\/\s]{11})/,
        /^([a-zA-Z0-9_-]{11})$/
    ];
    
    for (const pattern of patterns) {
        const match = url.match(pattern);
        if (match && match[1]) {
            return match[1];
        }
    }
    
    return null;
}

function saveYouTubeUrl() {
    const input = document.getElementById('youtube-url-input');
    const errorEl = document.getElementById('youtube-modal-error');
    
    if (!input || !errorEl || !currentTrackIdForYouTube) {
        return;
    }
    
    const url = input.value.trim();
    const videoId = parseYouTubeVideoId(url);
    
    if (!videoId) {
        errorEl.textContent = 'Please enter a valid YouTube URL';
        return;
    }
    
    fetch('/tracks/' + currentTrackIdForYouTube + '/youtube', {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ youtube_url: url })
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            errorEl.textContent = data.error;
            return;
        }
        
        closeYouTubeModal();
        
        if (pagination.track.query) {
            searchTracks(pagination.track.query);
        } else {
            loadTracks();
        }
    })
    .catch(error => {
        console.error('Error saving YouTube URL:', error);
        errorEl.textContent = 'Failed to save YouTube URL';
    });
}

function confirmClearYouTube() {
    if (!currentTrackIdForYouTube) {
        return;
    }
    
    const errorEl = document.getElementById('youtube-clear-modal-error');
    
    fetch('/tracks/' + currentTrackIdForYouTube + '/youtube', {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json',
        }
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            errorEl.textContent = data.error;
            return;
        }
        
        closeClearYouTubeModal();
        
        if (pagination.track.query) {
            searchTracks(pagination.track.query);
        } else {
            loadTracks();
        }
    })
    .catch(error => {
        console.error('Error clearing YouTube URL:', error);
        errorEl.textContent = 'Failed to clear YouTube URL';
    });
}

document.addEventListener('DOMContentLoaded', function() {
    console.log('DOMContentLoaded fired');
    const modal = document.getElementById('youtube-modal');
    const clearModal = document.getElementById('youtube-clear-modal');
    const closeBtn = modal ? modal.querySelector('.close-modal') : null;
    const clearCloseBtn = clearModal ? clearModal.querySelector('.close-modal') : null;
    const cancelBtn = document.getElementById('cancel-youtube-url');
    const saveBtn = document.getElementById('save-youtube-url');
    const input = document.getElementById('youtube-url-input');
    const clearCancelBtn = document.getElementById('cancel-clear-youtube');
    const clearConfirmBtn = document.getElementById('confirm-clear-youtube');
    
    console.log('Modal elements:', { modal: !!modal, closeBtn: !!closeBtn, cancelBtn: !!cancelBtn, saveBtn: !!saveBtn, input: !!input });
    
    if (closeBtn) {
        closeBtn.addEventListener('click', closeYouTubeModal);
    }
    
    if (cancelBtn) {
        cancelBtn.addEventListener('click', closeYouTubeModal);
    }
    
    if (saveBtn) {
        saveBtn.addEventListener('click', saveYouTubeUrl);
    }
    
    if (input) {
        input.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                saveYouTubeUrl();
            }
        });
    }
    
    if (modal) {
        modal.addEventListener('click', function(e) {
            if (e.target === modal) {
                closeYouTubeModal();
            }
        });
    }
    
    if (clearCloseBtn) {
        clearCloseBtn.addEventListener('click', closeClearYouTubeModal);
    }
    
    if (clearCancelBtn) {
        clearCancelBtn.addEventListener('click', closeClearYouTubeModal);
    }
    
    if (clearConfirmBtn) {
        clearConfirmBtn.addEventListener('click', confirmClearYouTube);
    }
    
    if (clearModal) {
        clearModal.addEventListener('click', function(e) {
            if (e.target === clearModal) {
                closeClearYouTubeModal();
            }
        });
    }
    
    // Album delete modal event listeners
    const deleteAlbumModal = document.getElementById('album-delete-modal');
    const deleteAlbumCloseBtn = document.getElementById('close-album-delete-modal');
    const deleteAlbumCancelBtn = document.getElementById('cancel-delete-album');
    const deleteAlbumConfirmBtn = document.getElementById('confirm-delete-album');
    
    if (deleteAlbumCloseBtn) {
        deleteAlbumCloseBtn.addEventListener('click', closeDeleteAlbumModal);
    }
    
    if (deleteAlbumCancelBtn) {
        deleteAlbumCancelBtn.addEventListener('click', closeDeleteAlbumModal);
    }
    
    if (deleteAlbumConfirmBtn) {
        deleteAlbumConfirmBtn.addEventListener('click', confirmDeleteAlbum);
    }
    
    if (deleteAlbumModal) {
        deleteAlbumModal.addEventListener('click', function(e) {
            if (e.target === deleteAlbumModal) {
                closeDeleteAlbumModal();
            }
        });
    }
});
