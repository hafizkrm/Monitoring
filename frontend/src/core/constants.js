// API Endpoints Configuration
export const API_ENDPOINTS = {
    METRICS: '/api/metrics',
    INVENTORY: '/api/inventory',
    DEVICES: '/api/devices',
    DEVICES_DELETE: '/api/devices/delete',
    DEVICES_UPDATE: '/api/devices/update',
    LOGS: '/api/logs',
    RELIABILITY: '/api/reliability',
    INTERFACES: '/api/interfaces',
    METRICS_HISTORY: '/api/metrics/history',
    HISTORY: '/api/metrics/history',
    PING: '/api/tools/ping',
    TRACE: '/api/tools/trace',

    // Phase 3: TSDB (Prometheus) Endpoints
    TSDB_DEVICE_HISTORY: '/api/tsdb/device-history',
    TSDB_QUERY: '/api/tsdb/query',
    TSDB_STATUS: '/api/tsdb/status',
};

// View Titles (used by router.js)
export const VIEW_TITLES = {
    dashboard: 'Dashboard',
    devices: 'Device Management',
    alerts: 'Alerts & Incident Center',
    reports: 'Reports',
    logs: 'Activity Logs',
    settings: 'Settings',
    users: 'User Management',
    integration: 'System Integration',
    backup: 'System Backup',
    docs: 'Documentation'
};

export const VIEW_SUBTITLES = {
    dashboard: 'Real-time Network Infrastructure Monitoring',
    devices: 'Manage and monitor all registered network devices',
    alerts: 'Monitor alerts and outage statuses on your network devices',
    reports: 'Generate network performance and availability reports',
    logs: 'View user activity logs and system events (audit trail)',
    settings: 'Configure system preferences, email, and SNMP parameters',
    users: 'Manage user access and permissions',
    integration: 'Connect with third-party services',
    backup: 'Backup and restore your NMS system configuration',
    docs: 'System user guide and documentation'
};
