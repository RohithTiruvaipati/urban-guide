#!/usr/bin/env bash
set -euo pipefail

echo "=================================================================="
echo "🛠️ SETTING UP DISTRIBUTED RENDER FARM ENVIRONMENT"
echo "=================================================================="

# Check Docker / Colima
if ! docker info >/dev/null 2>&1; then
  echo "🐳 Docker daemon is not running. Starting Colima..."
  colima start || true
fi

echo "📦 Building Worker and Orchestrator..."
(cd worker && go build -o bin/render-worker ./cmd/worker)
(cd orchestrator && mvn package -DskipTests -q)

echo "🎬 Checking test video fixture..."
if [ ! -f "test_assets/sample_source.mp4" ]; then
  ./scripts/generate_test_video.sh
fi

echo "=================================================================="
echo "✅ Environment setup complete!"
echo "You can now run:"
echo "  yarn infra:up        # Start Redpanda, Redis, Postgres, MinIO"
echo "  yarn dev:orchestrator # Run Spring Boot control plane"
echo "  yarn dev:worker       # Run Go distributed worker"
echo "=================================================================="
