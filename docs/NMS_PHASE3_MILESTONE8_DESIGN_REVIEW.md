# NMS Phase 3 - Milestone 8 Design Review: Metrics Contract & Instrumentation Standardization

## 1. Current Architecture
Infrastruktur saat ini (*FROZEN M7 Baseline*) berada pada arsitektur *Decoupled Event-Driven*. 
`Worker` (SNMP) mem-publish `DomainEventMetricsUpdated` (berisi *value struct* `cache.LatestDeviceMetrics`) ke `EventBus`. Kemudian `TSDBExporter` (sebagai *Subscriber* pasif) membaca event tersebut, memperbarui `prometheus.GaugeVec` internal, lalu mengeksposnya secara asinkron ke `/metrics` via `promhttp.Handler()`. Arsitektur M8 tidak menambahkan *synchronous I/O* atau *blocking operation* yang disengaja pada *EventBus publish path* (sinkronisasi internal *thread-safe* milik Prometheus tidak dianggap sebagai *synchronous I/O*).

## 2. Current Metrics Inventory
Inventaris metrik saat ini (berdasarkan eksportir M7):
- `nms_device_cpu_usage_percent` (Gauge)
- `nms_device_memory_usage_percent` (Gauge)
- `nms_device_latency_milliseconds` (Gauge)
- `nms_device_packet_loss_percent` (Gauge)
- `nms_device_tx_rate_bps` (Gauge)
- `nms_device_rx_rate_bps` (Gauge)
- `nms_device_status` (Gauge: 1 = Online, 0 = Offline)
- `nms_eventbus_dropped_messages_total` (CounterFunc)

## 3. Identified Gaps
Dari perspektif standarisasi tingkat-*Enterprise* (*OpenMetrics / Prometheus Standards*), terdapat celah ketidakselarasan (*gaps*):
1. **Standar Penamaan Unit (*Unit Standard*)**: Penggunaan `_milliseconds` pada *latency* bukanlah pelanggaran absolut Prometheus, namun penggunaan *base unit* (unit dasar mutlak) seperti `_seconds` merupakan *standardization/best practice*. Pada metrik jaringan (`tx_rate_bps`), audit repositori membuktikan `TxRate` dari `Worker` aslinya dihimpun dalam **Mbps (Megabits per second)**. Oleh sebab itu, mengeksposnya mentah-mentah di Prometheus sebagai `_bps` (seakan itu bits/sec) adalah keliru.
2. **Missing-Value Semantics**: Saat `Worker` gagal melakukan interogasi SNMP (perangkat mati), `Worker` mempertahankan data usang (*stale data*) di dalam `cache.LatestDeviceMetrics` sembari mengubah *Status* menjadi "offline". Jika diekspos mentah-mentah ke Prometheus, *scraper* akan terus merekam nilai usang ini selamanya sebagai nilai valid.
3. **Stabilitas Identitas (*Stable Device Identity*)**: Label M7 mencakup `ip_address`, `device_name`, dan `device_type`. `ip_address` adalah entitas dinamis (DCHP); menyertakannya dalam metrik dinamis akan menciptakan deret waktu (*time-series*) baru setiap kali IP berubah, merusak grafik historis.
4. **Parse Error Visibility**: Saat ini `TSDBExporter` tidak mengekspos visibilitas jika terjadi kegagalan konversi (contoh: *type-assertion failure* atau *malformed payload*). Eksportir hanya melempar log sinkron.

## 4. Proposed Metrics Contract
M8 akan mendiktekan *Immutable Metrics Contract* bagi sirkuit `Worker → EventBus → Exporter → Prometheus`.

## 5. Naming & Unit Standard
Perubahan penamaan/unit (Wajib disertai operasi konversi di eksportir):
- `nms_device_latency_milliseconds` → `nms_device_latency_seconds` (Dikonversi: `value / 1000.0`).
- `nms_device_tx_rate_bps` → `nms_device_tx_bytes_per_second` (Dikonversi dari Mbps ke Bytes/sec: `(value * 1000000) / 8`). Perlakuan sama untuk `RxRate`.
- Metrik baru penangkap *error*: `nms_exporter_parse_errors_total` (Counter).

## 6. Label/Cardinality Policy
Kardinalitas eksportir NMS terikat pada jumlah perangkat di database:
- **Primary Stable Identity**: `device_id`.
- **Secondary Identity**: `device_name` dan `device_type` dipertahankan untuk kebutuhan filter bisnis saat ini. Akan tetapi, ini bukan identitas *immutable*. Mengubah nama perangkat akan mengakibatkan terputusnya *series* (*series churn*). Hal ini didokumentasikan sebagai konsekuensi desain turunan.
- **Dilarang Keras**: `ip_address` dilarang digunakan sebagai label metrik dinamis.

## 7. Timestamp & Missing-Value Semantics
Kontrak `Offline Telemetry` dikunci secara eksplisit sebagai berikut:
- **Device Offline/Unresponsive**:
  - `CPUUsage` → diekspos sebagai `math.NaN()`
  - `MemoryUsage` → diekspos sebagai `math.NaN()`
  - `Latency` → diekspos sebagai `math.NaN()`
  - `PacketLoss` → diekspos sebagai `math.NaN()`
  - `TxRate` → diekspos sebagai `math.NaN()`
  - `RxRate` → diekspos sebagai `math.NaN()`
  - `Status` → diekspos sebagai `0`
  *Saat device offline, dynamic telemetry gauges diekspos sebagai NaN sehingga stale numeric value tidak dipertahankan sebagai nilai valid (merupakan Telemetry NaN, bukan Prometheus stale marker).*
- **Device Online**:
  - Nilai aktual `0` tetap valid sebagai `0`. (Jangan dikonversi menjadi `NaN` hanya karena nilainya nol!).

## 8. Architectural Impact
Jalur asinkron dan integrasi *boundary* (Worker → EventBus → Exporter) tidak terpengaruh. Tidak akan ada modifikasi apapun pada struktur sandi *Worker*, *EventBus*, atau `metrics_cache.go`. Segala resolusi metrik dipikul seutuhnya oleh logika internal `TSDBExporter`.

## 9. Risks
- Konversi *Missing-Value* menjadi `NaN` pada TSDBExporter akan memecah garis metrik pada dasbor Grafana lama yang belum menyesuaikan filter *null-value*. Ini murni konsekuensi pergeseran desain yang diharapkan.

## 10. Test Strategy
Pengujian diwajibkan menyertakan *unit test* dan skenario validasi:
- **Deterministic Unit Tests**: Validasi konversi metrik dengan nilai matematika yang pasti:
  - Latency: `100 ms` dikonversi mutlak menjadi `0.1 seconds`.
  - Throughput (Tx & Rx): `1 Mbps` dikonversi menjadi `125,000 bytes/sec`.
  - Throughput (Tx & Rx): `10 Mbps` dikonversi menjadi `1,250,000 bytes/sec`.
- **Parse Error Validation**: Melempar *malformed payload*. Verifikasi **kenaikan/inkremen** (bukan sebatas keberadaan) metrik kaunter `nms_exporter_parse_errors_total`, memastikan absennya intervensi *logging* sinkron di jalur tersebut.
- **Offline Injection Validation**: Injeksi event perangkat `offline`, pastikan seluruh atribut metrik berubah ke wujud `NaN`, kecuali status yang bertahan ke angka `0`.
- **Label Validation**: Injeksi dua event untuk `device_id` yang sama tetapi IP berbeda. Memverifikasi tidak terjadi penambahan *identity metric/series* baru di TSDBExporter.
- **Performance Benchmark Validation**: Memvalidasi performa aktual terhadap angka *threshold* numerik mutlak tanpa mengubah *baseline* historis M7.

## 11. Acceptance Criteria
Pintu rilis M8 mengamanatkan pemenuhan kriteria teknis berikut:
- *Latency* berubah unit dari `milliseconds` → `seconds` (Teruji konversinya).
- `Tx` dan `Rx` berubah unit dari `Mbps` → `bytes_per_second` (Teruji konversinya).
- Transmisi *offline telemetry* diekspos sebagai `NaN`.
- Status *offline* diekspos sebagai `0`.
- Status *online* dengan data nol (*actual zero*) diekspos mutlak sebagai `0`.
- Atribut `ip_address` dihapus dan terbukti tidak menjadi label pada metrik dinamis.
- **Performance Acceptance Threshold**: M8 dinyatakan lulus apabila hasil pengujian memenuhi spesifikasi numerik berikut:
  - **Minimum throughput: ≥ 500,000 events/sec**
  - **Maximum average publish latency: ≤ 10 µs**
  *(Catatan Historis M7 Baseline tetap immutable: Throughput ~641k events/sec, Latency ~4.1 µs).*

## 12. Rollback Plan
Rollback didefinisikan secara tegas sebagai *atomic revert* atas seluruh sirkuit modifikasi sandi M8, me-revert (*git revert*) komit untuk mengembalikan repositori persis dan absolut kembali ke kondisi status beku (*frozen baseline*) M7.

## 13. Implementation File Scope
Berkas yang diizinkan untuk direvisi pada saat implementasi M8 nantinya:
- `backend/internal/tsdb/exporter.go` (Perombakan Nama, Konversi Unit Skala Mutlak, Logika NaN `Status`, dan Eksisi Logging).
- `backend/cmd/agent/m8_load_test.go` (Klon deterministik atas skenario M7 untuk meresertifikasi kestabilan *EventBus*).

## 14. Readiness Verdict
**Status Dokumen: READY FOR IMPLEMENTATION AUTHORIZATION — AWAITING EXPLICIT APPROVAL.** 
Desain telah sepenuhnya dikoreksi dan disejajarkan. Semua operasi *production code* maupun sandi masa lampau disiagakan dalam keadaan henti-total (*STOP TOTAL*). Menunggu perizinan eksplisit tinjauan purna-desain.
