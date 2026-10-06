// VPN Users Component
// Menampilkan daftar pengguna VPN yang sedang aktif

import { formatUpTime } from '../../../shared/utils/helpers.js';
import { safeSetHTML } from '../stats.js';

function getEl(id) {
    return document.getElementById(id);
}

let lastVPNFetchTime = 0;

export function initVPNUsers() {
    // Poll VPN users setiap 10 detik saat dipanggil dari updateDashboardStats
    window.addEventListener('metrics-updated', () => {
        if (Date.now() - lastVPNFetchTime > 10000) {
            updateVPNUser();
            lastVPNFetchTime = Date.now();
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
                    container.innerHTML = '<div style="display:flex; flex-direction:column; align-items:center; justify-content:center; padding:16px 8px; color:var(--text-muted); gap:6px;"><i class="fas fa-user-shield" style="font-size:18px; color:var(--accent-blue); opacity:0.75;"></i><span style="font-size:11px; font-weight:500;">Tidak ada sesi OpenVPN aktif</span></div>';
                    return;
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
                container.innerHTML = '<div style="font-size:12px; color:var(--accent-red); text-align:center; padding:20px;">' + (data.error || 'Gagal memuat data') + '</div>';
            }
        } else {
            let errorMsg = 'Gagal terhubung ke API (status: ' + res.status + ')';
            try {
                const errData = await res.json();
                if (errData && errData.error) {
                    errorMsg = errData.error;
                }
            } catch (e) {
                // Ignore JSON parse errors for non-JSON responses
            }
            container.innerHTML = '<div style="font-size:12px; color:var(--accent-red); text-align:center; padding:20px;">' + errorMsg + '</div>';
        }
    } catch (err) {
        container.innerHTML = '<div style="font-size:12px; color:var(--accent-red); text-align:center; padding:20px;">Gagal mengambil data VPN (Network Error)</div>';
    }
}
