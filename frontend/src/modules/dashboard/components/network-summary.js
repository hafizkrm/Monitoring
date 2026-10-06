import { isOnline } from '../../../shared/utils/helpers.js';
import { safeSetHTML, safeSetText } from '../stats.js';
import { metricsStore } from '../../../core/state/store.js';

let lastPerfStats = null;
let prevDevicePingMap = {};
const sparklineHistory = {
    'sparkline-avg-bw': [],
    'sparkline-max-bw': [],
    'sparkline-packet-loss': [],
    'sparkline-latency': [],
    'sparkline-jitter': []
};

// --- DOM CACHE ---
function getEl(id) {
    return document.getElementById(id);
}

export function initNetworkSummary() {
    metricsStore.subscribe('fullMetrics', (metrics) => {
        updateNetworkPerformanceSummary(metrics);
    });
}

function updateNetworkPerformanceSummary(metrics) {
    if (!metrics || metrics.length === 0) return;

    let totalBw = 0;
    let maxDeviceBw = 0;
    let totalLatency = 0;
    let latencyCount = 0;
    let totalLoss = 0;
    let lossCount = 0;
    let totalJitter = 0;
    let jitterCount = 0;

    let nonRadioCount = 0;

    metrics.forEach(m => {
        const devType = (m.device_type || m.type || '').toLowerCase();
        const isRadio = devType === 'radio';

        if (!isRadio) {
            const rx = parseFloat(m.rx_rate ?? 0);
            const tx = parseFloat(m.tx_rate ?? 0);
            const bw = rx + tx;
            totalBw += bw;
            if (bw > maxDeviceBw) maxDeviceBw = bw;
            nonRadioCount++;
        }

        const devId = m.id || m.ip;

        if (isOnline(m.status)) {
            const ping = parseFloat(m.ping || m.latency || m.latencyMs || m.latency_ms || m.LatencyMs || 0);
            if (ping > 0) {
                totalLatency += ping;
                latencyCount++;

                const explicitJitter = parseFloat(m.jitter || m.jitter_ms || m.jitterMs || 0);
                if (explicitJitter > 0) {
                    totalJitter += explicitJitter;
                    jitterCount++;
                } else if (devId && prevDevicePingMap[devId] !== undefined) {
                    const diff = Math.abs(ping - prevDevicePingMap[devId]);
                    totalJitter += (diff > 0 ? diff : 0.3);
                    jitterCount++;
                }
                if (devId) prevDevicePingMap[devId] = ping;
            }

            const explicitLoss = parseFloat(m.packet_loss || m.packetLoss || m.loss || m.Loss || 0);
            totalLoss += explicitLoss;
            lossCount++;
        } else {
            totalLoss += 100;
            lossCount++;
        }
    });

    let avgBwValue = nonRadioCount > 0 ? (totalBw / nonRadioCount) : 0; 
    let maxBwValue = maxDeviceBw;

    const formatBw = (val) => {
        if (val >= 1000) return `${(val / 1000).toFixed(0)} Gbps`;
        return `${val.toFixed(0)} Mbps`;
    };

    const avgBwStr = formatBw(avgBwValue);
    const maxBwStr = formatBw(maxBwValue);

    const avgLatencyNum = latencyCount > 0 ? (totalLatency / latencyCount) : 0;
    const avgLossNum = lossCount > 0 ? (totalLoss / lossCount) : 0;
    const avgJitterNum = jitterCount > 0 ? (totalJitter / jitterCount) : 0;

    const avgLatency = avgLatencyNum.toFixed(1);
    const avgLoss = avgLossNum.toFixed(2);
    const avgJitter = avgJitterNum.toFixed(1);

    const elAvgBw = getEl('perf-avg-bw');
    if (elAvgBw) elAvgBw.innerText = avgBwStr;

    const elMaxBw = getEl('perf-max-bw');
    if (elMaxBw) elMaxBw.innerText = maxBwStr;

    const elPacketLoss = getEl('perf-packet-loss');
    if (elPacketLoss) {
        elPacketLoss.innerText = `${avgLoss}%`;
        if (avgLossNum > 5) elPacketLoss.style.color = 'var(--accent-red)';
        else if (avgLossNum > 1) elPacketLoss.style.color = 'var(--accent-orange)';
        else elPacketLoss.style.color = 'var(--text-main)';
    }

    const elLatency = getEl('perf-latency');
    if (elLatency) {
        elLatency.innerText = `${avgLatency} ms`;
        if (avgLatencyNum > 300) elLatency.style.color = 'var(--accent-red)';
        else if (avgLatencyNum > 100) elLatency.style.color = 'var(--accent-orange)';
        else elLatency.style.color = 'var(--text-main)';
    }

    const elJitter = getEl('perf-jitter');
    if (elJitter) {
        elJitter.innerText = `${avgJitter} ms`;
        if (avgJitterNum > 100) elJitter.style.color = 'var(--accent-red)';
        else if (avgJitterNum > 30) elJitter.style.color = 'var(--accent-orange)';
        else elJitter.style.color = 'var(--text-main)';
    }

    const updateTrendIndicator = (id, currentVal, prevVal, isLowerBetter = false) => {
        const el = getEl(id);
        if (!el) return;

        if (prevVal === undefined || prevVal === null) {
            el.innerHTML = `<i class="fas fa-minus" style="color:var(--text-muted); font-size:9px;"></i> <span style="color:var(--text-muted);">0.0%</span>`;
            return;
        }

        const diff = currentVal - prevVal;
        if (Math.abs(diff) < 0.001) {
            el.innerHTML = `<i class="fas fa-minus" style="color:var(--text-muted); font-size:9px;"></i> <span style="color:var(--text-muted);">0.0%</span>`;
            return;
        }

        const pct = prevVal > 0 ? (diff / prevVal) * 100 : 0;
        const absPct = Math.abs(pct).toFixed(1);
        const isUp = diff > 0;
        const icon = isUp ? 'fa-arrow-up' : 'fa-arrow-down';
        
        let color;
        if (isLowerBetter) {
            color = isUp ? 'var(--accent-red)' : 'var(--accent-green)';
        } else {
            color = isUp ? 'var(--accent-green)' : 'var(--accent-red)';
        }

        const prefix = isUp ? '↑ ' : '↓ ';
        el.innerHTML = `<i class="fas ${icon}" style="color:${color}; margin-right:2px;"></i> <span style="color:${color};">${prefix}${absPct}%</span>`;
    };

    if (lastPerfStats) {
        updateTrendIndicator('perf-avg-bw-trend', avgBwValue, lastPerfStats.avgBwValue, false);
        updateTrendIndicator('perf-max-bw-trend', maxBwValue, lastPerfStats.maxBwValue, false);
        updateTrendIndicator('perf-packet-loss-trend', avgLossNum, lastPerfStats.avgLossNum, true);
        updateTrendIndicator('perf-latency-trend', avgLatencyNum, lastPerfStats.avgLatencyNum, true);
        updateTrendIndicator('perf-jitter-trend', avgJitterNum, lastPerfStats.avgJitterNum, true);
    } else {
        updateTrendIndicator('perf-avg-bw-trend', avgBwValue, undefined, false);
        updateTrendIndicator('perf-max-bw-trend', maxBwValue, undefined, false);
        updateTrendIndicator('perf-packet-loss-trend', avgLossNum, undefined, true);
        updateTrendIndicator('perf-latency-trend', avgLatencyNum, undefined, true);
        updateTrendIndicator('perf-jitter-trend', avgJitterNum, undefined, true);
    }

    lastPerfStats = { avgBwValue, maxBwValue, avgLossNum, avgLatencyNum, avgJitterNum };

    const pointsToSmoothPath = (pts) => {
        if (pts.length === 0) return '';
        if (pts.length === 1) return `M ${pts[0].x.toFixed(1)},${pts[0].y.toFixed(1)}`;
        let path = `M ${pts[0].x.toFixed(1)},${pts[0].y.toFixed(1)}`;
        for (let i = 0; i < pts.length - 1; i++) {
            const p0 = pts[i === 0 ? i : i - 1];
            const p1 = pts[i];
            const p2 = pts[i + 1];
            const p3 = pts[i + 2 < pts.length ? i + 2 : i + 1];

            const cp1x = p1.x + (p2.x - p0.x) / 6;
            const cp1y = p1.y + (p2.y - p0.y) / 6;
            const cp2x = p2.x - (p3.x - p1.x) / 6;
            const cp2y = p2.y - (p3.y - p1.y) / 6;
            path += ` C ${cp1x.toFixed(1)},${cp1y.toFixed(1)} ${cp2x.toFixed(1)},${cp2y.toFixed(1)} ${p2.x.toFixed(1)},${p2.y.toFixed(1)}`;
        }
        return path;
    };

    const updateSparkline = (id, currentVal) => {
        const svgPath = document.getElementById(id);
        if (!svgPath) return;

        if (!sparklineHistory[id]) sparklineHistory[id] = [];
        const hist = sparklineHistory[id];
        hist.push(currentVal);
        if (hist.length > 10) hist.shift();

        let displayHist = [...hist];
        while (displayHist.length < 10) {
            const firstVal = displayHist[0] !== undefined ? displayHist[0] : currentVal;
            const idx = displayHist.length;
            const waveOffset = Math.sin(idx * 0.7) * (firstVal > 0 ? firstVal * 0.05 : 1.2);
            displayHist.unshift(Math.max(0, firstVal + waveOffset));
        }

        let minV = Math.min(...displayHist);
        let maxV = Math.max(...displayHist);

        if ((maxV - minV) < 0.01) {
            const base = maxV > 0 ? maxV : 10;
            const t = Date.now() / 800;
            displayHist = displayHist.map((v, i) => {
                const wave = Math.sin((i * 0.9) + t) * (base * 0.08 || 1.5);
                return base + wave;
            });
            minV = Math.min(...displayHist);
            maxV = Math.max(...displayHist);
        }

        const range = (maxV - minV) || 1;
        const pts = displayHist.map((val, i) => {
            const x = (i / (displayHist.length - 1)) * 100;
            const norm = (val - minV) / range;
            const y = 24 - (norm * 18);
            return { x, y };
        });

        svgPath.setAttribute('d', pointsToSmoothPath(pts));
    };

    updateSparkline('sparkline-avg-bw', avgBwValue);
    updateSparkline('sparkline-max-bw', maxBwValue);
    updateSparkline('sparkline-packet-loss', avgLossNum);
    updateSparkline('sparkline-latency', avgLatencyNum);
    updateSparkline('sparkline-jitter', avgJitterNum);
}
