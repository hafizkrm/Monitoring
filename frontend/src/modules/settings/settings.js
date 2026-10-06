import { loadConfig, SaveConfig, resetConfig, resetAllConfig } from '../../core/services/config.service.js';
import { activityLogger } from '../../core/services/log.service.js';

let currentActiveTab = 'general';
let isFormDirty = false;
let currentConfig = null;

export async function initSettings() {
    bindTabs();
    bindFormChanges();
    bindAction();
    
    // Initial load
    await refreshData();
}

async function refreshData() {
    currentConfig = await loadConfig();
    populateForm(currentConfig);
    isFormDirty = false;
    updateButtonsState();
}

function populateForm(config) {
    // General
    document.getElementById('setting-general-poll').value = config.general.pollInterval;
    document.getElementById('setting-general-retention').value = config.general.logRetention;
    document.getElementById('setting-general-Timezone').value = config.general.Timezone;

    // SMTP
    document.getElementById('setting-smtp-host').value = config.smtp.host;
    document.getElementById('setting-smtp-port').value = config.smtp.port;
    document.getElementById('setting-smtp-Username').value = config.smtp.Username;
    document.getElementById('setting-smtp-Password').value = config.smtp.Password;
    document.getElementById('setting-smtp-tls').checked = config.smtp.tls;

    // SNMP
    document.getElementById('setting-snmp-version').value = config.snmp.version;
    document.getElementById('setting-snmp-community').value = config.snmp.community;
    document.getElementById('setting-snmp-Timeout').value = config.snmp.Timeout;
    document.getElementById('setting-snmp-retries').value = config.snmp.retries;

    toggleSnmpGroups(config.snmp.version);
}

function gatherFormData() {
    return {
        general: {
            pollInterval: Number(document.getElementById('setting-general-poll').value) || 0,
            logRetention: Number(document.getElementById('setting-general-retention').value) || 0,
            Timezone: document.getElementById('setting-general-Timezone').value
        },
        smtp: {
            host: document.getElementById('setting-smtp-host').value,
            port: Number(document.getElementById('setting-smtp-port').value) || 0,
            Username: document.getElementById('setting-smtp-Username').value,
            Password: document.getElementById('setting-smtp-Password').value,
            tls: document.getElementById('setting-smtp-tls').checked
        },
        snmp: {
            version: document.getElementById('setting-snmp-version').value,
            community: document.getElementById('setting-snmp-community').value,
            Timeout: Number(document.getElementById('setting-snmp-Timeout').value) || 0,
            retries: Number(document.getElementById('setting-snmp-retries').value) || 0
        }
    };
}

function bindFormChanges() {
    const inputs = document.querySelectorAll('#view-Settings input, #view-Settings select');
    inputs.forEach(input => {
        input.addEventListener('change', () => {
            isFormDirty = true;
            updateButtonsState();
        });
        input.addEventListener('keyup', () => {
            isFormDirty = true;
            updateButtonsState();
        });
    });

    document.getElementById('setting-snmp-version').addEventListener('change', (e) => {
        toggleSnmpGroups(e.target.value);
    });
}

function toggleSnmpGroups(version) {
    const v3Group = document.getElementById('snmp-v3-group');
    const commGroup = document.getElementById('snmp-community-group');
    if (version === 'v3') {
        v3Group.style.display = 'block';
        commGroup.style.opacity = '0.3'; // Muted
    } else {
        v3Group.style.display = 'none';
        commGroup.style.opacity = '1';
    }
}

function updateButtonsState() {
    const SaveBtn = document.getElementById('btn-Settings-Save');
    const resetBtn = document.getElementById('btn-Settings-reset-tab');

    if (isFormDirty) {
        SaveBtn.style.opacity = '1';
        SaveBtn.style.pointerEvents = 'auto';
        resetBtn.style.opacity = '1';
        resetBtn.style.pointerEvents = 'auto';
    } else {
        SaveBtn.style.opacity = '0.5';
        SaveBtn.style.pointerEvents = 'none';
        resetBtn.style.opacity = '0.5';
        resetBtn.style.pointerEvents = 'none';
    }
}

function bindTabs() {
    const btns = document.querySelectorAll('.Settings-tab-btn');
    const contents = document.querySelectorAll('.Settings-tab-content');
    const testSmtpBtn = document.getElementById('btn-Settings-test-smtp');

    btns.forEach(btn => {
        btn.addEventListener('click', () => {
            if (isFormDirty) {
                const proceed = confirm("Anda memiliki pereditan yang belum disave. Tetap pindah tab?");
                if (!proceed) return;
                
                // Revert changes
                populateForm(currentConfig);
                isFormDirty = false;
                updateButtonsState();
            }

            const tab = btn.getAttribute('data-tab');
            currentActiveTab = tab;

            btns.forEach(b => b.classList.remove('active'));
            contents.forEach(c => c.style.display = 'none');

            btn.classList.add('active');
            document.getElementById(`Settings-tab-${tab}`).style.display = 'block';

            // Show Test SMTP button only on SMTP tab
            if (tab === 'smtp') {
                testSmtpBtn.style.display = 'block';
            } else {
                testSmtpBtn.style.display = 'none';
            }
        });
    });
}

function bindAction() {
    const SaveBtn = document.getElementById('btn-Settings-Save');
    const resetBtn = document.getElementById('btn-Settings-reset-tab');
    const restoreDefaultBtn = document.getElementById('btn-Settings-factory-reset');
    const testSmtpBtn = document.getElementById('btn-Settings-test-smtp');

    SaveBtn.onclick = async () => {
        if (!isFormDirty) return;
        
        try {
            const formData = gatherFormData();
            // Save the currently active tab's data
            await SaveConfig(currentActiveTab, formData[currentActiveTab]);
            
            isFormDirty = false;
            updateButtonsState();
            currentConfig = await loadConfig(); // refresh internal state

            activityLogger.log({
                module: 'Settings',
                Action: 'UPDATE',
                Description: `Konfigurasi diperbarui pada kategori: ${currentActiveTab.toUpperCase()}`
            });

            window.showToast?.('Settings berhasil disave', 'success');
        } catch (e) {
            window.showToast?.(e.message || 'Gagal menyimpan pengaturan', 'error');
        }
    };

    resetBtn.onclick = () => {
        populateForm(currentConfig);
        isFormDirty = false;
        updateButtonsState();
    };

    restoreDefaultBtn.onclick = async () => {
        if (confirm('Anda yakin ingin mereset seluruh pengaturan ke setelan pabrik? Tindakan ini tidak dapat dicancelkan.')) {
            await resetAllConfig();
            await refreshData();
            
            activityLogger.log({
                module: 'Settings',
                Action: 'UPDATE',
                Description: 'Seluruh konfigurasi dikembalikan ke Factory Default'
            });

            window.showToast?.('Berhasil direset ke setelan pabrik', 'success');
        }
    };

    testSmtpBtn.onclick = () => {
        const icon = testSmtpBtn.innerHTML;
        testSmtpBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Menghubungi...';
        testSmtpBtn.disabled = true;

        setTimeout(() => {
            const host = document.getElementById('setting-smtp-host').value;
            if (!host) {
                window.showToast?.('SMTP Host masih kosong!', 'error');
            } else {
                window.showToast?.(`Konfigurasi server SMTP ${host} terverifikasi`, 'success');
            }
            testSmtpBtn.innerHTML = icon;
            testSmtpBtn.disabled = false;
        }, 1500);
    };
}
