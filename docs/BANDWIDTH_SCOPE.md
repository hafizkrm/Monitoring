# Scope Perhitungan Bandwidth per Kategori

Dokumen ini mengunci masalah **perhitungan bandwidth dashboard**. Terpisah dari `FIRST_RENDER_SCOPE.md` karena ini bukan soal urutan render, tapi soal benar/tidaknya angka yang ditampilkan.

Ditemukan saat membersihkan placeholder hardcoded di `index.html` untuk `FIRST_RENDER_SCOPE.md` (9 Okt 2026). Tidak ada satu pun item di dokumen ini yang boleh masuk ke `FIRST_RENDER_SCOPE.md` atau `SIDEBAR_SCOPE.md`.

## Yang Terverifikasi (acuan, jangan diasumsikan ulang)

Konvensi backend sudah dicek langsung di kode, bukan asumsi:

| # | Lokasi | Fakta |
|---|---|---|
| 1 | `backend/internal/snmp/metrics.go:457` | `RxRate = totalDeltaIn * 8 / (intervalSec * 1e6)` |
| 2 | `backend/internal/snmp/metrics.go:458` | `TxRate = totalDeltaOut * 8 / (intervalSec * 1e6)` |
| 3 | `backend/internal/snmp/metrics.go:438-439` | `totalDeltaIn`/`totalDeltaOut` dijumlahkan untuk **semua interface aktif** pada satu device |
| 4 | `backend/internal/models/metric.go:21-22` | `tx_rate`/`rx_rate` dalam **Mbps** |
| 5 | `frontend/src/modules/devices/devices.html:119-124` | Kategori yang bisa dipilih user: `router`, `radio`, `access_point`, `switch`, `firewall`, `server`. **Tidak ada `gateway`** |

Poin 5 penting. S sempat terlihat seperti bug, tapi **bukan**: `backend/internal/tsdb/client.go:64,71` memakai PromQL `device_type=~".*(router|firewall|gateway).*"` sementara filter frontend hanya `router|firewall`. Kelihatannya tidak sinkron, tapi `gateway` tidak pernah muncul sebagai nilai `device_type` karena UI tidak menawarkannya. Filter frontend sudah lengkap. **Jangan diubah.**

---

## Bug 1 - Komentar dan kode bertentangan (paling kritis)

**Lokasi**: `frontend/src/modules/dashboard/dashboard.js:420-422` (komentar) vs `dashboard.js:437-438` (kode)

Komentar di baris 420-422 menyatakan:
- `tx_rate` (OutOctets) = data yang dikirim router ke pelanggan = **DOWNLOAD user**
- `rx_rate` (InOctets) = data yang diterima router dari pelanggan = **UPLOAD user**

Kode di baris 437-438 justru sebaliknya:
- `totalDownload += parseFloat(m.rx_rate)` -> RX dipetakan ke DOWNLOAD
- `totalUpload += parseFloat(m.tx_rate)` -> TX dipetakan ke UPLOAD

`stats.js:136-139` memakai pemetaan yang sama (rx ke kartu download, tx ke kartu upload).

Jadi **chart dan KPI card konsisten satu sama lain, tapi keduanya bertentangan dengan komentar di file yang sama**. Komentarnya pernah ditulis lalu kodenya diubah, atau sebaliknya, tanpa satu pun diperbarui.

### Arah mana yang benar

Dengan konvensi device-perspective (Fakta 1-2), `rx_rate` = InOctets = traffic **masuk ke device**. Untuk router ISP: paket datang dari internet via WAN (InOctets), lalu keluar ke pelanggan via LAN (OutOctets). Jadi **device tx = traffic yang diterima pelanggan = download**. Secara semantik, **komentarnya benar dan kodenya terbalik.**

### Tapi ini belum cukup untuk memutuskan

Belum ada data historis atau test di repo yang memverifikasi arah ini. Perlu konfirmasi manual: bandingkan angka chart dengan traffic meter upstream di router, atau ambil packet capture di interface WAN.

**Jangan balik pemetaan hanya karena argumen semantik.** Kalau ternyata kodenya yang benar, maka kartu download dan chart sama-sama menampilkan angka yang terbalik sejak dulu, dan itu masalah yang lebih besar.

---

## Bug 2 - Total bandwidth sekitar 2x (paling fundamental)

**Lokasi**: `backend/internal/snmp/metrics.go:438-439`

```go
totalDeltaIn += deltaIn
totalDeltaOut += deltaOut
```

Dijumlahkan untuk setiap interface aktif. Pada router, paket yang sama terhitung dua kali: sekali saat masuk WAN, sekali lagi saat keluar LAN. Jumlah `RxRate + TxRate` pada gateway menjadi **sekitar 2x throughput sebenarnya**, karena setiap byte terhitung di kedua arah.

Konsekuensi: kartu "Total Bandwidth" (`stats.js:132-134`) menampilkan angka di atas kapasitas link sebenarnya. Ini akan terlihat kalau ada router dengan throughput yang sudah diketahui, karena angkanya akan melebihi kapasitas link tersebut.

**Perbaikannya di backend, bukan frontend.** Frontend hanya menjumlahkan angka yang sudah mengembang. Opsi: pilih interface uplink/WAN saja per device, atau jangan agregasi seluruh interface untuk device gateway.

---

## Bug 3 - Split download/upload praktis meaningless

Karena Bug 2 membuat `rx` dan `tx` hampir sama (setiap arah menghitung traffic yang sama), grafik dua seri (`dashboard.js:437-438`) dan dua kartu download/upload (`stats.js:135-140`) menampilkan **split yang sebenarnya noise**, tapi ditampilkan seolah bermakna.

Kalau Bug 2 diperbaiki dengan memakai interface uplink, Bug 3 ikut teratasi. Kalau tidak, pertimbangkan menghapus atau melabeli ulang split ini agar tidak menyesatkan.

---

## Bug 4 - Komputasi diduplikasi, belum optimal

Filter dan penjumlahan kategori gateway yang identik berjalan di empat tempat, setiap 2 detik (`stats.js:2`):

| Lokasi | Yang diulang |
|---|---|
| `stats.js:114-123` | filter `router|firewall` + sum rx/tx untuk KPI card |
| `dashboard.js:431-439` | filter identik + sum rx/tx untuk chart |
| `dashboard/components/network-summary.js:41-46` | filter + sum untuk panel summary |
| `dashboard/components/top-traffic.js:44-45` | filter + sum untuk top traffic |

Empat lintasan atas array `metrics` yang sama per polling tick. Untuk puluhan perangkat mungkin belum terasa, tapi ini opportunity gratis: hitung sekali di `stats.js`, lalu teruskan objek hasilnya ke tiga konsumen lain.

**Jangan kerjakan sekarang** - ini refactor, bukan perbaikan bug. Masukkan kalau sekalian tackle Bug 1 dan Bug 2.

---

## Bug 5 - Dua masalah kecil di `stats.js`

**5.1 - Note "excluded" bisa nyangkut** (`stats.js:142-148`)

```js
if (radioBadgeEl && excludedCount > 0) {
    radioBadgeEl.style.display = 'block';
}
```

Kalau `excludedCount` pernah lebih besar dari 0 lalu menjadi 0 (misalnya perangkat diubah kategori), blok `if` tidak dieksekusi dan note **tetap tampil** dengan teks lama. Perlu cabang `else` yang menyembunyikannya.

**5.2 - Satuan tidak konsisten** (`stats.js:124-130`)

```js
if (!val || isNaN(val) || val <= 0) return '0 Mbps';   // 0 menghasilkan Mbps
...
return `${(val * 1000).toFixed(0)} Kbps`;               // 0.5 menghasilkan Kbps
```

Nilai 0 menghasilkan `0 Mbps`, sedangkan 0.5 menghasilkan `500 Kbps`. Tidak salah secara teknis, tapi terlihat aneh saat transisi dari 0 ke nilai pertama. Pertimbangkan `0 Kbps` atau konsistenkan ke Mbps.

---

## Di Luar Scope (jangan disentuh)

- Query agregat SQL di `backend/internal/database/queries.go` (line 194-195, 1331-1332) - belum diaudit, kemungkinan punya masalah konvensi serupa
- `backend/internal/tsdb/client.go` - filter kategori PromQL-nya sudah benar
- Visualisasi top-traffic dan top-interfaces - bukan soal perhitungan
- `deviceDetail.js` dan `interfaces.js` - memakai rx/tx apa adanya, tidak melakukan pemetaan download/upload

---

## Checklist

- [ ] Bug 1: konfirmasi arah rx/tx dengan data nyata, bukan argumen semantik, baru putuskan pembalikan
- [ ] Bug 2: putuskan strategi pemilihan interface uplink di backend
- [ ] Bug 3: inevitable, atau independen setelah Bug 2 selesai?
- [ ] Bug 4: lakukan saat tackle Bug 1 dan Bug 2
- [ ] Bug 5: dua perbaikan kecil, bisa dikerjakan sendiri
- [ ] Setelah semua selesai: stabilkan lewat test yang membandingkan agregasi gateway dengan interface uplink