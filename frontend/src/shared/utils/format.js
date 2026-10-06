// Formatting Utilities

// Format traffic from bytes/sec to human readable
export function formatTraffic(bytesPerSec) {
    const kbps = bytesPerSec / 1024;
    if (kbps > 1024) return `${(kbps / 1024).toFixed(2)} MB/s`;
    return `${kbps.toFixed(2)} KB/s`;
}

// Format link speed from bits/sec (used by interfaces.js)
export function formatLinkSpeed(bitsPerSecond) {
    const bps = Number(bitsPerSecond || 0);
    if (bps <= 0) return '--';
    if (bps >= 1000000000) return `${(bps / 1000000000).toFixed(bps % 1000000000 === 0 ? 0 : 1)} Gbps`;
    if (bps >= 1000000) return `${(bps / 1000000).toFixed(bps % 1000000 === 0 ? 0 : 1)} Mbps`;
    if (bps >= 1000) return `${(bps / 1000).toFixed(0)} Kbps`;
    return `${bps} bps`;
}