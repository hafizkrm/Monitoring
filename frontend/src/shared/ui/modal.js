// Komponen UI: Modal
// Fungsi global untuk memunculkan atau menyembunyikan modal popup.

export function openModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) modal.style.display = 'flex';
}

export function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) modal.style.display = 'none';
}
