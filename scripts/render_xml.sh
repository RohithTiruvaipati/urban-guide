#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

XML_PATH="${1:-}"

if [ -z "${XML_PATH}" ]; then
  echo "Usage: ./scripts/render_xml.sh <path_to_video.xml>"
  echo "Example: ./scripts/render_xml.sh ~/Downloads/video.xml"
  exit 1
fi

if [ ! -f "${XML_PATH}" ]; then
  echo "❌ Error: XML file not found at '${XML_PATH}'"
  exit 1
fi

ABS_XML=$(cd "$(dirname "${XML_PATH}")" && pwd)/$(basename "${XML_PATH}")

echo "=================================================================="
echo "🎬 SUBMITTING PREMIERE PRO XML TIMELINE TO RENDER FARM"
echo "📄 XML File: ${ABS_XML}"
echo "=================================================================="

# Check if orchestrator is running on port 8080
if ! curl -s http://localhost:8080/api/v1/health 2>/dev/null | grep -q "UP"; then
  echo "⚠️ Orchestrator is not running on http://localhost:8080."
  echo "👉 Start services in separate terminals:"
  echo "   Terminal 1 (Infrastructure): yarn infra:up"
  echo "   Terminal 2 (Orchestrator):   yarn dev:orchestrator"
  echo "   Terminal 3 (Worker Pool):    yarn dev:worker"
  exit 1
fi

# Submit the XML render job
echo "🚀 Submitting XML job to Orchestrator..."
SUBMIT_RESP=$(curl -s -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d "{
    \"sourcePath\": \"${ABS_XML}\",
    \"codec\": \"libx264\",
    \"preset\": \"medium\",
    \"crf\": 18,
    \"outputFilename\": \"premiere_rendered_timeline.mp4\"
  }")

JOB_ID=$(python3 -c "import sys, json; print(json.loads(sys.argv[1]).get('jobId', ''))" "${SUBMIT_RESP}" 2>/dev/null || echo "")

if [ -z "${JOB_ID}" ]; then
  echo "❌ Failed to submit job. Server response:"
  echo "${SUBMIT_RESP}"
  exit 1
fi

echo "✅ Job Accepted! ID: ${JOB_ID}"
echo "📊 Monitoring progress..."

for i in {1..120}; do
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
    err = d.get('errorMessage', '')
    print(f'{status}|{comp}|{total}|{pct}|{out}|{dur}|{err}')
except Exception as e:
    print(f'PARSING_ERROR|0|0|0.0||0|{e}')
" "${STATUS_RESP}")

  STATUS=$(echo "${INFO}" | cut -d'|' -f1)
  COMPLETED=$(echo "${INFO}" | cut -d'|' -f2)
  TOTAL=$(echo "${INFO}" | cut -d'|' -f3)
  PROGRESS=$(echo "${INFO}" | cut -d'|' -f4)
  FINAL_PATH=$(echo "${INFO}" | cut -d'|' -f5)
  TOTAL_MS=$(echo "${INFO}" | cut -d'|' -f6)
  ERR_MSG=$(echo "${INFO}" | cut -d'|' -f7)

  printf "\r⚡ Status: %-12s | Cuts Rendered: %s/%s (%s%% complete)" "${STATUS}" "${COMPLETED}" "${TOTAL}" "${PROGRESS}"

  if [ "${STATUS}" = "COMPLETED" ]; then
    echo ""
    echo "=================================================================="
    echo "🎉 TIMELINE RENDERED & STITCHED SUCCESSFULLY!"
    echo "📁 Output Video: ${FINAL_PATH}"
    echo "⏱️ Render Time: ${TOTAL_MS}ms"
    echo "=================================================================="
    break
  fi

  if [ "${STATUS}" = "FAILED" ]; then
    echo ""
    echo "❌ Job failed: ${ERR_MSG}"
    exit 1
  fi

  sleep 0.5
done
