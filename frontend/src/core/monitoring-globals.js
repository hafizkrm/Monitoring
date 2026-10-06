// Global switchView fallback definition to ensure inline onclick handlers never throw ReferenceError
window.switchView = window.switchView || function(view) {
    if (typeof window._switchViewImpl === 'function') {
        return window._switchViewImpl(view);
    }
    const navItem = document.querySelector(`.nav-item[data-view="${view}"]`);
    if (navItem) {
        navItem.click();
    }
};

(function fixCachedTooltipBug() {
    const oldTooltip = document.getElementById('global-tooltip');
    if (oldTooltip) oldTooltip.remove();
    
    const newTooltip = document.createElement('div');
    newTooltip.id = 'new-global-tooltip';
    newTooltip.style.cssText = 'display: none; position: absolute; z-index: 9999; background: rgba(15,23,42,0.95); border: 1px solid rgba(255,255,255,0.1); backdrop-filter: blur(8px); padding: 6px 12px; border-radius: 6px; font-size: 11px; font-weight: 500; color: #fff; box-shadow: 0 4px 12px rgba(0,0,0,0.5); pointer-events: none; transition: opacity 0.15s ease, transform 0.1s ease; transform: translateY(0); white-space: nowrap; opacity: 0;';
    document.body.appendChild(newTooltip);

    document.addEventListener('mouseover', (e) => {
        const target = e.target.closest('[data-tooltip], [title]');
        if (!target) return;
        
        if (target.hasAttribute('title') && target.getAttribute('title').trim() !== '') {
            target.setAttribute('data-tooltip', target.getAttribute('title'));
            target.removeAttribute('title');
        }
        
        const text = target.getAttribute('data-tooltip');
        if (text) {
            newTooltip.innerHTML = text;
            
            const positionTooltip = (evt) => {
                newTooltip.style.left = (evt.pageX + 15) + 'px';
                newTooltip.style.top = (evt.pageY + 15) + 'px';
            };
            positionTooltip(e);
            
            newTooltip.style.display = 'block';
            void newTooltip.offsetWidth;
            newTooltip.style.opacity = '1';
            newTooltip.style.transform = 'translateY(-4px)';
            
            const moveHandler = (evt) => positionTooltip(evt);
            target.addEventListener('mousemove', moveHandler);
            
            target.addEventListener('mouseleave', () => {
                newTooltip.style.opacity = '0';
                newTooltip.style.transform = 'translateY(0)';
                setTimeout(() => {
                    if (newTooltip.style.opacity === '0') newTooltip.style.display = 'none';
                }, 150);
                target.removeEventListener('mousemove', moveHandler);
            }, { once: true });
        }
    });
})();

        /* ---- SECTION 6: Alerts ---- */
        let _currentAlertFilter = 'Semua';
        let _monAlertCurrentPage = 1;
        const _monAlertPageLimit = 10;
        let _cachedAlertsList = null;

        window.monChangeAlertPage = function (page) {
            _monAlertCurrentPage = page;
            if (_cachedAlertsList) {
                window.monRenderAlerts(_cachedAlertsList);
            } else {
                window.monRenderAlerts();
            }
        };

        window.monFilterAlerts = function (category, btn) {
            if (btn) {
                const parent = btn.parentElement;
                parent.querySelectorAll('.mon-filter-chip').forEach(el => el.classList.remove('active'));
                btn.classList.add('active');
            }
            _currentAlertFilter = category;
            _monAlertCurrentPage = 1;
            if (_cachedAlertsList) {
                window.monRenderAlerts(_cachedAlertsList);
            } else {
                window.monRenderAlerts();
            }
        };

        window.monRenderAlerts = async function(precomputedAlerts = null) {
            let alerts = precomputedAlerts;
            
            if (!alerts) {
                try {
                    const res = await fetch('/api/alerts');
                    if (res.ok) {
                        const rawAlerts = await res.json();
                        alerts = (rawAlerts || []).map(a => {
                            let icon = 'fa-exclamation-triangle';
                            let type = a.type || 'warning';
                            if (type === 'danger' || type === 'critical') { type = 'critical'; icon = 'fa-power-off'; }
                            else if (type === 'success') { type = 'success'; icon = 'fa-check-circle'; }
                            else {
                                type = 'warning';
                                if ((a.message||'').toLowerCase().includes('cpu')) icon = 'fa-microchip';
                                if ((a.message||'').toLowerCase().includes('interface')) icon = 'fa-network-wired';
                            }
                            let devName = a.device_name;
                            if (!devName || devName === 'Unknown' || devName === 'Device') {
                                const matchName = (a.message || '').match(/Perangkat\s+([^\s(]+)/i);
                                if (matchName && matchName[1]) {
                                    devName = matchName[1];
                                } else {
                                    devName = 'Perangkat';
                                }
                            }
                            let alertIP = a.device_ip || '';
                            if (!alertIP && a.message) {
                                const matchIP = a.message.match(/\b(?:\d{1,3}\.){3}\d{1,3}\b/);
                                if (matchIP) alertIP = matchIP[0];
                            }
                            const alertDate = new Date(a.created_at);
                            const exactTime = isNaN(alertDate) ? '' : alertDate.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) + ' WIB';
                            const exactDate = isNaN(alertDate) ? '' : alertDate.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
                            
                            const diffSec = isNaN(alertDate) ? 0 : Math.floor((new Date() - alertDate) / 1000);
                            let durationAgo = 'baru saja';
                            if (diffSec >= 86400) {
                                durationAgo = `${Math.floor(diffSec / 86400)} hari ${Math.floor((diffSec % 86400) / 3600)} jam lalu`;
                            } else if (diffSec >= 3600) {
                                durationAgo = `${Math.floor(diffSec / 3600)} jam ${Math.floor((diffSec % 3600) / 60)} mnt lalu`;
                            } else if (diffSec >= 60) {
                                durationAgo = `${Math.floor(diffSec / 60)} mnt lalu`;
                            } else if (diffSec > 0) {
                                durationAgo = `${diffSec} dtk lalu`;
                            }

                            let titleText = `${devName} Terputus`;
                            if (type === 'warning') titleText = `Peringatan: ${devName}`;
                            else if (type === 'success') titleText = `Terselesaikan: ${devName}`;
                            else if ((a.message || '').toLowerCase().includes('offline') || type === 'critical') titleText = `${devName} Terputus`;
                            else titleText = `Pemberitahuan ${devName}`;

                            return {
                                id: a.id || '',
                                type: type, icon: icon, title: titleText,
                                desc: a.message, 
                                Time: exactDate ? `${exactDate}, ${exactTime}` : (a.created_at || ''),
                                durationAgo: durationAgo,
                                ip: alertIP,
                                isRead: !!a.is_read
                            };
                        });
                    } else {
                        alerts = [];
                    }
                } catch(e) {
                    console.error("monRenderAlerts fetch error:", e);
                    alerts = [];
                }
            }

            const SeverityWeight = { 'critical': 2, 'warning': 1 };
            alerts.sort((a, b) => SeverityWeight[b.type] - SeverityWeight[a.type]);
            _cachedAlertsList = alerts;

            const criticalCount = alerts.filter(a => a.type === 'critical').length;
            const warningCount = alerts.filter(a => a.type === 'warning').length;

            const filteredAlerts = alerts.filter(a => {
                if (_currentAlertFilter === 'Semua') return true;
                return a.type.toLowerCase() === _currentAlertFilter.toLowerCase();
            });

            // ── Update stat card & navbar badge ──
            const statAlerts = document.getElementById('stat-alerts');
            if (statAlerts) statAlerts.textContent = alerts.length;
            const topBadge = document.getElementById('top-alert-badge');
            if (topBadge) {
                topBadge.textContent = alerts.length;
                topBadge.style.display = alerts.length > 0 ? 'flex' : 'none';
            }

            // ── Update Alert page badge & Summary Stat Cards ──
            const pageCountEl = document.getElementById('alert-page-count');
            if (pageCountEl) pageCountEl.textContent = alerts.length;

            const elSumTotal = document.getElementById('alert-summary-total');
            const elSumCrit = document.getElementById('alert-summary-critical');
            const elSumWarn = document.getElementById('alert-summary-warning');
            if (elSumTotal) elSumTotal.textContent = alerts.length;
            if (elSumCrit) elSumCrit.textContent = criticalCount;
            if (elSumWarn) elSumWarn.textContent = warningCount;

            // Update filter chip count labels
            const chipAll = document.getElementById('chip-alert-all');
            const chipCrit = document.getElementById('chip-alert-critical');
            const chipWarn = document.getElementById('chip-alert-warning');
            if (chipAll) chipAll.textContent = `Semua (${alerts.length})`;
            if (chipCrit) chipCrit.textContent = `🔴 Kritis (${criticalCount})`;
            if (chipWarn) chipWarn.textContent = `🟡 Peringatan (${warningCount})`;

            const sidebarAlertBadge = document.getElementById('sidebar-alert-badge');
            if (sidebarAlertBadge) {
                sidebarAlertBadge.textContent = alerts.length;
                sidebarAlertBadge.style.display = alerts.length > 0 ? 'inline-block' : 'none';
            }

            // ── Build alert item HTML ──
            const emptyHtml = `<div class="mon-alert-empty" style="text-align: center; padding: 24px; color: var(--text-muted); font-size: 12px;"><i class="fas fa-check-circle" style="font-size: 20px; color: var(--accent-green); margin-bottom: 8px; display: block;"></i>Tidak ada alert untuk kategori ini</div>`;

            const buildHtml = (dataList, isDashboard = false) => {
                if (!dataList.length) return emptyHtml;

                return dataList.map(a => {
                    if (isDashboard) {
                        const miniAction = a.ip ? `
                            <div style="display: flex; gap: 5px; align-items: center; margin-top: 6px; flex-wrap: wrap;">
                                <button type="button" onclick="navigator.clipboard.writeText('${a.ip}'); if(window.showToast) window.showToast('IP ${a.ip} berhasil disalin');" title="Copy IP ${a.ip}" style="background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.15); color: #e2e8f0; font-size: 10px; font-weight: 500; padding: 3px 8px; border-radius: 5px; cursor: pointer; transition: all 0.2s;" onmouseover="this.style.background='rgba(59,130,246,0.25)'; this.style.borderColor='#3b82f6';" onmouseout="this.style.background='rgba(255,255,255,0.06)'; this.style.borderColor='rgba(255,255,255,0.15)';"><i class="fas fa-copy" style="margin-right: 4px; color: #60a5fa;"></i> Copy IP</button>
                                <button type="button" onclick="window.runPing('${a.ip}')" title="Ping ${a.ip}" style="background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.15); color: #e2e8f0; font-size: 10px; font-weight: 500; padding: 3px 8px; border-radius: 5px; cursor: pointer; transition: all 0.2s;" onmouseover="this.style.background='rgba(34,197,94,0.25)'; this.style.borderColor='#22c55e';" onmouseout="this.style.background='rgba(255,255,255,0.06)'; this.style.borderColor='rgba(255,255,255,0.15)';"><i class="fas fa-network-wired" style="margin-right: 4px; color: #4ade80;"></i> Ping</button>
                                <button type="button" onclick="window.runTrace('${a.ip}')" title="Tracert ${a.ip}" style="background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.15); color: #e2e8f0; font-size: 10px; font-weight: 500; padding: 3px 8px; border-radius: 5px; cursor: pointer; transition: all 0.2s;" onmouseover="this.style.background='rgba(139,92,246,0.25)'; this.style.borderColor='#8b5cf6';" onmouseout="this.style.background='rgba(255,255,255,0.06)'; this.style.borderColor='rgba(255,255,255,0.15)';"><i class="fas fa-route" style="margin-right: 4px; color: #a78bfa;"></i> Tracert</button>
                            </div>
                        ` : '';

                        const borderLeftColor = a.type === 'critical' ? 'var(--accent-red)' : 'var(--accent-orange)';
                        const iconColor = a.type === 'critical' ? 'var(--accent-red)' : 'var(--accent-orange)';
                        const iconBg = a.type === 'critical' ? 'rgba(239, 68, 68, 0.15)' : 'rgba(249, 115, 22, 0.15)';
                        const readBadge = a.isRead ? `<span style="font-size: 9px; padding: 1px 5px; border-radius: 4px; background: rgba(34,197,94,0.12); color: #4ade80; border: 1px solid rgba(34,197,94,0.25); font-weight: 500; margin-left: 6px;"><i class="fas fa-check"></i> Dibaca</span>` : '';

                        return `
                        <div style="display: flex; align-items: flex-start; gap: 10px; padding: 10px; margin-bottom: 8px; background: ${a.isRead ? 'rgba(255,255,255,0.015)' : 'rgba(255,255,255,0.035)'}; border-left: 3.5px solid ${borderLeftColor}; border-radius: 8px; border: 1px solid rgba(255,255,255,0.06); border-left-color: ${borderLeftColor}; opacity: ${a.isRead ? '0.8' : '1'};">
                            <div style="width: 26px; height: 26px; border-radius: 50%; background: ${iconBg}; color: ${iconColor}; display: flex; align-items: center; justify-content: center; flex-shrink: 0; font-size: 11px; margin-top: 2px;">
                                <i class="fas ${a.icon}"></i>
                            </div>
                            <div style="display: flex; flex-direction: column; gap: 4px; min-width: 0; flex: 1;">
                                <div style="display: flex; justify-content: space-between; align-items: center; gap: 6px;">
                                    <div style="font-size: 11.5px; font-weight: 700; color: #f8fafc; display: flex; align-items: center; min-width: 0;">
                                        <span style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" title="${a.title}">${a.title}</span>${readBadge}
                                    </div>
                                    <span style="font-size: 9.5px; color: var(--text-muted); white-space: nowrap; font-family: monospace; opacity: 0.85; flex-shrink: 0;">${a.Time}</span>
                                </div>
                                <div style="font-size: 11px; color: #cbd5e1; line-height: 1.35; word-break: break-word; font-weight: 400;" title="${a.desc}">${a.desc}</div>
                                ${miniAction}
                            </div>
                        </div>
                        `;
                    }

                    const readButtonOrBadge = a.isRead
                        ? `<span style="background:rgba(255,255,255,0.04); border:1px solid rgba(255,255,255,0.12); color:var(--text-muted); font-size:11px; padding:4px 8px; border-radius:6px; display:inline-flex; align-items:center; gap:4px;"><i class="fas fa-check-double" style="color:#4ade80;"></i> Dibaca</span>`
                        : (a.id ? `<button onclick="window.dismissAlert('${a.id}', this)" class="btn-sm inline-util-44" style="background:rgba(34,197,94,0.1); border:1px solid rgba(34,197,94,0.3); color:#4ade80; font-size:11px; padding:4px 8px; border-radius:6px; cursor:pointer;" onmouseover="this.style.background='rgba(34,197,94,0.25)';" onmouseout="this.style.background='rgba(34,197,94,0.1);'"><i class="fas fa-check"></i> Tandai Dibaca</button>` : '');

                    // Always render Copy IP / Ping / Tracert — disable when no IP
                    const btnCopyIp = a.ip
                        ? `<button onclick="navigator.clipboard.writeText('${a.ip}'); if(window.showToast) window.showToast('IP ${a.ip} berhasil disalin');" class="btn-sm inline-util-44" title="Copy IP ${a.ip}" style="background:rgba(255,255,255,0.05); border:1px solid var(--border-color); color:var(--text-main); font-size:11px; padding:4px 8px; border-radius:6px; cursor:pointer;" onmouseover="this.style.background='var(--accent-blue)';" onmouseout="this.style.background='rgba(255,255,255,0.05)';"><i class="fas fa-copy"></i> Copy IP</button>`
                        : `<button disabled title="IP tidak tersedia" style="background:rgba(255,255,255,0.03); border:1px solid rgba(255,255,255,0.08); color:rgba(255,255,255,0.25); font-size:11px; padding:4px 8px; border-radius:6px; cursor:not-allowed;"><i class="fas fa-copy"></i> Copy IP</button>`;
                    const btnPing = a.ip
                        ? `<button onclick="window.runPing('${a.ip}')" class="btn-sm inline-util-44" title="Ping ${a.ip}" style="background:rgba(255,255,255,0.05); border:1px solid var(--border-color); color:var(--text-main); font-size:11px; padding:4px 8px; border-radius:6px; cursor:pointer;" onmouseover="this.style.background='var(--accent-blue)'; this.style.borderColor='var(--accent-blue)';" onmouseout="this.style.background='rgba(255,255,255,0.05)'; this.style.borderColor='var(--border-color)';"><i class="fas fa-network-wired"></i> Ping</button>`
                        : `<button disabled title="IP tidak tersedia" style="background:rgba(255,255,255,0.03); border:1px solid rgba(255,255,255,0.08); color:rgba(255,255,255,0.25); font-size:11px; padding:4px 8px; border-radius:6px; cursor:not-allowed;"><i class="fas fa-network-wired"></i> Ping</button>`;
                    const btnTracert = a.ip
                        ? `<button onclick="window.runTrace('${a.ip}')" class="btn-sm inline-util-44" title="Tracert ${a.ip}" style="background:rgba(255,255,255,0.05); border:1px solid var(--border-color); color:var(--text-main); font-size:11px; padding:4px 8px; border-radius:6px; cursor:pointer;" onmouseover="this.style.background='var(--accent-purple)'; this.style.borderColor='var(--accent-purple)';" onmouseout="this.style.background='rgba(255,255,255,0.05)'; this.style.borderColor='var(--border-color)';"><i class="fas fa-route"></i> Tracert</button>`
                        : `<button disabled title="IP tidak tersedia" style="background:rgba(255,255,255,0.03); border:1px solid rgba(255,255,255,0.08); color:rgba(255,255,255,0.25); font-size:11px; padding:4px 8px; border-radius:6px; cursor:not-allowed;"><i class="fas fa-route"></i> Tracert</button>`;

                    const ActionHtml = `<div style="display: flex; gap: 6px; margin-top: 8px; flex-wrap: wrap; justify-content: flex-end;">
                        ${btnCopyIp}
                        ${btnPing}
                        ${btnTracert}
                        ${readButtonOrBadge}
                    </div>`;


                    return `<div class="mon-alert-item ${a.type}" style="${a.isRead ? 'opacity: 0.8;' : ''}">
                        <div class="mon-alert-icon ${a.type}"><i class="fas ${a.icon}"></i></div>
                        <div class="mon-alert-body">
                            <div class="mon-alert-title">${a.title}</div>
                            <div class="mon-alert-desc">${a.desc}</div>
                        </div>
                        <div class="mon-alert-Time" style="display: flex; flex-direction: column; align-items: flex-end; gap: 3px;">
                            <span style="font-size: 11px; font-weight: 600; color: var(--text-main);">${a.Time}</span>
                            <span style="font-size: 10px; color: var(--accent-orange); background: rgba(245,158,11,0.12); border: 1px solid rgba(245,158,11,0.25); padding: 2px 7px; border-radius: 4px; font-weight: 500;"><i class="far fa-clock" style="margin-right: 4px;"></i>${(a.durationAgo && a.durationAgo !== 'undefined') ? a.durationAgo : (a.Time || 'baru saja')}</span>
                            ${ActionHtml}
                        </div>
                    </div>`;
                }).join('');
            };

            // ── Render: Alert & Notifikasi page with Pagination ──
            const pageList = document.getElementById('alerts-page-list');
            if (pageList) {
                const totalAlerts = filteredAlerts.length;
                const totalPages = Math.ceil(totalAlerts / _monAlertPageLimit) || 1;
                if (_monAlertCurrentPage > totalPages) _monAlertCurrentPage = totalPages;

                const startIdx = (_monAlertCurrentPage - 1) * _monAlertPageLimit;
                const paginatedAlerts = filteredAlerts.slice(startIdx, startIdx + _monAlertPageLimit);

                pageList.innerHTML = buildHtml(paginatedAlerts);

                // Add pagination controls if more than 1 page
                if (totalPages > 1) {
                    let pageHtml = '<div style="display: flex; justify-content: center; gap: 8px; margin-top: 24px; margin-bottom: 8px;">';

                    pageHtml += `<button onclick="window.monChangeAlertPage(${Math.max(1, _monAlertCurrentPage - 1)})" class="btn-sm inline-util-45" ${_monAlertCurrentPage === 1 ? 'disabled style="opacity:0.5;cursor:not-allowed;"' : 'onmouseover="this.style.background=\'rgba(255,255,255,0.1)\'" onmouseout="this.style.background=\'rgba(255,255,255,0.05)\'"'}>Prev</button>`;

                    // Display max 5 page numbers to prevent overflow
                    let startPage = Math.max(1, _monAlertCurrentPage - 2);
                    let endPage = Math.min(totalPages, startPage + 4);
                    if (endPage - startPage < 4) startPage = Math.max(1, endPage - 4);

                    if (startPage > 1) {
                        pageHtml += `<button onclick="window.monChangeAlertPage(1)" class="btn-sm inline-util-46">1</button>`;
                        if (startPage > 2) pageHtml += `<span class="inline-util-47">...</span>`;
                    }

                    for (let i = startPage; i <= endPage; i++) {
                        const activeStyle = i === _monAlertCurrentPage
                            ? 'background: var(--accent-blue); border-color: var(--accent-blue); color: white;'
                            : 'background: rgba(255,255,255,0.05); border-color: var(--border-color); color: var(--text-main);';
                        pageHtml += `<button onclick="window.monChangeAlertPage(${i})" class="btn-sm" style="${activeStyle} font-size: 12px; cursor: pointer; padding: 6px 12px; border-radius: 6px; transition: all 0.2s;" ${i !== _monAlertCurrentPage ? 'onmouseover="this.style.background=\'rgba(255,255,255,0.1)\'" onmouseout="this.style.background=\'rgba(255,255,255,0.05)\'"' : ''}>${i}</button>`;
                    }

                    if (endPage < totalPages) {
                        if (endPage < totalPages - 1) pageHtml += `<span class="inline-util-47">...</span>`;
                        pageHtml += `<button onclick="window.monChangeAlertPage(${totalPages})" class="btn-sm inline-util-46">${totalPages}</button>`;
                    }

                    pageHtml += `<button onclick="window.monChangeAlertPage(${Math.min(totalPages, _monAlertCurrentPage + 1)})" class="btn-sm inline-util-45" ${_monAlertCurrentPage === totalPages ? 'disabled style="opacity:0.5;cursor:not-allowed;"' : 'onmouseover="this.style.background=\'rgba(255,255,255,0.1)\'" onmouseout="this.style.background=\'rgba(255,255,255,0.05)\'"'}>Next</button>`;
                    pageHtml += '</div>';

                    pageList.innerHTML += pageHtml;
                }
            }

            // ── Render: Dashboard "Alert Terbaru" card (max 5) ──
            const dashList = document.getElementById('alerts-list-container');
            if (dashList) {
                dashList.innerHTML = buildHtml(alerts.slice(0, 5), true);
            }
        };

        // Mobile Sidebar Toggle
        const btnMobileMenu = document.getElementById('btn-mobile-menu');
        const btnSidebarToggle = document.getElementById('btn-sidebar-toggle');
        const sidebar = document.querySelector('.sidebar');
        const overlay = document.getElementById('sidebar-overlay');

        function toggleSidebar() {
            const isOpen = sidebar.classList.toggle('open');
            overlay.classList.toggle('active');
            [btnMobileMenu, btnSidebarToggle].forEach(btn => {
                if (btn) {
                    btn.setAttribute('aria-expanded', isOpen);
                    btn.setAttribute('aria-label', isOpen ? 'Close navigation menu' : 'Open navigation menu');
                    const icon = btn.querySelector('i');
                    if (icon) icon.className = isOpen ? 'fas fa-times' : 'fas fa-bars';
                }
            });
        }

        if (btnMobileMenu) btnMobileMenu.addEventListener('click', toggleSidebar);
        if (btnSidebarToggle) btnSidebarToggle.addEventListener('click', toggleSidebar);
        if (overlay) overlay.addEventListener('click', toggleSidebar);

        // Close sidebar when clicking a nav item on mobile
        document.querySelectorAll('.nav-item').forEach(item => {
            item.addEventListener('click', () => {
                if (window.innerWidth <= 768) {
                    toggleSidebar();
                }
            });
        });

        // Close sidebar on Escape key
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && sidebar && sidebar.classList.contains('open')) {
                toggleSidebar();
            }
        });

        // 1-Click Alert Mark as Read with smooth animation
        window.dismissAlert = async function(id, btnElement) {
            try {
                const card = btnElement ? (btnElement.closest('.mon-alert-item') || btnElement.closest('.mon-alert-card') || btnElement.closest('div[style*="display: flex"]')) : null;
                if (btnElement) {
                    btnElement.disabled = true;
                    btnElement.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Dibaca...';
                }
                if (card) {
                    card.style.transition = 'all 0.3s ease';
                    card.style.opacity = '0';
                    card.style.transform = 'translateY(-10px)';
                }
                await fetch('/api/alerts/read?id=' + id);
                if (window.showToast) window.showToast('Alert berhasil ditandai dibaca');

                setTimeout(() => {
                    if (card && card.parentNode) {
                        card.parentNode.removeChild(card);
                    }
                    if (typeof window.monRenderAlerts === 'function') {
                        window.monRenderAlerts();
                    }
                }, 300);
            } catch(e) {
                console.error('Dismiss alert error:', e);
            }
        };
