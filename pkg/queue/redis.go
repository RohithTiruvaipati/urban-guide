package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/distributed-proxy-system/pkg/models"
	"github.com/redis/go-redis/v9"
)

// StreamClient manages Redis Streams interactions for job dispatch, auto-claim, and ACKs.
type StreamClient struct {
	rdb *redis.Client
}

// QueueStats holds real-time telemetry of the Redis Stream.
type QueueStats struct {
	StreamLength    int64 `json:"stream_length"`
	PendingJobs     int64 `json:"pending_jobs"`
	ActiveConsumers int64 `json:"active_consumers"`
	CompletedJobs   int64 `json:"completed_jobs"`
}

// ConsumedMessage encapsulates a pulled task and whether it was auto-claimed from a dead worker.
type ConsumedMessage struct {
	MsgID       string
	Job         models.ProxyJob
	IsReclaimed bool
}

// NewStreamClient initializes the Redis client connection.
func NewStreamClient(redisAddr, password string, db int) (*StreamClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed to %s: %w", redisAddr, err)
	}

	return &StreamClient{rdb: rdb}, nil
}

// EnsureConsumerGroup creates the Redis stream and consumer group if it doesn't already exist.
func (c *StreamClient) EnsureConsumerGroup(ctx context.Context, stream, group string) error {
	err := c.rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && !isBusyGroupErr(err) {
		return fmt.Errorf("failed to create consumer group %s on %s: %w", group, stream, err)
	}
	return nil
}

// PublishJob serializes and publishes a new proxy job to the stream.
func (c *StreamClient) PublishJob(ctx context.Context, stream string, job models.ProxyJob) (string, error) {
	jobBytes, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("failed to serialize job: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{
			"job_id":      job.JobID,
			"source_path": job.SourcePath,
			"output_dir":  job.OutputDir,
			"proxy_path":  job.ProxyPath,
			"codec":       job.Codec,
			"resolution":  job.Resolution,
			"payload":     string(jobBytes),
			"created_at":  job.CreatedAt,
		},
	}

	msgID, err := c.rdb.XAdd(ctx, args).Result()
	if err != nil {
		return "", fmt.Errorf("failed to add job to stream %s: %w", stream, err)
	}

	// Register initial status in Redis Hash
	_ = c.rdb.HSet(ctx, fmt.Sprintf("proxy:status:%s", job.JobID), map[string]interface{}{
		"status":      models.StatusPending,
		"source_path": job.SourcePath,
		"proxy_path":  job.ProxyPath,
		"created_at":  job.CreatedAt,
	}).Err()

	return msgID, nil
}

// ConsumeWithAutoClaim pulls tasks: first claiming pending tasks idle > minIdle from dead workers,
// then fetching new tasks from the stream.
func (c *StreamClient) ConsumeWithAutoClaim(
	ctx context.Context,
	stream, group, consumerName string,
	minIdle time.Duration,
) (*ConsumedMessage, error) {
	// 1. Check for stale pending tasks to auto-claim (Fault Recovery)
	autoClaimArgs := &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumerName,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    1,
	}

	claimedMsgs, _, err := c.rdb.XAutoClaim(ctx, autoClaimArgs).Result()
	if err == nil && len(claimedMsgs) > 0 {
		msg := claimedMsgs[0]
		job, parseErr := parseJobFromXMessage(msg)
		if parseErr == nil {
			return &ConsumedMessage{
				MsgID:       msg.ID,
				Job:         job,
				IsReclaimed: true,
			}, nil
		}
	}

	// 2. Read new task from stream (">" = unread by any consumer in group)
	readArgs := &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumerName,
		Streams:  []string{stream, ">"},
		Count:    1,
		Block:    1500 * time.Millisecond,
	}

	streams, err := c.rdb.XReadGroup(ctx, readArgs).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || ctx.Err() != nil {
			return nil, nil // Timeout / No new jobs
		}
		return nil, fmt.Errorf("xreadgroup error: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	msg := streams[0].Messages[0]
	job, err := parseJobFromXMessage(msg)
	if err != nil {
		return nil, err
	}

	return &ConsumedMessage{
		MsgID:       msg.ID,
		Job:         job,
		IsReclaimed: false,
	}, nil
}

// AckJob marks the stream entry as successfully processed.
func (c *StreamClient) AckJob(ctx context.Context, stream, group, msgID string) error {
	return c.rdb.XAck(ctx, stream, group, msgID).Err()
}

// RecordJobStatus records worker completion metrics in Redis.
func (c *StreamClient) RecordJobStatus(
	ctx context.Context,
	jobID string,
	status models.JobStatus,
	workerID string,
	durationMs int64,
	errMsg string,
) error {
	fields := map[string]interface{}{
		"status":      string(status),
		"worker_id":   workerID,
		"duration_ms": durationMs,
		"updated_at":  time.Now().UnixMilli(),
	}
	if errMsg != "" {
		fields["error"] = errMsg
	}
	if status == models.StatusCompleted {
		_ = c.rdb.Incr(ctx, "proxy:stats:completed_count").Err()
	}
	return c.rdb.HSet(ctx, fmt.Sprintf("proxy:status:%s", jobID), fields).Err()
}

// GetQueueStats returns queue depth, pending items, active consumers, and completed counts.
func (c *StreamClient) GetQueueStats(ctx context.Context, stream, group string) (*QueueStats, error) {
	stats := &QueueStats{}

	// Stream length
	lenVal, err := c.rdb.XLen(ctx, stream).Result()
	if err == nil {
		stats.StreamLength = lenVal
	}

	// Pending entries in group
	pending, err := c.rdb.XPending(ctx, stream, group).Result()
	if err == nil {
		stats.PendingJobs = pending.Count
	}

	// Consumers count
	consumers, err := c.rdb.XInfoConsumers(ctx, stream, group).Result()
	if err == nil {
		stats.ActiveConsumers = int64(len(consumers))
	}

	// Completed count
	completedStr, err := c.rdb.Get(ctx, "proxy:stats:completed_count").Result()
	if err == nil {
		if c, parseErr := strconv.ParseInt(completedStr, 10, 64); parseErr == nil {
			stats.CompletedJobs = c
		}
	}

	return stats, nil
}

// Close closes the Redis connection.
func (c *StreamClient) Close() error {
	return c.rdb.Close()
}

func parseJobFromXMessage(msg redis.XMessage) (models.ProxyJob, error) {
	var job models.ProxyJob

	if payload, ok := msg.Values["payload"].(string); ok && payload != "" {
		if err := json.Unmarshal([]byte(payload), &job); err == nil {
			return job, nil
		}
	}

	// Fallback to manual field extraction
	if jid, ok := msg.Values["job_id"].(string); ok {
		job.JobID = jid
	}
	if sp, ok := msg.Values["source_path"].(string); ok {
		job.SourcePath = sp
	}
	if od, ok := msg.Values["output_dir"].(string); ok {
		job.OutputDir = od
	}
	if pp, ok := msg.Values["proxy_path"].(string); ok {
		job.ProxyPath = pp
	}
	if cod, ok := msg.Values["codec"].(string); ok {
		job.Codec = cod
	}
	if res, ok := msg.Values["resolution"].(string); ok {
		job.Resolution = res
	}

	if job.SourcePath == "" {
		return job, fmt.Errorf("invalid xmessage %s: missing source_path", msg.ID)
	}

	return job, nil
}

func isBusyGroupErr(err error) bool {
	return err != nil && (err.Error() == "BUSYGROUP Consumer Group name already exists" ||
		errors.Is(err, redis.Nil))
}
