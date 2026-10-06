# NMS Phase 3 - Milestone 7 Concrete Design & Readiness Review

## 1. Exact DomainEvent Contract
Event yang beredar di dalam `EventBus` mematuhi struktur baku `contracts.DomainEvent`:
```go
type DomainEvent struct {
	EventID   string      `json:"event_id"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
	Type      EventType   `json:"type"`
	DeviceID  string      `json:"device_id,omitempty"`
	Payload   interface{} `json:"payload"`
}
```

## 2. MetricsUpdated Payload Contract
Untuk event bertipe `contracts.DomainEventMetricsUpdated`, `Payload` secara konkrit berisikan *value-type* dari struct `cache.LatestDeviceMetrics` (yang diterbitkan oleh `Worker`):
```go
type LatestDeviceMetrics struct {
	DeviceID    int
	Name        string
	IPAddress   string
	DeviceType  string
	Status      string
	CPUUsage    float64
	MemoryUsage float64
	Latency     float64
	PacketLoss  float64
	TxRate      float64
	RxRate      float64
	Jitter      float64
	Uptime      int64
	// (beserta metadata waktu lainnya)
}
```

## 3. Metric Naming Convention
Penamaan metrik Prometheus akan diprefiks dengan `nms_` untuk menghindari kolisi (*namespace isolation*):
- `nms_device_cpu_usage_percent` (Gauge)
- `nms_device_memory_usage_percent` (Gauge)
- `nms_device_latency_milliseconds` (Gauge)
- `nms_device_packet_loss_percent` (Gauge)
- `nms_device_tx_rate_bps` (Gauge)
- `nms_device_rx_rate_bps` (Gauge)
- `nms_device_status` (Gauge: 1 = Online, 0 = Offline)
- `nms_eventbus_dropped_messages_total` (Counter)

## 4. Label Strategy
Label akan digunakan untuk memberikan dimensi pada metrik perangkat:
- **Device Metrics Labels**: `device_id`, `device_name`, `ip_address`, `device_type`.
- **EventBus Metrics Labels**: `component` (misal: "websocket_hub", "tsdb_exporter"), `subscriber_id`.

## 5. Cardinality Limits
Kardinalitas metrik terikat secara ketat (*bounded*) oleh jumlah entitas perangkat (*devices*) di dalam *database*. Untuk skala NMS konvensional (hingga 10.000 perangkat), kardinalitas maksimum per metrik adalah ~10.000 kombinasi label, yang masih sangat aman bagi RAM Prometheus. Label dinamis (seperti UUID event) dilarang keras untuk digunakan sebagai label prometheus.

## 6. Prometheus Registry Ownership
- Modul `internal/tsdb/exporter` akan memiliki `prometheus.Registry` tersendiri (atau menggunakan *DefaultRegisterer*) untuk meregistrasi metrik *Device*.
- **Penting**: *EventBus* tidak akan bertindak sebagai pengumpul metrik secara terpusat. Setiap komponen yang memiliki *Subscriber* (seperti `WebSocket Hub` dan `TSDB Exporter`) bertanggung jawab untuk membungkus `DroppedCount()` milik *Subscriber*-nya masing-masing ke dalam `prometheus.Collector` / `prometheus.CounterFunc`. Ini menjaga enkapsulasi.

## 7. Exporter Lifecycle
- Diinisialisasi di `main.go`.
- Berjalan sebagai *goroutine* independen yang membaca dari *channel* *Subscriber*-nya.
- Berhenti secara alami (*graceful shutdown*) ketika *EventBus* ditutup (yang menyebabkan penutupan *channel* *Subscriber*).

## 8. EventBus Subscription Lifecycle
`TSDBExporter` akan mendaftar (*Subscribe*) ke `EventBus` saat inisialisasi aplikasi. Ukuran buffer akan diset ke nilai wajar (contoh: 100). Bila antrean pemrosesan Prometheus melambat, perlindungan *backpressure* (pengorbanan data terbaru) dari `EventBus` akan berlaku sama seperti pada WebSocket.

## 9. DroppedCount Observability Boundary
Tidak akan ada fungsi peretasan (*hacking*) ke dalam internal struktur `EventBus` untuk mengais seluruh *subscriber*. Observabilitas ditegakkan di batas komponen: `WebSocket Hub` membaca *Subscriber*-nya sendiri, `TSDB Exporter` membaca miliknya sendiri. Mereka mendaftarkan angka `DroppedCount` tersebut melalui `prometheus.MustRegister(prometheus.NewCounterFunc(...))`. Tidak ada antarmuka `EventBus` yang dinodai.

## 10. Concurrency Model
1 *Goroutine* Publisher (Worker) -> `EventBus` RWMutex RLock -> N *Goroutine* Subscriber.
Operasi pembaruan metrik ke dalam `prometheus.Gauge` internal di pustaka `client_golang` sudah sepenuhnya *thread-safe* menggunakan *atomic / mutex* internal berkinerja tinggi. Model berjalan paralel murni dan tidak memblokir goroutine *publisher*.

## 11. Memory Model
*Zero-allocation hot path*. Pembaruan *Gauge* tidak menginisiasi alokasi *Heap* baru. Data `cache.LatestDeviceMetrics` dioper melalui struktur *value* di dalam *Channel* (copy struct), sehingga tidak ada risiko *data race* memori saat dibaca oleh eksportir.

## 12. /metrics Exposition Flow
Permintaan HTTP `GET /metrics` diproses oleh `promhttp.Handler()`. Proses *scraping* ini diisolasi oleh pustaka Prometheus dan tidak memiliki interaksi apa pun dengan `EventBus`, menjamin ketiadaan efek perlambatan saat proses HTTP (*exposition*) berlangsung.

## 13. Error Handling
Jika payload dari `DomainEventMetricsUpdated` gagal di-*type-cast* menjadi `cache.LatestDeviceMetrics`, eksportir hanya akan membuang (mengabaikan) event tersebut (kemungkinan disertai *increment* metrik `nms_exporter_parse_errors_total`) tanpa membekukan sistem (*No Panic*).

## 14. Test Strategy
- *Unit test* `exporter_test.go` untuk memvalidasi *routing Payload* menjadi metrik Prometheus.
- *Integration test* untuk memastikan nilai `GET /metrics` benar-benar merefleksikan event terakhir yang dilempar.

## 15. Performance Benchmark
Tolak ukur tetap berpegang pada basis M6:
Target: Integrasi `TSDBExporter` tidak boleh menyumbangkan *publish latency* hingga melebihi batas elastisitas (~10 µs) dan menjaga *throughput* sistem pada rentang > 500k rps saat diuji menggunakan M5 Load Test.

## 16. Regression & Rollback Strategy
- **Regression**: Karena arsitektur sepenuhnya bersifat aditif (hanya menambahkan *Subscriber* pasif baru), probabilitas regresi fungsional M1-M6 sangat mendekati 0%.
- **Rollback**: Menghapus blok inisialisasi `TSDBExporter` di `main.go` mengembalikan sistem secara mutlak ke era pra-M7.

## 17. Exact Files Expected to Change
- `backend/internal/tsdb/exporter.go` (BERKAS BARU: Logika Subscriber)
- `backend/internal/transport/websocket/hub.go` (Menambahkan integrasi registrasi metrik `DroppedCount` khusus WebSocket)
- `backend/cmd/agent/main.go` (Penyuntikan TSDBExporter ke EventBus)

## 18. Acceptance Criteria
1. *Metrics correctness*: Nilai metrik HTTP `/metrics` sama persis dengan angka SNMP terakhir.
2. *Deterministic labels*: Semua label tervalidasi berbasis Device ID/IP.
3. *Bounded cardinality*: Terbatas pada perangkat yang terdaftar di basis data.
4. *EventBus non-blocking behavior*: Waktu proses di dalam Exporter dipisahkan secara memori lewat mekanisme *channel*.
5. *No synchronous I/O*: Tidak ada log/disk I/O pada fase pemrosesan `DomainEvent`.
6. *Accurate eventbus dropped-message metric*: Klien WebSocket dan Exporter mengekspos jumlah *dropped count*-nya masing-masing.
7. *Clean startup/shutdown*: Siklus hidup tertutup seiring matinya `main.go`.
8. *No regression M1-M6*: Perilaku asli (*legacy*) bertahan 100%.

---
**STATUS M7 DESIGN REVIEW: DRAFT COMPILED. MENUNGGU PERSETUJUAN.**
