// Toast Notification Component

let toastContainer = null;

// Initialize toast container
function initToast() {
    if (!toastContainer) {
        toastContainer = document.getElementById('toast-container');
        if (!toastContainer) {
            toastContainer = document.createElement('div');
            toastContainer.id = 'toast-container';
            toastContainer.className = 'toast-container';
            document.body.appendChild(toastContainer);
        }
    }
}

// Show toast notification
export function showToast(Message, type = 'success') {
    initToast();
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    const icon = type === 'success' ? 'fa-check-circle' : 'fa-exclamation-circle';
    toast.innerHTML = `<i class="fas ${icon}"></i> <span>${Message}</span>`;
    toastContainer.appendChild(toast);
    setTimeout(() => toast.remove(), 4000);
}

// Initialize on module load
initToast();