// VPN Users Component
// Menampilkan daftar pengguna VPN yang sedang aktif

import { formatUpTime } from '../../../shared/utils/helpers.js';
import { safeSetHTML } from '../stats.js';

function getEl(id) {
    return document.getElementById(id);
}

let lastVPNFetchTime = 0;
let lastVPNHtml = null;

export function initVPNUsers() {
    // Poll VPN users setiap 10 detik saat dipanggil dari updateDashboardStats
    window.addEventListener('metrics-updated', () => {
        if (Date.now() - lastVPNFetchTime > 10000) {
            updateVPNUser();
            lastVPNFetchTime = Date.now();
        } else if (lastVPNHtml !== null) {
            const container = getEl('vpn-User-container');
            if (container) safeSetHTML(container, lastVPNHtml);
        }
    });
}

export async function updateVPNUser() {
    const container = getEl('vpn-User-container');
    if (!container) return;

    try {
        const res = await fetch('/api/vpn/users');
        if (res.ok) {
            const data = await res.json();
            if (data && data.success) {
                const users = (data.users || data.User || []).slice(0, 5);
                
                if (users.length === 0) {
                    container.style.display = 'flex';
                    container.style.flexDirection = 'column';
                    container.style.justifyContent = 'center';
                    container.innerHTML = `
                    <div style="display:flex; flex-direction:column; align-items:center; justify-content:center; flex:1; width:100%; min-height:100px; padding:18px 12px; text-align:center; background:rgba(59, 130, 246, 0.05); border:1px solid rgba(59, 130, 246, 0.18); border-radius:10px; box-sizing:border-box;">
                        <div style="position:relative; display:flex; align-items:center; justify-content:center; width:36px; height:36px; background:rgba(59, 130, 246, 0.12); border-radius:50%; margin-bottom:8px; box-shadow:0 0 12px rgba(59, 130, 246, 0.25);">
                            <i class="fas fa-shield-halved" style="font-size:16px; color:#60a5fa;"></i>
                        </div>
                        <span style="font-size:12px; font-weight:700; color:#60a5fa; letter-spacing:0.4px;">NO ACTIVE SESSIONS</span>
                        <span style="font-size:10.5px; color:#94a3b8; margin-top:3px;">All OpenVPN user channels are idle</span>
                    </div>`;
                    return;
                } else {
                    container.style.display = '';
                    container.style.flexDirection = '';
                    container.style.justifyContent = '';
                }

                const newHTML = users.map((user, index) => {
                    return `
                    <div class="premium-list-box" style="margin-bottom: 8px;">
                        <div class="premium-box-icon" style="background:var(--accent-blue-transparent); color:var(--accent-blue);">
                            ${index + 1}
                        </div>
                        <div class="premium-box-details">
                            <div style="display:flex; justify-content:space-between; margin-bottom:2px;">
                                <span class="premium-box-title" title="${user.name}">${user.name}</span>
                                <span class="premium-box-action" style="color:var(--text-muted);">${formatUpTime(user.upTime)}</span>
                            </div>
                            <div style="display:flex; justify-content:space-between;">
                                <span class="premium-box-subtitle" title="IP: ${user.address}">IP: ${user.address}</span>
                                <span class="premium-box-subtitle" title="Caller ID: ${user.caller_id}">${user.caller_id}</span>
                            </div>
                        </div>
                    </div>
                    `;
                }).join('');
                
                safeSetHTML(container, newHTML);
            } else {
                setContainerCentered(container);
                safeSetHTML(container, renderVPNError(data.error || 'Failed to load VPN sessions'));
            }
        } else {
            let errorMsg = 'Gateway API unreachable (Status: ' + res.status + ')';
            try {
                const errData = await res.json();
                if (errData && errData.error) {
                    errorMsg = errData.error;
                }
            } catch (e) { }
            setContainerCentered(container);
            safeSetHTML(container, renderVPNError(errorMsg));
        }
    } catch (err) {
        setContainerCentered(container);
        safeSetHTML(container, renderVPNError('Network error connecting to Gateway API'));
    }
    
    lastVPNHtml = container.innerHTML;
}

function setContainerCentered(container) {
    if (container) {
        container.style.display = 'flex';
        container.style.flexDirection = 'column';
        container.style.justifyContent = 'center';
    }
}

function renderVPNError(msg) {
    return `
    <div style="display:flex; flex-direction:column; align-items:center; justify-content:center; flex:1; width:100%; min-height:100px; padding:18px 12px; text-align:center; background:rgba(239, 68, 68, 0.05); border:1px solid rgba(239, 68, 68, 0.18); border-radius:10px; box-sizing:border-box;">
        <div style="position:relative; display:flex; align-items:center; justify-content:center; width:36px; height:36px; background:rgba(239, 68, 68, 0.12); border-radius:50%; margin-bottom:8px; box-shadow:0 0 12px rgba(239, 68, 68, 0.25);">
            <i class="fas fa-plug-circle-xmark" style="font-size:16px; color:#f87171;"></i>
        </div>
        <span style="font-size:12px; font-weight:700; color:#f87171; letter-spacing:0.4px;">MIKROTIK GATEWAY OFFLINE</span>
        <span style="font-size:10.5px; color:#94a3b8; margin-top:3px;" title="${msg}">API (10.10.60.2:8728) unreachable</span>
    </div>
    `;
}
