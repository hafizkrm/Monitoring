import { getStorageItem, setStorageItem, STORAGE_KEYS } from './services/storage.service.js';

export function initEnterpriseFeatures() {
    initThemeToggle();
    initCommandPalette();
}

function initThemeToggle() {
    const toggleBtn = document.getElementById('btn-theme-toggle');
    const themeIcon = document.getElementById('theme-icon');
    if (!toggleBtn || !themeIcon) return;

    // Load Saved theme
    const SavedTheme = getStorageItem(STORAGE_KEYS.THEME) || 'dark';
    if (SavedTheme === 'light') {
        document.documentElement.setAttribute('data-theme', 'light');
        themeIcon.classList.replace('fa-moon', 'fa-sun');
    }

    toggleBtn.addEventListener('click', () => {
        const currentTheme = document.documentElement.getAttribute('data-theme') || 'dark';
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        
        if (newTheme === 'light') {
            document.documentElement.setAttribute('data-theme', 'light');
            themeIcon.classList.replace('fa-moon', 'fa-sun');
        } else {
            document.documentElement.removeAttribute('data-theme');
            themeIcon.classList.replace('fa-sun', 'fa-moon');
        }
        
        setStorageItem(STORAGE_KEYS.THEME, newTheme);
    });
}

function initCommandPalette() {
    const modal = document.getElementById('command-palette-modal');
    const input = document.getElementById('cmd-k-input');
    const resultsContainer = document.getElementById('cmd-k-results');
    const triggerInput = document.getElementById('global-search');
    const triggerContainer = document.getElementById('cmd-k-trigger');
    
    if (!modal || !input) return;

    const quickLinks = [
        { title: 'Dashboard', type: 'Module', icon: 'fa-th-large', view: 'dashboard' },
        { title: 'Device List', type: 'Module', icon: 'fa-network-wired', view: 'devices' },
        { title: 'Performance Reports', type: 'Module', icon: 'fa-chart-bar', view: 'Reports' },
        { title: 'System Settings', type: 'Module', icon: 'fa-cog', view: 'Settings' },
        { title: 'User Management', type: 'Module', icon: 'fa-user', view: 'User' },
        { title: 'Activity Logs', type: 'Module', icon: 'fa-history', view: 'logs' }
    ];

    function toggleModal(show) {
        if (show) {
            modal.style.display = 'flex';
            input.value = '';
            renderResults();
            setTimeout(() => input.focus(), 100);
        } else {
            modal.style.display = 'none';
        }
    }

    // Keyboard shortcut Ctrl+K or Cmd+K
    document.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
            e.preventDefault();
            toggleModal(modal.style.display !== 'flex');
        }
        if (e.key === 'Escape' && modal.style.display === 'flex') {
            toggleModal(false);
        }
    });

    if (triggerContainer) {
        triggerContainer.addEventListener('click', () => toggleModal(true));
    }

    modal.addEventListener('click', (e) => {
        if (e.target === modal) toggleModal(false);
    });

    input.addEventListener('input', (e) => {
        renderResults(e.target.value.toLowerCase());
    });

    function renderResults(query = '') {
        resultsContainer.innerHTML = '';
        const filtered = quickLinks.filter(item => item.title.toLowerCase().includes(query) || item.type.toLowerCase().includes(query));
        
        if (filtered.length === 0) {
            resultsContainer.innerHTML = '<div class="text-center p-4 text-muted text-sm">No results found.</div>';
            return;
        }

        filtered.forEach(item => {
            const div = document.createElement('div');
            div.className = 'flex justify-between items-center p-4 hover-glow cursor-pointer border-b border-color';
            div.style.background = 'rgba(255,255,255,0.02)';
            div.style.borderRadius = '6px';
            div.innerHTML = `
                <div class="flex items-center gap-3">
                    <i class="fas ${item.icon} text-muted"></i>
                    <span class="text-main">${item.title}</span>
                </div>
                <span class="text-xs text-muted" style="background: rgba(255,255,255,0.05); padding: 2px 6px; border-radius: 4px;">${item.type}</span>
            `;
            div.addEventListener('click', () => {
                toggleModal(false);
                // Trigger view change (relies on router in app.js or dispatch event)
                const navItem = document.querySelector(`li[data-view="${item.view}"]`);
                if (navItem) navItem.click();
            });
            resultsContainer.appendChild(div);
        });
    }
}
