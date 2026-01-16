// Basic frontend functionality
document.addEventListener('DOMContentLoaded', function() {
    console.log('Vinylfo frontend loaded');

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
        item.innerHTML = `
            <h3>${album.title || 'Unknown Title'}</h3>
            <p>Artist: ${album.artist || 'Unknown Artist'}</p>
            <p>Year: ${album.release_year || 'Unknown Year'}</p>
        `;
        item.addEventListener('click', function() {
            showTracksForAlbum(album.id);
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
        const item = document.createElement('div');
        item.className = 'track-item';
        item.innerHTML = `
            <h3>${track.title || 'Unknown Title'}</h3>
            <p>Artist: ${track.album_artist || 'Unknown Artist'}</p>
            <p>Album: ${track.album_title || 'Unknown Album'}</p>
            <p>Duration: ${formatDuration(track.duration) || 'Unknown Duration'}</p>
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
    loadAlbums();
    searchInput.focus();
});

let trackSearchTimeout;
document.getElementById('track-search').addEventListener('input', function(e) {
    clearTimeout(trackSearchTimeout);
    const query = e.target.value.trim();
    trackSearchTimeout = setTimeout(() => {
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
    loadTracks();
    searchInput.focus();
});

function searchAlbums(query) {
    fetch(`/albums/search?q=${encodeURIComponent(query)}`)
        .then(response => response.json())
        .then(data => renderAlbums(data))
        .catch(error => {
            console.error('Error searching albums:', error);
            document.getElementById('albums-list').innerHTML = '<p>Error searching albums</p>';
        });
}

function searchTracks(query) {
    fetch(`/tracks/search?q=${encodeURIComponent(query)}`)
        .then(response => response.json())
        .then(data => renderTracks(data))
        .catch(error => {
            console.error('Error searching tracks:', error);
            document.getElementById('tracks-list').innerHTML = '<p>Error searching tracks</p>';
        });
}

function loadAlbums() {
    fetch('/albums')
        .then(response => response.json())
        .then(data => renderAlbums(data))
        .catch(error => {
            console.error('Error loading albums:', error);
            document.getElementById('albums-list').innerHTML = '<p>Error loading albums</p>';
        });
}

// Load tracks for a specific album
function showTracksForAlbum(albumID) {
    fetch(`/albums/${albumID}/tracks`)
        .then(response => response.json())
        .then(data => {
            const list = document.getElementById('tracks-list');
            list.innerHTML = '';
            
            if (!data || data.length === 0) {
                list.innerHTML = '<p>No tracks found for this album.</p>';
                return;
            }
            
            data.forEach(track => {
                const item = document.createElement('div');
                item.className = 'track-item';
                item.innerHTML = `
                    <h3>${track.title || 'Unknown Title'}</h3>
                    <p>Artist: ${track.album_artist || 'Unknown Artist'}</p>
                    <p>Album: ${track.album_title || 'Unknown Album'}</p>
                    <p>Duration: ${formatDuration(track.duration) || 'Unknown Duration'}</p>
                `;
                item.addEventListener('click', function() {
                    window.location.href = '/track/' + track.id;
                });
                list.appendChild(item);
            });
            
            // Show tracks view
            document.querySelectorAll('.view').forEach(view => {
                view.style.display = 'none';
            });
            document.getElementById('tracks-view').style.display = 'block';
        })
        .catch(error => {
            console.error('Error loading tracks:', error);
            const list = document.getElementById('tracks-list');
            list.innerHTML = '<p>Error loading tracks</p>';
        });
}

function loadTracks() {
    fetch('/tracks')
        .then(response => response.json())
        .then(data => renderTracks(data))
        .catch(error => {
            console.error('Error loading tracks:', error);
            document.getElementById('tracks-list').innerHTML = '<p>Error loading tracks</p>';
        });
}

// Load track detail
function loadTrackDetail(trackId) {
    fetch('/tracks/' + trackId)
        .then(response => response.json())
        .then(track => {
            const detail = document.getElementById('track-detail');
            detail.innerHTML = `
                <div class="track-detail-item">
                    <h3>${track.title || 'Unknown Title'}</h3>
                    <p><strong>Artist:</strong> ${track.album_artist || 'Unknown Artist'}</p>
                    <p><strong>Album:</strong> ${track.album_title || 'Unknown Album'}</p>
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
