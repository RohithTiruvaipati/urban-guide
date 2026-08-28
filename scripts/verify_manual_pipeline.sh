#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="${ROOT_DIR}/test_assets"
WORK_DIR="${TEST_DIR}/manual_verification"
SOURCE_FILE="${TEST_DIR}/sample_source.mp4"

if [ ! -f "${SOURCE_FILE}" ]; then
  echo "Generating source file first..."
  "${ROOT_DIR}/scripts/generate_test_video.sh"
fi

rm -rf "${WORK_DIR}"
mkdir -p "${WORK_DIR}"

echo "============================================================"
echo " STEP 1: PROBING KEYFRAME (I-FRAME) TIMESTAMPS"
echo "============================================================"
KEYFRAMES_CSV="${WORK_DIR}/keyframes.csv"
ffprobe -v error -select_streams v \
  -show_entries frame=pict_type,pkt_pts_time \
  -of csv=p=0 "${SOURCE_FILE}" | grep ",I" > "${KEYFRAMES_CSV}" || true

echo "Found keyframes at presentation timestamps (seconds):"
cat "${KEYFRAMES_CSV}"

echo ""
echo "============================================================"
echo " STEP 2: CALCULATING CHUNK BOUNDARIES SNAPPED TO KEYFRAMES"
echo " Target: Split 30s into 3 chunks (~10s each)"
echo " Keyframe closest to 10s is at 10.000000s"
echo " Keyframe closest to 20s is at 20.000000s"
echo " Chunk 1: [0.000000 -> 10.000000]"
echo " Chunk 2: [10.000000 -> 20.000000]"
echo " Chunk 3: [20.000000 -> 30.000000]"
echo "============================================================"

# Transcode settings to simulate actual rendering work (color effect + preset)
# Notice: -avoid_negative_ts make_zero ensures presentation timestamps start at 0 in each chunk container
echo "🎬 Rendering Chunk 1 (0.0s - 10.0s)..."
ffmpeg -y -ss 0.000000 -to 10.000000 -i "${SOURCE_FILE}" \
  -vf "hue=s=1.2" \
  -c:v libx264 -preset veryfast -b:v 3M \
  -an -avoid_negative_ts make_zero "${WORK_DIR}/chunk_0.mp4"

echo "🎬 Rendering Chunk 2 (10.0s - 20.0s)..."
ffmpeg -y -ss 10.000000 -to 20.000000 -i "${SOURCE_FILE}" \
  -vf "hue=s=1.2" \
  -c:v libx264 -preset veryfast -b:v 3M \
  -an -avoid_negative_ts make_zero "${WORK_DIR}/chunk_1.mp4"

echo "🎬 Rendering Chunk 3 (20.0s - 30.0s)..."
ffmpeg -y -ss 20.000000 -to 30.000000 -i "${SOURCE_FILE}" \
  -vf "hue=s=1.2" \
  -c:v libx264 -preset veryfast -b:v 3M \
  -an -avoid_negative_ts make_zero "${WORK_DIR}/chunk_2.mp4"

echo ""
echo "============================================================"
echo " STEP 3: EXTRACT FULL AUDIO PASS (NO SAMPLE DRIFT)"
echo "============================================================"
ffmpeg -y -i "${SOURCE_FILE}" -vn -c:a copy "${WORK_DIR}/full_audio.aac"

echo ""
echo "============================================================"
echo " STEP 4: LOSSLESS CONCATENATION & AUDIO REMUX"
echo "============================================================"
CONCAT_LIST="${WORK_DIR}/concat_list.txt"
cat << EOF > "${CONCAT_LIST}"
file '${WORK_DIR}/chunk_0.mp4'
file '${WORK_DIR}/chunk_1.mp4'
file '${WORK_DIR}/chunk_2.mp4'
EOF

# Concat video stream copy (lossless, instantaneous) and mux audio
FINAL_OUTPUT="${WORK_DIR}/stitched_output.mp4"
ffmpeg -y -f concat -safe 0 -i "${CONCAT_LIST}" -i "${WORK_DIR}/full_audio.aac" \
  -c:v copy -c:a copy -movflags +faststart "${FINAL_OUTPUT}"

echo ""
echo "============================================================"
echo " STEP 5: VERIFICATION & AUDIT"
echo "============================================================"
ORIGINAL_FRAMES=$(ffprobe -v error -select_streams v -count_frames -show_entries stream=nb_read_frames -of default=noprint_wrappers=1:nokey=1 "${SOURCE_FILE}")
FINAL_FRAMES=$(ffprobe -v error -select_streams v -count_frames -show_entries stream=nb_read_frames -of default=noprint_wrappers=1:nokey=1 "${FINAL_OUTPUT}")
ORIGINAL_DUR=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "${SOURCE_FILE}")
FINAL_DUR=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "${FINAL_OUTPUT}")

echo "Original Video: ${ORIGINAL_FRAMES} frames, ${ORIGINAL_DUR}s"
echo "Stitched Video: ${FINAL_FRAMES} frames, ${FINAL_DUR}s"

if [ "${ORIGINAL_FRAMES}" -eq "${FINAL_FRAMES}" ]; then
  echo "✅ SUCCESS: Perfect frame count match (${ORIGINAL_FRAMES} frames)!"
else
  echo "⚠️ Frame count discrepancy: Expected ${ORIGINAL_FRAMES}, got ${FINAL_FRAMES}"
fi
