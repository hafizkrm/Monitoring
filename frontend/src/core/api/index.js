// API Module - Interacts with Golang Backend

const API_ENDPOINTS = {
    METRICS: '/api/metrics',
    METRICS_HISTORY: '/api/metrics/history',
    INVENTORY: '/api/inventory',
    DEVICES: '/api/devices',
    DEVICES_DELETE: '/api/devices/delete',
    DEVICES_UPDATE: '/api/devices/update',
    RELIABILITY: '/api/reliability',
    INTERFACES: '/api/interfaces',
    PING: '/api/tools/ping',
    TRACE: '/api/tools/trace',
    ALERTS: '/api/alerts'
};

// Fetch JSON with fallback URLs
export async function fetchJsonWithFallback(urls) {
    let lastError = null;
    for (const url of urls) {
        try {
            const res = await fetch(url);
            if (!res.ok) throw new Error(`${url} returned ${res.status}`);
            const contentType = res.headers.get('content-type') || '';
            const text = await res.text();
            if (!contentType.includes('json') && !text.trim().startsWith('[') && !text.trim().startsWith('{')) {
                throw new Error(`${url} did not return JSON`);
            }
            return JSON.parse(text);
        } catch (err) {
            lastError = err;
        }
    }
    throw lastError || new Error('No API fallback returned JSON');
}

// Fetch metrics
export async function fetchMetrics() {
    try {
        const res = await fetch(API_ENDPOINTS.METRICS);
        if (!res.ok) {
            throw new Error(`Server responded with status ${res.status}`);
        }
        const text = await res.text();
        const data = JSON.parse(text);
        if (!Array.isArray(data)) {
            throw new Error("Parsed data is not an array");
        }
        return data;
    } catch (error) {
        console.error("Failed to fetch metrics from Go API:", error);
        throw error;
    }
}

// Fetch inventory
export async function fetchInventory() {
    const res = await fetch(API_ENDPOINTS.INVENTORY);
    return await res.json();
}


// Fetch alerts
export async function fetchAlertsApi() {
    const res = await fetch(API_ENDPOINTS.ALERTS);
    return await res.json();
}

// Fetch reliability data
export async function fetchReliability() {
    try {
        const res = await fetch(API_ENDPOINTS.RELIABILITY);
        return await res.json();
    } catch (e) {
        return {};
    }
}

// Fetch interfaces for a device
export async function fetchInterfaces(ip) {
    const encodedIp = encodeURIComponent(ip);
    return await fetchJsonWithFallback([
        `${API_ENDPOINTS.INTERFACES}?ip=${encodedIp}`,
        `${API_ENDPOINTS.INTERFACES.slice(0, -4)}?ip=${encodedIp}` // api/interfaces
    ]);
}

// Fetch device history (Phase 3: TSDB-first with SQL fallback)
export async function fetchHistory(ip, duration = '30m') {
    const encodedIp = encodeURIComponent(ip);
    try {
        // Try TSDB endpoint first
        const tsdbRes = await fetch(`/api/tsdb/device-history?ip=${encodedIp}&duration=${encodeURIComponent(duration)}`);
        if (tsdbRes.ok) {
            const result = await tsdbRes.json();
            if (result && result.data && result.data.length > 0) {
                return result.data;
            }
        }
    } catch (e) {
        console.warn('[API] TSDB device-history failed, falling back to SQL:', e);
    }
    // Fallback to SQL
    return await fetchJsonWithFallback([
        `${API_ENDPOINTS.METRICS_HISTORY}?ip=${encodedIp}`
    ]);
}

// Register/connect device
export async function connectDevice(ip, name, type) {
    return await fetch(API_ENDPOINTS.DEVICES, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ip, name, type })
    });
}

// Delete device
export async function DeleteDevice(ip) {
    return await fetch(API_ENDPOINTS.DEVICES_DELETE, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ip })
    });
}

// Update device
export async function updateDevice(oldIp, newIp, name, type, parentIp) {
    return await fetch(API_ENDPOINTS.DEVICES_UPDATE, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            old_ip: oldIp,
            new_ip: newIp,
            name,
            type,
            parent_ip: parentIp
        })
    });
}

// Ping device (EventSource)
export function runPingEventSource(ip) {
    return new EventSource(`${API_ENDPOINTS.PING}?ip=${encodeURIComponent(ip)}&t=${Date.now()}`);
}

// Traceroute device (EventSource)
export function runTraceEventSource(ip) {
    return new EventSource(`${API_ENDPOINTS.TRACE}?ip=${encodeURIComponent(ip)}&t=${Date.now()}`);
}
