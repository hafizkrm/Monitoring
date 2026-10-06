// Device Registry (Kunci Scalable System)

// Import UI Renderers (Nanti akan di-implementasikan)
// import { MikroTikRenderer } from '../../modules/devices/renderers/mikrotik.js';
// import { UbiquitiRenderer } from '../../modules/devices/renderers/ubiquiti.js';
// import { TPLinkRenderer } from '../../modules/devices/renderers/tplink.js';
// import { CiscoRenderer } from '../../modules/devices/renderers/cisco.js';

// Fallback Renderer
const GenericRenderer = (device) => {
    return {
        render: () => `<div class="device-card generic"><h3>${device.name || 'Unknown Device'}</h3><p>IP: ${device.ip_address || device.ip}</p></div>`
    };
};

export const DeviceRegistry = {
    // mikrotik: MikroTikRenderer,
    // ubiquiti: UbiquitiRenderer,
    // tplink: TPLinkRenderer,
    // cisco: CiscoRenderer
};

export function getDeviceRenderer(vendor) {
    const key = (vendor || '').toLowerCase();
    return DeviceRegistry[key] || GenericRenderer;
}
