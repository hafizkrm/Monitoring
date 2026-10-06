# NMS Phase 2 Design & Readiness Review

## 1. Executive Summary & Readiness
Phase 1 telah dinyatakan **PASS** dengan baseline tag `phase1-final`. Sistem telah mencapai stabilitas untuk *SNMP polling*, penanganan goroutine yang aman, *graceful shutdown*, dan *database resilience*.
Saat ini, proyek berada dalam status **READY** untuk memulai eksekusi Phase 2.

Fokus Phase 2 adalah **Enterprise Architecture Foundation**, dengan prioritas pada pemisahan *domain boundaries*, standarisasi *event/envelope*, refaktorisasi `transport/` & `contracts/`, serta menyiapkan fondasi skalabilitas untuk 100+ *devices*.

---

## 2. Current State (Phase 1 Baseline)
Dari snapshot Phase 1, arsitektur saat ini memiliki karakteristik berikut:
- **Contracts (`internal/contracts/websocket.go`)**: Terdapat `WSEventEnvelope` yang menjadi standar baku pengiriman pesan ke Frontend. Namun, struktur ini sangat terikat (*tightly coupled*) dengan WebSocket.
- **Transport (`internal/transport/websocket`)**: Modul `hub.go` dan `client.go` mengatur koneksi secara *in-memory*. `DeviceManager` dan *worker* langsung memanggil fungsi `BroadcastMessage` pada instance `Hub` WebSocket.
- **Boundaries**: Belum ada pemisahan tegas antara *Domain Events* (internal sistem) dengan *Transport Events* (eksternal/WebSocket).

---

## 3. Gap Analysis (Menuju 100+ Devices & Enterprise Scale)
1. **Tight Coupling**: Modul internal (seperti *SNMP worker* atau *Device Manager*) bergantung langsung pada `websocket.Hub`. Jika ada transport lain (contoh: gRPC, Webhook) atau jika dipisah menjadi *microservices*, ini akan menjadi *blocker*.
2. **Event Standardization**: Belum ada *Internal Event Bus*. Pesan yang beredar saat ini dilempar dalam bentuk `WSEventEnvelope`, padahal internal sistem seharusnya berbicara menggunakan *Domain Events*.
3. **Scalability Limitations**: Skema iterasi linear pada `map[*Client]` di `hub.go` mungkin mencukupi untuk 100 devices/clients lokal, tetapi menjadi hambatan (*bottleneck*) jika harus *scale-out* menggunakan *message broker* (Redis Pub/Sub, RabbitMQ) di masa depan.

---

## 4. Proposed Architecture Changes (Phase 2 Foundation)

### A. Boundary & Domain Separation
- **Internal Domain Events**: Mendefinisikan *Standard Domain Event* (di `internal/models` atau *core domain*).
- **Transport Layer**: `internal/transport/websocket` hanya bertanggung jawab menerima koneksi dan meneruskan pesan dari *Event Bus*, bukan dari *worker* secara langsung.

### B. Standard Event / Envelope (`internal/contracts`)
- Ekstraksi `WSEventEnvelope` menjadi kontrak yang lebih general.
- Membuat *Standard Internal Event* yang mencakup metadata seperti `EventID`, `Timestamp`, `Source`, `Type`, dan `Payload`.
- `contracts` akan mendefinisikan *Interface* komunikasi (contoh: `EventPublisher` dan `EventSubscriber`).

### C. Abstraksi Transport & EventBus (`internal/transport`)
- Menerapkan **Pub/Sub Pattern** sederhana secara internal (`EventBus`).
- Alur Baru: `SNMP Worker` -> *publish ke* `EventBus` -> `WebSocket Hub` *subscribe ke* `EventBus` -> *Kirim ke Frontend*.
- Abstraksi ini memastikan di Phase 3 kelak (TSDB, HA), kita bisa mengganti *Internal EventBus* dengan *Message Broker* eksternal tanpa mengubah *business logic*.

---

## 5. Incremental Implementation Plan (Milestones)
Untuk mencegah regresi (*regression*), implementasi dilakukan bertahap dengan *unit test* dan verifikasi di setiap tahap:

- **Milestone 1**: Refaktorisasi `internal/contracts` untuk mendefinisikan *Standard Event Envelope* dan antarmuka *Event Bus*.
- **Milestone 2**: Implementasi *In-Memory Event Bus* (Pub/Sub) yang *thread-safe*.
- **Milestone 3**: Refaktorisasi `internal/transport/websocket/hub.go` agar berperan sebagai *Subscriber* pada *Event Bus*, bukan lagi diinjeksi langsung ke worker.
- **Milestone 4**: Mengubah `worker` dan `DeviceManager` agar melempar log/metrik/alert ke *Event Bus*.
- **Milestone 5**: *Load Testing* & *Verification* terhadap 100+ *mock devices* memastikan tidak ada *goroutine leak* dan performa stabil.

---
**STATUS:** Menunggu konfirmasi (Design Review) sebelum memulai **Milestone 1**.
