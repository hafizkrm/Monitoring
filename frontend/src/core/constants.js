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
    monitoring: 'Overview Monitoring',
    'mikrotik-ethernet': 'Monitoring Router',
    'radio-monitoring': 'Monitoring Radio',
    'ap-monitoring': 'Monitoring Access Point',
    'firewall-monitoring': 'Monitoring Firewall',
    'switch-monitoring': 'Monitoring Switch',
    devices: 'Device Management',
    topology: 'Network Topology Map',
    alerts: 'Alerts & Incident Center',
    Reports: 'Reports',
    logs: 'Activity Logs',
    Settings: 'Settings',
    User: 'User Management',
    'device-details': 'Node Analysis Detail',
    integration: 'System Integration',
    backup: 'System Backup',
    docs: 'Documentation'
};

export const VIEW_SUBTITLES = {
    dashboard: 'Real-time Network Infrastructure Monitoring',
    monitoring: 'Overview of network monitoring status',
    'mikrotik-ethernet': 'Monitor router ports and data traffic',
    'radio-monitoring': 'Monitor radio link status and signal',
    'ap-monitoring': 'Monitor access point connectivity and active clients',
    'firewall-monitoring': 'Monitor security status and firewall rules',
    'switch-monitoring': 'Monitor switch port status and interconnections',
    devices: 'Manage and monitor all registered network devices',
    topology: 'Visualization of network device interconnections',
    alerts: 'Monitor alerts and outage statuses on your network devices',
    Reports: 'Generate network performance and availability reports',
    logs: 'View user activity logs and system events (audit trail)',
    Settings: 'Configure system preferences, email, and SNMP parameters',
    User: 'Manage user access and permissions',
    'device-details': 'Complete information and in-depth monitoring for each network node',
    integration: 'Connect with third-party services',
    backup: 'Backup and restore your NMS system configuration',
    docs: 'System user guide and documentation'
};
