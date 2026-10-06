#!/usr/bin/env bash
# run_phase1_verification.sh
# Main entrypoint for Phase 1 Residual Verification in Linux/CI/Staging.

set -eo pipefail

echo "============================================="
echo "   NMS Phase 1 Verification Runner"
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

echo "1. Linux CI Build & Race Detector"
echo "---------------------------------"
cd ../backend

if ! go build -o /dev/null ./... 2>&1 | tee $EVIDENCE_DIR/1-build.log; then
    log_status "Build" "FAIL"
else
    log_status "Build" "PASS"
fi

if ! go test -v ./... 2>&1 | tee $EVIDENCE_DIR/2-test.log; then
    log_status "Unit Test" "FAIL"
else
    log_status "Unit Test" "PASS"
fi

if command -v gcc >/dev/null 2>&1; then
    if ! CGO_ENABLED=1 go test -race -v ./... 2>&1 | tee $EVIDENCE_DIR/3-race.log; then
        log_status "Race Detector" "FAIL"
    else
        log_status "Race Detector" "PASS"
    fi
else
    echo "GCC not found. Race detector BLOCKED." | tee $EVIDENCE_DIR/3-race.log
    log_status "Race Detector" "BLOCKED"
fi

cd $VERIFICATION_ROOT

echo "2. Docker Migration Verification"
echo "--------------------------------"
if bash docker/run.sh > $EVIDENCE_DIR/4-5-docker.log 2>&1; then
    log_status "Docker Migration" "PASS"
else
    log_status "Docker Migration" "FAIL/BLOCKED"
fi

echo "3. SNMP 30-Minute Stability Harness"
echo "-----------------------------------"
if bash snmp/run.sh > $EVIDENCE_DIR/6-snmp-30m.log 2>&1; then
    log_status "SNMP 30m" "PASS"
else
    log_status "SNMP 30m" "FAIL/BLOCKED"
fi

echo "4. Database Profiling"
echo "---------------------"
cd ../backend
# Assuming CI has a MySQL container running on 127.0.0.1:3306 as set up by verify.yml
if go test -v -run TestDatabaseProfiling ./internal/database 2>&1 | tee $EVIDENCE_DIR/7-db-profile.log; then
    log_status "Database Profiling" "PASS"
else
    log_status "Database Profiling" "FAIL/BLOCKED"
fi
cd $VERIFICATION_ROOT

echo "5. WebSocket Load Test"
echo "----------------------"
cd ../backend
if go test -v -run TestWSLoad200Devices ./internal/transport/websocket 2>&1 | tee $EVIDENCE_DIR/8-websocket-load.log; then
    log_status "WebSocket Load" "PASS"
else
    log_status "WebSocket Load" "FAIL/BLOCKED"
fi
cd $VERIFICATION_ROOT

echo "6. PromQL Runtime Verification"
echo "------------------------------"
cd ../backend
if go test -v -run TestTSDBQueryHandler_Limits ./internal/api/handlers/monitoring 2>&1 | tee $EVIDENCE_DIR/9-promql-matrix.log; then
    log_status "PromQL Runtime" "PASS"
else
    log_status "PromQL Runtime" "FAIL/BLOCKED"
fi
cd $VERIFICATION_ROOT

echo "7. Graceful Shutdown Test"
echo "-------------------------"
cd ../backend
if go test -race -v -run TestGracefulShutdown ./internal/worker 2>&1 | tee $EVIDENCE_DIR/10-shutdown.log; then
    log_status "Graceful Shutdown" "PASS"
else
    log_status "Graceful Shutdown" "FAIL/BLOCKED"
fi
cd $VERIFICATION_ROOT

echo "============================================="
echo "   Verification Run Complete"
echo "   Check verification/evidence/ directory"
echo "============================================="
cat $EVIDENCE_DIR/summary.log
