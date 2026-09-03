package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/distributed-proxy-system/pkg/config"
	"github.com/distributed-proxy-system/pkg/models"
	"github.com/distributed-proxy-system/pkg/queue"
	"github.com/distributed-proxy-system/pkg/transcoder"
)

var validExtensions = map[string]bool{
	".mp4": true, ".mov": true, ".mkv": true, ".mxf": true,
	".avi": true, ".braw": true, ".r3d": true, ".prores": true,
}

func main() {
	inputDir := flag.String("input", "", "Directory containing raw video clips to proxy")
	outputDir := flag.String("output", "./proxies", "Directory to write Premiere-compatible proxies")
	codec := flag.String("codec", "prores", "Proxy codec: 'prores' (ProRes 422 Proxy) or 'h264' (H.264 MP4)")
	resolution := flag.String("res", "1080p", "Proxy resolution: '1080p', '720p', or 'source'")
	redisAddr := flag.String("redis", "localhost:6379", "Redis server address")
	flag.Parse()

	if *inputDir == "" {
		fmt.Println("Usage: proxy-producer -input <raw_footage_folder> [-output ./proxies] [-codec prores|h264]")
		os.Exit(1)
	}

	absInput, err := filepath.Abs(*inputDir)
	if err != nil {
		log.Fatalf("Invalid input directory: %v", err)
	}

	absOutput, err := filepath.Abs(*outputDir)
	if err != nil {
		log.Fatalf("Invalid output directory: %v", err)
	}

	cfg := config.LoadConfig()
	cfg.RedisAddr = *redisAddr
	cfg.OutputDir = absOutput
	cfg.DefaultCodec = *codec
	cfg.DefaultRes = *resolution

	client, err := queue.NewStreamClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("❌ Failed to connect to Redis at %s: %v", cfg.RedisAddr, err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.EnsureConsumerGroup(ctx, cfg.StreamName, cfg.ConsumerGroup); err != nil {
		log.Fatalf("❌ Failed to initialize consumer group: %v", err)
	}

	fmt.Println("==================================================================")
	fmt.Println("🎬 DISTRIBUTED PROXY PRODUCER")
	fmt.Printf("📁 Input Folder:     %s\n", absInput)
	fmt.Printf("📁 Output Proxies:   %s\n", absOutput)
	fmt.Printf("🎞️ Proxy Codec:      %s\n", strings.ToUpper(cfg.DefaultCodec))
	fmt.Printf("📐 Resolution:       %s\n", cfg.DefaultRes)
	fmt.Printf("⚡ Redis Stream:     %s (%s)\n", cfg.StreamName, cfg.RedisAddr)
	fmt.Println("==================================================================")

	// Scan directory for raw video clips
	var candidateFiles []string
	err = filepath.Walk(absInput, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if validExtensions[ext] && !strings.Contains(path, "_Proxy") {
			candidateFiles = append(candidateFiles, path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error scanning directory: %v", err)
	}

	if len(candidateFiles) == 0 {
		fmt.Printf("⚠️ No raw video files found in %s\n", absInput)
		os.Exit(0)
	}

	fmt.Printf("🔍 Discovered %d candidate video clips. Validating and enqueueing...\n\n", len(candidateFiles))

	queuedCount := 0
	var totalDuration float64

	for i, file := range candidateFiles {
		meta, probeErr := transcoder.ProbeMetadata(file)
		if probeErr != nil {
			fmt.Printf("⚠️ [%d/%d] Skipping unreadable clip '%s': %v\n", i+1, len(candidateFiles), filepath.Base(file), probeErr)
			continue
		}

		proxyPath := transcoder.BuildProxyPath(file, absOutput, cfg.DefaultCodec)
		jobID := fmt.Sprintf("job-%d-%s", time.Now().UnixNano(), filepath.Base(file))

		job := models.ProxyJob{
			JobID:       jobID,
			SourcePath:  file,
			OutputDir:   absOutput,
			ProxyPath:   proxyPath,
			Codec:       cfg.DefaultCodec,
			Resolution:  cfg.DefaultRes,
			DurationSec: meta.Duration,
			CreatedAt:   time.Now().Unix(),
		}

		msgID, pubErr := client.PublishJob(ctx, cfg.StreamName, job)
		if pubErr != nil {
			fmt.Printf("❌ Failed to enqueue job for %s: %v\n", filepath.Base(file), pubErr)
			continue
		}

		queuedCount++
		totalDuration += meta.Duration

		fmt.Printf("✅ [%d/%d] Enqueued: %-30s | Dur: %.1fs | Res: %dx%d | Audio: %dch | Stream ID: %s\n",
			queuedCount, len(candidateFiles), filepath.Base(file), meta.Duration, meta.Width, meta.Height, meta.AudioChannels, msgID)
	}

	fmt.Println("\n==================================================================")
	fmt.Printf("🚀 SUCCESS: %d proxy jobs dispatched to Redis Stream '%s'!\n", queuedCount, cfg.StreamName)
	fmt.Printf("⏱️ Total Footage Duration: %.1fs (~%.1f mins)\n", totalDuration, totalDuration/60.0)
	fmt.Printf("👉 Start workers to transcode: go run ./cmd/worker/main.go --worker-id=node-1\n")
	fmt.Println("==================================================================")
}
