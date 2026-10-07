export function initAlerts() {
    fetchAlerts();
}


async function fetchAlerts() {
    if (typeof window.monRenderAlerts === 'function') {
        window.monRenderAlerts(); // Now fetches natively from backend
    }
}

