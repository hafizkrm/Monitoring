import { isOnline, formatUpTime } from '../../../shared/utils/helpers.js';
import { safeSetHTML, safeSetText } from '../stats.js';
import { metricsStore, uiStore } from '../../../core/state/store.js';

function getEl(id) {
    return document.getElementById(id);
}

export function initDeviceTable() {
    metricsStore.subscribe('fullMetrics', (metrics) => {
        updateDevicesTable(metrics);
    });

    uiStore.subscribe('deviceFilter', () => {
        updateDevicesTable(metricsStore.getState().fullMetrics);
    });
}

export function setDeviceFilter(filter) {
    uiStore.setState({ deviceFilter: filter || 'Semua' });
    
    // Update button active states in filter-pills and controls
    document.querySelectorAll('.filter-pills .filter-pill, .controls .btn-sm').forEach(btn => {
        btn.classList.remove('active');
        btn.style.background = '';
        btn.style.border = '';
        btn.style.color = '';
    });
    const filterLower = (filter || 'Semua').toLowerCase();
    const activeBtns = document.querySelectorAll(`
        .filter-pills .filter-pill[onclick*="'${filter}'"],
        .filter-pills .filter-pill[onclick*="'${filterLower}'"],
        .controls .btn-sm[onclick*="'${filter}'"]
    `);
    activeBtns.forEach(btn => {
        btn.classList.add('active');
    });
}

// Global fallback for onclick attributes in HTML
window.setDeviceFilter = setDeviceFilter;

function updateDevicesTable(metrics) {
    const tbody = getEl('dashboard-devices-table-body');
    if (!tbody) return;

    const baseList = (metrics && metrics.length > 0) ? metrics 
                   : (metricsStore.getState().fullMetrics || []);

    if (!baseList || baseList.length === 0) return;

    // Update filter pill counts dynamically
    if (Array.isArray(baseList)) {
        const counts = { all: baseList.length, router: 0, radio: 0, switch: 0, ap: 0, server: 0, firewall: 0 };
        baseList.forEach(m => {
            const rawType = (m.device_type || m.type || '').toLowerCase().replace(/_/g, ' ');
            const vendor = (m.vendor || '').toLowerCase();
            if (rawType.includes('router') || vendor.includes('mikrotik') || rawType === 'router') counts.router++;
            else if (rawType.includes('switch')) counts.switch++;
            else if (rawType.includes('radio') || vendor.includes('ubiquiti') || rawType.includes('airmax')) counts.radio++;
            else if (rawType.includes('access') || rawType.includes('ap') || vendor.includes('ruijie') || vendor.includes('tplink') || vendor.includes('tp-link')) counts.ap++;
            else if (rawType.includes('firewall') || rawType.includes('security')) counts.firewall++;
            else if (rawType.includes('server')) counts.server++;
            else counts.router++;
        });

        safeSetText('pill-count-all', counts.all);
        safeSetText('pill-count-router', counts.router);
        safeSetText('pill-count-radio', counts.radio);
        safeSetText('pill-count-switch', counts.switch);
        safeSetText('pill-count-ap', counts.ap);
        safeSetText('pill-count-server', counts.server);
        safeSetText('pill-count-firewall', counts.firewall);
    }

    const currentDeviceFilter = uiStore.getState().deviceFilter || 'Semua';
    let filtered = baseList;
    
    if (currentDeviceFilter && currentDeviceFilter !== 'Semua' && currentDeviceFilter !== 'all') {
        const filterLower = currentDeviceFilter.toLowerCase().replace(/_/g, ' ');
        filtered = baseList.filter(m => {
            const rawType = (m.device_type || m.type || '').toLowerCase().replace(/_/g, ' ');
            const vendor = (m.vendor || '').toLowerCase();
            
            if (filterLower === 'router') {
                return rawType.includes('router') || vendor.includes('mikrotik') || rawType === 'router';
            } else if (filterLower === 'radio') {
                return rawType.includes('radio') || vendor.includes('ubiquiti') || rawType.includes('airmax') || rawType === 'radio';
            } else if (filterLower === 'switch') {
                return rawType.includes('switch');
            } else if (filterLower === 'access point' || filterLower === 'access_point' || filterLower === 'ap') {
                return rawType.includes('access') || rawType.includes('ap') || vendor.includes('ruijie') || vendor.includes('tplink') || vendor.includes('tp-link');
            } else if (filterLower === 'server') {
                return rawType.includes('server');
            } else if (filterLower === 'firewall') {
                return rawType.includes('firewall');
            }
            return rawType.includes(filterLower);
        });
    }

    // Sort by latest updated / collected Timestamp first, then id descending
    const getDevTimestamp = (m) => {
        const tStr = m.collected_at || m.updated_at || m.last_polled_at || m.created_at || '';
        if (tStr) {
            const parsed = Date.parse(tStr);
            if (!isNaN(parsed)) return parsed;
        }
        return 0;
    };
    const getDevId = (m) => {
        const idVal = m.id || m.raw_id || 0;
        const num = parseInt(idVal, 10);
        return isNaN(num) ? 0 : num;
    };

    const sortedDesc = [...filtered].sort((a, b) => {
        const TimeA = getDevTimestamp(a);
        const TimeB = getDevTimestamp(b);
        if (TimeB !== TimeA) {
            return TimeB - TimeA;
        }
        return getDevId(b) - getDevId(a);
    });

    // Display top 5 recent devices
    const recent = sortedDesc.slice(0, 5);

    if (recent.length === 0) {
        safeSetHTML(tbody, `<tr><td colspan="5" style="text-align:center; padding:16px; color:var(--text-muted);">Tidak ada perangkat untuk kategori ini</td></tr>`);
        return;
    }

    const newHTML = recent.map(m => {
        const isMOnline = isOnline(m.status);
        const StatusColor = isMOnline ? 'var(--accent-green)' : 'var(--accent-red)';
        const StatusText = isMOnline ? 'Online' : 'Offline';
        
        let typeRaw = (m.device_type || m.type || 'Router').toLowerCase().replace(/_/g, ' ');
        let typeDisplay = 'Router';
        let typeIcon = 'fa-server';
        let iconColor = '#3b82f6'; // default blue

        if (typeRaw.includes('switch')) {
            typeDisplay = 'Switch';
            typeIcon = 'fa-network-wired';
            iconColor = '#8b5cf6';
        } else if (typeRaw.includes('radio') || (m.vendor && m.vendor.toLowerCase().includes('ubiquiti'))) {
            typeDisplay = 'Radio';
            typeIcon = 'fa-broadcast-tower';
            iconColor = '#f59e0b';
        } else if (typeRaw.includes('access') || typeRaw.includes('ap') || (m.vendor && (m.vendor.toLowerCase().includes('ruijie') || m.vendor.toLowerCase().includes('tplink')))) {
            typeDisplay = 'Access Point';
            typeIcon = 'fa-wifi';
            iconColor = '#06b6d4';
        } else if (typeRaw.includes('firewall')) {
            typeDisplay = 'Firewall';
            typeIcon = 'fa-shield-alt';
            iconColor = '#ef4444';
        } else if (typeRaw.includes('server')) {
            typeDisplay = 'Server';
            typeIcon = 'fa-database';
            iconColor = '#ec4899';
        } else {
            typeDisplay = 'Router';
            typeIcon = 'fa-server';
            iconColor = '#3b82f6';
        }

        let upTimeLastSeen = '-';
        if (isMOnline) {
            if (m.upTime !== undefined && m.upTime !== null && m.upTime !== '' && m.upTime > 0) {
                upTimeLastSeen = formatUpTime(m.upTime);
            } else {
                upTimeLastSeen = '<span style="color:var(--accent-green); font-size:11px; font-weight:500;"><i class="fas fa-check-circle" style="font-size:10px;"></i> Active</span>';
            }
        } else {
            let lastDown = m.last_down;
            if (lastDown && lastDown !== '0001-01-01 00:00:00 +0000 UTC') {
                try {
                    const d = new Date(lastDown);
                    upTimeLastSeen = `<span style="color:var(--accent-red); font-size:9px; line-height:1.2; display:block;">Offline Sejak:<br>${d.toLocaleDateString('id-ID', {day:'2-digit',month:'short'})} ${d.toLocaleTimeString('id-ID', {hour:'2-digit', minute:'2-digit'})}</span>`;
                } catch(e) {}
            } else {
                upTimeLastSeen = `<span style="color:var(--text-muted); font-size:10px;">Offline</span>`;
            }
        }
        
        const safeName = (m.name || m.ip || m.ip_address || 'Device').replace(/'/g, "\\'");

        return `
            <tr>
                <td>
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas ${typeIcon}" style="color: ${iconColor}; font-size: 13px;"></i>
                        <span style="font-weight: 500; text-overflow:ellipsis; overflow:hidden; white-space:nowrap; display:inline-block; max-width:100%; flex:1; min-width:0;" title="${safeName}">${m.name || m.ip || m.ip_address || 'Device'}</span>
                    </div>
                </td>
                <td style="color: var(--text-muted); text-align: center; font-family: monospace;">${m.ip || m.ip_address || '0.0.0.0'}</td>
                <td style="color: var(--text-muted); text-align: center;">${typeDisplay}</td>
                <td style="text-align: center;">
                    <div style="display: flex; align-items: center; justify-content: center; gap: 6px;">
                        <span style="width: 8px; height: 8px; border-radius: 50%; background: ${StatusColor}; box-shadow: ${isMOnline ? '0 0 4px ' + StatusColor : 'none'};"></span>
                        <span style="color: ${StatusColor}; font-weight: 500;">${StatusText}</span>
                    </div>
                </td>
                <td style="color: var(--text-muted); text-align: center;">${upTimeLastSeen}</td>
            </tr>
        `;
    }).join('');
    
    safeSetHTML(tbody, newHTML);
}
