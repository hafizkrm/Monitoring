// Terminal diagnostic functions (Ping, Trace) with SSE support
import { getStorageItem, STORAGE_KEYS } from '../../core/services/storage.service.js';
import { escapeHtml } from '../utils/helpers.js';

export function createTerminalModal(title, rawIp) {
    const ip = escapeHtml(rawIp);
    // Remove existing modal if any
    const existing = document.getElementById('terminal-modal');
    if (existing) existing.remove();

    const modal = document.createElement('div');
    modal.id = 'terminal-modal';
    modal.style.cssText = 'position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.8);backdrop-filter:blur(4px);z-index:200;display:flex;align-items:center;justify-content:center;';
    modal.innerHTML = `
        <div style="background:#0d1117;border:1px solid #30363d;border-radius:12px;width:100%;max-width:600px;max-height:80vh;display:flex;flex-direction:column;box-shadow:0 20px 60px rgba(0,0,0,0.8);">
            <div style="display:flex;justify-content:space-between;align-items:center;padding:16px 20px;border-bottom:1px solid #30363d;">
                <div style="display:flex;align-items:center;gap:10px;">
                    <span style="width:12px;height:12px;border-radius:50%;background:#ff5f57;display:inline-block;"></span>
                    <span style="width:12px;height:12px;border-radius:50%;background:#febc2e;display:inline-block;"></span>
                    <span style="width:12px;height:12px;border-radius:50%;background:#28c840;display:inline-block;"></span>
                    <span style="font-family:monospace;font-size:13px;color:#8b949e;margin-left:8px;">${title} → ${ip}</span>
                </div>
                <button id="terminal-close-btn" aria-label="Close terminal" style="background:transparent;border:none;color:#8b949e;font-size:18px;cursor:pointer;padding:4px;">
                    <i class="fas fa-times"></i>
                </button>
            </div>
            <div id="terminal-output" style="flex:1;overflow-y:auto;padding:16px 20px;font-family:monospace;font-size:12px;line-height:1.8;color:#c9d1d9;min-height:300px;max-height:60vh;background:#0d1117;border-radius:0 0 12px 12px;">
                <span style="color:#58a6ff;">$ ${title.toLowerCase()} ${ip}</span><br>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
    // Close on backdrop click or close button
    modal.addEventListener('click', (e) => { if (e.target === modal) modal.remove(); });
    modal.querySelector('#terminal-close-btn').addEventListener('click', () => modal.remove());
    return document.getElementById('terminal-output');
}

export async function autoResolveAlertIfSuccessful(ip) {
    if (!ip) return;
    try {
        const res = await fetch(`/api/alerts/resolve-by-ip?ip=${encodeURIComponent(ip)}`, { method: 'POST' });
        if (res.ok) {
            if (window.showToast) {
                window.showToast(`Status ${ip} telah dipulihkan. Alert diselesaikan.`);
            }
            if (typeof window.monRenderAlerts === 'function') {
                window.monRenderAlerts();
            }
            if (typeof window.monFetchAlerts === 'function') {
                window.monFetchAlerts();
            }
            if (typeof window.monFetchDashboardData === 'function') {
                window.monFetchDashboardData();
            }
        }
    } catch(err) {
        console.error('Failed to auto resolve alert by IP:', err);
    }
}

function runDiagnostic(type, ip) {
    const isPing = type === 'PING';
    const title = isPing ? 'PING' : 'TRACERT';
    const output = createTerminalModal(title, ip);
    let success = false;
    
    const appendLine = (text, color = '#c9d1d9') => {
        const line = document.createElement('div');
        line.style.color = color;
        line.textContent = text; // textContent: stream output must never be parsed as HTML
        output.appendChild(line);
        output.scrollTop = output.scrollHeight;
    };

    appendLine(isPing ? `Mengirim ping ke ${ip}...` : `Menjalankan traceroute ke ${ip}...`, '#58a6ff');

    const session = getStorageItem(STORAGE_KEYS.SESSION);
    const token = session ? session.token : '';
    const endpoint = isPing ? 'ping' : 'trace';

    const es = new EventSource(`/api/tools/${endpoint}?ip=${encodeURIComponent(ip)}&token=${encodeURIComponent(token)}&t=${Date.now()}`);
    
    es.onmessage = (e) => {
        const text = e.data || '';
        if (text === 'DONE' || text === '[DONE]' || text === '[Process Completed]') {
            es.close();
            appendLine(isPing ? '--- Selesai ---' : '--- Traceroute Selesai ---', '#28c840');
            if (success) {
                autoResolveAlertIfSuccessful(ip);
            }
            return;
        }
        
        const lower = text.toLowerCase();
        if (isPing) {
            if (lower.includes('reply from') || lower.includes('bytes from') || lower.includes('ttl=')) success = true;
        } else {
            if (lower.includes('trace complete') || lower.includes('selesai') || text.includes(ip) || (text.match(/^\s*\d+/) && text.includes('ms') && !lower.includes('request timed out'))) success = true;
        }
        
        appendLine(text);
    };
    
    es.onerror = () => {
        es.close();
        appendLine('Koneksi ke backend terputus atau tidak tersedia.', '#ff5f57');
    };
}

export function runPing(ip) {
    runDiagnostic('PING', ip);
}

export function runTrace(ip) {
    runDiagnostic('TRACE', ip);
}

// Maintain backward compatibility for inline HTML calls
window.runPing = runPing;
window.runTrace = runTrace;
