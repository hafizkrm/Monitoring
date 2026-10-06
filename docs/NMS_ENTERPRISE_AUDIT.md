# NMS Enterprise Audit Report

## 1. Executive Summary

Status: **NOT READY** untuk target 100+ devices.

Aplikasi saat ini memiliki beberapa masalah kritikal (P0) terkait goroutine retention pada operasi SNMP blocking terhadap device yang lambat, kegagalan sistem migrasi saat di-deploy via Docker (tidak fail-fast), serta tereksposnya credential `.env` dalam repository git. Selain itu, ada masalah skalabilitas (P1) pada write amplification database, pengiriman WebSocket yang tidak efisien, penggunaan `context.Background()` pada failure path, serta eksekusi PromQL yang tidak dibatasi (arbitrary).

Fase perbaikan (Phase 1) wajib dijalankan secara berurutan sesuai remediation plan sebelum *scale-up* atau deployment ke produksi.

## 2. Architecture Overview

- **Collector**: Berjalan dalam worker pool yang diatur oleh `manager.go` dan `scheduler.go`. Menggunakan library `gosnmp` untuk menarik metrik perangkat jaringan.
- **Processing**: Hasil metrik diteruskan ke `processor.go` untuk dievaluasi terhadap threshold insiden, lalu disimpan ke MySQL (inventaris, log, metrics harian) dan di-export ke Prometheus (via instrumentation `tsdb`).
- **State & Presentation**: Metrik terakhir di-cache di memori agar endpoint API cepat, serta dibroadcast ke frontend secara periodik melalui WebSocket.

## 3. Critical Findings

### [P0] SNMP Goroutine Retention on Context Timeout
Severity: BLOCKER
Category: Concurrency / Resource Management
File: `backend/internal/snmp/client.go`
Function: `queryGet`, `queryBulkWalk`
Line(s): 395, 434

**Current Behavior:** 
Fungsi `client.Get` dan `client.BulkWalk` dijalankan dalam *anonymous goroutine* dan di-select dengan context timeout (misal 15s).
**Why It Matters:** 
Library `gosnmp` bersifat sinkron. Membungkus eksekusi blocking dengan goroutine dan `select ctx.Done()` hanya menyelamatkan thread pemanggil (parent worker), tetapi goroutine yang sedang melakukan `client.Get` akan tertinggal (*goroutine retention / uncancellable blocking SNMP operation*) dan block hingga timeout default internal lib gosnmp atau TCP/UDP socket habis. 
**Impact:** 
Jumlah goroutine yang tertahan dapat terakumulasi selama durasi timeout operasi internal, sehingga berpotensi menyebabkan resource pressure dan degradasi performa apabila frekuensi timeout melebihi kemampuan operasi untuk selesai.
**Recommended Fix:** 
Tentukan lifecycle gosnmp secara lebih ketat, close underlying UDP connection jika operasi dibatalkan, atau ganti mekanisme timeout menjadi timeout koneksi native `gosnmp` sehingga eksekusi operasi secara otomatis terputus tepat waktu tanpa goroutine retention.

### [P0] Docker Migration Failure & Missing Fail-Fast
Severity: BLOCKER
Category: Deployment / Database
File: `deployments/docker/Dockerfile`, `backend/internal/database/migration.go`

**Current Behavior:** 
1. Pada `Dockerfile`, runtime container tidak menyalin folder autoritatif migrasi.
2. Pada `migration.go:31`, jika folder migrasi tidak ditemukan, sistem hanya mencetak warning (`log.Printf`) dan me-return `nil`, yang berarti aplikasi mengasumsikan sukses dan lanjut booting.
**Why It Matters:** 
Startup sequence harus deterministik. Jika skema tidak siap, aplikasi harus `FATAL`. Terlebih lagi, terdapat ambiguitas path (`internal/database/migrations` vs `backend/internal/database/migrations`).
**Impact:** 
Fresh deployment via Docker image akan menghasilkan container yang menyala dengan database kosong. Proses selanjutnya (login, worker write) dipastikan crash dengan "Table doesn't exist".
**Recommended Fix:** 
1. **Trace source of truth migration terlebih dahulu** untuk memastikan *exactly one authoritative migration source*.
2. Ubah `Dockerfile` agar membawa folder migrasi autoritatif tersebut.
3. Ubah perilaku dari me-return `nil` menjadi `return fmt.Errorf("migration directory missing")` agar `main.go` melakukan `os.Exit(1)` (Fail Fast).

### [P0] Credential Exposure in Repository
Severity: BLOCKER
Category: Security
File: `.env`

**Current Behavior:** 
File `.env` ikut di-commit ke dalam root repository. Berisi credential aktif (contoh: password RouterOS dan JWT secret).
**Why It Matters:** 
Credential plaintext tidak boleh berada di version control.
**Evidence:** 
Terdapat entry `ROUTEROS_PASSWORD=<REDACTED>` dalam file `.env` di sistem file, dan kemungkinan di git history.
**Impact:** 
Akses administratif RouterOS terbuka bagi siapa saja yang memiliki akses ke repository source code.
**Recommended Fix (WAJIB):** 
1. **Rotate** (ganti) semua password/credential yang telah terekspos di environment aktual.
2. Hapus `.env` dari repository tracking menggunakan `git rm --cached .env`.
3. Tambahkan aturan `*.env` dan `.env` ke dalam `.gitignore`.
4. Buat file template `.env.example` sebagai referensi.
5. Audit git history untuk membersihkan jejak commit dari file secret tersebut. (Gunakan `<REDACTED>` dalam pelaporan, jangan menulis plaintext secret).

### [P1] Arbitrary PromQL Execution
Severity: HIGH
Category: Security / API
File: `backend/internal/api/handlers/monitoring/tsdb_handler.go`
Function: `TSDBQueryHandler`

**Current Behavior:** 
Endpoint `/api/tsdb/query` mem-proxy payload PromQL dari parameter `query` user secara mentah ke Prometheus.
**Why It Matters:** 
Membuka ruang abuse terhadap resources TSDB.
**Recommended Fix:** 
Prioritaskan *Semantic API* (server-controlled PromQL). Backend menyediakan endpoint spesifik dan membungkus query secara aman. Jika *arbitrary PromQL* dipertahankan khusus role admin, wajib tambahkan mekanisme proteksi: timeout ketat, batas range waktu query, batas ukuran result set, dan rate limiting.

### [P1] Inefficient WebSocket Full Snapshot Broadcast
Severity: HIGH
Category: Performance / Architecture
File: `backend/cmd/agent/main.go`

**Current Behavior:** 
Ticker 5 detik membaca seluruh cache memori (`cache.GetMetricsCache().GetAll()`) dan mengirim (broadcast) data utuh setiap device ke seluruh WS clients.
**Why It Matters:** 
Metode ini adalah model *polling yang dibungkus websocket*. Seharusnya WebSocket bersifat realtime event-driven.
**Recommended Fix:** 
Rancang *event-driven / delta-based broadcast*:
`poll result -> compare state -> emit only changed device/event`.
Jangan mengirim payload apa pun jika tidak ada perubahan yang relevan pada device tersebut.

### [P1] DB Write Amplification
Severity: HIGH
Category: Performance / Database
File: `backend/internal/worker/processor.go`

**Current Behavior:** 
Setiap kali siklus poll perangkat sukses, worker melakukan serangkaian eksekusi SQL (Update Status, Incident Check, Insert Metric, Batch Interface Metric). Static code audit identifies at least 4 logical DB operations per successful poll; implementation must verify actual SQL statement count, transaction boundaries, and batch behavior through SQL logging/profiling.
**Why It Matters / Estimasi:** 
Pada asumsi awal 4 operasi/poll, untuk 500 perangkat interval 30s (~16.7 poll/detik):
- DB Statements/sec: ~67 statements / sec.
- Metric Rows/sec: ~183 rows / sec (asumsi avg 10 ifaces).
- Total per hari: ~15.8 Juta baris metric murni masuk ke MySQL setiap hari.
**Recommended Fix:** 
Kurangi statement melalui in-memory deduplication/debounce (hanya UPDATE jika status berubah). Profiling actual statements dan rapikan boundaries transaksi atau optimalkan batching.

### [P1] Unbounded `context.Background()` in Failure Path
Severity: HIGH
Category: Error Handling / Resilience
File: `backend/internal/worker/manager.go`

**Current Behavior:** 
Pada failure block (ketika SNMP gagal, timeout, atau circuit break), operasi persistence ke database seperti `UpdateDeviceStatus`, `InsertPollingLog`, dan `InsertActivityLog` dipanggil menggunakan `context.Background()`.
**Why It Matters:** 
Jika database melambat saat terjadi kegagalan jaringan massal, thread worker akan tersangkut permanen di operasi persistence DB karena tidak memiliki batas waktu (*bounded context*).
**Recommended Fix:** 
Selalu sediakan *bounded child context* (misal `context.WithTimeout(..., 5*time.Second)`) untuk semua operasi persistence atau error handling.

### [P2] Configuration Drift
Severity: MEDIUM
Category: Maintainability
File: `backend/config.yaml` vs `config/config.yaml`
**Current Behavior:** Duplikasi file config dengan nilai berbeda.
**Fix:** Hapus salah satu dan tetapkan source of truth yang konkrit.


## 4. Scalability Model (Estimates)

Asumsi dasar per device: rata-rata 10 interfaces, rata-rata durasi poll aktual 1.5 detik per device. TSDB samples = ~15 metric series per device. (Nilai DB statements berdasarkan *logical DB operations estimate*, harus diverifikasi via profil).

| Mode (Dev x Interval) | Polls/sec | Avg Workers Active | DB Statements/sec | Metric Rows/sec | Metric Rows/day | TSDB Samples/day | WS Events/sec (Idle)* |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 100 dev @ 30s | 3.33 | ~5 (of 50) | ~13.3 stmt/s | ~37 rows/s | ~3.19 Juta | ~4.32 Juta | 0 (jika delta) |
| 200 dev @ 30s | 6.67 | ~10 (of 50) | ~26.7 stmt/s | ~73 rows/s | ~6.30 Juta | ~8.64 Juta | 0 (jika delta) |
| 500 dev @ 30s | 16.6 | ~25 (of 50) | ~66.7 stmt/s | ~183 rows/s | ~15.8 Juta | ~21.6 Juta | 0 (jika delta) |
| 500 dev @ 15s | 33.3 | ~50 (FULL) | ~133 stmt/s | ~366 rows/s | ~31.6 Juta | ~43.2 Juta | 0 (jika delta) |

*Catatan WS Events: Nilai 0 saat idle akan tercapai JIKA model broadcast diubah ke delta-based event.*

## 5. Prioritized Remediation Plan (Phase 1)

**Urutan perbaikan wajib:**

1. **P0-1 Credential / Repository Security:** Rotasi password, hapus `.env` dari repo (git rm, gitignore, env.example, dan audit history).
2. **P0-2 Migration / Docker Fail-Fast:** Tentukan *exactly one authoritative migration source*, copy ke Dockerfile, dan berlakukan `FATAL` error jika skema missing.
3. **P0-3 SNMP Lifecycle / Timeout:** Tangani goroutine retention pada operasi SNMP dengan lifecycle connection/timeout yang native & deterministik.
4. **P1-1 DB Write Amplification + Context Boundaries:** Optimasi jumlah logical query/polling, gunakan profiling, dan selalu pakai *bounded child context* pada failure persistence.
5. **P1-2 WebSocket Event / Delta Architecture:** Ubah model dari Full Snapshot Ticker ke Event-Driven / Delta-based pada `processor.go` atau `manager.go`.
6. **P1-3 PromQL / API Hardening:** Implementasi Semantic API atau tambahkan pembatasan PromQL (timeout, limits) untuk admin.
7. **P2-1 Configuration Consolidation:** Penyatuan file duplikat konfigurasi.

## 6. Acceptance Criteria (Post-Phase 1)

Sebagai syarat lulus implementasi:

- `go build ./...` sukses.
- `go test ./...` passed.
- `go test -race ./...` passed (bebas data race).
- Fresh empty DB migration test sukses berjalan utuh saat deployment Docker.
- Missing migration directory menyebabkan startup `FAIL FAST` (`os.Exit`).

Kriteria Toleransi Baseline (Measurable Thresholds):
- **Goroutine Stability Test (50+ Slow/Offline Devices):** Setelah beban timeout berjalan repetitif selama 30 menit, *goroutine count* harus kembali berada dalam envelope baseline yang disepakati; tidak ada tren pertumbuhan (monotonic growth) pada goroutine, heap usage, ataupun pemakaian open files/sockets.
- **Graceful Shutdown Test:** Worker pool dan HTTP server berhasil mati dengan bersih (koneksi terputus dengan semestinya, tidak zombie).
- **200 Devices + WS Clients:** Beban trafik WebSocket, pemakaian CPU, dan DB latency harus divalidasi dengan alur *baseline → stress → compare*. Trafik harus mendekati nol saat status seluruh network idle/konstan (delta-based).
- **Arbitrary PromQL Rejection/Bounding:** Uji penetrasi query berat; Jika menggunakan Semantic API, arbitrary PromQL payload harus ditolak. Jika arbitrary PromQL dipertahankan untuk admin, query wajib memiliki timeout, range limit, result-size limit, dan rate limit, serta tidak boleh menyebabkan request handler atau service mengalami indefinite blocking.

---
*(Selesai Fase Audit - Phase 1 Menunggu Approval)*
