package kafka

// ChunkJobMsg is the message published by the Orchestrator to the `render.jobs` Kafka topic.
type ChunkJobMsg struct {
	JobID           string  `json:"jobId"`
	ChunkIndex      int     `json:"chunkIndex"`
	TotalChunks     int     `json:"totalChunks"`
	SourcePath      string  `json:"sourcePath"`
	OutputPath      string  `json:"outputPath"`
	StartSec        float64 `json:"startSec"`
	EndSec          float64 `json:"endSec"`
	Duration        float64 `json:"duration,omitempty"`
	Codec           string  `json:"codec"`
	Preset          string  `json:"preset"`
	Bitrate         string  `json:"bitrate"`
	CRF             *int    `json:"crf,omitempty"`
	VideoFilter     string  `json:"videoFilter"`
	AvoidNegativeTs bool    `json:"avoidNegativeTs"`
	IncludeAudio    bool    `json:"includeAudio,omitempty"`
}

// ChunkResultMsg is the message published by the Worker to the `render.results` Kafka topic.
type ChunkResultMsg struct {
	JobID      string  `json:"jobId"`
	ChunkIndex int     `json:"chunkIndex"`
	Status     string  `json:"status"` // "SUCCESS" or "FAILED"
	OutputPath string  `json:"outputPath"`
	DurationMs int64   `json:"durationMs"`
	WorkerID   string  `json:"workerId"`
	Error      *string `json:"error,omitempty"`
}
