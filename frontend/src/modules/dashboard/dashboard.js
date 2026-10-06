// Dashboard Module
// Network Monitoring System - Dashboard View Logic

import { isOnline } from '../../shared/utils/helpers.js';
import { metricsStore } from '../../core/state/store.js';
import Chart from 'chart.js/auto';
window.Chart = Chart; // Expose globally for reports.js and interfaces.js

let bandwidthChartInstance = null;
let distributionChartInstance = null;
let bandwidthData = {
    labels: [],
    download: [],
    upload: []
};

function generateBaselineTimeline(currentDownload = 0, currentUpload = 0) {
    const labels = [];
    const download = [];
    const upload = [];
    const now = new Date();
    for (let i = 19; i >= 0; i--) {
        const t = new Date(now.getTime() - i * 5000);
        const TimeLabel = `${t.getHours().toString().padStart(2, '0')}:${t.getMinutes().toString().padStart(2, '0')}:${t.getSeconds().toString().padStart(2, '0')}`;
        labels.push(TimeLabel);
        download.push(currentDownload);
        upload.push(currentUpload);
    }
    return { labels, download, upload };
}

let currentBwDuration = '30m';

export async function loadBandwidthHistory(duration = currentBwDuration) {
    currentBwDuration = duration;
    try {
        const res = await fetch('/api/bandwidth/history?duration=' + encodeURIComponent(duration));
        if (res.ok) {
            let data = await res.json();
            if (Array.isArray(data) && data.length > 0) {
                if (data.length < 20) {
                    const needed = 20 - data.length;
                    const padded = [];
                    const now = new Date();
                    for (let i = needed; i > 0; i--) {
                        const t = new Date(now.getTime() - (i + data.length) * 10000);
                        const TimeLabel = `${t.getHours().toString().padStart(2, '0')}:${t.getMinutes().toString().padStart(2, '0')}:${t.getSeconds().toString().padStart(2, '0')}`;
                        padded.push({ Timestamp: TimeLabel, download: 0, upload: 0 });
                    }
                    data = [...padded, ...data];
                }
                bandwidthData.labels = data.map(item => item.Timestamp);
                bandwidthData.download = data.map(item => parseFloat((item.download || 0).toFixed(0)));
                bandwidthData.upload = data.map(item => parseFloat((item.upload || 0).toFixed(0)));
            } else {
                const baseline = generateBaselineTimeline();
                bandwidthData.labels = baseline.labels;
                bandwidthData.download = baseline.download;
                bandwidthData.upload = baseline.upload;
            }
        } else {
            const baseline = generateBaselineTimeline();
            bandwidthData.labels = baseline.labels;
            bandwidthData.download = baseline.download;
            bandwidthData.upload = baseline.upload;
        }

        if (bandwidthChartInstance && isChartAttached(bandwidthChartInstance)) {
            bandwidthChartInstance.data.labels = [...bandwidthData.labels];
            bandwidthChartInstance.data.datasets[0].data = [...bandwidthData.download];
            bandwidthChartInstance.data.datasets[1].data = [...bandwidthData.upload];
            bandwidthChartInstance.update('none');
        }
    } catch (e) {
        console.warn('Failed to fetch bandwidth history:', e);
        const baseline = generateBaselineTimeline();
        bandwidthData.labels = baseline.labels;
        bandwidthData.download = baseline.download;
        bandwidthData.upload = baseline.upload;
    }
}

export async function initDashboard() {
    await loadBandwidthHistory();
    initBandwidthChart();
    initDistributionChart();
    initDeviceTabs();

    if (window._bandwidthInterval) {
        clearInterval(window._bandwidthInterval);
    }
    window._bandwidthInterval = setInterval(() => loadBandwidthHistory(currentBwDuration), 10000);
}

window.onBwTimeRangeChange = async function(duration) {
    // Show a loading state temporarily or just fetch
    await loadBandwidthHistory(duration);
    // Restart interval to avoid immediate fetch if it was just loaded
    if (window._bandwidthInterval) {
        clearInterval(window._bandwidthInterval);
        window._bandwidthInterval = setInterval(() => loadBandwidthHistory(currentBwDuration), 10000);
    }
};

window.resizeDashboardCharts = function() {
    if (bandwidthChartInstance && isChartAttached(bandwidthChartInstance)) {
        bandwidthChartInstance.resize();
    }
    if (distributionChartInstance && isChartAttached(distributionChartInstance)) {
        distributionChartInstance.resize();
    }
};

if (!window._dashboardResizeAttached) {
    window._dashboardResizeAttached = true;
    window.addEventListener('resize', () => {
        window.resizeDashboardCharts();
    });
}

window.filterDeviceTable = function(type, btn) {
    if (btn) {
        const container = btn.closest('.filter-pills');
        if (container) {
            container.querySelectorAll('.filter-pill').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
        }
    }
    const filterValue = type === 'all' ? 'Semua' : type;
    if (typeof window.setDeviceFilter === 'function') {
        window.setDeviceFilter(filterValue);
    }
};

window.switchTopRankingTab = function(tabName, btn) {
    if (btn) {
        const container = btn.closest('.filter-pills');
        if (container) {
            container.querySelectorAll('.filter-pill').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
        }
    }
    const tabs = {
        'health': 'top-health-tab',
        'bandwidth': 'top-bandwidth-tab',
        'interfaces': 'top-interface-tab'
    };
    Object.keys(tabs).forEach(k => {
        const el = document.getElementById(tabs[k]);
        if (el) {
            el.style.display = (k === tabName) ? 'block' : 'none';
        }
    });
};



function isChartAttached(chart) {
    if (!chart || chart.destroyed) return false;
    const canvas = chart.canvas;
    if (!canvas || !canvas.ownerDocument || !canvas.ownerDocument.defaultView) return false;
    if (!canvas.parentNode || !document.body.contains(canvas)) return false;
    const activeCanvas = canvas.id ? document.getElementById(canvas.id) : null;
    return activeCanvas === canvas;
}

function initBandwidthChart() {
    let canvas = document.getElementById('bandwidthChart');
    if (!canvas) {
        const container = document.querySelector('.chart-placeholder');
        if (!container) return;

        container.innerHTML = '<canvas id="bandwidthChart" class="chart-canvas"></canvas>';
        container.classList.remove('chart-placeholder');
        container.style.border = 'none';
        container.style.background = 'transparent';
        canvas = document.getElementById('bandwidthChart');
    }

    if (!canvas || !canvas.parentNode || !document.body.contains(canvas)) return;

    const ctx = canvas.getContext('2d');

    // Check if Chart is available
    if (typeof Chart === 'undefined') {
        setTimeout(initBandwidthChart, 500);
        return;
    }

    Chart.defaults.color = '#8b92a5';
    Chart.defaults.font.family = "'Inter', sans-serif";

    // Destroy instance yang menempel di canvas (via Chart.js registry) agar tidak error "Canvas already in use"
    const existingChart = Chart.getChart(canvas);
    if (existingChart) {
        try { existingChart.destroy(); } catch (e) {}
        bandwidthChartInstance = null;
    }

    if (bandwidthChartInstance) {
        try { bandwidthChartInstance.destroy(); } catch (e) {}
        bandwidthChartInstance = null;
    }

    const gradientDownload = ctx.createLinearGradient(0, 0, 0, 400);
    gradientDownload.addColorStop(0, 'rgba(129, 140, 248, 0.15)');
    gradientDownload.addColorStop(1, 'rgba(129, 140, 248, 0.0)');

    const gradientUpload = ctx.createLinearGradient(0, 0, 0, 400);
    gradientUpload.addColorStop(0, 'rgba(52, 211, 153, 0.15)');
    gradientUpload.addColorStop(1, 'rgba(52, 211, 153, 0.0)');

    bandwidthChartInstance = new Chart(ctx, {
        type: 'line',
        data: {
            labels: [...bandwidthData.labels],
            datasets: [
                {
                    label: 'RX (Receive) Mbps',
                    data: [...bandwidthData.download],
                    borderColor: '#818cf8',
                    backgroundColor: gradientDownload,
                    borderWidth: 3,
                    tension: 0.4,
                    fill: true,
                    pointRadius: 0,
                    pointHoverRadius: 6,
                    pointHoverBackgroundColor: '#818cf8',
                    pointHoverBorderColor: '#fff',
                    pointHoverBorderWidth: 2
                },
                {
                    label: 'TX (Transmit) Mbps',
                    data: [...bandwidthData.upload],
                    borderColor: '#34d399',
                    backgroundColor: gradientUpload,
                    borderWidth: 3,
                    tension: 0.4,
                    fill: true,
                    pointRadius: 0,
                    pointHoverRadius: 6,
                    pointHoverBackgroundColor: '#34d399',
                    pointHoverBorderColor: '#fff',
                    pointHoverBorderWidth: 2
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { display: false },
                tooltip: {
                    mode: 'index',
                    intersect: false,
                    backgroundColor: '#1c1e26',
                    titleColor: '#f0f2f5',
                    bodyColor: '#8b92a5',
                    borderColor: '#2b2e38',
                    borderWidth: 1
                }
            },
            scales: {
                x: {
                    grid: { display: false, drawBorder: false },
                    ticks: { maxTicksLimit: 8 }
                },
                y: {
                    grid: { color: 'rgba(255, 255, 255, 0.05)', drawBorder: false },
                    beginAtZero: true
                }
            },
            interAction: {
                mode: 'nearest',
                axis: 'x',
                intersect: false
            }
        }
    });

    if (bandwidthData.labels.length === 0) {
        loadBandwidthHistory();
    }
}


function initDistributionChart() {
    const canvas = document.getElementById('distributionChart');
    if (!canvas || !canvas.parentNode || !document.body.contains(canvas)) return;

    if (typeof Chart === 'undefined') {
        setTimeout(initDistributionChart, 300);
        return;
    }

    const ctx = canvas.getContext('2d');
    
    // Destroy existing chart instance if exists
    const existingChart = Chart.getChart(canvas);
    if (existingChart) {
        try { existingChart.destroy(); } catch (e) {}
    }
    if (distributionChartInstance) {
        try { distributionChartInstance.destroy(); } catch (e) {}
        distributionChartInstance = null;
    }
    
    distributionChartInstance = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: ['Router', 'Switch', 'Radio', 'Access Point', 'Firewall', 'Server', 'Empty'],
            datasets: [{
                data: [0, 0, 0, 0, 0, 0, 1],
                backgroundColor: ['#3b82f6', '#8b5cf6', '#f59e0b', '#06b6d4', '#ef4444', '#ec4899', 'rgba(255,255,255,0.05)'],
                borderWidth: 2,
                borderColor: '#0f172a',
                hoverOffset: 6
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            cutout: '74%',
            plugins: {
                legend: { display: false },
                tooltip: {
                    enabled: true,
                    backgroundColor: 'rgba(15, 23, 42, 0.95)',
                    titleColor: '#f8fafc',
                    bodyColor: '#cbd5e1',
                    borderColor: 'rgba(255, 255, 255, 0.1)',
                    borderWidth: 1,
                    padding: 8,
                    callbacks: {
                        label: function (context) {
                            let label = context.label || '';
                            if (label === 'Empty' || label === 'Offline/Unknown') return ` Offline/Unknown: ${context.parsed} unit`;
                            return ` ${label}: ${context.parsed} unit`;
                        }
                    }
                }
            },
            animation: { animateScale: true, animateRotate: true }
        }
    });

    const targetMetrics = metricsStore.getState().fullMetrics;
    if (targetMetrics && targetMetrics.length > 0) {
        updateDashboardCharts(targetMetrics);
    }
}

function initDeviceTabs() {
    const cardHeaders = document.querySelectorAll('.card-title');
    let tabContainer = null;
    cardHeaders.forEach(h => {
        if (h.innerText.trim() === 'Status Perangkat Terbaru' || h.innerText.trim() === 'Latest Device Status') {
            tabContainer = h.nextElementSibling;
        }
    });

    if (tabContainer) {
        const buttons = tabContainer.querySelectorAll('button');
        buttons.forEach(btn => {
            btn.addEventListener('click', (e) => {
                // Clear active states
                buttons.forEach(b => {
                    b.classList.remove('active');
                    // Remove inline styles if any are left over from HTML
                    b.style.background = '';
                    b.style.border = '';
                    b.style.color = '';
                });

                const target = e.currentTarget;
                target.classList.add('active');
                target.style.background = 'var(--accent-blue)';
                target.style.border = 'none';
                target.style.color = '#fff';

                // Trigger filter update
                if (typeof window.setDeviceFilter === 'function') {
                    window.setDeviceFilter(target.innerText.trim());
                }
            });
        });
    }
}

export function updateDashboardCharts(metrics) {
    // Gunakan metricsStore
    let fullList = metricsStore.getState().fullMetrics;
    if (!fullList || fullList.length === 0) {
        fullList = metrics || [];
    }

    if (!fullList || !Array.isArray(fullList) || fullList.length === 0) return;

    const total = fullList.length;
    let counts = { router: 0, switch: 0, radio: 0, ap: 0, firewall: 0, server: 0 };

    fullList.forEach(m => {
        const t = (m.device_type || m.type || '').toLowerCase();
        if (t.includes('router') || t.includes('mikrotik')) counts.router++;
        else if (t.includes('switch')) counts.switch++;
        else if (t.includes('radio') || t.includes('wireless') || t.includes('ptp') || t.includes('ubiquiti') || t.includes('airmax')) counts.radio++;
        else if (t.includes('ap') || t.includes('access point') || t.includes('access_point') || t.includes('ruijie') || t.includes('tplink') || t.includes('tp-link')) counts.ap++;
        else if (t.includes('firewall') || t.includes('security')) counts.firewall++;
        else if (t.includes('server')) counts.server++;
        else counts.router++; // default fallback
    });

    const trueTotal = Math.max(total, window.globalTrueDeviceTotal || 0);
    const unknownCount = trueTotal - total;

    const distTotal = document.getElementById('dist-total');
    if (distTotal) distTotal.innerText = trueTotal;

    const updateLegend = (id, count, itemId) => {
        const el = document.getElementById(id);
        if (el) {
            const perc = trueTotal > 0 ? ((count / trueTotal) * 100).toFixed(1) : '0.0';
            el.innerHTML = `${count} <span style="font-size: 9px; color: var(--text-muted); font-weight: 400;">(${perc}%)</span>`;
        }
        const itemEl = document.getElementById(itemId);
        if (itemEl) {
            itemEl.style.opacity = count === 0 ? '0.45' : '1';
        }
    };

    updateLegend('dist-router-text', counts.router, 'dist-router-item');
    updateLegend('dist-switch-text', counts.switch, 'dist-switch-item');
    updateLegend('dist-radio-text', counts.radio, 'dist-radio-item');
    updateLegend('dist-ap-text', counts.ap, 'dist-ap-item');
    updateLegend('dist-firewall-text', counts.firewall, 'dist-firewall-item');
    updateLegend('dist-server-text', counts.server, 'dist-server-item');

    if (distributionChartInstance && !isChartAttached(distributionChartInstance)) {
        try { distributionChartInstance.destroy(); } catch (e) {}
        distributionChartInstance = null;
    }

    if (!distributionChartInstance && document.getElementById('distributionChart')) {
        initDistributionChart();
    }

    if (isChartAttached(distributionChartInstance)) {
        try {
            const hasData = counts.router > 0 || counts.switch > 0 || counts.radio > 0 || counts.ap > 0 || counts.firewall > 0 || counts.server > 0 || unknownCount > 0;
            if (hasData) {
                distributionChartInstance.data.datasets[0].data = [counts.router, counts.switch, counts.radio, counts.ap, counts.firewall, counts.server, unknownCount];
            } else {
                distributionChartInstance.data.datasets[0].data = [0, 0, 0, 0, 0, 0, 1];
            }
            distributionChartInstance.update();
        } catch (e) {
            console.warn('Failed to update distribution chart:', e);
            try { distributionChartInstance.destroy(); } catch (_) {}
            distributionChartInstance = null;
        }
    }

    // Update Bandwidth Chart
    // Dari sudut pandang router distribusi:
    //   tx_rate (OutOctets) = data yang dikirim router ke pelanggan = DOWNLOAD User
    //   rx_rate (InOctets)  = data yang diterima router dari pelangkan = UPLOAD User
    // Exclude perangkat Radio (link lokal, bukan internet)

    // Hitung total download/upload (Eksklusi non-router untuk cegah double counting)
    const now = new Date();
    const TimeLabel = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`;

    let totalDownload = 0;
    let totalUpload = 0;
    metrics.forEach(m => {
        const devType = (m.device_type || m.type || '').toLowerCase();
        const isGateway = devType.includes('router') || devType.includes('firewall');
        if (!isGateway) return; // Hanya hitung router/firewall untuk chart global
        // rx_rate = Traffic IN (RX), kita petakan ke grafik Biru
        // tx_rate = Traffic OUT (TX), kita petakan ke grafik Hijau
        totalDownload += (parseFloat(m.rx_rate) || 0);
        totalUpload += (parseFloat(m.tx_rate) || 0);
    });

    const dlVal = parseFloat(totalDownload.toFixed(0));
    const ulVal = parseFloat(totalUpload.toFixed(0));

    if (bandwidthData.labels.length < 5) {
        const baseline = generateBaselineTimeline(dlVal, ulVal);
        bandwidthData.labels = baseline.labels;
        bandwidthData.download = baseline.download;
        bandwidthData.upload = baseline.upload;
    } else {
        const len = bandwidthData.labels.length;
        if (len > 0 && bandwidthData.labels[len - 1] === TimeLabel) {
            bandwidthData.download[len - 1] = dlVal;
            bandwidthData.upload[len - 1] = ulVal;
        } else {
            bandwidthData.labels.push(TimeLabel);
            bandwidthData.download.push(dlVal);
            bandwidthData.upload.push(ulVal);

            if (bandwidthData.labels.length > 30) {
                bandwidthData.labels.shift();
                bandwidthData.download.shift();
                bandwidthData.upload.shift();
            }
        }
    }

    if (bandwidthChartInstance && !isChartAttached(bandwidthChartInstance)) {
        try { bandwidthChartInstance.destroy(); } catch (e) {}
        bandwidthChartInstance = null;
    }

    if (!bandwidthChartInstance && document.getElementById('bandwidthChart')) {
        initBandwidthChart();
    }

    if (isChartAttached(bandwidthChartInstance)) {
        try {
            bandwidthChartInstance.data.labels = bandwidthData.labels;
            bandwidthChartInstance.data.datasets[0].data = bandwidthData.download;
            bandwidthChartInstance.data.datasets[1].data = bandwidthData.upload;
            bandwidthChartInstance.update('none');
        } catch (e) {
            console.warn('Failed to update bandwidth chart:', e);
            try { bandwidthChartInstance.destroy(); } catch (_) {}
            bandwidthChartInstance = null;
        }
    }
}