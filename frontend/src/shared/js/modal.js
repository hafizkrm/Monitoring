/* ============================================================
    MODAL TAMBAH PERANGKAT LOGIC
============================================================ */

export function openModal() {
    const modalAddDevice = document.getElementById('add-device-modal');
    if (modalAddDevice) {
        modalAddDevice.style.display = 'flex';
        modalAddDevice.style.opacity = '0';
        const nameInput = document.getElementById('device-name');
        setTimeout(() => {
            modalAddDevice.style.transition = 'opacity 0.2s ease-in-out';
            modalAddDevice.style.opacity = '1';
            if (nameInput) nameInput.focus();
        }, 10);
    }
}

export function closeModal() {
    const modalAddDevice = document.getElementById('add-device-modal');
    const formAddDevice = document.getElementById('add-device-form');
    if (modalAddDevice) {
        modalAddDevice.style.opacity = '0';
        setTimeout(() => {
            modalAddDevice.style.display = 'none';
            if (formAddDevice) formAddDevice.reset();
        }, 200);
    }
}

// Export to window so it can be called from HTML onclick attributes
window.openAddDeviceModal = openModal;
window.closeAddDeviceModal = closeModal;
