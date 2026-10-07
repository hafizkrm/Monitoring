// Reports Module
// Network Monitoring System
import { metricsStore } from '../../core/state/store.js';

let currentRawData = null;
let currentMetricsData = [];
let currentType = 'sla';
let currentPage = 1;
const pageSize = 15;
let sortColumnIndex = null;
let sortDirection = 'asc';
let currentChart = null;
let activeReportItems = [];

export function initReports() {
    setupReportEventListeners();
    
    window.generateReport = generateReport;
    window.exportCSV = exportCSV;
    window.printReport = printReport;
    window.copyReportIp = copyReportIp;
    window.handleReportHeaderClick = handleReportHeaderClick;
    window.goToReportPage = goToReportPage;

    // Set default custom dates (start: 7 days ago, end: today)
    const today = new Date();
    const sevenDaysAgo = new Date();
    sevenDaysAgo.setDate(today.getDate() - 7);

    const startDateEl = document.getElementById('report-start-date');
    const endDateEl = document.getElementById('report-end-date');
    if (startDateEl && !startDateEl.value) {
        startDateEl.value = sevenDaysAgo.toISOString().split('T')[0];
    }
    if (endDateEl && !endDateEl.value) {
        endDateEl.value = today.toISOString().split('T')[0];
    }

    // Auto-generate initial report when view is loaded (only if no data yet)
    if (document.getElementById('report-type') && !currentRawData) {
        setTimeout(generateReport, 150);
    }
}

function setupReportEventListeners() {
    const btnGenerate = document.getElementById('btn-generate-report');
    if (btnGenerate) {
        btnGenerate.addEventListener('click', generateReport);
    }

    const btnExportCsv = document.getElementById('btn-export-csv');
    if (btnExportCsv) {
        btnExportCsv.addEventListener('click', exportCSV);
    }

    const reportRangeEl = document.getElementById('report-range');
    const customDateContainer = document.getElementById('custom-date-container');
    if (reportRangeEl) {
        reportRangeEl.addEventListener('change', function () {
            if (customDateContainer) {
                customDateContainer.style.display = this.value === 'custom' ? 'flex' : 'none';
            }
        });
    }

    const reportTypeEl = document.getElementById('report-type');
    if (reportTypeEl) {
        reportTypeEl.addEventListener('change', function () {
            const rangeEl = document.getElementById('report-range');
            if (!rangeEl) return;
            if (['down_frequency', 'traffic_monthly', 'downTime_monthly', 'vpn_monthly'].includes(this.value)) {
                rangeEl.value = '30days';
                if (customDateContainer) customDateContainer.style.display = 'none';
            }
        });
    }

    const searchInput = document.getElementById('report-search-input');
    const clearBtn = document.getElementById('clear-report-search');

    if (searchInput) {
        searchInput.addEventListener('input', (e) => {
            if (clearBtn) clearBtn.style.display = e.target.value.trim() ? 'block' : 'none';
            applyReportFilter();
        });
    }

    if (clearBtn && searchInput) {
        clearBtn.addEventListener('click', () => {
            searchInput.value = '';
            clearBtn.style.display = 'none';
            applyReportFilter();
        });
    }
}

async function generateReport() {
    const typeEl = document.getElementById('report-type');
    const rangeEl = document.getElementById('report-range');
    if (!typeEl || !rangeEl) return;
    
    currentType = typeEl.value;
    const range = rangeEl.value;
    currentPage = 1;
    sortColumnIndex = null;
    sortDirection = 'asc';
    
    const tbody = document.getElementById('report-table-body');
    const rowCountEl = document.getElementById('report-row-count');
    if (rowCountEl) rowCountEl.textContent = '...';
    
    // Show skeleton loading state
    tbody.innerHTML = Array(5).fill(0).map(() => `
        <tr class="table-row">
            ${Array(6).fill('<td class="table-cell" style="padding:12px;"><div class="skeleton skeleton-row" style="height:16px; border-radius:4px;"></div></td>').join('')}
        </tr>
    `).join('');
    
    try {
        let apiUrl = `/api/reports?type=${currentType}&range=${range}`;
        if (range === 'custom') {
            const startDate = document.getElementById('report-start-date')?.value;
            const endDate = document.getElementById('report-end-date')?.value;
            if (startDate) apiUrl += `&start_date=${startDate}`;
            if (endDate) apiUrl += `&end_date=${endDate}`;
        }

        const [response, metricsRes] = await Promise.all([
            fetch(apiUrl),
            fetch('/api/metrics')
        ]);
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        
        currentRawData = await response.json();
        currentMetricsData = metricsRes.ok ? await metricsRes.json() : [];
        
        // Reset search input on new report creation
        const searchInput = document.getElementById('report-search-input');
        if (searchInput) searchInput.value = '';

        renderReport(currentType, currentRawData, currentMetricsData);
        
    } catch (error) {
        console.error('Failed to Buat Reports:', error);
        tbody.innerHTML = `<tr><td colspan="10" class="table-empty-msg text-danger" style="text-align:center; padding:30px;"><i class="fas fa-exclamation-triangle"></i> Failed to load report data. ${error.message}</td></tr>`;
        if (rowCountEl) rowCountEl.textContent = '0';
    }
}

function applyReportFilter() {
    if (!currentRawData) return;
    currentPage = 1;
    const query = (document.getElementById('report-search-input')?.value || '').toLowerCase().trim();
    
    if (!query) {
        renderReport(currentType, currentRawData, currentMetricsData);
        return;
    }

    const filterEditems = (currentRawData.items || []).filter(item => {
        const name = (item.name || item.device_name || '').toLowerCase();
        const ip = (item.ip || item.address || item.caller_id || '').toLowerCase();
        const type = (item.type || item.service || '').toLowerCase();
        const Status = (item.status || '').toLowerCase();
        const msg = (item.message || '').toLowerCase();
        
        return name.includes(query) || ip.includes(query) || type.includes(query) || Status.includes(query) || msg.includes(query);
    });

    renderReport(currentType, { ...currentRawData, items: filterEditems }, currentMetricsData, true);
}

function renderReport(type, data, metricsData = [], isFiltering = false) {
    const tbody = document.getElementById('report-table-body');
    const thead = document.getElementById('report-table-head');
    const summaryCards = document.getElementById('report-summary-cards');
    const rowCountEl = document.getElementById('report-row-count');
    
    tbody.innerHTML = '';
    thead.innerHTML = '';
    
    if (!isFiltering && summaryCards) {
        summaryCards.innerHTML = '';
    }
    
    let items = data ? (data.items || []) : [];

    // Deduplicate VPN User if needed
    if (type === 'vpn_monthly') {
        const seenUser = new Set();
        items = items.filter(item => {
            const Username = item.name || 'Unknown';
            if (seenUser.has(Username)) return false;
            seenUser.add(Username);
            return true;
        });
    }

    activeReportItems = [...items];

    if (rowCountEl) rowCountEl.textContent = activeReportItems.length;

    // Render Summary Cards (only when not filtering)
    if (!isFiltering && data.summary && summaryCards) {
        const summaryFragment = document.createDocumentFragment();
        for (const [key, val] of Object.entries(data.summary)) {
            let icon = 'fa-chart-pie';
            let iconColor = 'var(--accent-blue)';
            const kLow = key.toLowerCase();
            
            if (kLow.includes('total') || kLow.includes('perangkat')) {
                icon = 'fa-server'; iconColor = 'var(--accent-blue)';
            } else if (kLow.includes('upTime') || kLow.includes('online') || kLow.includes('sla')) {
                icon = 'fa-check-circle'; iconColor = 'var(--accent-green)';
            } else if (kLow.includes('down') || kLow.includes('downTime') || kLow.includes('loss')) {
                icon = 'fa-exclamation-triangle'; iconColor = 'var(--accent-red)';
            } else if (kLow.includes('traffic') || kLow.includes('bandwidth')) {
                icon = 'fa-exchange-alt'; iconColor = 'var(--accent-purple)';
            } else if (kLow.includes('latency') || kLow.includes('ping')) {
                icon = 'fa-tachometer-alt'; iconColor = 'var(--accent-yellow)';
            }

            const card = document.createElement('div');
            card.style.cssText = 'background: rgba(255,255,255,0.03); border: 1px solid var(--border-color); border-radius: 12px; padding: 16px; display: flex; align-items: center; gap: 14px; transition: all 0.2s ease;';
            card.innerHTML = `
                <div style="width: 40px; height: 40px; border-radius: 10px; background: ${iconColor}15; border: 1px solid ${iconColor}30; display: flex; align-items: center; justify-content: center; color: ${iconColor}; font-size: 16px; flex-shrink: 0;">
                    <i class="fas ${icon}"></i>
                </div>
                <div style="display: flex; flex-direction: column; gap: 2px;">
                    <span class="stat-label" style="font-size: 11px; color: var(--text-muted); text-transform: uppercase; font-weight: 600;">${key}</span>
                    <span class="stat-value" style="font-size: 18px; font-weight: 700; color: var(--text-main);">${val}</span>
                </div>
            `;
            summaryFragment.appendChild(card);
        }
        summaryCards.appendChild(summaryFragment);
    }

    // Render Visualization Chart
    if (!isFiltering) {
        renderReportChart(type, activeReportItems, data.summary);
    }

    if (activeReportItems.length === 0) {
        tbody.innerHTML = `<tr><td colspan="10" class="table-empty-msg" style="text-align:center; padding:36px; color:var(--text-muted);"><i class="fas fa-inbox mr-2"></i> Tidak ada data laporan yang cocok.</td></tr>`;
        renderPaginationControls(0);
        return;
    }

    // Render Table Headers
    let headers = [];
    if (type === 'availability' || type === 'sla') {
        headers = ['Device', 'IP Address', 'Type', 'Total UpTime', 'Total DownTime', 'Down Count', 'Availability %', 'Target SLA (99%)', 'Current Status'];
    } else if (type === 'performance') {
        headers = ['Device', 'IP Address', 'Avg CPU', 'Max CPU', 'Avg RAM', 'Max RAM', 'Avg Traffic', 'Current Status'];
    } else if (type === 'incidents') {
        headers = ['Time', 'Device', 'Severity', 'Alert Message', 'Status'];
    } else if (type === 'down_frequency') {
        headers = ['Rank', 'Device', 'IP Address', 'Type', 'Down Count', 'Last Down', 'Current Status'];
    } else if (type === 'traffic_monthly' || type === 'bandwidth_top') {
        headers = ['Device', 'IP Address', 'Avg Traffic', 'Max Traffic', 'Current Status'];
    } else if (type === 'downTime_monthly') {
        headers = ['Device', 'IP Address', 'Down Count', 'Total Downtime Duration', 'Current Status'];
    } else if (type === 'vpn_monthly') {
        headers = ['Last Connection', 'Username', 'Service', 'IP Address', 'Connected Duration'];
    } else if (type === 'network_quality') {
        headers = ['Device', 'IP Address', 'Avg Latency', 'Packet Loss', 'Current Status'];
    }
    
    const trHead = document.createElement('tr');
    trHead.style.cssText = 'font-size: 11px; text-transform: uppercase; background: rgba(255,255,255,0.02);';
    headers.forEach((h, colIdx) => {
        const th = document.createElement('th');
        th.style.cssText = 'padding: 12px 14px; font-weight: 700; color: var(--text-muted); border-bottom: 1px solid var(--border-color); cursor: pointer; User-select: none;';
        
        let sortIndicator = '';
        if (sortColumnIndex === colIdx) {
            sortIndicator = sortDirection === 'asc' ? ' ▲' : ' ▼';
        }
        th.textContent = h + sortIndicator;
        th.title = "Klik untuk mengurutkan kolom ini";
        th.onclick = () => window.handleReportHeaderClick(colIdx);
        trHead.appendChild(th);
    });
    thead.appendChild(trHead);

    renderCurrentTablePage();
}

function handleReportHeaderClick(colIdx) {
    if (sortColumnIndex === colIdx) {
        sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
        sortColumnIndex = colIdx;
        sortDirection = 'asc';
    }
    
    sortActiveItems(colIdx, sortDirection);
    renderReport(currentType, { ...currentRawData, items: activeReportItems }, currentMetricsData, true);
}

function sortActiveItems(colIdx, direction) {
    if (!activeReportItems || activeReportItems.length === 0) return;

    activeReportItems.sort((a, b) => {
        let valA = getItemValueByCol(a, colIdx);
        let valB = getItemValueByCol(b, colIdx);

        // Convert numeric strings if applicable
        const numA = parseFloat(valA);
        const numB = parseFloat(valB);
        if (!isNaN(numA) && !isNaN(numB)) {
            valA = numA;
            valB = numB;
        }

        if (valA < valB) return direction === 'asc' ? -1 : 1;
        if (valA > valB) return direction === 'asc' ? 1 : -1;
        return 0;
    });
}

function getItemValueByCol(item, colIdx) {
    if (currentType === 'availability' || currentType === 'sla') {
        const mapCols = [item.name, item.ip, item.type, item.upTime_str, item.downTime_str, item.down_count, item.availability_pct, item.status];
        return mapCols[colIdx] ?? '';
    } else if (currentType === 'performance') {
        const mapCols = [item.name, item.ip, item.avg_cpu, item.max_cpu, item.avg_ram, item.max_ram, item.avg_traffic, item.status];
        return mapCols[colIdx] ?? '';
    } else if (currentType === 'incidents') {
        const mapCols = [item.created_at, item.device_name, item.severity, item.message, item.status];
        return mapCols[colIdx] ?? '';
    } else if (currentType === 'down_frequency') {
        const mapCols = [item.rank, item.name, item.ip, item.type, item.down_count, item.last_down, item.status];
        return mapCols[colIdx] ?? '';
    } else if (currentType === 'network_quality') {
        const mapCols = [item.name, item.ip, item.latency, item.packet_loss, item.status];
        return mapCols[colIdx] ?? '';
    } else if (currentType === 'vpn_monthly') {
        const mapCols = [item.connected_at, item.name, item.service, item.ip || item.caller_id, item.upTime];
        return mapCols[colIdx] ?? '';
    } else if (currentType === 'traffic_monthly' || currentType === 'bandwidth_top') {
        const mapCols = [item.name, item.ip, item.avg_traffic, item.max_traffic, item.status];
        return mapCols[colIdx] ?? '';
    } else if (currentType === 'downTime_monthly') {
        const mapCols = [item.name, item.ip, item.kali_down, item.durasi_down, item.status];
        return mapCols[colIdx] ?? '';
    }
    return item.name || item.device_name || '';
}

function formatVpnDuration(val) {
    if (!val || val === '—') return '—';
    if (typeof val === 'string' && /[0-9]+[wdhms]/.test(val)) {
        return val.replace(/([0-9]+)([wdhms])/g, '$1$2 ').trim();
    }
    return val;
}


function formatPollsToHuman(val) {
    if (!val || val === '—') return '0m';
    if (typeof val === 'string' && val.includes('polls')) {
        const polls = parseInt(val.replace(/[^\d]/g, ''), 10) || 0;
        const totalSeconds = polls * 60;
        if (totalSeconds <= 0) return '0m';
        
        const days = Math.floor(totalSeconds / 86400);
        const hours = Math.floor((totalSeconds % 86400) / 3600);
        const mins = Math.floor((totalSeconds % 3600) / 60);

        if (days > 0) return `${days}d ${hours}h ${mins}m`;
        if (hours > 0) return `${hours}h ${mins}m`;
        return `${mins}m`;
    }
    return val;
}

function renderCurrentTablePage() {

    const tbody = document.getElementById('report-table-body');
    tbody.innerHTML = '';

    const totalItems = activeReportItems.length;
    const totalPages = Math.ceil(totalItems / pageSize) || 1;

    if (currentPage > totalPages) currentPage = totalPages;
    if (currentPage < 1) currentPage = 1;

    const startIdx = (currentPage - 1) * pageSize;
    const endIdx = Math.min(startIdx + pageSize, totalItems);
    const pageItems = activeReportItems.slice(startIdx, endIdx);

    const fragment = document.createDocumentFragment();
    pageItems.forEach((item, idx) => {
        const globalIdx = startIdx + idx;
        const tr = document.createElement('tr');
        tr.style.cssText = 'border-bottom: 1px solid rgba(255,255,255,0.04); transition: background 0.15s ease;';
        tr.addEventListener('mouseenter', () => tr.style.background = 'rgba(255,255,255,0.025)');
        tr.addEventListener('mouseleave', () => tr.style.background = 'transparent');
        
        let liveStatus = 'Unknown';
        const itemIp = item.ip || item.address || item.caller_id || '';
        if (currentMetricsData && currentMetricsData.length > 0 && itemIp) {
            const m = currentMetricsData.find(x => x.ip === itemIp);
            if (m) liveStatus = m.status || 'Offline';
        } else {
            const fullMetrics = metricsStore.getState().fullMetrics || [];
            if (fullMetrics.length > 0) {
                const dev = fullMetrics.find(d => d.ip === itemIp || d.name === item.name);
                if (dev) liveStatus = dev.status || 'Offline';
            }
        }
        
        const isLiveOnline = (liveStatus.toLowerCase() === 'up' || liveStatus.toLowerCase() === 'online');
        const liveBadge = isLiveOnline 
            ? `<span class="badge-Severity badge-Severity-success" style="font-size:10px; padding:3px 8px;"><i class="fas fa-circle text-xs mr-1"></i>ONLINE</span>`
            : `<span class="badge-Severity badge-Severity-danger" style="font-size:10px; padding:3px 8px;"><i class="fas fa-circle text-xs mr-1"></i>OFFLINE</span>`;

        const copyIpBtn = itemIp ? `<span onclick="window.copyReportIp('${itemIp}')" title="Klik untuk menyalin IP" style="cursor:pointer; font-family: monospace; color: var(--text-muted); font-size:11px;" class="hover:text-blue-400 transition-colors">${itemIp} <i class="fas fa-copy text-[10px] ml-1 opacity-60"></i></span>` : '—';

        if (currentType === 'availability' || currentType === 'sla') {
            const availPct = parseFloat(item.availability_pct) || 0;
            const isCompliant = item.sla_Status === 'COMPLIANT' || availPct >= 99.0;
            const slaBadge = isCompliant
                ? `<span class="badge-Severity badge-Severity-success" style="font-size:10px; padding:3px 8px; font-weight:700; background:rgba(16,185,129,0.15); color:#10b981; border:1px solid rgba(16,185,129,0.3); border-radius:4px;"><i class="fas fa-check-circle text-xs mr-1"></i>COMPLIANT</span>`
                : `<span class="badge-Severity badge-Severity-danger" style="font-size:10px; padding:3px 8px; font-weight:700; background:rgba(239,68,68,0.15); color:#ef4444; border:1px solid rgba(239,68,68,0.3); border-radius:4px;"><i class="fas fa-times-circle text-xs mr-1"></i>VIOLATED</span>`;

            tr.innerHTML = `
                <td style="padding: 12px 14px;" class="font-semibold">${item.name || '—'}</td>
                <td style="padding: 12px 14px;">${copyIpBtn}</td>
                <td style="padding: 12px 14px;"><span class="badge-outline" style="font-size:10px;">${item.type || '—'}</span></td>
                <td style="padding: 12px 14px;" class="text-success font-medium">${formatPollsToHuman(item.upTime_str)}</td>
                <td style="padding: 12px 14px;" class="text-danger font-medium">${formatPollsToHuman(item.downTime_str)}</td>
                <td style="padding: 12px 14px;" class="text-warning font-semibold">${item.down_count !== undefined ? item.down_count + 'x' : '—'}</td>
                <td style="padding: 12px 14px;">
                    <div style="display: flex; align-items: center; gap: 8px; width: 140px;">
                        <div style="flex: 1; height: 6px; background: rgba(255,255,255,0.1); border-radius: 3px; overflow: hidden;">
                            <div style="width: ${availPct}%; height: 100%; background: ${availPct >= 99 ? '#10b981' : (availPct >= 95 ? '#f59e0b' : '#ef4444')}; border-radius: 3px;"></div>
                        </div>
                        <span style="font-size: 11px; font-weight: 700; color: ${availPct >= 99 ? '#10b981' : (availPct >= 95 ? '#f59e0b' : '#ef4444')};">${availPct}%</span>
                    </div>
                </td>
                <td style="padding: 12px 14px;">${slaBadge}</td>
                <td style="padding: 12px 14px;">${liveBadge}</td>
            `;
        } else if (currentType === 'performance') {
            tr.innerHTML = `
                <td style="padding: 12px 14px;" class="font-semibold">${item.name || '—'}</td>
                <td style="padding: 12px 14px;">${copyIpBtn}</td>
                <td style="padding: 12px 14px;" class="font-medium">${item.avg_cpu || 0}%</td>
                <td style="padding: 12px 14px;" class="font-semibold text-warning">${item.max_cpu || 0}%</td>
                <td style="padding: 12px 14px;" class="font-medium">${item.avg_ram || 0}%</td>
                <td style="padding: 12px 14px;" class="font-semibold text-warning">${item.max_ram || 0}%</td>
                <td style="padding: 12px 14px;" class="text-info font-medium">${item.avg_traffic || '0 bps'}</td>
                <td style="padding: 12px 14px;">${liveBadge}</td>
            `;
        } else if (currentType === 'incidents') {
            const badgeClass = item.severity === 'critical' ? 'badge-Severity-critical' : (item.severity === 'warning' ? 'badge-Severity-warning' : 'badge-Severity-info');
            const TimeStr = item.created_at ? new Date(item.created_at).toLocaleString('id-ID') : '—';
            tr.innerHTML = `
                <td style="padding: 12px 14px;" class="text-muted text-xs">${TimeStr}</td>
                <td style="padding: 12px 14px;" class="font-semibold">${item.device_name || '—'}</td>
                <td style="padding: 12px 14px;"><span class="badge-Severity ${badgeClass}">${item.severity || 'info'}</span></td>
                <td style="padding: 12px 14px;" class="text-muted text-xs">${item.message || '—'}</td>
                <td style="padding: 12px 14px;"><span class="text-success font-semibold text-xs">${item.status || 'Resolved'}</span></td>
            `;
        } else if (currentType === 'down_frequency') {
            const downCount = item.down_count || 0;
            const maxDown = Math.max(...activeReportItems.map(i => i.down_count || 0), 1);
            const barPct = Math.round((downCount / maxDown) * 100);
            const barColor = downCount >= maxDown ? '#ef4444' : (downCount > maxDown * 0.5 ? '#f97316' : '#3b82f6');

            let lastDownStr = '—';
            if (item.last_down && item.last_down !== '0001-01-01 00:00:00 +0000 UTC' && item.last_down !== '') {
                try {
                    const d = new Date(item.last_down);
                    if (!isNaN(d.getTime())) {
                        lastDownStr = d.toLocaleString('id-ID', {
                            day: '2-digit', month: 'short', year: 'numeric',
                            hour: '2-digit', minute: '2-digit', hour12: false
                        }) + ' WIB';
                    }
                } catch (_) {}
            }

            const rank = globalIdx + 1;
            const rankBadge = rank <= 3
                ? `<span style="background: ${rank === 1 ? '#ef4444' : rank === 2 ? '#f97316' : '#eab308'}; color: #fff; padding: 2px 8px; border-radius: 6px; font-weight: 800; font-size: 11px;">#${rank}</span>`
                : `<span style="background: rgba(255,255,255,0.08); color: var(--text-muted); padding: 2px 8px; border-radius: 6px; font-weight: 600; font-size: 11px;">#${rank}</span>`;

            tr.innerHTML = `
                <td style="padding: 12px 14px;">${rankBadge}</td>
                <td style="padding: 12px 14px;" class="font-semibold">${item.name || '—'}</td>
                <td style="padding: 12px 14px;">${copyIpBtn}</td>
                <td style="padding: 12px 14px;"><span class="badge-outline" style="font-size:10px;">${item.type || '—'}</span></td>
                <td style="padding: 12px 14px;">
                    <div style="display: flex; align-items: center; gap: 8px; width: 120px;">
                        <div style="flex: 1; height: 6px; background: rgba(255,255,255,0.1); border-radius: 3px; overflow: hidden;">
                            <div style="width: ${barPct}%; height: 100%; background: ${barColor}; border-radius: 3px;"></div>
                        </div>
                        <span style="font-size: 11px; font-weight: 700; color: ${barColor};">${downCount}x</span>
                    </div>
                </td>
                <td style="padding: 12px 14px;" class="text-muted text-xs">${lastDownStr}</td>
                <td style="padding: 12px 14px;">${liveBadge}</td>
            `;
        } else if (currentType === 'traffic_monthly' || currentType === 'bandwidth_top') {
            tr.innerHTML = `
                <td style="padding: 12px 14px;" class="font-semibold">${item.name || '—'}</td>
                <td style="padding: 12px 14px;">${copyIpBtn}</td>
                <td style="padding: 12px 14px;" class="text-info font-semibold">${item.avg_traffic || '0 bps'}</td>
                <td style="padding: 12px 14px;" class="text-warning font-semibold">${item.max_traffic || '0 bps'}</td>
                <td style="padding: 12px 14px;">${liveBadge}</td>
            `;
        } else if (currentType === 'downTime_monthly') {
            tr.innerHTML = `
                <td style="padding: 12px 14px;" class="font-semibold">${item.name || '—'}</td>
                <td style="padding: 12px 14px;">${copyIpBtn}</td>
                <td style="padding: 12px 14px;" class="text-danger font-semibold">${item.kali_down || 0}x</td>
                <td style="padding: 12px 14px;" class="text-warning font-semibold">${item.durasi_down || '—'}</td>
                <td style="padding: 12px 14px;">${liveBadge}</td>
            `;
        } else if (currentType === 'vpn_monthly') {
            const TimeStr = item.connected_at ? new Date(item.connected_at).toLocaleString('id-ID') : '—';
            const cleanDuration = formatVpnDuration(item.upTime || '—');
            tr.innerHTML = `
                <td style="padding: 12px 14px;" class="text-muted text-xs">${TimeStr}</td>
                <td style="padding: 12px 14px;" class="text-info font-semibold">${item.name || '—'}</td>
                <td style="padding: 12px 14px;"><span class="badge-outline" style="font-size:10px;">${item.service || '—'}</span></td>
                <td style="padding: 12px 14px;">${copyIpBtn}</td>
                <td style="padding: 12px 14px;" class="text-success font-medium">${cleanDuration}</td>
            `;
        } else if (currentType === 'network_quality') {
            tr.innerHTML = `
                <td style="padding: 12px 14px;" class="font-semibold">${item.name || '—'}</td>
                <td style="padding: 12px 14px;">${copyIpBtn}</td>
                <td style="padding: 12px 14px;" class="text-warning font-medium">${item.latency || '0 ms'}</td>
                <td style="padding: 12px 14px;" class="text-danger font-medium">${item.packet_loss || '0%'}</td>
                <td style="padding: 12px 14px;">${liveBadge}</td>
            `;
        }
        
        fragment.appendChild(tr);
    });
    tbody.appendChild(fragment);

    renderPaginationControls(totalItems);
}

function renderPaginationControls(totalItems) {
    const infoEl = document.getElementById('report-pagination-info');
    const btnContainer = document.getElementById('report-pagination-buttons');
    if (!infoEl || !btnContainer) return;

    if (totalItems === 0) {
        infoEl.textContent = 'Menampilkan 0-0 dari 0 data';
        btnContainer.innerHTML = '';
        return;
    }

    const totalPages = Math.ceil(totalItems / pageSize) || 1;
    const startNum = (currentPage - 1) * pageSize + 1;
    const endNum = Math.min(currentPage * pageSize, totalItems);

    infoEl.textContent = `Menampilkan ${startNum}-${endNum} dari ${totalItems} data`;

    let buttonsHtml = '';
    
    // Prev Button
    buttonsHtml += `<button onclick="window.goToReportPage(${currentPage - 1})" ${currentPage === 1 ? 'disabled' : ''} style="background: rgba(255,255,255,0.05); border: 1px solid var(--border-color); color: var(--text-main); padding: 4px 10px; border-radius: 6px; font-size: 12px; cursor: pointer; opacity: ${currentPage === 1 ? 0.5 : 1};"><i class="fas fa-chevron-left"></i> Prev</button>`;

    // Page number buttons
    for (let p = 1; p <= totalPages; p++) {
        if (p === 1 || p === totalPages || (p >= currentPage - 1 && p <= currentPage + 1)) {
            const isActive = p === currentPage;
            buttonsHtml += `<button onclick="window.goToReportPage(${p})" style="background: ${isActive ? 'var(--accent-blue)' : 'rgba(255,255,255,0.05)'}; border: 1px solid ${isActive ? 'var(--accent-blue)' : 'var(--border-color)'}; color: #fff; padding: 4px 10px; border-radius: 6px; font-size: 12px; font-weight: ${isActive ? '700' : '400'}; cursor: pointer;">${p}</button>`;
        } else if (p === currentPage - 2 || p === currentPage + 2) {
            buttonsHtml += `<span style="color: var(--text-muted); font-size: 11px;">...</span>`;
        }
    }

    // Next Button
    buttonsHtml += `<button onclick="window.goToReportPage(${currentPage + 1})" ${currentPage === totalPages ? 'disabled' : ''} style="background: rgba(255,255,255,0.05); border: 1px solid var(--border-color); color: var(--text-main); padding: 4px 10px; border-radius: 6px; font-size: 12px; cursor: pointer; opacity: ${currentPage === totalPages ? 0.5 : 1};">Next <i class="fas fa-chevron-right"></i></button>`;

    btnContainer.innerHTML = buttonsHtml;
}

function goToReportPage(page) {
    const totalPages = Math.ceil(activeReportItems.length / pageSize) || 1;
    if (page < 1 || page > totalPages) return;
    currentPage = page;
    renderCurrentTablePage();
}

function renderReportChart(type, items, summary) {
    const chartCard = document.getElementById('report-chart-card');
    const canvas = document.getElementById('report-chart-canvas');
    const chartTitle = document.getElementById('report-chart-title');
    const chartSubtitle = document.getElementById('report-chart-subtitle');

    if (!chartCard || !canvas) return;

    const ChartLib = window.Chart;
    if (!ChartLib) {
        chartCard.style.display = 'none';
        return;
    }

    if (currentChart) {
        currentChart.destroy();
        currentChart = null;
    }

    if (!items || items.length === 0) {
        chartCard.style.display = 'none';
        return;
    }

    chartCard.style.display = 'block';
    const ctx = canvas.getContext('2d');

    let labels = [];
    let datasets = [];
    let chartType = 'bar';

    if (type === 'availability' || type === 'sla') {
        chartTitle.textContent = 'Device Availability / SLA Chart (%)';
        chartSubtitle.textContent = 'Availability Rate (%) per Network Device';
        labels = items.slice(0, 15).map(i => i.name || i.ip || 'Unknown');
        const dataVals = items.slice(0, 15).map(i => parseFloat(i.availability_pct) || 0);
        datasets = [{
            label: 'Availability (%)',
            data: dataVals,
            backgroundColor: dataVals.map(v => v >= 95 ? 'rgba(16, 185, 129, 0.7)' : (v >= 80 ? 'rgba(245, 158, 11, 0.7)' : 'rgba(239, 68, 68, 0.7)')),
            borderColor: dataVals.map(v => v >= 95 ? '#10b981' : (v >= 80 ? '#f59e0b' : '#ef4444')),
            borderWidth: 1,
            borderRadius: 6
        }];
    } else if (type === 'performance') {
        chartTitle.textContent = 'Resource Performance Chart (CPU & RAM)';
        chartSubtitle.textContent = 'Average CPU (%) and RAM (%) Usage';
        labels = items.slice(0, 15).map(i => i.name || i.ip || 'Unknown');
        datasets = [
            {
                label: 'Avg CPU (%)',
                data: items.slice(0, 15).map(i => parseFloat(i.avg_cpu) || 0),
                backgroundColor: 'rgba(59, 130, 246, 0.7)',
                borderColor: '#3b82f6',
                borderRadius: 4
            },
            {
                label: 'Avg RAM (%)',
                data: items.slice(0, 15).map(i => parseFloat(i.avg_ram) || 0),
                backgroundColor: 'rgba(168, 85, 247, 0.7)',
                borderColor: '#a855f7',
                borderRadius: 4
            }
        ];
    } else if (type === 'down_frequency' || type === 'downTime_monthly') {
        chartTitle.textContent = 'Device Down Frequency & Downtime';
        chartSubtitle.textContent = 'Number of outage incidents per device';
        labels = items.slice(0, 15).map(i => i.name || i.ip || 'Unknown');
        datasets = [{
            label: 'Down Frequency (times)',
            data: items.slice(0, 15).map(i => i.down_count || i.kali_down || 0),
            backgroundColor: 'rgba(239, 68, 68, 0.7)',
            borderColor: '#ef4444',
            borderRadius: 6
        }];
    } else if (type === 'vpn_monthly') {
        chartTitle.textContent = 'VPN User Connection History';
        chartSubtitle.textContent = 'Estimated VPN User Connection Duration (Hours/Day)';
        labels = items.slice(0, 15).map(i => i.name || i.Username || 'User');
        const durationVals = items.slice(0, 15).map(i => {
            const u = i.upTime || '';
            let hours = 0;
            const wMatch = u.match(/([0-9]+)w/);
            const dMatch = u.match(/([0-9]+)d/);
            const hMatch = u.match(/([0-9]+)h/);
            if (wMatch) hours += parseInt(wMatch[1], 10) * 168;
            if (dMatch) hours += parseInt(dMatch[1], 10) * 24;
            if (hMatch) hours += parseInt(hMatch[1], 10);
            return hours || 1;
        });
        datasets = [{
            label: 'Connected Duration (Hours)',
            data: durationVals,
            backgroundColor: 'rgba(6, 182, 212, 0.7)',
            borderColor: '#06b6d4',
            borderRadius: 6
        }];
    } else if (type === 'traffic_monthly' || type === 'bandwidth_top') {
        chartTitle.textContent = 'Traffic & Bandwidth Visualization';
        chartSubtitle.textContent = 'Estimated Device Bandwidth Usage (Mbps)';
        labels = items.slice(0, 15).map(i => i.name || i.ip || 'Unknown');
        datasets = [{
            label: 'Avg Traffic',
            data: items.slice(0, 15).map(i => {
                const tr = i.avg_traffic || '0';
                return parseFloat(tr.replace(/[^0-9.]/g, '')) || 0;
            }),
            backgroundColor: 'rgba(139, 92, 246, 0.7)',
            borderColor: '#8b5cf6',
            borderRadius: 6
        }];
    } else if (type === 'network_quality') {
        chartTitle.textContent = 'Network Quality (Latency ms)';
        chartSubtitle.textContent = 'Average Latency per Device';
        labels = items.slice(0, 15).map(i => i.name || i.ip || 'Unknown');
        datasets = [{
            label: 'Latency (ms)',
            data: items.slice(0, 15).map(i => parseFloat(i.latency) || 0),
            backgroundColor: 'rgba(245, 158, 11, 0.7)',
            borderColor: '#f59e0b',
            borderRadius: 6
        }];
    } else if (type === 'incidents') {
        chartType = 'doughnut';
        chartTitle.textContent = 'Incident Severity Distribution';
        chartSubtitle.textContent = 'Alert Severity Proportion';
        const critCount = items.filter(i => i.severity === 'critical').length;
        const warnCount = items.filter(i => i.severity === 'warning').length;
        const infoCount = items.filter(i => i.severity === 'info' || !i.severity).length;
        labels = ['Critical', 'Warning', 'Info'];
        datasets = [{
            data: [critCount, warnCount, infoCount],
            backgroundColor: ['#ef4444', '#f59e0b', '#3b82f6'],
            borderWidth: 0
        }];
    } else {
        chartCard.style.display = 'none';
        return;
    }

    currentChart = new ChartLib(ctx, {
        type: chartType,
        data: { labels, datasets },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { labels: { color: '#94a3b8', font: { size: 11 } } },
                tooltip: { backgroundColor: '#1e293b', titleColor: '#f8fafc', bodyColor: '#cbd5e1' }
            },
            scales: chartType === 'doughnut' ? {} : {
                x: { ticks: { color: '#94a3b8', font: { size: 10 } }, grid: { color: 'rgba(255,255,255,0.05)' } },
                y: { ticks: { color: '#94a3b8', font: { size: 10 } }, grid: { color: 'rgba(255,255,255,0.05)' }, beginAtZero: true }
            }
        }
    });
}

function copyReportIp(ip) {
    if (!ip) return;
    navigator.clipboard.writeText(ip).then(() => {
        if (window.showToast) {
            window.showToast(`IP ${ip} berhasil disalin!`, 'success');
        } else {
            alert(`IP ${ip} berhasil disalin!`);
        }
    }).catch(err => {
        console.error('Failed to copy IP:', err);
    });
}

function exportCSV() {
    if (!currentRawData || !currentRawData.items || currentRawData.items.length === 0) {
        if (window.showToast) window.showToast("Tidak ada data untuk diekspor", 'error');
        return;
    }

    let csvContent = "\uFEFF"; // Add UTF-8 BOM for Excel compatibility
    
    // Add Metadata header
    const typeLabel = document.getElementById('report-type')?.selectedOptions[0]?.text || currentType;
    csvContent += `LAPORAN: ${typeLabel.toUpperCase()}\n`;
    csvContent += `DICETAK PADA: ${new Date().toLocaleString('id-ID')}\n\n`;

    // Add Summary header if present
    if (currentRawData.summary) {
        csvContent += "RINGKASAN METRIK:\n";
        for (const [key, val] of Object.entries(currentRawData.summary)) {
            csvContent += `"${key}","${val}"\n`;
        }
        csvContent += "\n";
    }

    // Export raw JSON items cleanly
    const items = activeReportItems.length > 0 ? activeReportItems : currentRawData.items;
    if (items.length > 0) {
        csvContent += "DATA LAPORAN:\n";
        const keys = Object.keys(items[0]);
        csvContent += keys.map(k => `"${k.toUpperCase()}"`).join(",") + "\n";

        items.forEach(item => {
            const row = keys.map(k => {
                let val = item[k] !== undefined && item[k] !== null ? String(item[k]) : '';
                val = val.replace(/"/g, '""');
                return `"${val}"`;
            });
            csvContent += row.join(",") + "\n";
        });
    }

    // Download file
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement("a");
    const date = new Date().toISOString().split('T')[0];
    
    link.href = URL.createObjectURL(blob);
    link.setAttribute("download", `Reports_${currentType}_${date}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
}

function printReport() {
    const typeLabels = {
        'sla': 'Network Device Availability & SLA Report',
        'availability': 'Network Device Availability & SLA Report',
        'network_quality': 'Network Quality Report (Average Latency & Packet Loss)',
        'traffic_monthly': 'Device Traffic & Bandwidth Report (Last 30 Days)',
        'downTime_monthly': 'Device Downtime Duration Report (Last 30 Days)',
        'down_frequency': 'Device Down Frequency Report (Top Disruption)',
        'performance': 'Device Resource Performance Report (CPU & RAM)',
        'vpn_monthly': 'VPN User Connection History Report (Last 30 Days)',
        'incidents': 'Device Incidents & Alert Logs Report'
    };
    const rangeLabels = {
        'today': 'Today (' + new Date().toLocaleDateString('en-US', { day: 'numeric', month: 'short', year: 'numeric' }) + ')',
        '7days': 'Last 7 Days',
        '30days': 'Last 30 Days',
        'custom': 'Custom Range (' + (document.getElementById('report-start-date')?.value || '—') + ' to ' + (document.getElementById('report-end-date')?.value || '—') + ')'
    };

    const typeEl = document.getElementById('report-type');
    const rangeEl = document.getElementById('report-range');
    const typeVal = typeEl ? typeEl.value : 'sla';
    const rangeVal = rangeEl ? rangeEl.value : '7days';

    const titleEl = document.getElementById('print-report-title');
    const periodEl = document.getElementById('print-report-period');
    const generatedEl = document.getElementById('print-report-generated');

    if (titleEl) titleEl.textContent = typeLabels[typeVal] || 'Reports Jaringan';
    if (periodEl) periodEl.textContent = 'Periode: ' + (rangeLabels[rangeVal] || rangeVal);
    if (generatedEl) generatedEl.textContent = 'Dicetak pada: ' + new Date().toLocaleString('id-ID', {
        day: 'numeric', month: 'long', year: 'numeric',
        hour: '2-digit', minute: '2-digit'
    }) + ' WIB';

    window.print();
}

window.printReport = printReport;
