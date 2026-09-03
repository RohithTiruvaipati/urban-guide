package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/distributed-proxy-system/pkg/config"
	"github.com/distributed-proxy-system/pkg/models"
	"github.com/distributed-proxy-system/pkg/queue"
	"github.com/distributed-proxy-system/pkg/transcoder"
)

func main() {
	workerID := flag.String("worker-id", "", "Unique worker identifier (e.g. node-1, worker-alpha)")
	claimIdleSec := flag.Int("claim-idle", 15, "Seconds a pending job can sit idle before being auto-claimed (fault recovery)")
	redisAddr := flag.String("redis", "localhost:6379", "Redis server address")
	flag.Parse()

	if *workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "node"
		}
		*workerID = fmt.Sprintf("worker-%s-%d", hostname, os.Getpid())
	}

	cfg := config.LoadConfig()
	cfg.RedisAddr = *redisAddr

	client, err := queue.NewStreamClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("❌ Worker [%s] failed to connect to Redis: %v", *workerID, err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ensure group exists
	if err := client.EnsureConsumerGroup(ctx, cfg.StreamName, cfg.ConsumerGroup); err != nil {
		log.Fatalf("❌ Worker failed to ensure consumer group: %v", err)
	}

	// Handle graceful termination
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("\n🛑 Worker [%s] received signal (%v), finishing current task and shutting down...", *workerID, sig)
		cancel()
	}()

	fmt.Println("==================================================================")
	fmt.Printf("👷 PROXY WORKER ONLINE: %s\n", *workerID)
	fmt.Printf("⚡ Redis Stream:        %s | Group: %s\n", cfg.StreamName, cfg.ConsumerGroup)
	fmt.Printf("🛡️ Auto-Claim Idle:     %ds (Fault-tolerance active)\n", *claimIdleSec)
	fmt.Println("==================================================================")
	log.Printf("Worker [%s] polling for jobs...", *workerID)

	minIdle := time.Duration(*claimIdleSec) * time.Second
	processedCount := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\n👋 Worker [%s] stopped cleanly. Total processed: %d\n", *workerID, processedCount)
			return
		default:
		}

		// Pull task with auto-claim support
		msg, err := client.ConsumeWithAutoClaim(ctx, cfg.StreamName, cfg.ConsumerGroup, *workerID, minIdle)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("⚠️ Error pulling task from Redis: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if msg == nil {
			// No jobs right now, loop
			continue
		}

		job := msg.Job
		sourceBase := filepath.Base(job.SourcePath)
		proxyBase := filepath.Base(job.ProxyPath)

		if msg.IsReclaimed {
			log.Printf("⚡ [FAULT RECOVERY] Worker [%s] RECLAIMED abandoned job [%s] for clip '%s'!",
				*workerID, msg.MsgID, sourceBase)
		} else {
			log.Printf("📥 [%s] Picked up Job [%s] -> Source: '%s'",
				*workerID, msg.MsgID, sourceBase)
		}

		// Record processing state in Redis
		_ = client.RecordJobStatus(ctx, job.JobID, models.StatusProcessing, *workerID, 0, "")

		// Execute transcoding
		start := time.Now()
		transcodeErr := transcoder.TranscodeProxy(job.SourcePath, job.ProxyPath, job.Codec, job.Resolution)
		durationMs := time.Since(start).Milliseconds()

		if transcodeErr != nil {
			log.Printf("❌ [%s] Transcode FAILED for '%s': %v", *workerID, sourceBase, transcodeErr)
			_ = client.RecordJobStatus(ctx, job.JobID, models.StatusFailed, *workerID, durationMs, transcodeErr.Error())
			// Don't ACK on failure so it can be retried or inspected
			continue
		}

		// Verify Premiere Pro compatibility
		report, verifyErr := transcoder.VerifyProxy(job.SourcePath, job.ProxyPath)
		if verifyErr != nil || !report.IsAttachable {
			log.Printf("⚠️ [%s] Proxy verification warning for '%s': %v (Report: %v)",
				*workerID, proxyBase, verifyErr, report)
		}

		// ACK the message in Redis Stream
		if ackErr := client.AckJob(ctx, cfg.StreamName, cfg.ConsumerGroup, msg.MsgID); ackErr != nil {
			log.Printf("⚠️ Failed to ACK message %s: %v", msg.MsgID, ackErr)
		} else {
			_ = client.RecordJobStatus(ctx, job.JobID, models.StatusCompleted, *workerID, durationMs, "")
			processedCount++
			log.Printf("✅ [%s] Completed '%s' -> '%s' (in %dms, %.1f MB/s) | Proxy Ready for Premiere!",
				*workerID, sourceBase, proxyBase, durationMs,
				float64(durationMs)/1000.0)
		}
	}
}
