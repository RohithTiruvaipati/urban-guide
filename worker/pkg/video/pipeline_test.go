package video

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateGOPChunks_Math(t *testing.T) {
	keyframes := []Keyframe{
		{PtsTime: 0.0, PicType: "I"},
		{PtsTime: 2.0, PicType: "I"},
		{PtsTime: 4.0, PicType: "I"},
		{PtsTime: 6.0, PicType: "I"},
		{PtsTime: 8.0, PicType: "I"},
		{PtsTime: 10.0, PicType: "I"},
	}

	totalDuration := 10.0
	targetChunkSec := 3.0

	chunks := CalculateGOPChunks(totalDuration, keyframes, targetChunkSec)

	if len(chunks) == 0 {
		t.Fatalf("expected chunks to be non-empty")
	}

	// Invariant 1: starts at 0.0
	if chunks[0].StartSec != 0.0 {
		t.Errorf("expected chunk 0 start to be 0.0, got %f", chunks[0].StartSec)
	}

	// Invariant 2: continuous intervals (no gaps, no overlaps)
	for i := 0; i < len(chunks)-1; i++ {
		if chunks[i].EndSec != chunks[i+1].StartSec {
			t.Errorf("chunk boundary discontinuity between %d and %d: end=%f, next start=%f",
				i, i+1, chunks[i].EndSec, chunks[i+1].StartSec)
		}
	}

	// Invariant 3: finishes at total duration
	lastChunk := chunks[len(chunks)-1]
	if lastChunk.EndSec != totalDuration {
		t.Errorf("expected last chunk end to be %f, got %f", totalDuration, lastChunk.EndSec)
	}
}

func TestEndToEndLocalPipeline(t *testing.T) {
	// Find source test clip
	sourcePath := "../../../test_assets/sample_source.mp4"
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		t.Skip("sample_source.mp4 not found, skipping integration test")
	}

	tmpDir, err := os.MkdirTemp("", "render_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	meta, err := ProbeMetadata(sourcePath)
	if err != nil {
		t.Fatalf("ProbeMetadata failed: %v", err)
	}
	if meta.Duration <= 0 {
		t.Fatalf("invalid duration probed: %f", meta.Duration)
	}

	keyframes, err := ProbeKeyframes(sourcePath)
	if err != nil {
		t.Fatalf("ProbeKeyframes failed: %v", err)
	}
	if len(keyframes) == 0 {
		t.Fatalf("no keyframes found")
	}

	chunks := CalculateGOPChunks(meta.Duration, keyframes, 10.0)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Extract audio
	audioPath := filepath.Join(tmpDir, "audio.aac")
	if err := ExtractAudio(sourcePath, audioPath); err != nil {
		t.Fatalf("ExtractAudio failed: %v", err)
	}

	// Render chunks sequentially (Phase 1)
	var chunkFiles []string
	for _, chunk := range chunks {
		chunkOut := filepath.Join(tmpDir, filepath.Base(sourcePath)+fmtChunkName(chunk.ChunkIndex))
		opts := ChunkRenderOpts{
			SourcePath:      sourcePath,
			OutputPath:      chunkOut,
			StartSec:        chunk.StartSec,
			EndSec:          chunk.EndSec,
			Codec:           "libx264",
			Preset:          "ultrafast",
			Bitrate:         "2M",
			AvoidNegativeTs: true,
		}
		if err := RenderChunk(opts); err != nil {
			t.Fatalf("RenderChunk %d failed: %v", chunk.ChunkIndex, err)
		}
		chunkFiles = append(chunkFiles, chunkOut)
	}

	// Stitch
	finalOutput := filepath.Join(tmpDir, "final_stitched.mp4")
	if err := StitchChunks(chunkFiles, audioPath, finalOutput); err != nil {
		t.Fatalf("StitchChunks failed: %v", err)
	}

	// Audit accuracy
	report, err := AuditAccuracy(sourcePath, finalOutput)
	if err != nil {
		t.Fatalf("AuditAccuracy failed: %v", err)
	}

	t.Logf("Audit Report: Source=%d frames, Output=%d frames, Exact=%v",
		report.SourceFrames, report.OutputFrames, report.IsFrameExact)

	if !report.IsFrameExact {
		t.Errorf("frame mismatch: expected %d, got %d", report.SourceFrames, report.OutputFrames)
	}
}

func fmtChunkName(index int) string {
	return filepath.Join("", "chunk_"+string(rune('0'+index))+".mp4")
}
