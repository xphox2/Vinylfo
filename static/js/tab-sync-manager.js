// Tab Synchronization Manager for cross-tab playback state
// Uses BroadcastChannel API for communication between tabs

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

// Expose TabSyncManager globally for use in other scripts
window.TabSyncManager = TabSyncManager;
