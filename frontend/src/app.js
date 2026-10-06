// Main Application Entry Point
// Network Monitoring System - Modular Architecture

import '@fortawesome/fontawesome-free/css/all.min.css';
import { initRouter } from './core/router.js';
import { startPolling } from './core/services/polling.service.js';
import { startClock } from './core/services/datetime.service.js';
import { requestNotificationPermission } from './core/services/notification.service.js';
import { initRealTime } from './core/services/realtime.service.js';
import { showToast } from './shared/components/toast.js';
import { getCurrentUser, logout, isAuthenticated } from './core/services/auth.service.js';
import { canAccess } from './core/services/permission.service.js';
import { initEnterpriseFeatures } from './core/enterprise.js';
import { initDeviceDetail } from './modules/devices/deviceDetail.js';
import { isTSDBAvailable } from './core/services/tsdb.service.js';
import './core/services/audio.service.js';
import './shared/components/terminal.js';
import { initNocFeatures } from './shared/components/noc.js';
import './modules/devices/deviceModal.js';

// Expose toast to window globally
window.showToast = showToast;

// Chart.js is now imported dynamically in dashboard.js
// Initialize application
async function initApp() {
    // Check Auth
    if (!isAuthenticated()) {
        window.location.href = 'login.html';
        return;
    }

    // Apply Permissions to UI
    applyPermissions();
    updateUserProfile();

    // Init Enterprise UI Features
    initEnterpriseFeatures();
    initDeviceDetail();

    // Bind Logout (support multiple buttons, e.g., sidebar and top nav)
    const logoutBtns = document.querySelectorAll('.btn-logout');
    logoutBtns.forEach(btn => {
        btn.addEventListener('click', () => {
            logout();
            window.location.href = 'login.html';
        });
    });

    // Top Navbar Profile Dropdown Logic
    const profileContainer = document.getElementById('nav-profile-container');
    const profileDropdown = document.getElementById('nav-profile-dropdown');
    
    if (profileContainer && profileDropdown) {
        profileContainer.addEventListener('click', (e) => {
            e.stopPropagation(); // prevent document click from firing immediately
            const isVisible = profileDropdown.style.display === 'flex';
            profileDropdown.style.display = isVisible ? 'none' : 'flex';
        });

        // Close dropdown when clicking outside
        document.addEventListener('click', (e) => {
            if (!profileContainer.contains(e.target)) {
                profileDropdown.style.display = 'none';
            }
        });
    }

    // Wait for Chart.js to load
    await new Promise(resolve => {
        const checkChart = () => window.Chart ? resolve() : setTimeout(checkChart, 100);
        checkChart();
    });

    // Initialize router
    initRouter();

    // Start core services
    startPolling();
    startClock();
    requestNotificationPermission();
    
    // NMS RealTime WebSocket & Snapshot
    initRealTime();

    // Global Search Event
    const globalSearch = document.getElementById('global-search');
    if (globalSearch) {
        globalSearch.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                const query = e.target.value.trim();
                // Switch to devices view
                const devicesNav = document.querySelector('.nav-item[data-view="devices"]');
                if (devicesNav) devicesNav.click();
                
                // Set the value in devices search input and trigger input event
                const devicesSearch = document.getElementById('search-devices');
                if (devicesSearch) {
                    devicesSearch.value = query;
                    devicesSearch.dispatchEvent(new Event('input'));
                }
            }
        });
    }

    // Top Alert Bell Click
    const topAlertBtn = document.getElementById('btn-top-alert');
    if (topAlertBtn) {
        topAlertBtn.addEventListener('click', () => {
            const alertsNav = document.querySelector('.nav-item[data-view="alerts"]');
            if (alertsNav) alertsNav.click();
        });
    }

    // Init NOC Features (TV Fullscreen & Audio Alerts)
    initNocFeatures();

    // Start TSDB Status Poller
    startTSDBStatusPoller();
}

// initNocFeatures imported from noc.js

// Web Audio API Synthesizer logic extracted to audio.service.js

// Function to hide unauthorized menu items
function applyPermissions() {
    const navItems = document.querySelectorAll('.nav-item');
    navItems.forEach(item => {
        const view = item.getAttribute('data-view');
        if (view && !canAccess(view)) {
            item.style.display = 'none';
        }
    });
}

function startTSDBStatusPoller() {
    const updateTSDBBadge = async () => {
        const badge = document.getElementById('tsdb-status-badge');
        const icon = document.getElementById('tsdb-status-icon');
        const text = document.getElementById('tsdb-status-text');
        
        if (!badge || !icon || !text) return;
        
        const isAvailable = await isTSDBAvailable();
        badge.style.display = 'flex'; // Ensure it is visible

        if (isAvailable) {
            badge.style.background = 'rgba(16, 185, 129, 0.12)';
            badge.style.borderColor = 'rgba(16, 185, 129, 0.3)';
            badge.style.color = 'var(--accent-green)';
            icon.className = 'fas fa-database';
            icon.style.color = 'var(--accent-green)';
            text.innerText = 'TSDB AKTIF';
            badge.title = 'Prometheus Time-Series Database Terhubung';
        } else {
            badge.style.background = 'rgba(239, 68, 68, 0.12)';
            badge.style.borderColor = 'rgba(239, 68, 68, 0.3)';
            badge.style.color = 'var(--accent-red)';
            icon.className = 'fas fa-exclamation-triangle';
            icon.style.color = 'var(--accent-red)';
            text.innerText = 'TSDB OFFLINE';
            badge.title = 'Gagal Terhubung ke Prometheus TSDB (Fallback ke SQL)';
        }
    };

    // Initial check and set interval (every 60 seconds)
    updateTSDBBadge();
    setInterval(updateTSDBBadge, 60000);
}

// Function to update User profile in top navbar
function updateUserProfile() {
    const User = getCurrentUser();
    if (!User) return;
    
    // Find the elements assuming specific structure or add IDs to them in index.html later.
    // We'll target them via DOM traversal for now based on index.html structure.
    const UserProfileDiv = document.querySelector('.top-navbar-controls > div:last-child > div:last-child');
    if (UserProfileDiv && UserProfileDiv.classList.contains('inline-util-18')) {
        const spans = UserProfileDiv.querySelectorAll('span');
        if (spans.length >= 2) {
            spans[0].textContent = User.name;
            spans[1].textContent = User.Role.charAt(0).toUpperCase() + User.Role.slice(1);
        }
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', initApp);

// Device Modal Handlers extracted to deviceModal.js

// Terminal diagnostic functions extracted to terminal.js