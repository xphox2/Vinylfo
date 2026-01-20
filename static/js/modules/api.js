// API Utility Module
// Provides centralized HTTP request handling for the Vinylfo frontend

const API_BASE = '/api';

// Generic fetch wrapper with error handling
async function apiFetch(endpoint, options = {}) {
    const url = `${API_BASE}${endpoint}`;

    const defaultHeaders = {
        'Content-Type': 'application/json',
    };

    const config = {
        ...options,
        headers: {
            ...defaultHeaders,
            ...options.headers,
        },
    };

    try {
        const response = await fetch(url, config);

        if (!response.ok) {
            const error = await response.json().catch(() => ({ error: 'Request failed' }));
            throw new Error(error.error || `HTTP ${response.status}`);
        }

        return await response.json();
    } catch (error) {
        console.error(`API Error [${endpoint}]:`, error);
        throw error;
    }
}

// HTTP method shortcuts
export const api = {
    get: (endpoint, params = {}) => {
        const query = new URLSearchParams(params).toString();
        const url = query ? `${endpoint}?${query}` : endpoint;
        return apiFetch(url, { method: 'GET' });
    },

    post: (endpoint, data = {}) => {
        return apiFetch(endpoint, {
            method: 'POST',
            body: JSON.stringify(data),
        });
    },

    put: (endpoint, data = {}) => {
        return apiFetch(endpoint, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    },

    patch: (endpoint, data = {}) => {
        return apiFetch(endpoint, {
            method: 'PATCH',
            body: JSON.stringify(data),
        });
    },

    delete: (endpoint) => {
        return apiFetch(endpoint, { method: 'DELETE' });
    },
};

// Playback-specific API calls
export const playbackAPI = {
    getState: (playlistId) => api.get('/playback/state', { playlist_id: playlistId }),
    start: (playlistId, playlistName, trackIds) =>
        api.post('/playback/start', { playlist_id: playlistId, playlist_name: playlistName, track_ids: trackIds }),
    play: (playlistId) => api.post('/playback/play', { playlist_id: playlistId }),
    pause: (playlistId) => api.post('/playback/pause', { playlist_id: playlistId }),
    stop: (playlistId) => api.post('/playback/stop', { playlist_id: playlistId }),
    next: (playlistId) => api.post('/playback/next', { playlist_id: playlistId }),
    previous: (playlistId) => api.post('/playback/previous', { playlist_id: playlistId }),
    seek: (playlistId, position) => api.post('/playback/seek', { playlist_id: playlistId, position }),
    setQueueIndex: (playlistId, index) => api.post('/playback/queue-index', { playlist_id: playlistId, queue_index: index }),
    clear: (playlistId) => api.post('/playback/clear', { playlist_id: playlistId }),
    restore: (playlistId) => api.post('/playback/restore', { playlist_id: playlistId }),
    setVolume: (playlistId, volume) => api.post('/playback/volume', { playlist_id: playlistId, volume }),
};

// Playlist-specific API calls
export const playlistAPI = {
    getAll: () => api.get('/playback/sessions'),
    getTracks: (sessionId) => api.get(`/playback/session/${sessionId}/tracks`),
    delete: (sessionId) => api.delete(`/playback/session/${sessionId}`),
    getSessionPlaylistTracks: (sessionId) => api.get(`/playlist/${sessionId}/tracks`),
    create: (sessionId, trackIds) => api.post('/playlist/create', { session_id: sessionId, track_ids: trackIds }),
    addTrack: (sessionId, trackId) => api.post(`/playlist/${sessionId}/add`, { track_id: trackId }),
    removeTrack: (sessionId, trackId) => api.post(`/playlist/${sessionId}/remove`, { track_id: trackId }),
    reorder: (sessionId, fromIndex, toIndex) =>
        api.post(`/playlist/${sessionId}/reorder`, { from_index: fromIndex, to_index: toIndex }),
    clear: (sessionId) => api.delete(`/playlist/${sessionId}`),
};

// Album-specific API calls
export const albumAPI = {
    getAll: (page = 1, limit = 25) => api.get('/albums', { page, limit }),
    getById: (id) => api.get(`/album/${id}`),
    search: (query, page = 1, limit = 25) => api.get('/albums/search', { q: query, page, limit }),
    create: (data) => api.post('/album', data),
    update: (id, data) => api.put(`/album/${id}`, data),
    delete: (id) => api.delete(`/album/${id}`),
    getTracks: (id) => api.get(`/album/${id}/tracks`),
};

// Track-specific API calls
export const trackAPI = {
    getAll: (page = 1, limit = 25) => api.get('/tracks', { page, limit }),
    getById: (id) => api.get(`/track/${id}`),
    search: (query, page = 1, limit = 25) => api.get('/tracks/search', { q: query, page, limit }),
    create: (data) => api.post('/track', data),
    update: (id, data) => api.put(`/track/${id}`, data),
    delete: (id) => api.delete(`/track/${id}`),
};

// Search API
export const searchAPI = {
    all: (query, types = ['albums', 'tracks']) => api.get('/search', { q: query, type: types.join(',') }),
    albums: (query, page = 1) => api.get('/search/albums', { q: query, page }),
    tracks: (query, page = 1) => api.get('/search/tracks', { q: query, page }),
};

// Discogs sync API
export const discogsAPI = {
    search: (query) => api.post('/discogs/search', { query }),
    importAlbum: (discogsId) => api.post('/discogs/import', { discogs_id: discogsId }),
    getFolders: () => api.get('/discogs/folders'),
    startSync: (folderId) => api.post('/discogs/sync/start', { folder_id: folderId }),
    pauseSync: () => api.post('/discogs/sync/pause'),
    resumeSync: () => api.post('/discogs/sync/resume'),
    cancelSync: () => api.post('/discogs/sync/cancel'),
    getProgress: () => api.get('/discogs/sync/progress'),
    getHistory: () => api.get('/discogs/sync/history'),
    getCollection: (page = 1) => api.get('/discogs/collection', { page }),
    deleteAlbum: (discogsId) => api.post('/discogs/album/delete', { discogs_id: discogsId }),
};

// Duration resolution API
export const durationAPI = {
    getStats: () => api.get('/duration/stats'),
    getReviewQueue: (page = 1, limit = 20) => api.get('/duration/review', { page, limit }),
    getUnprocessed: (page = 1, limit = 20) => api.get('/duration/tracks', { page, limit }),
    getResolved: (page = 1, limit = 20) => api.get('/duration/review/resolved', { page, limit }),
    getReviewDetails: (resolutionId) => api.get(`/duration/review/${resolutionId}`),
    submitReview: (resolutionId, action, duration, notes) =>
        api.post(`/duration/review/${resolutionId}`, { action, duration, notes }),
    applySelected: (resolutionId, sourceId, notes) =>
        api.post(`/duration/review/${resolutionId}`, { action: 'apply', duration: sourceId, notes }),
    reject: (resolutionId, notes) => api.post(`/duration/review/${resolutionId}`, { action: 'reject', notes }),
    manualDuration: (trackId, duration, notes) =>
        api.post(`/duration/track/${trackId}/manual`, { duration, notes }),
    startBulkResolution: () => api.post('/duration/resolve/start'),
    pauseBulkResolution: () => api.post('/duration/resolve/pause'),
    resumeBulkResolution: () => api.post('/duration/resolve/resume'),
    cancelBulkResolution: () => api.post('/duration/resolve/cancel'),
    getProgress: () => api.get('/duration/resolve/progress'),
};

// Settings API
export const settingsAPI = {
    get: () => api.get('/settings'),
    set: (key, value) => api.post('/settings', { key, value }),
    getDiscogsAuthUrl: () => api.get('/discogs/oauth/url'),
};

// Make api available globally for backward compatibility
window.api = api;
