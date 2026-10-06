#!/usr/bin/env bash
set -e

echo "[PromQL] Runtime Verification"

cd ../../backend

if go test -v -run TestTSDBQueryHandler_Limits ./internal/api/handlers/monitoring; then
    echo "PromQL Runtime Test passed."
    exit 0
else
    echo "PromQL Runtime Test failed."
    exit 1
fi
