#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

RAW_DIR="${ROOT_DIR}/test_footage/raw"
NUM_CLIPS="${1:-12}"

echo "=================================================================="
echo "📊 DISTRIBUTED PROXY SYSTEM — AUTOMATED BENCHMARK HARNESS"
echo "🎞️ Batch Size: ${NUM_CLIPS} Raw 4K Clips"
echo "=================================================================="

# Ensure binaries are built
if [ ! -f "bin/proxy-worker" ]; then
  go build -o bin/proxy-producer ./cmd/producer
  go build -o bin/proxy-worker ./cmd/worker
  go build -o bin/proxy-monitor ./cmd/monitor
fi

# Ensure test footage is generated
bash scripts/generate_test_footage.sh "${NUM_CLIPS}"

# Ensure Redis is running
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

run_batch() {
  local num_workers=$1
  local out_dir="${ROOT_DIR}/test_footage/bench_${num_workers}_workers"
  rm -rf "${out_dir}"
  mkdir -p "${out_dir}"

  # Reset Redis queue
  rcli DEL proxy:jobs proxy:stats:completed_count >/dev/null 2>&1 || true

  # Enqueue jobs
  ./bin/proxy-producer -input "${RAW_DIR}" -output "${out_dir}" -codec prores >/dev/null 2>&1

  echo "------------------------------------------------------------------"
  echo "🚀 Running Benchmark with ${num_workers} Worker(s)..."

  local pids=()
  for w in $(seq 1 "${num_workers}"); do
    ./bin/proxy-worker --worker-id="bench-worker-${w}" >/dev/null 2>&1 &
    pids+=($!)
  done

  local start_time
  start_time=$(date +%s)

  # Wait for all jobs to complete
  while true; do
    local completed
    completed=$(rcli GET proxy:stats:completed_count 2>/dev/null || echo "0")
    completed=$(echo "${completed}" | tr -d '\r\n')
    if [ -z "${completed}" ]; then completed=0; fi
    printf "\r⏳ [Workers: %d] Processed: %s/%s clips" "${num_workers}" "${completed}" "${NUM_CLIPS}"
    if [ "${completed}" -ge "${NUM_CLIPS}" ]; then
      break
    fi
    sleep 0.5
  done

  local end_time
  end_time=$(date +%s)
  local elapsed=$(( end_time - start_time ))
  if [ "${elapsed}" -le 0 ]; then elapsed=1; fi

  # Stop workers
  for pid in "${pids[@]}"; do
    kill "${pid}" 2>/dev/null || true
  done

  echo ""
  echo "✅ ${num_workers} Worker(s) Finished in: ${elapsed} seconds!"
  echo "${elapsed}"
}

# Run 1 Worker (Baseline)
TIME_1=$(run_batch 1 | tail -n 1)

# Run 2 Workers
TIME_2=$(run_batch 2 | tail -n 1)

# Run 3 Workers
TIME_3=$(run_batch 3 | tail -n 1)

# Calculate speedup and efficiency
calc_speedup() {
  local t1=$1
  local tn=$2
  python3 -c "print(f'{(float($t1)/float($tn)):.2f}x')"
}

calc_eff() {
  local t1=$1
  local tn=$2
  local n=$3
  python3 -c "print(f'{((float($t1)/(float($tn)*float($n)))*100.0):.1f}%')"
}

SPEEDUP_1="1.00x"
EFF_1="100.0%"

SPEEDUP_2=$(calc_speedup "${TIME_1}" "${TIME_2}")
EFF_2=$(calc_eff "${TIME_1}" "${TIME_2}" 2)

SPEEDUP_3=$(calc_speedup "${TIME_1}" "${TIME_3}")
EFF_3=$(calc_eff "${TIME_1}" "${TIME_3}" 3)

echo ""
echo "=================================================================="
echo "🏆 BENCHMARK RESULTS (Batch: ${NUM_CLIPS} 4K Clips -> ProRes Proxy)"
echo "=================================================================="
echo ""
echo "| Workers | Wall-Clock Time | Speedup | Scaling Efficiency |"
echo "| :--- | :--- | :--- | :--- |"
echo "| **1 Worker (Baseline)** | ${TIME_1}s | ${SPEEDUP_1} | ${EFF_1} |"
echo "| **2 Workers** | ${TIME_2}s | ${SPEEDUP_2} | ${EFF_2} |"
echo "| **3 Workers** | ${TIME_3}s | ${SPEEDUP_3} | ${EFF_3} |"
echo ""
echo "=================================================================="
