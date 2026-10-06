# M9 — TSDB Persistence Integration: Design & Readiness Review (Option A)

## 6.1 Executive Summary
Milestone 8 (M8) sukses dibekukan sebagai *Frozen Baseline* (Commit: `cf8e5d4a1ee9d951c2e70e6d7f1762752077e4d1`) yang mana metrik PromQL telah sepenuhnya menganut *Immutable Metrics Contract* tanpa identitas IP dinamis. Milestone 9 (M9) ditujukan untuk melegitimasi integrasi persistensi data, namun dengan arah arsitektural yang ketat pada **Option A**: **SQL sebagai sumber kebenaran historis otentik (authoritative application store)**, sementara **Prometheus murni berstatus instrumen *observability/scraping***, BUKAN pangkalan data aplikasi apalagi sarana *durable queue*.

Audit komprehensif terhadap basis kode menyimpulkan bahwa implementasi M9 **NOT READY** (Tidak Siap) untuk eksekusi langsung karena masih terhambat oleh keberadaan *legacy coupling* di tingkat *Worker*, benturan desain kontrak API, dan anomali identitas parameter. Seluruh temuan tertuang terperinci di bawah ini.

## 6.2 Current Architecture
Arsitektur per hari ini terjebak dalam hibrida antara *legacy path* (P1) dan eksekusi asinkron (M8):

```text
SNMP Collector
|
v
Worker / Processor
|
+-------------------------------------------+
|                                           |
v                                           v
SQL (metricRepo.InsertDeviceMetric)         Legacy direct TSDB (tsdb.DeviceTxRate...)
|
v
EventPublisher -> EventBus -> TSDBExporter -> Prometheus (M8 Canonical)
```

## 6.3 Target Architecture
Target final *Option A* melarang *Worker* untuk mengetahui eksistensi eksportir TSDB secara langsung.

```text
SNMP Collector
|
v
Worker / Processor
|
+----------------------+
|                      |
v                      v
SQL Persistence       EventPublisher
(authoritative)            |
v
EventBus (Non-durable)
|
v
TSDBExporter
|
v
Prometheus (Observability)
```

## 6.4 Persistence Ownership
| Data                        | Authoritative Store | Secondary/Derived Store |
| --------------------------- | ------------------- | ----------------------- |
| Device historical telemetry | SQL                 | Prometheus              |
| Operational metrics         | Prometheus          | -                       |
| Application state/history   | SQL                 | -                       |

## 6.5 Metric Contract
* **Legacy Metric Path**: Digunakan secara terselubung oleh `internal/worker/manager.go` via sinkronus *set* (`tsdb.DeviceTxRate.WithLabelValues(...)`). Masih mengikutsertakan parameter kuno (misal Mbps tanpa konversi, serta penanaman eksplicit IP Address sebagai label).
* **M8 Canonical Metric Path**: Metrik diekspos *EventBus-subscriber* (`TSDBExporter`) menggunakan prefiks konvensi ketat (`nms_device_tx_bytes_per_second`), mengabaikan IP Address, mengubah format latensi, dan berpegang pada semantik NaN absolut.
* **Konflik**: *Worker* aktif melempar data ke dua arus yang berbeda standar/basis unit.

## 6.6 Identity Contract
Pada basis data SQL serta jalur M8, identitas utama entitas dipatok paten menggunakan **`device_id`** (Stabil).
Sebaliknya, sisa peninggalan API observabilitas historis (`/api/tsdb/device-history` pada `backend/internal/api/handlers/monitoring/tsdb_handler.go`) dan *legacy metrics* masih bergantung pada parameter dinamis **`ip_address`**. Dinamisasi identitas IP ini berlawanan tajam dengan stabilitas komparasi historis deret waktu (*time-series*).

## 6.7 Retention Model
- **SQL Retention**: Diatur secara otonom melalui konfigurasi internal NMS (`metrics_retention_days`) dan secara berkala disapu oleh kron `CleanupOldData()`.
- **Prometheus Retention**: Diatur eksklusif oleh argumen proses Prometheus (e.g. `--storage.tsdb.retention.time`), sepenuhnya berada di luar intervensi *source code* NMS.

Dua sumbu retensi ini tidak boleh saling membatasi.

## 6.8 Failure Semantics
- **SQL gagal**: Jika `InsertDeviceMetric` rontok, maka historis permanen **terancam/hilang**. (Aksi kritis).
- **EventBus drop**: Pesan yang terpental akibat kapasitas penuh kanal (100 elemen) akan dicatat metrik *counter*, namun hilang dari Prometheus. Sejarah SQL tak tersentuh.
- **Exporter gagal / Prometheus Down**: Grafana / antarmuka visualisasi tak sanggup menyajikan data mutakhir, namun **sejarah data perangkat 100% terjaga dan abadi di SQL**. Sistem wajib melakukan *fallback* ke SQL.
- **Prometheus query gagal**: Mekanisme pembacaan API jatuh pada `db.GetMetricsHistoryByIP`. SQL menopang *reliability* aplikasi.

## 6.9 API Contract
`backend/internal/api/handlers/monitoring/tsdb_handler.go` rutin memanggil `TSDBDeviceHistoryHandler`. Handler ini terpaksa dioperasikan menggunakan parameter wajib `ip`. Mengingat M8 telah menghapus pelabelan alamat IP di PromQL, API ini kini secara semantik akan **SELALU GAGAL** di sisi TSDB dan terpelanting menuju SQL *Fallback*. Ini harus diubah menuju fondasi identitas `device_id`. Jangan lakukan modifikasi apa pun sebelum delegasi penyelesaian utang kode (*code debt*) ini diputuskan.

## 6.10 Legacy Path
Rute peninggalan usang (*Legacy Path*):
- **Lokasi**: `backend/internal/worker/manager.go` baris 445-454.
- **Fungsi**: Sinkronisasi injeksi nilai terhadap metrik statis `internal/tsdb/metrics.go` (`DeviceCPU`, `DeviceTxRate`, dll).
- **Terdampak**: Data terekspos menjadi repetitif ganda (*duplicate exposure*), menghabiskan siklus I/O *Worker*, dan mengimpor kontrak nama usang.
- **Risiko**: Deviasi skema (*schema drift*) dan penolakan format yang membingungkan sisi antarmuka Prometheus.
- **Migration Strategy**: Cabut secara bedah total seluruh rujukan fungsi TSDB lokal dari *Worker* pada fase eksekusi M9.

## 6.11 EventBus Limitation
Harap dicatat dengan tingkat urgensi tinggi bahwa infrastruktur saluran **EventBus** dirancang murni sebagai medium transfer nir-pemblokiran (*non-blocking event distribution mechanism*). EventBus **TIDAK DURABLE**, tidak memegang jaminan (*guaranteed delivery*), dan tak sepantasnya dipandang sebagai antrean *persistence* seperti Kafka/RabbitMQ.

## 6.12 Risks
- **[BLOCKER]** Identitas `ip` tertanam sebagai syarat utama pengunduhan metrik PromQL di `tsdb_handler.go`. 
- **[BLOCKER]** Kehadiran injeksi langsung Prometheus di kodingan inti `internal/worker/manager.go` merusak arsitektur target *Option A*.
- **[ACCEPTABLE DEBT]** API terikat fallback SQL yang menjamin sejarah data selamat secara fisik, meskipun tak lagi optimal secara latensi kueri observabilitas.

## 6.13 Scope
### IN SCOPE M9
- Re-orientasi parameter API `device-history` beralih patokan dari parameter `ip` menuju `device_id`.
- Eksisi absolut *Legacy Metric Path* dan *Package* terkait (memutuskan instrumen langsung dari *Worker*).
- Harmonisasi SQL Fallback menggunakan parameter `device_id`.

### OUT OF SCOPE M9
- Penambahan / Konfigurasi parameter *remote_write* PromQL.
- Migrasi *Worker* menjadi subrutin baru.
- Refaktorisasi SQL Schema atau pemaksaan *downsampling*.
- Ekstraksi/pembesaran kanal EventBus menjadi model antrean Kafka-like.

## 6.14 Acceptance Criteria
- Fungsi *Worker* di dalam komputasi mutlak TIDAK memuat instruksi spesifik TSDB.
- `tsdb_handler.go` mengeksekusi kueri `nms_device_*` yang tervalidasi menggunakan selektor `device_id="..."`, bukan `ip_address="..."`.
- Pengguguran sistem Prometheus (Pemadaman Instans/Proses) membuktikan riwayat perangkat masih sukses dibaca lewat parameter SQL (*Fallback* deterministik LULUS).
- Angka performa M8 (658k EPS / 3.4 µs) tertahan aman tanpa degradasi.

## 6.15 Implementation Plan
1. Analisis titik rubuh kueri `GetMetricsHistoryByIP` di API untuk dipersiapkan bermutasi menjadi `GetMetricsHistoryByDeviceID`.
2. Ekskavasi mutlak baris pemanggilan sintaks prometheus `tsdb.Device*` dari dalam iterasi *ProcessMetrics* di file `manager.go`.
3. Revitalisasi skrip `tsdb_handler.go` agar selaras menyusun filter PromQL yang menitikberatkan `device_id`.
4. Hapus direktori / fungsionalitas `tsdb/metrics.go` untuk selamanya (Mencegah invasi zombie-metrics di masa mendatang).

## 6.16 Rollback Plan
Menggunakan *checkout* tunggal untuk mengembalikan seluruh eksekusi menuju titik *FROZEN BASELINE* `cf8e5d4a1ee9d951c2e70e6d7f1762752077e4d1` tanpa menggerus / merekayasa jejak *evidence* historis.

---
# 7. READINESS VERDICT
**NOT READY**

M9 tertahan dari gerbang eksekusi. Pemblokir arsitektural (*Architecture Blockers*) dominan telah terverifikasi:
1. `tsdb_handler.go` masih mengandalkan pencarian identitas `ip` di saat metrik M8 telah menanggalkan parameter `ip_address`.
2. Integrasi usang di *Worker* (`manager.go`) yang menginjeksikan data langsung ke TSDB mengakibatkan pelanggaran terhadap pemisahan tanggung jawab (*persistence ownership*) arsitektur *Option A*.

Menunggu instruksi eksplisit selanjutnya untuk pembukaan akses mitigasi.
