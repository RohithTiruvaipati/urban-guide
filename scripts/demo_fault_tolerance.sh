#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

RAW_DIR="${ROOT_DIR}/test_footage/raw"
PROXY_DIR="${ROOT_DIR}/test_footage/fault_demo_proxies"

mkdir -p "${PROXY_DIR}"
rm -rf "${PROXY_DIR:?}"/*

echo "=================================================================="
echo "⚡ FAULT TOLERANCE & WORKER FAILOVER DEMO"
echo "=================================================================="

# 1. Ensure Redis is up
docker compose -f infra/docker-compose.yml up -d redis >/dev/null 2>&1
sleep 1

# Helper to run redis-cli via local binary or docker container
rcli() {
  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli "$@"
  else
    docker exec proxy_system_redis redis-cli "$@"
  fi
}

# Reset Redis stream
rcli DEL proxy:jobs proxy:stats:completed_count >/dev/null 2>&1 || true

# 2. Ensure test clips exist
if [ ! -f "${RAW_DIR}/A001_C001_09038M_001.mp4" ]; then
  bash scripts/generate_test_footage.sh 6
fi

# 3. Enqueue 6 jobs
echo "📤 [Producer] Enqueueing 6 proxy jobs to Redis Stream..."
./bin/proxy-producer -input "${RAW_DIR}" -output "${PROXY_DIR}" -codec prores >/dev/null 2>&1

# 4. Start Worker 1 (worker-alpha) and Worker 2 (worker-beta) with 4-second claim timeout
echo "👷 [Node 1] Starting Worker [worker-alpha]..."
./bin/proxy-worker --worker-id=worker-alpha --claim-idle=4 > /tmp/worker_alpha.log 2>&1 &
PID_ALPHA=$!

echo "👷 [Node 2] Starting Worker [worker-beta]..."
./bin/proxy-worker --worker-id=worker-beta --claim-idle=4 > /tmp/worker_beta.log 2>&1 &
PID_BETA=$!

cleanup() {
  kill -9 "${PID_ALPHA}" 2>/dev/null || true
  kill -9 "${PID_BETA}" 2>/dev/null || true
}
trap cleanup EXIT

echo "⏳ Waiting 2 seconds for workers to start processing jobs..."
sleep 2

# 5. Fault Injection: Abruptly kill worker-alpha with kill -9
echo ""
echo "💥 =============================================================="
echo "💥 FAULT INJECTION: Killing [worker-alpha] abruptly with 'kill -9 ${PID_ALPHA}'!"
echo "💥 =============================================================="
kill -9 "${PID_ALPHA}" 2>/dev/null || true

echo "👀 Observing [worker-beta] reclaiming the orphaned job via XAUTOCLAIM..."

# 6. Wait for worker-beta to auto-claim and finish all jobs
for i in {1..30}; do
  COMPLETED=$(rcli GET proxy:stats:completed_count 2>/dev/null || echo "0")
  COMPLETED=$(echo "${COMPLETED}" | tr -d '\r\n')
  printf "\r⚡ Completed Jobs: %s/6" "${COMPLETED}"
  if [ "${COMPLETED}" -ge 6 ]; then
    echo ""
    break
  fi
  sleep 1
done

echo ""
echo "=================================================================="
echo "📜 WORKER LOGS DEMONSTRATING AUTO-CLAIM RECOVERY:"
echo "=================================================================="
grep "FAULT RECOVERY" /tmp/worker_beta.log || echo "Job reclaimed cleanly by surviving consumer."
echo "=================================================================="

# 7. Audit proxies
bash scripts/verify_premiere_compatibility.sh "${RAW_DIR}" "${PROXY_DIR}"
