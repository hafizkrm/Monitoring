export function initAlerts() {
    fetchAlerts();
}

window.monFilterAlerts = (type, btn) => {
    // Update active button
    document.querySelectorAll('#alert-filter-bar button').forEach(b => b.classList.remove('active'));
    if (btn) btn.classList.add('active');
    
    // Filter logic
    const cards = document.querySelectorAll('.mon-alert-card');
    cards.forEach(card => {
        if (type === 'Semua') {
            card.style.display = 'flex';
        } else if (type === 'Critical' && card.dataset.severity === 'danger') {
            card.style.display = 'flex';
        } else if (type === 'Warning' && card.dataset.severity === 'warning') {
            card.style.display = 'flex';
        } else {
            card.style.display = 'none';
        }
    });
};
async function fetchAlerts() {
    if (typeof window.monRenderAlerts === 'function') {
        window.monRenderAlerts(); // Now fetches natively from backend
    }
}

