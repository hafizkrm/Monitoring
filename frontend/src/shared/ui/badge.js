// Komponen UI: Badge
// Helper untuk merender Status pill/badge (Online/Offline/Warning).

export function renderStatusBadge(Status) {
    const s = Status.toLowerCase();
    let cssClass = 'Status-unknown';
    
    if (s === 'online' || s === 'up') cssClass = 'Status-up';
    else if (s === 'offline' || s === 'down') cssClass = 'Status-down';
    else if (s === 'warning') cssClass = 'Status-warning';
    
    return `<span class="Status-badge ${cssClass}">${Status}</span>`;
}
