package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/renderfarm/worker/pkg/kafka"
)

func main() {
	brokerList := flag.String("brokers", getEnv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092"), "Comma-separated Kafka brokers")
	topicJobs := flag.String("jobs-topic", getEnv("KAFKA_TOPIC_JOBS", "render.jobs"), "Kafka topic for incoming chunk jobs")
	topicResults := flag.String("results-topic", getEnv("KAFKA_TOPIC_RESULTS", "render.results"), "Kafka topic for completed results")
	groupID := flag.String("group-id", getEnv("KAFKA_CONSUMER_GROUP", "render-farm-workers"), "Kafka consumer group ID")
	workerID := flag.String("worker-id", getEnv("WORKER_ID", ""), "Unique worker node ID (auto-generated if empty)")
	flag.Parse()

	if *workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "node"
		}
		*workerID = fmt.Sprintf("worker-%s-%d", hostname, os.Getpid())
	}

	brokers := strings.Split(*brokerList, ",")

	cfg := kafka.ConsumerConfig{
		Brokers:      brokers,
		TopicJobs:    *topicJobs,
		TopicResults: *topicResults,
		GroupID:      *groupID,
		WorkerID:     *workerID,
	}

	consumer := kafka.NewJobConsumer(cfg)
	defer consumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received termination signal [%v], initiating graceful shutdown...", sig)
		cancel()
	}()

	fmt.Println("==================================================================")
	fmt.Printf("🚀 DISTRIBUTED RENDER WORKER: %s\n", *workerID)
	fmt.Printf("   Brokers:       %s\n", *brokerList)
	fmt.Printf("   Jobs Topic:    %s\n", *topicJobs)
	fmt.Printf("   Results Topic: %s\n", *topicResults)
	fmt.Printf("   Consumer Group:%s\n", *groupID)
	fmt.Println("==================================================================")

	if err := consumer.Start(ctx); err != nil {
		log.Fatalf("Worker consumer exited with error: %v", err)
	}

	log.Println("Worker terminated cleanly.")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
