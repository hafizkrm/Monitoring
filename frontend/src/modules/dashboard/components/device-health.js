import { isOnline, matchesCategoryFilter } from '../../../shared/utils/helpers.js';
import { safeSetHTML } from '../stats.js';
import { metricsStore, uiStore } from '../../../core/state/store.js';

function getEl(id) {
    return document.getElementById(id);
}

export function initDeviceHealth() {
    metricsStore.subscribe('fullMetrics', (metrics) => {
        updateDeviceHealth(metrics);
    });

    uiStore.subscribe('healthFilter', () => {
        updateDeviceHealth(metricsStore.getState().fullMetrics);
    });
}

// Window global fallback
window.onHealthCategoryChange = function(val) {
    uiStore.setState({ healthFilter: val || 'all' });
};

function updateDeviceHealth(metrics) {
    const container = getEl('top-health-container');
    if (!container) return;

    const list = (metrics && metrics.length > 0) ? metrics : (metricsStore.getState().fullMetrics || []);
    let filtered = list.filter(m => isOnline(m.status));

    const currentHealthFilter = uiStore.getState().healthFilter || 'all';

    if (currentHealthFilter && currentHealthFilter !== 'all') {
        filtered = filtered.filter(m => matchesCategoryFilter(m, currentHealthFilter));
    }

    const mapped = filtered
        .map(m => {
            const cpu = parseFloat(m.cpu_usage || m.cpu || 0);
            const mem = parseFloat(m.memory_usage || m.memory || 0);
            
            let stressScore = cpu;
            let stressType = 'CPU';
            
            if (mem > cpu) {
                stressScore = mem;
                stressType = 'RAM';
            }

            return {
                ...m,
                cpuUsage: cpu,
                memUsage: mem,
                stressScore: stressScore,
                stressType: stressType,
                name: (m.name || m.ip_address || m.ip || 'Unknown Device').toUpperCase()
            };
        });

    const sorted = mapped.sort((a, b) => b.stressScore - a.stressScore).slice(0, 5);

    if (sorted.length === 0) {
        container.innerHTML = `<div style="display:flex; flex-direction:column; align-items:center; justify-content:center; padding:16px 8px; color:var(--text-muted); gap:6px;"><i class="fas fa-heartbeat" style="font-size:18px; color:var(--accent-green); opacity:0.8;"></i><span style="font-size:11px; font-weight:500;">All devices operating normally</span></div>`;
        return;
    }

    const newHTML = sorted.map((device) => {
        const scorePercent = Math.min(device.stressScore, 100);
        
        let color = '#10b981';
        let badgeBg = 'rgba(16, 185, 129, 0.12)';
        if (scorePercent >= 90) {
            color = '#ef4444';
            badgeBg = 'rgba(239, 68, 68, 0.12)';
        } else if (scorePercent >= 70) {
            color = '#f59e0b';
            badgeBg = 'rgba(245, 158, 11, 0.12)';
        }
        
        const icon = device.stressType === 'CPU' ? 'fa-microchip' : 'fa-memory';
        const safeName = (device.name || device.ip_address || device.ip || 'Unknown').replace(/'/g, "\\'");
        const ipStr = device.ip_address || device.ip ? `IP: ${device.ip_address || device.ip}` : 'Network Device';

        return `
        <div class="premium-list-box" style="border-left: 3px solid ${color};">
            <div class="premium-box-icon" style="background: ${color}20; color: ${color};">
                <i class="fas ${icon}"></i>
            </div>
            <div class="premium-box-details">
                <span class="premium-box-title" title="${safeName}">${device.name}</span>
                <span class="premium-box-subtitle">${ipStr}</span>
            </div>
            <div class="premium-box-action" style="color: ${color}; background: ${badgeBg}; border: 1px solid ${color}33;">
                ${device.stressScore.toFixed(1)}% ${device.stressType}
            </div>
        </div>
        `;
    }).join('');
    
    safeSetHTML(container, newHTML);
}
