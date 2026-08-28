# Distributed Render Farm for Video Transcoding & Exports

## 1. Problem Statement

Exporting or pre-rendering long timelines or transcoding high-resolution video is typically a single-machine, single-pipeline bottleneck. Render time scales linearly with frame count and effect complexity.

**Goal:** Build a distributed system that accepts a video render/transcode job, splits the workload into independent keyframe-aligned chunks, distributes those chunks across a worker pool via Kafka/Redpanda, stores intermediate outputs in shared S3-compatible object storage (MinIO/AWS S3), tracks real-time progress in Redis, and seamlessly reassembles the final output via lossless ffmpeg concatenation.

---

## 2. High-Level Architecture

```
                    ┌─────────────────────────┐
                    │   Spring Boot Control   │
                    │   Plane (Orchestrator)  │
                    └────────────┬────────────┘
                                 │
                 1. Probe Keyframes & Split Chunks
                 2. Publish to render.jobs
                 3. Initialize Redis state
                                 │
                                 ▼
                    ┌─────────────────────────┐
                    │   Kafka / Redpanda      │
                    │   Topic: render.jobs    │
                    └────────────┬────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              ▼                  ▼                  ▼
        ┌───────────┐      ┌───────────┐      ┌───────────┐
        │ Go Worker │      │ Go Worker │ ...  │ Go Worker │
        │  (Node 1) │      │  (Node 2) │      │  (Node N) │
        └─────┬─────┘      └─────┬─────┘      └─────┬─────┘
              │                  │                  │
              │   Render chunk via ffmpeg / AME     │
              │   Upload to MinIO / S3              │
              │                  │                  │
              ▼                  ▼                  ▼
        ┌─────────────────────────────────────────────────┐
        │         Shared Storage (MinIO / S3)             │
        │         jobs/{jobId}/chunk-{chunkId}.mp4        │
        └────────────────────────┬────────────────────────┘
                                 │
                 Report Completion: render.results
                                 │
                                 ▼
                    ┌─────────────────────────┐
                    │   Redis State Store     │
                    │   & Kafka Result Listener│
                    └────────────┬────────────┘
                                 │
                 All chunks complete → Trigger Stitch
                                 ▼
                    ┌─────────────────────────┐
                    │    Stitcher Service     │
                    │  (ffmpeg concat + copy  │
                    │   + full audio remux)   │
                    └────────────┬────────────┘
                                 ▼
                     Final Assembled Video File
```

---

## 3. Core Components

### 3.1 Control Plane (Spring Boot Orchestrator)
- **REST Endpoints**:
  - `POST /api/v1/jobs` — Submit render job (source video URL/path, target preset/codec, resolution, chunk size target).
  - `GET /api/v1/jobs/{id}` — Poll job status & per-chunk progress directly from Redis.
  - `GET /api/v1/jobs/{id}/result` — Retrieve signed URL or file path for final output.
- **Keyframe Probing & Chunking**: Uses `ffprobe` to determine I-frame packet presentation timestamps (`pkt_pts_time`), snapping boundaries to keyframes so every chunk starts on a self-contained GOP.
- **Kafka Job Publisher**: Publishes chunk payloads to `render.jobs` partitioned for consumer group distribution.
- **Result Listener & Stitcher**: Consumes `render.results`, increments Redis progress counters, and initiates lossless concat stitching when all chunks are marked `DONE`.

### 3.2 Worker Pool (Go Workers)
- **Kafka Consumer Group**: Pulls chunk tasks evenly across instances.
- **ffmpeg Execution**: Executes targeted encode passes per chunk segment with accurate PTS handling (`-ss`, `-to`, `-avoid_negative_ts make_zero`).
- **S3 / MinIO Integration**: Streams completed chunk files to shared object storage.
- **Idempotency**: Verifies existing segment integrity in object storage before re-encoding to safely handle Kafka retries.

### 3.3 State Management (Redis + PostgreSQL)
- **Redis (Fast / Ephemeral)**:
  - `job:{jobId}` (Hash) — Overall status (`PENDING`, `RUNNING`, `STITCHING`, `COMPLETED`, `FAILED`), chunk count, completed chunks, timestamp.
  - `job:{jobId}:chunks` (Hash) — Per-chunk state (`PENDING`, `PROCESSING`, `DONE`, `FAILED`), worker ID, duration.
- **PostgreSQL (Durable)**:
  - Job audit history, benchmark metrics, user metadata, and final asset catalog.

---

## 4. Keyframe Alignment (GOP Theory & Verification)

Modern inter-frame video compression (H.264/H.265/AV1) encodes video as Groups of Pictures (GOP):
- **I-frames (Intra)**: Full standalone images / keyframes.
- **P-frames (Predicted)**: Delta changes based on prior frames.
- **B-frames (Bi-directional)**: Delta changes based on both prior and future frames.

Splitting arbitrarily mid-GOP causes decoder failure, corrupt start frames, or dropped frames. By probing keyframes with:
```bash
ffprobe -select_streams v -show_entries frame=pict_type,pkt_pts_time -of csv <source>
```
boundaries are snapped to exact I-frame timestamps, allowing lossless stream concatenation:
```bash
ffmpeg -f concat -safe 0 -i chunks.txt -c copy output.mp4
```

Audio is extracted once from the source to prevent sample drift and remuxed at the final stitch pass:
```bash
ffmpeg -i stitched_video.mp4 -i source_audio.aac -c:v copy -c:a copy final_output.mp4
```
