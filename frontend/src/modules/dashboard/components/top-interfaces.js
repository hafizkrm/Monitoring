// Top Interfaces Component
// Menampilkan interface dengan traffic tertinggi

import { safeSetHTML } from '../stats.js';
import { uiStore } from '../../../core/state/store.js';

function getEl(id) {
    return document.getElementById(id);
}

let cachedTopInterfacesData = [];
let lastInterfaceFetchTime = 0;

export function initTopInterfaces() {
    // Poll top interfaces setiap 10 detik
    window.addEventListener('metrics-updated', () => {
        if (Date.now() - lastInterfaceFetchTime > 10000) {
            fetchTopInterfaces();
            lastInterfaceFetchTime = Date.now();
        }
    });

    uiStore.subscribe('interfaceFilter', () => {
        renderTopInterfaces();
    });
}

// Window global fallback
window.onInterfaceCategoryChange = function(val) {
    uiStore.setState({ interfaceFilter: val || 'all' });
};

async function fetchTopInterfaces() {
    const container = getEl('top-interfaces-container');
    if (!container) return;

    try {
        const res = await fetch('/api/top-interfaces');
        if (!res.ok) throw new Error('API error');
        
        const data = await res.json();
        cachedTopInterfacesData = Array.isArray(data) ? data : [];
        renderTopInterfaces();
    } catch (err) {
        console.error('Failed to fetch top interfaces:', err);
        container.innerHTML = '<div style="font-size:12px; color:var(--accent-red); text-align:center; padding:15px;">Failed to load data</div>';
    }
}

function renderTopInterfaces() {
    const container = getEl('top-interfaces-container');
    if (!container) return;

    if (!cachedTopInterfacesData || cachedTopInterfacesData.length === 0) {
        container.innerHTML = '<div style="font-size:12px; color:var(--text-muted); text-align:center; padding:15px;">No interface metric data available</div>';
        return;
    }

    let list = [...cachedTopInterfacesData];

    const currentInterfaceFilter = uiStore.getState().interfaceFilter || 'all';

    if (currentInterfaceFilter && currentInterfaceFilter !== 'all') {
        const filterVal = currentInterfaceFilter.toLowerCase();
        list = list.filter(iface => {
            const ifName = (iface.interface_name || '').toLowerCase();
            const ifAlias = (iface.interface_alias || '').toLowerCase();
            const devName = (iface.device_name || '').toLowerCase();
            const combined = `${ifName} ${ifAlias} ${devName}`;

            if (filterVal === 'physical') {
                const isTrunkOrVirt = combined.includes('vlan') || combined.includes('bridge') || combined.includes('bond') || combined.includes('trunk') || combined.includes('tun') || combined.includes('ppp') || combined.includes('l2tp');
                return !isTrunkOrVirt;
            } else if (filterVal === 'trunk') {
                return combined.includes('vlan') || combined.includes('bridge') || combined.includes('bond') || combined.includes('trunk') || combined.includes('combo') || combined.includes('rumdin') || combined.includes('pusat');
            } else if (filterVal === 'virtual') {
                return combined.includes('tun') || combined.includes('tap') || combined.includes('ppp') || combined.includes('l2tp') || combined.includes('gre') || combined.includes('ovpn') || combined.includes('vpn');
            }
            return true;
        });
    }

    if (list.length === 0) {
        container.innerHTML = '<div style="font-size:11px; color:var(--text-muted); text-align:center; padding:15px;">Tidak ada interface untuk tipe ini</div>';
        return;
    }

    const topList = list.slice(0, 5);
    const maxBw = Math.max(...topList.map(d => (parseFloat(d.rx_mbps || d.rx_rate || 0) + parseFloat(d.tx_mbps || d.tx_rate || 0))), 1);
    const maxScale = maxBw > 1 ? Math.ceil(maxBw * 1.2) : 10;

    const newHTML = topList.map((iface) => {
        const rxValRaw = parseFloat(iface.rx_mbps || iface.rx_rate || 0);
        const txValRaw = parseFloat(iface.tx_mbps || iface.tx_rate || 0);
        const totalBw = rxValRaw + txValRaw;

        let deviceTitle = iface.device_name || iface.ip_address || 'Device';
        const ifName = (iface.interface_name || '').trim();
        const ifAlias = (iface.interface_alias || '').trim();

        let ifLabel = '';
        if (ifName && ifName !== 'undefined') {
            ifLabel = ifName;
            if (ifAlias && ifAlias !== 'undefined' && ifAlias !== ifName) {
                ifLabel += ` (${ifAlias})`;
            }
        } else if (ifAlias && ifAlias !== 'undefined') {
            ifLabel = ifAlias;
        }

        let titleName = ifLabel ? `${deviceTitle} - ${ifLabel}` : deviceTitle;
        titleName = titleName.replace(/</g, '&lt;').replace(/>/g, '&gt;');

        let icon = 'fa-ethernet';
        const mainColor = '#8b5cf6';
        const accentColor = '#a78bfa';
        const ifLower = (ifName + ' ' + titleName).toLowerCase();
        if (ifLower.includes('sfp') || ifLower.includes('te') || ifLower.includes('opt')) {
            icon = 'fa-plug';
        } else if (ifLower.includes('vlan') || ifLower.includes('bridge') || ifLower.includes('bond') || ifLower.includes('trunk') || ifLower.includes('rumdin') || ifLower.includes('pusat')) {
            icon = 'fa-project-diagram';
        }
        const badgeBg = 'rgba(139, 92, 246, 0.12)';

        const formatBwValue = (val) => {
            if (val >= 1000) return `${(val / 1000).toFixed(0)} Gbps`;
            return `${val.toFixed(0)} Mbps`;
        };

        const formatRateDetail = (val) => {
            if (val >= 1000) return `${(val / 1000).toFixed(0)}G`;
            return `${val.toFixed(0)}`;
        };

        const totalBwStr = formatBwValue(totalBw);
        const rxValStr = formatRateDetail(rxValRaw);
        const txValStr = formatRateDetail(txValRaw);

        const subtitleStr = totalBw > 0 
            ? `<i class="fas fa-arrow-down" style="color:#a78bfa;"></i> ${rxValStr} Mbps &nbsp; <i class="fas fa-arrow-up" style="color:#34d399;"></i> ${txValStr} Mbps`
            : 'Interface Idle';

        return `
        <div class="premium-list-box" style="border-left: 3px solid ${mainColor};">
            <div class="premium-box-icon" style="background: rgba(139, 92, 246, 0.15); color: ${accentColor};">
                <i class="fas ${icon}"></i>
            </div>
            <div class="premium-box-details">
                <span class="premium-box-title" title="${titleName}">${titleName}</span>
                <span class="premium-box-subtitle">${subtitleStr}</span>
            </div>
            <div class="premium-box-action" style="color: #c084fc; background: ${badgeBg}; border: 1px solid rgba(139, 92, 246, 0.25);">
                ${totalBwStr}
            </div>
        </div>
        `;
    }).join('');
    safeSetHTML(container, newHTML);
}

// Export for backward compatibility
export { fetchTopInterfaces as updateTopInterfaces };
