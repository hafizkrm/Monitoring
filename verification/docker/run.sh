#!/usr/bin/env bash
set -e

echo "[Docker] Checking environment..."
if ! command -v docker >/dev/null 2>&1; then
    echo "Docker not found. BLOCKED."
    exit 1
fi

COMPOSE_CMD="docker compose"
if command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_CMD="docker-compose"
fi

cd ../../deployments/docker

echo "============================================="
echo "[Docker] Scenario A: Fresh Migration Test"
echo "============================================="
$COMPOSE_CMD down -v || true
$COMPOSE_CMD build

echo "Starting containers with fresh database..."
$COMPOSE_CMD up -d

echo "Waiting for 15 seconds for DB and Agent to startup..."
sleep 15

LOGS=$($COMPOSE_CMD logs agent)
echo "--- Agent Logs (Startup & Migration) ---"
echo "$LOGS"
echo "----------------------------------------"

if echo "$LOGS" | grep -q "migration failed"; then
    echo "ERROR: Fresh migration failed."
    $COMPOSE_CMD down -v
    exit 1
fi

if echo "$LOGS" | grep -qi "fatal"; then
    echo "ERROR: Container startup failed fatally on fresh DB."
    $COMPOSE_CMD down -v
    exit 1
fi

echo "SUCCESS: Fresh DB started, migration succeeded, application is running."

echo "============================================="
echo "[Docker] Scenario B: Missing Migration Fail-Fast Test"
echo "============================================="
$COMPOSE_CMD down -v || true

echo "Temporarily moving migrations directory to simulate missing migrations..."
mv ../../backend/internal/database/migrations ../../backend/internal/database/migrations_bak

echo "Starting agent container..."
# Build again so that the docker context picks up the missing directory
$COMPOSE_CMD build
$COMPOSE_CMD up -d || true
sleep 10

LOGS=$($COMPOSE_CMD logs agent)
EXIT_CODE=$($COMPOSE_CMD ps -q agent | xargs docker inspect -f '{{.State.ExitCode}}')

echo "--- Agent Logs (Missing Migration) ---"
echo "$LOGS"
echo "--- Exit Code: $EXIT_CODE ---"
echo "--------------------------------------"

# Restore migrations directory
mv ../../backend/internal/database/migrations_bak ../../backend/internal/database/migrations

if [ "$EXIT_CODE" -eq 0 ]; then
    echo "ERROR: Container did not fail-fast! Exit code is 0 but migrations were missing."
    $COMPOSE_CMD down -v
    exit 1
fi

if echo "$LOGS" | grep -qi "fatal"; then
    echo "SUCCESS: Fail-fast verified. Agent panicked or exited with FATAL when migrations were missing."
else
    echo "WARNING: Exit code was non-zero ($EXIT_CODE), but no 'fatal' log was detected. Assuming fail-fast worked."
fi

$COMPOSE_CMD down -v
echo "[Docker] Both Scenarios PASSED."
exit 0
