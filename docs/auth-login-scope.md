# Ruang Lingkup Login dan Sesi

## Tujuan

Menuntaskan alur Login dari halaman sampai backend dan sesi, termasuk temuan audit yang berhubungan langsung dengan autentikasi. Perubahan tetap dibatasi pada area login dan kontrol akses.

## Keputusan

- Gunakan bahasa Indonesia pada halaman login dan dokumentasi terkait.
- Role `viewer` hanya boleh membuka Dashboard dan Alerts & Notifications.
- Viewer tidak logout otomatis selama aktif memantau; refresh sesi berjalan otomatis saat aktivitas berlangsung.
- Sesi Viewer berakhir saat logout manual, akun dinonaktifkan, atau role berubah; jangan membuat token abadi yang tidak bisa dicabut.
- Setelah browser ditutup atau komputer restart, Viewer harus login kembali; selama halaman monitoring tetap terbuka, refresh berjalan tanpa batas idle.
- Opsi `Ingat Saya` hanya boleh mengingat username, bukan sesi atau token autentikasi.
- Gangguan jaringan/server sementara tidak langsung menghapus sesi Viewer; tampilkan status koneksi dan coba pulihkan sesi kembali.
- Jika backend memastikan sesi tidak valid, akun nonaktif, atau role berubah, hapus sesi dan minta login kembali.
- Role `admin` mempertahankan hak admin; sesi memiliki batas absolut 12 jam sejak login dan tidak diperpanjang melewati batas itu.
- Jangan gunakan token yang berlaku abadi tanpa mekanisme pemeriksaan ulang dan pencabutan.
- Sesi harus dilacak di backend agar logout, penonaktifan akun, dan perubahan role dapat mencabut akses segera.
- Login produksi wajib memakai HTTPS; cookie autentikasi wajib memakai `Secure`, termasuk saat TLS berakhir di reverse proxy tepercaya.
- Seluruh temuan audit di bawah masuk ruang kerja Login, bukan hanya perubahan tampilan halaman.

## Temuan Validasi

- Frontend menandai sesi berlaku 72 jam, sedangkan backend menerbitkan JWT dan cookie selama 1 jam.
- Endpoint refresh tersedia, tetapi frontend belum menggunakannya.
- Backend menerima role `viewer`, sedangkan ACL frontend mendefinisikan `view`; nama role belum konsisten.
- Sidebar mengirim ID `settings` dan `users`, sedangkan ACL/router menggunakan `Settings` dan `User`; pencocokan persis dapat menolak akses admin.
- Pembatas percobaan login memakai map proses tanpa pembersihan entri IP gagal yang kedaluwarsa.
- Kegagalan penyimpanan activity log login diabaikan; autentikasi tetap berhasil tanpa rekaman audit.
- Database lokal memiliki tabel `users`, indeks unik pada `username`, dan dua akun yang keduanya ber-role `admin` saat audit.
- Skema pengguna belum memiliki penanda akun aktif/nonaktif; middleware sesi perlu memeriksa status tersebut.
- Satu dari dua hash password tidak berformat bcrypt, sedangkan login hanya memvalidasi bcrypt; akun terkait kemungkinan tidak dapat login.
- Hash password disimpan pada `password_hash`; respons model tidak menyertakan hash tersebut.
- Upaya login belum diuji langsung karena akan menulis activity log dan mengubah keadaan rate limit.

## Batas Implementasi

- Boleh menyentuh halaman login, layanan autentikasi/sesi, ACL/router frontend, middleware/handler autentikasi backend, serta tes yang langsung terkait.
- Refaktor terbatas: pindahkan JavaScript inline dari `frontend/public/login.html` ke modul login agar alur submit dan status UI dapat diuji; pertahankan tampilan dan struktur form.
- `auth.service.js` menjadi satu-satunya pemilik alur login, sesi, refresh, dan logout.
- Frontend hanya menyimpan sesi jika respons login sesuai kontrak backend: `data.user` berisi `id`, `username`, dan role valid. Respons tidak valid harus gagal tanpa menyimpan sesi.
- Jangan simpan token autentikasi di `localStorage` atau `sessionStorage`.
- Jangan simpan sesi autentikasi lintas browser restart; `Ingat Saya` hanya menyimpan username.
- Jangan refaktor CSS atau merombak desain login tanpa kebutuhan fungsi yang tervalidasi.
- Wajib menegakkan izin Viewer pada frontend dan endpoint backend; menyembunyikan menu saja tidak cukup.
- WebSocket Viewer hanya boleh menerima event yang diperlukan Dashboard dan Alerts; larang topik `all` dan topik admin.
- Masa berlaku JWT, cookie, dan sesi frontend harus selaras dengan kebijakan per role.
- Refresh sesi Viewer harus memeriksa ulang status akun dan role serta berjalan berkala selama halaman monitoring terbuka, tanpa bergantung pada input mouse/keyboard.
- Kegagalan refresh sementara harus dibedakan dari sesi yang dicabut; retry terkontrol dan jangan menghapus sesi akibat gangguan jaringan sementara.
- Sesi/refresh token harus dilacak di backend dan dapat dicabut; menghapus cookie saja bukan pencabutan sesi.
- Cookie autentikasi harus memakai `HttpOnly`, `Secure`, dan `SameSite` yang sesuai. Jika TLS berakhir di proxy, hanya percayai header TLS dari proxy tepercaya.
- Jangan menerima koneksi login produksi melalui HTTP tanpa TLS.
- Sesi Viewer harus dicabut saat logout manual, akun dinonaktifkan, atau role berubah.
- Sesi Admin harus kedaluwarsa maksimal 12 jam sejak login; refresh tidak boleh memperpanjang batas absolut ini.
- Akses admin pada menu dan API yang diizinkan harus tetap berfungsi setelah ID view diselaraskan.
- Pembatas login harus membatasi percobaan gagal tanpa menyimpan entri IP kedaluwarsa terus-menerus.
- Kegagalan menulis audit login harus ditangani dan dilaporkan dengan jelas, bukan diabaikan.
- Akun dengan hash non-bcrypt harus ditangani lewat reset password resmi; jangan menampilkan atau menyalin hash.
- Perubahan role harus mencabut atau memperbarui izin pada sesi aktif.
- Jangan mengubah atau mencetak kredensial, password hash, maupun data pribadi pengguna.

## Di Luar Ruang Lingkup

- Reports dan pengolahan histori laporan.
- Perombakan Dashboard atau sidebar selain pembatasan akses Viewer.
- Perubahan skema/isi database pengguna di luar kebutuhan minimal untuk melacak sesi, mencabut akses, dan memeriksa status akun.
- Implementasi pemulihan password atau pendaftaran mandiri; halaman saat ini mengarahkan pengguna ke administrator.
- Perubahan modul perangkat, alert, monitoring, atau konfigurasi jaringan yang tidak diperlukan autentikasi.

## Kriteria Penerimaan

- Viewer hanya melihat dan dapat membuka Dashboard serta Alerts & Notifications.
- Admin tetap dapat membuka fitur admin yang diizinkan; menu Settings dan User Management tidak ditolak karena beda kapitalisasi ID.
- Request API di luar izin Viewer ditolak backend, termasuk jika URL dipanggil langsung.
- Koneksi WebSocket Viewer tidak dapat berlangganan topik admin atau menerima event di luar Dashboard/Alerts.
- Viewer aktif tidak logout otomatis; refresh memeriksa status akun/role dan token tidak terekspos ke JavaScript.
- Backend dapat mencabut sesi aktif saat logout, akun dinonaktifkan, atau role berubah.
- Cookie produksi hanya dikirim melalui HTTPS dan memiliki atribut keamanan yang diwajibkan.
- Respons login yang tidak sesuai kontrak ditolak tanpa menyimpan sesi frontend.
- Viewer diminta login kembali setelah browser ditutup atau komputer restart.
- Opsi `Ingat Saya` tidak memulihkan sesi dan tidak menyimpan token autentikasi.
- Logout manual, penonaktifan akun, atau perubahan role mencabut sesi Viewer.
- Gangguan jaringan sementara menampilkan status koneksi dan retry; sesi yang ditolak backend meminta login kembali.
- Sesi Admin berakhir paling lambat 12 jam sejak login, termasuk setelah refresh.
- Percobaan login dibatasi; data rate limit kedaluwarsa dibersihkan atau dibatasi agar tidak tumbuh tanpa batas.
- Login berhasil tercatat atau kegagalan audit ditangani secara eksplisit.
- Login gagal tetap memberi pesan umum dan tidak membocorkan apakah username terdaftar.
- JavaScript halaman login dipisah dari HTML dan alur submit/status UI bisa diuji tanpa merombak tampilan.
- Tes mencakup login berhasil/gagal, masa sesi/refresh, logout, serta izin Viewer di frontend dan backend.
