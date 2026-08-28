package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/renderfarm/worker/pkg/video"
	"github.com/segmentio/kafka-go"
)

// ConsumerConfig holds connection and tuning parameters for the worker consumer.
type ConsumerConfig struct {
	Brokers      []string
	TopicJobs    string
	TopicResults string
	GroupID      string
	WorkerID     string
}

// JobConsumer manages polling render tasks, invoking ffmpeg, and reporting results.
type JobConsumer struct {
	reader   *kafka.Reader
	producer *ResultProducer
	config   ConsumerConfig
}

// NewJobConsumer instantiates a Kafka consumer group reader.
func NewJobConsumer(cfg ConsumerConfig) *JobConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.TopicJobs,
		GroupID:        cfg.GroupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
		StartOffset:    kafka.FirstOffset,
	})

	producer := NewResultProducer(cfg.Brokers, cfg.TopicResults)

	return &JobConsumer{
		reader:   reader,
		producer: producer,
		config:   cfg,
	}
}

// Start begins the consumer loop, processing messages until ctx is canceled.
func (c *JobConsumer) Start(ctx context.Context) error {
	log.Printf("👷 Worker [%s] listening for render jobs on topic '%s' (group '%s')...",
		c.config.WorkerID, c.config.TopicJobs, c.config.GroupID)

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Worker context canceled, shutting down consumer...")
			return nil
		default:
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("⚠️ Error fetching message from kafka: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var job ChunkJobMsg
		if err := json.Unmarshal(msg.Value, &job); err != nil {
			log.Printf("❌ Failed to parse chunk job json: %v", err)
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		log.Printf("📥 [%s] Received Chunk #%d/%d for Job [%s] (range: %.2fs -> %.2fs)",
			c.config.WorkerID, job.ChunkIndex, job.TotalChunks, job.JobID, job.StartSec, job.EndSec)

		// Process chunk render
		result := c.processChunk(job)

		// Publish result to results topic
		if err := c.producer.PublishResult(ctx, result); err != nil {
			log.Printf("❌ Failed to publish result for Job [%s] Chunk #%d: %v", job.JobID, job.ChunkIndex, err)
		} else {
			log.Printf("📤 [%s] Published result for Job [%s] Chunk #%d (status: %s, duration: %dms)",
				c.config.WorkerID, job.JobID, job.ChunkIndex, result.Status, result.DurationMs)
		}

		// Commit offset
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("⚠️ Failed to commit message offset: %v", err)
		}
	}
}

// processChunk renders the video segment or reuses existing output if already rendered (idempotent).
func (c *JobConsumer) processChunk(job ChunkJobMsg) ChunkResultMsg {
	start := time.Now()

	// Ensure output directory exists
	outDir := filepath.Dir(job.OutputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		errMsg := fmt.Sprintf("failed to create chunk output directory: %v", err)
		return ChunkResultMsg{
			JobID:      job.JobID,
			ChunkIndex: job.ChunkIndex,
			Status:     "FAILED",
			OutputPath: job.OutputPath,
			DurationMs: time.Since(start).Milliseconds(),
			WorkerID:   c.config.WorkerID,
			Error:      &errMsg,
		}
	}

	// Idempotency check: if file already exists with non-zero size, verify with ffprobe
	if fi, err := os.Stat(job.OutputPath); err == nil && fi.Size() > 0 {
		meta, probeErr := video.ProbeMetadata(job.OutputPath)
		if probeErr == nil && meta.Duration > 0 {
			log.Printf("⚡ Idempotency hit: Chunk #%d already rendered and valid at %s", job.ChunkIndex, job.OutputPath)
			return ChunkResultMsg{
				JobID:      job.JobID,
				ChunkIndex: job.ChunkIndex,
				Status:     "SUCCESS",
				OutputPath: job.OutputPath,
				DurationMs: time.Since(start).Milliseconds(),
				WorkerID:   c.config.WorkerID,
			}
		}
	}

	renderOpts := video.ChunkRenderOpts{
		SourcePath:      job.SourcePath,
		OutputPath:      job.OutputPath,
		StartSec:        job.StartSec,
		EndSec:          job.EndSec,
		Codec:           job.Codec,
		Preset:          job.Preset,
		Bitrate:         job.Bitrate,
		VideoFilter:     job.VideoFilter,
		AvoidNegativeTs: job.AvoidNegativeTs,
	}

	if err := video.RenderChunk(renderOpts); err != nil {
		errMsg := err.Error()
		log.Printf("❌ Render failed for Job [%s] Chunk #%d: %v", job.JobID, job.ChunkIndex, err)
		return ChunkResultMsg{
			JobID:      job.JobID,
			ChunkIndex: job.ChunkIndex,
			Status:     "FAILED",
			OutputPath: job.OutputPath,
			DurationMs: time.Since(start).Milliseconds(),
			WorkerID:   c.config.WorkerID,
			Error:      &errMsg,
		}
	}

	durationMs := time.Since(start).Milliseconds()
	return ChunkResultMsg{
		JobID:      job.JobID,
		ChunkIndex: job.ChunkIndex,
		Status:     "SUCCESS",
		OutputPath: job.OutputPath,
		DurationMs: durationMs,
		WorkerID:   c.config.WorkerID,
	}
}

// Close closes reader and producer.
func (c *JobConsumer) Close() error {
	_ = c.reader.Close()
	return c.producer.Close()
}
