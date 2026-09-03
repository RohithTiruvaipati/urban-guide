#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RAW_DIR="${1:-${ROOT_DIR}/test_footage/raw}"
PROXY_DIR="${2:-${ROOT_DIR}/proxies}"

echo "=================================================================="
echo "🔍 PREMIERE PRO PROXY CONFORMITY AUDIT"
echo "📁 Raw Footage:  ${RAW_DIR}"
echo "📁 Proxy Folder: ${PROXY_DIR}"
echo "=================================================================="

if [ ! -d "${PROXY_DIR}" ]; then
  echo "❌ Error: Proxy directory '${PROXY_DIR}' does not exist."
  exit 1
fi

TOTAL=0
PASS=0
FAIL=0

for raw_file in "${RAW_DIR}"/*; do
  [ -f "${raw_file}" ] || continue
  BASE=$(basename "${raw_file}")
  NAME="${BASE%.*}"
  
  # Check for matching proxy
  PROXY_FILE="${PROXY_DIR}/${NAME}_Proxy.mov"
  if [ ! -f "${PROXY_FILE}" ]; then
    PROXY_FILE="${PROXY_DIR}/${NAME}_Proxy.mp4"
  fi

  TOTAL=$((TOTAL + 1))

  if [ ! -f "${PROXY_FILE}" ]; then
    echo "❌ [MISSING] No proxy found for '${BASE}'"
    FAIL=$((FAIL + 1))
    continue
  fi

  # Probe raw and proxy streams
  RAW_FRAMES=$(ffprobe -v error -select_streams v -count_frames -show_entries stream=nb_read_frames -of default=noprint_wrappers=1:nokey=1 "${raw_file}" 2>/dev/null || echo "0")
  PRX_FRAMES=$(ffprobe -v error -select_streams v -count_frames -show_entries stream=nb_read_frames -of default=noprint_wrappers=1:nokey=1 "${PROXY_FILE}" 2>/dev/null || echo "0")
  
  RAW_CHANNELS=$(ffprobe -v error -select_streams a -show_entries stream=channels -of default=noprint_wrappers=1:nokey=1 "${raw_file}" 2>/dev/null || echo "0")
  PRX_CHANNELS=$(ffprobe -v error -select_streams a -show_entries stream=channels -of default=noprint_wrappers=1:nokey=1 "${PROXY_FILE}" 2>/dev/null || echo "0")
  
  PRX_CODEC=$(ffprobe -v error -select_streams v -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1 "${PROXY_FILE}" 2>/dev/null || echo "unknown")
  PRX_RES=$(ffprobe -v error -select_streams v -show_entries stream=width,height -of csv=s=x:p=0 "${PROXY_FILE}" 2>/dev/null || echo "unknown")

  FRAME_DIFF=$(( RAW_FRAMES - PRX_FRAMES ))
  if [ "${FRAME_DIFF}" -lt 0 ]; then FRAME_DIFF=$(( -FRAME_DIFF )); fi

  if [ "${FRAME_DIFF}" -le 1 ] && [ "${RAW_CHANNELS}" -eq "${PRX_CHANNELS}" ]; then
    echo "✅ [PERFECT] ${NAME}_Proxy (${PRX_CODEC} ${PRX_RES}) | Frames: ${PRX_FRAMES}/${RAW_FRAMES} | Audio: ${PRX_CHANNELS}ch | Premiere Ready!"
    PASS=$((PASS + 1))
  else
    echo "⚠️ [MISMATCH] ${NAME}_Proxy | Frames: ${PRX_FRAMES}/${RAW_FRAMES} | Audio: ${PRX_CHANNELS}/${RAW_CHANNELS}ch"
    FAIL=$((FAIL + 1))
  fi
done

echo ""
echo "=================================================================="
echo "📊 AUDIT SUMMARY: ${PASS}/${TOTAL} Proxies Passed Strict Premiere Pro Compliance"
echo "=================================================================="

if [ "${FAIL}" -eq 0 ] && [ "${TOTAL}" -gt 0 ]; then
  echo "🎉 100% SUCCESS: All proxies can be cleanly attached via Premiere Pro 'Attach Proxies'!"
  exit 0
else
  echo "⚠️ Some proxies failed audit."
  exit 1
fi
