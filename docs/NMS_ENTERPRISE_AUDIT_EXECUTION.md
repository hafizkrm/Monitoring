# NMS ENTERPRISE — AUDIT & HARDENING EXECUTION PLAN

## Document Purpose

Dokumen ini adalah **execution specification** untuk Antigravity.

Project NMS ditargetkan untuk **100+ network devices** dan akan berkembang menuju skala ratusan perangkat.

**ATURAN UTAMA:**

> **AUDIT DAHULU. JANGAN MENGUBAH KODE PADA FASE AUDIT.**

ZIP/project yang diberikan adalah source of truth untuk kondisi aktual. Semua kesimpulan harus berdasarkan kode, konfigurasi, migration, test, dan artifact yang benar-benar ditemukan.

---

# PHASE 0 — FULL AUDIT

## 0.1 Project Inventory

Inventarisasi:

- semua directory
- Go packages
- frontend
- database/migrations
- Docker
- systemd/service
- configuration
- Prometheus/TSDB
- scripts
- tests
- deployment files
- environment files
- generated files
- binaries
- logs
- data directories

Tandai artifact yang tidak seharusnya berada di source/release package:

- `.env`
- credential
- database dump
- Prometheus WAL/TSDB
- log besar
- compiled binaries
- generated data

**Jangan hapus apa pun.**

---

# 1. ARCHITECTURE & DATA FLOW

Trace berdasarkan kode aktual:

```text
Network Device
      |
      +-- ICMP / SNMP / API
      |
      v
Collector
      |
      v
Scheduler
      |
      v
Worker Pool
      |
      v
Metric Processing
      |
      +------> MySQL
      |
      +------> Prometheus / TSDB
      |
      v
API
      |
      +------> WebSocket
      |
      v
Frontend
```

Identifikasi dengan jelas:

- siapa membuat goroutine
- siapa membuat network connection
- siapa melakukan DB query/write
- siapa melakukan retry
- siapa melakukan timeout
- siapa melakukan logging
- siapa melakukan WebSocket broadcast
- siapa bertanggung jawab terhadap lifecycle
- siapa menangani shutdown

Buat data-flow/dependency map berdasarkan implementation aktual.

---

# 2. PERFORMANCE & RESOURCE

## 2.1 Goroutine / Thread Leak

Cari semua:

```go
go func()
go worker()
go poll()
```

Periksa:

- apakah goroutine selalu terminate
- apakah context cancellation benar-benar menghentikan operasi underlying
- apakah timeout hanya membatalkan caller tetapi network operation tetap berjalan
- apakah offline device menyebabkan goroutine retention
- apakah retry membuat goroutine baru
- apakah shutdown menunggu semua worker
- apakah channel berpotensi block forever

Khusus SNMP, audit pola seperti:

```go
go client.Get(...)
select {
case <-ctx.Done():
case result := <-channel:
}
```

Jangan menganggap context otomatis membatalkan library call yang tidak context-aware.

---

# 3. WORKER POOL & BACKPRESSURE

Audit:

- worker count
- queue size
- scheduler
- duplicate-job prevention
- stale-job handling
- retry
- backpressure
- shutdown
- worker starvation

Hitung kemampuan teoritis:

```text
devices
x polling frequency
x average poll duration
x DB writes
```

Evaluasi target:

- 100 devices
- 200 devices
- 500 devices

Jangan menaikkan worker count secara membabi buta.

---

# 4. SNMP / ICMP COLLECTOR

Audit:

- SNMP timeout
- SNMP retry
- SNMP session lifecycle
- Get
- Walk
- BulkWalk
- ICMP timeout
- vendor detection
- connectivity probe
- offline behavior
- slow device behavior
- packet loss behavior

Cari polling cycle yang melakukan discovery/probe berulang.

Jika vendor sudah diketahui dari inventory/cache, evaluasi apakah discovery dapat di-cache.

Target arsitektur:

```text
Device Inventory
      |
      v
Cached Vendor
      |
      v
Lightweight Connectivity Probe
      |
      +---- failure ---> degraded/down
      |
      v
Vendor Collector
      |
      v
Metrics
```

Hindari pola:

```text
brand detection
-> multiple connectivity probes
-> fallback walks
-> brand detection lagi
-> metrics
```

jika tidak benar-benar diperlukan.

---

# 5. DATABASE

## 5.1 Connection Pool

Audit:

```text
MaxOpenConns
MaxIdleConns
ConnMaxLifetime
ConnMaxIdleTime
```

Pastikan hanya ada satu canonical database initialization path.

Cari duplicate DB abstraction/configuration.

---

## 5.2 Query Audit

Cari:

- N+1 query
- query di dalam loop
- repeated SELECT
- repeated UPDATE
- unnecessary transaction
- missing transaction
- SELECT *
- expensive GROUP BY
- expensive ORDER BY
- unbounded query
- full table scan

Cocokkan query penting dengan index aktual.

Fokus pada:

```text
device_id
collected_at
interface_name
status
created_at
```

---

## 5.3 Write Amplification

Petakan operasi per polling cycle:

```text
poll
 -> device update
 -> metric insert
 -> interface metric insert
 -> polling log
 -> incident evaluation
 -> alert/log operation
```

Hitung estimasi DB operations untuk 100/200/500 devices.

Evaluasi kebutuhan:

```text
Collector
   |
   v
Bounded Metric Queue
   |
   v
Metric Writer
   |
   v
Batch INSERT
```

**Jangan implementasikan sebelum approval.**

---

# 6. TIME-SERIES STORAGE

Hitung estimasi data untuk:

- 100 devices
- 200 devices
- 500 devices

pada:

- 5s
- 15s
- 30s
- 60s

Pisahkan:

- device metrics
- interface metrics
- polling logs
- incidents
- activity/audit logs

Evaluasi data mana yang cocok di MySQL dan mana yang cocok di Prometheus/TSDB.

Roadmap yang diharapkan:

```text
MySQL
- inventory
- configuration
- incidents
- users
- audit/activity
- relational state

Prometheus/TSDB
- CPU
- memory
- latency
- packet loss
- bandwidth
- interface metrics
- other time-series metrics
```

Jangan mengubah schema hanya berdasarkan asumsi.

---

# 7. ERROR HANDLING & RESILIENCE

Audit:

- timeout
- retry
- exponential backoff
- circuit breaker
- rate limiting
- graceful degradation
- SNMP failure
- MySQL failure
- Prometheus failure
- WebSocket failure
- external API failure

Cari penggunaan:

```go
context.Background()
```

di dalam request/poll/failure lifecycle.

Jangan menggunakan context tanpa batas untuk operation yang seharusnya mengikuti lifecycle request/poll.

Evaluasi apakah failure handler sendiri memiliki timeout.

---

# 8. CONCURRENCY / RACE CONDITION

Cari shared state:

```text
map
slice
cache
counter
session
device state
scheduler state
worker state
WebSocket clients
circuit breaker
```

Periksa:

- mutex
- RWMutex
- atomic
- channel ownership
- single-flight
- data ownership

Jika environment mendukung, siapkan:

```bash
go test -race ./...
```

Jangan menyatakan aman hanya karena ada mutex; jelaskan reasoning berdasarkan ownership dan access pattern.

---

# 9. SECURITY

Cari:

- hardcoded password
- SNMP community
- RouterOS credential
- API key
- JWT secret
- default password
- credentials di config
- credentials di Docker
- credentials di migration
- credentials di log
- `.env` dalam repository/release

Audit:

- SQL injection
- command injection
- SSRF
- path traversal
- XSS
- CSRF
- authentication
- authorization
- RBAC
- sensitive information disclosure
- arbitrary PromQL
- resource/query abuse

Untuk setiap vulnerability:

```text
Severity:
File:
Function:
Line:
Problem:
Impact:
Recommended fix:
```

**Jangan mencetak secret asli ke audit report. Redact value.**

Jika ditemukan credential yang mungkin aktif, tandai sebagai **P0 — rotate immediately**.

---

# 10. API SECURITY & VALIDATION

Audit seluruh endpoint:

- authentication
- authorization
- input validation
- pagination
- rate limiting
- request timeout
- response size
- query parameter
- arbitrary query execution

Validasi khusus:

- IP address
- hostname
- device name
- enum
- interval
- numeric range
- IDs
- query parameters

Gunakan parser/validator yang tepat, misalnya `net.ParseIP()` untuk IP.

---

# 11. PROMETHEUS / PROMQL

Cari endpoint yang menerima arbitrary PromQL.

Jika authenticated user dapat mengirim:

```text
/api/tsdb/query?query=...
```

evaluasi:

- admin-only
- predefined query
- query validation
- timeout
- rate limit
- maximum range
- maximum result size

Tujuannya mencegah query abuse terhadap Prometheus.

---

# 12. CONFIGURATION CONSISTENCY

Cari seluruh:

```text
config.yaml
config.example.yaml
.env
Docker environment
systemd environment
deployment config
```

Bandingkan:

- DB host
- DB user
- DB password
- DB name
- polling interval
- worker count
- queue size
- Prometheus URL
- JWT
- SNMP settings

Identifikasi configuration drift.

Target:

> Satu canonical configuration schema/source of truth.

Environment-specific values boleh dioverride, tetapi tidak boleh ada beberapa file yang masing-masing menjadi authoritative dengan nilai berbeda.

---

# 13. DATABASE MIGRATION

Cari seluruh migration directory.

Pastikan hanya ada satu authoritative migration system.

Audit:

- duplicate migration
- migration order
- missing migration
- schema drift
- migration packaging
- Docker migration availability
- migration failure behavior

**Migration directory missing atau migration gagal harus menyebabkan FAIL FAST pada production startup.**

Jangan:

```text
migration missing
-> warning
-> application tetap start
```

Target:

```text
migration missing/failure
-> FATAL
-> application tidak start
```

Cari juga evidence schema drift dari log/error.

---

# 14. DOCKER / DEPLOYMENT

Audit:

- Dockerfile
- docker-compose
- runtime image
- environment variables
- migration files
- config files
- volume
- healthcheck
- startup order
- DB readiness
- Prometheus readiness

Pastikan environment variable yang diberikan Docker benar-benar dibaca aplikasi.

Contoh mismatch yang harus dicari:

```text
DATABASE_PASSWORD
vs
DB_PASSWORD
```

atau:

```text
monitoring
vs
nms_db
```

Fresh deployment harus deterministic:

```text
empty DB
-> migration
-> schema ready
-> application start
```

---

# 15. LOGGING & OBSERVABILITY

Audit:

- log level
- log rotation
- max size
- retention
- compression
- structured logging
- request ID
- device ID
- poll duration
- error classification
- sensitive data

Cari repeated log spam.

Jika error yang sama muncul ribuan kali, rekomendasikan:

- throttling
- aggregation
- counter/metric
- structured error code

Production default sebaiknya tidak DEBUG kecuali memang diperlukan.

---

# 16. WEBSOCKET

Audit:

- number of clients
- broadcast frequency
- message size
- full snapshot vs delta
- slow clients
- disconnected clients
- cleanup
- backpressure

Evaluasi scenario:

```text
100 devices
20 dashboard users
100 dashboard users
```

Target scalable:

```text
HTTP
-> initial authoritative snapshot

WebSocket
-> delta events
```

Hindari mengirim full snapshot semua device secara periodik jika delta event cukup.

---

# 17. FRONTEND

Audit:

- HTTP polling
- WebSocket
- reconnect
- duplicate request
- rendering frequency
- memory leak
- redundant API call

Cari pola:

```text
WebSocket realtime
+
HTTP polling
```

Tentukan apakah payload yang sama dikirim dua kali.

HTTP polling idealnya digunakan untuk:

- initial load
- reconnect reconciliation
- manual refresh

dan WebSocket untuk delta realtime.

---

# 18. MAINTAINABILITY

Identifikasi file >500 LOC.

Untuk file besar, jelaskan apakah tanggung jawabnya terlalu banyak.

Collector idealnya dapat berkembang menuju:

```text
snmp/
├── collector
├── connectivity
├── system
├── interfaces
├── wireless
└── vendor/
    ├── mikrotik
    ├── ubiquiti
    ├── ruijie
    ├── tplink
    └── cisco
```

Database/repository idealnya dapat berkembang menuju:

```text
repository/
├── device
├── metrics
├── incidents
├── polling_logs
├── activity_logs
└── reports
```

**Jangan refactor otomatis pada fase audit.**

---

# 19. TESTING

Audit existing tests.

Evaluasi kebutuhan:

```text
Unit
Integration
API
Database
Collector
Race
Failure
Load
```

Minimal scenario:

## Device online

```text
SNMP success
-> metrics collected
-> metrics persisted
```

## Device offline

```text
timeout
-> bounded failure
-> device degraded/down
-> no goroutine accumulation
```

## Device slow

```text
response > timeout
-> bounded execution
```

## MySQL unavailable

```text
DB failure
-> collector does not collapse
-> retry/backpressure behavior defined
```

## Prometheus unavailable

```text
TSDB failure
-> application remains operational according to defined degradation policy
```

## 100 concurrent devices

```text
100 polling jobs
```

## 500 concurrent devices

```text
500 polling jobs
```

---

# 20. SCALABILITY MODEL

Buat estimasi aktual berdasarkan kode.

Gunakan tabel:

| Devices | Interval | Workers | Estimated polling load | DB writes | Risk |
|---:|---:|---:|---|---|---|
| 100 | 30s | ? | ? | ? | ? |
| 200 | 30s | ? | ? | ? | ? |
| 500 | 30s | ? | ? | ? | ? |
| 100 | 15s | ? | ? | ? | ? |
| 200 | 15s | ? | ? | ? | ? |

Jangan mengarang angka. Jika asumsi diperlukan, tulis asumsi dengan jelas.

---

# 21. CURRENT FINDINGS TO VERIFY

Audit secara khusus temuan yang sudah terindikasi pada review sebelumnya:

### P0 — SNMP lifecycle

Ada indikasi operasi seperti:

```go
go client.Get(...)
```

dibungkus `select` terhadap context.

Verifikasi apakah cancellation benar-benar membatalkan underlying SNMP operation.

---

### P0 — Migration behavior

Ada indikasi migration directory yang hilang dapat membuat aplikasi tetap start.

Verifikasi.

Target:

```text
migration unavailable/failure
-> FATAL
```

---

### P0 — Docker migration packaging

Verifikasi apakah runtime Docker image benar-benar berisi migration files yang dibutuhkan application startup.

---

### P0 — Credential exposure

Verifikasi:

- `.env`
- SNMP community
- RouterOS credentials
- JWT secret
- default users/password
- DB credentials

Redact semua secret dalam report.

---

### P1 — Configuration drift

Verifikasi apakah terdapat beberapa config file dengan:

- polling interval berbeda
- DB berbeda
- worker count berbeda
- pool berbeda

---

### P1 — DB write amplification

Verifikasi jumlah DB round-trip per polling cycle.

---

### P1 — SNMP connectivity overhead

Verifikasi apakah polling melakukan connectivity/vendor detection berulang yang dapat dikurangi dengan cache/inventory.

---

### P1 — Arbitrary PromQL

Verifikasi apakah authenticated non-admin user dapat menjalankan arbitrary PromQL.

---

### P1 — WebSocket broadcast

Verifikasi apakah full device snapshot dikirim berkala kepada semua clients dan hitung dampaknya.

---

### P2 — Large files

Verifikasi file besar seperti collector/metrics dan query/repository layer.

Jangan pecah tanpa analisis responsibility terlebih dahulu.

---

# 22. PRIORITY SYSTEM

Gunakan:

## P0 — BLOCKER

Harus diperbaiki sebelum production scaling.

Contoh:

- active credential exposure
- migration failure
- deployment inconsistency
- unrecoverable goroutine accumulation
- severe security vulnerability
- data corruption

## P1 — HIGH

Harus diperbaiki sebelum target 100+ devices.

Contoh:

- polling bottleneck
- excessive DB writes
- SNMP inefficiency
- missing timeout
- arbitrary query abuse
- serious observability gap

## P2 — MEDIUM

Maintainability/performance improvement.

## P3 — LOW

Nice-to-have.

---

# 23. REQUIRED AUDIT REPORT

Buat file:

```text
docs/NMS_ENTERPRISE_AUDIT.md
```

Isi:

# NMS Enterprise Audit Report

## 1. Executive Summary

Status:

```text
READY
CONDITIONAL
NOT READY
```

untuk target 100+ devices.

## 2. Architecture Overview

## 3. Critical Findings

## 4. Performance & Resource

## 5. Worker & Scheduler

## 6. SNMP/ICMP

## 7. Database

## 8. Time-Series

## 9. Error Handling & Resilience

## 10. Concurrency

## 11. Security

## 12. API

## 13. Configuration

## 14. Migration

## 15. Docker/Deployment

## 16. Logging/Observability

## 17. WebSocket

## 18. Frontend

## 19. Maintainability

## 20. Testing

## 21. Scalability Model

## 22. Prioritized Remediation Plan

## 23. Recommended Target Architecture

## 24. Acceptance Criteria

---

# 24. FINDING FORMAT

Setiap finding harus menggunakan format:

```text
### [P0/P1/P2/P3] Short Title

Severity:
Category:
File:
Function:
Line(s):

Current Behavior:
...

Why It Matters:
...

Evidence:
...

Impact:
...

Recommended Fix:
...

Validation:
...
```

Jika tidak ditemukan:

```text
Status: PASS
Evidence:
...
```

Jangan membuat finding tanpa evidence.

---

# 25. IMPORTANT — NO CODE CHANGES YET

Pada PHASE 0:

**DO NOT:**

- refactor
- rename
- delete
- rewrite
- migrate schema
- change architecture
- change API
- change behavior
- change configuration

kecuali perubahan tersebut diperlukan hanya untuk menjalankan read-only audit tooling dan tidak mengubah project source.

Output pertama hanya:

```text
AUDIT REPORT
+
PRIORITIZED REMEDIATION PLAN
+
ACCEPTANCE CRITERIA
```

Setelah report selesai, **STOP dan tunggu approval.**

---

# PHASE 1 — IMPLEMENTATION AFTER APPROVAL

Jangan masuk Phase 1 sebelum user memberikan approval.

Urutan implementasi yang direkomendasikan:

```text
P0 Security
    ↓
P0 Migration / Deployment
    ↓
P0 SNMP Lifecycle
    ↓
P1 Polling / Backpressure
    ↓
P1 Database Write Optimization
    ↓
P1 API / PromQL hardening
    ↓
P1 WebSocket optimization
    ↓
P2 Maintainability Refactor
```

Setiap tahap:

1. backup/current state
2. minimal targeted change
3. build
4. unit test
5. integration test jika relevan
6. race test jika relevan
7. lint
8. migration validation
9. behavior verification
10. report changed files

---

# PHASE 2 — POST-IMPLEMENTATION GATE

Setelah implementation:

```text
go build ./...
go test ./...
go test -race ./...
```

sesuaikan dengan project/toolchain aktual.

Jalankan frontend build/test yang tersedia.

Validasi:

- 100 devices
- offline devices
- slow devices
- DB failure
- TSDB failure
- WebSocket reconnect
- shutdown
- restart
- migration

Kemudian buat:

```text
docs/NMS_ENTERPRISE_HARDENING_REPORT.md
```

---

# FINAL PRINCIPLE

Target bukan sekadar:

> "Application builds and works."

Target:

> "Application tetap stabil ketika perangkat lambat/offline, SNMP gagal, database lambat, TSDB gagal, banyak perangkat polling bersamaan, banyak dashboard user aktif, dan jumlah perangkat meningkat dari 100 -> 200 -> 500."

**AUDIT → REPORT → APPROVAL → IMPLEMENT → TEST → RE-AUDIT**

Jangan melompati gate tersebut.
