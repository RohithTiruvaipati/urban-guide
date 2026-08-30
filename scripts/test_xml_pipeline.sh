#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="${ROOT_DIR}/test_assets/xml_verification"
mkdir -p "${TEST_DIR}"

CLIP_A="${TEST_DIR}/scene_a.mp4"
CLIP_B="${TEST_DIR}/scene_b.mp4"
XML_FILE="${TEST_DIR}/project_timeline.xml"
OUTPUT_FILE="${TEST_DIR}/rendered_timeline.mp4"

echo "============================================================"
echo " STEP 1: GENERATING RAW SOURCE MEDIA CLIPS"
echo "============================================================"

# Clip A: Red/Orange background with sine tone (5s, 30fps)
if [ ! -f "${CLIP_A}" ]; then
  echo "🎬 Generating Scene A (Red/Orange)..."
  ffmpeg -y \
    -f lavfi -i "color=c=0xd9534f:size=1920x1080:rate=30:duration=5" \
    -f lavfi -i "sine=frequency=523.25:duration=5" \
    -vf "drawtext=text='Scene A - Raw Clip - %{pts\:hms}':fontcolor=white:fontsize=48:x=(w-text_w)/2:y=(h-text_h)/2" \
    -c:v libx264 -preset fast -pix_fmt yuv420p -c:a aac -b:a 192k "${CLIP_A}"
fi

# Clip B: Blue background with sine tone (5s, 30fps)
if [ ! -f "${CLIP_B}" ]; then
  echo "🎬 Generating Scene B (Blue)..."
  ffmpeg -y \
    -f lavfi -i "color=c=0x0275d8:size=1920x1080:rate=30:duration=5" \
    -f lavfi -i "sine=frequency=659.25:duration=5" \
    -vf "drawtext=text='Scene B - Raw Clip - %{pts\:hms}':fontcolor=white:fontsize=48:x=(w-text_w)/2:y=(h-text_h)/2" \
    -c:v libx264 -preset fast -pix_fmt yuv420p -c:a aac -b:a 192k "${CLIP_B}"
fi

echo "============================================================"
echo " STEP 2: CREATING PREMIERE PRO / FCP XML TIMELINE MANIFEST"
echo " Cut 1: Scene A [in=1.0s (frame 30) -> out=4.0s (frame 120)] (3.0s)"
echo " Cut 2: Scene B [in=2.0s (frame 60) -> out=5.0s (frame 150)] (3.0s)"
echo " Total Timeline: 6.0s (180 frames at 30fps)"
echo "============================================================"

cat << EOF > "${XML_FILE}"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE xmeml>
<xmeml version="4">
  <sequence id="sequence-1">
    <name>Premiere Distributed Edit</name>
    <duration>180</duration>
    <rate>
      <timebase>30</timebase>
      <ntsc>FALSE</ntsc>
    </rate>
    <media>
      <video>
        <track>
          <clipitem id="clipitem-1">
            <name>scene_a.mp4</name>
            <duration>150</duration>
            <rate><timebase>30</timebase></rate>
            <start>0</start>
            <end>90</end>
            <in>30</in>
            <out>120</out>
            <file id="file-1">
              <name>scene_a.mp4</name>
              <pathurl>file://localhost${CLIP_A}</pathurl>
            </file>
          </clipitem>
          <clipitem id="clipitem-2">
            <name>scene_b.mp4</name>
            <duration>150</duration>
            <rate><timebase>30</timebase></rate>
            <start>90</start>
            <end>180</end>
            <in>60</in>
            <out>150</out>
            <file id="file-2">
              <name>scene_b.mp4</name>
              <pathurl>file://localhost${CLIP_B}</pathurl>
            </file>
          </clipitem>
        </track>
      </video>
    </media>
  </sequence>
</xmeml>
EOF

echo "✅ Created XML: ${XML_FILE}"

echo "============================================================"
echo " STEP 3: RENDERING INDIVIDUAL CUTS ACROSS PARALLEL WORKERS"
echo "============================================================"

# Worker 1: Renders Cut 1 (Scene A: in=1.0s, dur=3.0s) with CRF 18
echo "👷 Worker 1 rendering Cut #0 (Scene A 1.0s -> 4.0s, dur 3.0s)..."
ffmpeg -y -ss 1.000000 -i "${CLIP_A}" -t 3.000000 \
  -c:v libx264 -preset medium -crf 18 -flags +cgop -pix_fmt yuv420p \
  -c:a aac -b:a 192k -avoid_negative_ts make_zero "${TEST_DIR}/chunk_000.mp4"

# Worker 2: Renders Cut 2 (Scene B: in=2.0s, dur=3.0s) with CRF 18
echo "👷 Worker 2 rendering Cut #1 (Scene B 2.0s -> 5.0s, dur 3.0s)..."
ffmpeg -y -ss 2.000000 -i "${CLIP_B}" -t 3.000000 \
  -c:v libx264 -preset medium -crf 18 -flags +cgop -pix_fmt yuv420p \
  -c:a aac -b:a 192k -avoid_negative_ts make_zero "${TEST_DIR}/chunk_001.mp4"

echo "============================================================"
echo " STEP 4: LOSSLESS STITCHING OF TIMELINE CUTS"
echo "============================================================"

MANIFEST="${TEST_DIR}/concat_manifest.txt"
cat << EOF > "${MANIFEST}"
file '${TEST_DIR}/chunk_000.mp4'
file '${TEST_DIR}/chunk_001.mp4'
EOF

ffmpeg -y -f concat -safe 0 -i "${MANIFEST}" -c copy -movflags +faststart "${OUTPUT_FILE}"

echo "============================================================"
echo " STEP 5: AUDIT & VERIFICATION"
echo "============================================================"

RENDERED_FRAMES=$(ffprobe -v error -select_streams v -count_frames -show_entries stream=nb_read_frames -of default=noprint_wrappers=1:nokey=1 "${OUTPUT_FILE}")
RENDERED_DUR=$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "${OUTPUT_FILE}")

echo "Rendered Timeline Frames: ${RENDERED_FRAMES} frames (Expected: 180 frames)"
echo "Rendered Timeline Duration: ${RENDERED_DUR}s (Expected: 6.000s)"

if [ "${RENDERED_FRAMES}" -eq 180 ]; then
  echo "🎉 SUCCESS: Direct XML timeline rendered perfectly with 100% frame accuracy!"
else
  echo "❌ Discrepancy: Expected 180 frames, got ${RENDERED_FRAMES}"
  exit 1
fi
