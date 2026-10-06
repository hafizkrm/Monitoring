#!/usr/bin/env bash
# run_phase1_verification.sh
# Main entrypoint for Phase 1 Residual Verification in Linux/CI/Staging.

set -eo pipefail

echo "============================================="
echo "   NMS Phase 1 Verification Runner"
echo "============================================="

# Ensure directories exist
mkdir -p evidence
rm -f evidence/*

# Helper to log status
log_status() {
    local step=$1
    local status=$2
    echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')] $step : $status" | tee -a evidence/summary.log
}

export VERIFICATION_ROOT=$(pwd)
export EVIDENCE_DIR=$VERIFICATION_ROOT/evidence

echo "1. Linux CI Build & Race Detector"
echo "---------------------------------"
cd ../backend
go env > $EVIDENCE_DIR/go-env.log
if ! go build -o /dev/null ./... 2>&1 | tee $EVIDENCE_DIR/build.log; then
    log_status "Build" "FAIL"
else
    log_status "Build" "PASS"
fi

if ! go test -v ./... 2>&1 | tee $EVIDENCE_DIR/test.log; then
    log_status "Unit Test" "FAIL"
else
    log_status "Unit Test" "PASS"
fi

if command -v gcc >/dev/null 2>&1; then
    if ! CGO_ENABLED=1 go test -race -v ./... 2>&1 | tee $EVIDENCE_DIR/race.log; then
        log_status "Race Detector" "FAIL"
    else
        log_status "Race Detector" "PASS"
    fi
else
    echo "GCC not found. Race detector BLOCKED." | tee $EVIDENCE_DIR/race.log
    log_status "Race Detector" "BLOCKED"
fi

cd $VERIFICATION_ROOT

echo "2. Docker Migration Verification"
echo "--------------------------------"
if bash docker/run.sh > $EVIDENCE_DIR/docker.log 2>&1; then
    log_status "Docker Migration" "PASS"
else
    log_status "Docker Migration" "FAIL/BLOCKED"
fi

echo "3. SNMP 30-Minute Stability Harness"
echo "-----------------------------------"
if bash snmp/run.sh > $EVIDENCE_DIR/snmp-30m.log 2>&1; then
    log_status "SNMP 30m" "PASS"
else
    log_status "SNMP 30m" "FAIL/BLOCKED"
fi

echo "4. Database Profiling"
echo "---------------------"
if bash database/run.sh > $EVIDENCE_DIR/db-profile.log 2>&1; then
    log_status "Database Profiling" "PASS"
else
    log_status "Database Profiling" "FAIL/BLOCKED"
fi

echo "5. WebSocket Load Test"
echo "----------------------"
if bash websocket/run.sh > $EVIDENCE_DIR/websocket-load.log 2>&1; then
    log_status "WebSocket Load" "PASS"
else
    log_status "WebSocket Load" "FAIL/BLOCKED"
fi

echo "6. PromQL Runtime Verification"
echo "------------------------------"
if bash promql/run.sh > $EVIDENCE_DIR/promql-runtime.log 2>&1; then
    log_status "PromQL Runtime" "PASS"
else
    log_status "PromQL Runtime" "FAIL/BLOCKED"
fi

echo "7. Graceful Shutdown Test"
echo "-------------------------"
if bash shutdown/run.sh > $EVIDENCE_DIR/shutdown.log 2>&1; then
    log_status "Graceful Shutdown" "PASS"
else
    log_status "Graceful Shutdown" "FAIL/BLOCKED"
fi

echo "============================================="
echo "   Verification Run Complete"
echo "   Check verification/evidence/ directory"
echo "============================================="
cat evidence/summary.log
