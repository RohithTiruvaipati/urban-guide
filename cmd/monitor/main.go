package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/distributed-proxy-system/pkg/config"
	"github.com/distributed-proxy-system/pkg/queue"
)

func main() {
	redisAddr := flag.String("redis", "localhost:6379", "Redis server address")
	flag.Parse()

	cfg := config.LoadConfig()
	cfg.RedisAddr = *redisAddr

	client, err := queue.NewStreamClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("❌ Monitor failed to connect to Redis: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	fmt.Println("==================================================================")
	fmt.Println("📊 DISTRIBUTED PROXY MONITOR & TELEMETRY DASHBOARD")
	fmt.Printf("⚡ Stream: %s | Group: %s | Redis: %s\n", cfg.StreamName, cfg.ConsumerGroup, cfg.RedisAddr)
	fmt.Println("==================================================================")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n👋 Monitor exited.")
			return
		case <-ticker.C:
			stats, err := client.GetQueueStats(ctx, cfg.StreamName, cfg.ConsumerGroup)
			if err != nil {
				fmt.Printf("\r⚠️ Error fetching stats: %v", err)
				continue
			}

			unprocessed := stats.StreamLength - stats.CompletedJobs
			if unprocessed < 0 {
				unprocessed = 0
			}

			fmt.Printf("\r📊 Total Jobs: %-4d | Pending/In-Flight: %-4d | Completed: %-4d | Active Workers: %-2d",
				stats.StreamLength, stats.PendingJobs, stats.CompletedJobs, stats.ActiveConsumers)
		}
	}
}
