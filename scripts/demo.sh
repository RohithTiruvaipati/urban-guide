#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

echo "=================================================================="
echo "🎬 DISTRIBUTED RENDER FARM - LIVE DEMO & VALIDATION"
echo "=================================================================="

# 1. Ensure test video exists
if [ ! -f "test_assets/sample_source.mp4" ]; then
  echo "📹 Generating 30s 1080p sample video fixture..."
  ./scripts/generate_test_video.sh >/dev/null 2>&1
  echo "✅ Sample video ready."
fi

# 2. Check and start infrastructure
echo "🐳 Ensuring Docker containers (Redpanda, Redis, MinIO, Postgres) are online..."
docker compose -f infra/docker-compose.yml up -d >/dev/null 2>&1
echo "✅ Infrastructure online."

# 3. Build binaries if not built
if [ ! -f "worker/bin/render-worker" ]; then
  echo "📦 Compiling Go Worker..."
  (cd worker && go build -o bin/render-worker ./cmd/worker)
fi

if [ ! -f "orchestrator/target/orchestrator-1.0.0-SNAPSHOT.jar" ]; then
  echo "📦 Compiling Spring Boot Orchestrator..."
  (cd orchestrator && mvn package -DskipTests -q)
fi

# 4. Start Orchestrator in background
echo "🚀 Starting Spring Boot Orchestrator (port 8080)..."
JAVA_HOME="${JAVA_HOME:-/opt/homebrew/Cellar/openjdk/26.0.1/libexec/openjdk.jdk/Contents/Home}"
"${JAVA_HOME}/bin/java" -jar orchestrator/target/orchestrator-1.0.0-SNAPSHOT.jar > /tmp/render_orchestrator.log 2>&1 &
ORCH_PID=$!

cleanup() {
  echo ""
  echo "🧹 Shutting down background demo processes..."
  kill "${ORCH_PID}" 2>/dev/null || true
  kill "${WORKER_PID}" 2>/dev/null || true
}
trap cleanup EXIT

# Wait for Spring Boot to be healthy
echo -n "⏳ Waiting for Orchestrator to become ready"
for i in {1..30}; do
  if curl -s http://localhost:8080/api/v1/health 2>/dev/null | grep -q "UP"; then
    echo " -> Ready!"
    break
  fi
  echo -n "."
  sleep 1
done

# 5. Start Go Worker in background
echo "👷 Starting Go Worker [worker-alpha] (listening to Kafka)..."
./worker/bin/render-worker --worker-id=worker-alpha > /tmp/render_worker.log 2>&1 &
WORKER_PID=$!
sleep 2

# 6. Submit Render Job
echo "📤 Submitting Render Job to POST /api/v1/jobs..."
ABS_SOURCE="$(pwd)/test_assets/sample_source.mp4"

SUBMIT_RESP=$(curl -s -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d "{
    \"sourcePath\": \"${ABS_SOURCE}\",
    \"targetChunkSec\": 5.0,
    \"codec\": \"libx264\",
    \"preset\": \"veryfast\",
    \"bitrate\": \"3M\",
    \"outputFilename\": \"demo_final_video.mp4\"
  }")

JOB_ID=$(python3 -c "import sys, json; print(json.loads(sys.argv[1]).get('jobId', ''))" "${SUBMIT_RESP}")

if [ -z "${JOB_ID}" ]; then
  echo "❌ Failed to submit job. Response: ${SUBMIT_RESP}"
  cat /tmp/render_orchestrator.log
  exit 1
fi

echo "✅ Job Created with ID: ${JOB_ID}"
echo "------------------------------------------------------------------"
echo "📊 MONITORING REAL-TIME PROGRESS (Polling Redis via GET /api/v1/jobs/${JOB_ID})"
echo "------------------------------------------------------------------"

for i in {1..60}; do
  STATUS_RESP=$(curl -s "http://localhost:8080/api/v1/jobs/${JOB_ID}")
  
  INFO=$(python3 -c "
import sys, json
try:
    d = json.loads(sys.argv[1])
    status = d.get('status', 'UNKNOWN')
    comp = d.get('completedChunks', 0)
    total = d.get('totalChunks', 0)
    pct = d.get('progressPercent', 0.0)
    out = d.get('finalOutputPath', '')
    dur = d.get('totalDurationMs', 0)
    print(f'{status}|{comp}|{total}|{pct}|{out}|{dur}')
except Exception as e:
    print(f'PARSING_ERROR|0|0|0.0||0')
" "${STATUS_RESP}")

  STATUS=$(echo "${INFO}" | cut -d'|' -f1)
  COMPLETED=$(echo "${INFO}" | cut -d'|' -f2)
  TOTAL=$(echo "${INFO}" | cut -d'|' -f3)
  PROGRESS=$(echo "${INFO}" | cut -d'|' -f4)
  FINAL_PATH=$(echo "${INFO}" | cut -d'|' -f5)
  TOTAL_MS=$(echo "${INFO}" | cut -d'|' -f6)

  printf "\r⚡ Status: %-12s | Chunks: %s/%s (%s%% complete)" "${STATUS}" "${COMPLETED}" "${TOTAL}" "${PROGRESS}"

  if [ "${STATUS}" = "COMPLETED" ]; then
    echo ""
    echo "------------------------------------------------------------------"
    echo "🎉 JOB COMPLETED & STITCHED SUCCESSFULLY!"
    echo "📁 Output File: ${FINAL_PATH}"
    echo "⏱️ Total Wall Clock Time: ${TOTAL_MS}ms"
    echo "=================================================================="
    break
  fi

  if [ "${STATUS}" = "FAILED" ]; then
    echo ""
    echo "❌ Job failed: ${STATUS_RESP}"
    exit 1
  fi

  sleep 0.5
done
