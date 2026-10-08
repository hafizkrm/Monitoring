        /* ---- SECTION 6: Alerts ---- */
        let _currentAlertFilter = 'All';
        let _monAlertCurrentPage = 1;
        const _monAlertPageLimit = 10;
        let _cachedAlertsList = null;
        let _rawAlertsList = null;

        window.resolveSingleAlert = async function(id, btnElement) {
            try {
                if (btnElement) {
                    btnElement.disabled = true;
                    btnElement.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Resolving...';
                }

                let targetIds = [id];
                if (_cachedAlertsList) {
                    const item = _cachedAlertsList.find(a => String(a.id) === String(id));
                    if (item && item.subAlerts && item.subAlerts.length > 0) {
                        targetIds = item.subAlerts.map(sub => sub.id).filter(Boolean);
                    }
                }

                await Promise.all(targetIds.map(targetId => fetch('/api/alerts/read?id=' + targetId, { method: 'POST' }).catch(() => {})));
                if (window.showToast) window.showToast('Incident marked as resolved', 'success');
                window.monRenderAlerts();
            } catch(e) {
                console.error('Resolve alert error:', e);
                if (btnElement) {
                    btnElement.disabled = false;
                    btnElement.innerHTML = '<i class="fas fa-check"></i> Resolve';
                }
            }
        };

        window.monChangeAlertPage = function (page) {
            _monAlertCurrentPage = page;
            if (_rawAlertsList) {
                window.monRenderAlerts(_rawAlertsList);
            } else {
                window.monRenderAlerts();
            }
        };

        let _currentAlertSearchQuery = '';
        let _currentAlertViewMode = 'matrix'; // Default to matrix as requested
        let _outageTickerInterval = null;

        window.monSearchAlerts = function(query) {
            _currentAlertSearchQuery = (query || '').toLowerCase().trim();
            _monAlertCurrentPage = 1;
            if (_rawAlertsList) {
                window.monRenderAlerts(_rawAlertsList);
            }
        };

        window.monSwitchAlertView = function(mode) {
            _currentAlertViewMode = mode || 'cards';
            const btnCards = document.getElementById('btn-view-cards');
            const btnMatrix = document.getElementById('btn-view-matrix');
            if (btnCards && btnMatrix) {
                if (_currentAlertViewMode === 'cards') {
                    btnCards.classList.add('active');
                    btnMatrix.classList.remove('active');
                } else {
                    btnCards.classList.remove('active');
                    btnMatrix.classList.add('active');
                }
            }
            if (_rawAlertsList) {
                window.monRenderAlerts(_rawAlertsList);
            }
        };

        window.monFilterAlerts = function (category, btn) {
            if (btn) {
                const parent = document.getElementById('alert-summary-filter-bar');
                if (parent) {
                    parent.querySelectorAll('.summary-filter-card').forEach(el => {
                        el.classList.remove('active');
                        el.style.opacity = '0.5';
                        el.style.boxShadow = 'none';
                    });
                    btn.classList.add('active');
                    btn.style.opacity = '1';
                    btn.style.boxShadow = '0 0 10px rgba(255,255,255,0.05)';
                } else {
                    const oldParent = btn.parentElement;
                    if(oldParent) {
                        oldParent.querySelectorAll('.mon-filter-chip').forEach(el => el.classList.remove('active'));
                        btn.classList.add('active');
                    }
                }
            }
            _currentAlertFilter = category;
            _monAlertCurrentPage = 1;
            if (_rawAlertsList) {
                window.monRenderAlerts(_rawAlertsList);
            } else {
                window.monRenderAlerts();
            }
        };

        // NOC Incident Drawer Controls
        window.openNocDrawer = function(alertId) {
            if (!_cachedAlertsList) return;
            const alertItem = _cachedAlertsList.find(a => String(a.id) === String(alertId));
            if (!alertItem) return;

            const drawer = document.getElementById('noc-incident-drawer');
            const backdrop = document.getElementById('noc-drawer-backdrop');
            const titleEl = document.getElementById('noc-drawer-title');
            const badgeEl = document.getElementById('noc-drawer-severity-badge');
            const contentEl = document.getElementById('noc-drawer-content');

            if (!drawer || !contentEl) return;

            const isCrit = alertItem.type === 'critical';
            if (badgeEl) {
                badgeEl.style.background = isCrit ? 'var(--accent-red)' : (alertItem.type === 'resolved' || alertItem.type === 'success' ? 'var(--accent-green)' : 'var(--accent-orange)');
                badgeEl.style.boxShadow = isCrit ? '0 0 10px var(--accent-red)' : (alertItem.type === 'resolved' || alertItem.type === 'success' ? '0 0 10px var(--accent-green)' : '0 0 10px var(--accent-orange)');
            }
            if (titleEl) {
                titleEl.textContent = alertItem.title;
            }

            const elapsedSec = alertItem.createdAtDate ? Math.floor((new Date() - alertItem.createdAtDate) / 1000) : 0;
            const formatOutage = (sec) => {
                if (sec <= 0) return '00h 00m 00s';
                const h = Math.floor(sec / 3600);
                const m = Math.floor((sec % 3600) / 60);
                const s = sec % 60;
                return `${String(h).padStart(2, '0')}h ${String(m).padStart(2, '0')}m ${String(s).padStart(2, '0')}s`;
            };

            const escapeHtml = (unsafe) => String(unsafe || '').replace(/[&<"']/g, m => ({'&': '&amp;', '<': '&lt;', '"': '&quot;', "'": '&#39;'}[m]));
            const subItemsHtml = (alertItem.subAlerts && alertItem.subAlerts.length > 0)
                ? alertItem.subAlerts.map(sub => `
                    <div style="padding: 8px 12px; background: rgba(0,0,0,0.2); border: 1px solid rgba(255,255,255,0.06); border-radius: 6px; font-size: 11px; display: flex; justify-content: space-between; align-items: center;">
                        <span style="color: var(--text-main); font-weight: 500;">${escapeHtml(sub.desc)}</span>
                        <span style="color: var(--text-muted); font-size: 10px;">${escapeHtml(sub.Time)}</span>
                    </div>
                `).join('')
                : `<div style="font-size: 11px; color: var(--text-muted);">No sub-incidents detected. Standalone incident.</div>`;

            contentEl.innerHTML = `
                <!-- Status Banner -->
                <div style="padding: 14px; background: ${isCrit ? 'rgba(239, 68, 68, 0.1)' : (alertItem.type === 'resolved' ? 'rgba(34, 197, 94, 0.1)' : 'rgba(245, 158, 11, 0.1)')}; border: 1px solid ${isCrit ? 'rgba(239, 68, 68, 0.3)' : (alertItem.type === 'resolved' ? 'rgba(34, 197, 94, 0.3)' : 'rgba(245, 158, 11, 0.3)')}; border-radius: 10px; display: flex; justify-content: space-between; align-items: center;">
                    <div>
                        <span style="font-size: 10px; font-weight: 700; text-transform: uppercase; color: ${isCrit ? 'var(--accent-red)' : (alertItem.type === 'resolved' ? 'var(--accent-green)' : 'var(--accent-orange)')}; letter-spacing: 0.5px;">INCIDENT STATUS</span>
                        <div style="font-size: 14px; font-weight: 700; color: var(--text-main); margin-top: 2px;">${escapeHtml(alertItem.title)}</div>
                    </div>
                    <div class="noc-outage-ticker" data-created-at="${alertItem.createdAtIso || ''}">
                        <i class="far fa-clock"></i> DOWN ${formatOutage(elapsedSec)}
                    </div>
                </div>

                <!-- Device Info & IP Card -->
                <div style="padding: 14px; background: rgba(255,255,255,0.02); border: 1px solid var(--border-color); border-radius: 10px; display: flex; flex-direction: column; gap: 8px;">
                    <div style="font-size: 11px; font-weight: 600; color: var(--text-muted); text-transform: uppercase;">Identity &amp; Address</div>
                    <div style="display: flex; justify-content: space-between; align-items: center;">
                        <span style="font-size: 12px; color: var(--text-main); font-weight: 600;">Target IP:</span>
                        <code style="font-size: 12px; background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.3); padding: 2px 8px; border-radius: 4px;">${escapeHtml(alertItem.ip || 'N/A')}</code>
                    </div>
                    <div style="display: flex; justify-content: space-between; align-items: center;">
                        <span style="font-size: 12px; color: var(--text-main); font-weight: 600;">Incident Time:</span>
                        <span style="font-size: 11px; color: var(--text-muted); font-family: monospace;">${escapeHtml(alertItem.Time)} (${escapeHtml(alertItem.durationAgo)})</span>
                    </div>
                </div>

                <!-- Root Cause & Aggregation Tree -->
                <div style="display: flex; flex-direction: column; gap: 8px;">
                    <div style="font-size: 11px; font-weight: 600; color: var(--text-muted); text-transform: uppercase;">RCA &amp; Aggregated Impact Tree</div>
                    <div style="display: flex; flex-direction: column; gap: 6px;">
                        ${subItemsHtml}
                    </div>
                </div>

                <!-- Quick Diagnostic Suite -->
                <div style="padding: 14px; background: rgba(255,255,255,0.02); border: 1px solid var(--border-color); border-radius: 10px; display: flex; flex-direction: column; gap: 10px;">
                    <div style="font-size: 11px; font-weight: 600; color: var(--text-muted); text-transform: uppercase;">NOC Quick Diagnostics</div>
                    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px;">
                        <button type="button" onclick="window.runPing('${escapeHtml(alertItem.ip)}')" class="btn-sm" style="background: rgba(34, 197, 94, 0.15); border: 1px solid rgba(34, 197, 94, 0.3); color: #4ade80; padding: 8px; border-radius: 8px; font-size: 11px; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 6px;"><i class="fas fa-network-wired"></i> Run Ping</button>
                        <button type="button" onclick="window.runTrace('${escapeHtml(alertItem.ip)}')" class="btn-sm" style="background: rgba(139, 92, 246, 0.15); border: 1px solid rgba(139, 92, 246, 0.3); color: #c084fc; padding: 8px; border-radius: 8px; font-size: 11px; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 6px;"><i class="fas fa-route"></i> Run Tracert</button>
                    </div>
                    <button type="button" onclick="navigator.clipboard.writeText('${escapeHtml(alertItem.ip)}'); if(window.showToast) window.showToast('IP Address copied to clipboard');" class="btn-sm" style="background: rgba(255,255,255,0.05); border: 1px solid var(--border-color); color: var(--text-main); padding: 8px; border-radius: 8px; font-size: 11px; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 6px;"><i class="fas fa-copy"></i> Copy IP Address</button>
                    ${alertItem.type !== 'resolved' && alertItem.type !== 'success' ? `<button type="button" onclick="window.resolveSingleAlert('${escapeHtml(alertItem.id)}', this); window.closeNocDrawer();" class="btn-sm" style="background: rgba(34, 197, 94, 0.2); border: 1px solid #22c55e; color: #4ade80; padding: 8px; border-radius: 8px; font-size: 11px; font-weight: 700; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 6px; margin-top: 4px;"><i class="fas fa-check"></i> Resolve Incident</button>` : ''}
                </div>
            `;

            drawer.classList.add('open');
            if (backdrop) backdrop.classList.add('open');
        };

        window.closeNocDrawer = function() {
            const drawer = document.getElementById('noc-incident-drawer');
            const backdrop = document.getElementById('noc-drawer-backdrop');
            if (drawer) drawer.classList.remove('open');
            if (backdrop) backdrop.classList.remove('open');
        };

        // Helper to convert alert message strings from backend to English
        function formatAlertMessageToEnglish(rawMsg) {
            if (!rawMsg) return '';
            let msg = String(rawMsg).trim();

            // 1. Direct Regex Pattern Replacements
            msg = msg.replace(/^Perangkat terputus\s*\(Offline\)$/i, 'Device disconnected (Offline)');
            msg = msg.replace(/^Perangkat terputus$/i, 'Device disconnected (Offline)');
            msg = msg.replace(/^Perangkat kembali online$/i, 'Device reconnected (Online)');
            msg = msg.replace(/^Perangkat pulih$/i, 'Device restored (Online)');

            msg = msg.replace(/^Antarmuka\s+([^\s]+)\s+terputus\s*\(Down\)$/i, 'Interface $1 disconnected (Down)');
            msg = msg.replace(/^Antarmuka\s+([^\s]+)\s+terputus$/i, 'Interface $1 disconnected (Down)');

            msg = msg.replace(/^Terdeteksi latensi ping tinggi:\s*(.+)$/i, 'High ping latency detected: $1');
            msg = msg.replace(/^Koneksi SNMP ke perangkat terganggu\s*\(Degraded\)$/i, 'SNMP connection to device degraded');
            msg = msg.replace(/^Koneksi SNMP ke perangkat terganggu$/i, 'SNMP connection to device degraded');

            msg = msg.replace(/^Terdeteksi kehilangan paket:\s*(.+)$/i, 'Packet loss detected: $1');
            msg = msg.replace(/^Terdeteksi beban CPU tinggi:\s*(.+)$/i, 'High CPU load detected: $1');
            msg = msg.replace(/^Terdeteksi penggunaan memori tinggi:\s*(.+)$/i, 'High memory usage detected: $1');

            msg = msg.replace(/^(\d+)\s+interface terputus pada\s+(.+)\.\s+Klik detail untuk melihat rincian interface\.$/i, '$1 interfaces down on $2. Click detail to inspect interface breakdown.');

            // 2. Substring fallback replacements for mixed phrases
            msg = msg.replace(/\bPerangkat\b/g, 'Device');
            msg = msg.replace(/\bterputus\b/g, 'disconnected');
            msg = msg.replace(/\bterganggu\b/g, 'degraded');
            msg = msg.replace(/\bTerdeteksi\b/g, 'Detected');
            msg = msg.replace(/\bkehilangan paket\b/g, 'packet loss');
            msg = msg.replace(/\blatensi ping tinggi\b/g, 'high ping latency');
            msg = msg.replace(/\bAntarmuka\b/g, 'Interface');
            msg = msg.replace(/\bpada\b/g, 'on');
            msg = msg.replace(/\brincian\b/g, 'details');
            msg = msg.replace(/\bkoneksi\b/g, 'connection');

            return msg;
        }

        window.monRenderAlerts = async function(precomputedAlerts = null) {
            let alerts = precomputedAlerts;
            
            if (!alerts) {
                try {
                    const res = await fetch('/api/alerts');
                    if (res.ok) {
                        const rawAlerts = await res.json();
                        alerts = (rawAlerts || []).map(a => {
                            let icon = 'fa-exclamation-triangle';
                            let rawType = (a.type || 'warning').toLowerCase();
                            const isResolved = !!a.is_read || (a.incident_status && a.incident_status.toLowerCase() === 'resolved') || rawType === 'success' || rawType === 'resolved';
                            
                            let type = 'warning';
                            if (isResolved) {
                                type = 'resolved';
                                icon = 'fa-check-circle';
                            } else if (rawType === 'danger' || rawType === 'critical') {
                                type = 'critical';
                                icon = 'fa-power-off';
                            } else {
                                type = 'warning';
                                if ((a.message||'').toLowerCase().includes('cpu')) icon = 'fa-microchip';
                                if ((a.message||'').toLowerCase().includes('interface')) icon = 'fa-network-wired';
                            }

                            let devName = a.device_name;
                            if (!devName || devName === 'Unknown' || devName === 'Device') {
                                const matchName = (a.message || '').match(/(?:Perangkat|Device)\s+([^\s(]+)/i);
                                if (matchName && matchName[1]) {
                                    devName = matchName[1];
                                } else {
                                    devName = 'Device';
                                }
                            }
                            let alertIP = a.device_ip || '';
                            if (!alertIP && a.message) {
                                const matchIP = a.message.match(/\b(?:\d{1,3}\.){3}\d{1,3}\b/);
                                if (matchIP) alertIP = matchIP[0];
                            }
                            let dateStr = a.created_at;
                            if (dateStr && typeof dateStr === 'string' && dateStr.endsWith('Z')) {
                                dateStr = dateStr.slice(0, -1);
                            }
                            const alertDate = new Date(dateStr);
                            const exactTime = isNaN(alertDate) ? '' : alertDate.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
                            const exactDate = isNaN(alertDate) ? '' : alertDate.toLocaleDateString('en-US', { day: 'numeric', month: 'short', year: 'numeric' });
                            
                            const diffSec = isNaN(alertDate) ? 0 : Math.floor((new Date() - alertDate) / 1000);
                            let durationAgo = 'just now';
                            if (diffSec >= 86400) {
                                durationAgo = `${Math.floor(diffSec / 86400)}d ${Math.floor((diffSec % 86400) / 3600)}h ago`;
                            } else if (diffSec >= 3600) {
                                durationAgo = `${Math.floor(diffSec / 3600)}h ${Math.floor((diffSec % 3600) / 60)}m ago`;
                            } else if (diffSec >= 60) {
                                durationAgo = `${Math.floor(diffSec / 60)}m ago`;
                            } else if (diffSec > 0) {
                                durationAgo = `${diffSec}s ago`;
                            }

                            let titleText = devName;
                            const msgLower = (a.message || '').toLowerCase();
                            if (msgLower.includes('offline')) {
                                titleText = `${devName} Disconnected`;
                            } else if (msgLower.includes('antarmuka') || msgLower.includes('interface')) {
                                titleText = `${devName} Interface Down`;
                            }

                            const formattedDesc = formatAlertMessageToEnglish(a.message);

                            let resDateStr = (a.resolved_at || '');
                            if (resDateStr && resDateStr.endsWith('Z')) {
                                resDateStr = resDateStr.slice(0, -1);
                            }
                            const resDate = resDateStr ? new Date(resDateStr) : null;

                            return {
                                id: a.id || '',
                                type: type, icon: icon, title: titleText,
                                devName: devName,
                                desc: formattedDesc, 
                                Time: exactDate ? `${exactDate}, ${exactTime}` : (a.created_at || ''),
                                durationAgo: durationAgo,
                                ip: alertIP,
                                isRead: isResolved,
                                createdAtDate: isNaN(alertDate) ? new Date() : alertDate,
                                createdAtIso: dateStr || new Date().toISOString(),
                                resolvedAtDate: (!resDate || isNaN(resDate)) ? null : resDate
                            };
                        });
                        _rawAlertsList = alerts;
                    } else {
                        alerts = [];
                        _rawAlertsList = [];
                    }
                } catch(e) {
                    console.error("monRenderAlerts fetch error:", e);
                    alerts = [];
                    _rawAlertsList = [];
                }
            }

            // ── Smart Alert Aggregation (Strictly Separate Active vs Resolved Alerts) ──
            const activeAlerts = alerts.filter(a => a.type !== 'resolved' && a.type !== 'success');
            const resolvedAlerts = alerts.filter(a => a.type === 'resolved' || a.type === 'success');

            // 1. Group Active Alerts per Device IP/Name
            const activeDeviceMap = new Map();
            activeAlerts.forEach(a => {
                let key = (a.ip || a.devName || a.id).toLowerCase().trim();
                if (a.ip) key = a.ip.toLowerCase().trim();

                if (!activeDeviceMap.has(key)) {
                    activeDeviceMap.set(key, { ...a, subAlerts: [a] });
                } else {
                    const group = activeDeviceMap.get(key);
                    group.subAlerts.push(a);

                    const SevPriority = { 'critical': 3, 'warning': 2 };
                    if ((SevPriority[a.type] || 0) > (SevPriority[group.type] || 0)) {
                        group.type = a.type;
                        group.icon = a.icon;
                        group.title = a.title;
                    }
                    if (a.createdAtDate > group.createdAtDate) {
                        group.Time = a.Time;
                        group.durationAgo = a.durationAgo;
                        group.createdAtDate = a.createdAtDate;
                        group.createdAtIso = a.createdAtIso;
                    }
                }
            });

            const processedActiveAlerts = Array.from(activeDeviceMap.values()).map(group => {
                if (group.subAlerts.length > 1) {
                    if (group.type === 'critical') {
                        group.title = `${group.devName} Disconnected (${group.subAlerts.length} Events)`;
                    } else {
                        group.title = `${group.devName} (${group.subAlerts.length} Active Events)`;
                    }
                    const uniqueDescs = Array.from(new Set(group.subAlerts.map(s => s.desc).filter(Boolean)));
                    group.desc = uniqueDescs.join(' • ');
                }
                return group;
            });

            // 2. Group Resolved Alerts per Device IP/Name
            const resolvedDeviceMap = new Map();
            resolvedAlerts.forEach(a => {
                let key = (a.ip || a.devName || a.id).toLowerCase().trim();
                if (a.ip) key = a.ip.toLowerCase().trim();

                if (!resolvedDeviceMap.has(key)) {
                    resolvedDeviceMap.set(key, { ...a, subAlerts: [a] });
                } else {
                    const group = resolvedDeviceMap.get(key);
                    group.subAlerts.push(a);
                    if (a.createdAtDate > group.createdAtDate) {
                        group.Time = a.Time;
                        group.durationAgo = a.durationAgo;
                        group.createdAtDate = a.createdAtDate;
                        group.createdAtIso = a.createdAtIso;
                    }
                }
            });

            const processedResolvedAlerts = Array.from(resolvedDeviceMap.values()).map(group => {
                group.type = 'resolved';
                group.icon = 'fa-check-circle';
                group.isRead = true;
                if (group.subAlerts.length > 1) {
                    group.title = `${group.devName} (${group.subAlerts.length} Resolved Events)`;
                    const uniqueDescs = Array.from(new Set(group.subAlerts.map(s => s.desc).filter(Boolean)));
                    group.desc = uniqueDescs.join(' • ');
                } else {
                    group.title = `${group.devName}`;
                }
                return group;
            });

            const processedAlerts = [...processedActiveAlerts, ...processedResolvedAlerts];

            const SeverityWeight = { 'critical': 3, 'warning': 2, 'resolved': 1, 'success': 1 };
            processedAlerts.sort((a, b) => (SeverityWeight[b.type] || 0) - (SeverityWeight[a.type] || 0) || (b.createdAtDate - a.createdAtDate));
            _cachedAlertsList = processedAlerts;

            const criticalCount = processedActiveAlerts.filter(a => a.type === 'critical').length;
            const warningCount = processedActiveAlerts.filter(a => a.type === 'warning').length;
            const resolvedCount = processedResolvedAlerts.length;
            const unreadCount = processedActiveAlerts.filter(a => !a.isRead).length;

            // Apply Search & Category Filters
            const filteredAlerts = processedAlerts.filter(a => {
                // Category Filter
                if (_currentAlertFilter === 'Critical' && a.type !== 'critical') return false;
                if (_currentAlertFilter === 'Warning' && a.type !== 'warning') return false;
                if (_currentAlertFilter === 'Resolved' && a.type !== 'resolved' && a.type !== 'success') return false;

                // Search Filter
                if (_currentAlertSearchQuery) {
                    const q = _currentAlertSearchQuery;
                    const matchTitle = (a.title || '').toLowerCase().includes(q);
                    const matchDesc = (a.desc || '').toLowerCase().includes(q);
                    const matchIP = (a.ip || '').toLowerCase().includes(q);
                    const matchDev = (a.devName || '').toLowerCase().includes(q);
                    return matchTitle || matchDesc || matchIP || matchDev;
                }
                return true;
            });

            // ── Update stat cards & badges ──
            const statAlerts = document.getElementById('stat-alerts');
            if (statAlerts) statAlerts.textContent = unreadCount;
            const topBadge = document.getElementById('top-alert-badge');
            if (topBadge) {
                topBadge.textContent = unreadCount;
                topBadge.style.display = unreadCount > 0 ? 'flex' : 'none';
            }

            const pageCountEl = document.getElementById('alert-page-count');
            if (pageCountEl) pageCountEl.textContent = processedAlerts.length;

            const elSumTotal = document.getElementById('alert-summary-total');
            const elSumCrit = document.getElementById('alert-summary-critical');
            const elSumWarn = document.getElementById('alert-summary-warning');
            const elSumRes = document.getElementById('alert-summary-resolved');
            if (elSumTotal) elSumTotal.textContent = processedAlerts.length;
            if (elSumCrit) elSumCrit.textContent = criticalCount;
            if (elSumWarn) elSumWarn.textContent = warningCount;
            if (elSumRes) elSumRes.textContent = resolvedCount;

            const sidebarAlertBadge = document.getElementById('sidebar-alert-badge');
            if (sidebarAlertBadge) {
                sidebarAlertBadge.textContent = unreadCount;
                sidebarAlertBadge.style.display = unreadCount > 0 ? 'inline-block' : 'none';
            }

            // ── Build alert HTML based on view mode (Cards vs Matrix Table) ──
            const emptyHtml = `<div class="mon-alert-empty" style="text-align: center; padding: 32px; color: var(--text-muted); font-size: 13px;"><i class="fas fa-check-circle" style="font-size: 28px; color: var(--accent-green); margin-bottom: 10px; display: block;"></i>No alerts matching the selected filter.</div>`;

            const formatOutageStr = (createdDate, resolvedDate) => {
                if (!createdDate || isNaN(createdDate)) return '00h 00m 00s';
                const end = (resolvedDate && !isNaN(resolvedDate)) ? resolvedDate : new Date();
                const sec = Math.max(0, Math.floor((end - createdDate) / 1000));
                const h = Math.floor(sec / 3600);
                const m = Math.floor((sec % 3600) / 60);
                const s = sec % 60;
                return `${String(h).padStart(2, '0')}h ${String(m).padStart(2, '0')}m ${String(s).padStart(2, '0')}s`;
            };

            const buildHtml = (dataList) => {
                const escapeHtml = (unsafe) => String(unsafe || '').replace(/[&<"']/g, m => ({'&': '&amp;', '<': '&lt;', '"': '&quot;', "'": '&#39;'}[m]));
                if (!dataList.length) return emptyHtml;

                if (_currentAlertViewMode === 'matrix') {
                    // Render High-Density NOC Wall Matrix Table
                    const rowsHtml = dataList.map(a => {
                        const sevColor = a.type === 'critical' ? 'var(--accent-red)' : (a.type === 'success' || a.type === 'resolved' ? 'var(--accent-green)' : 'var(--accent-orange)');
                        const sevLabel = a.type === 'critical' ? 'CRITICAL' : (a.type === 'success' || a.type === 'resolved' ? 'RESOLVED' : 'WARNING');
                        const outageTicker = formatOutageStr(a.createdAtDate, a.resolvedAtDate);
                        
                        return `
                        <tr>
                            <td>
                                <span style="font-size: 10px; font-weight: 700; padding: 2px 8px; border-radius: 4px; background: ${sevColor}22; color: ${sevColor}; border: 1px solid ${sevColor}44;">
                                    ${sevLabel}
                                </span>
                            </td>
                            <td>
                                <div style="font-weight: 700; color: var(--text-main); cursor: pointer;" onclick="window.openNocDrawer('${a.id}')">${escapeHtml(a.title)}</div>
                                <code style="font-size: 10px; color: #60a5fa;">${escapeHtml(a.ip || 'No IP')}</code>
                            </td>
                            <td style="max-width: 350px; color: var(--text-muted); line-height: 1.5; font-size: 11.5px;">
                                ${escapeHtml(a.desc).split(' • ').map(item => `<div style="margin-bottom: 2px;"><span style="color: var(--accent-blue); margin-right: 6px; opacity: 0.8;">•</span> ${item}</div>`).join('')}
                            </td>
                            <td>
                                <span class="${a.type === 'resolved' || a.type === 'success' ? 'noc-outage-static' : 'noc-outage-ticker'}" data-created-at="${a.createdAtIso || ''}" data-status="${a.type}" style="${a.type === 'resolved' || a.type === 'success' ? 'background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); animation: none;' : (a.type === 'warning' ? 'background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3);' : '')}">
                                    <i class="${a.type === 'resolved' || a.type === 'success' ? 'fas fa-check-circle' : 'far fa-clock'}"></i> ${a.type === 'resolved' || a.type === 'success' ? 'TOTAL DOWN: ' + outageTicker : 'DOWN ' + outageTicker}
                                </span>
                            </td>
                            <td style="font-family: monospace; font-size: 11px; color: var(--text-muted);">
                                ${escapeHtml(a.Time)}
                            </td>
                            <td style="text-align: right;">
                                <div style="display: flex; gap: 6px; justify-content: flex-end; align-items: center;">
                                    <button type="button" onclick="window.openNocDrawer('${a.id}')" title="Inspect Incident Drawer" style="background: rgba(59, 130, 246, 0.15); border: 1px solid rgba(59, 130, 246, 0.3); color: #60a5fa; font-size: 11px; font-weight: 600; padding: 5px 10px; border-radius: 6px; cursor: pointer; display: inline-flex; align-items: center; gap: 5px; transition: all 0.2s ease;" onmouseover="this.style.background='rgba(59, 130, 246, 0.25)';" onmouseout="this.style.background='rgba(59, 130, 246, 0.15)';"><i class="fas fa-search-plus"></i> Inspect</button>

                                </div>
                            </td>
                        </tr>
                        `;
                    }).join('');

                    return `
                    <div style="overflow-x: auto;">
                        <table class="noc-matrix-table">
                            <thead>
                                <tr>
                                    <th>Severity</th>
                                    <th>Device &amp; IP</th>
                                    <th>Incident Description</th>
                                    <th>Live Outage</th>
                                    <th>Incident Time</th>
                                    <th style="text-align: right;">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                ${rowsHtml}
                            </tbody>
                        </table>
                    </div>
                    `;
                }

                // Render Timeline Stream View Cards (Default)
                return dataList.map(a => {
                    const outageTicker = formatOutageStr(a.createdAtDate, a.resolvedAtDate);
                    const unreadDot = !a.isRead ? `<span style="width: 8px; height: 8px; border-radius: 50%; background: var(--accent-blue); display: inline-block; box-shadow: 0 0 6px var(--accent-blue);"></span>` : '';
                    const groupBadge = a.subAlerts && a.subAlerts.length > 1 ? `<span style="font-size: 10px; padding: 2px 7px; border-radius: 4px; background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.3); font-weight: 600; margin-left: 6px;"><i class="fas fa-layer-group"></i> ${a.subAlerts.length} Events</span>` : '';

                    return `
                    <div class="noc-card-item ${a.type} ${a.isRead ? 'read' : 'unread'}" style="${a.isRead ? 'opacity: 0.85;' : ''}">
                        <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; flex-wrap: wrap;">
                            
                            <!-- Left Block: Title & Device Info -->
                            <div style="display: flex; align-items: flex-start; gap: 12px; flex: 1; min-width: 240px;">
                                <div class="mon-alert-icon ${a.type}" style="margin-top: 2px;">
                                    <i class="fas ${a.icon}"></i>
                                </div>
                                <div style="display: flex; flex-direction: column; gap: 4px;">
                                    <div style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap;">
                                        ${unreadDot}
                                        <h4 style="margin: 0; font-size: 13.5px; font-weight: 700; color: var(--text-main); cursor: pointer;" onclick="window.openNocDrawer('${escapeHtml(a.id)}')">${escapeHtml(a.title)}</h4>
                                        ${groupBadge}
                                    </div>
                                    <div style="display: flex; align-items: center; gap: 8px; font-size: 11px; color: var(--text-muted);">
                                        ${a.ip ? `<span style="background: rgba(255,255,255,0.06); padding: 2px 6px; border-radius: 4px; font-family: monospace; color: #60a5fa; border: 1px solid rgba(255,255,255,0.1);"><i class="fas fa-network-wired" style="margin-right: 4px;"></i>${escapeHtml(a.ip)}</span>` : ''}
                                        <span><i class="far fa-clock" style="margin-right: 4px;"></i>${escapeHtml(a.Time)}</span>
                                    </div>
                                    <div style="font-size: 12px; color: #cbd5e1; margin-top: 6px; line-height: 1.5;">
                                        ${escapeHtml(a.desc).split(' • ').map(item => `<div style="margin-bottom: 2px;"><span style="color: var(--accent-blue); margin-right: 6px; opacity: 0.8;">•</span> ${item}</div>`).join('')}
                                    </div>
                                </div>
                            </div>

                            <!-- Right Block: Outage Timer & Actions -->
                            <div style="display: flex; flex-direction: column; align-items: flex-end; gap: 8px;">
                                <span class="${a.type === 'resolved' || a.type === 'success' ? 'noc-outage-static' : 'noc-outage-ticker'}" data-created-at="${a.createdAtIso || ''}" data-status="${a.type}" style="${a.type === 'resolved' || a.type === 'success' ? 'background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); animation: none;' : (a.type === 'warning' ? 'background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3);' : '')}">
                                    <i class="${a.type === 'resolved' || a.type === 'success' ? 'fas fa-check-circle' : 'far fa-clock'}"></i> ${a.type === 'resolved' || a.type === 'success' ? 'TOTAL DOWN: ' + outageTicker : 'DOWN ' + outageTicker}
                                </span>

                                <div style="display: flex; gap: 6px; flex-wrap: wrap;">
                                    <button type="button" onclick="window.openNocDrawer('${escapeHtml(a.id)}')" title="Inspect Incident Drawer" style="background: rgba(59, 130, 246, 0.15); border: 1px solid rgba(59, 130, 246, 0.3); color: #60a5fa; font-size: 11px; font-weight: 600; padding: 5px 10px; border-radius: 6px; cursor: pointer; display: inline-flex; align-items: center; gap: 5px; transition: all 0.2s ease;" onmouseover="this.style.background='rgba(59, 130, 246, 0.25)';" onmouseout="this.style.background='rgba(59, 130, 246, 0.15)';"><i class="fas fa-search-plus"></i> Inspect</button>
                                    ${a.ip ? `<button type="button" onclick="navigator.clipboard.writeText('${escapeHtml(a.ip)}'); if(window.showToast) window.showToast('IP ${escapeHtml(a.ip)} copied');" title="Copy IP" style="background: rgba(255,255,255,0.05); border: 1px solid var(--border-color); color: var(--text-main); font-size: 11px; font-weight: 500; padding: 5px 8px; border-radius: 6px; cursor: pointer; display: inline-flex; align-items: center; gap: 4px;"><i class="fas fa-copy"></i></button>` : ''}
                                    ${a.ip ? `<button type="button" onclick="window.runPing('${escapeHtml(a.ip)}')" title="Ping ${escapeHtml(a.ip)}" style="background: rgba(34, 197, 94, 0.15); border: 1px solid rgba(34, 197, 94, 0.3); color: #4ade80; font-size: 11px; font-weight: 600; padding: 5px 10px; border-radius: 6px; cursor: pointer; display: inline-flex; align-items: center; gap: 5px;"><i class="fas fa-network-wired"></i> Ping</button>` : ''}
                                    ${a.ip ? `<button type="button" onclick="window.runTrace('${escapeHtml(a.ip)}')" title="Tracert ${escapeHtml(a.ip)}" style="background: rgba(139, 92, 246, 0.15); border: 1px solid rgba(139, 92, 246, 0.3); color: #c084fc; font-size: 11px; font-weight: 600; padding: 5px 10px; border-radius: 6px; cursor: pointer; display: inline-flex; align-items: center; gap: 5px;"><i class="fas fa-route"></i> Tracert</button>` : ''}
                                    ${a.type !== 'resolved' && a.type !== 'success' ? `<button type="button" onclick="window.resolveSingleAlert('${escapeHtml(a.id)}', this)" title="Resolve Incident" style="background: rgba(234, 179, 8, 0.15); border: 1px solid rgba(234, 179, 8, 0.3); color: #facc15; font-size: 11px; font-weight: 600; padding: 5px 10px; border-radius: 6px; cursor: pointer; display: inline-flex; align-items: center; gap: 5px;"><i class="fas fa-check"></i> Resolve</button>` : ''}
                                </div>
                            </div>
                        </div>
                    </div>
                    `;
                }).join('');
            };

            // ── Render: Alert Page ──
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
                    pageHtml += `<button onclick="window.monChangeAlertPage(${Math.max(1, _monAlertCurrentPage - 1)})" class="btn-sm" ${_monAlertCurrentPage === 1 ? 'disabled style="opacity:0.5;cursor:not-allowed;"' : ''}>Prev</button>`;

                    let startPage = Math.max(1, _monAlertCurrentPage - 2);
                    let endPage = Math.min(totalPages, startPage + 4);
                    if (endPage - startPage < 4) startPage = Math.max(1, endPage - 4);

                    for (let i = startPage; i <= endPage; i++) {
                        const activeStyle = i === _monAlertCurrentPage
                            ? 'background: var(--accent-blue); border-color: var(--accent-blue); color: white;'
                            : 'background: rgba(255,255,255,0.05); border-color: var(--border-color); color: var(--text-main);';
                        pageHtml += `<button onclick="window.monChangeAlertPage(${i})" class="btn-sm" style="${activeStyle} font-size: 12px; cursor: pointer; padding: 6px 12px; border-radius: 6px;">${i}</button>`;
                    }

                    pageHtml += `<button onclick="window.monChangeAlertPage(${Math.min(totalPages, _monAlertCurrentPage + 1)})" class="btn-sm" ${_monAlertCurrentPage === totalPages ? 'disabled style="opacity:0.5;cursor:not-allowed;"' : ''}>Next</button>`;
                    pageHtml += '</div>';

                    pageList.innerHTML += pageHtml;
                }
            }

            // ── Live Outage Stopwatch Ticker Interval (Updates every 1s) ──
            if (!_outageTickerInterval) {
                _outageTickerInterval = setInterval(() => {
                    document.querySelectorAll('.noc-outage-ticker[data-created-at]').forEach(el => {
                        const iso = el.getAttribute('data-created-at');
                        const status = el.getAttribute('data-status');
                        if (iso) {
                            const dt = new Date(iso);
                            if (!isNaN(dt)) {
                                const ticker = formatOutageStr(dt);
                                if (status === 'resolved' || status === 'success') {
                                    el.innerHTML = `<i class="far fa-clock"></i> ${ticker}`;
                                } else {
                                    el.innerHTML = `<i class="far fa-clock"></i> DOWN ${ticker}`;
                                }
                            }
                        }
                    });
                }, 1000);
            }

            // ── Render: Dashboard "Alert Terbaru" card (max 5) ──
            const dashList = document.getElementById('alerts-list-container');
            if (dashList) {
                const dashAlerts = processedAlerts.slice(0, 5);
                if (dashAlerts.length === 0) {
                    dashList.innerHTML = `<div style="text-align: center; padding: 20px; color: var(--text-muted); font-size: 12px;"><i class="fas fa-check-circle" style="font-size: 24px; color: var(--accent-green); margin-bottom: 8px; display: block; opacity: 0.5;"></i>Sistem aman. Tidak ada alert aktif.</div>`;
                } else {
                    dashList.innerHTML = dashAlerts.map(a => {
                        const sevColor = a.type === 'critical' ? 'var(--accent-red)' : (a.type === 'success' || a.type === 'resolved' ? 'var(--accent-green)' : 'var(--accent-orange)');
                        const icon = a.type === 'critical' ? 'fa-times-circle' : (a.type === 'success' || a.type === 'resolved' ? 'fa-check-circle' : 'fa-exclamation-triangle');
                        const groupBadge = a.subAlerts && a.subAlerts.length > 1 ? `<span style="font-size: 9px; padding: 1px 5px; border-radius: 4px; background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.3); font-weight: 600; margin-left: 6px;"><i class="fas fa-layer-group"></i> ${a.subAlerts.length} Events</span>` : '';
                        
                        return `
                        <div style="padding: 10px 12px; border-bottom: 1px solid rgba(255,255,255,0.05); display: flex; flex-direction: column; gap: 6px; transition: background 0.2s; cursor: pointer;" onmouseover="this.style.background='rgba(255,255,255,0.02)'" onmouseout="this.style.background='transparent'" onclick="window.openNocDrawer('${a.id}')">
                            <div style="display: flex; justify-content: space-between; align-items: flex-start;">
                                <div style="display: flex; align-items: center; gap: 6px;">
                                    <i class="fas ${icon}" style="color: ${sevColor}; font-size: 13px;"></i>
                                    <span style="font-size: 12px; font-weight: 700; color: var(--text-main);">${a.title}</span>
                                    ${groupBadge}
                                </div>
                                <span style="font-size: 10px; color: var(--text-muted); white-space: nowrap; margin-left: 8px;">${a.Time}</span>
                            </div>
                            <div style="font-size: 11px; color: #cbd5e1; line-height: 1.4; margin-top: 4px;">
                                ${(() => {
                                    const events = (a.desc || '').split(' • ').filter(Boolean);
                                    if (events.length === 0) return '';
                                    const displayEvents = events.slice(0, 3);
                                    const extra = events.length - 3;
                                    const escapeHtml = (unsafe) => String(unsafe || '').replace(/[&<"']/g, m => ({'&': '&amp;', '<': '&lt;', '"': '&quot;', "'": '&#39;'}[m]));
                                    let html = displayEvents.map(e => `<div style="margin-bottom: 4px; line-height: 1.4;"><span style="color: var(--accent-blue); opacity: 0.7; margin-right: 4px;">•</span> ${escapeHtml(e)}</div>`).join('');
                                    if (extra > 0) {
                                        html += `<div style="margin-top: 4px; font-size: 10px; color: #60a5fa; font-weight: 600; font-style: italic; padding-left: 10px;">+ ${extra} more events...</div>`;
                                    }
                                    return html;
                                })()}
                            </div>
                        </div>
                        `;
                    }).join('');
                }
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
                    btnElement.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Reading...';
                }
                if (card) {
                    card.style.transition = 'all 0.3s ease';
                    card.style.opacity = '0';
                    card.style.transform = 'translateY(-10px)';
                }
                await fetch('/api/alerts/read?id=' + id, { method: 'POST' });
                if (window.showToast) window.showToast('Alert marked as read');

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
