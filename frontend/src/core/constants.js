// API Endpoints Configuration
export const API_ENDPOINTS = {
    METRICS: '/api/metrics',
    INVENTORY: '/api/inventory',
    DEVICES: '/api/devices',
    DEVICES_HAPUS: '/api/devices/delete',
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
    devices: 'Manajemen Perangkat',
    topology: 'Peta Jaringan',
    alerts: 'Alert & Notifikasi',
    Reports: 'Reports',
    logs: 'Logs Aktivitas',
    Settings: 'Settings',
    User: 'User Management',
    'device-details': 'Detail Analisis Node',
    integration: 'Integrasi Sistem',
    backup: 'Backup Sistem',
    docs: 'Dokumentasi'
};

export const VIEW_SUBTITLES = {
    dashboard: 'Monitoring jaringan secara real-Time',
    monitoring: 'Overview Status pemantauan jaringan',
    'mikrotik-ethernet': 'Pantau port dan lalu lintas data router',
    'radio-monitoring': 'Pantau Status dan sinyal perangkat radio link',
    'ap-monitoring': 'Pantau konektivitas dan klien access point',
    'firewall-monitoring': 'Pantau Status keamanan dan aturan firewall',
    'switch-monitoring': 'Pantau Status port dan interkoneksi switch',
    devices: 'Kelola dan pantau seluruh perangkat jaringan yang terdaftar',
    topology: 'Visualisasi interkoneksi antar perangkat jaringan',
    alerts: 'Pantau peringatan dan Status gangguan pada perangkat jaringan Anda',
    Reports: 'Hasilkan laporan kinerja dan ketersediaan perangkat jaringan',
    logs: 'Lihat catatan aktivitas user dan kejadian sistem (audit trail)',
    Settings: 'Konfigurasikan preferensi sistem, email, dan parameter SNMP',
    User: 'Kelola akses dan hak prerogatif tiap user',
    'device-details': 'Informasi lengkap dan pemantauan mendalam setiap titik jaringan',
    integration: 'Hubungkan dengan layanan pihak ketiga',
    backup: 'Cadangkan dan pulihkan konfigurasi sistem NMS Anda',
    docs: 'Panduan useran sistem'
};
