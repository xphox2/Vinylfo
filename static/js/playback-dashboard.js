// Playback Dashboard JavaScript with Queue and Resume Support
document.addEventListener('DOMContentLoaded', function() {
    console.log('Playback dashboard loaded');

    const playbackManager = new PlaybackManager();
    playbackManager.init();
});

class TabSyncManager {
    constructor() {
        this.channel = new BroadcastChannel('vinylfo_playback_channel');
        this.tabId = this.generateTabId();
        this.setupListeners();
        console.log('[TabSync] Channel initialized, tabId:', this.tabId);
    }

    generateTabId() {
        return 'tab_' + Math.random().toString(36).substr(2, 9) + '_' + Date.now();
    }

    setupListeners() {
        this.channel.onmessage = (event) => {
            const { type, data, sourceTabId, timestamp } = event.data;
            if (sourceTabId === this.tabId) {
                console.log('[TabSync] Ignoring own message:', type);
                return;
            }
            console.log('[TabSync] Received:', type, 'from', sourceTabId);

            switch (type) {
                case 'state_update':
                    this.handleStateUpdate(data);
                    break;
                case 'play':
                    this.handlePlay(data);
                    break;
                case 'pause':
                    this.handlePause();
                    break;
                case 'stop':
                    this.handleStop();
                    break;
                case 'skip':
                    this.handleSkip(data);
                    break;
                case 'seek':
                    this.handleSeek(data);
                    break;
            }
        };
    }

    broadcast(type, data = {}) {
        const message = {
            type,
            data,
            sourceTabId: this.tabId,
            timestamp: Date.now()
        };
        console.log('[TabSync] Broadcasting:', type, 'with data:', data);
        this.channel.postMessage(message);
    }

    handleStateUpdate(data) {
        console.log('[TabSync] Dispatching state_update event');
        window.dispatchEvent(new CustomEvent('vinylfo_state_update', { detail: data }));
    }

    handlePlay(data) {
        console.log('[TabSync] Dispatching play event');
        window.dispatchEvent(new CustomEvent('vinylfo_play', { detail: data }));
    }

    handlePause() {
        console.log('[TabSync] Dispatching pause event');
        window.dispatchEvent(new CustomEvent('vinylfo_pause'));
    }

    handleStop() {
        console.log('[TabSync] Dispatching stop event');
        window.dispatchEvent(new CustomEvent('vinylfo_stop'));
    }

    handleSkip(data) {
        console.log('[TabSync] Dispatching skip event');
        window.dispatchEvent(new CustomEvent('vinylfo_skip', { detail: data }));
    }

    handleSeek(data) {
        console.log('[TabSync] Dispatching seek event');
        window.dispatchEvent(new CustomEvent('vinylfo_seek', { detail: data }));
    }

    broadcastStateUpdate(state) {
        this.broadcast('state_update', state);
    }

    broadcastPlay(track, position) {
        this.broadcast('play', { track, position });
    }

    broadcastPause() {
        this.broadcast('pause');
    }

    broadcastStop() {
        this.broadcast('stop');
    }

    broadcastSkip(track, queueIndex, queue) {
        this.broadcast('skip', { track, queueIndex, queue });
    }

    broadcastSeek(position) {
        this.broadcast('seek', { position });
    }
}

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
        this.tabSync = new TabSyncManager();
        this.syncInterval = null;
        this.isLocalChange = false;
        this.serverTimeOffset = 0;
        this.lastServerSync = 0;
        this.autoPlayEnabled = false;
        this.trackEndedHandler = null;
    }

    async init() {
        console.log('[PlaybackManager] Initializing...');
        
        // Load current playback status from server
        await this.loadCurrentPlayback();
        
        this.setupControls();
        this.setupTabSyncListeners();
        this.startStatusUpdate();
        this.setupBeforeUnload();
        this.startPeriodicStateSync();
        console.log('[PlaybackManager] Initialization complete');
    }

    async loadCurrentPlayback() {
        try {
            const response = await fetch('/playback/current');
            const data = await response.json();
            
            console.log('[PlaybackManager] loadCurrentPlayback response:', JSON.stringify(data, null, 2));
            
            if (data.track) {
                // Display the track
                this.currentTrack = data.track;
                this.currentPosition = data.position || 0;
                this.queueIndex = data.queue_index || 0;
                
                document.getElementById('track-title').textContent = data.track.title || 'Unknown Track';
                document.getElementById('track-artist').textContent = data.track.album_title || 'Unknown Album';
                document.getElementById('track-album').textContent = 'Album: ' + (data.track.album_title || 'Unknown');
                document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(data.track.duration || 0);
                document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(this.currentPosition);
                document.getElementById('progress-bar').style.width = '0%';
                
                // Set playing state
                const playBtn = document.getElementById('play-btn');
                const pauseBtn = document.getElementById('pause-btn');
                
                // If position < duration, assume playing
                const isPlaying = data.position < (data.track.duration || 0);
                
                if (isPlaying) {
                    this.isPlaying = true;
                    this.isPaused = false;
                    playBtn.disabled = true;
                    pauseBtn.disabled = false;
                    this.startProgressSaving();
                } else {
                    this.isPlaying = false;
                    this.isPaused = false;
                    playBtn.disabled = false;
                    pauseBtn.disabled = true;
                }
                
                // Load queue
                if (data.queue && data.queue.length > 0) {
                    this.queue = data.queue;
                    this.renderQueue();
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
        try {
            const response = await fetch('/playback/current');
            const data = await response.json();
            
            console.log('[Sync] Server track:', data.track?.title, 'queue_index:', data.queue_index);
            console.log('[Sync] Local currentTrack:', this.currentTrack?.title, 'queueIndex:', this.queueIndex);
            
            // Sync track if changed
            if (data.track && (!this.currentTrack || data.track.id !== this.currentTrack.id)) {
                console.log('[Sync] TRACK CHANGED on server, updating from', this.currentTrack?.title, 'to', data.track.title);
                this.currentTrack = data.track;
                
                document.getElementById('track-title').textContent = data.track.title || 'Unknown Track';
                document.getElementById('track-artist').textContent = data.track.album_title || 'Unknown Album';
                document.getElementById('track-album').textContent = 'Album: ' + (data.track.album_title || 'Unknown');
                document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(data.track.duration || 0);
            } else {
                console.log('[Sync] No track change needed');
            }
            
            // Sync queue - always sync queue_index and update queue if order differs
            if (data.queue && data.queue.length > 0) {
                const serverQueueIds = data.queue.map(t => t.id).join(',');
                const localQueueIds = this.queue.map(t => t.id).join(',');
                
                const queueChanged = serverQueueIds !== localQueueIds;
                const indexChanged = this.queueIndex !== data.queue_index;
                
                if (queueChanged || indexChanged) {
                    console.log('[Sync] Queue update needed:', { queueChanged, indexChanged });
                    console.log('[Sync] Server queue first 3:', data.queue.slice(0, 3).map(t => t.title));
                    console.log('[Sync] Local queue first 3:', this.queue.slice(0, 3).map(t => t.title));
                    this.queue = data.queue;
                    this.queueIndex = data.queue_index || 0;
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
            console.log('[PlaybackManager] applyRemoteSkip - updating UI with track:', trackToPlay.title);
            
            document.getElementById('track-title').textContent = trackToPlay.title || 'Unknown Track';
            document.getElementById('track-artist').textContent = trackToPlay.album_title || 'Unknown Album';
            document.getElementById('track-album').textContent = 'Album: ' + (trackToPlay.album_title || 'Unknown');
            document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(trackToPlay.duration || 0);
            document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
            document.getElementById('progress-bar').style.width = '0%';
        } else {
            console.log('[PlaybackManager] applyRemoteSkip - NO TRACK to play!');
        }
        
        if (data.queue) {
            this.renderQueue();
        }
        
        this.currentPosition = 0;
        this.updatePositionDisplay();
        this.isPlaying = true;
        this.isPaused = false;
        this.startProgressSaving();
        this.updateButtonStates();
    }

    applyRemoteSeek(position) {
        console.log('[PlaybackManager] Applying remote seek to:', position);
        this.currentPosition = position;
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
    }

    seekToPosition(event) {
        if (!this.currentTrack || !this.currentTrack.duration) return;

        const progressContainer = event.currentTarget;
        const rect = progressContainer.getBoundingClientRect();
        const clickX = event.clientX - rect.left;
        const containerWidth = rect.width;

        // Calculate percentage
        const percentage = clickX / containerWidth;

        // Calculate new position in seconds
        const newPosition = Math.floor(percentage * this.currentTrack.duration);

        console.log('[PlaybackManager] Seeking to position:', newPosition);

        // Update position
        this.currentPosition = newPosition;
        this.updatePositionDisplay();

        // Broadcast seek to other tabs
        this.isLocalChange = true;
        this.tabSync.broadcastSeek(newPosition);

        // Save progress to backend
        this.saveProgress();
    }

    async play() {
        console.log('[PlaybackManager] Play button clicked');
        try {
            const response = await fetch('/playback/resume', { method: 'POST' });
            if (response.ok) {
                const data = await response.json();
                this.isPlaying = data.is_playing;
                this.isPaused = data.is_paused;
                this.startProgressSaving();
                this.updateButtonStates();

                // Broadcast play to other tabs
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
            const response = await fetch('/playback/pause', { method: 'POST' });
            if (response.ok) {
                const data = await response.json();
                this.isPaused = data.is_paused;
                this.isPlaying = data.is_playing;
                this.stopProgressSaving();
                this.saveProgress();
                this.updateButtonStates();

                // Broadcast pause to other tabs
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
            const response = await fetch('/playback/previous', { method: 'POST' });
            if (response.ok) {
                const data = await response.json();
                console.log('[PlaybackManager] previous() - server response:', JSON.stringify(data, null, 2));
                
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
                    console.log('[PlaybackManager] previous() - FINAL this.currentTrack:', this.currentTrack.title);
                    
                    document.getElementById('track-title').textContent = prevTrack.title || 'Unknown Track';
                    document.getElementById('track-artist').textContent = prevTrack.album_title || 'Unknown Album';
                    document.getElementById('track-album').textContent = 'Album: ' + (prevTrack.album_title || 'Unknown');
                    document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(prevTrack.duration || 0);
                    document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
                    document.getElementById('progress-bar').style.width = '0%';
                }
                
                // ALWAYS update queue from server to ensure sync
                if (data.queue && data.queue.length > 0) {
                    console.log('[PlaybackManager] previous() - ALWAYS updating queue from server');
                    console.log('[PlaybackManager] previous() - new queue first 3:', data.queue.slice(0, 3).map(t => t.title));
                    this.queue = data.queue;
                    this.queueIndex = data.queue_index || this.queueIndex;
                    this.renderQueue();
                }
                
                this.currentPosition = 0;
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
            const response = await fetch('/playback/skip', { method: 'POST' });
            if (response.ok) {
                const data = await response.json();
                console.log('[PlaybackManager] next() - server response:', JSON.stringify(data, null, 2));
                
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
                    console.log('[PlaybackManager] next() - FINAL this.currentTrack:', this.currentTrack.title);
                    
                    document.getElementById('track-title').textContent = nextTrack.title || 'Unknown Track';
                    document.getElementById('track-artist').textContent = nextTrack.album_title || 'Unknown Album';
                    document.getElementById('track-album').textContent = 'Album: ' + (nextTrack.album_title || 'Unknown');
                    document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(nextTrack.duration || 0);
                    document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
                    document.getElementById('progress-bar').style.width = '0%';
                }
                
                // ALWAYS update queue from server to ensure sync
                if (data.queue && data.queue.length > 0) {
                    console.log('[PlaybackManager] next() - ALWAYS updating queue from server');
                    console.log('[PlaybackManager] next() - new queue first 3:', data.queue.slice(0, 3).map(t => t.title));
                    this.queue = data.queue;
                    this.queueIndex = data.queue_index || this.queueIndex;
                    this.renderQueue();
                }
                
                this.currentPosition = 0;
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

    async stop() {
        console.log('[PlaybackManager] Stop button clicked');
        try {
            const response = await fetch('/playback/stop', { method: 'POST' });
            if (response.ok) {
                this.isPlaying = false;
                this.isPaused = false;
                this.stopProgressSaving();
                this.saveProgress();
                this.updatePlaybackStatus();

                // Broadcast stop to other tabs
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
        if (!queueList) return;

        queueList.innerHTML = '';

        if (this.queue.length === 0) {
            queueList.innerHTML = '<p class="empty-message">Queue is empty</p>';
            return;
        }

        this.queue.forEach((track, index) => {
            const item = document.createElement('div');
            item.className = 'queue-item';
            if (index === this.queueIndex) {
                item.classList.add('current');
            }
            item.innerHTML = `
                <span class="queue-number">${index + 1}.</span>
                <span class="queue-title">${this.escapeHtml(track.title || 'Unknown')}</span>
                <span class="queue-album">${this.escapeHtml(track.album_title || 'Unknown Album')}</span>
            `;
            item.addEventListener('click', () => {
                this.playQueueItem(index);
            });
            queueList.appendChild(item);
        });
    }

    playQueueItem(index) {
        if (index >= 0 && index < this.queue.length) {
            this.queueIndex = index;
            this.currentPosition = 0;
            this.displayTrack(this.queue[index]);
            this.renderQueue();
            this.startProgressSaving();
            this.saveProgress();

            this.isLocalChange = true;
            this.broadcastState();
        }
    }

    displayTrack(track, playlistName) {
        this.currentTrack = track;
        
        console.log('[PlaybackManager] displayTrack called with:', {
            title: track.title,
            album_title: track.album_title,
            duration: track.duration,
            id: track.id
        });

        document.getElementById('track-title').textContent = track.title || 'Unknown Track';
        document.getElementById('track-artist').textContent = track.album_title || 'Unknown Album';
        document.getElementById('track-album').textContent = 'Album: ' + (track.album_title || 'Unknown');
        document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(track.duration || 0);
        document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(this.currentPosition);
        document.getElementById('progress-bar').style.width = '0%';

        // Update playlist info if available
        const playlistInfo = document.getElementById('playlist-info');
        if (playlistInfo && playlistName) {
            playlistInfo.textContent = 'Playlist: ' + playlistName;
            playlistInfo.style.display = 'block';
        }
    }

    startStatusUpdate() {
        setInterval(() => {
            if (this.isPlaying && !this.isPaused) {
                this.currentPosition++;
                this.updatePositionDisplay();
                this.checkTrackEnd();
            }
        }, 1000);
    }

    checkTrackEnd() {
        if (!this.currentTrack) return;
        
        if (this.currentPosition >= this.currentTrack.duration) {
            console.log('[PlaybackManager] Track ended! QueueIndex:', this.queueIndex, 'Queue length:', this.queue.length);
            console.log('[PlaybackManager] Queue first 3:', this.queue.slice(0, 3).map(t => t.title));
            
            if (this.queueIndex < this.queue.length - 1) {
                const nextTrack = this.queue[this.queueIndex + 1];
                console.log('[PlaybackManager] Next track from queue:', nextTrack?.title);
                
                if (this.autoPlayEnabled) {
                    console.log('[PlaybackManager] Auto-playing next track:', nextTrack?.title);
                    
                    this.queueIndex++;
                    this.currentPosition = 0;
                    this.currentTrack = nextTrack;
                    
                    document.getElementById('track-title').textContent = nextTrack.title || 'Unknown Track';
                    document.getElementById('track-artist').textContent = nextTrack.album_title || 'Unknown Album';
                    document.getElementById('track-album').textContent = 'Album: ' + (nextTrack.album_title || 'Unknown');
                    document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(nextTrack.duration || 0);
                    document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
                    document.getElementById('progress-bar').style.width = '0%';
                    
                    this.renderQueue();
                    this.startProgressSaving();
                    this.updateButtonStates();
                    this.saveProgress();
                    
                    console.log('[PlaybackManager] Broadcasting skip - track:', nextTrack?.title, 'queueIndex:', this.queueIndex);
                    this.isLocalChange = true;
                    this.tabSync.broadcastSkip(nextTrack, this.queueIndex, this.queue);
                } else {
                    console.log('[PlaybackManager] Queueing next track (auto-play disabled)');
                    
                    this.queueIndex++;
                    const newTrack = this.queue[this.queueIndex];
                    if (newTrack) {
                        this.currentTrack = newTrack;
                        this.currentPosition = 0;
                        
                        document.getElementById('track-title').textContent = newTrack.title || 'Unknown Track';
                        document.getElementById('track-artist').textContent = newTrack.album_title || 'Unknown Album';
                        document.getElementById('track-album').textContent = 'Album: ' + (newTrack.album_title || 'Unknown');
                        document.getElementById('track-duration').textContent = 'Duration: ' + this.formatTime(newTrack.duration || 0);
                        document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(0);
                        document.getElementById('progress-bar').style.width = '0%';
                    }
                    
                    this.renderQueue();
                    this.stop();
                    
                    this.isLocalChange = true;
                    this.broadcastState();
                }
            } else {
                console.log('[PlaybackManager] No more tracks in queue');
                this.stop();
            }
        }
    }

    updatePositionDisplay() {
        document.getElementById('track-position').textContent = 'Position: ' + this.formatTime(this.currentPosition);

        if (this.currentTrack && this.currentTrack.duration > 0) {
            const progress = (this.currentPosition / this.currentTrack.duration) * 100;
            document.getElementById('progress-bar').style.width = progress + '%';
        }
    }

    startProgressSaving() {
        // Save progress every 1.5 seconds to ensure we capture the latest position
        // (after the position increment happens)
        this.stopProgressSaving();
        this.saveInterval = setInterval(() => {
            if (this.isPlaying && !this.isPaused) {
                // Small delay to ensure position increment has happened
                setTimeout(() => {
                    this.saveProgress();
                }, 600);
            }
        }, 1500);
    }

    stopProgressSaving() {
        if (this.saveInterval) {
            clearInterval(this.saveInterval);
            this.saveInterval = null;
        }
    }

    saveProgress() {
        if (!this.currentTrack) return;

        fetch('/playback/update-progress', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
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
            const response = await fetch('/playback/current');
            const data = await response.json();

            // Sync time with server periodically
            if (data.server_time) {
                const serverTime = new Date(data.server_time).getTime();
                const localTime = Date.now();
                this.serverTimeOffset = serverTime - localTime;
                this.lastServerSync = localTime;
                console.log('[TimeSync] Updated offset:', this.serverTimeOffset, 'ms');
            }

            if (data.track) {
                // Only update track info from server, trust our local position when playing
                if (!this.currentTrack || data.track.id !== this.currentTrack.id) {
                    this.displayTrack(data.track);
                }

                // Update button states - but don't override if we're already in a known state
                const playBtn = document.getElementById('play-btn');
                const pauseBtn = document.getElementById('pause-btn');

                // Only update from server if we haven't started playing yet (initial load)
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

                // Update queue if we got queue data
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
                // No track playing
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
        const secs = seconds % 60;
        return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}
