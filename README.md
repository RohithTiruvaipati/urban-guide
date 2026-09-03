# Distributed Proxy Generation System

A lightweight, fault-tolerant distributed proxy generation engine designed to transcode batches of raw 4K/6K camera footage into **Adobe Premiere Pro-compatible proxy files** across a cluster of worker nodes faster than Adobe Media Encoder's default single-machine sequential queue.

Built with **Go**, **Redis Streams**, and **FFmpeg**.

---

## Architecture Overview

```
[Raw Footage Directory] (Watch folder / S3 / Shared NFS)
          │
          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Go Producer CLI                        │
│   • Validates footage accessibility & probes streams        │
│   • Dispatches 1 job per clip to Redis Stream               │
└──────────────────────────────┬──────────────────────────────┘
                               │
            Redis Stream: `proxy:jobs` (Group: `proxy_workers`)
                               │
            ┌──────────────────┼──────────────────┐
            ▼                  ▼                  ▼
     ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
     │ Go Worker 1 │    │ Go Worker 2 │    │ Go Worker 3 │
     │  (Node #1)  │    │  (Node #2)  │    │  (Node #3)  │
     └──────┬──────┘    └──────┬──────┘    └──────┬──────┘
            │                  │                  │
      FFmpeg Transcode   FFmpeg Transcode   FFmpeg Transcode
      (ProRes / H.264)   (ProRes / H.264)   (ProRes / H.264)
            │                  │                  │
            └──────────────────┼──────────────────┘
                               ▼
┌─────────────────────────────────────────────────────────────┐
│                    Proxy Output Folder                      │
│   `<original_basename>_Proxy.mov` (Apple ProRes 422 Proxy)  │
│   `<original_basename>_Proxy.mp4` (H.264 1080p/720p Proxy)  │
│                                                             │
│   • Audio Track Conformity: 1:1 matching channel layout     │
│   • Framerate & Timecode Lock: Zero drift                   │
│   • Premiere Pro "Attach Proxies": Instant 100% Match       │
└─────────────────────────────────────────────────────────────┘
                               │
                  Redis `XACK` / `XAUTOCLAIM`
       (Dead workers auto-claimed after timeout with 0 lost jobs)
```

---

## Why This Exists

In professional video post-production (commercials, features, multicam shoots), assistants spend hours waiting for Adobe Media Encoder (AME) or DaVinci Resolve to sequentially transcode hundreds of gigabytes of raw camera rushes (Sony FX6/A7SIII, RED R3D, ARRI ProRes, Blackmagic BRAW) on a single editing workstation.

This system distributes transcoding across multiple local or cloud worker instances:
1. **Durable Work Distribution**: Redis Streams consumer groups (`XREADGROUP`) ensure each clip is claimed by exactly one worker with zero duplicated work.
2. **Automatic Crash Recovery**: If a worker node crashes mid-transcode (`kill -9`, spot VM eviction, OOM), surviving workers reclaim the orphaned job via `XAUTOCLAIM` and finish the batch without human intervention.
3. **Premiere Pro Native Compliance**: Generates true ProRes 422 Proxy or H.264 proxies with exact audio track preservation so Premiere Pro's native **"Attach Proxies"** feature attaches every clip automatically.

---

## Tech Stack & Design Tradeoffs

| Component | Technology | Rationale & Tradeoffs |
| :--- | :--- | :--- |
| **Queue / Coordination** | **Redis Streams** (`XREADGROUP`, `XAUTOCLAIM`, `XACK`) | Durable queuing, single-delivery consumer groups, and pending-entries tracking without Kafka cluster overhead. *Tradeoff: lower throughput ceiling than Kafka, but perfect for media transcoding pipelines.* |
| **Worker Engine** | **Go (Golang)** | Compiles to a single zero-dependency static binary you can scp/deploy to any VM in seconds. |
| **Transcode Core** | **FFmpeg** | Industry-standard codec engine with hardware acceleration, ProRes 422 Proxy (`prores_ks`), and multichannel audio copy. |
| **Telemetry** | **CLI Dashboard** | Lightweight terminal telemetry polling Redis for progress, pending tasks, active workers, and throughput. |

> [!NOTE]
> **What was deliberately excluded**: Kafka, Spring Boot, and Kubernetes were omitted to keep the stack lightweight, reproducible on standalone cloud VMs or local machines in under 2 minutes, and hyper-focused on media pipeline fault tolerance.

---

## Benchmark Results

Transcoding a batch of **Raw 4K UHD (3840x2160 @ 30fps)** clips into **Apple ProRes 422 Proxy (1920x1080)**:

| Worker Pool | Wall-Clock Time | Speedup | Scaling Efficiency | Notes |
| :--- | :--- | :--- | :--- | :--- |
| **1 Worker (Baseline)** | **19s** | **1.00x** | **100.0%** | Sequential single-pipeline baseline (similar to AME) |
| **2 Workers** | **14s** | **1.36x** | **67.9%** | 2 parallel worker processes |
| **3 Workers** | **18s** | **1.06x** | **35.2%** | Demonstrates CPU core & disk I/O saturation on single host |

*(Note: On separate cloud VMs with independent compute and network bandwidth, scaling approaches linear speedup before shared storage saturation).*

---

## Live Fault-Tolerance & Auto-Recovery Demo

To prove the system recovers when a worker is killed mid-job without dropping tasks:

```bash
# Run the automated fault injection test
yarn demo:fault
# or: bash scripts/demo_fault_tolerance.sh
```

### What Happens:
1. Producer enqueues 6 raw 4K jobs to `proxy:jobs`.
2. `Worker 1` (`worker-alpha`) and `Worker 2` (`worker-beta`) start processing.
3. **Fault Injection**: `kill -9 <Worker-1-PID>` abruptly terminates Worker 1 while transcoding Clip #1.
4. `Worker 2` detects the orphaned un-ACKed job in Redis via `XAUTOCLAIM` (idle timeout threshold: 4s).
5. `Worker 2` reclaims the abandoned task:
   ```
   2026/09/03 15:23:10 ⚡ [FAULT RECOVERY] Worker [worker-beta] RECLAIMED abandoned job [1788466983576-0] for clip 'A001_C001_09038M_001.mp4'!
   ```
6. The entire batch finishes with **6/6 completed proxies** and **zero lost jobs**.

---

## Premiere Pro "Attach Proxies" Verification

Premiere Pro's proxy engine strictly requires that proxy files match the source footage in:
1. **Audio Channel Layout**: If the source has 4 mono audio channels, the proxy must have 4 mono audio channels (or Premiere rejects attachment).
2. **Framerate & Duration**: Exact frame count match ($\le 1$ frame delta) to prevent audio drift.
3. **File Suffix**: Standard `<name>_Proxy.<ext>` naming.

Run the compliance audit tool:
```bash
yarn verify
# or: bash scripts/verify_premiere_compatibility.sh ./test_footage/raw ./proxies
```

### Sample Output:
```
==================================================================
🔍 PREMIERE PRO PROXY CONFORMITY AUDIT
==================================================================
✅ [PERFECT] A001_C001_09038M_001_Proxy (prores 1920x1080) | Frames: 120/120 | Audio: 1ch | Premiere Ready!
✅ [PERFECT] A001_C002_09038M_001_Proxy (prores 1920x1080) | Frames: 120/120 | Audio: 1ch | Premiere Ready!
✅ [PERFECT] A001_C003_09038M_001_Proxy (prores 1920x1080) | Frames: 120/120 | Audio: 1ch | Premiere Ready!
==================================================================
📊 AUDIT SUMMARY: 6/6 Proxies Passed Strict Premiere Pro Compliance
🎉 100% SUCCESS: All proxies can be cleanly attached via Premiere Pro 'Attach Proxies'!
```

---

## Quick Start Guide

### Prerequisites
* Go 1.22+
* Docker (for Redis)
* FFmpeg & FFprobe installed (`brew install ffmpeg` on macOS or `apt install ffmpeg` on Ubuntu)

### 1. Start Redis Infrastructure
```bash
yarn infra:up
# or: docker compose -f infra/docker-compose.yml up -d redis
```

### 2. Build Binaries
```bash
yarn build
```

### 3. Generate Test Footage (or use your own camera files)
```bash
yarn footage:gen
```

### 4. Run Workers (in separate terminals or machines)
```bash
# Terminal 1 (Worker Node 1)
./bin/proxy-worker --worker-id=node-1

# Terminal 2 (Worker Node 2)
./bin/proxy-worker --worker-id=node-2
```

### 5. Enqueue Footage Batch
```bash
./bin/proxy-producer -input ./test_footage/raw -output ./proxies -codec prores
```

### 6. Monitor in Real Time
```bash
yarn proxy:monitor
```

---

## Constraints & Honest Limitations

* **Source File Transcoding, Not Sequence Rendering**: This system operates on raw camera media files, not multi-track timeline sequences. It does not evaluate Premiere Lumetri color grades, transitions, or After Effects Dynamic Link comps.
* **Shared Storage Bound**: Speedup scales with worker count until shared network storage / disk read bandwidth becomes the bottleneck.
* **Intended Use Case**: Designed for video production teams, assistant editors, and post-houses offloading proxy generation to local network nodes or cloud VMs.
* **Security & Auth**: Intended for trusted private networks; production deployment across public subnets requires TLS on Redis and authentication tokens.

---

## License
MIT License
