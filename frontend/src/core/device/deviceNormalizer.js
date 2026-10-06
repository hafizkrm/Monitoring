// Device Normalizer
// Menyamakan format data dari berbagai vendor (Cisco, MikroTik, Ubiquiti, dll) 
// agar UI tidak kebingungan.

export function normalizeDevice(device) {
    // Jika backend Golang sudah menormalkan datanya dengan sempurna, 
    // fungsi ini hanya sebagai pass-through atau penambah nilai default.
    return {
        ...device,
        vendor: device.vendor || 'Unknown',
        status: device.status || 'Offline',
        name: device.name || device.ip_address || device.ip || 'Unnamed Device',
        upTime: device.upTime || device.uptime || 0,
        uptime: device.uptime || device.upTime || 0,
    };
}
