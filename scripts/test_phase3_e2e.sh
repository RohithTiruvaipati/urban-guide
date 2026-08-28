#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_FILE="${ROOT_DIR}/test_assets/sample_source.mp4"

echo "=================================================================="
echo "🚀 DISTRIBUTED RENDER FARM - PHASE 3 END-TO-END ORCHESTRATION"
echo "=================================================================="

if [ ! -f "${SOURCE_FILE}" ]; then
  echo "🎬 Generating test video first..."
  "${ROOT_DIR}/scripts/generate_test_video.sh"
fi

echo "1. Ensuring local infrastructure (Redpanda, Redis) is running..."
echo "   Run: docker compose -f ${ROOT_DIR}/infra/docker-compose.yml up -d"

echo "2. Building Go worker..."
(cd "${ROOT_DIR}/worker" && go build -o "${ROOT_DIR}/worker/bin/render-worker" ./cmd/worker)
echo "✅ Go worker built at worker/bin/render-worker"

echo "3. Building Spring Boot orchestrator..."
(cd "${ROOT_DIR}/orchestrator" && mvn package -DskipTests -q)
echo "✅ Spring Boot orchestrator built at orchestrator/target/orchestrator-1.0.0-SNAPSHOT.jar"

echo ""
echo "=================================================================="
echo "🎯 HOW TO RUN THE DISTRIBUTED SYSTEM:"
echo "=================================================================="
echo "Terminal 1 (Infrastructure):"
echo "  docker compose -f infra/docker-compose.yml up -d"
echo ""
echo "Terminal 2 (Spring Boot Orchestrator):"
echo "  cd orchestrator && java -jar target/orchestrator-1.0.0-SNAPSHOT.jar"
echo ""
echo "Terminal 3 (Go Worker 1):"
echo "  ./worker/bin/render-worker --worker-id=worker-alpha"
echo ""
echo "Terminal 4 (Submit Render Job via REST API):"
cat << 'EOF'
  curl -X POST http://localhost:8080/api/v1/jobs \
    -H "Content-Type: application/json" \
    -d '{
      "sourcePath": "'$(pwd)'/test_assets/sample_source.mp4",
      "targetChunkSec": 5.0,
      "codec": "libx264",
      "preset": "veryfast",
      "bitrate": "3M",
      "outputFilename": "final_orchestrated_video.mp4"
    }'
EOF
echo ""
echo "Terminal 4 (Check Live Progress from Redis):"
echo "  curl http://localhost:8080/api/v1/jobs/{jobId}"
echo "=================================================================="
