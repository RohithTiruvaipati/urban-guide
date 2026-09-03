#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FOOTAGE_DIR="${ROOT_DIR}/test_footage/raw"
NUM_CLIPS="${1:-6}" # Default 6 clips for fast test, can be 20 for full benchmark

mkdir -p "${FOOTAGE_DIR}"

echo "=================================================================="
echo "🎬 GENERATING SYNTHETIC 4K RAW TEST FOOTAGE BATCH"
echo "📁 Target Folder: ${FOOTAGE_DIR}"
echo "🎞️ Total Clips:   ${NUM_CLIPS}"
echo "=================================================================="

COLORS=("0xd9534f" "0x0275d8" "0x5cb85c" "0xf0ad4e" "0x6f42c1" "0x20c997")
FREQS=("440" "523.25" "659.25" "783.99" "880" "1046.5")

for i in $(seq 1 "${NUM_CLIPS}"); do
  CLIP_NAME=$(printf "A001_C%03d_09038M_001.mp4" "${i}")
  CLIP_PATH="${FOOTAGE_DIR}/${CLIP_NAME}"
  
  if [ -f "${CLIP_PATH}" ]; then
    echo "⚡ [${i}/${NUM_CLIPS}] Exists: ${CLIP_NAME}"
    continue
  fi

  COLOR_IDX=$(( (i - 1) % 6 ))
  COLOR="${COLORS[$COLOR_IDX]}"
  FREQ="${FREQS[$COLOR_IDX]}"
  DURATION=4 # 4 seconds 4K clip

  echo "📹 [${i}/${NUM_CLIPS}] Rendering 4K Source: ${CLIP_NAME} (Color: ${COLOR}, Tone: ${FREQ}Hz, Dur: ${DURATION}s)..."
  
  ffmpeg -y -hide_banner -loglevel error \
    -f lavfi -i "color=c=${COLOR}:size=3840x2160:rate=30:duration=${DURATION}" \
    -f lavfi -i "sine=frequency=${FREQ}:duration=${DURATION}" \
    -vf "drawtext=text='RAW 4K MASTER - Clip #${i} - %{pts\:hms}':fontcolor=white:fontsize=72:box=1:boxcolor=black@0.6:boxborderw=15:x=(w-text_w)/2:y=(h-text_h)/2" \
    -c:v libx264 -preset ultrafast -pix_fmt yuv420p -b:v 25M \
    -c:a aac -b:a 192k \
    "${CLIP_PATH}"
done

echo ""
echo "✅ Successfully generated ${NUM_CLIPS} raw 4K clips in ${FOOTAGE_DIR}!"
echo "📊 Summary:"
ls -lh "${FOOTAGE_DIR}" | awk '{print $9, $5}' | tail -n "${NUM_CLIPS}"
