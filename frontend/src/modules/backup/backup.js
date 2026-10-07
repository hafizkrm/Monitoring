import { exportConfig, importConfig, loadConfig, SaveConfig } from '../../core/services/config.service.js';
import { activityLogger } from '../../core/services/log.service.js';

let pendingImportData = null;

export async function initBackup() {
    bindExport();
    await bindAutoBackup();
    bindImport();
}

function bindExport() {
    const btnExport = document.getElementById('btn-backup-export');
    if (!btnExport) return;
    btnExport.addEventListener('click', () => {
        try {
            const jsonString = exportConfig();
            
            // Create a downloadable blob
            const blob = new Blob([jsonString], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            
            const Timestamp = new Date().toISOString().replace(/[:.]/g, '-');
            const filename = `nms-backup-${Timestamp}.json`;

            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);

            activityLogger.log({
                module: 'backup',
                Action: 'EXPORT',
                Description: `Exported system configuration to file: ${filename}`
            });

            window.showToast?.('Successfully exported configuration', 'success');
        } catch (e) {
            console.error(e);
            window.showToast?.('Failed to export configuration', 'error');
        }
    });
}

async function bindAutoBackup() {
    const toggle = document.getElementById('backup-auto-enable');
    const schedule = document.getElementById('backup-auto-schedule');
    if (!toggle || !schedule) return;

    let backupConfig = { enabled: false, schedule: 'daily' };
    try {
        const config = await loadConfig();
        if (config && config.backup) {
            backupConfig = config.backup;
        }
    } catch (e) {
        console.warn('Failed to load backup config:', e);
    }

    // Load initial state
    toggle.checked = !!backupConfig.enabled;
    schedule.value = backupConfig.schedule || 'daily';
    schedule.disabled = !toggle.checked;

    toggle.addEventListener('change', () => {
        schedule.disabled = !toggle.checked;
        SaveAutoBackupSetting(toggle.checked, schedule.value);
    });

    schedule.addEventListener('change', () => {
        SaveAutoBackupSetting(toggle.checked, schedule.value);
    });
}

function SaveAutoBackupSetting(enabled, schedule) {
    try {
        SaveConfig('backup', { enabled, schedule });
        activityLogger.log({
            module: 'Settings',
            Action: 'UPDATE',
            Description: `Edited Auto Backup settings: ${enabled ? 'Active ('+schedule+')' : 'Inactive'}`
        });
        window.showToast?.('Auto Backup settings saved', 'success');
    } catch (e) {
        window.showToast?.(e.message || 'Failed to save backup settings', 'error');
    }
}

function bindImport() {
    const fileInput = document.getElementById('backup-file-input');
    const dropZone = document.getElementById('backup-upload-zone');
    const previewZone = document.getElementById('backup-preview-zone');
    
    // UI Elements for preview
    const elFilename = document.getElementById('backup-preview-filename');
    const elDate = document.getElementById('backup-preview-date');
    const elAuthor = document.getElementById('backup-preview-author');
    const elCategories = document.getElementById('backup-preview-categories');
    const elUser = document.getElementById('backup-preview-User');

    const btnCancel = document.getElementById('btn-backup-Cancel');
    const btnConfirm = document.getElementById('btn-backup-confirm');

    fileInput.addEventListener('change', (e) => {
        const file = e.target.files[0];
        if (!file) return;

        const reader = new FileReader();
        reader.onload = (ev) => {
            try {
                const content = ev.target.result;
                const parsed = JSON.parse(content);
                
                // Keep the string for actual import later
                pendingImportData = content;

                // Populate preview
                elFilename.textContent = file.name;
                elDate.textContent = new Date(parsed.metadata?.exportedAt || Date.now()).toLocaleString('id-ID');
                elAuthor.textContent = parsed.metadata?.createdBy || 'Unknown';
                
                const cats = parsed.payload?.Settings ? Object.keys(parsed.payload.Settings).length : 0;
                const User = Array.isArray(parsed.payload?.User) ? parsed.payload.User.length : 0;
                
                elCategories.textContent = `${cats} Categories`;
                elUser.textContent = `${User} Accounts`;

                // Switch UI
                dropZone.style.display = 'none';
                previewZone.style.display = 'block';

            } catch (err) {
                window.showToast?.('File tidak valid atau bukan format JSON', 'error');
                fileInput.value = '';
            }
        };
        reader.readAsText(file);
    });

    btnCancel.addEventListener('click', () => {
        pendingImportData = null;
        fileInput.value = '';
        previewZone.style.display = 'none';
        dropZone.style.display = 'block';
    });

    btnConfirm.addEventListener('click', () => {
        if (!pendingImportData) return;

        const oldIcon = btnConfirm.innerHTML;
        btnConfirm.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Memproses...';
        btnConfirm.disabled = true;

        setTimeout(() => { // Simulate slight delay for UX
            try {
                const result = importConfig(pendingImportData);
                
                activityLogger.log({
                    module: 'backup',
                    Action: 'IMPORT',
                    Description: `Sistem dipulihkan dari file: ${elFilename.textContent}`
                });

                window.showToast?.(`Restore berhasil! (${result.summary.categories} kategori, ${result.summary.User} user)`, 'success');
                
                // Reload page to apply new Settings globally
                setTimeout(() => window.location.reload(), 1500);
            } catch (err) {
                window.showToast?.(err.message || 'Error occurred', 'error');
                
                btnConfirm.innerHTML = oldIcon;
                btnConfirm.disabled = false;
            }
        }, 800);
    });
}

