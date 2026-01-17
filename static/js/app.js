// Basic frontend functionality
document.addEventListener('DOMContentLoaded', function() {
    console.log('Vinylfo frontend loaded');

    // Pagination state
    window.albumPagination = { page: 1, limit: 25, totalPages: 1, query: '' };
    window.trackPagination = { page: 1, limit: 25, totalPages: 1, query: '' };
    
    // Add helper function for cleaning album titles
    window.cleanAlbumTitle = function(albumTitle, trackTitle) {
        if (!albumTitle) return 'Unknown Album';
        
        if (albumTitle.includes(' / ') && albumTitle.includes(trackTitle)) {
            const parts = albumTitle.split(' / ');
            return parts[parts.length - 1].trim();
        }
        
        return albumTitle;
    };

    // Pagination event listeners
    document.getElementById('album-limit').addEventListener('change', function() {
        window.albumPagination.limit = parseInt(this.value);
        window.albumPagination.page = 1;
        if (window.albumPagination.query) {
            searchAlbums(window.albumPagination.query);
        } else {
            loadAlbums();
        }
    });

    document.getElementById('track-limit').addEventListener('change', function() {
        window.trackPagination.limit = parseInt(this.value);
        window.trackPagination.page = 1;
        if (window.trackPagination.query) {
            searchTracks(window.trackPagination.query);
        } else {
            loadTracks();
        }
    });

    document.getElementById('album-prev').addEventListener('click', function() {
        if (window.albumPagination.page > 1) {
            window.albumPagination.page--;
            if (window.albumPagination.query) {
                searchAlbums(window.albumPagination.query);
            } else {
                loadAlbums();
            }
        }
    });

    document.getElementById('album-next').addEventListener('click', function() {
        if (window.albumPagination.page < window.albumPagination.totalPages) {
            window.albumPagination.page++;
            if (window.albumPagination.query) {
                searchAlbums(window.albumPagination.query);
            } else {
                loadAlbums();
            }
        }
    });

    document.getElementById('track-prev').addEventListener('click', function() {
        if (window.trackPagination.page > 1) {
            window.trackPagination.page--;
            if (window.trackPagination.query) {
                searchTracks(window.trackPagination.query);
            } else {
                loadTracks();
            }
        }
    });

    document.getElementById('track-next').addEventListener('click', function() {
        if (window.trackPagination.page < window.trackPagination.totalPages) {
            window.trackPagination.page++;
            if (window.trackPagination.query) {
                searchTracks(window.trackPagination.query);
            } else {
                loadTracks();
            }
        }
    });

    // Navigation
    const navLinks = document.querySelectorAll('nav a');
    navLinks.forEach(link => {
        link.addEventListener('click', function(e) {
            const href = this.getAttribute('href');

            // Handle hash-based navigation - let the hashchange event handle it naturally
            if (href.startsWith('#')) {
                // Allow default behavior but prevent full page navigation for same-page hashes
                const targetHash = href.substring(1);
                const viewElement = document.getElementById(`${targetHash}-view`);
                if (viewElement) {
                    e.preventDefault();
                    window.location.hash = targetHash;
                }
                return;
            }

            // Handle external routes (paths starting with /)
            if (href.startsWith('/') && href !== '/') {
                e.preventDefault();
                window.location.href = href;
                return;
            }
        });
    });

    // Handle back button for hash navigation
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

    // Handle initial hash on page load
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
        // No hash - show albums view by default
        document.querySelectorAll('.view').forEach(view => {
            view.style.display = 'none';
        });
        document.getElementById('albums-view').style.display = 'block';
        loadAlbums();
    }

    // Back to tracks button handler
    document.getElementById('back-to-tracks').addEventListener('click', function() {
        document.querySelectorAll('.view').forEach(view => {
            view.style.display = 'none';
        });
        document.getElementById('tracks-view').style.display = 'block';
        loadTracks();
    });

    // Load initial data
    loadAlbums();
});

// Helper function to format duration
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

function updatePaginationControls(type) {
    const pagination = type === 'album' ? window.albumPagination : window.trackPagination;
    const prevBtn = document.getElementById(`${type}-prev`);
    const nextBtn = document.getElementById(`${type}-next`);
    const pageInfo = document.getElementById(`${type}-page-info`);
    const limitSelect = document.getElementById(`${type}-limit`);

    if (limitSelect) {
        limitSelect.value = pagination.limit;
    }

    pageInfo.textContent = `Page ${pagination.page} of ${pagination.totalPages}`;
    prevBtn.disabled = pagination.page <= 1;
    nextBtn.disabled = pagination.page >= pagination.totalPages;
}

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
        infoDiv.innerHTML = '<h3>' + (album.title || 'Unknown Title') + '</h3><p>Artist: ' + (album.artist || 'Unknown Artist') + '</p><p>Year: ' + (album.release_year || 'Unknown Year') + '</p>';
        
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
                <p>${track.album_artist || 'Unknown Artist'}</p>
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

let albumSearchTimeout;
document.getElementById('album-search').addEventListener('input', function(e) {
    clearTimeout(albumSearchTimeout);
    const query = e.target.value.trim();
    albumSearchTimeout = setTimeout(() => {
        window.albumPagination.page = 1;
        if (query) {
            searchAlbums(query);
        } else {
            loadAlbums();
        }
    }, 300);
});

document.querySelector('#albums-view .search-clear').addEventListener('click', function() {
    const searchInput = document.getElementById('album-search');
    searchInput.value = '';
    window.albumPagination.page = 1;
    loadAlbums();
    searchInput.focus();
});

let trackSearchTimeout;
document.getElementById('track-search').addEventListener('input', function(e) {
    clearTimeout(trackSearchTimeout);
    const query = e.target.value.trim();
    trackSearchTimeout = setTimeout(() => {
        window.trackPagination.page = 1;
        if (query) {
            searchTracks(query);
        } else {
            loadTracks();
        }
    }, 300);
});

document.querySelector('#tracks-view .search-clear').addEventListener('click', function() {
    const searchInput = document.getElementById('track-search');
    searchInput.value = '';
    window.trackPagination.page = 1;
    loadTracks();
    searchInput.focus();
});

function searchAlbums(query) {
    window.albumPagination.query = query;
    const url = `/albums/search?q=${encodeURIComponent(query)}&page=${window.albumPagination.page}&limit=${window.albumPagination.limit}`;
    fetch(url)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                console.error('API error:', data.error);
                document.getElementById('albums-list').innerHTML = '<p>Error: ' + data.error + '</p>';
                return;
            }
            const albums = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
            window.albumPagination.totalPages = data.totalPages || 1;
            updatePaginationControls('album');
            renderAlbums(albums);
        })
        .catch(error => {
            console.error('Error searching albums:', error);
            document.getElementById('albums-list').innerHTML = '<p>Error searching albums</p>';
        });
}

function searchTracks(query) {
    window.trackPagination.query = query;
    const url = `/tracks/search?q=${encodeURIComponent(query)}&page=${window.trackPagination.page}&limit=${window.trackPagination.limit}`;
    fetch(url)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                console.error('API error:', data.error);
                document.getElementById('tracks-list').innerHTML = '<p>Error: ' + data.error + '</p>';
                return;
            }
            const tracks = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
            window.trackPagination.totalPages = data.totalPages || 1;
            updatePaginationControls('track');
            renderTracks(tracks);
        })
        .catch(error => {
            console.error('Error searching tracks:', error);
            document.getElementById('tracks-list').innerHTML = '<p>Error searching tracks</p>';
        });
}

function loadAlbums() {
    window.albumPagination.query = '';
    const url = `/albums?page=${window.albumPagination.page}&limit=${window.albumPagination.limit}`;
    fetch(url)
        .then(response => response.json())
        .then(data => {
            if (data.error) {
                console.error('API error:', data.error);
                document.getElementById('albums-list').innerHTML = '<p>Error: ' + data.error + '</p>';
                return;
            }
            const albums = Array.isArray(data.data) ? data.data : (Array.isArray(data) ? data : []);
            window.albumPagination.totalPages = data.totalPages || 1;
            updatePaginationControls('album');
            renderAlbums(albums);
        })
        .catch(error => {
            console.error('Error loading albums:', error);
            document.getElementById('albums-list').innerHTML = '<p>Error loading albums</p>';
        });
}

function loadTracks() {
    window.trackPagination.query = '';
    const url = `/tracks?page=${window.trackPagination.page}&limit=${window.trackPagination.limit}`;
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
            window.trackPagination.totalPages = data.totalPages || 1;
            updatePaginationControls('track');
            renderTracks(tracks);
        })
        .catch(error => {
            console.error('Error loading tracks:', error);
            document.getElementById('tracks-list').innerHTML = '<p>Error loading tracks</p>';
        });
}

// Load tracks for a specific album
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
                                <p style="margin: 0; color: #666;">${escapeHtml(album.artist || 'Unknown Artist')}</p>
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
                                <p>${track.album_artist || 'Unknown Artist'}</p>
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
    
    // Show tracks view
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
                    <p><strong>Artist:</strong> ${track.album_artist || 'Unknown Artist'}</p>
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
                item.innerHTML = `
                    <h3>Session ${session.id}</h3>
                    <p>Started: ${session.start_time || 'Unknown time'}</p>
                    <p>Duration: ${session.duration || 'Unknown duration'} seconds</p>
                `;
                list.appendChild(item);
            });
        })
        .catch(error => {
            console.error('Error loading sessions:', error);
            const list = document.getElementById('sessions-list');
            list.innerHTML = '<p>Error loading sessions</p>';
        });
}
