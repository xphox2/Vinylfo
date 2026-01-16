// Frontend JavaScript for Vinylfo music streaming platform

document.addEventListener('DOMContentLoaded', function() {
    // Navigation handling - now handled by app.js
    // This file is kept for compatibility but main navigation is in app.js

    // Show initial section - albums view is shown by default
    showSection('home');

    // Load data when switching to relevant sections
    const sections = document.querySelectorAll('section');
    sections.forEach(section => {
        section.addEventListener('show', function() {
            if (this.id === 'sessions') {
                loadSessions();
            } else if (this.id === 'playback') {
                loadPlaybackStatus();
            }
        });
    });
});

// Show a specific section
function showSection(sectionId) {
    // Hide all sections
    const sections = document.querySelectorAll('section');
    sections.forEach(section => {
        section.style.display = 'none';
    });
    
    // Show the requested section
    const section = document.getElementById(sectionId);
    if (section) {
        section.style.display = 'block';
        // Trigger data load if needed
        if (sectionId === 'sessions') {
            loadSessions();
        } else if (sectionId === 'playback') {
            loadPlaybackStatus();
        }
    }
}

// Load sessions data
function loadSessions() {
    const sessionList = document.getElementById('session-list');
    sessionList.innerHTML = '<div class="loading">Loading sessions...</div>';
    
    fetch('/sessions')
        .then(response => response.json())
        .then(data => {
            if (data && data.length > 0) {
                let html = '<div class="session-list">';
                data.forEach(session => {
                    html += `
                        <div class="session-item">
                            <h4>Session #${session.id}</h4>
                            <p>Album ID: ${session.album_id}</p>
                            <p>Track ID: ${session.track_id}</p>
                            <p>Position: ${session.position} seconds</p>
                            <p>Started: ${new Date(session.started_at).toLocaleString()}</p>
                            <p>Updated: ${new Date(session.updated_at).toLocaleString()}</p>
                        </div>
                    `;
                });
                html += '</div>';
                sessionList.innerHTML = html;
            } else {
                sessionList.innerHTML = '<p>No sessions found.</p>';
            }
        })
        .catch(error => {
            console.error('Error loading sessions:', error);
            sessionList.innerHTML = '<p>Error loading sessions. Please try again.</p>';
        });
}

// Load playback status
function loadPlaybackStatus() {
    const playbackStatus = document.getElementById('playback-status');
    playbackStatus.innerHTML = '<div class="loading">Loading playback status...</div>';
    
    fetch('/playback')
        .then(response => response.json())
        .then(data => {
            if (data) {
                playbackStatus.innerHTML = `
                    <div class="session-item">
                        <h4>Current Playback Status</h4>
                        <p>Session ID: ${data.id}</p>
                        <p>Album ID: ${data.album_id}</p>
                        <p>Track ID: ${data.track_id}</p>
                        <p>Position: ${data.position} seconds</p>
                        <p>Started: ${new Date(data.started_at).toLocaleString()}</p>
                        <p>Updated: ${new Date(data.updated_at).toLocaleString()}</p>
                    </div>
                `;
            } else {
                playbackStatus.innerHTML = '<p>No active playback session.</p>';
            }
        })
        .catch(error => {
            console.error('Error loading playback status:', error);
            playbackStatus.innerHTML = '<p>Error loading playback status. Please try again.</p>';
        });
}

// Playback control functions
function startPlayback() {
    fetch('/playback/start', { method: 'POST' })
        .then(response => response.json())
        .then(data => {
            alert('Playback started successfully');
            loadPlaybackStatus();
        })
        .catch(error => {
            console.error('Error starting playback:', error);
            alert('Error starting playback');
        });
}

function pausePlayback() {
    fetch('/playback/pause', { method: 'POST' })
        .then(response => response.json())
        .then(data => {
            alert('Playback paused successfully');
            loadPlaybackStatus();
        })
        .catch(error => {
            console.error('Error pausing playback:', error);
            alert('Error pausing playback');
        });
}

function stopPlayback() {
    fetch('/playback/stop', { method: 'POST' })
        .then(response => response.json())
        .then(data => {
            alert('Playback stopped successfully');
            loadPlaybackStatus();
        })
        .catch(error => {
            console.error('Error stopping playback:', error);
            alert('Error stopping playback');
        });
}

function skipTrack() {
    fetch('/playback/skip', { method: 'POST' })
        .then(response => response.json())
        .then(data => {
            alert('Skipped to next track successfully');
            loadPlaybackStatus();
        })
        .catch(error => {
            console.error('Error skipping track:', error);
            alert('Error skipping track');
        });
}

// Utility function to format duration
function formatDuration(seconds) {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    
    if (h > 0) {
        return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`;
    } else {
        return `${m}:${s.toString().padStart(2, '0')}`;
    }
}