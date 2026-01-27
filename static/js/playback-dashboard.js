// Playback Dashboard JavaScript with Queue and Resume Support

import { normalizeArtistName, normalizeTitle } from './modules/utils.js';

document.addEventListener('DOMContentLoaded', function() {
    console.log('Playback dashboard loaded');

    const savedPlaylistId = localStorage.getItem('vinylfo_currentPlaylistId');
    if (savedPlaylistId) {
        console.log('[PlaybackManager] Found saved playlist ID:', savedPlaylistId);
    }

    window.playbackManager = new PlaybackManager();
    window.playbackManager.init();
});

function cleanAlbumTitle(albumTitle, trackTitle) {
    if (!albumTitle) return 'Unknown Album';

    if (albumTitle.includes(' / ') && albumTitle.includes(trackTitle)) {
        const parts = albumTitle.split(' / ');
        return parts[parts.length - 1].trim();
    }

    return albumTitle;
}

function cleanArtistName(artistName) {
    if (!artistName) return 'Unknown Artist';
    return normalizeArtistName(artistName) || 'Unknown Artist';
}

function cleanTrackTitle(trackTitle) {
    if (!trackTitle) return 'Unknown Track';
    return normalizeTitle(trackTitle) || 'Unknown Track';
}

// TabSyncManager is loaded from tab-sync-manager.js

class PlaybackManager {
    constructor() {
        this.queue = [];
        this.queueIndex = 0;
        this.currentPosition = 0;
        this.isPlaying = false;
        this.isPaused = false;
        this.isQueueVisible = false;
        this.saveInterval = null;
        this.autoResumeTimer = null;
        this.currentTrack = null;
        this.currentPlaylistId = null;
        this.currentPlaylistName = null;
        this.tabSync = new TabSyncManager();
        this.syncInterval = null;
        this.isLocalChange = false;
        this.serverTimeOffset = 0;
        this.lastServerSync = 0;
        this.autoPlayEnabled = false;
        this.trackEndedHandler = null;
        this.queueCurrentPage = 1;
        this.queueItemsPerPage = 25;
        this.stateSyncBlocked = false;
        this.useYouTubeDuration = false;
        this.currentYouTubeDuration = 0;
        this.lastServerPosition = 0;
        this.lastServerTimestamp = 0;
        this.positionUpdateListenerAdded = false;
        this.forcePositionResync = false;
        this.lastTickDebug = 0;
        this.lastSyncDebug = 0;
        this.lastSaveDebug = 0;
        this.pendingSeekRevision = null;
    }

    async init() {
        console.log('[PlaybackManager] Initializing...');

        // Load current playback status from server
        await this.loadCurrentPlayback();

        this.setupControls();
        this.setupTabSyncListeners();
        this.setupPositionUpdateListener();
        this.setupVisibilityHandlers();
        this.startStatusUpdate();
        this.setupBeforeUnload();
        this.startPeriodicStateSync();
        console.log('[PlaybackManager] Initialization complete');
    }

    setupVisibilityHandlers() {
        // Browsers throttle timers/network in background tabs/windows.
        // When we become visible again, snap UI back to the server's authoritative position/state.
        document.addEventListener('visibilitychange', () => {
            if (!document.hidden) {
                this.forcePositionResync = true;
                this.syncStateFromServer();
            }
        });

        // Some browsers resume without firing visibilitychange in certain flows.
        window.addEventListener('focus', () => {
            this.forcePositionResync = true;
            this.syncStateFromServer();
        });
    }

    async loadCurrentPlayback() {
        try {
            let url = '/playback/current';
            const savedPlaylistId = localStorage.getItem('vinylfo_currentPlaylistId');
            if (savedPlaylistId) {
                url = `/playback/current?playlist_id=${encodeURIComponent(savedPlaylistId)}`;
                console.log('[PlaybackManager] Loading playback for saved playlist:', savedPlaylistId);
            }

            const response = await fetch(url);
            const data = await response.json();
            
            console.log('[PlaybackManager] loadCurrentPlayback response:', JSON.stringify(data, null, 2));
            
            if (data.track && data.playlist_id) {
                this.currentPlaylistId = data.playlist_id;
                this.currentPlaylistName = data.playlist_name || null;
                this.currentTrack = data.track;
                this.currentYouTubeDuration = data.track.youtube_video_duration || 0;
                
                console.log('[PlaybackManager] Initial load track:', {
                    title: data.track.title,
                    duration: data.track.duration,
                    youtube_video_duration: data.track.youtube_video_duration,
                    youtube_video_id: data.track.youtube_video_id
                });
                
                document.getElementById('track-title').textContent = cleanTrackTitle(data.track.title) || 'Unknown Track';
                document.getElementById('track-artist').textContent = cleanArtistName(data.track.album_artist);
                document.getElementById('track-album').textContent = 'Album: ' + cleanAlbumTitle(data.track.album_title, data.track.title);
                document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(this.getEffectiveDuration());
                document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(data.position || 0);
                document.getElementById('progress-bar').style.width = '0%';
                this.updateDurationComparison();
                
                const playBtn = document.getElementById('play-btn');
                const pauseBtn = document.getElementById('pause-btn');
                
                this.isPlaying = data.is_playing;
                this.isPaused = data.is_paused;
                
                if (this.isPlaying) {
                    playBtn.disabled = true;
                    pauseBtn.disabled = false;
                    this.startProgressSaving();
                } else if (this.isPaused) {
                    playBtn.disabled = false;
                    pauseBtn.disabled = true;
                } else {
                    playBtn.disabled = false;
                    pauseBtn.disabled = true;
                }
                
                if (data.queue && data.queue.length > 0) {
                    console.log('[PlaybackManager] Queue received, count:', data.queue.length);
                    console.log('[PlaybackManager] First queue track:', data.queue[0]);
                    this.queue = data.queue;
                    this.queueIndex = data.queue_index || 0;
                    console.log('[PlaybackManager] Queue first 3:', this.queue.slice(0, 3).map(t => ({ title: t.title, yt_duration: t.youtube_video_duration })));
                    this.renderQueue();
                } else {
                    console.log('[PlaybackManager] No queue in response');
                    // Check for saved queue from restore
                    const savedQueue = localStorage.getItem('vinylfo_queue');
                    const savedQueueIndex = localStorage.getItem('vinylfo_queueIndex');
                    if (savedQueue) {
                        try {
                            this.queue = JSON.parse(savedQueue);
                            this.queueIndex = parseInt(savedQueueIndex) || 0;
                            console.log('[PlaybackManager] Restored queue from localStorage, count:', this.queue.length);
                            this.renderQueue();
                            // Clear saved queue
                            localStorage.removeItem('vinylfo_queue');
                            localStorage.removeItem('vinylfo_queueIndex');
                        } catch (e) {
                            console.error('[PlaybackManager] Error parsing saved queue:', e);
                        }
                    }
                }

                // Restore queue position
                const savedPosition = localStorage.getItem('vinylfo_queuePosition');
                if (savedPosition) {
                    this.currentPosition = parseInt(savedPosition) || 0;
                    console.log('[PlaybackManager] Restored queue position:', this.currentPosition);
                    this.updatePositionDisplay();
                    localStorage.removeItem('vinylfo_queuePosition');
                } else if (data.queue_position !== undefined) {
                    this.currentPosition = data.queue_position;
                    this.updatePositionDisplay();
                }

                // Initialize wall-clock timer for resume
                if (this.isPlaying) {
                    this.lastWallClockUpdate = Date.now();
                    this.cachedPosition = this.currentPosition;
                }

                // Store server revision for ordering
                this.lastAppliedRevision = data.revision || 0;

                const playlistInfo = document.getElementById('playlist-info');
                if (playlistInfo && this.currentPlaylistName) {
                    playlistInfo.textContent = 'Playlist: ' + this.currentPlaylistName;
                    playlistInfo.style.display = 'block';
                } else if (playlistInfo) {
                    playlistInfo.style.display = 'none';
                }
            } else {
                console.log('[PlaybackManager] No track currently playing');
            }
        } catch (error) {
            console.error('[PlaybackManager] Error loading playback:', error);
        }
    }

    startPeriodicStateSync() {
        this.stateSyncInterval = setInterval(() => {
            this.syncStateFromServer();
        }, 2000);
    }

    async syncStateFromServer() {
        if (this.stateSyncBlocked) {
            return;
        }

        try {
            const url = this.currentPlaylistId
                ? `/playback/current?playlist_id=${encodeURIComponent(this.currentPlaylistId)}`
                : '/playback/current';
            const response = await fetch(url);
            const data = await response.json();

            const serverRevision = data.revision || 0;
            if (serverRevision <= this.lastAppliedRevision) {
                return;
            }
            this.lastAppliedRevision = serverRevision;
            
            // Track/state/position are authoritative from the server.
            // This ensures the UI stays correct even if the tab was backgrounded.
            if (data.playlist_id) {
                this.currentPlaylistId = data.playlist_id;
            }
            if (data.playlist_name) {
                this.currentPlaylistName = data.playlist_name;
            }
            if (data.track) {
                if (!this.currentTrack || data.track.id !== this.currentTrack.id) {
                    this.displayTrack(data.track, data.playlist_name);
                }
            }

            if (typeof data.is_playing === 'boolean') {
                this.isPlaying = data.is_playing;
            }
            if (typeof data.is_paused === 'boolean') {
                this.isPaused = data.is_paused;
            }
            this.updateButtonStates();

            if (typeof data.position === 'number' && !Number.isNaN(data.position)) {
                const serverPosition = data.position;
                const localPosition = this.currentPosition;
                const drift = serverPosition - localPosition;
                const driftSeconds = Math.abs(drift);
                const now = Date.now();

                if (serverPosition < localPosition && driftSeconds >= 1 && now - this.lastSyncDebug >= 2000) {
                    console.log('[TimeDebug] Server position behind local', {
                        serverPosition,
                        localPosition,
                        driftSeconds,
                        forcePositionResync: this.forcePositionResync,
                        isPlaying: this.isPlaying,
                        isPaused: this.isPaused
                    });
                    this.lastSyncDebug = now;
                }

                const shouldResyncForward = drift >= 2;
                const shouldResyncBackward = drift <= -2 && this.forcePositionResync;

                // Avoid backward jitter while visible; allow backward resync only when forced.
                if (shouldResyncForward || shouldResyncBackward) {
                    this.currentPosition = serverPosition;
                    this.cachedPosition = this.currentPosition;
                    this.lastWallClockUpdate = Date.now();
                    this.updatePositionDisplay();
                    console.log('[TimeDebug] Resynced position from server', {
                        serverPosition,
                        localPositionBefore: localPosition,
                        drift,
                        driftSeconds,
                        forcePositionResync: this.forcePositionResync
                    });
                }

                this.forcePositionResync = false;
            }

            if (data.queue && data.queue.length > 0) {
                const serverQueueIds = data.queue.map(t => t.id).join(',');
                const localQueueIds = this.queue.map(t => t.id).join(',');
                
                const queueChanged = serverQueueIds !== localQueueIds;
                const indexChanged = this.queueIndex !== data.queue_index;
                
                if (queueChanged || indexChanged) {
                    this.queue = data.queue;
                    if (indexChanged && this.queueIndex === data.queue_index) {
                        this.queueIndex = data.queue_index || 0;
                    }
                    this.renderQueue();
                }
            }
        } catch (error) {
            console.error('[PlaybackManager] Error syncing state:', error);
        }
    }

    setupTabSyncListeners() {
        console.log('[PlaybackManager] Setting up tab sync listeners');
        
        window.addEventListener('vinylfo_state_update', (e) => {
            console.log('[PlaybackManager] Received state_update event');
            if (!this.isLocalChange) {
                this.applyRemoteState(e.detail);
            }
            this.isLocalChange = false;
        });

        window.addEventListener('vinylfo_play', (e) => {
            console.log('[PlaybackManager] Received play event');
            if (!this.isLocalChange) {
                this.applyRemotePlay(e.detail);
            }
            this.isLocalChange = false;
        });

        window.addEventListener('vinylfo_pause', () => {
            console.log('[PlaybackManager] Received pause event');
            if (!this.isLocalChange) {
                this.applyRemotePause();
            }
            this.isLocalChange = false;
        });

        window.addEventListener('vinylfo_stop', () => {
            console.log('[PlaybackManager] Received stop event');
            if (!this.isLocalChange) {
                this.applyRemoteStop();
            }
            this.isLocalChange = false;
        });

        window.addEventListener('vinylfo_skip', (e) => {
            console.log('[PlaybackManager] Received skip event');
            if (!this.isLocalChange) {
                this.applyRemoteSkip(e.detail);
            }
            this.isLocalChange = false;
        });

        window.addEventListener('vinylfo_seek', (e) => {
            console.log('[PlaybackManager] Received seek event');
            if (!this.isLocalChange) {
                this.applyRemoteSeek(e.detail.position);
            }
            this.isLocalChange = false;
        });
    }

    setupPositionUpdateListener() {
        if (this.positionUpdateListenerAdded) {
            return;
        }
        this.positionUpdateListenerAdded = true;

        window.addEventListener('vinylfo_position_update', (e) => {
            const { position, timestamp } = e.detail;
            if (typeof position === 'number' && typeof timestamp === 'number') {
                if (position < this.currentPosition) {
                    console.log('[TimeDebug] Position update behind local', {
                        position,
                        localPosition: this.currentPosition,
                        timestamp
                    });
                }
                this.lastServerPosition = position;
                this.lastServerTimestamp = timestamp;
                if (!this.isLocalChange) {
                    this.currentPosition = position;
                    this.updatePositionDisplay();
                }
            }
        });
    }

    applyRemoteState(state) {
        console.log('[PlaybackManager] Applying remote state:', state);
        if (state.track) {
            this.displayTrack(state.track, state.playlist_name);
        }
        if (state.queue) {
            this.queue = state.queue;
            this.queueIndex = state.queue_index || 0;
            this.renderQueue();
        }
        this.isPlaying = state.is_playing;
        this.isPaused = state.is_paused;
        this.updateButtonStates();
        
        // Sync auto-play preference
        if (state.auto_play_enabled !== undefined) {
            this.autoPlayEnabled = state.auto_play_enabled;
            const autoPlayBtn = document.getElementById('auto-play-btn');
            if (autoPlayBtn) {
                autoPlayBtn.textContent = this.autoPlayEnabled ? 'Auto-Play: ON' : 'Auto-Play: OFF';
                autoPlayBtn.classList.toggle('active', this.autoPlayEnabled);
            }
        }
    }

    applyRemotePlay(data) {
        console.log('[PlaybackManager] Applying remote play:', data);
        if (data.track) {
            this.displayTrack(data.track);
        }
        this.currentPosition = data.position || 0;
        this.lastWallClockUpdate = Date.now();
        this.cachedPosition = this.currentPosition;
        this.updatePositionDisplay();
        this.isPlaying = true;
        this.isPaused = false;
        this.startProgressSaving();
        this.updateButtonStates();
    }

    applyRemotePause() {
        console.log('[PlaybackManager] Applying remote pause');
        this.isPaused = true;
        this.isPlaying = false;
        this.stopProgressSaving();
        this.updateButtonStates();
    }

    applyRemoteStop() {
        console.log('[PlaybackManager] Applying remote stop');
        this.isPlaying = false;
        this.isPaused = false;
        this.stopProgressSaving();
        this.updateButtonStates();
    }

    applyRemoteSkip(data) {
        console.log('[PlaybackManager] applyRemoteSkip received:', JSON.stringify(data, null, 2));
        
        if (data.queue_index !== undefined) {
            this.queueIndex = data.queue_index;
        }
        
        if (data.queue) {
            this.queue = data.queue;
        }
        
        let trackToPlay = data.track;
        console.log('[PlaybackManager] applyRemoteSkip - data.track:', trackToPlay);
        
        if (!trackToPlay && this.queue && this.queue.length > 0 && this.queueIndex < this.queue.length) {
            trackToPlay = this.queue[this.queueIndex];
            console.log('[PlaybackManager] applyRemoteSkip - fell back to queue, trackToPlay:', trackToPlay);
        }
        
        if (trackToPlay) {
            this.currentTrack = trackToPlay;
            this.currentYouTubeDuration = trackToPlay.youtube_video_duration || 0;
            console.log('[PlaybackManager] applyRemoteSkip - updating UI with track:', trackToPlay.title);
            
            document.getElementById('track-title').textContent = trackToPlay.title || 'Unknown Track';
            document.getElementById('track-artist').textContent = cleanArtistName(trackToPlay.album_artist);
            document.getElementById('track-album').textContent = 'Album: ' + cleanAlbumTitle(trackToPlay.album_title, trackToPlay.title);
            document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(this.getEffectiveDuration());
            document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
            document.getElementById('progress-bar').style.width = '0%';
            this.updateDurationComparison();
        } else {
            console.log('[PlaybackManager] applyRemoteSkip - NO TRACK to play!');
        }
        
        if (data.queue) {
            this.renderQueue();
        }

        this.currentPosition = 0;
        this.lastWallClockUpdate = Date.now();
        this.cachedPosition = this.currentPosition;
        this.updatePositionDisplay();
        this.isPlaying = true;
        this.isPaused = false;
        this.startProgressSaving();
        this.updateButtonStates();
    }

    applyRemoteSeek(position) {
        console.log('[PlaybackManager] Applying remote seek to:', position);
        this.setPositionFromAuthoritative(position);
    }

    setPositionFromAuthoritative(position) {
        this.currentPosition = position;
        this.cachedPosition = position;
        this.lastWallClockUpdate = Date.now();
        this.updatePositionDisplay();
    }

    broadcastState() {
        console.log('[PlaybackManager] Broadcasting state');
        this.tabSync.broadcastStateUpdate({
            track: this.currentTrack,
            playlist_name: document.getElementById('playlist-info')?.textContent?.replace('Playlist: ', ''),
            queue: this.queue,
            queue_index: this.queueIndex,
            is_playing: this.isPlaying,
            is_paused: this.isPaused
        });
    }

    setupControls() {
        // Play button
        document.getElementById('play-btn').addEventListener('click', () => {
            this.play();
        });

        // Pause button
        document.getElementById('pause-btn').addEventListener('click', () => {
            this.pause();
        });

        // Previous button
        document.getElementById('previous-btn').addEventListener('click', () => {
            this.previous();
        });

        // Next button
        document.getElementById('next-btn').addEventListener('click', () => {
            this.next();
        });

        // Stop button
        document.getElementById('stop-btn').addEventListener('click', () => {
            this.stop();
        });

        // Queue toggle button
        const queueToggle = document.getElementById('toggle-queue-btn');
        if (queueToggle) {
            queueToggle.addEventListener('click', () => {
                this.toggleQueue();
            });
        }

        // Progress bar click to seek
        const progressContainer = document.querySelector('.progress-container');
        if (progressContainer) {
            progressContainer.addEventListener('click', (e) => {
                this.seekToPosition(e);
            });
        }

        // Auto-play toggle button
        const autoPlayBtn = document.getElementById('auto-play-btn');
        if (autoPlayBtn) {
            autoPlayBtn.addEventListener('click', () => {
                this.autoPlayEnabled = !this.autoPlayEnabled;
                autoPlayBtn.textContent = this.autoPlayEnabled ? 'Auto-Play: ON' : 'Auto-Play: OFF';
                autoPlayBtn.classList.toggle('active', this.autoPlayEnabled);
                localStorage.setItem('autoPlayEnabled', this.autoPlayEnabled);
                
                // Broadcast auto-play state change
                this.isLocalChange = true;
                this.tabSync.broadcastStateUpdate({
                    track: this.currentTrack,
                    playlist_name: document.getElementById('playlist-info')?.textContent?.replace('Playlist: ', ''),
                    queue: this.queue,
                    queue_index: this.queueIndex,
                    is_playing: this.isPlaying,
                    is_paused: this.isPaused,
                    auto_play_enabled: this.autoPlayEnabled
                });
            });
            
            // Restore auto-play preference
            const savedAutoPlay = localStorage.getItem('autoPlayEnabled');
            if (savedAutoPlay === 'true') {
                this.autoPlayEnabled = true;
                autoPlayBtn.textContent = 'Auto-Play: ON';
                autoPlayBtn.classList.add('active');
            }
        }
        
        // YouTube duration toggle button
        const useYouTubeDurationCheckbox = document.getElementById('use-youtube-duration');
        if (useYouTubeDurationCheckbox) {
            // Restore preference
            const savedUseYouTubeDuration = localStorage.getItem('useYouTubeDuration');
            if (savedUseYouTubeDuration === 'true') {
                this.useYouTubeDuration = true;
                useYouTubeDurationCheckbox.checked = true;
            }
            
            useYouTubeDurationCheckbox.addEventListener('change', () => {
                this.useYouTubeDuration = useYouTubeDurationCheckbox.checked;
                localStorage.setItem('useYouTubeDuration', this.useYouTubeDuration);
                this.updateDurationComparison();
            });
        }
        
        // YouTube duration refresh button
        const refreshDurationBtn = document.getElementById('refresh-duration-btn');
        if (refreshDurationBtn) {
            refreshDurationBtn.addEventListener('click', () => this.refreshYouTubeDuration());
        }
        
        // Queue pagination buttons
        const prevPageBtn = document.getElementById('prev-page-btn');
        const nextPageBtn = document.getElementById('next-page-btn');
        
        if (prevPageBtn) {
            prevPageBtn.addEventListener('click', () => {
                if (this.queueCurrentPage > 1) {
                    this.queueCurrentPage--;
                    this.renderQueue();
                    this.updatePaginationControls();
                }
            });
        }
        
        if (nextPageBtn) {
            nextPageBtn.addEventListener('click', () => {
                const totalPages = Math.ceil(this.queue.length / this.queueItemsPerPage);
                if (this.queueCurrentPage < totalPages) {
                    this.queueCurrentPage++;
                    this.renderQueue();
                    this.updatePaginationControls();
                }
            });
        }
    }

    seekToPosition(event) {
        const effectiveDuration = this.getEffectiveDuration();
        if (!this.currentTrack || !effectiveDuration) return;

        const progressContainer = event.currentTarget;
        const rect = progressContainer.getBoundingClientRect();
        const clickX = event.clientX - rect.left;
        const containerWidth = rect.width;

        const percentage = clickX / containerWidth;
        const newPosition = Math.floor(percentage * effectiveDuration);

        console.log('[PlaybackManager] Seeking to position:', newPosition);

        this.setPositionFromAuthoritative(newPosition);

        this.isLocalChange = true;
        this.tabSync.broadcastSeek(newPosition);

        fetch('/playback/seek', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                playlist_id: this.currentPlaylistId,
                position_seconds: newPosition
            })
        }).then(response => response.json())
        .then(data => {
            if (data.revision) {
                this.lastAppliedRevision = data.revision;
            }
        }).catch(error => {
            console.error('[PlaybackManager] Seek failed:', error);
            this.syncStateFromServer();
        });
    }

    async play() {
        console.log('[PlaybackManager] Play button clicked');
        try {
            const response = await fetch('/playback/resume', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ playlist_id: this.currentPlaylistId })
            });
            if (response.ok) {
                const data = await response.json();
                this.isPlaying = data.is_playing;
                this.isPaused = data.is_paused;
                if (data.revision) {
                    this.lastAppliedRevision = data.revision;
                }
                this.lastWallClockUpdate = Date.now();
                this.cachedPosition = this.currentPosition;
                this.startProgressSaving();
                this.updateButtonStates();

                console.log('[PlaybackManager] Broadcasting play');
                this.isLocalChange = true;
                this.tabSync.broadcastPlay(this.currentTrack, this.currentPosition);
            }
        } catch (error) {
            console.error('Error resuming playback:', error);
        }
    }

    async pause() {
        console.log('[PlaybackManager] Pause button clicked');
        try {
            const response = await fetch('/playback/pause', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ playlist_id: this.currentPlaylistId })
            });
            if (response.ok) {
                const data = await response.json();
                this.isPaused = data.is_paused;
                this.isPlaying = data.is_playing;
                if (data.revision) {
                    this.lastAppliedRevision = data.revision;
                }
                this.stopProgressSaving();
                this.saveProgress();
                this.updateButtonStates();

                console.log('[PlaybackManager] Broadcasting pause');
                this.isLocalChange = true;
                this.tabSync.broadcastPause();
            }
        } catch (error) {
            console.error('Error pausing playback:', error);
        }
    }

    async previous() {
        console.log('[PlaybackManager] Previous button clicked');
        try {
            const response = await fetch('/playback/previous', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ playlist_id: this.currentPlaylistId })
            });
            if (response.ok) {
                const data = await response.json();
                console.log('[PlaybackManager] previous() - server response:', JSON.stringify(data, null, 2));

                if (data.revision) {
                    this.lastAppliedRevision = data.revision;
                }

                if (data.queue_index !== undefined) {
                    this.queueIndex = data.queue_index;
                }
                
                let prevTrack = data.track;
                if (!prevTrack && data.queue && data.queue.length > 0 && this.queueIndex < data.queue.length) {
                    prevTrack = data.queue[this.queueIndex];
                }
                console.log('[PlaybackManager] previous() - queueIndex:', this.queueIndex, 'track:', prevTrack?.title);
                
                if (prevTrack) {
                    this.currentTrack = prevTrack;
                    this.currentYouTubeDuration = prevTrack.youtube_video_duration || 0;
                    console.log('[PlaybackManager] previous() - FINAL this.currentTrack:', this.currentTrack.title);
                    
                    document.getElementById('track-title').textContent = prevTrack.title || 'Unknown Track';
                    document.getElementById('track-artist').textContent = cleanArtistName(prevTrack.album_artist);
                    document.getElementById('track-album').textContent = 'Album: ' + cleanAlbumTitle(prevTrack.album_title, prevTrack.title);
                    document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(this.getEffectiveDuration());
                    document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
                    document.getElementById('progress-bar').style.width = '0%';
                    this.updateDurationComparison();
                }
                
                if (data.queue && data.queue.length > 0) {
                    console.log('[PlaybackManager] previous() - ALWAYS updating queue from server');
                    console.log('[PlaybackManager] previous() - new queue first 3:', data.queue.slice(0, 3).map(t => t.title));
                    this.queue = data.queue;
                    this.queueIndex = data.queue_index || this.queueIndex;
                    this.renderQueue();
                }

                this.currentPosition = 0;
                this.lastWallClockUpdate = Date.now();
                this.cachedPosition = this.currentPosition;
                this.updatePositionDisplay();
                this.isPlaying = true;
                this.isPaused = false;
                this.startProgressSaving();
                this.updateButtonStates();
                
                console.log('[PlaybackManager] Broadcasting skip');
                this.isLocalChange = true;
                this.tabSync.broadcastSkip(this.currentTrack, this.queueIndex, this.queue);
            }
        } catch (error) {
            console.error('Error skipping to previous:', error);
        }
    }

    async next() {
        console.log('[PlaybackManager] Next button clicked');
        try {
            const response = await fetch('/playback/skip', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ playlist_id: this.currentPlaylistId })
            });
            if (response.ok) {
                const data = await response.json();
                console.log('[PlaybackManager] next() - server response:', JSON.stringify(data, null, 2));

                if (data.revision) {
                    this.lastAppliedRevision = data.revision;
                }

                if (data.queue_index !== undefined) {
                    this.queueIndex = data.queue_index;
                }
                
                let nextTrack = data.track;
                console.log('[PlaybackManager] next() - data.track:', nextTrack?.title);
                console.log('[PlaybackManager] next() - this.queueIndex:', this.queueIndex);
                
                if (!nextTrack && data.queue && data.queue.length > 0 && this.queueIndex < data.queue.length) {
                    nextTrack = data.queue[this.queueIndex];
                    console.log('[PlaybackManager] next() - fell back to queue, nextTrack:', nextTrack?.title);
                }
                
                if (nextTrack) {
                    this.currentTrack = nextTrack;
                    this.currentYouTubeDuration = nextTrack.youtube_video_duration || 0;
                    console.log('[PlaybackManager] next() - FINAL this.currentTrack:', this.currentTrack.title);
                    
                    document.getElementById('track-title').textContent = nextTrack.title || 'Unknown Track';
                    document.getElementById('track-artist').textContent = cleanArtistName(nextTrack.album_artist);
                    document.getElementById('track-album').textContent = 'Album: ' + cleanAlbumTitle(nextTrack.album_title, nextTrack.title);
                    document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(this.getEffectiveDuration());
                    document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
                    document.getElementById('progress-bar').style.width = '0%';
                    this.updateDurationComparison();
                }
                
                if (data.queue && data.queue.length > 0) {
                    console.log('[PlaybackManager] next() - ALWAYS updating queue from server');
                    console.log('[PlaybackManager] next() - new queue first 3:', data.queue.slice(0, 3).map(t => t.title));
                    this.queue = data.queue;
                    this.queueIndex = data.queue_index || this.queueIndex;
                    this.renderQueue();
                }

                this.currentPosition = 0;
                this.lastWallClockUpdate = Date.now();
                this.cachedPosition = this.currentPosition;
                this.updatePositionDisplay();
                this.isPlaying = true;
                this.isPaused = false;
                this.startProgressSaving();
                this.updateButtonStates();
                
                console.log('[PlaybackManager] Broadcasting skip - this.currentTrack:', this.currentTrack?.title, 'this.queueIndex:', this.queueIndex);
                console.log('[PlaybackManager] Broadcasting skip - this.queue[0]:', this.queue[0]?.title, 'this.queue[1]:', this.queue[1]?.title);
                this.isLocalChange = true;
                this.tabSync.broadcastSkip(this.currentTrack, this.queueIndex, this.queue);
            }
        } catch (error) {
            console.error('Error skipping to next:', error);
        }
    }

    // Advance to next track but stay paused (used when track ends with auto-play disabled)
    async advanceAndPause() {
        console.log('[PlaybackManager] Advancing to next track and pausing');
        try {
            // First skip to next track on server
            const response = await fetch('/playback/skip', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ playlist_id: this.currentPlaylistId })
            });

            if (response.ok) {
                const data = await response.json();

                if (data.queue_index !== undefined) {
                    this.queueIndex = data.queue_index;
                }

                let nextTrack = data.track;
                if (!nextTrack && data.queue && data.queue.length > 0 && this.queueIndex < data.queue.length) {
                    nextTrack = data.queue[this.queueIndex];
                }

                if (nextTrack) {
                    this.currentTrack = nextTrack;
                    this.currentYouTubeDuration = nextTrack.youtube_video_duration || 0;

                    document.getElementById('track-title').textContent = nextTrack.title || 'Unknown Track';
                    document.getElementById('track-artist').textContent = cleanArtistName(nextTrack.album_artist);
                    document.getElementById('track-album').textContent = 'Album: ' + cleanAlbumTitle(nextTrack.album_title, nextTrack.title);
                    document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(this.getEffectiveDuration());
                    document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
                    document.getElementById('progress-bar').style.width = '0%';
                    this.updateDurationComparison();
                }

                if (data.queue && data.queue.length > 0) {
                    this.queue = data.queue;
                    this.queueIndex = data.queue_index || this.queueIndex;
                    this.renderQueue();
                }

                this.currentPosition = 0;
                this.lastWallClockUpdate = Date.now();
                this.cachedPosition = this.currentPosition;

                // Now pause immediately
                await this.pause();
            }
        } catch (error) {
            console.error('Error advancing to next track:', error);
        }
    }

    async stop() {
        console.log('[PlaybackManager] Stop button clicked');
        try {
            const response = await fetch('/playback/stop', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ playlist_id: this.currentPlaylistId })
            });
            if (response.ok) {
                this.isPlaying = false;
                this.isPaused = false;
                this.stopProgressSaving();
                this.currentPlaylistId = null;
                this.currentPlaylistName = null;
                this.currentTrack = null;
                this.queue = [];
                this.queueIndex = 0;
                this.currentPosition = 0;
                this.updatePlaybackStatus();

                console.log('[PlaybackManager] Broadcasting stop');
                this.isLocalChange = true;
                this.tabSync.broadcastStop();
            }
        } catch (error) {
            console.error('Error stopping playback:', error);
        }
    }

    toggleQueue() {
        const queuePanel = document.getElementById('queue-panel');
        const queueToggle = document.getElementById('toggle-queue-btn');

        this.isQueueVisible = !this.isQueueVisible;

        if (queuePanel) {
            queuePanel.style.display = this.isQueueVisible ? 'block' : 'none';
        }

        if (queueToggle) {
            queueToggle.textContent = this.isQueueVisible ? 'Hide Queue' : 'Show Queue';
        }
    }

    updateButtonStates() {
        const playBtn = document.getElementById('play-btn');
        const pauseBtn = document.getElementById('pause-btn');

        if (this.isPlaying) {
            playBtn.disabled = true;
            pauseBtn.disabled = false;
        } else if (this.isPaused) {
            playBtn.disabled = false;
            pauseBtn.disabled = true;
        } else {
            playBtn.disabled = false;
            pauseBtn.disabled = true;
        }
    }

    renderQueue() {
        const queueList = document.getElementById('queue-list');
        const paginationContainer = document.querySelector('#queue-panel .pagination-controls');
        if (!queueList) return;

        queueList.innerHTML = '';

        if (this.queue.length === 0) {
            queueList.innerHTML = '<p class="empty-message">Queue is empty</p>';
            if (paginationContainer) {
                paginationContainer.style.display = 'none';
            }
            return;
        }
        
        const totalPages = Math.ceil(this.queue.length / this.queueItemsPerPage);
        const startIndex = (this.queueCurrentPage - 1) * this.queueItemsPerPage;
        const endIndex = Math.min(startIndex + this.queueItemsPerPage, this.queue.length);
        const pageQueue = this.queue.slice(startIndex, endIndex);
        
        pageQueue.forEach((track, index) => {
            const globalIndex = startIndex + index;
            const item = document.createElement('div');
            item.className = 'queue-item';
            if (globalIndex === this.queueIndex) {
                item.classList.add('current');
            }
            item.innerHTML = `
                <span class="queue-number">${globalIndex + 1}.</span>
                <span class="queue-title">${this.escapeHtml(cleanTrackTitle(track.title) || 'Unknown')}</span>
                <span class="queue-album">${this.escapeHtml(cleanAlbumTitle(track.album_title, track.title) || 'Unknown Album')}</span>
                <span class="queue-duration">${this.formatTime(track.duration || 0)}</span>
            `;
            item.addEventListener('click', () => {
                this.playQueueItem(globalIndex);
            });
            queueList.appendChild(item);
        });
        
        if (paginationContainer) {
            if (this.queue.length > this.queueItemsPerPage) {
                paginationContainer.style.display = 'flex';
                this.updatePaginationControls();
            } else {
                paginationContainer.style.display = 'none';
            }
        }
    }
    
    updatePaginationControls() {
        const prevBtn = document.getElementById('prev-page-btn');
        const nextBtn = document.getElementById('next-page-btn');
        const pageInfo = document.getElementById('page-info');
        
        if (!prevBtn || !nextBtn || !pageInfo) return;
        
        const totalPages = Math.ceil(this.queue.length / this.queueItemsPerPage);
        pageInfo.textContent = `Page ${this.queueCurrentPage} of ${totalPages}`;
        prevBtn.disabled = this.queueCurrentPage <= 1;
        nextBtn.disabled = this.queueCurrentPage >= totalPages;
    }

    playQueueItem(index) {
        if (index >= 0 && index < this.queue.length) {
            const track = this.queue[index];
            
            // Prevent periodic sync from overwriting our update
            this.isLocalChange = true;
            this.stateSyncBlocked = true;
            
            fetch('/playback/play-index', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ 
                    playlist_id: this.currentPlaylistId,
                    queue_index: index
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.track) {
                    this.queueIndex = index;
                    this.currentPosition = 0;
                    this.lastWallClockUpdate = Date.now();
                    this.cachedPosition = this.currentPosition;
                    this.displayTrack(track);
                    this.renderQueue();
                    this.startProgressSaving();
                    this.updateButtonStates();

                    // Wait for server to persist
                    setTimeout(() => {
                        this.stateSyncBlocked = false;
                    }, 1000);

                    this.isLocalChange = true;
                    this.tabSync.broadcastSkip(data.track, index, this.queue);
                } else {
                    this.stateSyncBlocked = false;
                }
            })
            .catch(error => {
                console.error('[Queue] Error playing queue item:', error);
                this.stateSyncBlocked = false;
            });
        }
    }

    displayTrack(track, playlistName) {
        this.currentTrack = track;
        this.currentPlaylistName = playlistName || this.currentPlaylistName;
        this.currentYouTubeDuration = track.youtube_video_duration || 0;

        console.log('[PlaybackManager] displayTrack called with:', {
            title: track.title,
            album_title: track.album_title,
            duration: track.duration,
            youtube_video_duration: this.currentYouTubeDuration,
            youtube_video_id: track.youtube_video_id,
            id: track.id,
            fullTrack: track
        });

        document.getElementById('track-title').textContent = cleanTrackTitle(track.title) || 'Unknown Track';
        document.getElementById('track-artist').textContent = cleanArtistName(track.album_artist);
        document.getElementById('track-album').textContent = 'Album: ' + cleanAlbumTitle(track.album_title, track.title);
        document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(this.getEffectiveDuration());
        document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(this.currentPosition);
        document.getElementById('progress-bar').style.width = '0%';

        // Update playlist info if available
        const playlistInfo = document.getElementById('playlist-info');
        if (playlistInfo && this.currentPlaylistName) {
            playlistInfo.textContent = 'Playlist: ' + this.currentPlaylistName;
            playlistInfo.style.display = 'block';
        } else if (playlistInfo) {
            playlistInfo.style.display = 'none';
        }

        // Update duration comparison
        this.updateDurationComparison();
    }

    startStatusUpdate() {
        this.lastWallClockUpdate = Date.now();
        this.cachedPosition = this.currentPosition;

        setInterval(() => {
            if (this.isPlaying && !this.isPaused) {
                const now = Date.now();
                const elapsedMs = now - this.lastWallClockUpdate;
                const elapsedSeconds = Math.floor(elapsedMs / 1000);

                if (elapsedSeconds > 0) {
                    this.cachedPosition += elapsedSeconds;
                    this.lastWallClockUpdate += elapsedSeconds * 1000;
                    this.currentPosition = this.cachedPosition;
                    this.updatePositionDisplay();
                    this.checkTrackEnd();
                    if (now - this.lastTickDebug >= 5000) {
                        console.log('[TimeDebug] Tick', {
                            elapsedMs,
                            elapsedSeconds,
                            currentPosition: this.currentPosition,
                            cachedPosition: this.cachedPosition,
                            lastWallClockUpdate: this.lastWallClockUpdate
                        });
                        this.lastTickDebug = now;
                    }
                }
            }
        }, 1000);
    }

    checkTrackEnd() {
        if (!this.currentTrack) return;

        const effectiveDuration = this.getEffectiveDuration();

        if (this.currentPosition >= effectiveDuration) {
            console.log('[PlaybackManager] Track ended! QueueIndex:', this.queueIndex, 'Queue length:', this.queue.length);
            console.log('[PlaybackManager] Queue first 3:', this.queue.slice(0, 3).map(t => t.title));
            
            if (this.queueIndex < this.queue.length - 1) {
                const nextTrack = this.queue[this.queueIndex + 1];
                console.log('[PlaybackManager] Next track from queue:', nextTrack?.title);
                
                if (this.autoPlayEnabled) {
                    console.log('[PlaybackManager] Auto-playing next track via server skip');

                    // Call the server's skip endpoint to properly update in-memory state
                    // This ensures the video feed gets notified via SSE
                    this.next();
                    return; // next() handles everything
                } else {
                    console.log('[PlaybackManager] Advancing to next track (auto-play disabled)');

                    // Call the server to advance to next track, then pause
                    // This ensures the video feed gets notified via SSE
                    this.advanceAndPause();
                }
            } else {
                console.log('[PlaybackManager] No more tracks in queue');
                this.stop();
            }
        }
    }

    updatePositionDisplay() {
        document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(this.currentPosition);
        document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(this.getEffectiveDuration());

        const effectiveDuration = this.getEffectiveDuration();
        if (effectiveDuration > 0) {
            const progress = (this.currentPosition / effectiveDuration) * 100;
            document.getElementById('progress-bar').style.width = progress + '%';
        }
    }

    startProgressSaving() {
        this.stopProgressSaving();
        this.saveInterval = setInterval(() => {
            if (this.isPlaying && !this.isPaused) {
                setTimeout(() => {
                    this.saveProgress();
                }, 600);
            }
        }, 30000);
    }

    stopProgressSaving() {
        if (this.saveInterval) {
            clearInterval(this.saveInterval);
            this.saveInterval = null;
        }
    }

    saveProgress() {
        if (!this.currentTrack || !this.currentPlaylistId) return;

        const now = Date.now();
        if (now - this.lastSaveDebug >= 5000) {
            console.log('[TimeDebug] Saving progress', {
                playlistId: this.currentPlaylistId,
                trackId: this.currentTrack.id,
                positionSeconds: this.currentPosition,
                queueIndex: this.queueIndex
            });
            this.lastSaveDebug = now;
        }

        fetch('/playback/update-progress', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                playlist_id: this.currentPlaylistId,
                track_id: this.currentTrack.id,
                position_seconds: this.currentPosition,
                queue_index: this.queueIndex
            })
        }).catch(error => {
            console.error('Error saving progress:', error);
        });
    }

    setupBeforeUnload() {
        // Save progress when leaving page
        window.addEventListener('beforeunload', () => {
            if (this.isPlaying || this.isPaused) {
                this.saveProgress();
            }
            if (this.syncInterval) {
                clearInterval(this.syncInterval);
            }
            if (this.stateSyncInterval) {
                clearInterval(this.stateSyncInterval);
            }
        });
    }

    async updatePlaybackStatus() {
        try {
            const url = this.currentPlaylistId 
                ? `/playback/current?playlist_id=${encodeURIComponent(this.currentPlaylistId)}`
                : '/playback/current';
            const response = await fetch(url);
            const data = await response.json();

            if (data.server_time) {
                const serverTime = new Date(data.server_time).getTime();
                const localTime = Date.now();
                this.serverTimeOffset = serverTime - localTime;
                this.lastServerSync = localTime;
                console.log('[TimeSync] Updated offset:', this.serverTimeOffset, 'ms');
            }

            if (data.track) {
                if (!this.currentTrack || data.track.id !== this.currentTrack.id) {
                    this.displayTrack(data.track);
                }

                const playBtn = document.getElementById('play-btn');
                const pauseBtn = document.getElementById('pause-btn');

                if (!this.isPlaying && !this.isPaused) {
                    if (data.is_playing) {
                        this.isPlaying = true;
                        this.isPaused = false;
                        playBtn.disabled = true;
                        pauseBtn.disabled = false;
                    } else if (data.is_paused) {
                        this.isPaused = true;
                        this.isPlaying = false;
                        playBtn.disabled = false;
                        pauseBtn.disabled = true;
                    } else {
                        playBtn.disabled = false;
                        pauseBtn.disabled = true;
                    }
                }

                if (data.queue) {
                    try {
                        const queueTracks = typeof data.queue === 'string'
                            ? JSON.parse(data.queue)
                            : data.queue;
                        if (Array.isArray(queueTracks) && queueTracks.length > 0) {
                            this.queue = queueTracks;
                            this.queueIndex = data.queue_index || 0;
                            this.renderQueue();
                        }
                    } catch (e) {
                        console.error('Error parsing queue:', e);
                    }
                }
            } else {
                document.getElementById('track-title').textContent = 'No Track Playing';
                document.getElementById('track-artist').textContent = '-';
                document.getElementById('track-album').textContent = '-';
                document.getElementById('track-duration').textContent = 'Duration: 00:00';
                document.getElementById('track-position').textContent = 'Position: 00:00';
                document.getElementById('progress-bar').style.width = '0%';
            }
        } catch (error) {
            console.error('Error fetching playback status:', error);
        }
    }

    formatTime(seconds) {
        if (!seconds || seconds <= 0) return '00:00';
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }

    getEffectiveDuration() {
        if (this.useYouTubeDuration && this.currentYouTubeDuration > 0) {
            return this.currentYouTubeDuration;
        }
        return this.currentTrack?.duration || 0;
    }

    updateDurationComparison() {
        const dbDuration = this.currentTrack?.duration || 0;
        const ytDuration = this.currentYouTubeDuration || 0;
        const comparisonDiv = document.getElementById('duration-comparison');
        const refreshBtn = document.getElementById('refresh-duration-btn');
        const statusDiv = document.getElementById('duration-status');
        
        console.log('[Duration] DB:', dbDuration, 'YT:', ytDuration, 'currentTrack:', this.currentTrack);
        
        if (ytDuration > 0 && dbDuration > 0 && dbDuration !== ytDuration) {
            comparisonDiv.classList.add('visible');
            document.getElementById('db-duration').textContent = `DB: ${this.formatTime(dbDuration)}`;
            document.getElementById('yt-duration').textContent = `YT: ${this.formatTime(ytDuration)}`;
            
            const diff = ytDuration - dbDuration;
            const diffAbs = Math.abs(diff);
            const diffText = diff > 0 ? `+${this.formatTime(diffAbs)}` : `-${this.formatTime(diffAbs)}`;
            const diffEl = document.getElementById('duration-diff');
            diffEl.textContent = `Diff: ${diffText}`;
            diffEl.style.color = diff > 0 ? '#4CAF50' : '#ff5722';
            
            refreshBtn.style.display = 'none';
            statusDiv.textContent = '';
        } else if (ytDuration === 0 && this.currentTrack?.youtube_video_id) {
            console.log('[Duration] No YouTube duration found - showing refresh button');
            comparisonDiv.classList.add('visible');
            document.getElementById('db-duration').textContent = `DB: ${this.formatTime(dbDuration)}`;
            document.getElementById('yt-duration').textContent = `YT: --:--`;
            document.getElementById('duration-diff').textContent = 'No duration cached';
            document.getElementById('duration-diff').style.color = '#888';
            
            refreshBtn.style.display = 'inline-block';
            statusDiv.textContent = '';
        } else {
            comparisonDiv.classList.remove('visible');
            refreshBtn.style.display = 'none';
            statusDiv.textContent = '';
        }
    }

    async refreshYouTubeDuration() {
        const refreshBtn = document.getElementById('refresh-duration-btn');
        const statusDiv = document.getElementById('duration-status');
        
        if (!this.currentTrack?.youtube_video_id) {
            statusDiv.textContent = 'No YouTube video ID found for this track';
            statusDiv.className = 'duration-status error';
            return;
        }
        
        refreshBtn.disabled = true;
        refreshBtn.textContent = 'Fetching...';
        statusDiv.textContent = 'Fetching duration from YouTube...';
        statusDiv.className = 'duration-status loading';
        
        try {
            const response = await fetch('/playback/video/refresh-duration', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ video_id: this.currentTrack.youtube_video_id })
            });
            
            const data = await response.json();
            
            if (response.status === 401) {
                statusDiv.textContent = 'YouTube not connected. Go to Settings to connect YouTube.';
                statusDiv.className = 'duration-status error';
                refreshBtn.disabled = false;
                refreshBtn.textContent = 'Retry';
                return;
            }
            
            if (data.success) {
                this.currentYouTubeDuration = data.video_duration;
                console.log('[Duration] Fetched new duration:', this.currentYouTubeDuration);
                statusDiv.textContent = `Duration updated: ${this.formatTime(data.video_duration)}`;
                statusDiv.className = 'duration-status success';
                refreshBtn.textContent = 'Refresh Again';
                refreshBtn.disabled = false;
                
                this.updateDurationComparison();
                
                document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(this.getEffectiveDuration());
            } else {
                statusDiv.textContent = data.error || 'Failed to fetch duration';
                statusDiv.className = 'duration-status error';
                refreshBtn.disabled = false;
                refreshBtn.textContent = 'Retry';
            }
        } catch (error) {
            console.error('[Duration] Error fetching duration:', error);
            statusDiv.textContent = 'Error: ' + error.message;
            statusDiv.className = 'duration-status error';
            refreshBtn.disabled = false;
            refreshBtn.textContent = 'Retry';
        }
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}
