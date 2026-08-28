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

## Quick Start with Yarn

### 1. Install & Setup Environment
```bash
yarn setup
```

### 2. Start Local Infrastructure (Redpanda, Redis, PostgreSQL, MinIO)
```bash
yarn infra:up
```

### 3. Run Test Suite
```bash
yarn test
```

### 4. Run the Distributed Services
In Terminal 1 (Control Plane API):
```bash
yarn dev:orchestrator
```

In Terminal 2 (Worker Node):
```bash
yarn dev:worker
```

In Terminal 3 (Submit Job):
```bash
yarn job:submit
```
