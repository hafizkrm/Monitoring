// DateTime Service
export function startClock() {
    updateDateTime();
    setInterval(updateDateTime, 1000);
}

function updateDateTime() {
    const dateEl = document.getElementById('header-current-date');
    const TimeEl = document.getElementById('header-current-time');
    const serverTimeEl = document.getElementById('server-Time'); // Based on new UI HTML ID
    
    const now = new Date();
    // Use Indonesian locale
    const dateOptions = { day: '2-digit', month: 'short', year: 'numeric' };
    const TimeOptions = { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false };
    
    if (dateEl) dateEl.innerText = now.toLocaleDateString('id-ID', dateOptions);
    if (TimeEl) TimeEl.innerText = `${now.toLocaleTimeString('id-ID', TimeOptions)} WIB`;
    if (serverTimeEl) serverTimeEl.innerText = `${now.toLocaleDateString('id-ID', dateOptions)} ${now.toLocaleTimeString('id-ID', TimeOptions)} WIB`;
}
