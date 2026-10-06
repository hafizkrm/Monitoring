// Device Detail Sidebar Module
import { formatBytes, isOnline, formatUpTime } from '../../shared/utils/helpers.js';
import { subscribeToTopics, unsubscribeFromTopics } from '../../core/services/realtime.service.js';
import { metricsStore } from '../../core/state/store.js';

let activeDeviceId = null;
let activeDeviceTopic = null;
let latestMetrics = [];
let isInitialized = false;

export function initDeviceDetail() {
    if (isInitialized) return;
    isInitialized = true;
    const closeBtn = document.getElementById('close-device-sidebar');
    const overlay = document.getElementById('device-sidebar-overlay');
    if (closeBtn) closeBtn.onclick = closeDeviceSidebar;
    if (overlay) overlay.onclick = closeDeviceSidebar;

    // Listen for real-Time updates via metricsStore
    metricsStore.subscribe('fullMetrics', (metrics) => handleMetricsUpdated({ detail: metrics }));
}

function handleMetricsUpdated(e) {
    latestMetrics = e.detail || [];
    const sidebar = document.getElementById('device-sidebar');
    if (activeDeviceId && sidebar && sidebar.classList.contains('open')) {
        const currentDevice = latestMetrics.find(m => (m.ip || m.ip_address) === activeDeviceId || (m.id || m.device_id) == activeDeviceId || m.name === activeDeviceId);
        if (currentDevice) {
            renderDeviceData(currentDevice);
        }
    }
}

export async function openDeviceSidebar(deviceSummary) {
    const sidebar = document.getElementById('device-sidebar');
    const overlay = document.getElementById('device-sidebar-overlay');
    if (!sidebar) return;
    
    // Set identifier to track updates
    if (typeof deviceSummary === 'string') {
        activeDeviceId = deviceSummary;
    } else {
        activeDeviceId = deviceSummary.ip || deviceSummary.ip_address || deviceSummary.id || deviceSummary.device_id;
    }

    // Unsubscribe from previous if any
    if (activeDeviceTopic) {
        unsubscribeFromTopics([activeDeviceTopic]);
    }
    
    // Subscribe to new device if we have ID
    const devId = deviceSummary.id || deviceSummary.device_id;
    if (devId) {
        activeDeviceTopic = `device:${devId}`;
        subscribeToTopics([activeDeviceTopic]);
    }

    // Show sidebar
    sidebar.classList.add('open');
    if (overlay) overlay.classList.add('active');

    // Ensure event listeners are attached
    initDeviceDetail();

    // Trigger animations by removing and re-adding stagger classes
    const header = sidebar.querySelector('.device-sidebar-header');
    const sections = sidebar.querySelectorAll('.ds-section');
    
    if (header) {
        header.classList.remove('stagger-1');
        void header.offsetWidth; // Trigger reflow
        header.classList.add('stagger-1');
    }
    
    sections.forEach((sec, idx) => {
        sec.classList.remove('stagger-1', 'stagger-2', 'stagger-3');
        void sec.offsetWidth; // Trigger reflow
        sec.classList.add(`stagger-${Math.min(idx + 1, 3)}`);
    });

    const summaryObj = typeof deviceSummary === 'string' 
        ? { ip: deviceSummary, name: deviceSummary } 
        : deviceSummary;

    // Initial render with summary data
    renderDeviceData(summaryObj);

    // Look for full rich metric data across caches
    let fullData = null;
    let fullMetrics = metricsStore.getState().fullMetrics || [];
    if (fullMetrics.length > 0) {
        fullData = fullMetrics.find(m => (m.ip || m.ip_address) === activeDeviceId || (m.id || m.device_id) == activeDeviceId || m.name === activeDeviceId);
    }
    if (!fullData && latestMetrics && latestMetrics.length > 0) {
        fullData = latestMetrics.find(m => (m.ip || m.ip_address) === activeDeviceId || (m.id || m.device_id) == activeDeviceId || m.name === activeDeviceId);
    }
    if (fullData) {
        renderDeviceData({ ...summaryObj, ...fullData });
    }
}

export function closeDeviceSidebar() {
    const sidebar = document.getElementById('device-sidebar');
    const overlay = document.getElementById('device-sidebar-overlay');
    if (sidebar) sidebar.classList.remove('open');
    if (overlay) overlay.classList.remove('active');
    activeDeviceId = null;
    
    if (activeDeviceTopic) {
        unsubscribeFromTopics([activeDeviceTopic]);
        activeDeviceTopic = null;
    }
}

// Expose to window for inline onclick from HTML
window.openDeviceSidebar = openDeviceSidebar;
window.closeDeviceSidebar = closeDeviceSidebar;
window.viewDeviceDetails = openDeviceSidebar;

function renderDeviceData(device) {
    const elName = document.getElementById('ds-name');
    const elIp = document.getElementById('ds-ip');
    const elStatus = document.getElementById('ds-status-badge');
    const elUpTime = document.getElementById('ds-uptime');
    const elIcon = document.getElementById('ds-icon');
    const elVendor = document.getElementById('ds-vendor-badge');

    const elCpuVal = document.getElementById('ds-cpu-val');
    const elCpuBar = document.getElementById('ds-cpu-bar');
    const elRamVal = document.getElementById('ds-ram-val');
    const elRamBar = document.getElementById('ds-ram-bar');
    const elDynamic = document.getElementById('ds-dynamic-section');

    const elLatency = document.getElementById('ds-latency-val');
    const elJitter = document.getElementById('ds-jitter-val');
    const elLoss = document.getElementById('ds-loss-val');

    if (!elName || !elIp) return;

    // 1. Basic Info
    const ipAddr = device.ip || device.ip_address || '0.0.0.0';
    elName.innerText = device.name || 'Unknown Device';
    elIp.innerText = ipAddr;
    
    const isUp = isOnline(device.status);
    const StatusText = (device.status || 'Unknown').toUpperCase();
    if (elStatus) {
        elStatus.innerHTML = `<span class="pulsing-dot"></span>${StatusText}`;
        elStatus.className = `badge ${isUp ? 'online' : (device.status === 'warning' || device.status === 'degraded' ? 'warning' : 'offline')}`;
    }
    
    if (elUpTime) {
        if (isUp) {
            if (device.uptime && device.uptime > 0) {
                elUpTime.innerHTML = `<i class="far fa-clock"></i> ${formatUpTime(device.uptime)}`;
            } else {
                elUpTime.innerHTML = `<i class="fas fa-check-circle" style="color: #4ade80; margin-right: 4px;"></i> Aktif (Ping)`;
            }
        } else {
            elUpTime.innerHTML = `<i class="far fa-clock"></i> Offline`;
        }
    }

    // Ping Statistics
    let latVal = parseFloat(device.latency_ms || device.latency || 0);
    if (isUp && latVal === 0) latVal = "<1";
    
    if (elLatency) elLatency.innerHTML = `${latVal} <span style="font-size: 10px; color: var(--text-muted);">ms</span>`;
    
    let jitVal = parseFloat(device.jitter_ms || device.jitter || 0);
    if (isUp && jitVal === 0) jitVal = "<1";
    if (elJitter) elJitter.innerHTML = `${typeof jitVal === 'number' ? jitVal.toFixed(1) : jitVal} <span style="font-size: 10px; color: var(--text-muted);">ms</span>`;
    
    if (elLoss) {
        const lossVal = parseFloat(device.packet_loss || 0);
        elLoss.innerHTML = `${lossVal.toFixed(0)}<span style="font-size: 10px; color: var(--text-muted);">%</span>`;
        elLoss.style.color = lossVal > 5 ? 'var(--accent-red)' : 'var(--accent-green)';
    }

    // Set Vendor Badge & Icon based on Type / Brand / Name
    const type = (device.device_type || device.type || '').toLowerCase();
    const devNameLower = (device.name || '').toLowerCase();
    const devVendorLower = (device.vendor || '').toLowerCase();

    let iconClass = 'fas fa-server';
    let iconColor = 'var(--accent-blue)';
    let vendorName = 'GENERIC';
    let vendorBg = 'rgba(59, 130, 246, 0.2)';
    let vendorColor = '#60a5fa';

    if (type.includes('palo alto') || type.includes('paloalto') || devNameLower.includes('palo alto') || devVendorLower.includes('palo alto')) {
        vendorName = 'PALO ALTO';
        vendorBg = 'rgba(239, 68, 68, 0.2)'; vendorColor = '#f87171';
        iconClass = 'fas fa-shield-alt'; iconColor = '#f87171';
    } else if (type.includes('ruijie') || devNameLower.includes('ruijie') || devVendorLower.includes('ruijie')) {
        vendorName = 'RUIJIE';
        vendorBg = 'rgba(236, 72, 153, 0.2)'; vendorColor = '#f472b6';
        iconClass = 'fas fa-wifi'; iconColor = '#f472b6';
    } else if (type.includes('tplink') || type.includes('tp-link') || devNameLower.includes('tp-link') || devVendorLower.includes('tp-link')) {
        vendorName = 'TP-LINK';
        vendorBg = 'rgba(16, 185, 129, 0.2)'; vendorColor = '#34d399';
        iconClass = 'fas fa-wifi'; iconColor = '#34d399';
    } else if (type.includes('ubiquiti') || type.includes('ubnt') || type.includes('radio') || devNameLower.includes('ubiquiti')) {
        vendorName = 'UBIQUITI';
        vendorBg = 'rgba(245, 158, 11, 0.2)'; vendorColor = '#fbbf24';
        iconClass = 'fas fa-broadcast-tower'; iconColor = '#fbbf24';
    } else if (type.includes('mikrotik') || type.includes('router') || devNameLower.includes('mikrotik') || devVendorLower.includes('mikrotik')) {
        vendorName = 'MIKROTIK';
        vendorBg = 'rgba(59, 130, 246, 0.2)'; vendorColor = '#60a5fa';
        iconClass = 'fas fa-route'; iconColor = '#60a5fa';
    } else if (type.includes('cisco') || devNameLower.includes('cisco') || devVendorLower.includes('cisco')) {
        vendorName = 'CISCO';
        vendorBg = 'rgba(6, 182, 212, 0.2)'; vendorColor = '#22d3ee';
        iconClass = 'fas fa-network-wired'; iconColor = '#22d3ee';
    } else if (type.includes('switch')) {
        vendorName = 'SWITCH';
        vendorBg = 'rgba(139, 92, 246, 0.2)'; vendorColor = '#c084fc';
        iconClass = 'fas fa-network-wired'; iconColor = '#c084fc';
    } else if (type.includes('ap') || type.includes('access')) {
        vendorName = 'ACCESS POINT';
        vendorBg = 'rgba(6, 182, 212, 0.2)'; vendorColor = '#22d3ee';
        iconClass = 'fas fa-wifi'; iconColor = '#22d3ee';
    } else if (type.includes('firewall')) {
        vendorName = 'FIREWALL';
        vendorBg = 'rgba(239, 68, 68, 0.2)'; vendorColor = '#f87171';
        iconClass = 'fas fa-shield-alt'; iconColor = '#f87171';
    } else if (type.includes('server')) {
        vendorName = 'SERVER';
        vendorBg = 'rgba(34, 197, 94, 0.2)'; vendorColor = '#4ade80';
        iconClass = 'fas fa-server'; iconColor = '#4ade80';
    }

    if (device.vendor && device.vendor !== 'Generic' && device.vendor !== 'unknown') {
        vendorName = device.vendor.toUpperCase();
    }

    if (elVendor) {
        elVendor.innerText = vendorName;
        elVendor.style.background = vendorBg;
        elVendor.style.color = vendorColor;
        elVendor.style.borderColor = `${vendorColor}40`;
    }

    if (elIcon) {
        elIcon.innerHTML = `<i class="${iconClass}"></i>`;
        elIcon.style.background = iconColor;
        elIcon.style.boxShadow = `0 4px 20px ${iconColor}`;
    }

    // 2. CPU and Memory Resources
    const cpuRaw = parseFloat(device.cpu_usage || device.cpu || 0);
    const cpu = isNaN(cpuRaw) ? 0 : Number(cpuRaw.toFixed(1));
    const ramRaw = parseFloat(device.memory_usage || device.ram || 0);
    const ram = isNaN(ramRaw) ? 0 : Number(ramRaw.toFixed(1));

    if (elCpuVal) elCpuVal.innerText = `${cpu}%`;
    if (elCpuBar) {
        elCpuBar.style.width = `${cpu}%`;
        elCpuBar.style.background = cpu > 80 ? 'var(--accent-red)' : 'var(--accent-blue)';
    }

    if (elRamVal) elRamVal.innerText = `${ram}%`;
    if (elRamBar) {
        elRamBar.style.width = `${ram}%`;
        elRamBar.style.background = ram > 80 ? 'var(--accent-red)' : 'var(--accent-green)';
    }

    // 3. Dynamic Section based on Device Type
    renderDynamicSection(elDynamic, type, device);

    // 4. Fetch and render interface ports
    const ifaceListContainer = document.getElementById('ds-interfaces-list');
    if (ifaceListContainer) {
        ifaceListContainer.innerHTML = '<div style="text-align: center; padding: 12px; opacity: 0.6;"><i class="fas fa-spinner fa-spin"></i> Memuat port interface...</div>';
    }
    fetchDeviceInterfaces(ipAddr);
}

function renderDynamicSection(elDynamic, type, device) {
    if (!elDynamic) return;
    let html = '';
    const devNameLower = (device.name || '').toLowerCase();

    if (type.includes('firewall') || type.includes('palo alto') || devNameLower.includes('palo alto')) {
        const isUp = isOnline(device.status);
        html = `
            <h4 class="ds-section-title"><i class="fas fa-shield-alt" style="margin-right: 6px; color: #f87171;"></i> Keamanan & Inspeksi Firewall</h4>
            <div class="ds-metric-card" style="margin-bottom: 8px;">
                <div class="ds-metric-icon" style="color: #f87171;"><i class="fas fa-lock"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted);">Mesin Inspeksi Palo Alto</span>
                    <div style="font-size: 14px; font-weight: 700; color: ${isUp ? '#4ade80' : '#f87171'};">
                        ${isUp ? '<i class="fas fa-check-circle"></i> Proteksi Aktif' : '<i class="fas fa-exclamation-triangle"></i> Mesin Offline'}
                    </div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card" style="margin-bottom: 0;">
                    <div class="ds-metric-icon" style="color: var(--accent-green)"><i class="fas fa-download"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Rx (Unduh)</span>
                        <div style="font-size: 14px; font-weight: 700; color: var(--text-main);">${formatBytes(device.rx_rate || 0, 'Mbps')}</div>
                    </div>
                </div>
                <div class="ds-metric-card" style="margin-bottom: 0;">
                    <div class="ds-metric-icon" style="color: var(--accent-blue)"><i class="fas fa-upload"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Tx (Unggah)</span>
                        <div style="font-size: 14px; font-weight: 700; color: var(--text-main);">${formatBytes(device.tx_rate || 0, 'Mbps')}</div>
                    </div>
                </div>
            </div>
        `;
    } else if (type.includes('switch') || type.includes('cisco') || devNameLower.includes('cisco')) {
        const tx = device.tx_rate || 0;
        const rx = device.rx_rate || 0;
        html = `
            <h4 class="ds-section-title"><i class="fas fa-network-wired" style="margin-right: 6px; color: #22d3ee;"></i> Ringkasan Port & Throughput Switch</h4>
            <div class="ds-metric-card" style="margin-bottom: 8px;">
                <div class="ds-metric-icon" style="color: #22d3ee;"><i class="fas fa-server"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted);">Switch Terkelola Cisco</span>
                    <div id="ds-switch-ports-val" style="font-size: 14px; font-weight: 700; color: #60a5fa;">Sinkronisasi Port Aktif...</div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card" style="margin-bottom: 0;">
                    <div class="ds-metric-icon" style="color: var(--accent-green)"><i class="fas fa-download"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Rx (Unduh)</span>
                        <div id="ds-main-rx-val" style="font-size: 14px; font-weight: 700; color: var(--text-main);">${formatBytes(rx, 'Mbps')}</div>
                    </div>
                </div>
                <div class="ds-metric-card" style="margin-bottom: 0;">
                    <div class="ds-metric-icon" style="color: var(--accent-blue)"><i class="fas fa-upload"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Tx (Unggah)</span>
                        <div id="ds-main-tx-val" style="font-size: 14px; font-weight: 700; color: var(--text-main);">${formatBytes(tx, 'Mbps')}</div>
                    </div>
                </div>
            </div>
        `;
    } else if (type.includes('router') || type.includes('mikrotik')) {
        const tx = device.tx_rate || 0;
        const rx = device.rx_rate || 0;
        html = `
            <h4 class="ds-section-title"><i class="fas fa-route" style="margin-right: 6px; color: #60a5fa;"></i> Lalu Lintas Interface Router (Utama)</h4>
            <div class="ds-metric-card">
                <div class="ds-metric-icon" style="color: var(--accent-green)"><i class="fas fa-download"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted);">Unduh (Rx)</span>
                    <div id="ds-main-rx-val" style="font-size: 16px; font-weight: 700; color: var(--text-main);">${formatBytes(rx, 'Mbps')}</div>
                </div>
            </div>
            <div class="ds-metric-card">
                <div class="ds-metric-icon" style="color: var(--accent-blue)"><i class="fas fa-upload"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted);">Unggah (Tx)</span>
                    <div id="ds-main-tx-val" style="font-size: 16px; font-weight: 700; color: var(--text-main);">${formatBytes(tx, 'Mbps')}</div>
                </div>
            </div>
        `;
    } else if (type.includes('radio') || type.includes('ubiquiti')) {
        const signalVal = device.signal_strength !== undefined ? device.signal_strength : device.signal;
        const signal = (signalVal !== undefined && signalVal !== null && signalVal !== 0) ? signalVal : null;
        const ccqVal = device.ccq !== undefined ? device.ccq : null;
        const ccq = (ccqVal !== undefined && ccqVal !== null && ccqVal !== 0) ? ccqVal : null;
        
        let sigColor = 'var(--accent-green)';
        if (!signal) sigColor = 'var(--accent-blue)';
        else if (signal < -80) sigColor = 'var(--accent-red)';
        else if (signal < -70) sigColor = 'var(--accent-orange)';

        const sigDisplay = signal ? `${signal} dBm` : '<span style="font-size: 13px; color: #60a5fa;">AirMAX Link</span>';
        const ccqDisplay = ccq ? `${ccq}%` : '<span style="font-size: 13px; color: #4ade80;">Optimal</span>';

        html = `
            <h4 class="ds-section-title"><i class="fas fa-broadcast-tower" style="margin-right: 6px; color: #fbbf24;"></i> Kualitas Link Radio & Lalu Lintas</h4>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 10px;">
                <div class="ds-metric-card" style="margin-bottom: 0;">
                    <div class="ds-metric-icon" style="color: ${sigColor}"><i class="fas fa-signal"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Sinyal</span>
                        <div style="font-size: 15px; font-weight: 700; color: var(--text-main);">${sigDisplay}</div>
                    </div>
                </div>
                <div class="ds-metric-card" style="margin-bottom: 0;">
                    <div class="ds-metric-icon" style="color: var(--accent-blue)"><i class="fas fa-link"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Kualitas / CCQ</span>
                        <div style="font-size: 15px; font-weight: 700; color: var(--text-main);">${ccqDisplay}</div>
                    </div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card">
                    <div class="ds-metric-icon" style="color: var(--accent-green)"><i class="fas fa-download"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Rx (Unduh)</span>
                        <div style="font-size: 14px; font-weight: 700; color: var(--text-main);">${formatBytes(device.rx_rate || 0, 'Mbps')}</div>
                    </div>
                </div>
                <div class="ds-metric-card">
                    <div class="ds-metric-icon" style="color: var(--accent-blue)"><i class="fas fa-upload"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Tx (Unggah)</span>
                        <div style="font-size: 14px; font-weight: 700; color: var(--text-main);">${formatBytes(device.tx_rate || 0, 'Mbps')}</div>
                    </div>
                </div>
            </div>
        `;
    } else if (type.includes('ap') || type.includes('access point') || type.includes('ruijie') || type.includes('tplink')) {
        const clients = device.connected_clients || device.clients || 0;
        
        html = `
            <h4 class="ds-section-title"><i class="fas fa-wifi" style="margin-right: 6px; color: #34d399;"></i> Statistik AP Nirkabel</h4>
            <div class="ds-metric-card">
                <div class="ds-metric-icon" style="color: var(--accent-purple)"><i class="fas fa-user"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted);">Klien Terhubung</span>
                    <div style="font-size: 16px; font-weight: 700; color: var(--text-main);">${clients} Pengguna Aktif</div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card" style="margin-bottom: 0;">
                    <div class="ds-metric-icon" style="color: var(--accent-green)"><i class="fas fa-download"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Rx (Unduh)</span>
                        <div style="font-size: 14px; font-weight: 700; color: var(--text-main);">${formatBytes(device.rx_rate || 0, 'Mbps')}</div>
                    </div>
                </div>
                <div class="ds-metric-card" style="margin-bottom: 0;">
                    <div class="ds-metric-icon" style="color: var(--accent-blue)"><i class="fas fa-upload"></i></div>
                    <div class="ds-metric-details">
                        <span style="font-size: 11px; color: var(--text-muted);">Tx (Unggah)</span>
                        <div style="font-size: 14px; font-weight: 700; color: var(--text-main);">${formatBytes(device.tx_rate || 0, 'Mbps')}</div>
                    </div>
                </div>
            </div>
        `;
    } else {
        const isUp = isOnline(device.status);
        html = `
            <h4 class="ds-section-title"><i class="fas fa-info-circle" style="margin-right: 6px; color: var(--accent-blue);"></i> Kesehatan & Status Perangkat</h4>
            <div class="ds-metric-card">
                <div class="ds-metric-icon" style="color: ${isUp ? 'var(--accent-green)' : 'var(--accent-red)'}"><i class="fas fa-heartbeat"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted);">Status Operasional Sistem</span>
                    <div style="font-size: 14px; font-weight: 700; color: ${isUp ? '#4ade80' : '#f87171'};">
                        ${isUp ? 'Operasional & Terpantau' : 'Perangkat Offline'}
                    </div>
                </div>
            </div>
        `;
    }

    elDynamic.innerHTML = html;
}

// Fetch and render physical & wireless interfaces for device
async function fetchDeviceInterfaces(ip) {
    const listContainer = document.getElementById('ds-interfaces-list');
    if (!listContainer || !ip) return;

    try {
        const controller = new AbortController();
        const TimeoutId = setTimeout(() => controller.abort(), 5000);
        
        const res = await fetch(`/api/interfaces?ip=${encodeURIComponent(ip)}`, { signal: controller.signal });
        clearTimeout(TimeoutId);
        
        if (!res.ok) {
            listContainer.innerHTML = '<div style="color: var(--text-muted); padding: 8px; font-size: 12px; opacity: 0.8;"><i class="fas fa-info-circle" style="margin-right: 4px;"></i> Monitoring ICMP aktif.</div>';
            return;
        }
        let ifaces = await res.json();
        if (!Array.isArray(ifaces) || ifaces.length === 0) {
            listContainer.innerHTML = '<div style="color: var(--text-muted); padding: 8px; font-size: 12px; opacity: 0.8;"><i class="fas fa-shield-alt" style="margin-right: 4px; color: var(--accent-red);"></i> Interface SNMP tidak terdeteksi (ICMP Monitor Aktif).</div>';
            return;
        }

        // Filter out dummy & virtual interfaces (apcli, apclix, bluetooth, loopback, internal bridges, etc.)
        const isRealInterface = (ifaceObj) => {
            const name = (ifaceObj.name || ifaceObj.interface_name || '').toLowerCase().trim();
            if (!name) return false;
            if (name === 'lo' || name.startsWith('lo0') || name.startsWith('loopback')) return false;
            if (name.startsWith('apcli') || name.startsWith('apclix') || name.startsWith('ra') || name.startsWith('wds')) return false;
            if (name.startsWith('bt') || name.startsWith('bluetooth')) return false;
            if (name.startsWith('br') || name.startsWith('bridge')) return false;
            if (name.startsWith('vlan')) return false;
            if (name.startsWith('tun') || name.startsWith('tap') || name.startsWith('docker') || name.startsWith('veth') || name.startsWith('bond') || name.startsWith('team')) return false;
            return true;
        };

        const displayIfaces = ifaces.filter(isRealInterface);
        const activeIfaces = displayIfaces.length > 0 ? displayIfaces : ifaces;

        // Smart Sort:
        // 1. Interfaces with active traffic (rx + tx > 0) come FIRST
        // 2. UP Status before DOWN
        activeIfaces.sort((a, b) => {
            const nameA = (a.name || a.interface_name || '').toLowerCase();
            const nameB = (b.name || b.interface_name || '').toLowerCase();
            const isUpA = (a.status || a.interface_status || '').toLowerCase() === 'up';
            const isUpB = (b.status || b.interface_status || '').toLowerCase() === 'up';
            const rxA = a.rx_rate || a.rx_mbps || 0;
            const txA = a.tx_rate || a.tx_mbps || 0;
            const rxB = b.rx_rate || b.rx_mbps || 0;
            const txB = b.tx_rate || b.tx_mbps || 0;
            const trafficA = rxA + txA;
            const trafficB = rxB + txB;

            if (trafficA > 0 && trafficB === 0) return -1;
            if (trafficB > 0 && trafficA === 0) return 1;
            if (trafficA !== trafficB) return trafficB - trafficA;

            if (isUpA && !isUpB) return -1;
            if (!isUpA && isUpB) return 1;

            return nameA.localeCompare(nameB, undefined, { numeric: true });
        });

        let html = '';
        let totalPhysRx = 0;
        let totalPhysTx = 0;
        let upCount = 0;

        activeIfaces.forEach(iface => {
            const isPortUp = (iface.status || iface.interface_status || '').toLowerCase() === 'up';
            if (isPortUp) upCount++;
        });

        const elPortSummary = document.getElementById('ds-switch-ports-val');
        if (elPortSummary) {
            elPortSummary.innerText = `${upCount} / ${activeIfaces.length} Port Aktif`;
        }

        activeIfaces.slice(0, 8).forEach(iface => {
            const isPortUp = (iface.status || iface.interface_status || '').toLowerCase() === 'up';
            const StatusColor = isPortUp ? '#22c55e' : '#ef4444';
            const name = iface.name || iface.interface_name || 'Port';
            const rxMbps = iface.rx_rate || iface.rx_mbps || 0;
            const txMbps = iface.tx_rate || iface.tx_mbps || 0;

            if (isPortUp) {
                totalPhysRx += rxMbps;
                totalPhysTx += txMbps;
            }

            html += `
                <div style="display: flex; justify-content: space-between; align-items: center; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 6px 10px; border-radius: 6px; margin-bottom: 4px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <span style="width: 7px; height: 7px; border-radius: 50%; background: ${StatusColor}; display: inline-block; box-shadow: ${isPortUp ? '0 0 6px #22c55e' : 'none'};"></span>
                        <strong style="color: var(--text-main); font-size: 11px;">${name}</strong>
                    </div>
                    <div style="font-size: 10px; color: var(--text-muted); font-family: monospace;">
                        <span style="color: #4ade80;"><i class="fas fa-arrow-down" style="font-size:9px;"></i> ${formatBytes(rxMbps, 'Mbps')}</span>
                        <span style="color: #60a5fa; margin-left: 6px;"><i class="fas fa-arrow-up" style="font-size:9px;"></i> ${formatBytes(txMbps, 'Mbps')}</span>
                    </div>
                </div>
            `;
        });

        // If physical active ports have throughput, update Main interface card
        if (totalPhysRx > 0 || totalPhysTx > 0) {
            const elRx = document.getElementById('ds-main-rx-val');
            const elTx = document.getElementById('ds-main-tx-val');
            if (elRx) elRx.innerText = formatBytes(totalPhysRx, 'Mbps');
            if (elTx) elTx.innerText = formatBytes(totalPhysTx, 'Mbps');
        }

        if (activeIfaces.length > 8) {
            html += `<div style="text-align: center; font-size: 10px; color: var(--text-muted); padding-top: 6px;">+ ${activeIfaces.length - 8} port lainnya...</div>`;
        }

        listContainer.innerHTML = html;
    } catch (e) {
        listContainer.innerHTML = '<div style="color: var(--text-muted); padding: 8px; font-size: 12px; opacity: 0.8;"><i class="fas fa-info-circle" style="margin-right: 4px;"></i> Monitoring ICMP aktif.</div>';
    }
}
