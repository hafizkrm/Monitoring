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
        container.style.display = 'flex';
        container.style.flexDirection = 'column';
        container.style.justifyContent = 'center';
        container.innerHTML = `
        <div style="display:flex; flex-direction:column; align-items:center; justify-content:center; flex:1; width:100%; min-height:100px; padding:18px 12px; text-align:center; background:rgba(16, 185, 129, 0.05); border:1px solid rgba(16, 185, 129, 0.18); border-radius:10px; box-sizing:border-box;">
            <div style="position:relative; display:flex; align-items:center; justify-content:center; width:36px; height:36px; background:rgba(16, 185, 129, 0.12); border-radius:50%; margin-bottom:8px; box-shadow:0 0 12px rgba(16, 185, 129, 0.25);">
                <i class="fas fa-shield-halved" style="font-size:16px; color:#4ade80;"></i>
            </div>
            <span style="font-size:12px; font-weight:700; color:#4ade80; letter-spacing:0.4px;">SYSTEM STABLE</span>
            <span style="font-size:10.5px; color:#94a3b8; margin-top:3px;">No unhandled reboots in the last 24 hours</span>
        </div>`;
        return;
    } else {
        container.style.display = '';
        container.style.flexDirection = '';
        container.style.justifyContent = '';
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
                <span class="premium-box-subtitle">Uptime: ${formattedUpTime}</span>
            </div>
            <div class="premium-box-action" style="color: #ffffff; background: ${badgeBg}; padding: 3px 8px; border-radius: 4px; letter-spacing: 0.5px;">
                ${isCritical ? 'Just Now' : '< 24 Hours'}
            </div>
        </div>
        `;
    }).join('');
    
    safeSetHTML(container, newHTML);
}
