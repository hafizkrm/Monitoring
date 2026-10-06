// Real-Time Service for NMS (WebSocket)
import { showToast } from '../../shared/components/toast.js';
import { metricsStore } from '../state/store.js';

let ws = null;
let reconnectAttempts = 0;
const maxReconnectDelay = 30000;

let activeTopics = [];

export async function initRealTime() {
    connectWebSocket();
}

export function subscribeToTopics(topics) {
    topics.forEach(t => {
        if (!activeTopics.includes(t)) activeTopics.push(t);
    });
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ action: 'subscribe', topics: topics }));
    }
}

export function unsubscribeFromTopics(topics) {
    activeTopics = activeTopics.filter(t => !topics.includes(t));
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ action: 'unsubscribe', topics: topics }));
    }
}

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host || 'localhost:8080';
    const wsUrl = `${protocol}//${host}/ws`;
    
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        console.log("[RealTime] WebSocket Connected");
        reconnectAttempts = 0;
        
        // Always subscribe to global topic for alerts
        if (!activeTopics.includes('global')) {
            activeTopics.push('global');
        }
        
        // Re-subscribe to active topics upon reconnect
        if (activeTopics.length > 0) {
            ws.send(JSON.stringify({ action: 'subscribe', topics: activeTopics }));
        }
    };

    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            handleWSEvent(data);
        } catch (e) {
            console.error("[RealTime] Error parsing WS Message", e);
        }
    };

    ws.onclose = () => {
        scheduleReconnect();
    };

    ws.onerror = (err) => {
        console.error("[RealTime] WebSocket Error", err);
        ws.close();
    };
}

function scheduleReconnect() {
    reconnectAttempts++;
    const delay = Math.min(1000 * Math.pow(2, reconnectAttempts) + Math.random() * 1000, maxReconnectDelay);
    console.log(`[RealTime] Reconnecting in ${(delay / 1000).toFixed(1)}s (attempt ${reconnectAttempts})...`);
    setTimeout(connectWebSocket, delay);
}

function handleWSEvent(envelope) {
    switch (envelope.event) {
        case "device.connected":
            showToast(`Device ${envelope.payload.hostname || envelope.payload.name || ''} is online`, "success");
            break;
            
        case "metrics.updated":
            if (envelope.payload) {
                if (envelope.payload.uptime !== undefined && envelope.payload.upTime === undefined) {
                    envelope.payload.upTime = envelope.payload.uptime;
                }
                let currentMetrics = metricsStore.getState().fullMetrics;
                if (!Array.isArray(currentMetrics)) {
                    currentMetrics = [];
                }
                const devIp = envelope.payload.ip || envelope.payload.ip_address;
                const idx = currentMetrics.findIndex(x => x.ip === devIp || x.ip_address === devIp);
                
                let newMetrics = [...currentMetrics];
                if (idx !== -1) {
                    newMetrics[idx] = { ...newMetrics[idx], ...envelope.payload };
                } else {
                    newMetrics.push(envelope.payload);
                }
                
                if (!window._wsDispatchTimer) {
                    // Update the local variable continuously, but throttle the reactive store update
                    window._wsPendingMetrics = newMetrics;
                    window._wsDispatchTimer = setTimeout(() => {
                        metricsStore.setState({ fullMetrics: window._wsPendingMetrics });
                        window._wsDispatchTimer = null;
                    }, 1000);
                } else {
                     window._wsPendingMetrics = newMetrics;
                }
            }
            break;
            
        default:
            break;
    }
}
