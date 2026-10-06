// Interfaces Module
// Network Monitoring System - Interface Monitoring Logic

import { isOnline, getInterfaceStatusMeta, normalizeInterfaceStatus, metricQualityClass } from '../../shared/utils/helpers.js';
import { formatTraffic, formatLinkSpeed } from '../../shared/utils/format.js';
import { state } from '../../core/state.js';

// Initialize interfaces module
export function initInterfaces() {
    // Setup event listeners
    setupInterfaceEventListeners();
}

// Setup event listeners for interfaces
function setupInterfaceEventListeners() {
    // Traffic interface select change
    const select = document.getElementById('traffic-interface-select');
    if (select) {
        select.addEventListener('change', handleInterfaceChange);
    }
}

// Handle interface selection change
function handleInterfaceChange(e) {
    const val = e.target.value;
    const ip = state.currentDetailIp;
    if (!ip || !state.ifaceTrafficHistory[ip]) return;
    
    const selectedHistory = state.ifaceTrafficHistory[ip][val];
    const chart = state.charts.detailTraffic;
    if (selectedHistory && chart && !chart.destroyed && chart.canvas && chart.canvas.parentNode && document.body.contains(chart.canvas)) {
        try {
            chart.data.labels = Array(selectedHistory.inRates.length).fill('');
            chart.data.datasets[0].data = [...selectedHistory.inRates];
            chart.data.datasets[1].data = [...selectedHistory.outRates];
            chart.update('none');
        } catch (err) {
            console.error('Failed to update detailTraffic chart:', err);
        }
    }
}

// Render interface summary
export function renderInterfaceSummary(interfaces) {
    const counts = { up: 0, down: 0, disabled: 0, total: Array.isArray(interfaces) ? interfaces.length : 0 };
    (interfaces || []).forEach(iface => {
        const Status = normalizeInterfaceStatus(iface.status);
        if (Status === 'up') counts.up += 1;
        else if (Status === 'disabled') counts.disabled += 1;
        else counts.down += 1;
    });
    
    const upEl = document.getElementById('iface-count-up');
    const downEl = document.getElementById('iface-count-down');
    const disabledEl = document.getElementById('iface-count-disabled');
    const totalEl = document.getElementById('iface-count-total');
    
    if (upEl) upEl.innerText = counts.up;
    if (downEl) downEl.innerText = counts.down;
    if (disabledEl) disabledEl.innerText = counts.disabled;
    if (totalEl) totalEl.innerText = counts.total;
}

// Select interface for chart
export function selectInterfaceForChart(ifaceName) {
    const selectEl = document.getElementById('traffic-interface-select');
    if (selectEl) {
        selectEl.value = ifaceName;
        selectEl.dispatchEvent(new Event('change'));
    }
}

// Initialize detail chart
export function initDetailChart() {
    if (!window.Chart) return;
    const ctxRes = document.getElementById('detailChart');
    const ctxTraff = document.getElementById('detailTrafficChart');
    if (!ctxRes || !ctxTraff) return;
    
    if (state.charts.detail) state.charts.detail.destroy();
    if (state.charts.detailTraffic) state.charts.detailTraffic.destroy();
    
    state.charts.detail = new Chart(ctxRes, {
        type: 'line',
        data: {
            labels: Array(20).fill(''),
            datasets: [
                { label: 'CPU (%)', data: Array(20).fill(0), borderColor: '#38bdf8', borderWidth: 3, tension: 0.4, fill: true, backgroundColor: 'rgba(56, 189, 248, 0.08)', pointRadius: 0, yAxisID: 'y' },
                { label: 'RAM (%)', data: Array(20).fill(0), borderColor: '#818cf8', borderWidth: 3, tension: 0.4, fill: true, backgroundColor: 'rgba(129, 140, 248, 0.08)', pointRadius: 0, yAxisID: 'y' },
                { label: 'Ping Latency (ms)', data: Array(20).fill(0), borderColor: '#fbbf24', borderWidth: 2, tension: 0.3, fill: false, pointRadius: 2, yAxisID: 'y1' }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: { legend: { position: 'bottom', labels: { color: '#94a3b8', boxWidth: 12 } } },
            scales: {
                y: { min: 0, max: 100, position: 'left', grid: { color: 'rgba(255,255,255,0.05)' } },
                y1: { min: 0, position: 'right', grid: { drawOnChartArea: false }, ticks: { color: '#fbbf24' } },
                x: { display: false }
            }
        }
    });
    
    state.charts.detailTraffic = new Chart(ctxTraff, {
        type: 'line',
        data: {
            labels: Array(20).fill(''),
            datasets: [
                { label: 'Inbound (Mbps)', data: Array(20).fill(0), borderColor: '#10b981', borderWidth: 3, tension: 0.4, fill: true, backgroundColor: 'rgba(16, 185, 129, 0.1)', pointRadius: 0 },
                { label: 'Outbound (Mbps)', data: Array(20).fill(0), borderColor: '#38bdf8', borderWidth: 3, tension: 0.4, fill: true, backgroundColor: 'rgba(56, 189, 248, 0.1)', pointRadius: 0 }
            ]
        },
        options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { position: 'bottom', labels: { color: '#94a3b8', boxWidth: 12 } } }, scales: { y: { beginAtZero: true, grid: { color: 'rgba(255,255,255,0.05)' } }, x: { display: false } } }
    });
}

// Render wireless quality
export function renderWirelessQuality(metric) {
    const card = document.getElementById('wireless-quality-card');
    const panel = document.getElementById('wireless-quality-panel');
    if (!card || !panel) return;
    
    const type = String(metric?.device_type || '').toLowerCase();
    const isWireless = type === 'radio' || type === 'access point' || type.includes('radio') || type.includes('ubiquiti') || type.includes('airmax') || type.includes('wireless');
    const hasWirelessData = Number(metric?.signal_strength || 0) !== 0 || Number(metric?.ccq || 0) !== 0 || Number(metric?.tx_rate || 0) !== 0 || Number(metric?.rx_rate || 0) !== 0;
    
    card.style.display = (isWireless || hasWirelessData) ? 'block' : 'none';
    if (card.style.display === 'none') return;
    
    // Render wireless quality items
    panel.innerHTML = renderWirelessQualityItems(metric);
}

function renderWirelessQualityItems(metric) {
    const signal = Number(metric?.signal_strength || 0);
    const ccq = Number(metric?.ccq || 0);
    const txRate = Number(metric?.tx_rate || 0);
    const rxRate = Number(metric?.rx_rate || 0);
    const noiseFloor = metric?.noise_floor ?? metric?.noise ?? null;
    const mcsRate = metric?.mcs_rate ?? metric?.mcs ?? null;
    
    const items = [
        { label: 'RSSI / Signal', value: signal ? `${signal.toFixed(0)} dBm` : 'N/A', raw: signal, type: 'signal', icon: 'fas fa-signal' },
        { label: 'CCQ', value: ccq ? `${ccq.toFixed(1)}%` : 'N/A', raw: ccq, type: 'ccq', icon: 'fas fa-chart-line' },
        { label: 'Noise Floor', value: noiseFloor !== null ? `${Number(noiseFloor).toFixed(0)} dBm` : 'N/A', raw: noiseFloor, type: 'noise', icon: 'fas fa-wave-square' },
        { label: 'TX / RX Rate', value: `${txRate ? txRate.toFixed(2) : '0.00'} / ${rxRate ? rxRate.toFixed(2) : '0.00'} Mbps`, raw: Math.max(txRate, rxRate), type: 'rate', icon: 'fas fa-right-left' }
    ];
    
    return items.map(item => {
        const quality = metricQualityClass(item.raw, item.type);
        const width = Math.max(8, Math.min(100, Number(item.raw || 0)));
        return `
            <div class="wireless-quality-item ${quality}">
                <div class="quality-icon"><i class="${item.icon}"></i></div>
                <div class="quality-content">
                    <span class="quality-label">${item.label}</span>
                    <strong>${item.value}</strong>
                    <div class="quality-meter"><span style="width:${width}%"></span></div>
                </div>
            </div>
        `;
    }).join('');
}