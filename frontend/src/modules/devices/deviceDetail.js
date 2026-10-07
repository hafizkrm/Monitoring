// Device Detail Sidebar Module - High Performance & Enterprise UI/UX
import { formatBytes, isOnline, formatUpTime } from '../../shared/utils/helpers.js';
import { subscribeToTopics, unsubscribeFromTopics } from '../../core/services/realtime.service.js';
import { metricsStore } from '../../core/state/store.js';

let activeDeviceId = null;
let activeDeviceTopic = null;
let activeDeviceObj = null;
let isInitialized = false;

// Short-term memory cache for interface lists (TTL: 20 seconds)
const interfaceCache = new Map();
const CACHE_TTL_MS = 20000;

export function initDeviceDetail() {
    if (isInitialized) return;
    isInitialized = true;
    
    const closeBtn = document.getElementById('close-device-sidebar');
    const overlay = document.getElementById('device-sidebar-overlay');
    if (closeBtn) closeBtn.onclick = closeDeviceSidebar;
    if (overlay) overlay.onclick = closeDeviceSidebar;

    // Quick Action Diagnostic Event Handlers
    const btnPing = document.getElementById('ds-btn-ping');
    const btnTrace = document.getElementById('ds-btn-trace');
    const btnEdit = document.getElementById('ds-btn-edit');
    const btnDelete = document.getElementById('ds-btn-delete');

    if (btnPing) {
        btnPing.onclick = () => {
            const ip = document.getElementById('ds-ip')?.innerText;
            if (ip && window.runPing) window.runPing(ip);
        };
    }
    if (btnTrace) {
        btnTrace.onclick = () => {
            const ip = document.getElementById('ds-ip')?.innerText;
            if (ip && window.runTrace) window.runTrace(ip);
        };
    }
    if (btnEdit) {
        btnEdit.onclick = () => {
            if (!activeDeviceObj) return;
            const ip = activeDeviceObj.ip || activeDeviceObj.ip_address || document.getElementById('ds-ip')?.innerText;
            const name = activeDeviceObj.name || document.getElementById('ds-name')?.innerText;
            const type = activeDeviceObj.device_type || activeDeviceObj.type || 'Router';
            const vendor = activeDeviceObj.vendor || '';
            if (window.editDevice) window.editDevice(ip, name, type, vendor);
        };
    }
    if (btnDelete) {
        btnDelete.onclick = () => {
            if (!activeDeviceObj) return;
            const ip = activeDeviceObj.ip || activeDeviceObj.ip_address || document.getElementById('ds-ip')?.innerText;
            const name = activeDeviceObj.name || document.getElementById('ds-name')?.innerText;
            if (window.deleteDevice) window.deleteDevice(ip, name);
        };
    }

    // Subscribe to real-time metric updates via metricsStore
    metricsStore.subscribe('fullMetrics', (metrics) => handleMetricsUpdated(metrics));
}

function handleMetricsUpdated(metrics) {
    const fullList = Array.isArray(metrics) ? metrics : (metrics?.detail || []);
    const sidebar = document.getElementById('device-sidebar');
    if (activeDeviceId && sidebar && sidebar.classList.contains('open')) {
        const currentDevice = fullList.find(m => 
            (m.ip || m.ip_address) === activeDeviceId || 
            (m.id || m.device_id) == activeDeviceId || 
            m.name === activeDeviceId
        );
        if (currentDevice) {
            activeDeviceObj = { ...activeDeviceObj, ...currentDevice };
            renderDeviceData(activeDeviceObj);
        }
    }
}

// Single Source of Truth for Vendor Classification & Styling
function getVendorInfo(device) {
    const type = (device.device_type || device.type || '').toLowerCase();
    const devNameLower = (device.name || '').toLowerCase();
    const devVendorLower = (device.vendor || '').toLowerCase();

    let vendorName = 'GENERIC';
    let vendorBg = 'rgba(59, 130, 246, 0.2)';
    let vendorColor = '#60a5fa';
    let iconClass = 'fas fa-server';
    let iconColor = '#3b82f6';

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
    } else if (type.includes('ap') || type.includes('access point')) {
        vendorName = 'ACCESS POINT';
        vendorBg = 'rgba(6, 182, 212, 0.2)'; vendorColor = '#22d3ee';
        iconClass = 'fas fa-wifi'; iconColor = '#22d3ee';
    } else if (type.includes('firewall')) {
        vendorName = 'FIREWALL';
        vendorBg = 'rgba(239, 68, 68, 0.2)'; vendorColor = '#f87171';
        iconClass = 'fas fa-shield-alt'; iconColor = '#f87171';
    }

    if (device.vendor && device.vendor.trim() && !['generic', 'unknown'].includes(device.vendor.toLowerCase())) {
        vendorName = device.vendor.toUpperCase();
    }

    return { vendorName, vendorBg, vendorColor, iconClass, iconColor, typeKey: type };
}

export async function openDeviceSidebar(deviceSummary) {
    const sidebar = document.getElementById('device-sidebar');
    const overlay = document.getElementById('device-sidebar-overlay');
    if (!sidebar) return;
    
    // Set identifier
    if (typeof deviceSummary === 'string') {
        activeDeviceId = deviceSummary;
        activeDeviceObj = { ip: deviceSummary, name: deviceSummary };
    } else {
        activeDeviceId = deviceSummary.ip || deviceSummary.ip_address || deviceSummary.id || deviceSummary.device_id;
        activeDeviceObj = { ...deviceSummary };
    }

    // Unsubscribe previous WebSocket topic
    if (activeDeviceTopic) {
        unsubscribeFromTopics([activeDeviceTopic]);
    }
    
    // Subscribe to new device topic if available
    const devId = activeDeviceObj.id || activeDeviceObj.device_id;
    if (devId) {
        activeDeviceTopic = `device:${devId}`;
        subscribeToTopics([activeDeviceTopic]);
    }

    // Show sidebar & overlay
    sidebar.classList.add('open');
    if (overlay) overlay.classList.add('active');

    // Ensure event listeners are attached
    initDeviceDetail();

    // Trigger staggered entrance animations
    const header = sidebar.querySelector('.device-sidebar-header');
    const sections = sidebar.querySelectorAll('.ds-section');
    
    if (header) {
        header.classList.remove('stagger-1');
        void header.offsetWidth;
        header.classList.add('stagger-1');
    }
    
    sections.forEach((sec, idx) => {
        sec.classList.remove('stagger-1', 'stagger-2', 'stagger-3');
        void sec.offsetWidth;
        sec.classList.add(`stagger-${Math.min(idx + 1, 3)}`);
    });

    // Initial render with current device object
    renderDeviceData(activeDeviceObj);

    // Look up enriched cached metric data
    const fullMetrics = metricsStore.getState().fullMetrics || [];
    const fullData = fullMetrics.find(m => 
        (m.ip || m.ip_address) === activeDeviceId || 
        (m.id || m.device_id) == activeDeviceId || 
        m.name === activeDeviceId
    );
    if (fullData) {
        activeDeviceObj = { ...activeDeviceObj, ...fullData };
        renderDeviceData(activeDeviceObj);
    }
}

export function closeDeviceSidebar() {
    const sidebar = document.getElementById('device-sidebar');
    const overlay = document.getElementById('device-sidebar-overlay');
    if (sidebar) sidebar.classList.remove('open');
    if (overlay) overlay.classList.remove('active');
    
    activeDeviceId = null;
    activeDeviceObj = null;
    
    if (activeDeviceTopic) {
        unsubscribeFromTopics([activeDeviceTopic]);
        activeDeviceTopic = null;
    }
}

// Expose functions globally for inline DOM invocations
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
    const elSlaBadge = document.getElementById('ds-sla-badge');

    if (!elName || !elIp) return;

    // 1. Basic Header Information
    const ipAddr = device.ip || device.ip_address || '0.0.0.0';
    elName.innerText = device.name || 'Unknown Device';
    elIp.innerText = ipAddr;
    
    const isUp = isOnline(device.status);
    const statusText = isUp ? 'ONLINE' : (device.status === 'warning' || device.status === 'degraded' ? 'WARNING' : 'OFFLINE');
    if (elStatus) {
        elStatus.innerHTML = `<span class="pulsing-dot"></span>${statusText}`;
        elStatus.className = `badge ${isUp ? 'online' : (device.status === 'warning' || device.status === 'degraded' ? 'warning' : 'offline')}`;
    }

    if (elUpTime) {
        if (isUp) {
            if (device.uptime && device.uptime > 0) {
                elUpTime.innerHTML = `<i class="far fa-clock"></i> ${formatUpTime(device.uptime)}`;
            } else {
                elUpTime.innerHTML = `<i class="fas fa-check-circle" style="color: #4ade80; margin-right: 4px;"></i> Active (Ping)`;
            }
        } else {
            elUpTime.innerHTML = `<i class="far fa-clock"></i> Offline`;
        }
    }

    // 2. Ping & SLA Statistics
    let latVal = parseFloat(device.latency_ms || device.latency || 0);
    if (isUp && latVal === 0) latVal = "<1";
    if (elLatency) elLatency.innerHTML = `${latVal} <span style="font-size: 10px; color: var(--text-muted);">ms</span>`;
    
    let jitVal = parseFloat(device.jitter_ms || device.jitter || 0);
    if (isUp && jitVal === 0) jitVal = "<1";
    if (elJitter) elJitter.innerHTML = `${typeof jitVal === 'number' ? jitVal.toFixed(1) : jitVal} <span style="font-size: 10px; color: var(--text-muted);">ms</span>`;
    
    const lossVal = parseFloat(device.packet_loss || 0);
    if (elLoss) {
        elLoss.innerHTML = `${lossVal.toFixed(0)}<span style="font-size: 10px; color: var(--text-muted);">%</span>`;
        elLoss.style.color = lossVal > 5 ? '#f87171' : '#4ade80';
    }

    // SLA Badge Calculator
    if (elSlaBadge) {
        if (!isUp) {
            elSlaBadge.innerText = 'SLA Down';
            elSlaBadge.style.background = 'rgba(239, 68, 68, 0.2)';
            elSlaBadge.style.color = '#f87171';
            elSlaBadge.style.borderColor = 'rgba(239, 68, 68, 0.3)';
        } else if (lossVal > 5 || (typeof latVal === 'number' && latVal > 100)) {
            elSlaBadge.innerText = 'SLA Degraded';
            elSlaBadge.style.background = 'rgba(245, 158, 11, 0.2)';
            elSlaBadge.style.color = '#fbbf24';
            elSlaBadge.style.borderColor = 'rgba(245, 158, 11, 0.3)';
        } else {
            elSlaBadge.innerText = 'SLA Excellent';
            elSlaBadge.style.background = 'rgba(34, 197, 94, 0.15)';
            elSlaBadge.style.color = '#4ade80';
            elSlaBadge.style.borderColor = 'rgba(34, 197, 94, 0.3)';
        }
    }

    // 3. Single Source Vendor Info & Styling
    const vendorInfo = getVendorInfo(device);
    if (elVendor) {
        elVendor.innerText = vendorInfo.vendorName;
        elVendor.style.background = vendorInfo.vendorBg;
        elVendor.style.color = vendorInfo.vendorColor;
        elVendor.style.borderColor = `${vendorInfo.vendorColor}40`;
    }

    if (elIcon) {
        elIcon.innerHTML = `<i class="${vendorInfo.iconClass}"></i>`;
        elIcon.style.background = vendorInfo.vendorColor;
        elIcon.style.boxShadow = `0 4px 20px ${vendorInfo.vendorColor}40`;
    }

    // 4. Hardware Resources (CPU & RAM) with Dynamic Threshold Gradients
    const cpuRaw = parseFloat(device.cpu_usage || device.cpu || 0);
    const cpu = isNaN(cpuRaw) ? 0 : Number(cpuRaw.toFixed(1));
    const ramRaw = parseFloat(device.memory_usage || device.ram || 0);
    const ram = isNaN(ramRaw) ? 0 : Number(ramRaw.toFixed(1));

    if (elCpuVal) elCpuVal.innerText = `${cpu}%`;
    if (elCpuBar) {
        elCpuBar.style.width = `${cpu}%`;
        elCpuBar.style.background = cpu > 85 ? '#ef4444' : (cpu > 60 ? '#f59e0b' : '#3b82f6');
    }

    if (elRamVal) elRamVal.innerText = `${ram}%`;
    if (elRamBar) {
        elRamBar.style.width = `${ram}%`;
        elRamBar.style.background = ram > 85 ? '#ef4444' : (ram > 60 ? '#f59e0b' : '#10b981');
    }

    // 5. Render Vendor Dynamic Section
    renderDynamicSection(elDynamic, vendorInfo.typeKey, device);

    // 6. Fetch Physical Interface Ports
    const ifaceListContainer = document.getElementById('ds-interfaces-list');
    if (ifaceListContainer) {
        ifaceListContainer.innerHTML = '<div style="text-align: center; padding: 12px; opacity: 0.6;"><i class="fas fa-spinner fa-spin"></i> Loading interfaces...</div>';
    }
    fetchDeviceInterfaces(ipAddr);
}

function renderDynamicSection(elDynamic, type, device) {
    if (!elDynamic) return;
    let html = '';
    const devNameLower = (device.name || '').toLowerCase();
    const isUp = isOnline(device.status);

    if (type.includes('firewall') || type.includes('palo alto') || devNameLower.includes('palo alto')) {
        html = `
            <h4 class="ds-section-title"><i class="fas fa-shield-alt" style="margin-right: 6px; color: #f87171;"></i> Security & Firewall Inspection</h4>
            <div class="ds-metric-card" style="margin-bottom: 8px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px; display: flex; align-items: center; gap: 12px;">
                <div class="ds-metric-icon" style="color: #f87171; font-size: 20px;"><i class="fas fa-lock"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted); display: block;">Inspection Engine</span>
                    <div style="font-size: 14px; font-weight: 700; color: ${isUp ? '#4ade80' : '#f87171'};">
                        ${isUp ? '<i class="fas fa-check-circle"></i> Protection Active' : '<i class="fas fa-exclamation-triangle"></i> Engine Offline'}
                    </div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas fa-download" style="color: #4ade80;"></i>
                        <span style="font-size: 11px; color: var(--text-muted);">Rx (Download)</span>
                    </div>
                    <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(device.rx_rate || 0, 'Mbps')}</div>
                </div>
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas fa-upload" style="color: #60a5fa;"></i>
                        <span style="font-size: 11px; color: var(--text-muted);">Tx (Upload)</span>
                    </div>
                    <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(device.tx_rate || 0, 'Mbps')}</div>
                </div>
            </div>
        `;
    } else if (type.includes('switch') || type.includes('cisco') || devNameLower.includes('cisco')) {
        const tx = device.tx_rate || 0;
        const rx = device.rx_rate || 0;
        html = `
            <h4 class="ds-section-title"><i class="fas fa-network-wired" style="margin-right: 6px; color: #22d3ee;"></i> Switch Port & Throughput Summary</h4>
            <div class="ds-metric-card" style="margin-bottom: 8px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px; display: flex; align-items: center; gap: 12px;">
                <div class="ds-metric-icon" style="color: #22d3ee; font-size: 20px;"><i class="fas fa-server"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted); display: block;">Managed Network Switch</span>
                    <div id="ds-switch-ports-val" style="font-size: 14px; font-weight: 700; color: #60a5fa;">Syncing Port Status...</div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas fa-download" style="color: #4ade80;"></i>
                        <span style="font-size: 11px; color: var(--text-muted);">Rx (Download)</span>
                    </div>
                    <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(rx, 'Mbps')}</div>
                </div>
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas fa-upload" style="color: #60a5fa;"></i>
                        <span style="font-size: 11px; color: var(--text-muted);">Tx (Upload)</span>
                    </div>
                    <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(tx, 'Mbps')}</div>
                </div>
            </div>
        `;
    } else if (type.includes('router') || type.includes('mikrotik')) {
        const tx = device.tx_rate || 0;
        const rx = device.rx_rate || 0;
        html = `
            <h4 class="ds-section-title"><i class="fas fa-route" style="margin-right: 6px; color: #60a5fa;"></i> Main Router Interface Traffic</h4>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas fa-download" style="color: #4ade80;"></i>
                        <span style="font-size: 11px; color: var(--text-muted);">Download (Rx)</span>
                    </div>
                    <div style="font-size: 16px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(rx, 'Mbps')}</div>
                </div>
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas fa-upload" style="color: #60a5fa;"></i>
                        <span style="font-size: 11px; color: var(--text-muted);">Upload (Tx)</span>
                    </div>
                    <div style="font-size: 16px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(tx, 'Mbps')}</div>
                </div>
            </div>
        `;
    } else if (type.includes('radio') || type.includes('ubiquiti')) {
        const signalVal = device.signal_strength !== undefined ? device.signal_strength : device.signal;
        const signal = (signalVal !== undefined && signalVal !== null && signalVal !== 0) ? signalVal : null;
        const ccqVal = device.ccq !== undefined ? device.ccq : null;
        const ccq = (ccqVal !== undefined && ccqVal !== null && ccqVal !== 0) ? ccqVal : null;
        
        let sigColor = '#4ade80';
        if (!signal) sigColor = '#60a5fa';
        else if (signal < -80) sigColor = '#f87171';
        else if (signal < -70) sigColor = '#fbbf24';

        const sigDisplay = signal ? `${signal} dBm` : '<span style="font-size: 13px; color: #60a5fa;">AirMAX Link</span>';
        const ccqDisplay = ccq ? `${ccq}%` : '<span style="font-size: 13px; color: #4ade80;">Optimal</span>';

        html = `
            <h4 class="ds-section-title"><i class="fas fa-broadcast-tower" style="margin-right: 6px; color: #fbbf24;"></i> Radio Link Quality & Traffic</h4>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 10px;">
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas fa-signal" style="color: ${sigColor}"></i>
                        <span style="font-size: 11px; color: var(--text-muted);">Signal Strength</span>
                    </div>
                    <div style="font-size: 15px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${sigDisplay}</div>
                </div>
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <div style="display: flex; align-items: center; gap: 8px;">
                        <i class="fas fa-link" style="color: #60a5fa;"></i>
                        <span style="font-size: 11px; color: var(--text-muted);">Quality / CCQ</span>
                    </div>
                    <div style="font-size: 15px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${ccqDisplay}</div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <span style="font-size: 11px; color: var(--text-muted);"><i class="fas fa-download" style="color: #4ade80;"></i> Rx (Download)</span>
                    <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(device.rx_rate || 0, 'Mbps')}</div>
                </div>
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <span style="font-size: 11px; color: var(--text-muted);"><i class="fas fa-upload" style="color: #60a5fa;"></i> Tx (Upload)</span>
                    <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(device.tx_rate || 0, 'Mbps')}</div>
                </div>
            </div>
        `;
    } else if (type.includes('ap') || type.includes('access point') || type.includes('ruijie') || type.includes('tplink')) {
        const clients = device.connected_clients || device.clients || 0;
        
        html = `
            <h4 class="ds-section-title"><i class="fas fa-wifi" style="margin-right: 6px; color: #34d399;"></i> Wireless AP Statistics</h4>
            <div class="ds-metric-card" style="margin-bottom: 8px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px; display: flex; align-items: center; gap: 12px;">
                <div class="ds-metric-icon" style="color: #c084fc; font-size: 20px;"><i class="fas fa-users"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted); display: block;">Connected Clients</span>
                    <div style="font-size: 16px; font-weight: 700; color: var(--text-main);">${clients} Active Users</div>
                </div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <span style="font-size: 11px; color: var(--text-muted);"><i class="fas fa-download" style="color: #4ade80;"></i> Rx (Download)</span>
                    <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(device.rx_rate || 0, 'Mbps')}</div>
                </div>
                <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px;">
                    <span style="font-size: 11px; color: var(--text-muted);"><i class="fas fa-upload" style="color: #60a5fa;"></i> Tx (Upload)</span>
                    <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 4px;">${formatBytes(device.tx_rate || 0, 'Mbps')}</div>
                </div>
            </div>
        `;
    } else {
        html = `
            <h4 class="ds-section-title"><i class="fas fa-info-circle" style="margin-right: 6px; color: var(--accent-blue);"></i> Device Health & Operational Status</h4>
            <div class="ds-metric-card" style="background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 10px; border-radius: 8px; display: flex; align-items: center; gap: 12px;">
                <div class="ds-metric-icon" style="color: ${isUp ? '#4ade80' : '#f87171'}; font-size: 20px;"><i class="fas fa-heartbeat"></i></div>
                <div class="ds-metric-details">
                    <span style="font-size: 11px; color: var(--text-muted); display: block;">System Operational Status</span>
                    <div style="font-size: 14px; font-weight: 700; color: ${isUp ? '#4ade80' : '#f87171'};">
                        ${isUp ? 'Operational & Monitored' : 'Device Offline'}
                    </div>
                </div>
            </div>
        `;
    }

    elDynamic.innerHTML = html;
}

// Fetch and render physical & wireless interfaces for device with caching
async function fetchDeviceInterfaces(ip) {
    const listContainer = document.getElementById('ds-interfaces-list');
    if (!listContainer || !ip) return;

    // Check short-term memory cache
    const now = Date.now();
    if (interfaceCache.has(ip)) {
        const cached = interfaceCache.get(ip);
        if (now - cached.timestamp < CACHE_TTL_MS) {
            renderInterfacesUI(cached.data, listContainer);
            return;
        }
    }

    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 5000);
        
        const res = await fetch(`/api/interfaces?ip=${encodeURIComponent(ip)}`, { signal: controller.signal });
        clearTimeout(timeoutId);
        
        if (!res.ok) {
            listContainer.innerHTML = '<div style="color: var(--text-muted); padding: 8px; font-size: 12px; opacity: 0.8;"><i class="fas fa-info-circle" style="margin-right: 4px;"></i> ICMP Monitoring Active.</div>';
            return;
        }
        
        let ifaces = await res.json();
        if (!Array.isArray(ifaces) || ifaces.length === 0) {
            listContainer.innerHTML = '<div style="color: var(--text-muted); padding: 8px; font-size: 12px; opacity: 0.8;"><i class="fas fa-shield-alt" style="margin-right: 4px; color: #f87171;"></i> SNMP Interfaces not detected (ICMP Monitor Active).</div>';
            return;
        }

        // Cache response
        interfaceCache.set(ip, { timestamp: now, data: ifaces });
        renderInterfacesUI(ifaces, listContainer);
    } catch (e) {
        listContainer.innerHTML = '<div style="color: var(--text-muted); padding: 8px; font-size: 12px; opacity: 0.8;"><i class="fas fa-info-circle" style="margin-right: 4px;"></i> ICMP Monitoring Active.</div>';
    }
}

function renderInterfacesUI(ifaces, listContainer) {
    if (!listContainer) return;

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
    let upCount = 0;

    activeIfaces.forEach(iface => {
        const isPortUp = (iface.status || iface.interface_status || '').toLowerCase() === 'up';
        if (isPortUp) upCount++;
    });

    const elPortSummary = document.getElementById('ds-switch-ports-val');
    if (elPortSummary) {
        elPortSummary.innerText = `${upCount} / ${activeIfaces.length} Ports Active`;
    }

    activeIfaces.slice(0, 8).forEach(iface => {
        const isPortUp = (iface.status || iface.interface_status || '').toLowerCase() === 'up';
        const statusColor = isPortUp ? '#22c55e' : '#ef4444';
        const name = iface.name || iface.interface_name || 'Port';
        const rxMbps = iface.rx_rate || iface.rx_mbps || 0;
        const txMbps = iface.tx_rate || iface.tx_mbps || 0;

        html += `
            <div style="display: flex; justify-content: space-between; align-items: center; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06); padding: 8px 12px; border-radius: 6px;">
                <div style="display: flex; align-items: center; gap: 8px;">
                    <span style="width: 8px; height: 8px; border-radius: 50%; background: ${statusColor}; display: inline-block; box-shadow: ${isPortUp ? '0 0 8px #22c55e' : 'none'};"></span>
                    <strong style="color: var(--text-main); font-size: 11px;">${name}</strong>
                    <span style="font-size: 9px; padding: 1px 5px; border-radius: 4px; background: ${isPortUp ? 'rgba(34, 197, 94, 0.15)' : 'rgba(239, 68, 68, 0.15)'}; color: ${statusColor}; font-weight: 700;">${isPortUp ? 'UP' : 'DOWN'}</span>
                </div>
                <div style="font-size: 10px; color: var(--text-muted); font-family: monospace;">
                    <span style="color: #4ade80;"><i class="fas fa-arrow-down" style="font-size:9px;"></i> ${formatBytes(rxMbps, 'Mbps')}</span>
                    <span style="color: #60a5fa; margin-left: 6px;"><i class="fas fa-arrow-up" style="font-size:9px;"></i> ${formatBytes(txMbps, 'Mbps')}</span>
                </div>
            </div>
        `;
    });

    if (activeIfaces.length > 8) {
        html += `<div style="text-align: center; font-size: 10px; color: var(--text-muted); padding-top: 6px;">+ ${activeIfaces.length - 8} more ports...</div>`;
    }

    listContainer.innerHTML = html;
}
