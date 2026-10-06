import { isOnline, formatUpTime } from '../../../shared/utils/helpers.js';
import { safeSetHTML } from '../stats.js';
import { metricsStore } from '../../../core/state/store.js';

function getEl(id) {
    return document.getElementById(id);
}

export function initRecentlyRebooted() {
    metricsStore.subscribe('fullMetrics', (metrics) => {
        updateRecentlyRebooted(metrics);
    });
}

function updateRecentlyRebooted(metrics) {
    const container = getEl('recently-reboot-container');
    if (!container) return;

    // Filter devices that are online and have upTime < 24 hours (86400 seconds)
    const recentlyRebooted = metrics.filter(m => {
        if (!isOnline(m.status) || !m.upTime) return false;
        const upTimeStr = String(m.upTime);
        
        let upTimeSeconds = 0;
        if (upTimeStr.endsWith('s')) {
            upTimeSeconds = parseFloat(upTimeStr.slice(0, -1));
        } else {
            upTimeSeconds = parseFloat(upTimeStr);
        }
        
        return upTimeSeconds > 0 && upTimeSeconds < 86400;
    });

    const sorted = recentlyRebooted.sort((a, b) => {
        const getUpTime = (val) => {
            const str = String(val.upTime);
            return str.endsWith('s') ? parseFloat(str.slice(0, -1)) : parseFloat(str);
        };
        return getUpTime(a) - getUpTime(b);
    }).slice(0, 5); // Take top 5

    if (sorted.length === 0) {
        container.innerHTML = `<div style="display:flex; flex-direction:column; align-items:center; justify-content:center; padding:16px 8px; color:var(--text-muted); gap:6px;"><i class="fas fa-check-circle" style="font-size:18px; color:var(--accent-green); opacity:0.8;"></i><span style="font-size:11px; font-weight:500;">Semua perangkat stabil (>24 Jam)</span></div>`;
        return;
    }

    const newHTML = sorted.map((device) => {
        const upTimeStr = String(device.upTime);
        let upTimeSeconds = upTimeStr.endsWith('s') ? parseFloat(upTimeStr.slice(0, -1)) : parseFloat(upTimeStr);
        
        let formattedUpTime = formatUpTime(upTimeSeconds);

        const isCritical = upTimeSeconds < 3600;
        const color = isCritical ? 'var(--accent-red)' : 'var(--accent-orange)';
        const badgeBg = isCritical ? '#dc2626' : '#d97706';
        const icon = isCritical ? 'fa-exclamation-triangle' : 'fa-info-circle';

        const safeName = (device.name || device.ip_address || device.ip || 'Unknown').replace(/'/g, "\\'");

        return `
        <div class="premium-list-box" style="border-left: 3px solid ${color};">
            <div class="premium-box-icon" style="background: ${color}20; color: ${color};">
                <i class="fas ${icon}"></i>
            </div>
            <div class="premium-box-details">
                <span class="premium-box-title" title="${safeName}">${device.name || device.ip_address || device.ip || 'Unknown'}</span>
                <span class="premium-box-subtitle">UpTime: ${formattedUpTime}</span>
            </div>
            <div class="premium-box-action" style="color: #ffffff; background: ${badgeBg}; padding: 3px 8px; border-radius: 4px; letter-spacing: 0.5px;">
                ${isCritical ? 'Baru Saja!' : '< 24 Jam'}
            </div>
        </div>
        `;
    }).join('');
    
    safeSetHTML(container, newHTML);
}
