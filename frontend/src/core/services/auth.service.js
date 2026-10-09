import { getStorageItem, setStorageItem, removeStorageItem, STORAGE_KEYS } from './storage.service.js';

const REMEMBERED_USERNAME_KEY = 'netmon_remembered_username';
const REFRESH_INTERVAL = 10 * 60 * 1000;
const ROLE_NAMES = new Set(['admin', 'viewer']);
let refreshTimer = null;
let refreshInFlight = null;
let lastRefreshAt = 0;
let connectionLost = false;

function normalizeUser(user) {
    if (!user || user.id == null || user.id === '' || Number(user.id) < 1 || !Number.isInteger(Number(user.id)) || !user.username || !ROLE_NAMES.has(user.role)) {
        throw new Error('Respons autentikasi tidak valid');
    }
    return {
        id: Number(user.id),
        Username: String(user.username),
        name: String(user.name || user.username),
        Role: user.role
    };
}

function saveSession(user) {
    const session = { User: normalizeUser(user) };
    setStorageItem(STORAGE_KEYS.SESSION, session, false);
    return session;
}

async function readJSON(response) {
    return response.json().catch(() => ({}));
}

export async function login(username, password, remember = false) {
    const response = await fetch('/api/login', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: username.trim(), password })
    });
    const data = await readJSON(response);
    if (!response.ok) throw new Error(data.message || 'Username atau password salah');

    const session = saveSession(data.user);
    try {
        if (remember) localStorage.setItem(REMEMBERED_USERNAME_KEY, username.trim());
        else localStorage.removeItem(REMEMBERED_USERNAME_KEY);
    } catch { /* Username memory is optional. */ }
    lastRefreshAt = Date.now();
    return session;
}

export function getRememberedUsername() {
    try { return localStorage.getItem(REMEMBERED_USERNAME_KEY) || ''; }
    catch { return ''; }
}

async function refreshSession() {
    if (refreshInFlight) return refreshInFlight;
    refreshInFlight = (async () => {
        const response = await fetch('/api/refresh', {
            method: 'POST',
            credentials: 'same-origin'
        });
        const data = await readJSON(response);
        if (response.status === 401) {
            removeStorageItem(STORAGE_KEYS.SESSION);
            window.dispatchEvent(new CustomEvent('auth-session-expired'));
            return null;
        }
        if (!response.ok) throw new Error(data.message || 'Refresh sesi gagal');
        lastRefreshAt = Date.now();
        if (connectionLost) {
            connectionLost = false;
            window.dispatchEvent(new CustomEvent('auth-session-restored'));
        }
        return saveSession(data.user).User;
    })().finally(() => { refreshInFlight = null; });
    return refreshInFlight;
}

export async function verifySession() {
    let response = await fetch('/api/session', { credentials: 'same-origin' });
    let data = await readJSON(response);
    if (response.status === 401) {
        const refreshedUser = await refreshSession();
        if (!refreshedUser) {
            removeStorageItem(STORAGE_KEYS.SESSION);
            return null;
        }
        return refreshedUser;
    }
    if (!response.ok) throw new Error(data.message || 'Pemeriksaan sesi gagal');
    lastRefreshAt = Date.now();
    return saveSession(data.user).User;
}

export function startSessionRefresh() {
    if (refreshTimer) return;
    let retrying = false;
    const refresh = async () => {
        try {
            const user = await refreshSession();
            if (!user) return;
            if (retrying) {
                clearInterval(refreshTimer);
                refreshTimer = setInterval(refresh, REFRESH_INTERVAL);
                retrying = false;
            }
        } catch {
            if (!connectionLost) {
                connectionLost = true;
                window.dispatchEvent(new CustomEvent('auth-connection-lost'));
            }
            if (!retrying) {
                clearInterval(refreshTimer);
                refreshTimer = setInterval(refresh, 30 * 1000);
                retrying = true;
            }
        }
    };
    refreshTimer = setInterval(refresh, REFRESH_INTERVAL);
    if (Date.now() - lastRefreshAt > 60 * 1000) refresh();
    document.addEventListener('visibilitychange', () => {
        if (!document.hidden && Date.now() - lastRefreshAt > 60 * 1000) refresh();
    });
}

export async function logout() {
    const response = await fetch('/api/logout', {
        method: 'POST',
        credentials: 'same-origin'
    });
    const data = await readJSON(response);
    if (!response.ok) throw new Error(data.message || 'Gagal mengakhiri sesi di server');
    if (refreshTimer) clearInterval(refreshTimer);
    refreshTimer = null;
    removeStorageItem(STORAGE_KEYS.SESSION);
}

export function isAuthenticated() {
    const session = getStorageItem(STORAGE_KEYS.SESSION);
    return Boolean(session?.User?.Username && ROLE_NAMES.has(session.User.Role));
}

export function getCurrentUser() {
    if (!isAuthenticated()) return null;
    return getStorageItem(STORAGE_KEYS.SESSION)?.User || null;
}
