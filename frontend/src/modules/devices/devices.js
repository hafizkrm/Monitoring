// Devices Module
// Network Monitoring System - Device Inventory Logic

import { isOnline, escapeHtml, renderPagination, formatUpTime, getDeviceStatusMeta } from '../../shared/utils/helpers.js';
import { state } from '../../core/state.js';
import { showToast } from '../../shared/components/toast.js';
import { metricsStore } from '../../core/state/store.js';
import { initDeviceDetail, openDeviceSidebar } from './deviceDetail.js';

const API_ENDPOINTS = {
    INVENTORY: '/api/inventory',
    DEVICES_HAPUS: '/api/devices/delete',
    DEVICES_UPDATE: '/api/devices/update'
};

// Initialize devices module
export function initDevices() {
    setupDeviceEventListeners();
    initDeviceDetail();
    bindAddDeviceModal();
    // Expose functions to window for router.js calls and inline HTML handlers
    window.fetchDevicesTable = fetchDevicesTable;
    window.DeleteDevice = DeleteDevice;
    window.EditDevice = EditDevice;
    window.changeDevicesPage = changeDevicesPage;
    window.viewDeviceDetails = openDeviceSidebar;
    window.openDeviceSidebar = openDeviceSidebar;
    window.handleDeviceSort = handleDeviceSort;
    window.exportDevicesCSV = exportDevicesCSV;
}

// Bind Tambah Perangkat Modal Form & Events
function bindAddDeviceModal() {
    const modal = document.getElementById('add-device-modal');
    const form = document.getElementById('add-device-form');
    const closeBtn = document.getElementById('close-modal');
    const CancelBtn = document.getElementById('Cancel-modal');

    if (closeBtn) closeBtn.onclick = () => window.closeAddDeviceModal();
    if (CancelBtn) CancelBtn.onclick = () => window.closeAddDeviceModal();

    if (modal) {
        modal.onclick = (e) => {
            if (e.target === modal) window.closeAddDeviceModal();
        };
    }

    if (form && !form.hasAttribute('data-bound')) {
        form.setAttribute('data-bound', 'true');
        form.onsubmit = async (e) => {
            e.preventDefault();
            const submitBtn = form.querySelector('button[type="submit"]');
            const nameInput = document.getElementById('device-name');
            const categoryInput = document.getElementById('device-category');
            const ipInput = document.getElementById('device-ip');

            const name = nameInput ? nameInput.value.trim() : '';
            const category = categoryInput ? categoryInput.value : '';
            const ip = ipInput ? ipInput.value.trim() : '';

            // Input Validation
            if (!name) {
                showToast('Nama perangkat wajib diisi', 'warning');
                return;
            }
            if (!category) {
                showToast('Pilih kategori perangkat', 'warning');
                return;
            }
            if (!ip) {
                showToast('IP Address wajib diisi', 'warning');
                return;
            }

            // IPv4 Format Validation
            const ipRegex = /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
            if (!ipRegex.test(ip)) {
                showToast('Format IP Address tidak valid! Contoh: 192.168.1.1', 'error');
                return;
            }

            // Lock Submit Button with Spinner
            if (submitBtn) {
                submitBtn.disabled = true;
                submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Menyimpan...';
            }

            try {
                const res = await fetch('/api/devices', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name: name, type: category, ip: ip })
                });

                if (res.ok) {
                    const data = await res.json().catch(() => ({}));
                    let msg = `Perangkat ${name} (${ip}) berhasil ditambahkan!`;
                    if (data.vendor && data.vendor !== 'Generic') {
                        msg = `Perangkat ${name} (${ip}) terdeteksi sebagai vendor ${data.vendor} dan berhasil ditambahkan!`;
                    }
                    showToast(msg, 'success');
                    
                    // Reset filter, sorting & pagination so new device is immediately visible on top
                    currentFilterType = 'Semua';
                    currentSearchTerm = '';
                    currentSortCol = null;
                    updateSortIcons();
                    state.pagination.devices.current = 1;
                    
                    const searchInput = document.getElementById('search-devices');
                    if (searchInput) searchInput.value = '';
                    const clearBtn = document.getElementById('clear-search-devices');
                    if (clearBtn) clearBtn.style.display = 'none';

                    const filterBar = document.getElementById('mon-device-filter-bar');
                    if (filterBar) {
                        filterBar.querySelectorAll('.mon-filter-chip').forEach(b => {
                            b.classList.toggle('active', b.innerText.trim() === 'Semua');
                        });
                    }

                    if (window.closeAddDeviceModal) window.closeAddDeviceModal();
                    fetchDevicesTable();
                } else {
                    const errText = await res.text();
                    showToast(`Gagal menambahkan: ${errText}`, 'error');
                }
            } catch (err) {
                console.error('Tambah Perangkat Error:', err);
                showToast('Error koneksi ke server', 'error');
            } finally {
                if (submitBtn) {
                    submitBtn.disabled = false;
                    submitBtn.innerHTML = 'Save';
                }
            }
        };
    }
}

// Local state for filters & sorting
let currentFilterType = 'Semua';
let currentSearchTerm = '';
let currentSortCol = null;
let currentSortDir = 'asc';

// Setup event listeners for devices view
function setupDeviceEventListeners() {
    bindAddDeviceModal();
    const searchInput = document.getElementById('search-devices');
    const clearBtn = document.getElementById('clear-search-devices');

    if (searchInput) {
        let debounceTimer;
        searchInput.addEventListener('input', (e) => {
            const val = e.target.value;
            if (clearBtn) clearBtn.style.display = val.trim() ? 'block' : 'none';
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(() => {
                currentSearchTerm = val;
                state.pagination.devices.current = 1;
                fetchDevicesTable();
            }, 300);
        });
    }

    if (clearBtn && searchInput) {
        clearBtn.addEventListener('click', () => {
            searchInput.value = '';
            clearBtn.style.display = 'none';
            currentSearchTerm = '';
            state.pagination.devices.current = 1;
            fetchDevicesTable();
        });
    }

    // Badge click listeners
    const applyBadgeFilter = (filterText) => {
        if (searchInput) {
            searchInput.value = filterText;
            if (clearBtn) clearBtn.style.display = filterText ? 'block' : 'none';
            currentSearchTerm = filterText;
            state.pagination.devices.current = 1;
            fetchDevicesTable();
        }
    };

    const badgeTotal = document.getElementById('badge-filter-total');
    if (badgeTotal) badgeTotal.addEventListener('click', () => applyBadgeFilter(''));

    const badgeOnline = document.getElementById('badge-filter-online');
    if (badgeOnline) badgeOnline.addEventListener('click', () => applyBadgeFilter('Online'));

    const badgeOffline = document.getElementById('badge-filter-offline');
    if (badgeOffline) badgeOffline.addEventListener('click', () => applyBadgeFilter('Offline'));
}

window.handleDeviceSort = function(colName) {
    if (currentSortCol === colName) {
        currentSortDir = currentSortDir === 'asc' ? 'desc' : 'asc';
    } else {
        currentSortCol = colName;
        currentSortDir = 'asc';
    }
    updateSortIcons();
    fetchDevicesTable();
};

function updateSortIcons() {
    ['name', 'device_type', 'ip_address', 'upTime', 'status'].forEach(col => {
        const icon = document.getElementById(`sort-icon-${col}`);
        if (!icon) return;
        if (col === currentSortCol) {
            icon.className = `fas fa-sort-${currentSortDir === 'asc' ? 'up' : 'down'} sort-icon`;
            icon.style.opacity = '1';
            icon.style.color = '#60a5fa';
        } else {
            icon.className = 'fas fa-sort sort-icon';
            icon.style.opacity = '0.4';
            icon.style.color = 'inherit';
        }
    });
}

window.exportDevicesCSV = async function() {
    try {
        showToast('Menyiapkan file CSV...', 'info');
        const cacheBuster = '?t=' + new Date().getTime();
        let typeQuery = currentFilterType === 'Semua' ? '' : currentFilterType;
        if (typeQuery === 'Access Point') typeQuery = 'access_point';
        else if (typeQuery) typeQuery = typeQuery.toLowerCase();

        const resInv = await fetch(`${API_ENDPOINTS.INVENTORY}?page=1&limit=10000&search=${encodeURIComponent(currentSearchTerm)}&type=${encodeURIComponent(typeQuery)}&t=${new Date().getTime()}`);
        const invResponse = await resInv.json();
        
        // Phase 9: Use realTime WebSocket state instead of duplicate HTTP polling
        const metrics = metricsStore.getState().fullMetrics || [];
        const devices = invResponse.data || [];

        if (devices.length === 0) {
            showToast('Tidak ada data perangkat untuk diexport', 'warning');
            return;
        }

        let csv = 'Nama Perangkat,Tipe,IP Address,Status,UpTime (detik)\n';
        devices.forEach(d => {
            const m = Array.isArray(metrics) ? metrics.find(x => (x.ip_address || x.ip) === d.ip_address) : null;
            const status = m ? m.status : (d.status || 'UNKNOWN');
            const upTime = (m && m.uptime) ? m.uptime : 0;
            const cleanName = `"${(d.name || '').replace(/"/g, '""')}"`;
            const cleanType = `"${(d.device_type || '').replace(/"/g, '""')}"`;
            csv += `${cleanName},${cleanType},${d.ip_address},${status},${upTime}\n`;
        });

        const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.setAttribute('href', url);
        link.setAttribute('download', `inventory_perangkat_${new Date().toISOString().slice(0,10)}.csv`);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        showToast('Berhasil mengunduh data CSV perangkat');
    } catch (e) {
        console.error('CSV Export Error:', e);
        showToast('Gagal mengunduh CSV', 'error');
    }
};

window.monFilterTable = function(type, btn) {
    if (btn) {
        const container = btn.closest('.mon-filter-bar');
        if (container) {
            container.querySelectorAll('.mon-filter-chip').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
        }
    }
    currentFilterType = type;
    state.pagination.devices.current = 1;
    fetchDevicesTable();
};

// Fetch and render devices table
export async function fetchDevicesTable() {
    const tbody = document.getElementById('devices-module-table-body');
    if (!tbody) return;

    if (window._pendingDeviceSearch !== undefined) {
        currentSearchTerm = window._pendingDeviceSearch;
        const searchInput = document.getElementById('search-devices');
        if (searchInput) searchInput.value = currentSearchTerm;
        const clearBtn = document.getElementById('clear-search-devices');
        if (clearBtn) clearBtn.style.display = currentSearchTerm ? 'block' : 'none';
        window._pendingDeviceSearch = undefined;
    }

    const refreshIcon = document.getElementById('refresh-devices-icon');
    if (refreshIcon) refreshIcon.classList.add('fa-spin');

    try {
        const cacheBuster = '?t=' + new Date().getTime();
        let typeQuery = currentFilterType === 'Semua' ? '' : currentFilterType;
        
        // Map UI labels to backend database enum values
        if (typeQuery === 'Access Point') {
            typeQuery = 'access_point';
        } else if (typeQuery) {
            typeQuery = typeQuery.toLowerCase();
        }

        const resInv = await fetch(`${API_ENDPOINTS.INVENTORY}?page=${state.pagination.devices.current}&limit=${state.pagination.devices.limit}&search=${encodeURIComponent(currentSearchTerm)}&type=${encodeURIComponent(typeQuery)}&t=${new Date().getTime()}`);
        const invResponse = await resInv.json();
        
        // Phase 9: Use realTime WebSocket state instead of duplicate HTTP polling
        const metrics = metricsStore.getState().fullMetrics || [];

        let paginated = invResponse.data || [];
        const totalItems = invResponse.total || 0;
        state.pagination.devices.total = totalItems;

        // Update top inventory summary stats with GLOBAL totals so KPI math is 100% accurate
        const elTotalCount = document.getElementById('devices-total-count');
        const elOnlineCount = document.getElementById('devices-online-count');
        const elOfflineCount = document.getElementById('devices-offline-count');
        const badgeTableCount = document.getElementById('device-table-count-badge');
        
        if (Array.isArray(metrics) && metrics.length > 0) {
            const globalTotal = Math.max(metrics.length, totalItems);
            const onCount = metrics.filter(m => isOnline(m.status)).length;
            const offCount = Math.max(0, globalTotal - onCount);
            
            if (elTotalCount) elTotalCount.innerText = globalTotal;
            if (elOnlineCount) elOnlineCount.innerText = onCount;
            if (elOfflineCount) elOfflineCount.innerText = offCount;

            // Update filter chip count badges dynamically
            const counts = { semua: globalTotal, router: 0, radio: 0, ap: 0, switch: 0, server: 0, firewall: 0 };
            metrics.forEach(m => {
                const t = (m.device_type || m.type || '').toLowerCase();
                if (t.includes('router')) counts.router++;
                else if (t.includes('radio')) counts.radio++;
                else if (t.includes('access point') || t.includes('access_point') || t === 'ap') counts.ap++;
                else if (t.includes('switch')) counts.switch++;
                else if (t.includes('server')) counts.server++;
                else if (t.includes('firewall')) counts.firewall++;
            });
            const setChipCount = (id, val) => {
                const el = document.getElementById(id);
                if (el) el.innerText = val > 0 ? val : '0';
            };
            setChipCount('count-chip-semua', counts.semua);
            setChipCount('count-chip-router', counts.router);
            setChipCount('count-chip-radio', counts.radio);
            setChipCount('count-chip-ap', counts.ap);
            setChipCount('count-chip-switch', counts.switch);
            setChipCount('count-chip-server', counts.server);
            setChipCount('count-chip-firewall', counts.firewall);
        } else if (elTotalCount) {
            elTotalCount.innerText = totalItems;
        }

        if (badgeTableCount) {
            if (currentFilterType !== 'Semua' || currentSearchTerm) {
                badgeTableCount.style.display = 'inline-block';
                badgeTableCount.innerText = `${totalItems} ${currentFilterType !== 'Semua' ? currentFilterType : ''} ditemukan`.trim();
            } else {
                badgeTableCount.style.display = 'none';
            }
        }

        // Apply sorting if a sort column is active
        if (currentSortCol && paginated.length > 1) {
            paginated.sort((a, b) => {
                let valA = a[currentSortCol] || '';
                let valB = b[currentSortCol] || '';
                
                if (currentSortCol === 'status') {
                    const mA = metrics.find(x => (x.ip_address || x.ip) === a.ip_address);
                    const mB = metrics.find(x => (x.ip_address || x.ip) === b.ip_address);
                    valA = isOnline(mA ? mA.status : a.status) ? 1 : 0;
                    valB = isOnline(mB ? mB.status : b.status) ? 1 : 0;
                } else if (currentSortCol === 'upTime') {
                    const mA = metrics.find(x => (x.ip_address || x.ip) === a.ip_address);
                    const mB = metrics.find(x => (x.ip_address || x.ip) === b.ip_address);
                    valA = (mA && mA.uptime) ? mA.uptime : 0;
                    valB = (mB && mB.uptime) ? mB.uptime : 0;
                } else if (currentSortCol === 'ip_address') {
                    const numA = (a.ip_address || '').split('.').reduce((acc, octet) => (acc << 8) + (parseInt(octet, 10) || 0), 0);
                    const numB = (b.ip_address || '').split('.').reduce((acc, octet) => (acc << 8) + (parseInt(octet, 10) || 0), 0);
                    return currentSortDir === 'asc' ? numA - numB : numB - numA;
                }

                if (typeof valA === 'string') {
                    return currentSortDir === 'asc' 
                        ? valA.localeCompare(valB, undefined, { numeric: true, sensitivity: 'base' })
                        : valB.localeCompare(valA, undefined, { numeric: true, sensitivity: 'base' });
                }
                return currentSortDir === 'asc' ? valA - valB : valB - valA;
            });
        }

        tbody.innerHTML = '';
        if (paginated.length === 0) {
            tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; padding: 40px; color: var(--text-muted);"><i class="fas fa-inbox fa-2x" style="margin-bottom: 10px; opacity: 0.5;"></i><br>Tidak ada data perangkat</td></tr>`;
            renderPagination('devices-module-pagination', 0, state.pagination.devices.limit, 1, 'changeDevicesPage');
            return;
        }

        const fragment = document.createDocumentFragment();
        
        // Set up Event Delegation once
        if (!tbody.hasAttribute('data-delegated')) {
            tbody.addEventListener('click', (e) => {
                const btnDetail = e.target.closest('.btn-mini-action.detail');
                const btnEdit = e.target.closest('.btn-mini-action.edit');
                const btnDel = e.target.closest('.btn-mini-action.delete');
                const tr = e.target.closest('.clickable-row');
                
                if (btnDetail) {
                    const d = btnDetail.dataset;
                    if (window.openDeviceSidebar) {
                        window.openDeviceSidebar({ id: d.ip, name: d.name, ip: d.ip, type: d.type, status: 'up' });
                    } else {
                        viewDeviceDetails(d.ip, d.name);
                    }
                    return;
                }
                if (btnEdit) {
                    EditDevice(btnEdit.dataset.ip, btnEdit.dataset.name, btnEdit.dataset.type, btnEdit.dataset.parent);
                    return;
                }
                if (btnDel) {
                    DeleteDevice(btnDel.dataset.ip, btnDel.dataset.name);
                    return;
                }
                
                if (tr && !e.target.closest('.table-Action')) {
                    const data = tr.dataset;
                    if(window.openDeviceSidebar) {
                        window.openDeviceSidebar({
                            id: data.ip, name: data.name, ip: data.ip,
                            type: data.type, status: data.status
                        });
                    } else {
                        viewDeviceDetails(data.ip, data.name);
                    }
                }
            });
            tbody.setAttribute('data-delegated', 'true');
        }

        paginated.forEach((d, index) => {
            const liveMetric = metrics.find(m => m.ip === d.ip_address);
            const currentStatus = liveMetric ? liveMetric.status : (d.status || 'UNKNOWN');
            
            let upTimeText = '-';
            if (isOnline(currentStatus)) {
                if (liveMetric && liveMetric.uptime && liveMetric.uptime > 0) {
                    upTimeText = formatUpTime(liveMetric.uptime);
                } else {
                    upTimeText = '<span style="color: var(--accent-green); font-size: 11px; font-weight: 500; background: rgba(34, 197, 94, 0.1); padding: 2px 7px; border-radius: 4px; border: 1px solid rgba(34, 197, 94, 0.25); display: inline-flex; align-items: center; gap: 4px;"><i class="fas fa-check-circle" style="font-size: 10px;"></i>Active</span>';
                }
            } else {
                upTimeText = '<span style="color:var(--text-muted);">-</span>';
            }

            const row = document.createElement('tr');
            row.className = 'clickable-row fade-in';
            row.style.cursor = 'pointer';
            
            row.dataset.ip = d.ip_address;
            row.dataset.name = d.name;
            row.dataset.type = d.device_type;
            row.dataset.status = currentStatus;
            
            let typeBg = 'rgba(59, 130, 246, 0.12)';
            let typeColor = '#60a5fa';
            let typeIcon = 'fa-server';
            const tLow = (d.device_type || '').toLowerCase();
            
            if (tLow.includes('ruijie')) {
                typeBg = 'rgba(236, 72, 153, 0.15)'; typeColor = '#f472b6'; typeIcon = 'fa-wifi';
            } else if (tLow.includes('tplink') || tLow.includes('tp-link')) {
                typeBg = 'rgba(16, 185, 129, 0.15)'; typeColor = '#34d399'; typeIcon = 'fa-wifi';
            } else if (tLow.includes('router')) {
                typeBg = 'rgba(59, 130, 246, 0.15)'; typeColor = '#60a5fa'; typeIcon = 'fa-server';
            } else if (tLow.includes('switch')) {
                typeBg = 'rgba(139, 92, 246, 0.15)'; typeColor = '#c084fc'; typeIcon = 'fa-network-wired';
            } else if (tLow.includes('radio')) {
                typeBg = 'rgba(245, 158, 11, 0.15)'; typeColor = '#fbbf24'; typeIcon = 'fa-broadcast-tower';
            } else if (tLow.includes('access point') || tLow === 'ap' || tLow.includes('access_point')) {
                typeBg = 'rgba(6, 182, 212, 0.15)'; typeColor = '#22d3ee'; typeIcon = 'fa-wifi';
            } else if (tLow.includes('firewall')) {
                typeBg = 'rgba(239, 68, 68, 0.15)'; typeColor = '#f87171'; typeIcon = 'fa-shield-alt';
            } else if (tLow.includes('server')) {
                typeBg = 'rgba(34, 197, 94, 0.15)'; typeColor = '#4ade80'; typeIcon = 'fa-server';
            }

            const formattedTypeLabel = d.device_type ? d.device_type.replace(/_/g, ' ').toUpperCase() : 'ROUTER';
            const vendorSubText = (d.vendor && d.vendor !== 'Generic') ? d.vendor : (d.model || '');

            const StatusMeta = getDeviceStatusMeta(currentStatus);
            
            row.innerHTML = `
                <td>
                    <div style="display: flex; align-items: center; gap: 12px;">
                        <div style="width: 30px; height: 30px; border-radius: 8px; background: ${typeBg}; display: flex; align-items: center; justify-content: center; flex-shrink: 0; border: 1px solid ${typeColor}25;">
                            <i class="fas ${typeIcon}" style="color: ${typeColor}; font-size: 13px;"></i>
                        </div>
                        <div style="display: flex; flex-direction: column;">
                            <span style="font-weight: 600; font-size: 13px; color: var(--text-main); letter-spacing: 0.2px;">${escapeHtml(d.name)}</span>
                            ${vendorSubText ? `<span style="font-size: 10px; color: var(--text-muted); opacity: 0.85; margin-top: 1px; font-weight: 400;">${escapeHtml(vendorSubText)}</span>` : ''}
                        </div>
                    </div>
                </td>
                <td><span style="font-size: 10px; font-weight: 600; padding: 3px 8px; border-radius: 6px; background: ${typeBg}; color: ${typeColor}; border: 1px solid ${typeColor}30;">${formattedTypeLabel}</span></td>
                <td style="font-family: monospace; font-size: 12px; color: rgba(255,255,255,0.9); font-weight: 500;">${d.ip_address}</td>
                <td style="font-size: 12px; color: var(--text-muted);">${upTimeText}</td>
                <td>
                    <div style="display: flex; align-items: center; gap: 6px;">
                        <div class="${StatusMeta.blink ? 'led-blink' : ''}" style="width: 8px; height: 8px; border-radius: 50%; background: ${StatusMeta.color}; box-shadow: ${StatusMeta.blink ? `0 0 6px ${StatusMeta.color}` : 'none'};"></div>
                        <span style="font-size: 12px; font-weight: 500; color: ${StatusMeta.color};">${StatusMeta.label}</span>
                    </div>
                </td>
                <td style="text-align: right;">
                    <div class="table-actions">
                        <button class="btn-mini-action detail" aria-label="Detail device" data-ip="${d.ip_address}" data-name="${escapeHtml(d.name)}" data-type="${escapeHtml(d.device_type || 'Router')}" title="Detail Perangkat"><i class="fas fa-eye"></i></button>
                        <button class="btn-mini-action edit" aria-label="Edit device" data-ip="${d.ip_address}" data-name="${escapeHtml(d.name)}" data-type="${escapeHtml(d.device_type || 'router')}" data-parent="${escapeHtml(d.parent_ip || '')}" title="Edit"><i class="fas fa-edit"></i></button>
                        <button class="btn-mini-action delete" aria-label="Delete device" data-ip="${d.ip_address}" data-name="${escapeHtml(d.name)}" title="Delete"><i class="fas fa-trash"></i></button>
                    </div>
                </td>
            `;
            fragment.appendChild(row);
        });
        
        tbody.appendChild(fragment);

        renderPagination('devices-module-pagination', state.pagination.devices.total, state.pagination.devices.limit, state.pagination.devices.current, 'changeDevicesPage');
    } catch (e) {
        console.error('Table Fetch Error:', e);
        showToast('Gagal memuat data perangkat', 'error');
    } finally {
        if (refreshIcon) {
            setTimeout(() => refreshIcon.classList.remove('fa-spin'), 400);
        }
    }
}

// Delete device - Custom Modern Confirmation Modal
export function DeleteDevice(ip, name) {
    const safeName = name || ip;
    // Remove existing confirm modal if any
    const existing = document.getElementById('custom-confirm-modal');
    if (existing) existing.remove();

    const modal = document.createElement('div');
    modal.id = 'custom-confirm-modal';
    modal.className = 'modal-overlay modal-overlay-bg';
    modal.style.cssText = 'position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(11,15,25,0.85);backdrop-filter:blur(8px);z-index:9999;display:flex;align-items:center;justify-content:center;opacity:0;transition:opacity 0.2s ease;';

    modal.innerHTML = `
        <div class="modal-box" style="max-width:440px; background:#151c2c; border:1px solid rgba(239,68,68,0.3); border-radius:12px; padding:24px; box-shadow:0 20px 50px rgba(0,0,0,0.7); display:flex; flex-direction:column; gap:16px;">
            <div style="display:flex; align-items:center; gap:14px;">
                <div style="width:48px; height:48px; border-radius:12px; background:rgba(239,68,68,0.15); border:1px solid rgba(239,68,68,0.3); display:flex; align-items:center; justify-content:center; color:#ef4444; font-size:22px; flex-shrink:0;">
                    <i class="fas fa-exclamation-triangle"></i>
                </div>
                <div>
                    <h3 style="margin:0; font-size:16px; font-weight:700; color:#ffffff;">Delete Perangkat?</h3>
                    <span style="font-size:12px; color:#94a3b8; font-family:monospace;">IP: ${escapeHtml(ip)}</span>
                </div>
            </div>
            
            <p style="margin:0; font-size:13px; color:#cbd5e1; line-height:1.5;">
                Apakah Anda yakin ingin mengdelete perangkat <strong style="color:#ffffff;">"${escapeHtml(safeName)}"</strong>?
                Semua data metrik dan riwayat terkait perangkat ini akan didelete dari sistem.
            </p>

            <div style="display:flex; gap:10px; justify-content:flex-end; margin-top:8px;">
                <button type="button" id="confirm-Delete-Cancel" class="btn-secondary" style="background:rgba(255,255,255,0.08); border:1px solid rgba(255,255,255,0.15); color:#ffffff; padding:9px 16px; border-radius:8px; font-size:13px; font-weight:600; cursor:pointer;">Cancel</button>
                <button type="button" id="confirm-Delete-btn" class="btn-danger" style="background:#ef4444; border:none; color:#ffffff; padding:9px 18px; border-radius:8px; font-size:13px; font-weight:600; cursor:pointer; display:inline-flex; align-items:center; gap:6px; box-shadow:0 4px 14px rgba(239,68,68,0.4); transition:all 0.2s ease;">
                    <i class="fas fa-trash"></i> Delete Perangkat
                </button>
            </div>
        </div>
    `;

    document.body.appendChild(modal);
    setTimeout(() => { modal.style.opacity = '1'; }, 10);

    const btnCancel = document.getElementById('confirm-Delete-Cancel');
    const btnConfirm = document.getElementById('confirm-Delete-btn');

    const closeModal = () => {
        modal.style.opacity = '0';
        setTimeout(() => modal.remove(), 200);
    };

    btnCancel.onclick = closeModal;
    modal.onclick = (e) => { if (e.target === modal) closeModal(); };

    btnConfirm.onclick = async () => {
        btnConfirm.disabled = true;
        btnConfirm.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Mengdelete...';

        try {
            const res = await fetch(API_ENDPOINTS.DEVICES_HAPUS, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ip })
            });

            if (res.ok) {
                showToast(`Perangkat "${safeName}" (${ip}) berhasil didelete!`, 'success');
                closeModal();
                

                const currentMetrics = metricsStore.getState().fullMetrics;
                if (Array.isArray(currentMetrics)) {
                    metricsStore.setState({
                        fullMetrics: currentMetrics.filter(x => x.ip_address !== ip && x.ip !== ip)
                    });
                }
                
                // Refresh table & state
                if (window.fetchDevicesTable) {
                    window.fetchDevicesTable();
                }
            } else {
                const errText = await res.text();
                showToast(`Gagal mengdelete: ${errText}`, 'error');
                btnConfirm.disabled = false;
                btnConfirm.innerHTML = '<i class="fas fa-trash"></i> Delete Perangkat';
            }
        } catch (e) {
            console.error('Delete Error:', e);
            showToast('Error koneksi saat mengdelete perangkat', 'error');
            btnConfirm.disabled = false;
            btnConfirm.innerHTML = '<i class="fas fa-trash"></i> Delete Perangkat';
        }
    };
}

// Edit device
function injectEditModal() {
    const existing = document.getElementById('Edit-device-modal');
    if (existing) {
        existing.remove();
    }
    const html = `
    <div class="modal-overlay modal-overlay-bg" id="Edit-device-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(11, 15, 25, 0.85); backdrop-filter: blur(8px); z-index: 9999; align-items: center; justify-content: center;">
        <div class="modal-box" style="max-width: 480px; width: 90%; background: #151c2c; border: 1px solid rgba(59, 130, 246, 0.3); border-radius: 12px; padding: 24px; box-shadow: 0 20px 50px rgba(0,0,0,0.8); color: #fff;">
            <div class="modal-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; border-bottom: 1px solid rgba(255,255,255,0.08); padding-bottom: 14px;">
                <h2 style="margin: 0; font-size: 18px; font-weight: 700; color: #ffffff; display: flex; align-items: center; gap: 10px;">
                    <i class="fas fa-edit" style="color: #3b82f6;"></i> Edit Perangkat
                </h2>
                <button onclick="document.getElementById('Edit-device-modal').style.display='none'" aria-label="Close modal" style="background: none; border: none; color: #94a3b8; font-size: 18px; cursor: pointer; transition: color 0.2s;"><i class="fas fa-times"></i></button>
            </div>
            <form id="Edit-device-form">
                <input type="hidden" id="Edit-old-ip">
                <div style="margin-bottom: 16px;">
                    <label for="Edit-device-name" style="display: block; font-size: 13px; font-weight: 500; color: #cbd5e1; margin-bottom: 6px;">Nama Perangkat</label>
                    <input type="text" id="Edit-device-name" required class="form-control" style="width: 100%; padding: 10px 14px; background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.15); border-radius: 8px; color: #fff; font-size: 14px; outline: none; box-sizing: border-box;">
                </div>
                <div style="margin-bottom: 16px;">
                    <label for="Edit-device-category" style="display: block; font-size: 13px; font-weight: 500; color: #cbd5e1; margin-bottom: 6px;">Kategori Perangkat</label>
                    <select id="Edit-device-category" required class="form-control" style="width: 100%; padding: 10px 14px; background: #1e293b; border: 1px solid rgba(255,255,255,0.15); border-radius: 8px; color: #fff; font-size: 14px; outline: none; appearance: auto; box-sizing: border-box;" aria-label="Edit-device-category">
                        <option value="router">Router</option>
                        <option value="radio">Radio</option>
                        <option value="access_point">Access Point (Auto-Detect Vendor)</option>
                        <option value="switch">Switch</option>
                        <option value="firewall">Firewall</option>
                        <option value="server">Server</option>
                    </select>
                </div>
                <div style="margin-bottom: 24px;">
                    <label for="Edit-device-ip" style="display: block; font-size: 13px; font-weight: 500; color: #cbd5e1; margin-bottom: 6px;">IP Address</label>
                    <input type="text" id="Edit-device-ip" required class="form-control" style="width: 100%; padding: 10px 14px; background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.15); border-radius: 8px; color: #fff; font-size: 14px; outline: none; box-sizing: border-box;">
                </div>
                <div class="modal-footer" style="display: flex; justify-content: flex-end; gap: 10px; margin-top: 20px;">
                    <button type="button" onclick="document.getElementById('Edit-device-modal').style.display='none'" class="btn-secondary" style="padding: 9px 18px; background: rgba(255,255,255,0.08); border: 1px solid rgba(255,255,255,0.15); border-radius: 8px; color: #fff; font-size: 13px; font-weight: 600; cursor: pointer;">Cancel</button>
                    <button type="submit" class="btn-primary" style="padding: 9px 20px; background: #3b82f6; border: none; border-radius: 8px; color: #fff; font-size: 13px; font-weight: 600; cursor: pointer; box-shadow: 0 4px 14px rgba(59,130,246,0.4);">Save Pereditan</button>
                </div>
            </form>
        </div>
    </div>`;
    document.body.insertAdjacentHTML('beforeend', html);
    
    document.getElementById('Edit-device-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const submitBtn = e.target.querySelector('button[type="submit"]');
        const oldIp = document.getElementById('Edit-old-ip').value;
        const newIp = document.getElementById('Edit-device-ip').value;
        const name = document.getElementById('Edit-device-name').value;
        const type = document.getElementById('Edit-device-category').value;
        
        if (submitBtn) {
            submitBtn.disabled = true;
            submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Menyimpan...';
        }

        try {
            const res = await fetch(API_ENDPOINTS.DEVICES_UPDATE, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ old_ip: oldIp, new_ip: newIp, name, type, parent_ip: '' })
            });
            if (res.ok) {
                showToast(`Perangkat "${name}" berhasil diperbarui`, 'success');
                document.getElementById('Edit-device-modal').style.display = 'none';
                if (window.fetchDevicesTable) {
                    window.fetchDevicesTable();
                }
            } else {
                const errTxt = await res.text();
                showToast(`Gagal memperbarui perangkat: ${errTxt}`, 'error');
            }
        } catch (err) {
            showToast('Error koneksi ke server', 'error');
        } finally {
            if (submitBtn) {
                submitBtn.disabled = false;
                submitBtn.innerHTML = 'Save Pereditan';
            }
        }
    });
}

export function EditDevice(ip, name, type, parentIp) {
    injectEditModal();
    const oldIpInput = document.getElementById('Edit-old-ip');
    const ipInput = document.getElementById('Edit-device-ip');
    const nameInput = document.getElementById('Edit-device-name');
    const select = document.getElementById('Edit-device-category');

    if (oldIpInput) oldIpInput.value = ip;
    if (ipInput) ipInput.value = ip;
    if (nameInput) nameInput.value = name;
    
    const typeValue = (type || '').toLowerCase().replace(/ /g, '_');
    if (select) {
        let matched = false;
        for (let opt of select.options) {
            if (opt.value === typeValue || opt.value === (type || '').toLowerCase()) {
                select.value = opt.value;
                matched = true;
                break;
            }
        }
        if (!matched) {
            if (typeValue.includes('ruijie')) {
                select.value = 'ruijie';
            } else if (typeValue.includes('tplink') || typeValue.includes('tp-link')) {
                select.value = 'tplink';
            } else if (typeValue.includes('ap') || typeValue.includes('access')) {
                select.value = 'access_point';
            } else if (typeValue.includes('switch')) {
                select.value = 'switch';
            } else if (typeValue.includes('radio')) {
                select.value = 'radio';
            } else if (typeValue.includes('firewall')) {
                select.value = 'firewall';
            } else if (typeValue.includes('server')) {
                select.value = 'server';
            } else {
                select.value = 'router';
            }
        }
    }
    
    const modal = document.getElementById('Edit-device-modal');
    if (modal) {
        modal.style.display = 'flex';
        modal.style.opacity = '1';
    }
}

// View device details (navigate to device-details view)
export function viewDeviceDetails(ip, name) {
    state.currentDetailIp = ip;
    const el = document.querySelector('[data-view="device-details"]');
    if (el) el.click();
}

// Change devices page
export function changeDevicesPage(page) {
    state.pagination.devices.current = page;
    fetchDevicesTable();
}

// Open Tambah Perangkat modal
export function openAddDeviceModal() {
    const addDeviceModal = document.getElementById('add-device-modal');
    if (addDeviceModal) {
        addDeviceModal.style.display = 'flex';
        addDeviceModal.style.opacity = '1';
        const nameInput = document.getElementById('device-name');
        if (nameInput) nameInput.focus();
    }
}

