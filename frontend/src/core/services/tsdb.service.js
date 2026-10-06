// TSDB Service - Interface to Prometheus/VictoriaMetrics TSDB Backend
// Phase 3: Provides time-series queries for dashboard charts and device detail views

const TSDB_ENDPOINTS = {
    DEVICE_HISTORY: '/api/tsdb/device-history',
    QUERY: '/api/tsdb/query',
    STATUS: '/api/tsdb/status',
};

let _tsdbAvailable = null; // cached status

/**
 * Check if TSDB (Prometheus) is reachable.
 * @returns {Promise<boolean>}
 */
export async function isTSDBAvailable() {
    if (_tsdbAvailable !== null) return _tsdbAvailable;
    try {
        const res = await fetch(TSDB_ENDPOINTS.STATUS);
        if (!res.ok) { _tsdbAvailable = false; return false; }
        const data = await res.json();
        _tsdbAvailable = data.status === 'connected';
        // Re-check every 60s
        setTimeout(() => { _tsdbAvailable = null; }, 60000);
        return _tsdbAvailable;
    } catch {
        _tsdbAvailable = false;
        setTimeout(() => { _tsdbAvailable = null; }, 30000);
        return false;
    }
}

/**
 * Fetch per-device metrics history from TSDB with SQL fallback.
 * Returns { source: 'tsdb'|'sql', data: [...] }
 * @param {string} ip - Device IP address
 * @param {string} [duration='30m'] - Time range (e.g. '30m', '1h', '6h', '24h')
 */
export async function fetchDeviceHistory(ip, duration = '30m') {
    try {
        const url = `${TSDB_ENDPOINTS.DEVICE_HISTORY}?ip=${encodeURIComponent(ip)}&duration=${encodeURIComponent(duration)}`;
        const res = await fetch(url);
        if (!res.ok) throw new Error(`TSDB device-history returned ${res.status}`);
        return await res.json();
    } catch (err) {
        console.warn('[TSDB] fetchDeviceHistory failed:', err);
        throw err;
    }
}

/**
 * Run an instant PromQL query.
 * @param {string} promql - PromQL expression
 * @returns {Promise<object>} Raw Prometheus query response
 */
export async function queryTSDB(promql) {
    const url = `${TSDB_ENDPOINTS.QUERY}?query=${encodeURIComponent(promql)}`;
    const res = await fetch(url);
    if (!res.ok) throw new Error(`TSDB query returned ${res.status}`);
    return await res.json();
}

/**
 * Parse Prometheus instant query result into a simple key-value map.
 * @param {object} promResponse - Response from queryTSDB()
 * @param {string} labelKey - The label to use as the key (e.g. 'ip', 'hostname')
 * @returns {Map<string, number>}
 */
export function parseInstantResult(promResponse, labelKey = 'ip') {
    const map = new Map();
    if (!promResponse?.data?.result) return map;
    for (const series of promResponse.data.result) {
        const key = series.metric?.[labelKey] || 'unknown';
        const value = series.value?.[1] ? parseFloat(series.value[1]) : 0;
        map.set(key, value);
    }
    return map;
}

/**
 * Parse Prometheus range query result into time-series arrays.
 * @param {object} promResponse
 * @param {string} labelKey
 * @returns {Map<string, {timestamps: number[], values: number[]}>}
 */
export function parseRangeResult(promResponse, labelKey = 'ip') {
    const map = new Map();
    if (!promResponse?.data?.result) return map;
    for (const series of promResponse.data.result) {
        const key = series.metric?.[labelKey] || 'unknown';
        const timestamps = [];
        const values = [];
        for (const [ts, val] of (series.values || [])) {
            timestamps.push(ts);
            values.push(parseFloat(val));
        }
        map.set(key, { timestamps, values });
    }
    return map;
}
