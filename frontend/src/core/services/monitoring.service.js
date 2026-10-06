// Monitoring Service
import { state } from '../state.js';
import { isOnline } from '../../shared/utils/helpers.js';
import { sendNotification } from './notification.service.js';

export function checkDeviceStatusChanges(currentData) {
    if (!currentData || !Array.isArray(currentData)) return;
    
    currentData.forEach(device => {
        const ipAddr = device.ip || device.ip_address;
        if (!ipAddr) return;
        const lastState = state.deviceStates[ipAddr];
        const currentState = isOnline(device.status) ? 'online' : 'offline';
        
        if (lastState && lastState !== currentState) {
            if (currentState === 'offline') {
                if (typeof window.playNocAudioAlert === 'function') {
                    window.playNocAudioAlert();
                }
                sendNotification('CRITICAL: Device Down', `Node ${device.name} (${ipAddr}) is UNREACHABLE!`);
            } else {
                sendNotification('RECOVERED: Device Online', `Node ${device.name} (${ipAddr}) is back online.`);
            }
        }
        state.deviceStates[ipAddr] = currentState;
    });
}
