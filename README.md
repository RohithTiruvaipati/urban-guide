# Distributed Render Farm

A distributed video transcoding and render orchestration farm designed to cut wall-clock render time by splitting video frame ranges across a pool of worker nodes and reassembling them with zero-re-encode stream concatenation.

## Project Structure

```
├── docs/                   # Architecture, GOP theory, benchmarks, and API specs
├── infra/                  # Docker Compose, Redpanda, Redis, PostgreSQL, MinIO configs
├── orchestrator/           # Spring Boot control plane (REST API, keyframe probe, chunk scheduler)
├── worker/                 # Go distributed render worker (Kafka consumer, ffmpeg runner, S3 client)
├── scripts/                # Verification, test video generators, and benchmark harnesses
└── test_assets/            # Fixtures, sample clips, and probe diagnostics
```

## Quick Start

### 1. Infrastructure
Spin up local Redpanda (Kafka), Redis, PostgreSQL, and MinIO:
```bash
docker compose -f infra/docker-compose.yml up -d
```

### 2. Run Verification Harness
```bash
./scripts/generate_test_video.sh
./scripts/verify_manual_pipeline.sh
```
