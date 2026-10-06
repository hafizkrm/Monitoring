// Device Modal handlers

export function openAddDeviceModal() {
    const modal = document.getElementById('add-device-modal');
    if (modal) {
        modal.style.display = 'flex';
        modal.style.opacity = '0';
        setTimeout(() => {
            modal.style.transition = 'opacity 0.2s ease-in-out';
            modal.style.opacity = '1';
        }, 10);
    }
}

export function closeAddDeviceModal() {
    const modal = document.getElementById('add-device-modal');
    const form = document.getElementById('add-device-form');
    if (modal) {
        modal.style.opacity = '0';
        setTimeout(() => {
            modal.style.display = 'none';
            if (form) form.reset();
        }, 200);
    }
}

// Keep backward compatibility for inline HTML
window.openAddDeviceModal = openAddDeviceModal;
window.closeAddDeviceModal = closeAddDeviceModal;
