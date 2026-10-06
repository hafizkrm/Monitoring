// Web Audio API Synthesizer for Device Down Alarm (Zero mp3 file dependency)
import { getStorageItem, STORAGE_KEYS } from './storage.service.js';

let globalAudioCtx = null;
let UserHasInteracted = false;

// Global User gesture listener to unlock Web Audio API seamlessly
export function initAudioUnlock() {
    const unlock = () => {
        UserHasInteracted = true;
        if (!globalAudioCtx) {
            const AudioCtx = window.AudioContext || window.webkitAudioContext;
            if (AudioCtx) {
                globalAudioCtx = new AudioCtx();
            }
        } else if (globalAudioCtx.state === 'suspended') {
            globalAudioCtx.resume().catch(() => {});
        }
        document.removeEventListener('click', unlock);
        document.removeEventListener('keydown', unlock);
        document.removeEventListener('touchstart', unlock);
        document.removeEventListener('pointerdown', unlock);
    };
    document.addEventListener('click', unlock, { once: true });
    document.addEventListener('keydown', unlock, { once: true });
    document.addEventListener('touchstart', unlock, { once: true });
    document.addEventListener('pointerdown', unlock, { once: true });
}

export function playNocAudioAlert() {
    if (!UserHasInteracted || !globalAudioCtx) return;
    try {
        if (getStorageItem(STORAGE_KEYS.AUDIO_MUTED)) return;
        if (globalAudioCtx.state === 'suspended') {
            globalAudioCtx.resume().catch(() => {});
            if (globalAudioCtx.state === 'suspended') return;
        }
        const ctx = globalAudioCtx;
        const playBeep = (freq, duration, delay) => {
            const osc = ctx.createOscillator();
            const gain = ctx.createGain();
            osc.type = 'sawtooth';
            osc.frequency.setValueAtTime(freq, ctx.currentTime + delay);
            gain.gain.setValueAtTime(0.12, ctx.currentTime + delay);
            gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + delay + duration);
            osc.connect(gain);
            gain.connect(ctx.destination);
            osc.start(ctx.currentTime + delay);
            osc.stop(ctx.currentTime + delay + duration);
        };
        playBeep(880, 0.15, 0);       // High A5
        playBeep(1046.5, 0.25, 0.18); // High C6
    } catch (e) {
        // Silent catch for audio playback
    }
}

// Ensure audio is unlocked on load
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAudioUnlock);
} else {
    initAudioUnlock();
}

// Maintain backward compatibility for inline HTML calls
window.playNocAudioAlert = playNocAudioAlert;
