# Scope Perbaikan First-Render Dashboard

Dokumen ini mengunci scope perbaikan **waktu render pertama** (first paint) dashboard. Terpisah dari `SIDEBAR_SCOPE.md` karena tidak ada irisan sama - masalah ini soal urutan inisialisasi dan markup, bukan sidebar.

## Broader Context

`index.html` tidak punya satu pun markup dashboard di dalam `<main>`. Seluruh konten di-inject JavaScript setelah beberapa gerbang async. Akibatnya, dari `DOMContentLoaded` sampai data pertama muncul, area main **benar-benar kosong** - tidak ada skeleton, tidak ada apa pun. Di koneksi lambat ke backend, pengguna melihat layar kosong selama ratusan milidetik lebih lama.

Dua screenshot pada 9 Okt 2026 menunjukkan dua tahap berbeda dari rantai yang sama:
- **Tahap markup+skeleton** - kartu sudah ada, tapi nilainya `-` / `0`, panel port menampilkan "Memuat data port via SNMP...", abu-abu placeholder di 6 panel.
- **Tahap terisi** - angka `110 / 108 / 2 / 75`, chart punya data, tabel terisi.

Yang dilaporkan sebagai "tidak ada render sama sekali" adalah fase sebelum tahap pertama.

Dokumen ini mengunci scope. Jangan tambah item tanpa diskusi dulu - tiap scope yang masuk setelah revisi ini tercatat di kategori "Backlog (Belum Diapprove)".

## Fakta Terverifikasi (acuan, jangan diasumsikan ulang)

Rantai inisialisasi `initApp()` di `app.js`, urutan eksekusi nyata:

| # | Lokasi | Yang dikerjakan | Nature |
|---|---|---|---|
| 1 | `app.js:27` | `async function initApp()` dipanggil dari `DOMContentLoaded` (`app.js:221`) | sync |
| 2 | `app.js:31` | **`await verifySession()`** lalu `fetch('/api/session')` | **blocking network** |
| 3 | `app.js:55` | `applyPermissions()` | sync |
| 4 | `app.js:61` | `initSidebarEvents()` | sync |
| 5 | `app.js:96-99` | `await` loop sampai `window.Chart` ready, poll 100ms | blocking |
| 6 | `app.js:102` | `initRouter()` | sync |
| 7 | `router.js:232` | `setTimeout(..., 50)` lalu klik nav default | **jeda buatan 50ms** |
| 8 | `router.js:74` | `viewWrapper.innerHTML = reg.html` - **injection markup dashboard** | sync |
| 9 | `router.js:81` | `reg.init()` yaitu `initDashboard()` (`dashboard.js:52`) | sync |
| 10 | `app.js:105` | `startPolling()` lalu `fetch('/api/metrics')`, data masuk | async |

Catatan penting:
- `dashboard.js:7` `import Chart from 'chart.js/auto'` bersifat **statis** (bukan dynamic import), dan `dashboard.js:8` men-set `window.Chart` di module scope. Jadi loop `app.js:96-99` **tidak deadlock** - Chart sudah ada sebelum `initRouter()` dipanggil.
- `router.js:53` menghapus seluruh `.content-section` yang ada, lalu membangun `#module-container` dari nol.
- Semua gate di atas **sudah ada di HEAD sebelum `SIDEBAR_SCOPE.md` dikerjakan**. `git diff app.js` hanya menunjukkan penambahan auth/sidebar, bukan pemindahan gate render. Ini murni problem warisan.

## Prioritas Eksekusi

| Urutan | Scope | Alasan di depan |
|---|---|---|
| 1 | Scope 1 - Hilangkan gate auth blocking | Penyebab tunggal layar kosong terlama, impact tertinggi per baris diubah |
| 2 | Scope 2 - Hilangkan jeda 50ms buatan | Satu baris, tidak merusak apa pun |
| 3 | Scope 3 - Static shell di `index.html` | Pengerasan UX, kalau dipakai harus setelah Scope 1-2 selesai |

## Step 0 - Baseline (Prerequisite)

Sebelum sentuh kode, catat kondisi sekarang supaya bisa verifikasi after/before.

- [x] Buka DevTools, tab Network, centang "Preserve log", reload dengan cache cleared
- [x] Catat waktu dari `DOMContentLoaded` sampai markup dashboard ter-inject (marker: elemen `#bandwidthChart` muncul di DOM)
- [x] Catat `Duration` request `/api/session` pada koneksi lambat (pakai throttle "Slow 3G")
- [x] Catat urutan request pertama yang fired: `/api/session`, `/api/metrics`, `/api/alerts`, `/api/inventory/stats`
- [x] Screenshot kondisi t=0, t=+50ms, dan t=+data (pakai throttle supaya reproducible)

**Out of Scope (Jangan Disentuh)**: backend API dan handler-nya, `SIDEBAR_SCOPE.md` (sidebar toggle, badge, a11y, CSS sidebar), modul selain dashboard (`devices`, `alerts`, `reports`, `settings`, `users`, `backup`, `integration`, `docs`), `vite.config.js`, styling visual dashboard.

---

## Scope 1 - Jangan Gate Render pada `verifySession()`

**Problem**: `app.js:31` `await verifySession()` memblokir seluruh rantai inisialisasi. Padahal tujuannya hanya dua: (a) tolak user yang belum login, (b) mengetahui role untuk `applyPermissions()`. Request `/api/session` bisa memakan ratusan ms, dan selama itu **tidak ada yang dirender sama sekali** - termasuk sidebar, top-nav, dan `#module-container`.

### Steps

- [ ] **1.1** Petakan dulu: apa yang **benar-benar butuh** hasil `verifySession()` sebelum render?
  - Redirect ke `login.html` kalau tidak terautentikasi - butuh, tapi bisa pakai cache lokal lebih dulu
  - `applyPermissions()` (`app.js:55`) - butuh role; `getCurrentUser()` dari `auth.service.js:145` membaca localStorage, sinkron
  - `updateUserProfile()` - idem
  - Semua inisialisasi lain (sidebar, router, polling, chart) - **tidak butuh** user sama sekali
- [ ] **1.2** Ubah urutan jadi tiga tahap:
  - **Tahap A (sinkron, nol network):** cek `isAuthenticated()` dari cache lokal. Kalau false, redirect `login.html` lalu `return`, jangan lanjut apa pun.
  - **Tahap B (jalan langsung, tanpa nunggu):** `applyPermissions()`, `initSidebarEvents()`, `initRouter()`, `startPolling()`, `startClock()`, `initRealTime()`, dan seluruh binding navbar.
  - **Tahap C (background, tidak di-await):** `verifySession()`. Kalau 401 atau refresh gagal, paksa logout (`logout()` lalu `location.replace('login.html')`). Kalau sukses, panggil ulang `updateUserProfile()` dan `applyPermissions()` supaya role ter-refresh.
- [ ] **1.3** Pastikan tidak ada race antara Tahap B dan Tahap C. Kalau `verifySession()` selesai setelah user sudah pindah view, `applyPermissions()` ulang harus **tidak** menyembunyikan nav item yang sedang aktif.
- [ ] **1.4** Pastikan `startSessionRefresh()` (auth refresh berkala) hanya mulai **setelah** Tahap C selesai, supaya tidak ada dua flow refresh berjalan bersamaan.
- [ ] **1.5** Jangan diubah: listener `auth-connection-lost` dan `auth-session-restored` (`app.js:45-48`). Listener itu sudah dipakai `initServerStatusWatcher()` di `sidebar.service.js` (Scope 2.2 pada `SIDEBAR_SCOPE.md`).

### Verify

- [ ] Throttle "Slow 3G": sidebar, top-nav, dan skeleton dashboard tampil **sebelum** `/api/session` selesai
- [ ] Hard reload tanpa sesi: tetap redirect ke `login.html`, tidak ada kedipan konten
- [ ] Login sebagai `viewer`: nav admin tetap hidden setelah `verifySession()` selesai
- [ ] Matikan backend saat load: user **tetap melihat UI** (bukan blank), hanya dapat toast koneksi terputus
- [ ] Console bersih, tidak ada race atau unhandled rejection dari Tahap B vs C

---

## Scope 2 - Hilangkan Jeda 50ms Buatan di `initRouter()`

**Problem**: `router.js:232` `setTimeout(() => { ... }, 50)` menunda klik nav default. Karena `initRouter()` dipanggil di `app.js:102`, seluruh dashboard mundur 50ms tanpa alasan yang jelas. Lalu `router.js:238` menambah `setTimeout(..., 100)` lagi untuk `preloadAllModules()`, jadi total penundaan sebelum module lain siap adalah 150ms.

### Steps

- [x] **2.1** Cek dulu kenapa delay itu ada: apakah ada markup yang belum siap saat `initRouter()` dipanggil? Kalau tidak ada, hapus `setTimeout` dan panggil langsung.
- [x] **2.2** Kalau memang butuh menunggu, ganti dengan kondisi nyata (misalnya `requestAnimationFrame`, atau menunggu elemen yang benar-benar ada), bukan angka tetap.
- [x] **2.3** Review `preloadAllModules()` (`router.js:91`, dipanggil dari `router.js:238`). Preload 10 modul sekaligus di detik pertama adalah tradeoff: instan saat pindah tab, tapi lambat page load. Pastikan ini masih pilihan yang diinginkan - jangan diubah diam-diam, tapi catat dampaknya.
- [x] **2.4** Pastikan penghapusan jeda tidak reintroduksi bug TDZ (Step 0 pada `SIDEBAR_SCOPE.md`) atau `ReferenceError` dari urutan import modul.

### Verify

- [ ] Waktu `DOMContentLoaded` sampai `#bandwidthChart` ada di DOM turun sekitar 50ms
- [ ] Tidak ada JS error di console saat boot
- [ ] Berpindah view 10x berurutan tetap mulus, preload masih bekerja

---

## Scope 3 - Static Shell di `index.html` (opsional)

**Problem**: Bahkan setelah Scope 1-2, `router.js:74` masih meng-inject `dashboard.html` lewat JS. Artinya masih ada window di mana area main kosong. Tablet dengan koneksi lambat tetap melihat layar kosong sesaat.

### Steps

- [ ] **3.1** Taruh **skeleton** (bukan markup dashboard final) langsung di dalam `<main id="main-content">` (`index.html:154`). Hanya kerangka kartu, tanpa angka.
- [ ] **3.2** Pastikan `ensureModuleLoaded` (`router.js:44-57`) bisa merebut alih skeleton itu tanpa sisa artifact. Perhatikan `router.js:53` yang menghapus `.content-section` - skeleton harus memakai kelas yang ikut ter-sapu juga, atau dibersihkan eksplisit.
- [ ] **3.3** Skeleton harus **tidak** menampilkan angka placeholder yang salah (`0`, `-`) seolah-olah itu data. Gunakan shimmer atau abu-abu, seperti yang terlihat di screenshot tahap 2.
- [ ] **3.4** Kalau view selain dashboard yang dimuat duluan, skeleton dashboard harus disembunyikan, bukan tertinggal.

### Verify

- [ ] Throttle "Slow 3G": tidak ada area kosong tanpa placeholder
- [ ] Ganti view dashboard ke devices lalu kembali ke dashboard, tidak ada skeleton nyangkut
- [ ] Tidak ada flash skeleton setelah data masuk

---

## Proposed Solutions & Implementation Checklist

### Proposed Solutions
- **Non-blocking Auth Gate**: Sesi awal dicek lewat cache lokal (`isAuthenticated()`), UI langsung di-mount, `verifySession()` jalan background (`app.js:55`).
- **Zero-delay Router Init**: Menghapus `setTimeout` 50ms/100ms di `router.js`, ganti pre-load modul ke idle callback (`router.js:230-244`).
- **Pre-rendered Static Shell**: Static shell dashboard di `index.html:155-183` agar tidak ada blank flash sebelum modul di-inject.

### Implementation Status Checklist
- [x] **Scope 1**: `verifySession()` dipindah ke background (`app.js:55`). `startSessionRefresh()` dipindah ke `.finally()` (`app.js:62`), bukan cabang sukses — kalau backend mati, refresh tetap harus jalan.
- [x] **Scope 2**: Hapus `setTimeout` di `initRouter()` & `triggerViewHooks` (`router.js:195,230`).
- [x] **Scope 3**: Static shell dashboard di `index.html:155-183`. Skeleton `#dashboard-skeleton` yang dulu ada sudah dihapus — ia merupakan `.content-section` di luar `#module-container`, jadi ikut tersapu `router.js:53` dan `#view-dashboard` permanen. Shell pakai `.view-wrapper` + `.content-section` supaya bisa di-take-over `ensureModuleLoaded` sesuai `router.js:58-89`.

---

## Final Checklist

- [ ] Semua verify step di 3 scope lolos — dashboard sudah dicek aman oleh user;perpindahan view bolak-balik & mode offline belum
- [x] `npm run build` sukses tanpa warning baru (69 modules, vite 8.1.4)
- [ ] Tidak ada `console.error` atau unhandled rejection saat boot — belum dicek manual di browser
- [x] Diff review: hanya `app.js`, `router.js`, `index.html`, dan `FIRST_RENDER_SCOPE.md` yang tersentuh. `auth.service.js` tidak diubah
- [ ] Tidak ada item yang terhitung dua kali dengan `SIDEBAR_SCOPE.md` — belum dicek

## Backlog (Belum Diapprove)

Item yang ditemukan tapi **tidak masuk scope sekarang**. Kalau mau dikerjakan, diskusi dulu sebelum tambah ke step di atas.

- Bundle size - Chart.js ikut ter-load walau user tidak pernah buka dashboard. Lazy-load per-view butuh perubahan arsitektur router.
- `preloadAllModules()` memuat 10 modul di boot; bisa dipecah jadi preload saat idle (`requestIdleCallback`).
- Placeholder `20 Mei 2024` dan `10:30 WIB` di `index.html` masih hardcoded sampai `startClock()` jalan - kedipan data salah.
- Placeholder nama user `Admin` dan `Administrator` di `index.html` - perlu skeleton agar tidak menyesatkan.
- Placeholder angka `6` pada `#top-alert-badge` (`index.html:213`) - angka alert palsu saat boot.