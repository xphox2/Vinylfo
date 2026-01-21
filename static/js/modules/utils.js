// Utility Module
// Provides common utility functions used across the Vinylfo frontend

// HTML escaping for XSS prevention
export function escapeHtml(text) {
    if (text === null || text === undefined) return '';
    const div = document.createElement('div');
    div.textContent = String(text);
    return div.innerHTML;
}

// Format duration from seconds to MM:SS
export function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return '0:00';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

// Format duration from seconds to human-readable string
export function formatDurationLong(seconds) {
    if (!seconds || seconds <= 0) return 'Unknown';

    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);

    const parts = [];
    if (hours > 0) parts.push(`${hours}h`);
    if (mins > 0) parts.push(`${mins}m`);
    if (secs > 0 || parts.length === 0) parts.push(`${secs}s`);

    return parts.join(' ');
}

// Format a date for display
export function formatDate(dateString) {
    if (!dateString) return 'Never';
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
    });
}

// Format relative time (e.g., "2 hours ago")
export function formatRelativeTime(dateString) {
    if (!dateString) return 'Never';

    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now - date;
    const diffSecs = Math.floor(diffMs / 1000);
    const diffMins = Math.floor(diffSecs / 60);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffSecs < 60) return 'Just now';
    if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
    if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;

    return formatDate(dateString);
}

// Truncate text with ellipsis
export function truncate(text, maxLength = 50) {
    if (!text) return '';
    text = String(text);
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength - 3) + '...';
}

// Debounce function calls
export function debounce(func, wait = 300) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Throttle function calls
export function throttle(func, limit = 300) {
    let inThrottle;
    return function executedFunction(...args) {
        if (!inThrottle) {
            func(...args);
            inThrottle = true;
            setTimeout(() => (inThrottle = false), limit);
        }
    };
}

// Generate a unique ID
export function generateId() {
    return Date.now().toString(36) + Math.random().toString(36).substring(2);
}

// Deep clone an object
export function deepClone(obj) {
    return JSON.parse(JSON.stringify(obj));
}

// Check if an object is empty
export function isEmpty(obj) {
    if (!obj) return true;
    if (Array.isArray(obj)) return obj.length === 0;
    if (typeof obj === 'object') return Object.keys(obj).length === 0;
    return false;
}

// Get nested object property safely
export function get(obj, path, defaultValue = undefined) {
    const keys = Array.isArray(path) ? path : path.split('.');
    let result = obj;
    for (const key of keys) {
        if (result === null || result === undefined) return defaultValue;
        result = result[key];
    }
    return result !== undefined ? result : defaultValue;
}

// Capitalize first letter
export function capitalize(str) {
    if (!str) return '';
    return String(str).charAt(0).toUpperCase() + String(str).slice(1);
}

// Convert to title case
export function titleCase(str) {
    if (!str) return '';
    return String(str)
        .toLowerCase()
        .split(' ')
        .map((word) => capitalize(word))
        .join(' ');
}

// Slugify a string
export function slugify(str) {
    if (!str) return '';
    return String(str)
        .toLowerCase()
        .trim()
        .replace(/[^\w\s-]/g, '')
        .replace(/[\s_-]+/g, '-')
        .replace(/^-+|-+$/g, '');
}

// Parse query string to object
export function parseQueryString(queryString) {
    const params = new URLSearchParams(queryString);
    const result = {};
    for (const [key, value] of params) {
        result[key] = value;
    }
    return result;
}

// Build query string from object
export function buildQueryString(params) {
    return new URLSearchParams(params).toString();
}

// Local storage helpers with JSON support
export const storage = {
    get: (key, defaultValue = null) => {
        try {
            const item = localStorage.getItem(key);
            return item ? JSON.parse(item) : defaultValue;
        } catch {
            return defaultValue;
        }
    },
    set: (key, value) => {
        try {
            localStorage.setItem(key, JSON.stringify(value));
            return true;
        } catch {
            return false;
        }
    },
    remove: (key) => {
        localStorage.removeItem(key);
    },
    clear: () => {
        localStorage.clear();
    },
};

// Session storage helpers with JSON support
export const session = {
    get: (key, defaultValue = null) => {
        try {
            const item = sessionStorage.getItem(key);
            return item ? JSON.parse(item) : defaultValue;
        } catch {
            return defaultValue;
        }
    },
    set: (key, value) => {
        try {
            sessionStorage.setItem(key, JSON.stringify(value));
            return true;
        } catch {
            return false;
        }
    },
    remove: (key) => {
        sessionStorage.removeItem(key);
    },
    clear: () => {
        sessionStorage.clear();
    },
};

// Copy text to clipboard
export async function copyToClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
        return true;
    } catch {
        return false;
    }
}

// Show a notification toast
export function showNotification(message, type = 'info', duration = 4000) {
    const container = document.getElementById('notification-container');
    if (!container) return;

    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.textContent = message;

    container.appendChild(notification);

    setTimeout(() => {
        notification.style.animation = 'slideOut 0.3s ease';
        setTimeout(() => notification.remove(), 300);
    }, duration);
}

// Clean album title by removing track prefix
export function cleanAlbumTitle(albumTitle, trackTitle) {
    if (!albumTitle) return 'Unknown Album';

    if (albumTitle.includes(' / ') && albumTitle.includes(trackTitle)) {
        const parts = albumTitle.split(' / ');
        return parts[parts.length - 1].trim();
    }

    return albumTitle;
}

// Normalize artist name - removes disambiguation suffixes like "(2)", "(3)", "(rapper)", etc.
export function normalizeArtistName(name) {
    if (!name) return '';
    return name.replace(/\s*\(\d+\)\s*$/, '').replace(/\s*\([^)]*(?:rapper|singer|artist|band|musician|producer|dj|DJ)\)\s*$/i, '').trim();
}

// Normalize title - removes edition suffixes like "(Remastered)", "(Deluxe)", etc.
export function normalizeTitle(title) {
    if (!title) return '';
    let normalized = title;
    // Apply up to 3 times to handle multiple suffixes
    for (let i = 0; i < 3; i++) {
        const prev = normalized;
        // Only match content inside parentheses at the end, like "(Remastered)" or "(Deluxe Edition)"
        normalized = normalized.replace(/\s*\((?:[^)]*\s)?(?:remaster(?:ed)?|digital|deluxe|bonus|anniversary|expanded|special|collector|limited|edition|version|mix|remix|mono|stereo|selected works|works|hits|best of|greatest|complete|original|enhanced)(?:\s[^)]*)?\)\s*$/gi, '').trim();
        if (normalized === prev) break;
    }
    return normalized;
}

// Make utility functions globally available for backward compatibility
window.escapeHtml = escapeHtml;
window.formatDuration = formatDuration;
window.formatDurationLong = formatDurationLong;
window.formatDate = formatDate;
window.formatRelativeTime = formatRelativeTime;
window.truncate = truncate;
window.debounce = debounce;
window.throttle = throttle;
window.generateId = generateId;
window.deepClone = deepClone;
window.isEmpty = isEmpty;
window.get = get;
window.capitalize = capitalize;
window.titleCase = titleCase;
window.slugify = slugify;
window.parseQueryString = parseQueryString;
window.buildQueryString = buildQueryString;
window.storage = storage;
window.session = session;
window.copyToClipboard = copyToClipboard;
window.showNotification = showNotification;
window.cleanAlbumTitle = cleanAlbumTitle;
window.normalizeArtistName = normalizeArtistName;
window.normalizeTitle = normalizeTitle;
