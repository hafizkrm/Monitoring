// Router Module (Zero-Latency Static Pre-bundled Module Loader & View Isolation)

import { VIEW_TITLES, VIEW_SUBTITLES } from './constants.js';
import { setState, getState } from './state.js';
import { canAccess } from './services/permission.service.js';
import { isAuthenticated } from './services/auth.service.js';

import dashboardHtml from '../modules/dashboard/dashboard.html?raw';
import devicesHtml from '../modules/devices/devices.html?raw';
import alertsHtml from '../modules/alerts/alerts.html?raw';
import ReportsHtml from '../modules/reports/reports.html?raw';
import logsHtml from '../modules/logs/logs.html?raw';
import SettingsHtml from '../modules/settings/settings.html?raw';
import UserHtml from '../modules/users/users.html?raw';
import backupHtml from '../modules/backup/backup.html?raw';

import { initDevices, fetchDevicesTable } from '../modules/devices/devices.js';
import { initAlerts } from '../modules/alerts/alerts.js';
import { initReports } from '../modules/reports/reports.js';
import { initLogs } from '../modules/logs/logs.js';
import { initSettings } from '../modules/settings/settings.js';
import { initUser, refreshUsers } from '../modules/users/users.js';
import { initBackup } from '../modules/backup/backup.js';

const MODULE_REGISTRY = {
    dashboard: { html: dashboardHtml },
    devices: { html: devicesHtml, init: initDevices, fetch: fetchDevicesTable },
    alerts: { html: alertsHtml, init: initAlerts },
    Reports: { html: ReportsHtml, init: initReports },
    logs: { html: logsHtml, init: initLogs },
    Settings: { html: SettingsHtml, init: initSettings },
    User: { html: UserHtml, init: initUser, fetch: refreshUsers },
    backup: { html: backupHtml, init: initBackup }
};

const htmlCache = {};

function ensureModuleLoaded(view) {
    let container = document.getElementById('module-container');
    if (!container) {
        const mainContent = document.getElementById('main-content');
        if (mainContent) {
            container = document.createElement('div');
            container.id = 'module-container';
            container.style.cssText = 'display: flex; flex-direction: column; position: relative; flex: 1; min-height: 0; width: 100%; height: 100%; overflow: hidden;';
            mainContent.appendChild(container);
            document.querySelectorAll('.content-section').forEach(s => s.remove());
        }
    }
    if (!container) return null;

    let viewWrapper = container.querySelector(`.view-wrapper[data-view="${view}"]`);
    if (viewWrapper && htmlCache[view]) {
        return viewWrapper;
    }

    if (!viewWrapper) {
        viewWrapper = document.createElement('div');
        viewWrapper.className = 'view-wrapper';
        viewWrapper.setAttribute('data-view', view);
        viewWrapper.style.cssText = 'display: none; flex-direction: column; position: absolute; top: 0; left: 0; width: 100%; height: 100%; overflow: hidden; background: var(--bg-body, #0b0f19); z-index: 1;';
        container.appendChild(viewWrapper);
    }

    if (!htmlCache[view]) {
        const reg = MODULE_REGISTRY[view];
        if (reg && reg.html) {
            viewWrapper.innerHTML = reg.html;
            const injectedSection = viewWrapper.querySelector('.content-section');
            if (injectedSection) {
                injectedSection.style.display = injectedSection.id === 'view-dashboard' ? 'flex' : 'block';
                injectedSection.style.height = '100%';
            }
            if (typeof reg.init === 'function') {
                try { reg.init(); } catch (e) { console.warn(`Init error for ${view}:`, e); }
            }
            htmlCache[view] = true;
        } else {
            viewWrapper.innerHTML = `<div style="padding: 40px; color: #ef4444; text-align: center;">Modul <b>${view}</b> belum tersedia.</div>`;
        }
    }
    return viewWrapper;
}

function preloadAllModules() {
    // Only pre-mount modules the current role may open; otherwise admin-only
    // init hooks (e.g. initUser) would run for viewer accounts.
    Object.keys(MODULE_REGISTRY).filter(canAccess).forEach(view => {
        try {
            ensureModuleLoaded(view);
        } catch (e) {
            console.warn(`Preload failed for ${view}:`, e);
        }
    });
}

import { subscribeToTopics, unsubscribeFromTopics } from './services/realtime.service.js';

// View switching function
export async function switchView(view) {
    if (!isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    if (!canAccess(view)) {
        console.warn(`Access denied to view: ${view}`);
        window.showToast?.('Anda tidak memiliki akses ke halaman ini', 'error');
        if (view !== 'dashboard') {
            switchView('dashboard');
        }
        return;
    }

    // We no longer subscribe to 'all' on websocket to avoid full table render issues.
    // Dashboard and Devices now rely on HTTP polling via polling.service.js

    const navItems = document.querySelectorAll('.nav-item');
    
    // Track previous view
    const currentActive = document.querySelector('.nav-item.active');
    if (currentActive) {
        const currentView = currentActive.getAttribute('data-view');
        if (currentView !== 'device-details') {
            setState('previousView', currentView);
        }
    }
    
    // Update active nav item
    navItems.forEach(item => {
        if (item.getAttribute('data-view') === view) {
            item.classList.add('active');
        } else {
            item.classList.remove('active');
        }
    });
    
    // Update title and subtitle
    const titleEl = document.getElementById('view-title');
    if (titleEl) {
        titleEl.innerText = VIEW_TITLES[view] || 'Network Monitor';
    }
    const subtitleEl = document.getElementById('view-subtitle');
    if (subtitleEl) {
        subtitleEl.innerText = VIEW_SUBTITLES[view] || 'Sistem Monitoring Jaringan';
    }

    try {
        const viewWrapper = ensureModuleLoaded(view);
        
        let container = document.getElementById('module-container');
        if (container) {
            // Hide all existing view wrappers COMPLETELY to prevent ghosting/flashlight effect
            const allWrappers = container.querySelectorAll('.view-wrapper');
            allWrappers.forEach(w => {
                if (w.style.display !== 'none' && typeof window.cleanupCurrentView === 'function') {
                    window.cleanupCurrentView(w.getAttribute('data-view'));
                }
                w.style.display = 'none';
                w.style.opacity = '0';
                w.style.pointerEvents = 'none';
                w.style.zIndex = '1';
                w.style.visibility = 'hidden';
            });
            
            if (viewWrapper) {
                viewWrapper.style.display = 'flex';
                viewWrapper.style.visibility = 'visible';
                viewWrapper.style.opacity = '1';
                viewWrapper.style.pointerEvents = 'auto';
                viewWrapper.style.zIndex = '10';
            }
            
            triggerViewHooks(view);
            return true;
        }
    } catch (e) {
        console.error("Gagal memuat modul:", e);
        return false;
    }
}

// Trigger view-specific data loading hooks
function triggerViewHooks(view) {
    if (view === 'dashboard') {
        if (typeof window.resizeDashboardCharts === 'function') {
            setTimeout(() => window.resizeDashboardCharts(), 50);
        }
    }
    if (view === 'devices') {
        if (typeof MODULE_REGISTRY.devices.fetch === 'function') {
            MODULE_REGISTRY.devices.fetch();
        } else if (window.fetchDevicesTable) {
            window.fetchDevicesTable();
        }
    }
    if (view === 'alerts') {
        if (window.fetchAlertsTable) window.fetchAlertsTable();
        if (window.monRenderAlerts) window.monRenderAlerts();
    }
    if (view === 'logs') {
        if (window.switchLogsTab) {
            window.switchLogsTab('system');
        } else if (window.fetchLogs) {
            window.fetchLogs();
        }
    }
    if (view === 'monitoring') {
        window.monInitView && window.monInitView();
    }
    if (view === 'Reports') {
        // Only auto-generate if no data loaded yet; otherwise re-use cached data
        if (window.generateReport && !window._ReportsDataLoaded) {
            window._ReportsDataLoaded = true;
            window.generateReport();
        }
    }
    if (view === 'User') {
        MODULE_REGISTRY.User.fetch();
    }
    if (view === 'Settings') {
        if (window.refreshSettingsData) window.refreshSettingsData();
    }
}


// Expose switchView globally to window object
window._switchViewImpl = switchView;
window.switchView = switchView;

// Initialize navigation
export function initRouter() {
    const navItems = document.querySelectorAll('.nav-item');
    
    navItems.forEach(item => {
        item.addEventListener('click', async (e) => {
            if (item.classList.contains('has-submenu')) return;
            e.stopPropagation();
            
            const view = item.getAttribute('data-view');
            if (!view) return;
            
            await switchView(view);
        });
    });

    // Default view on load
    setTimeout(() => {
        const defaultView = document.querySelector('.nav-item.active') || document.querySelector('.nav-item[data-view="dashboard"]');
        if (defaultView) {
            defaultView.click();
        }
        // Synchronously mount & pre-cache all remaining modules for instant tab switching
        setTimeout(() => preloadAllModules(), 100);
    }, 50);
}

// Go back to previous view
export function goBackToPreviousView() {
    const view = getState('previousView');
    if (view) {
        document.querySelector(`[data-view="${view}"]`)?.click();
    }
}