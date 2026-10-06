// Global State Management
export const state = {
    // Pagination State
    pagination: {
        health: { current: 1, limit: 50 },
        devices: { current: 1, limit: 10 },
        logs: { current: 1, limit: 50 }
    },

    // Device Status Tracking (for notifications)
    deviceStates: {},

    // Traffic History (per-device octet tracking)
    trafficHistory: {},

    // Interface Traffic History
    ifaceTrafficHistory: {},

    // Current Detail View IP
    currentDetailIp: null,

    // Previous View for back navigation
    previousView: null,

    // Chart Instances
    charts: {
        cpu: null,
        ram: null,
        routerTraffic: null,
        radioTraffic: null,
        detail: null,
        detailTraffic: null,
        dashTraffic: null,
        dashStatusPie: null,
        dashResource: null,
        dashLatency: null,
        dashDeviceType: null,
        sparklineAvgBandwidth: null,
        sparklineMaxBandwidth: null,
        sparklinePacketLoss: null,
        sparklineLatency: null,
        sparklineJitter: null
    },

    // Latency Map
    latestLatencyMap: {}
};

// Get state value
export function getState(key) {
    return state[key];
}

// Set state value
export function setState(key, value) {
    state[key] = value;
}

// Get pagination current
export function getPage(view) {
    return state.pagination[view]?.current || 1;
}

// Update pagination
export function setPage(view, page) {
    if (state.pagination[view]) {
        state.pagination[view].current = page;
    }
}