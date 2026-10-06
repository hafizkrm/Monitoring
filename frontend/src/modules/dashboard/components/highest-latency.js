import { isOnline, matchesCategoryFilter } from '../../../shared/utils/helpers.js';
import { safeSetHTML } from '../stats.js';
import { metricsStore, uiStore } from '../../../core/state/store.js';

function getEl(id) {
    return document.getElementById(id);
}

export function initHighestLatency() {
    metricsStore.subscribe('fullMetrics', (metrics) => {
        updateHighestLatency(metrics);
    });

    uiStore.subscribe('latencyFilter', () => {
        updateHighestLatency(metricsStore.getState().fullMetrics);
    });
}

// Window global fallback
window.onLatencyCategoryChange = function(val) {
    uiStore.setState({ latencyFilter: val || 'all' });
};

function updateHighestLatency(metrics) {
    const container = getEl('highest-latency-container');
    if (!container) return;

    const list = (metrics && metrics.length > 0) ? metrics : (metricsStore.getState().fullMetrics || []);

    if (!list || list.length === 0) {
        container.innerHTML = `<div style="color:var(--text-muted); font-size:12px; text-align:center; padding:10px;">Belum ada perangkat terdaftar</div>`;
        return;
    }

    let devicesWithLatency = list.filter(m => {
        return isOnline(m.status) || parseFloat(m.latency || m.ping || m.latency_ms || m.latencyMs || 0) > 0;
    });

    const currentLatencyFilter = uiStore.getState().latencyFilter || 'all';

    if (currentLatencyFilter && currentLatencyFilter !== 'all') {
        devicesWithLatency = devicesWithLatency.filter(m => matchesCategoryFilter(m, currentLatencyFilter));
    }

    const sorted = devicesWithLatency.sort((a, b) => {
        const latA = parseFloat(a.latency || a.ping || a.latency_ms || a.latencyMs || 0);
        const latB = parseFloat(b.latency || b.ping || b.latency_ms || b.latencyMs || 0);
        return latB - latA;
    }).slice(0, 5);

    const listToDisplay = sorted.length > 0 ? sorted : list.slice(0, 5);

    const newHTML = listToDisplay.map((device) => {
        const rawLat = parseFloat(device.latency || device.ping || device.latency_ms || device.latencyMs || 1.5);
        const latencyStr = rawLat > 1000 ? `${(rawLat / 1000).toFixed(1)} ms` : `${rawLat.toFixed(1)} ms`;
        
        let color = '#f59e0b';
        let badgeBg = 'rgba(245, 158, 11, 0.12)';
        if (rawLat >= 100) {
            color = '#ef4444';
            badgeBg = 'rgba(239, 68, 68, 0.12)';
        } else if (rawLat >= 50) {
            color = '#f97316';
            badgeBg = 'rgba(249, 115, 22, 0.12)';
        }
        const icon = 'fa-tachometer-alt';
        const safeName = (device.name || device.ip_address || device.ip || 'Device').replace(/'/g, "\\'");

        return `
        <div class="premium-list-box" style="border-left: 3px solid ${color};">
            <div class="premium-box-icon" style="background: ${color}20; color: ${color};">
                <i class="fas ${icon}"></i>
            </div>
            <div class="premium-box-details">
                <span class="premium-box-title" title="${safeName}">${device.name || device.ip_address || device.ip || 'Device'}</span>
                <span class="premium-box-subtitle">${device.ip_address || device.ip || ''}</span>
            </div>
            <div class="premium-box-action" style="color: ${color}; background: ${badgeBg}; border: 1px solid ${color}33;">
                ${latencyStr}
            </div>
        </div>
        `;
    }).join('');

    safeSetHTML(container, newHTML);
}
