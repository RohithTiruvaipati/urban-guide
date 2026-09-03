package models

import "fmt"

// ProxyJob represents a single video file proxy transcoding task.
type ProxyJob struct {
	JobID       string  `json:"job_id"`
	SourcePath  string  `json:"source_path"`
	OutputDir   string  `json:"output_dir"`
	ProxyPath   string  `json:"proxy_path"`
	Codec       string  `json:"codec"`       // "prores" (ProRes 422 Proxy) or "h264" (H.264 MP4 Proxy)
	Resolution  string  `json:"resolution"`  // "1080p", "720p", "source"
	FileSizeMB  float64 `json:"file_size_mb"`
	DurationSec float64 `json:"duration_sec"`
	CreatedAt   int64   `json:"created_at"`
	Attempts    int     `json:"attempts"`
}

// ClipMetadata stores probed media stream information.
type ClipMetadata struct {
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	Duration        float64 `json:"duration"`
	FrameRate       float64 `json:"frame_rate"`
	TotalFrames     int     `json:"total_frames"`
	VideoCodec      string  `json:"video_codec"`
	AudioCodec      string  `json:"audio_codec"`
	AudioChannels   int     `json:"audio_channels"`
	AudioSampleRate int     `json:"audio_sample_rate"`
	HasAudio        bool    `json:"has_audio"`
}

// JobStatus defines the lifecycle states of a proxy task.
type JobStatus string

const (
	StatusPending    JobStatus = "PENDING"
	StatusProcessing JobStatus = "PROCESSING"
	StatusCompleted  JobStatus = "COMPLETED"
	StatusFailed     JobStatus = "FAILED"
	StatusReclaimed  JobStatus = "RECLAIMED"
)

// ProxyReport summarizes the verification between source and proxy.
type ProxyReport struct {
	SourcePath    string  `json:"source_path"`
	ProxyPath     string  `json:"proxy_path"`
	SourceFrames  int     `json:"source_frames"`
	ProxyFrames   int     `json:"proxy_frames"`
	AudioMatch    bool    `json:"audio_match"`
	FrameMatch    bool    `json:"frame_match"`
	DurationDelta float64 `json:"duration_delta"`
	IsAttachable  bool    `json:"is_attachable"`
}

func (r ProxyReport) String() string {
	return fmt.Sprintf("ProxyReport[Attachable=%v, FrameMatch=%v (src:%d, prx:%d), AudioMatch=%v]",
		r.IsAttachable, r.FrameMatch, r.SourceFrames, r.ProxyFrames, r.AudioMatch)
}
