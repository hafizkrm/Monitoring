$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = 1

$EVIDENCE_DIR = "..\verification\evidence"
New-Item -ItemType Directory -Force -Path $EVIDENCE_DIR | Out-Null

Write-Host "1. Build Verification"
go build -o nul ./... 2>&1 | Tee-Object -FilePath "$EVIDENCE_DIR\1-build.log"

Write-Host "2. Unit Test"
go test -v ./... 2>&1 | Tee-Object -FilePath "$EVIDENCE_DIR\2-test.log"

Write-Host "3. Race Detector"
go test -race -v ./... 2>&1 | Tee-Object -FilePath "$EVIDENCE_DIR\3-race.log"

Write-Host "4 & 5. Docker Tests"
Write-Output "Docker command is not available in this Windows sandbox. Status: BLOCKED" | Tee-Object -FilePath "$EVIDENCE_DIR\4-5-docker.log"

Write-Host "6. SNMP 30-minute Polling Test"
Write-Output "TestGoroutineStability tests concurrent 50 offline simulated SNMP timeouts. A genuine 30-minute continuous run cannot be executed due to sandbox lifecycle limits. Marking as NOT VERIFIED / BLOCKED for true 30m scale." | Tee-Object -FilePath "$EVIDENCE_DIR\6-snmp-30m.log"

Write-Host "7. Database Profiling"
go test -v -run TestDatabaseProfiling ./internal/database 2>&1 | Tee-Object -FilePath "$EVIDENCE_DIR\7-db-profile.log"

Write-Host "8. WebSocket Load"
go test -v -run TestWSLoad200Devices ./internal/transport/websocket 2>&1 | Tee-Object -FilePath "$EVIDENCE_DIR\8-websocket-load.log"

Write-Host "9. PromQL Runtime Matrix"
go test -v -run TestTSDBQueryHandler_Limits ./internal/api/handlers/monitoring 2>&1 | Tee-Object -FilePath "$EVIDENCE_DIR\9-promql-matrix.log"

Write-Host "10. Graceful Shutdown"
go test -race -v -run TestGracefulShutdown ./internal/worker 2>&1 | Tee-Object -FilePath "$EVIDENCE_DIR\10-graceful-shutdown.log"

Write-Host "11. Configuration Audit"
go test -v -run TestConfig_Validate ./internal/config 2>&1 | Tee-Object -FilePath "$EVIDENCE_DIR\11-config-audit.log"

Write-Host "Done collecting evidence."
