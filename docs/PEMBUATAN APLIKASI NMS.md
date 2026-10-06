# Stack Teknologi Aplikasi Viscod (Network Monitoring Agent)

Aplikasi ini adalah sebuah sistem **Network Monitoring** (Viscod) yang terbagi menjadi dua bagian utama: **Backend** dan **Frontend**. Berikut adalah rincian teknologi dan pustaka (library) yang digunakan dalam aplikasi ini:

## 1. Backend (Go / Golang)
Backend aplikasi ini dibangun menggunakan bahasa pemrograman **Go (Golang)** versi 1.26.0. 

Berikut adalah teknologi dan library utama yang digunakan pada Backend:
- **Routing & HTTP Server:** Menggunakan `go-chi/chi` untuk routing API yang ringan dan cepat, serta `go-chi/cors` untuk menangani kebijakan CORS.
- **Database:** Menggunakan **MySQL** sebagai sistem manajemen basis data relasional (RDBMS), dihubungkan dengan driver `go-sql-driver/mysql`.
- **Autentikasi & Keamanan:** Menggunakan `golang-jwt/jwt` untuk implementasi JSON Web Token (JWT) pada autentikasi, serta `golang.org/x/crypto` untuk enkripsi data.
- **Komunikasi Real-time:** Menggunakan `gorilla/websocket` untuk fitur WebSocket (biasanya untuk memberikan update pemantauan jaringan secara real-time).
- **Protokol Jaringan & Monitoring:**
  - `go-ping/ping`: Digunakan untuk melakukan ICMP Ping ke perangkat jaringan.
  - `gosnmp/gosnmp`: Digunakan untuk memantau perangkat melalui protokol SNMP.
  - `routeros.v2`: Digunakan untuk berinteraksi dengan API perangkat MikroTik (RouterOS).
- **Logging:** Menggunakan `natefinch/lumberjack` untuk manajemen rotasi file log.
- **Konfigurasi:** Menggunakan `yaml.v3` untuk memproses file konfigurasi berekstensi `.yaml` atau `.yml`.
- **Development & Build:** Menggunakan **Air** (`.air.toml`) untuk fitur *live-reloading* atau *hot-reload* saat pengembangan backend, dan **Makefile** untuk skrip build, test, dan deployment.
- **Deployment:** Mendukung instalasi sebagai service *systemd* di Linux dan containerisasi menggunakan **Docker** / **Docker Compose**.

## 2. Frontend (HTML, CSS, JavaScript)
Frontend aplikasi ini adalah antarmuka web berbasis Vanilla JavaScript/HTML yang menggunakan *modern build tools*.

Berikut adalah teknologi dan library utama yang digunakan pada Frontend:
- **Build Tool & Dev Server:** Menggunakan **Vite** yang sangat cepat untuk *bundling* dan *development server*. Terdapat juga plugin `vite-plugin-purgecss` untuk mengoptimasi ukuran file CSS dengan menghapus CSS yang tidak terpakai.
- **Visualisasi Data:** Menggunakan **Chart.js** untuk menampilkan grafik pemantauan jaringan (seperti grafik traffic, ping, CPU usage, dll).
- **Ikon:** Menggunakan **FontAwesome** (`@fortawesome/fontawesome-free`) untuk ikon antarmuka (UI).
- **Manajemen Package:** Menggunakan **npm** (terlihat dari adanya `package.json` dan `package-lock.json`).

## Kesimpulan
Aplikasi Viscod ini merupakan sistem pemantauan jaringan berkinerja tinggi. Backend Golang bertugas melakukan komunikasi intensif dengan perangkat keras jaringan (via ICMP, SNMP, RouterOS API) dan menyimpannya di database MySQL. Data pemantauan tersebut kemudian disajikan kepada pengguna secara visual dengan Chart.js di antarmuka web yang di-build menggunakan Vite.
