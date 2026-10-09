# Scope Perbaikan Sidebar Dashboard

Dokumen ini mengunci scope refactor sidebar agar tidak melebar.

## Broader Context

Sidebar adalah satu-satunya entry point navigasi di SPA ini. Saat ini punya 4 kelas masalah: ghost route (menu yang tidak terdaftar di router), state desync (badge & active class dimutasi via DOM langsung), duplikasi logic (toggle & permission di-ceck di banyak tempat), dan a11y gap (tidak ada keyboard handler, aria-current, atau title tooltip).

Dokumen ini mengunci scope. Jangan tambah item tanpa diskusi dulu — tiap scope yang masuk setelah revisi ini tercatat di kategori "Backlog (Belum Diapprove)".

## Prioritas Eksekusi

| Urutan | Scope | Alasan di depan |
|---|---|---|
| 1 | Scope 1-2 — Routing & Dead Code | Fix bug user-visible (menu error klik) |
| 2 | Scope 3-4 — Toggle & Event Consolidation | Prerequisite aman sebelum refactor state |
| 3 | Scope 5-6 — Badge & Permission State | Pindah ke store, hilangkan double work |
| 4 | Scope 7-8 — A11y & Polish | Tidak breaking, aman di akhir |

## Step 0 — Baseline (Prerequisite)

Sebelum sentuh kode, catat kondisi sekarang supaya bisa verifikasi after/before.

- [ ] Screenshot sidebar (desktop open, desktop collapsed, mobile open, mobile closed)
- [ ] Catat urutan dan jumlah item menu yang tampil per role (admin, operator, viewer)
- [ ] Catat nilai badge device & alert saat load pertama
- [ ] Catat `MODULE_REGISTRY` keys: `dashboard, devices, alerts, reports, logs, settings, users, backup` (8 keys)
- [ ] Verifikasi TDZ: buka browser, pastikan sidebar tampil tanpa JS error di console

**Out of Scope (Jangan Disentuh)**: Dashboard cards, device sidebar overlay, top navbar, backend API.

---

## Scope 1 — Register Ghost Route `integration` & Docs

**Problem**: `index.html:125` dan `index.html:133` punya `data-view="integration"` dan `data-view="docs"` tapi tidak ada di `MODULE_REGISTRY` (`router.js:26`). Klik menu → `Module <b>integration</b> is not available.`

### Steps

- [ ] **1.1** Tentukan keputusan per view (dua opsi, pilih satu per menu):
  - Opsi A — Register: buat `frontend/src/modules/integration/integration.html` + `integration.js`, import di `router.js`, tambah entry di `MODULE_REGISTRY`
  - Opsi B — Remove: hapus `<li>` dari `index.html` sampai fitur memang siap
- [ ] **1.2** Untuk `docs`: cek apakah halaman dokumentasi eksternal sudah ada (mis. `/docs`). Kalau iya, jangan register sebagai view internal — ganti anchor `<a href="/docs" target="_blank">`. Kalau belum ada, remove dari sidebar.
- [ ] **1.3** Kalau register `integration`: pastikan `canAccess('integration')` sudah didefinisikan di `permission.service.js`. Kalau belum, tambahkan mapping role.
- [ ] **1.4** View Title: cek `VIEW_TITLES` / `VIEW_SUBTITLES` di `core/constants.js`. Kalau key belum ada, tambah agar title tidak fallback ke "Network Monitor".
- [ ] **1.5** Tambah `init` hook di `triggerViewHooks` (`router.js:185`) kalau view baru butuh fetch data saat switch.

### Verify

- [ ] Klik menu Integrations → tidak ada "Module not available"
- [ ] Klik menu Documentation → redirect atau render benar
- [ ] Browser console bersih (tidak ada `is not available`)

---

## Scope 2 — Hapus Dead Code & Ghost Route Internal

**Problem**: `router.js:209` referensi `view === 'monitoring'` + `window.monInitView` tapi tidak ada di `MODULE_REGISTRY` dan tidak ada di sidebar HTML. `sidebar-server-status` (`index.html:140`) ada di DOM tapi tidak ada logic yang update.

### Steps

- [ ] **2.1** Cari seluruh referensi `monInitView` (`grep -r "monInitView" frontend/src`). Kalau fungsi ada, tentukan: implementasi memang dimaksudkan (register `monitoring` di sidebar + registry) atau dead code (hapus blok `if (view === 'monitoring')`).
- [ ] **2.2** Cari seluruh referensi `sidebar-server-status`. Kalau tidak ada yang update: putuskan — wire ke `/api/health` atau `/api/session` untuk update text `Online`/`Offline`, atau hapus blok `.server-status` dari `index.html`.
- [ ] **2.3** Cari CSS selector yang hanya dipakai sidebar gaya lama: `grep -n "sidebar-widget-container\|sidebar-node-container\|sidebar-input" frontend/src/shared/css/`. Kalau tidak ada match di `index.html` atau JS, hapus dari CSS.
- [ ] **2.4** Hapus hardcoded backup/logs komentar kosong di `index.html` sekitar baris 263-278. Router sudah render dinamis ke `#module-container` — markup statis itu dead layout.

### Verify

- [ ] `grep -r "monInitView" frontend/src` → zero result atau terpakai aktif
- [ ] `grep -n "sidebar-server-status" frontend/src` → ada logic updater, atau zero result di HTML
- [ ] Scroll `index.html` → tidak ada blok komentar kosong untuk module view

---

## Scope 3 — Konsolidasi Sidebar Toggle (Single Source of Truth)

**Problem**: 3 trigger di-scatter: `#btn-mobile-menu` (`index.html:76`), `#btn-sidebar-toggle` (`index.html:159`), `#sidebar-overlay` (`index.html:70`). Handler tersebar di `monitoring-alerts.js:709-741` dan mungkin modul lain → class `.open` dan `.active` bisa desync.

### Steps

- [ ] **3.1** Buat satu function tunggal `toggleSidebar(force)` — tempatkan di `frontend/src/core/sidebar.service.js` (file baru, satu-satunya tempat logic toggle hidup).
- [ ] **3.2** Function wajib sinkronkan **dua state sekaligus** dalam satu call:
  - `document.querySelector('.sidebar').classList.toggle('open', open)`
  - `document.getElementById('sidebar-overlay').classList.toggle('active', open)`
  - set `aria-expanded` pada tombol trigger yang aktif
- [ ] **3.3** Bind ketiga trigger ke function ini. Cek `monitoring-alerts.js` handler yang ada sekarang — hapus, jangan biarkan double binding.
- [ ] **3.4** Escape key: tambahkan `keydown` listener di document, kalau `key === 'Escape'` dan sidebar open → `toggleSidebar(false)`.
- [ ] **3.5** Resize guard: kalau `window.innerWidth > breakpoint_desktop`, sidebar harus auto-close supaya state mobile tidak nyangkut.

### Verify

- [ ] Klik hamburger di mobile → sidebar buka, overlay muncul
- [ ] Klik overlay → sidebar tutup, overlay hilang
- [ ] Klik hamburger di top-nav desktop → toggle jalan
- [ ] Tekan Escape saat sidebar open → tutup
- [ ] Resize dari mobile ke desktop saat sidebar open → sidebar balik ke posisi default, overlay hilang

---

## Scope 4 — Event Delegation untuk Navigasi

**Problem**: `router.js:229` loop `navItems.forEach` bind click ke setiap `li`. Plus `monitoring-alerts.js` bind handler mobile-close terpisah. Nav item bersifat statis (tidak ada add/remove runtime) delegasi lebih aman dan satu file.

### Steps

- [ ] **4.1** Di `sidebar.service.js`: bind satu click listener pada `<aside class="sidebar">` menggunakan `e.target.closest('.nav-item')` — bukan per-item.
- [ ] **4.2** Handler cek `has-submenu` (skip kalau submenu — menyesuaikan behavior lama `router.js:231`), lalu `e.stopPropagation()`, ambil `data-view`, panggil `switchView(view)`.
- [ ] **4.3** Hapus `navItems.forEach` loop di `router.js:229-239`. Jaga `initRouter` tetap export dan tetap expose `window.switchView` untuk caller non-sidebar (`app.js:117, 134`).
- [ ] **4.4** Cek duplikat: `grep -rn "nav-item" frontend/src --include=*.js`. Semua binding click harus hanya ada di `sidebar.service.js`.

### Verify

- [ ] Semua menu navigable via click
- [ ] `grep -rn "nav-item" frontend/src` → hanya match di `sidebar.service.js` dan selector query (bukan addEventListener)
- [ ] Global search (`app.js:117`) dan top-alert (`app.js:134`) masih bisa trigger switch view

---

## Scope 5 — Badge ke State Store (Anti-Desync)

**Problem**: `sidebar-alert-badge` dimutasi via DOM langsung (`monitoring-alerts.js:474-477`), `sidebar-device-badge` dari modul terpisah (`stats.js:91`). Kalau view di-render ulang atau reconnect, badge stuck atau salah.

### Steps

- [ ] **5.1** Definisikan shape store: cek `core/state/store.js` — ada `metricsStore` dan `alertStore`? Kalau hanya satu, gemakan pattern yang ada untuk membuat store baru.
- [ ] **5.2** Tambah state: `alertCount` dan `deviceCount` di store yang sesuai. Update value harus terjadi di service layer (polling / realtime), bukan di modul render.
- [ ] **5.3** Sidebar subscribe ke kedua count itu. Render badge jadi **pure function dari state** — tidak boleh ada code yang set `.textContent` atau `.style.display` dari luar sidebar.
- [ ] **5.4** Ganti mutation lama: hapus `document.getElementById('sidebar-alert-badge')` mutation dari `monitoring-alerts.js`. Ganti dengan `setState('alertCount', n)`.
- [ ] **5.5** Ganti mutation `sidebar-device-badge` dari `stats.js`. Sama — state write, bukan DOM write.
- [ ] **5.6** Badge count `0` → sembunyikan (`display: none`). Count `> 0` → tampilkan angka.

### Verify

- [ ] Saat alert masuk, badge sidebar alert ikut naik tanpa reload
- [ ] Saat switch view dan balik lagi, badge masih benar (tidak stick)
- [ ] `grep -rn "sidebar-alert-badge\|sidebar-device-badge" frontend/src` → hanya ada di `sidebar.service.js` (read/render), bukan di `monitoring-alerts.js` atau `stats.js`

---

## Scope 6 — Satukan Permission Check (Hapus Double Work)

**Problem**: `router.js:89` filter `canAccess` saat preload. `app.js:151` `applyPermissions()` juga `display: none` nav-item yang tidak boleh diakses. Dua mekanisme terpisah, bisa divergen.

### Steps

- [ ] **6.1** Tetapkan **satu sumber kebenaran**: `applyPermissions` di `app.js` jadi satu-satunya yang mengatur visibility menu (run saat login/session verified).
- [ ] **6.2** `preloadAllModules` (`router.js:89`) tetap filter `canAccess` — tapi jangan redundan dengan step 6.1. Kalau hidden item tidak di-preload, itu optimasi load, bukan security. Biarkan dua-duanya jalan, tapi pastikan logic-nya sama-sama dari `permission.service.js`, tidak ada hardcoded role list.
- [ ] **6.3** Empty section handling sudah ada (`applyPermissions:159` — section dengan semua item hidden jadi `display: none`). Verifikasi masih jalan setelah refactor.
- [ ] **6.4** `switchView` tetap panggil `canAccess` sebagai guard (`router.js:107`) — sifatnya defense-in-depth, biarkan.

### Verify

- [ ] Login sebagai viewer → menu admin hidden, tidak bisa klik atau navigate
- [ ] Semua item hidden tidak di-preload (cek Network tab — tidak ada fetch view admin)
- [ ] Section kosong (semua item hidden) tidak tampil sebagai header kosong

---

## Scope 7 — Keyboard Navigation & ARIA

**Problem**: `li` sudah punya `role="button"` dan `tabindex="0"` tapi tidak ada `Enter`/`Space` handler. Tidak ada `aria-current`. Global search tidak meng-announce hasil ke screen reader.

### Steps

- [ ] **7.1** Di `sidebar.service.js` (atau service yang barusan dibuat): bind `keydown` pada sidebar container.
  - `Enter` atau `Space` pada `.nav-item` → panggil handler yang sama dengan click (`e.preventDefault()` untuk Space supaya tidak scroll)
  - `ArrowUp`/`ArrowDown` → pindah focus antar `.nav-item` visible
- [ ] **7.2** Tambah `aria-current="page"` pada item yang aktif — taruh di `switchView` (`router.js:131` beri `classList.add('active')`), sekalian tambah `aria-current`.
- [ ] **7.3** Tambah `aria-expanded` pada tombol toggle ( hamburger + top-nav ) — set di `toggleSidebar` (Scope 3).
- [ ] **7.4** Tambah `title` attribute pada setiap `li.nav-item` di `index.html` (tooltip native — sudah ada global custom tooltip handler di `index.html:398-438`, otomatis upgrade jadi custom tooltip).
- [ ] **7.5** Hapus hardcoded `class="nav-item active"` di `index.html:96`. Biarkan router yang set class saat `switchView('dashboard')` pertama kali jalan (`initRouter:242-246` sudah handle default click).

### Verify

- [ ] Tab ke menu, tekan Enter → navigate
- [ ] Spasi pada menu fokus → navigate, halaman tidak scroll
- [ ] Arrow up/down gerak antar menu
- [ ] Inspect element: item aktif punya `aria-current="page"`
- [ ] Hover item menu → custom tooltip muncul (bukan native browser tooltip)

---

## Scope 8 — CSS Transition Performa

**Problem**: `main.css` sidebar transition pakai `all` yang trigger reflow. Layout thrashing saat resize atau rapid toggle.

### Steps

- [ ] **8.1** Cari rule sidebar transition: `grep -n "transition: all" frontend/src/shared/css/main.css frontend/src/shared/css/inline.css`
- [ ] **8.2** Ganti `transition: all 0.3s` menjadi hanya property yang dibutuhkan: `transition: transform 0.3s ease, opacity 0.3s ease` (atau width kalau memang ada animasi lebar).
- [ ] **8.3** Cek `will-change` — jangan set permanent. Kalau toggle rapat, `will-change: transform` boleh tapi hapus setelah transisi selesai, atau skip kalau noise.
- [ ] **8.4** Mobile open/close: pastikan animasi pakai `translateX` bukan `left` margin shift untuk hindari layout recalc.

### Verify

- [ ] Buka DevTools Performance → record 3x toggle sidebar → tidak ada panjang Layout/paint spike berulang
- [ ] Resize window saat sidebar buka → tidak ada jank terlihat
- [ ] Tampilan visual toggle tetap sama seperti sebelum refactor

---

## Final Checklist

- [ ] Semua verify step di 8 scope lolos
- [ ] `npm run build` (atau `vite build`) sukses tanpa warning baru
- [ ] Tidak ada `console.error` saat navigate antar semua view
- [ ] Diff review: hanya file `sidebar.service.js` (baru), `index.html`, `router.js`, `app.js`, `monitoring-alerts.js`, `stats.js`, `core/state/store.js`, dan CSS sidebar yang tersentuh. File di luar daftar ini jangan masuk.

## Backlog (Belum Diapprove)

Item yang ditemukan tapi **tidak masuk scope sekarang**. Kalau mau dikerjakan, diskusi dulu sebelum tambah ke step di atas.

- Desktop collapse mode (hamburger-only) — belum ada requirement jelas, butuh UX decision
- Konsolidasi icon library Font Awesome — belum ada bukti mismatch di codebase
- Logout button placement UX (3 lokasi) — bukan sidebar murni, menyentuh navbar
- Logout confirm modal — feature request, bukan bug fix
