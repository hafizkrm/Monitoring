/**
 * switch-ports.js
 * Dynamic SNMP-powered Cisco C9200L switch port visualization
 * Fetches real-Time port data from /api/interfaces and renders it
 */

const POLLING_INTERVAL_MS = 30000; // Refresh every 30 seconds
let switchPollingTimer = null;

/**
 * Map interface name to port number and speed for Cisco C9200L
 */
function parseInterfacePort(name) {
    if (!name) return null;
    
    // Match GigabitEthernet/FastEthernet 1/0/X → regular ports 1-48
    const portMatch = name.match(/(FastEthernet|GigabitEthernet|Fa|Gi)1\/0\/(\d+)/i);
    if (portMatch) {
        const isFast = portMatch[1].toLowerCase().startsWith('f');
        return { type: 'port', speed: isFast ? 'fast' : 'gigabit', num: parseInt(portMatch[2]) };
    }
    
    // Match GigabitEthernet/TenGigabitEthernet 1/1/X → SFP uplinks
    const sfpMatch = name.match(/(GigabitEthernet|TenGigabitEthernet|Gi|Te)1\/1\/(\d+)/i);
    if (sfpMatch) {
        const isTen = sfpMatch[1].toLowerCase().startsWith('t');
        return { type: 'sfp', speed: isTen ? 'ten-gigabit' : 'gigabit', num: parseInt(sfpMatch[2]) };
    }

    return null;
}

/**
 * Build the switch HTML from interface data
 */
function buildSwitchHTML(portStatuses, sfpStatuses, portSpeeds, sfpSpeeds, portDescriptions, sfpDescriptions, portNames, sfpNames) {
    function getPortClass(portNum) {
        const s = portStatuses[portNum];
        if (!s) return 'off';         // no data = grey
        if (s === 'up') return 'up';   // green
        if (s === 'warning') return 'warning'; // yellow
        if (s === 'down') return 'down'; // red/grey
        return 'off';
    }

    function getSfpClass(sfpNum) {
        const s = sfpStatuses[sfpNum];
        if (!s) return 'off';
        if (s === 'up') return 'up';
        if (s === 'warning') return 'warning';
        if (s === 'down') return 'down';
        return 'off';
    }

    function portLed(num, isTop) {
        const cls = getPortClass(num);
        const speed = portSpeeds[num] || 'gigabit'; // default gigabit
        let ledColor, ledShadow, ledBlink = false;
        
        if (cls === 'up') {
            if (speed === 'gigabit') {
                ledColor = '#22c55e';
                ledShadow = '0 0 6px #22c55e';
                ledBlink = true;
            } else {
                ledColor = '#f97316';
                ledShadow = '0 0 6px #f97316';
                ledBlink = false;
            }
        } else if (cls === 'warning') {
            ledColor = '#eab308';
            ledShadow = '0 0 6px #eab308';
            ledBlink = true; // Warning always blinks
        } else if (cls === 'down') {
            ledColor = '#ef4444';
            ledShadow = 'none';
        } else {
            ledColor = '#374151';
            ledShadow = 'none';
        }

        const ledPos = isTop ? 'top: 3px; left: 3px;' : 'bottom: 3px; right: 3px;';
        const blinkClass = ledBlink ? 'class="led-blink"' : '';
        const ledStyle = `position: absolute; ${ledPos} width: 9px; height: 9px; border-radius: 50%; background: ${ledColor}; box-shadow: ${ledShadow};`;
        const speedTitle = (portNames && portNames[num]) ? portNames[num] : (speed === 'fast' ? 'FastEthernet ' + num : 'GigabitEthernet ' + num);
        const desc = (portDescriptions && portDescriptions[num]) ? portDescriptions[num] : cls;
        let displayLabel = num;
        if (desc !== cls && desc !== '') {
            displayLabel = `${num}<br><span style="font-size: 7px; opacity: 0.8; letter-spacing: 0;">${desc}</span>`;
        }
        const label = `<div class="port-label" style="max-width: 35px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; text-align: center; line-height: 1.1;" title="${speedTitle} - ${desc}">${displayLabel}</div>`;
        const socket = `<div title="${speedTitle} - ${desc}" style="width: 35px; height: 35px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.18); border-radius: 5px; position: relative; transition: all 0.2s ease;">
            <div ${blinkClass} style="${ledStyle}"></div>
        </div>`;
        return `<div style="display: flex; flex-direction: column; align-items: center; gap: 4px; cursor: help;">${isTop ? label + socket : socket + label}</div>`;
    }

    function portBlock(startPort, isTop) {
        let html = '<div style="display: flex; gap: 5px; justify-content: space-between;">';
        for (let i = 0; i < 6; i++) {
            html += portLed(startPort + (i * 2), isTop);
        }
        html += '</div>';
        return html;
    }

    function sfpPort(num) {
        const cls = getSfpClass(num);
        const speed = sfpSpeeds[num] || 'gigabit';
        let ledColor = '#374151', ledShadow = 'none', blinkClass = '';
        
        if (cls === 'up') {
            ledColor = '#22c55e';
            ledShadow = '0 0 8px #22c55e';
            blinkClass = 'class="led-blink"';
        } else if (cls === 'warning') {
            ledColor = '#eab308';
            ledShadow = '0 0 8px #eab308';
            blinkClass = 'class="led-blink"';
        } else if (cls === 'down') {
            ledColor = '#ef4444';
        }
        
        const speedTitle = (sfpNames && sfpNames[num]) ? sfpNames[num] : (speed === 'ten-gigabit' ? '10G SFP+ ' + num : '1G SFP ' + num);
        const desc = (sfpDescriptions && sfpDescriptions[num]) ? sfpDescriptions[num] : cls;
        let displayLabel = `SFP ${num}`;
        if (desc !== cls && desc !== '') {
            displayLabel = `SFP ${num}<br><span style="font-size: 8px; opacity: 0.8; letter-spacing: 0;">${desc}</span>`;
        }
        return `<div style="display: flex; flex-direction: column; align-items: center; gap: 8px; cursor: help;" title="${speedTitle} - ${desc}">
            <div style="width: 44px; height: 30px; background: rgba(255,255,255,0.07); border: 1px solid rgba(255,255,255,0.18); position: relative; border-radius: 5px;">
                <div ${blinkClass} style="position: absolute; bottom: -11px; left: 17px; width: 9px; height: 9px; border-radius: 50%; background: ${ledColor}; box-shadow: ${ledShadow};"></div>
            </div>
            <div style="font-size: 10px; color: rgba(255,255,255,0.65); font-weight: 600; margin-top: 8px; text-align: center; max-width: 44px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; line-height: 1.1;">${displayLabel}</div>
        </div>`;
    }

    // Build top row (odd ports: 1,3,5...47) in blocks of 12
    let topRowHTML = '<div style="display: flex; gap: 8px; justify-content: space-between; width: 100%;">';
    for (let b = 0; b < 4; b++) {
        topRowHTML += portBlock(1 + b * 12, true);
    }
    topRowHTML += '</div>';

    // Build bottom row (even ports: 2,4,6...48)
    let botRowHTML = '<div style="display: flex; gap: 8px; justify-content: space-between; width: 100%;">';
    for (let b = 0; b < 4; b++) {
        botRowHTML += portBlock(2 + b * 12, false);
    }
    botRowHTML += '</div>';

    // SFP ports
    let sfpHTML = `<div style="display: flex; gap: 12px; align-items: center;">`;
    for (let i = 1; i <= 4; i++) {
        sfpHTML += sfpPort(i);
    }
    sfpHTML += '</div>';

    return { topRowHTML, botRowHTML, sfpHTML };
}

/**
 * Fetch interface data from the API and update the switch visualization
 */
async function updateSwitchPorts(switchIP) {
	if (!switchIP) return;

	// Only fetch if the switch ports UI is actually in the DOM (User is on Dashboard)
	const portsContainer = document.getElementById('switch-ports-grid');
	if (!portsContainer) return;

	try {
		const res = await fetch(`/api/interfaces?ip=${encodeURIComponent(switchIP)}`);
        if (!res.ok) {
            console.warn('[SwitchPorts] API error:', res.status);
            return;
        }
        const interfaces = await res.json();
        if (!Array.isArray(interfaces) || interfaces.length === 0) {
            console.warn('[SwitchPorts] No interface data returned');
            return;
        }

        // Map port number → Status
        const portStatuses = {};
        const sfpStatuses = {};
        const portSpeeds = {};
        const sfpSpeeds = {};
        const portDescriptions = {};
        const sfpDescriptions = {};
        const portNames = {};
        const sfpNames = {};

        for (const iface of interfaces) {
            const parsed = parseInterfacePort(iface.name);
            if (!parsed) continue;
            
            // Determine negotiated speed based on API data if available
            let negotiatedSpeed = parsed.speed; // Fallback to name-based speed
            if (iface.speed) {
                const speedBps = parseInt(iface.speed, 10);
                if (speedBps > 0 && speedBps <= 100000000) { 
                    // 10 Mbps or 100 Mbps
                    negotiatedSpeed = 'fast';
                } else if (speedBps === 1000000000) {
                    // 1 Gbps
                    negotiatedSpeed = 'gigabit';
                } else if (speedBps >= 10000000000) {
                    // 10 Gbps
                    negotiatedSpeed = 'ten-gigabit';
                }
            }

            const desc = iface.alias || iface.status;

            if (parsed.type === 'port') {
                portStatuses[parsed.num] = iface.status; 
                portSpeeds[parsed.num] = negotiatedSpeed;
                portDescriptions[parsed.num] = desc;
                portNames[parsed.num] = iface.name;
            } else if (parsed.type === 'sfp') {
                sfpStatuses[parsed.num] = iface.status;
                sfpSpeeds[parsed.num] = negotiatedSpeed;
                sfpDescriptions[parsed.num] = desc;
                sfpNames[parsed.num] = iface.name;
            }
        }

        console.log(`[SwitchPorts] Fetched ${interfaces.length} interfaces. Ports up: ${Object.values(portStatuses).filter(s => s === 'up').length}, SFPs up: ${Object.values(sfpStatuses).filter(s => s === 'up').length}`);

        // Render
        const portsContainer = document.getElementById('switch-ports-grid');
        const sfpContainer = document.getElementById('switch-sfp-ports');
        const lastUpdated = document.getElementById('switch-last-updated');

        if (!portsContainer || !sfpContainer) {
            console.warn('[SwitchPorts] Container elements not found in DOM');
            return;
        }

        const { topRowHTML, botRowHTML, sfpHTML } = buildSwitchHTML(portStatuses, sfpStatuses, portSpeeds, sfpSpeeds, portDescriptions, sfpDescriptions, portNames, sfpNames);
        portsContainer.innerHTML = topRowHTML + botRowHTML;
        sfpContainer.innerHTML = sfpHTML;

        // Build port summary list
        const summaryContainer = document.getElementById('switch-port-summary');
        if (summaryContainer) {
            const summaryHTML = `
                <div style="display: flex; flex-direction: column; gap: 5px; font-size: 11px;">
                    <div style="display: flex; align-items: center; gap: 6px;">
                        <div style="width: 9px; height: 9px; background: #22c55e; border-radius: 2px; box-shadow: 0 0 5px rgba(34,197,94,0.5); flex-shrink: 0;"></div>
                        <span style="color: var(--text-main);">Gigabit (1000 Mbps)</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 6px;">
                        <div style="width: 9px; height: 9px; background: #f97316; border-radius: 2px; flex-shrink: 0;"></div>
                        <span style="color: var(--text-main);">Fast Ether (10/100 Mbps)</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 6px;">
                        <div style="width: 9px; height: 9px; background: #ef4444; border-radius: 2px; box-shadow: 0 0 5px rgba(239,68,68,0.5); flex-shrink: 0;"></div>
                        <span style="color: #ef4444; font-weight: 600;">Port Down / Link Error</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 6px;">
                        <div style="width: 9px; height: 9px; background: #eab308; border-radius: 2px; flex-shrink: 0;"></div>
                        <span style="color: #eab308;">Warning / High Latency</span>
                    </div>
                    <div style="display: flex; align-items: center; gap: 6px;">
                        <div style="width: 9px; height: 9px; background: #374151; border-radius: 2px; flex-shrink: 0;"></div>
                        <span style="color: var(--text-muted);">Tidak Terhubung / Off</span>
                    </div>
                </div>
            `;
            
            summaryContainer.innerHTML = summaryHTML;
        }

        if (lastUpdated) {
            const now = new Date();
            lastUpdated.textContent = `Diperbarui: ${now.toLocaleTimeString('id-ID')}`;
        }

    } catch (err) {
        console.error('[SwitchPorts] Fetch failed:', err);
    }
}

/**
 * Setup horizontal drag-to-scroll, wheel scroll, and button controls for Cisco Switch
 */
export function setupSwitchScroll() {
    const container = document.getElementById('cisco-switch-scroll-container');
    if (!container) return;

    if (container.dataset.scrollInit) return;
    container.dataset.scrollInit = 'true';

    // 1. Mouse wheel horizontal scrolling
    container.addEventListener('wheel', (e) => {
        if (e.deltaY !== 0) {
            e.preventDefault();
            container.scrollLeft += e.deltaY * 1.5;
        }
    }, { passive: false });

    // 2. Mouse drag-to-scroll (grab and drag)
    let isDown = false;
    let startX, scrollLeft;

    container.addEventListener('mousedown', (e) => {
        isDown = true;
        container.style.cursor = 'grabbing';
        startX = e.pageX - container.offsetLeft;
        scrollLeft = container.scrollLeft;
    });

    container.addEventListener('mouseleave', () => {
        isDown = false;
        container.style.cursor = 'grab';
    });

    container.addEventListener('mouseup', () => {
        isDown = false;
        container.style.cursor = 'grab';
    });

    container.addEventListener('mousemove', (e) => {
        if (!isDown) return;
        e.preventDefault();
        const x = e.pageX - container.offsetLeft;
        const walk = (x - startX) * 1.8;
        container.scrollLeft = scrollLeft - walk;
    });

    // 3. Arrow buttons
    const btnLeft = document.getElementById('btn-switch-scroll-left');
    const btnRight = document.getElementById('btn-switch-scroll-right');

    if (btnLeft) {
        btnLeft.addEventListener('click', (e) => {
            e.preventDefault();
            container.scrollBy({ left: -250, behavior: 'smooth' });
        });
    }

    if (btnRight) {
        btnRight.addEventListener('click', (e) => {
            e.preventDefault();
            container.scrollBy({ left: 250, behavior: 'smooth' });
        });
    }
}

/**
 * Find the Cisco switch device from the metrics list and start polling
 */
export async function initSwitchPortsFromMetrics(metrics) {
    setupSwitchScroll();
    if (!metrics || !Array.isArray(metrics)) return;

    // Find first switch device
    const switchDevice = metrics.find(m => {
        const type = (m.device_type || m.type || '').toLowerCase();
        return type === 'switch';
    });

    if (!switchDevice) {
        console.log('[SwitchPorts] No switch device found in metrics');
        const portsContainer = document.getElementById('switch-ports-grid');
        if (portsContainer) {
            portsContainer.innerHTML = '<div style="font-size: 12px; color: rgba(255,255,255,0.6); padding-left:10px; margin-top:20px; line-height:1.5;">Belum ada perangkat Switch (contoh: Cisco C9200L) yang ditambahkan ke sistem.<br>Silakan tambah perangkat dengan tipe "Switch" pada menu <b>Devices</b> agar data port dapat dimuat.</div>';
        }
        return;
    }

    const ip = switchDevice.ip_address || switchDevice.ip;
    if (!ip) {
        console.warn('[SwitchPorts] Switch found but no IP Address');
        return;
    }

    const portsContainer = document.getElementById('switch-ports-grid');
    if (!portsContainer) return; // Don't initialize if DOM is not ready!

    // Update model label in UI if available
    const modelLabel = document.getElementById('switch-model-label');
    if (modelLabel && switchDevice.model) {
        modelLabel.textContent = `Cisco ${switchDevice.model}`;
    }
    const hostnameLabel = document.getElementById('switch-hostname-label');
    if (hostnameLabel && switchDevice.hostname) {
        hostnameLabel.textContent = switchDevice.hostname;
    }

    window.currentSwitchIp = ip;
    window.switchPortsInitialized = true;

    // Initial fetch
    await updateSwitchPorts(ip);

    // Poll periodically
    if (switchPollingTimer) clearInterval(switchPollingTimer);
    switchPollingTimer = setInterval(() => updateSwitchPorts(ip), POLLING_INTERVAL_MS);

    console.log(`[SwitchPorts] Polling started for switch at ${ip} every ${POLLING_INTERVAL_MS/1000}s`);
}

export function stopSwitchPortsPolling() {
    if (switchPollingTimer) {
        clearInterval(switchPollingTimer);
        switchPollingTimer = null;
    }
}
