package video

// Keyframe represents a probed I-frame presentation timestamp.
type Keyframe struct {
	PtsTime float64 `json:"pts_time"`
	PicType string  `json:"pict_type"` // Always "I"
}

// ChunkRange defines the time interval in seconds for an independent render segment.
// Invariants:
// - StartSec must align to an I-frame (keyframe) except at file start (0.0).
// - EndSec must align to the next chunk's StartSec (or video duration) to prevent dropped/duplicated frames.
type ChunkRange struct {
	ChunkIndex int     `json:"chunk_index"`
	StartSec   float64 `json:"start_sec"`
	EndSec     float64 `json:"end_sec"`
	Duration   float64 `json:"duration"`
}

// VideoMetadata contains duration, dimensions, framerate, and stream counts.
type VideoMetadata struct {
	Duration   float64 `json:"duration"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	FrameRate  float64 `json:"frame_rate"`
	TotalFrames int    `json:"total_frames"`
	HasAudio   bool    `json:"has_audio"`
}

// ChunkRenderOpts specifies encoding parameters for rendering an individual video chunk.
type ChunkRenderOpts struct {
	SourcePath      string
	OutputPath      string
	StartSec        float64
	EndSec          float64
	Codec           string // e.g. "libx264"
	Preset          string // e.g. "veryfast", "medium"
	Bitrate         string // e.g. "3M"
	VideoFilter     string // e.g. "hue=s=1.2,eq=contrast=1.1"
	AvoidNegativeTs bool
}

// AccuracyReport summarizes frame count and duration audit between source and output.
type AccuracyReport struct {
	SourceFrames int     `json:"source_frames"`
	OutputFrames int     `json:"output_frames"`
	SourceDuration float64 `json:"source_duration"`
	OutputDuration float64 `json:"output_duration"`
	IsFrameExact bool    `json:"is_frame_exact"`
}
