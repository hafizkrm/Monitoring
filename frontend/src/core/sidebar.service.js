// Sidebar Service - Centralized Sidebar Logic, Event Delegation & Badge Rendering
import { switchView } from '../core/router.js';
import { sidebarStore } from './state/store.js';

// Must match the mobile breakpoint in shared/css/inline.css (@media max-width: 768px)
const MOBILE_BREAKPOINT = 768;

/**
 * Toggle sidebar visibility
 * @param {boolean} [forceState] - Force open/close state
 */
export function toggleSidebar(forceState) {
    const sidebar = document.querySelector('.sidebar');
    const overlay = document.getElementById('sidebar-overlay');
    if (!sidebar) return;

    const isCurrentlyOpen = sidebar.classList.contains('open');
    const shouldOpen = forceState !== undefined ? forceState : !isCurrentlyOpen;

    sidebar.classList.toggle('open', shouldOpen);
    overlay?.classList.toggle('active', shouldOpen);

    document.querySelectorAll('#btn-mobile-menu, #btn-sidebar-toggle').forEach(btn => {
        btn.setAttribute('aria-expanded', String(shouldOpen));
    });
}

/**
 * Render sidebar badges as pure function of state
 */
function renderBadge(elementId, count) {
    const badge = document.getElementById(elementId);
    if (!badge) return;
    badge.textContent = count;
    badge.style.display = count > 0 ? 'inline-block' : 'none';
}

function renderServerStatus(isOnline) {
    const status = document.getElementById('sidebar-server-status');
    if (!status) return;
    const text = status.querySelector('.status-text');
    const dot = status.querySelector('.live-dot');
    if (text) {
        text.textContent = isOnline ? 'Online' : 'Offline';
        text.style.color = isOnline ? 'var(--accent-green)' : 'var(--accent-red)';
    }
    if (dot) {
        dot.style.background = isOnline ? 'var(--accent-green)' : 'var(--accent-red)';
        dot.style.boxShadow = isOnline ? '' : '0 0 10px var(--accent-red)';
    }
}

/**
 * Track backend reachability for the sidebar server status indicator
 */
function initServerStatusWatcher() {
    renderServerStatus(true);

    window.addEventListener('auth-connection-lost', () => renderServerStatus(false));
    window.addEventListener('auth-session-restored', () => renderServerStatus(true));

    const probe = async () => {
        try {
            const res = await fetch('/api/session', { credentials: 'same-origin', cache: 'no-store' });
            // Any HTTP response means the backend is reachable, even 401.
            renderServerStatus(res.status < 500);
        } catch {
            renderServerStatus(false);
        }
    };
    probe();
    setInterval(probe, 60000);
}

/**
 * Initialize Sidebar event delegation
 */
export function initSidebarEvents() {
    const sidebar = document.querySelector('.sidebar');
    if (!sidebar) return;

    // Delegated Click Handler
    sidebar.addEventListener('click', (e) => {
        const item = e.target.closest('.nav-item');
        if (!item || item.classList.contains('has-submenu')) return;

        e.stopPropagation();
        const view = item.getAttribute('data-view');
        if (view) switchView(view);

        // Mobile auto-close
        if (window.innerWidth <= MOBILE_BREAKPOINT) {
            toggleSidebar(false);
        }
    });

    // Keyboard Handler (A11y Scope 7)
    sidebar.addEventListener('keydown', (e) => {
        const item = e.target.closest('.nav-item');
        if (!item) return;

        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            item.click();
            return;
        }

        if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
            const items = [...sidebar.querySelectorAll('.nav-item')].filter(i => i.style.display !== 'none');
            if (!items.length) return;
            const currentIndex = items.indexOf(item);
            const nextIndex = e.key === 'ArrowDown'
                ? (currentIndex + 1) % items.length
                : (currentIndex - 1 + items.length) % items.length;
            items[nextIndex].focus();
        }
    });

    // Mobile & Desktop triggers
    document.getElementById('sidebar-overlay')?.addEventListener('click', () => toggleSidebar(false));
    document.getElementById('btn-mobile-menu')?.addEventListener('click', () => toggleSidebar());
    document.getElementById('btn-sidebar-toggle')?.addEventListener('click', () => toggleSidebar());

    // Global Escape Handler
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && document.querySelector('.sidebar')?.classList.contains('open')) {
            toggleSidebar(false);
        }
    });

    // Resize guard: drop mobile state when returning to desktop width
    window.addEventListener('resize', () => {
        if (window.innerWidth > MOBILE_BREAKPOINT) {
            toggleSidebar(false);
        }
    });

    // Badge rendering from state (Scope 5).
    // Seed first: monitoring-alerts.js is a separate module script and may have
    // written the counters before initApp() awaited verifySession(), so the
    // first setState() notifications fired with zero subscribers attached.
    const renderDeviceBadge = (storeState) => {
        renderBadge('sidebar-device-badge', storeState.deviceCount);
        const badge = document.getElementById('sidebar-device-badge');
        if (badge && storeState.deviceStatusSummary) {
            badge.title = storeState.deviceStatusSummary;
        }
    };

    sidebarStore.subscribe('alertCount', (count) => renderBadge('sidebar-alert-badge', count));
    sidebarStore.subscribe('deviceCount', (count, storeState) => renderDeviceBadge(storeState));
    renderBadge('sidebar-alert-badge', sidebarStore.getState().alertCount);
    renderDeviceBadge(sidebarStore.getState());

    initServerStatusWatcher();
}