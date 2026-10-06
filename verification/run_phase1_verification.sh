#!/usr/bin/env bash
# run_phase1_verification.sh
# Main entrypoint for Phase 1 Residual Verification in Linux/CI/Staging.
# All evidence is saved to verification/evidence/ directory.

set -eo pipefail

echo "============================================="
echo "   NMS Phase 1 Verification Runner"
echo "   $(date +'%Y-%m-%dT%H:%M:%S%z')"
echo "============================================="

# Ensure directories exist relative to the script location
cd "$(dirname "$0")"
export VERIFICATION_ROOT=$(pwd)
export EVIDENCE_DIR=$VERIFICATION_ROOT/evidence
mkdir -p $EVIDENCE_DIR
rm -f $EVIDENCE_DIR/*

# Helper to log status
log_status() {
    local step=$1
    local status=$2
    echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')] $step : $status" | tee -a $EVIDENCE_DIR/summary.log
}

BACKEND_DIR="$VERIFICATION_ROOT/../backend"

# =============================================
# 1. Build
# =============================================
echo ""
echo "=== 1. go build ./... ==="
cd "$BACKEND_DIR"
if go build -o /dev/null ./... 2>&1 | tee $EVIDENCE_DIR/1-build.log; then
    log_status "Build" "PASS"
else
    log_status "Build" "FAIL"
fi

# =============================================
# 2. Unit Tests
# =============================================
echo ""
echo "=== 2. go test -v ./... ==="
cd "$BACKEND_DIR"
if go test -short -v ./... 2>&1 | tee $EVIDENCE_DIR/2-test.log; then
    log_status "Unit Test" "PASS"
else
    log_status "Unit Test" "FAIL"
fi

# =============================================
# 3. Race Detector
# =============================================
echo ""
echo "=== 3. go test -race -v ./... ==="
cd "$BACKEND_DIR"
if command -v gcc >/dev/null 2>&1; then
    if CGO_ENABLED=1 go test -short -race -v ./... 2>&1 | tee $EVIDENCE_DIR/3-race.log; then
        log_status "Race Detector" "PASS"
    else
        log_status "Race Detector" "FAIL"
    fi
else
    echo "GCC not found. Race detector BLOCKED." | tee $EVIDENCE_DIR/3-race.log
    log_status "Race Detector" "BLOCKED"
fi

# =============================================
# 4-5. Docker Migration Verification
# =============================================
echo ""
echo "=== 4-5. Docker Migration (Fresh + Fail-Fast) ==="
cd "$VERIFICATION_ROOT"
if command -v docker >/dev/null 2>&1; then
    if bash docker/run.sh 2>&1 | tee $EVIDENCE_DIR/4-5-docker.log; then
        log_status "Docker Fresh Migration" "PASS"
        log_status "Docker Fail-Fast" "PASS"
    else
        log_status "Docker Migration" "FAIL"
    fi
else
    echo "Docker not available in this environment." | tee $EVIDENCE_DIR/4-5-docker.log
    log_status "Docker Migration" "BLOCKED"
fi

# =============================================
# 6. SNMP 30-Minute Stability Test
# =============================================
echo ""
echo "=== 6. SNMP 30-Minute Continuous Polling ==="
cd "$BACKEND_DIR"
echo "Starting SNMP 30-minute stability test at $(date +'%Y-%m-%dT%H:%M:%S%z')..."
if go test -v -timeout 40m -run TestGoroutineStability30m ./internal/snmp 2>&1 | tee $EVIDENCE_DIR/6-snmp-30m.log; then
    log_status "SNMP 30m Stability" "PASS"
else
    log_status "SNMP 30m Stability" "FAIL"
fi

# =============================================
# 7. Database Profiling
# =============================================
echo ""
echo "=== 7. Database Profiling ==="
cd "$BACKEND_DIR"
if go test -v -run TestDatabaseProfiling ./internal/database 2>&1 | tee $EVIDENCE_DIR/7-db-profile.log; then
    log_status "Database Profiling" "PASS"
else
    log_status "Database Profiling" "FAIL/BLOCKED"
fi

# =============================================
# 8. WebSocket Load Test
# =============================================
echo ""
echo "=== 8. WebSocket Load (200 Devices + Concurrent Clients) ==="
cd "$BACKEND_DIR"
if go test -v -run TestWSLoad200Devices ./internal/transport/websocket 2>&1 | tee $EVIDENCE_DIR/8-websocket-load.log; then
    log_status "WebSocket Load" "PASS"
else
    log_status "WebSocket Load" "FAIL"
fi

# =============================================
# 9. PromQL Runtime Matrix
# =============================================
echo ""
echo "=== 9. PromQL Runtime Matrix ==="
cd "$BACKEND_DIR"
if go test -v -run TestTSDBQueryHandler_Limits ./internal/api/handlers/monitoring 2>&1 | tee $EVIDENCE_DIR/9-promql-matrix.log; then
    log_status "PromQL Runtime" "PASS"
else
    log_status "PromQL Runtime" "FAIL"
fi

# =============================================
# 10. Graceful Shutdown
# =============================================
echo ""
echo "=== 10. Graceful Shutdown ==="
cd "$BACKEND_DIR"
if go test -race -v -run TestGracefulShutdown ./internal/worker 2>&1 | tee $EVIDENCE_DIR/10-shutdown.log; then
    log_status "Graceful Shutdown" "PASS"
else
    log_status "Graceful Shutdown" "FAIL"
fi

# =============================================
# Summary
# =============================================
echo ""
echo "============================================="
echo "   Verification Run Complete"
echo "   $(date +'%Y-%m-%dT%H:%M:%S%z')"
echo "============================================="
echo ""
echo "--- SUMMARY ---"
cat $EVIDENCE_DIR/summary.log
echo "----------------"
echo ""
echo "Evidence artifacts saved to: $EVIDENCE_DIR/"
ls -la $EVIDENCE_DIR/
