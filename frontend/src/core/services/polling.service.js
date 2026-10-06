// Polling Service is now Event-Driven from WebSocket
import { checkDeviceStatusChanges } from './monitoring.service.js';
import { updateDashboardStats, updateSLA, updateLiveStats } from '../../modules/dashboard/stats.js';
import { state } from '../state.js';
import { metricsStore } from '../state/store.js';

let pollingInterval = null;

export async function startPolling() {
    // Phase 2 Item 4: Fetch full table via HTTP to avoid websocket overload
    pollMetrics();
    if (!pollingInterval) {
        pollingInterval = setInterval(pollMetrics, 10000); // Poll every 10 seconds
    }
    
    // Listen for WebSocket updates via metricsStore
    metricsStore.subscribe('fullMetrics', (data) => {
        processMetricsUpdate(data);
    });
}

async function pollMetrics() {
    // Only poll if on dashboard or devices to save bandwidth
    const activeNav = document.querySelector('.nav-item.active');
    const view = activeNav ? activeNav.getAttribute('data-view') : null;
    
    if (view === 'dashboard' || view === 'devices') {
        try {
            const res = await fetch('/api/metrics?t=' + Date.now());
            if (res.ok) {
                const data = await res.json();
                if (data && Array.isArray(data)) {
                    metricsStore.setState({ fullMetrics: data });
                }
            }
        } catch (err) {
            console.error('[Polling] Failed to fetch metrics', err);
        }
    }
}

function processMetricsUpdate(data) {
    try {
        state.latestLatencyMap = {};
        
        // Update dashboard stats
        updateDashboardStats(data);
        
        // Check for Status changes
        checkDeviceStatusChanges(data);
        
        // Update SLA
        updateSLA(data);
        
        // Update charts and live stats
        if (data && data.length > 0) {
            updateLiveStats(data[0]);
        }
        
    } catch (err) {
        console.error('[WebSocket] Error processing metrics update:', err);
    }
}

