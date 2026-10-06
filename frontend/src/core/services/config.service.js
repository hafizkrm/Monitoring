import { getStorageItem, setStorageItem, STORAGE_KEYS } from './storage.service.js';

const CONFIG_SCHEMA_VERSION = 1;

/**
 * Default configuration structure
 */
export const DEFAULT_CONFIG = {
    general: {
        pollInterval: 10,
        logRetention: 7,
        Timezone: "Asia/Jakarta"
    },
    smtp: {
        host: "",
        port: 587,
        Username: "",
        Password: "",
        tls: true
    },
    snmp: {
        version: "v2c",
        community: "public",
        Timeout: 5,
        retries: 2
    },
    backup: {
        enabled: false,
        schedule: "daily"
    }
};

/**
 * Validates a configuration object
 * @param {Object} config The config payload to validate
 * @returns {Array} List of validation error Messages (empty if valid)
 */
export function validateConfig(config) {
    const errors = [];
    
    // Check required categories
    const requiredCategories = ['general', 'smtp', 'snmp', 'backup'];
    for (const cat of requiredCategories) {
        if (!config[cat]) {
            errors.push(`Missing required configuration category: ${cat}`);
        }
    }

    if (errors.length > 0) return errors; // Abort deeper checks if structure is entirely wrong

    // General Validation
    if (config.general.pollInterval < 5 || config.general.pollInterval > 3600) {
        errors.push("General: Polling interval must be between 5 and 3600 seconds.");
    }
    if (config.general.logRetention < 1) {
        errors.push("General: Log retention must be at least 1 day.");
    }

    // SMTP Validation
    const smtpPort = Number(config.smtp.port);
    if (config.smtp.host !== "" && (isNaN(smtpPort) || smtpPort < 1 || smtpPort > 65535)) {
        errors.push("SMTP: Port must be a valid number between 1 and 65535.");
    }

    // SNMP Validation
    const snmpTimeout = Number(config.snmp.Timeout);
    if (isNaN(snmpTimeout) || snmpTimeout < 1) {
        errors.push("SNMP: Timeout must be at least 1 second.");
    }

    return errors;
}

/**
 * Migrates a configuration object if schemaVersion is older
 */
export function migrateConfig(schemaVersion, payload) {
    let currentVersion = schemaVersion || 0;
    let migratedPayload = JSON.parse(JSON.stringify(payload)); // Deep copy

    // Future-proofing: Example migration block
    // if (currentVersion === 1) {
    //     // migrate from v1 to v2
    //     currentVersion = 2;
    // }

    return migratedPayload;
}

/**
 * Loads configuration from backend. If missing, returns default.
 */
export async function loadConfig() {
    try {
        const res = await fetch('/api/settings');
        if (res.ok) {
            const result = await res.json();
            if (result.data && Object.keys(result.data).length > 0) {
                const merged = JSON.parse(JSON.stringify(DEFAULT_CONFIG));
                for (const cat of Object.keys(result.data)) {
                    merged[cat] = result.data[cat];
                }
                return merged;
            }
        }
    } catch (e) {
        console.error("Failed to load config from backend", e);
    }
    
    return JSON.parse(JSON.stringify(DEFAULT_CONFIG));
}

/**
 * Saves a specific category of configuration to backend
 */
export async function SaveConfig(category, data) {
    // Validate first (requires current full config which we might not have synchronously, so we skip full config validation or do it partially)
    // For simplicity, we assume frontend input validation handles most. 
    
    const res = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: category, value: data })
    });
    
    if (!res.ok) {
        throw new Error('Gagal menyimpan konfigurasi ke server');
    }
    return true;
}

/**
 * Resets a specific category to its default values
 */
export async function resetConfig(category) {
    const currentConfig = await loadConfig();
    currentConfig[category] = JSON.parse(JSON.stringify(DEFAULT_CONFIG[category]));
    await SaveConfig(category, currentConfig[category]);
    return currentConfig[category];
}

/**
 * Resets entirely to factory defaults
 */
export async function resetAllConfig() {
    for (const cat of Object.keys(DEFAULT_CONFIG)) {
        await SaveConfig(cat, DEFAULT_CONFIG[cat]);
    }
    return JSON.parse(JSON.stringify(DEFAULT_CONFIG));
}

/**
 * Generates an exportable JSON configuration (including metadata and User)
 */
export async function exportConfig() {
    const currentConfig = await loadConfig();
    
    let User = [];
    try {
        const res = await fetch('/api/users');
        if (res.ok) {
            const data = await res.json();
            User = data.data || [];
        }
    } catch (e) {}
    
    const exportData = {
        metadata: {
            schemaVersion: CONFIG_SCHEMA_VERSION,
            appVersion: "1.0.0",
            exportedAt: new Date().toISOString(),
            createdBy: "Administrator" // Mock metadata
        },
        payload: {
            Settings: currentConfig,
            User: User
        }
    };
    
    return JSON.stringify(exportData, null, 2);
}

/**
 * Validates and imports a JSON configuration string
 */
export async function importConfig(jsonString) {
    try {
        const parsed = JSON.parse(jsonString);
        
        if (!parsed.metadata || !parsed.payload) {
            throw new Error("Invalid format: Missing metadata or payload block.");
        }

        const schemaVersion = parsed.metadata.schemaVersion || 1;
        let incomingSettings = parsed.payload.Settings;
        const incomingUser = parsed.payload.User;

        // Migrate if necessary
        if (schemaVersion < CONFIG_SCHEMA_VERSION) {
            incomingSettings = migrateConfig(schemaVersion, incomingSettings);
        }

        // Validate structure
        const errors = validateConfig(incomingSettings);
        if (errors.length > 0) {
            throw new Error("Validation failed:\n" + errors.join('\n'));
        }

        // Save safely
        for (const key of Object.keys(incomingSettings)) {
            await SaveConfig(key, incomingSettings[key]);
        }
        
        if (Array.isArray(incomingUser)) {
            for (const u of incomingUser) {
                // Best effort to import User
                try {
                    await fetch('/api/users', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(u)
                    });
                } catch (e) {}
            }
        }

        return {
            success: true,
            summary: {
                User: incomingUser ? incomingUser.length : 0,
                categories: Object.keys(incomingSettings).length
            }
        };
    } catch (e) {
        throw new Error(`Import failed: ${e.message}`);
    }
}
