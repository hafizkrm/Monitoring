// NOC TV Mode and Audio Features
import { getStorageItem, setStorageItem, STORAGE_KEYS } from '../../core/services/storage.service.js';

export function initNocFeatures() {
    // 1. Audio Alert Toggle
    const btnAudio = document.getElementById('btn-audio-toggle');
    const audioIcon = document.getElementById('audio-icon');

    const updateAudioIcon = () => {
        const isMuted = getStorageItem(STORAGE_KEYS.AUDIO_MUTED);
        if (audioIcon) {
            audioIcon.className = isMuted ? 'fas fa-volume-mute' : 'fas fa-volume-up';
            audioIcon.style.color = isMuted ? 'var(--accent-red)' : 'var(--accent-green)';
        }
        if (btnAudio) {
            btnAudio.title = isMuted ? 'Notifikasi Suara Alert (Muted)' : 'Notifikasi Suara Alert (Aktif)';
        }
    };

    updateAudioIcon();

    if (btnAudio) {
        btnAudio.addEventListener('click', () => {
            const isMuted = getStorageItem(STORAGE_KEYS.AUDIO_MUTED);
            setStorageItem(STORAGE_KEYS.AUDIO_MUTED, !isMuted);
            updateAudioIcon();
            if (window.showToast) {
                window.showToast(!isMuted ? 'Suara alert dimatikan' : 'Suara alert diaktifkan', 'info');
            }
        });
    }

    // 2. TV Fullscreen Toggle
    const btnFullscreen = document.getElementById('btn-fullscreen-toggle');
    const fullscreenIcon = document.getElementById('fullscreen-icon');

    const toggleNocFs = () => {
        const isFs = !!document.fullscreenElement;
        if (!isFs) {
            if (document.documentElement.requestFullscreen) {
                document.documentElement.requestFullscreen().catch(() => {});
            }
            document.body.classList.add('noc-tv-mode');
        } else {
            if (document.exitFullscreen) {
                document.exitFullscreen().catch(() => {});
            }
            document.body.classList.remove('noc-tv-mode');
        }
    };

    if (btnFullscreen) {
        btnFullscreen.addEventListener('click', toggleNocFs);
    }

    document.addEventListener('fullscreenchange', () => {
        const isFs = !!document.fullscreenElement;
        if (fullscreenIcon) {
            fullscreenIcon.className = isFs ? 'fas fa-compress' : 'fas fa-tv';
        }
        if (btnFullscreen) {
            btnFullscreen.style.color = isFs ? 'var(--accent-blue)' : '';
        }
        document.body.classList.toggle('noc-tv-mode', isFs);
    });
}
