# NMS Phase 2 - Milestone 6 Design & Readiness Review

## 1. M6 Problem Statement
Penggunaan fungsi *logging* sinkron (`log.Printf`) di dalam jalur kritis (*hot path*) pada mekanisme *drop message* (ketika *subscriber* lambat) di `EventBus.Publish()` menciptakan *bottleneck* performa yang parah. Hal ini menyebabkan antrean panjang (lock contention) pada *global mutex* milik *standard logger*, yang kemudian menahan *publisher goroutines* dan menurunkan *throughput* sistem secara keseluruhan.

## 2. Current Behavior
Saat *channel* milik *subscriber* penuh, blok `select` pada `EventBus.Publish()` secara otomatis membuang (*drop*) pesan untuk melindungi sistem (kebijakan *backpressure*). Namun, tepat setelah pesan dibuang, sistem memanggil `log.Printf` untuk mencatat kejadian tersebut. Operasi ini bersifat *synchronous I/O* dan memblokir eksekusi *publisher*.

## 3. Root Cause
*Root cause* dari degradasi *throughput* bukan berasal dari desain konkurensi EventBus (`sync.RWMutex`), melainkan dari operasi I/O dan persaingan *lock logger* (`sync.Mutex` internal package `log`) pada saat puluhan atau ratusan goroutine secara bersamaan mencoba mencatat *event* yang dibuang.

## 4. Design Goals
- Mengeliminasi *synchronous I/O* pada jalur kritis *drop message*.
- Mempertahankan visibilitas/observabilitas metrik pesan yang dibuang untuk kebutuhan *monitoring* jangka panjang.
- Mempertahankan integritas kebijakan *backpressure* tanpa memblokir *publisher*.

## 5. Non-Goals
- Merombak arsitektur atau desain `EventBus`.
- Mengubah ukuran *buffer channel subscriber*.
- Mengubah semantik pengiriman pesan (tetap *best-effort/drop newest*).
- Menuntaskan kendala flag `-race` pada environment Windows (ini dicatat secara historis sebagai *verification limitation* di OS terkait, dan pengujian dapat didelegasikan ke CI/Linux jika diperlukan nanti).

---

## 6. Candidate Designs
Opsi desain untuk mengatasi permasalahan observabilitas tanpa mengganggu performa:
- **Opsi 1: Global Atomic Counter**. Satu variabel atomic di level `EventBus` untuk menghitung seluruh *drop*.
- **Opsi 2: Per-Subscriber Atomic Drop Counter**. Menambahkan field `Dropped uint64` pada struct `Subscriber` yang dikalkulasi secara atomic.
- **Opsi 3: Rate-Limited Logging**. Tetap menggunakan `log.Printf`, tetapi dibatasi (*throttle*), misal 1 log per 1.000 drop atau per 1 detik.
- **Opsi 4: Hybrid (Atomic Counter + Background Logger)**. Kombinasi *atomic counter* dengan *goroutine* terpisah yang membaca counter secara berkala dan mencetaknya jika ada selisih.

## 7. Comparison Matrix

| Kriteria | Opsi 1 (Global) | Opsi 2 (Per-Sub) | Opsi 3 (Rate-Limit) | Opsi 4 (Hybrid) |
|---|---|---|---|---|
| **Overhead Hot Path** | Minimal (~1 ns) | Minimal (~1 ns) | Sedang (Perlu cek timer/branch) | Minimal (~1 ns) |
| **Concurrency Safety**| Aman (`sync/atomic`) | Aman (`sync/atomic`) | Aman (dengan Mutex tambahan) | Aman |
| **Kardinalitas** | Rendah (Blind metric) | Tinggi (Tahu ID klien mana) | N/A | Tinggi |
| **Observability** | Buruk | Sangat Baik (Metric-ready) | Terpotong (Bisa miss burst) | Sangat Baik |
| **Coupling / API** | Tidak ada | Ubah `Subscriber` struct | Butuh pustaka rate-limit | Perlu *goroutine lifecycle* |
| **Memory Overhead** | ~8 bytes total | ~8 bytes per subscriber | Tergantung implementasi | ~8 bytes + goroutine stack |
| **Ekstensibilitas TSDB**| Cukup | Sangat Ideal | Buruk | Ideal |

## 8. Recommended Design
**Opsi 2 (Per-Subscriber Atomic Drop Counter)** adalah desain yang paling dianjurkan. Pendekatan ini murni berbasis memori, tidak membebani CPU, tidak mengandung I/O, memberikan informasi yang rinci tentang klien mana yang bermasalah, dan sangat ideal untuk diekspor ke Prometheus (Phase 3). 

## 9. Proposed API / Contract Changes
Modifikasi terbatas pada deklarasi struct `Subscriber` di `internal/transport/eventbus/bus.go`:
```go
type Subscriber struct {
	ID      string
	Channel chan contracts.DomainEvent
	Dropped uint64 // NEW: Penghitung atomic message yang di-drop
}
```

## 10. Hot-path Impact Analysis
Eksekusi `atomic.AddUint64(&sub.Dropped, 1)` menggantikan eksekusi I/O sinkron. Perintah atomic beroperasi di level perangkat keras (hardware instruction) dengan waktu eksekusi ~1-2 nanodetik. Ini menggaransi latensi `Publish()` nyaris tidak terasa (kembali ke profil ~4 µs seperti saat pengujian `io.Discard`).

## 11. Backpressure Compatibility Analysis
Semantik sama sekali **TIDAK BERUBAH**. Klien lambat akan tetap dibuang pesannya jika buffer penuh. Perbedaan hanya terletak pada pencatatan angka statistik (`Dropped`) ketimbang mencetak tulisan ke terminal. Pengiriman ke *fast subscriber* tidak akan tertunda karena *hot path* sama-sama lancar.

---

## 12. Test & Benchmark Plan
- **Benchmark**: Membuat fungsi `BenchmarkEventBus_Publish` pada `bus_test.go` untuk membandingkan performa (nanodetik/operasi dan alokasi memori) sebelum vs sesudah M6.
- **Load Test**: Memodifikasi kembali `m5_load_test.go` (menghapus `io.Discard`) untuk membuktikan performa aktual tanpa *mock logging* di lingkungan aslinya. Serta memverifikasi bahwa total `atomic.LoadUint64(&sub.Dropped)` sama persis dengan defisit jumlah pesan yang hilang pada akhir tes.

## 13. Acceptance Criteria
1. Tidak ada panggilan `I/O sinkron` (log/print) pada *drop hot path* di dalam `EventBus.Publish()`.
2. Pesan yang di-drop terhitung akurat dan dapat diverifikasi via properti `Dropped`.
3. *Publisher latency* kembali stabil (menuju baseline ~4-5 µs).
4. Semantik *backpressure* terbukti tetap identik dengan perilaku aslinya.
5. *Fast subscriber delivery* tetap tidak tersentuh.
6. *Slow subscriber* tetap terisolasi dan mandiri (tidak menjadi beban).
7. *Shutdown* dan *resource lifecycle* bersih (no leak).
8. Tersedia *benchmark* sebelum/sesudah yang menvalidasi optimasi.

## 14. Rollback Strategy
Risiko sangat terlokalisir. Jika terdapat isu struktural, *rollback* dilakukan hanya dengan *revert git commit* terhadap `bus.go` dan `bus_test.go`. Ini tidak akan merusak kontrak *Worker* atau antarmuka *WebSocket Hub*.

## 15. Files Expected to Change
- `backend/internal/transport/eventbus/bus.go`
- `backend/internal/transport/eventbus/bus_test.go`
- `backend/cmd/agent/m5_load_test.go` (Penghapusan bypass logger & penambahan validasi counter)

## 16. Risks
- *Visibility Loss*: Administrator yang selama ini melihat terminal untuk mendeteksi *subscriber* bermasalah secara *real-time* tidak akan melihat teks itu lagi.
- *Mitigation*: Mengingatkan administrator bahwa data *drop* ini nantinya akan disambungkan dengan metrik / grafik TSDB (Phase 3). Saat ini, data dapat diakses jika program menambahkan logika monitoring khusus.

## 17. Final Recommendation
Desain M6 telah dipastikan aman secara struktural dan konseptual. Solusi siap dieksekusi tanpa mengganggu kontrak M1-M4.

**REKOMENDASI:** Implementasi Opsi 2 (*Per-Subscriber Atomic Drop Counter*). Menunggu persetujuan eksplisit untuk memulai modifikasi *production code* dan penyelesaian M6.
