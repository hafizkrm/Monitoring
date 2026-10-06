// Helper Functions

// Check if device is online
export const isOnline = (StatusOrDevice, pingVal, latencyVal) => {
    let s = '';
    let p = 0;
    
    if (typeof StatusOrDevice === 'object' && StatusOrDevice !== null) {
        const d = StatusOrDevice;
        p = parseFloat(d.ping || d.latency || d.latency_ms || d.latencyMs || 0);
        s = String(d.status || '').toLowerCase();
    } else {
        p = parseFloat(pingVal || latencyVal || 0);
        if (StatusOrDevice) {
            s = String(StatusOrDevice).toLowerCase();
        }
    }

    if (['down', 'offline', 'failed', 'timeout', 'disabled', 'maintenance', 'unreachable'].includes(s)) {
        return false;
    }
    
    if (['up', 'online', 'success', 'ok', 'reachable', '1', 'active', 'normal', 'degraded'].includes(s)) {
        return true;
    }

    if (p > 0) return true;

    return false;
};

export function getDeviceStatusMeta(Status) {
    const s = String(Status || '').toLowerCase();
    if (['up', 'online', 'success', 'ok', 'reachable', '1', 'active', 'normal'].includes(s)) {
        return { label: 'Online', color: '#22c55e', blink: true };
    }
    if (s === 'degraded') {
        return { label: 'Online (Ping)', color: '#22c55e', blink: false }; // Treat as online but indicate it's ping only
    }
    if (s === 'unknown') {
        return { label: 'Menunggu...', color: '#94a3b8', blink: false }; // slate
    }
    return { label: 'Offline', color: '#ef4444', blink: false };
}

// Escape HTML special characters
export const escapeHtml = (value) => String(value ?? '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#039;'
}[char]));

// Normalize interface Status
export function normalizeInterfaceStatus(Status) {
    const s = String(Status || '').toLowerCase();
    if (['disabled', 'admin-down', 'administratively down', 'admin_down'].includes(s)) return 'disabled';
    if (['up', 'online', '1'].includes(s)) return 'up';
    if (['down', 'offline', '2'].includes(s)) return 'down';
    return s || 'unknown';
}

// Get interface Status metadata
export function getInterfaceStatusMeta(Status) {
    const normalized = normalizeInterfaceStatus(Status);
    if (normalized === 'up') return { label: 'UP', badgeClass: 'up', ledClass: 'led-green' };
    if (normalized === 'disabled') return { label: 'DISABLED', badgeClass: 'disabled', ledClass: 'led-gray' };
    return { label: 'DOWN', badgeClass: 'down', ledClass: 'led-amber' };
}

// Metric quality class
export function metricQualityClass(value, type) {
    if (value === null || value === undefined || value === '' || Number(value) === 0) return 'muted';
    const n = Number(value);
    if (type === 'signal') return n >= -67 ? 'good' : (n >= -75 ? 'warn' : 'bad');
    if (type === 'ccq' || type === 'capacity') return n >= 85 ? 'good' : (n >= 70 ? 'warn' : 'bad');
    if (type === 'noise') return n <= -90 ? 'good' : (n <= -80 ? 'warn' : 'bad');
    return 'good';
}

export function renderPagination(targetId, totalItems, limit, currentPage, callbackName) {
    const container = document.getElementById(targetId);
    if (!container) return;

    limit = parseInt(limit, 10) || 10;
    totalItems = parseInt(totalItems, 10) || 0;
    currentPage = parseInt(currentPage, 10) || 1;
    const totalPages = Math.max(1, Math.ceil(totalItems / limit));

    const startItem = totalItems > 0 ? (currentPage - 1) * limit + 1 : 0;
    const endItem = Math.min(currentPage * limit, totalItems);

    let html = `
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; width: 100%; padding: 12px 4px; border-top: 1px solid rgba(255, 255, 255, 0.08); margin-top: 8px;">
            <div style="font-size: 13px; color: var(--text-muted, #94a3b8); font-weight: 500;">
                Menampilkan <strong style="color: #60a5fa;">${startItem} - ${endItem}</strong> dari <strong style="color: #f8fafc;">${totalItems}</strong> perangkat (Halaman <strong style="color: #60a5fa;">${currentPage}</strong> dari <strong style="color: #f8fafc;">${totalPages}</strong>)
            </div>
            <div class="logs-pagination" style="display: flex !important; align-items: center !important; gap: 6px !important; margin: 0 !important; width: auto !important;">
    `;

    html += `<button class="page-btn" ${currentPage <= 1 ? 'disabled' : ''} aria-label="Previous page" onclick="${callbackName}(${currentPage - 1})"><i class="fas fa-chevron-left"></i></button>`;

    for (let i = 1; i <= totalPages; i++) {
        if (i === 1 || i === totalPages || (i >= currentPage - 2 && i <= currentPage + 2)) {
            html += `<button class="page-btn ${i === currentPage ? 'active' : ''}" onclick="${callbackName}(${i})">${i}</button>`;
        } else if (i === currentPage - 3 || i === currentPage + 3) {
            html += `<span class="page-dots">...</span>`;
        }
    }

    html += `<button class="page-btn" ${currentPage >= totalPages ? 'disabled' : ''} aria-label="Next page" onclick="${callbackName}(${currentPage + 1})"><i class="fas fa-chevron-right"></i></button>`;

    html += `
            </div>
        </div>
    `;

    container.innerHTML = html;
}

// Format bytes to human readable format
export function formatBytes(bytes, defaultUnit = 'Mbps') {
    if (bytes === 0 || bytes === null || bytes === undefined) return `0 ${defaultUnit}`;
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];

    // If the input is already in Mbps, let's just return it or handle accordingly
    // Assuming the backend sends data directly matching the default unit if it's not raw bytes
    if (defaultUnit === 'Mbps') {
        return `${parseFloat(bytes).toFixed(1)} Mbps`;
    }

    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Format upTime (seconds or RouterOS string) to compact human readable format
export function formatUpTime(seconds) {
    if (!seconds && seconds !== 0) return '0s';
    if (typeof seconds === 'number' || (!isNaN(seconds) && !/[a-z]/i.test(seconds))) {
        const totalSeconds = parseInt(seconds, 10);
        if (isNaN(totalSeconds) || totalSeconds <= 0) return '0s';
        const w = Math.floor(totalSeconds / (3600 * 24 * 7));
        const d = Math.floor((totalSeconds % (3600 * 24 * 7)) / (3600 * 24));
        const h = Math.floor((totalSeconds % (3600 * 24)) / 3600);
        const m = Math.floor((totalSeconds % 3600) / 60);
        const s = totalSeconds % 60;
        const parts = [];
        if (w > 0) parts.push(`${w}w`);
        if (d > 0) parts.push(`${d}d`);
        if (h > 0) parts.push(`${h}h`);
        if (m > 0 && parts.length < 3) parts.push(`${m}m`);
        if (s > 0 && parts.length < 2) parts.push(`${s}s`);
        return parts.join(' ');
    }
    const str = String(seconds).trim();
    const matchW = str.match(/(\d+)\s*w/i);
    const matchD = str.match(/(\d+)\s*d/i);
    const matchH = str.match(/(\d+)\s*(?:h|j)/i);
    const matchM = str.match(/(\d+)\s*m(?!s)/i);
    const matchS = str.match(/(\d+)\s*s/i);

    const parts = [];
    if (matchW) parts.push(`${matchW[1]}w`);
    if (matchD) parts.push(`${matchD[1]}d`);
    if (matchH) parts.push(`${matchH[1]}h`);
    if (matchM && parts.length < 3) parts.push(`${matchM[1]}m`);
    if (matchS && parts.length < 2) parts.push(`${matchS[1]}s`);

    return parts.length > 0 ? parts.join(' ') : str;
}

export function matchesCategoryFilter(device, filter) {
    if (!filter || filter === 'all') return true;
    const filterLower = filter.toLowerCase().replace(/_/g, ' ');
    const rawType = (device.device_type || device.type || '').toLowerCase().replace(/_/g, ' ');
    const vendor = (device.vendor || '').toLowerCase();
    
    if (filterLower === 'router') return rawType.includes('router') || vendor.includes('mikrotik') || rawType === 'router';
    if (filterLower === 'radio') return rawType.includes('radio') || vendor.includes('ubiquiti') || rawType.includes('airmax') || rawType.includes('wireless') || rawType.includes('ptp');
    if (filterLower === 'switch') return rawType.includes('switch');
    if (filterLower === 'access_point' || filterLower === 'ap') return rawType.includes('access') || rawType.includes('ap') || vendor.includes('ruijie') || vendor.includes('tplink');
    if (filterLower === 'firewall') return rawType.includes('firewall');
    
    return rawType.includes(filterLower);
}