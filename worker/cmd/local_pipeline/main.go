package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/renderfarm/worker/pkg/video"
)

func main() {
	sourcePath := flag.String("source", "test_assets/sample_source.mp4", "Path to source video file")
	chunkSec := flag.Float64("chunk-sec", 5.0, "Target chunk duration in seconds")
	concurrency := flag.Int("concurrency", 1, "Number of concurrent render workers (1 = Phase 1 sequential, >1 = Phase 2 parallel)")
	outputDir := flag.String("output-dir", "test_assets/output", "Directory to store rendered chunks and final stitched file")
	codec := flag.String("codec", "libx264", "Video codec")
	preset := flag.String("preset", "veryfast", "x264 preset")
	bitrate := flag.String("bitrate", "3M", "Target video bitrate")
	flag.Parse()

	absSource, err := filepath.Abs(*sourcePath)
	if err != nil {
		log.Fatalf("Invalid source path: %v", err)
	}

	if _, err := os.Stat(absSource); os.IsNotExist(err) {
		log.Fatalf("Source video not found: %s", absSource)
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	fmt.Println("==================================================================")
	fmt.Printf("🚀 DISTRIBUTED RENDER FARM - LOCAL ENGINE (Concurrency: %d)\n", *concurrency)
	fmt.Println("==================================================================")
	fmt.Printf("Source File:     %s\n", absSource)
	fmt.Printf("Target Chunk:    %.1fs\n", *chunkSec)
	fmt.Printf("Codec / Preset:  %s (%s @ %s)\n", *codec, *preset, *bitrate)
	fmt.Println("------------------------------------------------------------------")

	wallClockStart := time.Now()

	// Step 1: Probe metadata
	fmt.Print("🔍 Probing video metadata... ")
	meta, err := video.ProbeMetadata(absSource)
	if err != nil {
		log.Fatalf("Failed to probe metadata: %v", err)
	}
	fmt.Printf("OK (%.2fs, %dx%d @ %.2ffps, %d frames)\n",
		meta.Duration, meta.Width, meta.Height, meta.FrameRate, meta.TotalFrames)

	// Step 2: Probe keyframes
	fmt.Print("🔑 Probing GOP keyframes (I-frames)... ")
	keyframes, err := video.ProbeKeyframes(absSource)
	if err != nil {
		log.Fatalf("Failed to probe keyframes: %v", err)
	}
	fmt.Printf("Found %d I-frames\n", len(keyframes))

	// Step 3: Calculate GOP-aligned chunk boundaries
	chunks := video.CalculateGOPChunks(meta.Duration, keyframes, *chunkSec)
	fmt.Printf("📦 Calculated %d keyframe-aligned chunks:\n", len(chunks))
	for _, c := range chunks {
		fmt.Printf("   Chunk #%d: [%.3fs -> %.3fs] (duration: %.3fs)\n",
			c.ChunkIndex, c.StartSec, c.EndSec, c.Duration)
	}

	// Step 4: Extract audio once
	audioPath := filepath.Join(*outputDir, "audio_track.aac")
	if meta.HasAudio {
		fmt.Print("🎵 Extracting master audio track (lossless copy)... ")
		if err := video.ExtractAudio(absSource, audioPath); err != nil {
			log.Fatalf("Failed to extract audio: %v", err)
		}
		fmt.Println("OK")
	} else {
		audioPath = ""
	}

	// Step 5: Render chunks (Sequential or Concurrent Goroutines)
	renderStart := time.Now()
	chunkFiles := make([]string, len(chunks))

	if *concurrency <= 1 {
		fmt.Println("\n🎬 Rendering chunks sequentially (Phase 1)...")
		for _, chunk := range chunks {
			chunkOut := filepath.Join(*outputDir, fmt.Sprintf("chunk_%03d.mp4", chunk.ChunkIndex))
			chunkFiles[chunk.ChunkIndex] = chunkOut
			cStart := time.Now()
			fmt.Printf("   Rendering chunk #%d [%.2fs - %.2fs]... ", chunk.ChunkIndex, chunk.StartSec, chunk.EndSec)

			opts := video.ChunkRenderOpts{
				SourcePath:      absSource,
				OutputPath:      chunkOut,
				StartSec:        chunk.StartSec,
				EndSec:          chunk.EndSec,
				Codec:           *codec,
				Preset:          *preset,
				Bitrate:         *bitrate,
				VideoFilter:     "hue=s=1.1,eq=contrast=1.05", // simulate color grade
				AvoidNegativeTs: true,
			}
			if err := video.RenderChunk(opts); err != nil {
				log.Fatalf("Failed chunk #%d: %v", chunk.ChunkIndex, err)
			}
			fmt.Printf("Done in %v\n", time.Since(cStart).Round(time.Millisecond))
		}
	} else {
		fmt.Printf("\n⚡ Rendering chunks concurrently with %d worker goroutines (Phase 2)...\n", *concurrency)
		sem := make(chan struct{}, *concurrency)
		var wg sync.WaitGroup
		var renderErr error
		var errMu sync.Mutex

		for _, chunk := range chunks {
			wg.Add(1)
			sem <- struct{}{}

			chunkOut := filepath.Join(*outputDir, fmt.Sprintf("chunk_%03d.mp4", chunk.ChunkIndex))
			chunkFiles[chunk.ChunkIndex] = chunkOut

			go func(c video.ChunkRange, outPath string) {
				defer wg.Done()
				defer func() { <-sem }()

				cStart := time.Now()
				opts := video.ChunkRenderOpts{
					SourcePath:      absSource,
					OutputPath:      outPath,
					StartSec:        c.StartSec,
					EndSec:          c.EndSec,
					Codec:           *codec,
					Preset:          *preset,
					Bitrate:         *bitrate,
					VideoFilter:     "hue=s=1.1,eq=contrast=1.05",
					AvoidNegativeTs: true,
				}
				if err := video.RenderChunk(opts); err != nil {
					errMu.Lock()
					renderErr = err
					errMu.Unlock()
					return
				}
				fmt.Printf("   ✓ Chunk #%d [%.2fs - %.2fs] finished in %v\n",
					c.ChunkIndex, c.StartSec, c.EndSec, time.Since(cStart).Round(time.Millisecond))
			}(chunk, chunkOut)
		}
		wg.Wait()
		if renderErr != nil {
			log.Fatalf("Concurrent render encountered error: %v", renderErr)
		}
	}

	renderDuration := time.Since(renderStart)
	fmt.Printf("🏁 All chunks rendered in %v\n", renderDuration.Round(time.Millisecond))

	// Step 6: Lossless concatenation + Audio remux
	finalOutput := filepath.Join(*outputDir, "final_rendered_output.mp4")
	fmt.Print("\n🧩 Lossless stream concatenation & audio remux... ")
	stitchStart := time.Now()
	if err := video.StitchChunks(chunkFiles, audioPath, finalOutput); err != nil {
		log.Fatalf("Stitch failed: %v", err)
	}
	stitchDuration := time.Since(stitchStart)
	fmt.Printf("Done in %v (instantaneous stream copy)\n", stitchDuration.Round(time.Millisecond))

	// Step 7: Frame accuracy audit
	fmt.Print("🛡️ Auditing frame count and duration integrity... ")
	report, err := video.AuditAccuracy(absSource, finalOutput)
	if err != nil {
		log.Fatalf("Audit failed: %v", err)
	}

	totalWallClock := time.Since(wallClockStart)

	fmt.Println("\n==================================================================")
	fmt.Println("📊 PIPELINE EXECUTION SUMMARY")
	fmt.Println("==================================================================")
	fmt.Printf("Source Frames:    %d\n", report.SourceFrames)
	fmt.Printf("Output Frames:    %d\n", report.OutputFrames)
	fmt.Printf("Frame Exact:      %t\n", report.IsFrameExact)
	fmt.Printf("Source Duration:  %.3fs\n", report.SourceDuration)
	fmt.Printf("Output Duration:  %.3fs\n", report.OutputDuration)
	fmt.Printf("Render Time:      %v\n", renderDuration.Round(time.Millisecond))
	fmt.Printf("Stitch Time:      %v\n", stitchDuration.Round(time.Millisecond))
	fmt.Printf("Total Wall Clock: %v\n", totalWallClock.Round(time.Millisecond))
	fmt.Printf("Final Output:     %s\n", finalOutput)
	fmt.Println("==================================================================")

	if !report.IsFrameExact {
		log.Fatalf("❌ ERROR: Frame count mismatch between source (%d) and output (%d)!",
			report.SourceFrames, report.OutputFrames)
	}
	fmt.Println("✅ SUCCESS: Render pipeline verified with 100% frame accuracy.")
}
