export const pagination = {
    album: { page: 1, limit: 25, totalPages: 1, query: '' },
    track: { page: 1, limit: 25, totalPages: 1, query: '' }
};

export function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return '0:00';
    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes}:${secs < 10 ? '0' : ''}${secs}`;
}

export function formatTime(seconds) {
    if (!seconds || seconds <= 0) return '00:00';
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
}

export function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

export function cleanAlbumTitle(albumTitle, trackTitle) {
    if (!albumTitle) return 'Unknown Album';

    if (albumTitle.includes(' / ') && albumTitle.includes(trackTitle)) {
        const parts = albumTitle.split(' / ');
        return parts[parts.length - 1].trim();
    }

    return albumTitle;
}

export function cleanArtistName(artistName) {
    if (!artistName) return 'Unknown Artist';
    // Use the global normalizeArtistName from utils.js if available
    if (typeof window.normalizeArtistName === 'function') {
        return window.normalizeArtistName(artistName) || 'Unknown Artist';
    }
    return artistName;
}

export function updatePaginationControls(type) {
    const pag = pagination[type];
    
    document.querySelectorAll(`.${type}-limit`).forEach(el => {
        el.value = pag.limit;
    });

    const pageInfoElements = document.querySelectorAll(`[id^="${type}-page-info"]`);
    pageInfoElements.forEach(el => {
        el.textContent = `Page ${pag.page} of ${pag.totalPages}`;
    });

    document.querySelectorAll(`.${type}-prev`).forEach(el => {
        el.disabled = pag.page <= 1;
    });
    document.querySelectorAll(`.${type}-next`).forEach(el => {
        el.disabled = pag.page >= pag.totalPages;
    });
}
