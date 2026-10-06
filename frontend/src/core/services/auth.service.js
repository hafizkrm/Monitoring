import { getStorageItem, setStorageItem, removeStorageItem, STORAGE_KEYS } from './storage.service.js';

// Configuration
const CONFIG = {
    SESSION_DURATION: 72 * 60 * 60 * 1000 // 72 hours
};

/**
 * API call for login
 * @param {string} Username 
 * @param {string} Password 
 * @param {boolean} remember
 * @returns {Promise<Object>} The session object
 */
export async function login(Username, Password, remember = true) {
    const res = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: Username.trim(), password: Password })
    });
    
    if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.message || 'Username atau Password salah');
    }
    
    const data = await res.json();
    
    // Create session object
    const session = {
        expiredAt: new Date(Date.now() + CONFIG.SESSION_DURATION).toISOString(),
        User: {
            id: data.user.id,
            Username: data.user.username,
            name: data.user.name,
            Role: data.user.role
        }
    };
    
    setStorageItem(STORAGE_KEYS.SESSION, session, remember);
    return session;
}

/**
 * Logout User
 */
export function logout() {
    removeStorageItem(STORAGE_KEYS.SESSION);
    fetch('/api/logout', { method: 'POST' }).catch(() => {});
}

/**
 * Check if session is valid and not expired
 * @returns {boolean}
 */
export function isAuthenticated() {
    const session = getStorageItem(STORAGE_KEYS.SESSION);
    if (!session) {
        return false;
    }
    
    const now = new Date();
    const expiry = new Date(session.expiredAt);
    
    if (now > expiry) {
        logout(); // Auto logout on expire
        return false;
    }
    
    return true;
}

/**
 * Get current logged in User object
 * @returns {Object|null}
 */
export function getCurrentUser() {
    if (!isAuthenticated()) {
        return null;
    }
    const session = getStorageItem(STORAGE_KEYS.SESSION);
    if (!session) return null;
    
    const user = session.User || session.user;
    if (user && !user.Role && user.role) {
        user.Role = user.role;
    }
    return user || null;
}
