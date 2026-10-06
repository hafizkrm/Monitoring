// Storage Service
// Handles localStorage interAction, keys, and versioning

export const STORAGE_KEYS = {
    SESSION: "netmon_session",
    USER: "netmon_User",
    PENGATURAN: "netmon_Settings",
    DB_VERSION: "netmon_db_version",
    APP_CONFIG: "netmon_app_config",
    AUDIO_MUTED: "netmon_audio_alert_muted",
    THEME: "nms_theme"
};

// Keys from older builds that must be purged (v2: plaintext password cache).
const LEGACY_KEYS = ["mon_User_Passwords", "mon_user_passwords"];

const CURRENT_DB_VERSION = 2;

/**
 * Initialize storage versioning
 */
export function initStorage() {
    const version = getStorageItem(STORAGE_KEYS.DB_VERSION);
    if (!version || version < CURRENT_DB_VERSION) {
        // v2: remove plaintext passwords cached by the old User module
        LEGACY_KEYS.forEach((key) => {
            try {
                localStorage.removeItem(key);
                sessionStorage.removeItem(key);
            } catch { /* storage unavailable */ }
        });
        setStorageItem(STORAGE_KEYS.DB_VERSION, CURRENT_DB_VERSION);
    }
}

/**
 * Save item to storage (localStorage or sessionStorage)
 * @param {string} key 
 * @param {any} value 
 * @param {boolean} isPersistent 
 */
export function setStorageItem(key, value, isPersistent = true) {
    try {
        const serializedValue = JSON.stringify(value);
        const storage = isPersistent ? localStorage : sessionStorage;
        storage.setItem(key, serializedValue);
        // Clean up from the other storage to avoid conflicting states
        const otherStorage = isPersistent ? sessionStorage : localStorage;
        otherStorage.removeItem(key);
    } catch (error) {
        console.error(`Error saving to storage (key: ${key}):`, error);
    }
}

/**
 * Get item from storage (checks localStorage then sessionStorage)
 * @param {string} key 
 * @returns {any}
 */
export function getStorageItem(key) {
    try {
        const serializedValue = localStorage.getItem(key) || sessionStorage.getItem(key);
        if (serializedValue === null) return null;
        return JSON.parse(serializedValue);
    } catch (error) {
        console.error(`Error reading from storage (key: ${key}):`, error);
        return null;
    }
}

/**
 * Remove item from all storage engines
 * @param {string} key 
 */
export function removeStorageItem(key) {
    try {
        localStorage.removeItem(key);
        sessionStorage.removeItem(key);
    } catch (error) {
        console.error(`Error removing from storage (key: ${key}):`, error);
    }
}

// Initialize on load
initStorage();
