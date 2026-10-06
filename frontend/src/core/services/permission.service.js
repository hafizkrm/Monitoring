import { getCurrentUser } from './auth.service.js';

// Access Control List
export const ACL = {
    admin: [
        "dashboard",
        "devices",
        "alerts",
        "Reports",
        "logs",
        "Settings",
        "User",
        "integration",
        "backup",
        "docs",
        "mikrotik-ethernet",
        "monitoring"
    ],
    view: [
        "dashboard",
        "devices",
        "alerts",
        "Reports",
        "monitoring"
    ]
};

/**
 * Check if current User has access to a specific view/module
 * @param {string} viewName 
 * @returns {boolean}
 */
export function canAccess(viewName) {
    const User = getCurrentUser();
    if (!User || !User.Role) return false;
    
    const RolePermissions = ACL[User.Role];
    if (!RolePermissions) return false;
    
    return RolePermissions.includes(viewName);
}

/**
 * Check if current User is an admin
 * @returns {boolean}
 */
export function isAdmin() {
    const User = getCurrentUser();
    return User && User.Role === 'admin';
}
