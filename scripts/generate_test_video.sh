#!/usr/bin/env bash
set -euo pipefail

# Output directory for test assets
OUTPUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/test_assets"
mkdir -p "${OUTPUT_DIR}"

TARGET_FILE="${OUTPUT_DIR}/sample_source.mp4"
DURATION=30
FPS=30
GOP_SIZE=60  # Keyframe every 2 seconds at 30 fps

echo "🎬 Generating synthetic test video with motion, color grading, and audio..."
echo "Duration: ${DURATION}s | Framerate: ${FPS}fps | GOP (Keyframe Interval): ${GOP_SIZE} frames"

# Generate 30s 1080p video with testsrc2 (timestamp counter, color bars, moving element)
# and a continuous 440Hz/880Hz audio tone
ffmpeg -y \
  -f lavfi -i "testsrc2=size=1920x1080:rate=${FPS}:duration=${DURATION}" \
  -f lavfi -i "sine=frequency=440:beep_factor=2:duration=${DURATION}" \
  -vf "eq=contrast=1.2:brightness=0.05:saturation=1.3,drawtext=text='Render Farm Test Clip - %{pts\:hms}':fontcolor=white:fontsize=48:box=1:boxcolor=black@0.6:boxborderw=10:x=(w-text_w)/2:y=h-100" \
  -c:v libx264 -preset medium -g "${GOP_SIZE}" -keyint_min "${GOP_SIZE}" -sc_threshold 0 \
  -pix_fmt yuv420p -b:v 4M -maxrate 4M -bufsize 8M \
  -c:a aac -b:a 192k \
  "${TARGET_FILE}"

echo "✅ Generated: ${TARGET_FILE}"
echo "📊 Probing generated video details:"
ffprobe -v error -show_entries format=duration,size,bit_rate -show_entries stream=codec_name,width,height,r_frame_rate,nb_frames -of default=noprint_wrappers=1 "${TARGET_FILE}"
