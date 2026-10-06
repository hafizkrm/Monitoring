import { isOnline, matchesCategoryFilter } from '../../../shared/utils/helpers.js';
import { safeSetHTML } from '../stats.js';
import { metricsStore, uiStore } from '../../../core/state/store.js';

function getEl(id) {
    return document.getElementById(id);
}

export function initTopTraffic() {
    metricsStore.subscribe('fullMetrics', (metrics) => {
        updateAllTrafficStats(metrics);
    });

    uiStore.subscribe('topBwFilter', () => {
        updateAllTrafficStats(metricsStore.getState().fullMetrics);
    });
}

// Window global fallback
window.onTopBwCategoryChange = function(val) {
    uiStore.setState({ topBwFilter: val || 'all' });
};

function updateAllTrafficStats(metrics) {
    if (!metrics || metrics.length === 0) return;
    const container = getEl('top-devices-container');
    if (!container) return;

    let validMetrics = metrics.filter(m => {
        const rawType = (m.device_type || m.type || '').toLowerCase().replace(/_/g, ' ');
        const vendor = (m.vendor || '').toLowerCase();
        const isRadio = rawType.includes('radio') || vendor.includes('ubiquiti') || rawType.includes('airmax') || rawType.includes('wireless') || rawType.includes('ptp');
        return !isRadio;
    });

    const currentTopBwFilter = uiStore.getState().topBwFilter || 'all';

    if (currentTopBwFilter && currentTopBwFilter !== 'all') {
        validMetrics = validMetrics.filter(m => matchesCategoryFilter(m, currentTopBwFilter));
    }

    const mapped = validMetrics
        .map(m => {
            const rx = (parseFloat(m.rx_rate) || 0);
            const tx = (parseFloat(m.tx_rate) || 0);
            return { ...m, totalBw: rx + tx, name: (m.name || m.ip_address || m.ip || 'Unknown Device').toUpperCase() };
        });

    const sorted = mapped.sort((a, b) => {
        if (b.totalBw !== a.totalBw) return b.totalBw - a.totalBw;
        const aOnline = isOnline(a.status) ? 1 : 0;
        const bOnline = isOnline(b.status) ? 1 : 0;
        return bOnline - aOnline;
    }).slice(0, 5);

    const newHTML = sorted.map((device) => {
        const typeRaw = (device.device_type || device.type || '').toLowerCase();
        const vendor = (device.vendor || '').toLowerCase();
        const devName = (device.name || '').toLowerCase();
        
        let icon = 'fa-server';
        const mainColor = '#06b6d4';
        const accentColor = '#22d3ee';
        
        if (typeRaw.includes('switch') || devName.includes('cisco') || devName.includes('switch')) {
            icon = 'fa-network-wired';
        } else if (typeRaw.includes('access') || typeRaw.includes('ap') || vendor.includes('ruijie') || vendor.includes('tplink') || vendor.includes('tp-link')) {
            icon = 'fa-wifi';
        } else if (typeRaw.includes('firewall') || devName.includes('firewall')) {
            icon = 'fa-shield-alt';
        } else if (typeRaw.includes('server') || devName.includes('server')) {
            icon = 'fa-database';
        } else if (devName.includes(' - ') || devName.includes('trunk') || devName.includes('tower')) {
            icon = 'fa-project-diagram';
        }

        const badgeBg = 'rgba(6, 182, 212, 0.12)';
        const safeName = (device.name || device.ip_address || device.ip || 'Unknown Device').replace(/'/g, "\\'");

        const rxValRaw = parseFloat(device.rx_rate || 0);
        const txValRaw = parseFloat(device.tx_rate || 0);
        
        const formatBwValue = (val) => {
            if (val >= 1000) return `${(val / 1000).toFixed(0)} Gbps`;
            return `${val.toFixed(0)} Mbps`;
        };

        const formatRateDetail = (val) => {
            if (val >= 1000) return `${(val / 1000).toFixed(0)}G`;
            return `${val.toFixed(0)}`;
        };

        const totalBwStr = formatBwValue(device.totalBw);
        const rxValStr = formatRateDetail(rxValRaw);
        const txValStr = formatRateDetail(txValRaw);

        const subtitleStr = device.totalBw > 0 
            ? `<i class="fas fa-arrow-down" style="color:#38bdf8;"></i> ${rxValStr} Mbps &nbsp; <i class="fas fa-arrow-up" style="color:#34d399;"></i> ${txValStr} Mbps`
            : (isOnline(device.status) ? 'Traffic Idle' : 'Offline');

        return `
        <div class="premium-list-box" style="border-left: 3px solid ${mainColor};">
            <div class="premium-box-icon" style="background: rgba(6, 182, 212, 0.15); color: ${accentColor};">
                <i class="fas ${icon}"></i>
            </div>
            <div class="premium-box-details">
                <span class="premium-box-title" title="${safeName}">${device.name}</span>
                <span class="premium-box-subtitle">${subtitleStr}</span>
            </div>
            <div class="premium-box-action" style="color: ${accentColor}; background: ${badgeBg}; border: 1px solid rgba(6, 182, 212, 0.25);">
                ${totalBwStr}
            </div>
        </div>
        `;
    }).join('');

    safeSetHTML(container, newHTML);
}
