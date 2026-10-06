// Device Factory (UI Otomatis)

import { getDeviceRenderer } from './deviceRegistry.js';
import { normalizeDevice } from './deviceNormalizer.js';

export function renderDeviceCard(rawDevice) {
    // 1. Normalisasi data terlebih dahulu
    const device = normalizeDevice(rawDevice);
    
    // 2. Pilih renderer yang tepat berdasarkan vendor
    const Renderer = getDeviceRenderer(device.vendor);
    
    // 3. Render HTML
    return Renderer(device).render();
}
