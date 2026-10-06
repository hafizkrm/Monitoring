// Dashboard Stats Module
// Dipanggil oleh polling.service.js setiap 2 detik

import { isOnline, formatUpTime } from '../../shared/utils/helpers.js';
import { updateDashboardCharts } from './dashboard.js';
import { initSwitchPortsFromMetrics } from './switch-ports.js';
import { initNetworkSummary } from './components/network-summary.js';
import { initTopTraffic } from './components/top-traffic.js';
import { initDeviceHealth } from './components/device-health.js';
import { initRecentlyRebooted } from './components/recently-rebooted.js';
import { initHighestLatency } from './components/highest-latency.js';
import { initDeviceTable } from './components/device-table.js';
import { initVPNUsers } from './components/vpn-users.js';
import { initTopInterfaces } from './components/top-interfaces.js';
import { metricsStore } from '../../core/state/store.js';
import { state } from '../../core/state.js';

// DOM Delta Update Helper to prevent layout thrashing
export const safeSetHTML = (el, html) => {
    if (el && el.innerHTML !== html) el.innerHTML = html;
};

let globalTrueDeviceTotal = 0;
async function fetchTrueDeviceTotal() {
    try {
        const res = await fetch('/api/inventory?page=1&limit=1');
        const data = await res.json();
        if (data && data.total) {
            globalTrueDeviceTotal = data.total;
            window.globalTrueDeviceTotal = data.total;
            if (state && state.pagination && state.pagination.devices) {
                state.pagination.devices.total = data.total;
            }
        }
    } catch (e) { }
}
fetchTrueDeviceTotal();

export const safeSetText = (idOrEl, text) => {
    const el = typeof idOrEl === 'string' ? document.getElementById(idOrEl) : idOrEl;
    if (el) {
        const str = String(text ?? '');
        if (el.textContent !== str) el.textContent = str;
    }
};

// --- DOM CACHE ---
function getEl(id) {
    return document.getElementById(id);
}
// -----------------

export function updateDashboardStats(metrics) {
    if (!metrics || !Array.isArray(metrics) || metrics.length === 0) return;
    
    // Update Store — this triggers all subscribed components
    metricsStore.setState({ fullMetrics: metrics });
    
    // Normalize uptime field from backend API to match frontend upTime expectations
    metrics.forEach(m => {
        if (m.uptime !== undefined && m.upTime === undefined) {
            m.upTime = m.uptime;
        }
    });

    // Update dynamic switch port visualization from real SNMP data
    if (!window._lastSwitchPortFetchTime || Date.now() - window._lastSwitchPortFetchTime > 5000) {
        initSwitchPortsFromMetrics(metrics);
        window._lastSwitchPortFetchTime = Date.now();
    }

    const total = Math.max(metrics.length, state?.pagination?.devices?.total || 0, globalTrueDeviceTotal);
    let onlineCount = 0;
    let offlineCount = 0;
    let disabledCount = 0;

    // Single Source of Truth Loop for Device Online/Offline Counts
    metrics.forEach(m => {
        const Status = (m.status || '').toLowerCase();
        if (Status === 'disabled' || Status === 'maintenance') {
            disabledCount++;
        } else if (isOnline(m.status)) {
            onlineCount++;
        }
    });

    offlineCount = Math.max(0, total - onlineCount - disabledCount);

    if (getEl('stat-total-devices')) getEl('stat-total-devices').innerText = total;
    if (getEl('stat-online')) getEl('stat-online').innerText = onlineCount;
    if (getEl('stat-offline')) getEl('stat-offline').innerText = offlineCount;

    if (total > 0) {
        const onPct = ((onlineCount / total) * 100).toFixed(1);
        const offPct = ((offlineCount / total) * 100).toFixed(1);
        if (getEl('stat-online-percentage')) getEl('stat-online-percentage').innerText = `${onPct}% Online`;
        if (getEl('stat-offline-percentage')) getEl('stat-offline-percentage').innerText = `${offPct}% Offline`;
    } else {
        if (getEl('stat-online-percentage')) getEl('stat-online-percentage').innerText = `0% Online`;
        if (getEl('stat-offline-percentage')) getEl('stat-offline-percentage').innerText = `0% Offline`;
    }

    // Hitung total bandwidth HANYA dari perangkat Router/Firewall (Gateway)
    // Untuk mencegah Double Counting (duplikasi) dengan Switch & AP di bawahnya
    let totalBw = 0;
    let totalRx = 0;
    let totalTx = 0;
    let excludedCount = 0;
    metrics.forEach(m => {
        const devType = (m.device_type || m.type || '').toLowerCase();
        const isGateway = devType.includes('router') || devType.includes('firewall');
        if (!isGateway) { excludedCount++; return; }
        const rx = parseFloat(m.rx_rate ?? 0);
        const tx = parseFloat(m.tx_rate ?? 0);
        totalRx += rx;
        totalTx += tx;
        totalBw += (rx + tx);
    });
    function formatBwStr(val) {
        if (!val || isNaN(val) || val <= 0) return '0 Mbps';
        if (val >= 1000) return `${(val / 1000).toFixed(2)} Gbps`;
        if (val >= 10) return `${val.toFixed(0)} Mbps`;
        if (val >= 1) return `${val.toFixed(1)} Mbps`;
        return `${(val * 1000).toFixed(0)} Kbps`;
    }

    if (getEl('stat-bandwidth')) {
        getEl('stat-bandwidth').innerText = formatBwStr(totalBw);
    }
    if (getEl('stat-bandwidth-download')) {
        getEl('stat-bandwidth-download').innerText = formatBwStr(totalRx);
    }
    if (getEl('stat-bandwidth-upload')) {
        getEl('stat-bandwidth-upload').innerText = formatBwStr(totalTx);
    }
    // Tampilkan info perangkat yang dieksklusi agar User tahu
    const radioBadgeEl = getEl('stat-bandwidth-note');
    if (radioBadgeEl && excludedCount > 0) {
        const noteText = `*${excludedCount} Perangkat non-Router tidak digabung (Mencegah Double Counting)`;
        radioBadgeEl.innerText = noteText;
        radioBadgeEl.title = noteText;
        radioBadgeEl.style.display = 'block';
    }

    // Alert badge logic has been moved to index.html (monRenderAlerts) to support the unified 5-tier alert system

    const online = onlineCount;
    if (total > 0) {
        const upTimePercent = ((online / total) * 100).toFixed(1);
        if (getEl('stat-upTime')) {
            getEl('stat-upTime').innerText = `${upTimePercent}%`;
        }
    } else {
        if (getEl('stat-upTime')) {
            getEl('stat-upTime').innerText = `0%`;
        }
    }

    updateDashboardCharts(metrics);
    metricsStore.setState({ fullMetrics: metrics });

    if (!window._lastExternalFetchTime || Date.now() - window._lastExternalFetchTime > 5000) {
        if (typeof window.monRenderAlerts === 'function') {
            window.monRenderAlerts();
        }
        window._lastExternalFetchTime = Date.now();
    }
    window.dispatchEvent(new Event('metrics-updated'));
}
window.updateDashboardStats = updateDashboardStats;

export function updateSLA(data) {
    if (!data || data.length === 0) return;
    const total = data.length;
    const online = data.filter(d => isOnline(d.status)).length;
    const percentage = (online / total) * 100;

    const textEl = getEl('sla-percentage');
    if (textEl) textEl.innerText = `${percentage.toFixed(1)}%`;
}

export function updateLiveStats(device) { }

// Global listener for WebSocket Real-Time updates
let metricsUpdateTimer = null;
let latestMetrics = null;

window.addEventListener('metrics-updated', (e) => {
    if (e.detail && Array.isArray(e.detail)) {
        latestMetrics = e.detail;
        if (!metricsUpdateTimer) {
            metricsUpdateTimer = setTimeout(() => {
                updateDashboardStats(latestMetrics);
                updateSLA(latestMetrics);
                if (latestMetrics.length > 0) updateLiveStats(latestMetrics[0]);
                metricsUpdateTimer = null;
            }, 1000); // Batasi render DOM maksimal 1x per detik
        }
    }
});

// Initialize all dashboard components that subscribe to the store
initNetworkSummary();
initTopTraffic();
initDeviceHealth();
initRecentlyRebooted();
initHighestLatency();
initDeviceTable();
initVPNUsers();
initTopInterfaces();
