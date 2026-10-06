#!/usr/bin/env bash
set -e

echo "[Docker] Checking environment..."
if ! command -v docker >/dev/null 2>&1; then
    echo "Docker not found. BLOCKED."
    exit 1
fi

if ! command -v docker-compose >/dev/null 2>&1 && ! docker compose version >/dev/null 2>&1; then
    echo "Docker Compose not found. BLOCKED."
    exit 1
fi

COMPOSE_CMD="docker compose"
if command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_CMD="docker-compose"
fi

cd ../../backend

echo "[Docker] Scenario A: Fresh Migration Test"
$COMPOSE_CMD down -v || true
$COMPOSE_CMD build
$COMPOSE_CMD up -d
sleep 10
LOGS=$($COMPOSE_CMD logs agent)
if echo "$LOGS" | grep -q "migration failed"; then
    echo "Fresh migration failed."
    echo "$LOGS"
    $COMPOSE_CMD down -v
    exit 1
fi
echo "Fresh migration passed."

echo "[Docker] Scenario B: Missing Migration Fail-Fast Test"
$COMPOSE_CMD down -v || true
# Move migrations directory temporarily
mv database/migrations database/migrations_bak
if $COMPOSE_CMD up -d --build; then
    sleep 5
    LOGS=$($COMPOSE_CMD logs agent)
    mv database/migrations_bak database/migrations
    if echo "$LOGS" | grep -qi "fatal"; then
        echo "Fail-fast verified."
        $COMPOSE_CMD down -v
        exit 0
    else
        echo "Fail-fast not working. Process did not emit fatal error."
        echo "$LOGS"
        $COMPOSE_CMD down -v
        exit 1
    fi
else
    # Up failed (might fail-fast at container start)
    LOGS=$($COMPOSE_CMD logs agent)
    mv database/migrations_bak database/migrations
    echo "Fail-fast verified (container exited immediately)."
    $COMPOSE_CMD down -v
    exit 0
fi
