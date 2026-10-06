#!/usr/bin/env bash
set -e

echo "[SNMP] Checking environment..."
cd ../../backend

echo "[SNMP] Running 30-minute stability test..."
# Go test default timeout is 10m, we need to extend it
if go test -v -timeout 40m -run TestGoroutineStability30m ./internal/snmp; then
    echo "SNMP 30m test passed."
    exit 0
else
    echo "SNMP 30m test failed."
    exit 1
fi
