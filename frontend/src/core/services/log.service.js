import { getCurrentUser } from './auth.service.js';

/**
 * Activity Logger Helper
 * Sends User Activity Logs to the backend securely and asynchronously.
 */
export const activityLogger = {
    /**
     * @param {Object} params
     * @param {string} params.module - The module name (e.g. 'User', 'devices')
     * @param {string} params.Action - The Action enum (CREATE, UPDATE, HAPUS, LOGIN, etc)
     * @param {string} params.Description - Human readable Description
     */
    log: async ({ module, Action, Description }) => {
        try {
            const User = getCurrentUser();
            if (!User) return; // Ignore if not logged in

            const payload = {
                UserId: User.id || null,
                Username: User.Username,
                Action: Action.toUpperCase(),
                module: module.toLowerCase(),
                Description: Description
            };

            const response = await fetch('/api/v2/activity-logs', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(payload)
            });

            if (!response.ok) {
                console.warn(`[ActivityLogger] Failed to log activity: ${response.statusText}`);
            }
        } catch (error) {
            // Non-blocking catch to ensure UX isn't interrupted by logging failures
            console.error('[ActivityLogger] Error logging activity:', error);
        }
    }
};
