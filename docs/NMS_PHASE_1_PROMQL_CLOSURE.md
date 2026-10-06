# Phase 1 Residual Finding Closure: PromQL Hardening

Dokumen ini berisi laporan penutupan *residual finding* pada proteksi TSDB/PromQL (`tsdb_handler.go`) sesuai *acceptance criteria* dari Phase 1. 

## 1. Perubahan File
1. `backend/config.yaml`: Menambahkan blok konfigurasi `promql` untuk memisahkan batasan (limit) agar tidak *hardcoded*.
2. `backend/internal/config/models.go`: Mendeklarasikan `PromQLConfig` `struct` beserta *default values*-nya.
3. `backend/internal/api/handlers/monitoring/tsdb_handler.go`: 
   - Mengintegrasikan `parser.ParseExpr` dari paket `github.com/prometheus/prometheus/promql/parser` untuk memvalidasi *time range*.
   - Menerapkan pembatasan `ResultSize` pada objek *response* sebelum dikembalikan.
   - Menerapkan *rate limiter* lintas *handler* menggunakan `golang.org/x/time/rate`.
4. `backend/internal/api/handlers/monitoring/tsdb_handler_test.go`: Menyertakan *mock server* Prometheus dengan pengujian ketat (*boundary limits*).
5. `backend/cmd/agent/main.go`: Pembaruan *signature* untuk mengoper *configuration* `PromQL` ke dalam *handler*.
6. `backend/go.mod` & `go.sum`: Penyertaan *dependency* prometheus parser, time/rate, serta perbaikan modul lawas (*go-ping errgroup*).

## 2. Configuration yang Ditambahkan
Menambahkan parameter berikut ke dalam *root* konfigurasi:
```yaml
promql:
  max_range: 24h             # Max time range for arbitrary queries (Matrix/Subquery)
  max_result_size: 1000      # Max array result allowed from Prometheus
  rate_limit_requests: 10    # Max allowed requests per window
  rate_limit_window: 1m      # The window duration for rate limiting
```

## 3. Test Cases (tsdb_handler_test.go)
1. `non-admin rejected` (Memverifikasi role-based auth, menolak selain admin)
2. `admin accepted` (Admin diizinkan meneruskan query normal)
3. `range below limit` (Matrix selektor di bawah `MaxRange`, misalnya `[30m]` → ACCEPT)
4. `range above limit rejected` (Matrix selektor di atas `MaxRange`, misalnya `[2h]` saat limit 1h → REJECT)
5. `result below limit` (Ukuran vektor balasan di bawah limit → ACCEPT)
6. `result above limit rejected` (Vektor masif melampaui limit → REJECT HTTP 413)
7. `timeout rejected` (Koneksi putus jika eksekusi melebihi 5 detik → REJECT 502)
8. `rate limit exceeded` (Simulasi request ke-3 dalam batasan window 2 req/menit → REJECT HTTP 429)

## 4. Hasil Kompilasi & Pengujian
- **`go build ./...`**: LULUS (Berhasil mengkompilasi *main agent* beserta perubahannya)
- **`go test ./...`**: LULUS (Semua package, termasuk *monitoring* yang menaungi 8 unit test di atas lulus dengan waktu ~7.4 detik)
- **`go test -race ./...`**: BLOCKED (OS Sandbox saat ini belum mendukung instalasi/penggunaan CGO yang dibutuhkan pendeteksi *data race*)

## 5. Status Masing-Masing Acceptance Criteria
| Acceptance Criteria | Status | Keterangan |
| :--- | :---: | :--- |
| Query normal → ACCEPT | **PASS** | `admin accepted` berlalu sukses. |
| Non-admin → HTTP 403 | **PASS** | Dicegat oleh konteks otorisasi role. |
| Query timeout → timeout/rejection | **PASS** | Strict context timeout (5s) berhasil menangkal eksekusi lamban. |
| Range di bawah limit → ACCEPT | **PASS** | Validasi Abstract Syntax Tree (AST) AST PromQL melalui `parser.Inspect` berhasil membaca `[30m]`. |
| Range di atas limit → REJECT | **PASS** | Matrix > MaxRange digagalkan sebelum terkirim ke prometheus. |
| Result di bawah limit → ACCEPT | **PASS** | Payload aman dikembalikan ke *client*. |
| Result melebihi limit → REJECT | **PASS** | Ukuran elemen array dibatasi untuk mencegah OOM (*Out-Of-Memory*). |
| Request rate normal → ACCEPT | **PASS** | *Token bucket* rate limiter. |
| Request rate melebihi limit → REJECT | **PASS** | Permintaan berlebih dilempar dengan kode 429. |

## 6. Residual Findings
- **PromQL Limits:** **CLOSED**. Seluruh celah beban TSDB dari rute administrasi telah ditambal deterministik.
- **Keseluruhan Temuan Kode Source:** **CLEAN / CLOSED**.
- **Evidence Verification (Lingkungan Asli):** Masih menunggu dipindah ke **Linux/CI/Staging**.

**Phase 2 Tetap BLOCKED.**
Langkah selanjutnya sepenuhnya berada di ranah CI/Staging untuk eksekusi ulang `docs/NMS_PHASE_1_RESIDUAL_VERIFICATION.md` secara nyata.
