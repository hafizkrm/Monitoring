@echo off
title NMS - Network Monitoring System Agent
cls

rem Otomatis mendeteksi direktori projek meskipun script dipindahkan ke Desktop
set "PROJECT_DIR=%~dp0"
if not exist "%PROJECT_DIR%backend\config.yaml" (
    if exist "c:\xampp\htdocs\monitoring-network\backend\config.yaml" (
        set "PROJECT_DIR=c:\xampp\htdocs\monitoring-network\"
    )
)

rem Pindah ke direktori projek utama
cd /d "%PROJECT_DIR%"

echo ========================================================
echo        NETWORK MONITORING SYSTEM (NMS) AGENT           
echo ========================================================
echo [INFO] Direktori Projek: %CD%
echo.

rem 1. Validasi file konfigurasi
if not exist "backend\config.yaml" (
    echo [ERROR] File konfigurasi "backend\config.yaml" tidak ditemukan!
    echo        Lokasi yang diperiksa: %CD%\backend\config.yaml
    echo Harap pastikan folder projek monitoring-network berada di lokasi yang tepat.
    echo.
    pause
    exit /b 1
)

rem 2. Memastikan folder build tersedia
if not exist "build" mkdir "build"

rem 3. Bersihkan proses agent dan prometheus lama di background (mencegah bentrok port)
echo [1/5] Menghentikan sisa proses agent dan TSDB di background...
taskkill /F /IM nms-agent.exe >nul 2>&1
taskkill /F /IM prometheus.exe >nul 2>&1

rem 4. Kompilasi otomatis untuk memastikan selalu versi terbaru
echo [2/5] Memeriksa dan memperbarui biner backend...
where go >nul 2>&1
if %ERRORLEVEL%==0 (
    echo       Membangun biner Go terbaru...
    pushd "%CD%\backend"
    go build -o "..\build\nms-agent.exe" ./cmd/agent
    if %ERRORLEVEL%==0 (
        popd
        echo       [OK] Biner ter-update dan siap dijalankan.
    ) else (
        popd
        echo [WARNING] Kompilasi Go gagal! Menggunakan biner yang tersedia.
        echo.
    )
) else (
    echo       Go compiler tidak ditemukan. Menggunakan biner build\nms-agent.exe yang ada.
)

rem 5. Kompilasi otomatis untuk frontend (Dinonaktifkan agar startup cepat)
echo [3/5] Melewati build frontend (gunakan npm run build secara manual jika perlu)...
rem where npm >nul 2>&1
rem if %ERRORLEVEL%==0 (
rem     echo       Membangun frontend terbaru...
rem     pushd "%CD%\frontend"
rem     call npm install >nul 2>&1
rem     call npm run build >nul 2>&1
rem     popd
rem     echo       [OK] Frontend ter-update.
rem ) else (
rem     echo       NPM tidak ditemukan. Menggunakan frontend/dist yang ada.
rem )

rem 6. Validasi keberadaan biner nms-agent.exe
if not exist "build\nms-agent.exe" (
    echo [ERROR] Biner "build\nms-agent.exe" tidak ditemukan!
    echo.
    pause
    exit /b 1
)

rem 7. Jalankan Prometheus TSDB di background (jika tersedia)
echo [4/5] Menjalankan Prometheus TSDB...
if exist "prometheus\prometheus.exe" (
    if exist "prometheus.yml" (
        start /B "Prometheus TSDB" "%CD%\prometheus\prometheus.exe" --config.file="%CD%\prometheus.yml" --storage.tsdb.retention.time=15d --web.listen-address=:9090 > prometheus.log 2>&1
        echo       [OK] Prometheus TSDB berjalan di background pada http://localhost:9090
    ) else (
        echo       [SKIP] prometheus.yml tidak ditemukan, TSDB dilewati.
    )
) else (
    echo       [SKIP] prometheus.exe tidak ditemukan, TSDB dilewati.
    echo       Unduh dari https://prometheus.io/download/ lalu extract ke folder prometheus\
)

rem 8. Jalankan Agent Server
echo [5/5] Menjalankan Monitoring Agent Server...
echo ========================================================
echo  Dashboard NMS siap diakses di: http://localhost:8080
echo  Prometheus TSDB tersedia di:   http://localhost:9090
echo ========================================================
echo.

pushd "%CD%\backend"
"..\build\nms-agent.exe" -config "config.yaml"
popd

rem 9. Cleanup Otomatis Saat Agent Berhenti dan Jendela Ditutup
echo.
echo ========================================================
echo [INFO] Menghentikan server dan membersihkan background...
echo ========================================================
taskkill /F /IM nms-agent.exe >nul 2>&1
taskkill /F /IM prometheus.exe >nul 2>&1
echo [OK] Seluruh proses NMS dan TSDB telah dihentikan secara 100%% bersih.
echo.
pause

