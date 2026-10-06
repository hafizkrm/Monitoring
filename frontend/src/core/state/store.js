/**
 * Reactive State Manager (Store)
 * Menggantikan pola window._monFullMetrics dan CustomEvent global
 */

class Store {
    constructor(initialState = {}) {
        this.state = { ...initialState };
        this.listeners = {};
    }

    /**
     * Mendapatkan state saat ini
     */
    getState() {
        return this.state;
    }

    /**
     * Memperbarui state secara parsial dan memberi tahu pendengar yang sesuai
     */
    setState(partialState) {
        const changedKeys = [];
        
        for (const key in partialState) {
            if (this.state[key] !== partialState[key]) {
                this.state[key] = partialState[key];
                changedKeys.push(key);
            }
        }

        // Trigger listeners only for changed keys
        changedKeys.forEach(key => {
            if (this.listeners[key]) {
                this.listeners[key].forEach(callback => callback(this.state[key], this.state));
            }
        });
        
        // Trigger global '*' listeners if anything changed
        if (changedKeys.length > 0 && this.listeners['*']) {
            this.listeners['*'].forEach(callback => callback(this.state, changedKeys));
        }
    }

    /**
     * Berlangganan pada perubahan property tertentu
     * @param {string} key Nama property state (atau '*' untuk semua perubahan)
     * @param {Function} callback Callback yang dipanggil dengan (newValue, fullState)
     * @returns {Function} Fungsi untuk berhenti berlangganan (unsubscribe)
     */
    subscribe(key, callback) {
        if (!this.listeners[key]) {
            this.listeners[key] = [];
        }
        this.listeners[key].push(callback);

        return () => {
            this.listeners[key] = this.listeners[key].filter(cb => cb !== callback);
        };
    }
}

// Inisialisasi Domain Store Utama
export const metricsStore = new Store({
    fullMetrics: [],       // Array metrik terbaru semua perangkat
    topTraffic: [],        // Perangkat dengan traffic tertinggi
    networkSummary: {      // Agregat jaringan (total bandwidth, avg latency)
        avgBw: 0,
        maxBw: 0,
        avgLoss: 0,
        avgLatency: 0,
        avgJitter: 0
    },
    alerts: []             // Alert sistem
});

export const uiStore = new Store({
    deviceFilter: 'Semua', // Kategori perangkat aktif ('Semua', 'Router', 'Switch', dll)
    activeView: 'dashboard'
});
