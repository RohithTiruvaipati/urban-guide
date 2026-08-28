package video

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ProbeMetadata queries ffprobe for container and stream properties.
func ProbeMetadata(filePath string) (*VideoMetadata, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate,duration,nb_frames:format=duration",
		"-of", "default=noprint_wrappers=1:nokey=0",
		filePath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to probe metadata for %s: %w", filePath, err)
	}

	meta := &VideoMetadata{}
	lines := strings.Split(out.String(), "\n")
	var formatDuration, streamDuration float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := parts[0], parts[1]
		switch k {
		case "width":
			meta.Width, _ = strconv.Atoi(v)
		case "height":
			meta.Height, _ = strconv.Atoi(v)
		case "duration":
			if d, err := strconv.ParseFloat(v, 64); err == nil {
				if streamDuration == 0 {
					streamDuration = d
				} else {
					formatDuration = d
				}
			}
		case "nb_frames":
			meta.TotalFrames, _ = strconv.Atoi(v)
		case "r_frame_rate":
			ratParts := strings.Split(v, "/")
			if len(ratParts) == 2 {
				num, _ := strconv.ParseFloat(ratParts[0], 64)
				den, _ := strconv.ParseFloat(ratParts[1], 64)
				if den > 0 {
					meta.FrameRate = num / den
				}
			}
		}
	}

	if streamDuration > 0 {
		meta.Duration = streamDuration
	} else {
		meta.Duration = formatDuration
	}

	// Check if audio stream exists
	audioCmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)
	var audioOut bytes.Buffer
	audioCmd.Stdout = &audioOut
	if err := audioCmd.Run(); err == nil && strings.TrimSpace(audioOut.String()) != "" {
		meta.HasAudio = true
	}

	return meta, nil
}

type probeFrame struct {
	KeyFrame int    `json:"key_frame"`
	PtsTime  string `json:"pts_time"`
	PictType string `json:"pict_type"`
}

type probeFrameOutput struct {
	Frames []probeFrame `json:"frames"`
}

// ProbeKeyframes queries ffprobe for exact presentation timestamps (pts_time) of all I-frames.
func ProbeKeyframes(filePath string) ([]Keyframe, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "frame=pts_time,pict_type,key_frame",
		"-of", "json",
		filePath,
	)

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe keyframe probe failed: %w, stderr: %s", err, errOut.String())
	}

	var parsed probeFrameOutput
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe json output: %w", err)
	}

	var keyframes []Keyframe
	for _, frame := range parsed.Frames {
		if frame.KeyFrame == 1 || frame.PictType == "I" {
			pts, err := strconv.ParseFloat(frame.PtsTime, 64)
			if err == nil {
				keyframes = append(keyframes, Keyframe{
					PtsTime: pts,
					PicType: "I",
				})
			}
		}
	}

	return keyframes, nil
}

// CalculateGOPChunks splits a total duration into continuous ChunkRanges, ensuring each
// chunk boundary snaps to an I-frame timestamp to avoid decoding artifacts.
//
// Invariants:
// 1. chunks[0].StartSec == 0.0
// 2. chunks[i].EndSec == chunks[i+1].StartSec
// 3. chunks[last].EndSec == totalDuration
// 4. For i > 0, chunks[i].StartSec == keyframes[k].PtsTime for some keyframe k.
func CalculateGOPChunks(totalDuration float64, keyframes []Keyframe, targetChunkSec float64) []ChunkRange {
	if targetChunkSec <= 0 || totalDuration <= 0 {
		return []ChunkRange{
			{
				ChunkIndex: 0,
				StartSec:   0.0,
				EndSec:     totalDuration,
				Duration:   totalDuration,
			},
		}
	}

	if len(keyframes) == 0 {
		// Fallback if no keyframe info: single chunk or naive split
		return []ChunkRange{
			{
				ChunkIndex: 0,
				StartSec:   0.0,
				EndSec:     totalDuration,
				Duration:   totalDuration,
			},
		}
	}

	var splitPoints []float64
	splitPoints = append(splitPoints, 0.0)

	currentBoundary := targetChunkSec
	lastAssigned := 0.0

	for currentBoundary < totalDuration-(targetChunkSec*0.25) { // don't make a tiny sliver chunk at the end
		// Find the keyframe closest to currentBoundary that is strictly greater than lastAssigned
		var bestKeyframe *Keyframe
		minDiff := 1e9

		for _, kf := range keyframes {
			if kf.PtsTime <= lastAssigned {
				continue
			}
			if kf.PtsTime >= totalDuration {
				continue
			}
			diff := kf.PtsTime - currentBoundary
			if diff < 0 {
				diff = -diff
			}
			if diff < minDiff {
				minDiff = diff
				kfCopy := kf
				bestKeyframe = &kfCopy
			}
		}

		if bestKeyframe != nil && bestKeyframe.PtsTime > lastAssigned {
			splitPoints = append(splitPoints, bestKeyframe.PtsTime)
			lastAssigned = bestKeyframe.PtsTime
			currentBoundary = bestKeyframe.PtsTime + targetChunkSec
		} else {
			// No more distinct keyframes found ahead
			break
		}
	}

	splitPoints = append(splitPoints, totalDuration)

	var chunks []ChunkRange
	for i := 0; i < len(splitPoints)-1; i++ {
		start := splitPoints[i]
		end := splitPoints[i+1]
		chunks = append(chunks, ChunkRange{
			ChunkIndex: i,
			StartSec:   start,
			EndSec:     end,
			Duration:   end - start,
		})
	}

	return chunks
}

// RenderChunk executes ffmpeg to encode a specific time segment with accurate PTS handling.
func RenderChunk(opts ChunkRenderOpts) error {
	if opts.Codec == "" {
		opts.Codec = "libx264"
	}
	if opts.Preset == "" {
		opts.Preset = "veryfast"
	}
	if opts.Bitrate == "" {
		opts.Bitrate = "3M"
	}

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.6f", opts.StartSec),
		"-to", fmt.Sprintf("%.6f", opts.EndSec),
		"-i", opts.SourcePath,
	}

	if opts.VideoFilter != "" {
		args = append(args, "-vf", opts.VideoFilter)
	}

	args = append(args,
		"-c:v", opts.Codec,
		"-preset", opts.Preset,
		"-b:v", opts.Bitrate,
		"-an", // audio extracted separately
	)

	if opts.AvoidNegativeTs {
		args = append(args, "-avoid_negative_ts", "make_zero")
	}

	args = append(args, opts.OutputPath)

	cmd := exec.Command("ffmpeg", args...)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg chunk render failed (range %.2f-%.2f): %w, details: %s",
			opts.StartSec, opts.EndSec, err, errOut.String())
	}

	return nil
}

// ExtractAudio extracts the uncompressed or native audio stream once to prevent sample drift.
func ExtractAudio(sourcePath, audioOutPath string) error {
	cmd := exec.Command("ffmpeg",
		"-y",
		"-i", sourcePath,
		"-vn",
		"-c:a", "copy",
		audioOutPath,
	)

	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract audio: %w, details: %s", err, errOut.String())
	}
	return nil
}

// StitchChunks performs lossless stream concatenation of all chunk files and remuxes the audio track.
func StitchChunks(chunkPaths []string, audioPath string, outputPath string) error {
	if len(chunkPaths) == 0 {
		return fmt.Errorf("no chunk paths provided for stitching")
	}

	// Create temporary concat manifest
	tmpDir := filepath.Dir(outputPath)
	manifestPath := filepath.Join(tmpDir, "concat_manifest.txt")

	var manifestBuf bytes.Buffer
	for _, p := range chunkPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			absPath = p
		}
		manifestBuf.WriteString(fmt.Sprintf("file '%s'\n", absPath))
	}

	if err := os.WriteFile(manifestPath, manifestBuf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write concat manifest: %w", err)
	}
	defer os.Remove(manifestPath)

	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", manifestPath,
	}

	hasAudio := audioPath != ""
	if hasAudio {
		if _, err := os.Stat(audioPath); err == nil {
			args = append(args, "-i", audioPath)
		} else {
			hasAudio = false
		}
	}

	args = append(args, "-c:v", "copy")

	if hasAudio {
		args = append(args, "-c:a", "copy")
	}

	args = append(args, "-movflags", "+faststart", outputPath)

	cmd := exec.Command("ffmpeg", args...)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat stitch failed: %w, details: %s", err, errOut.String())
	}

	return nil
}

// AuditAccuracy performs exact frame count and duration validation between original and stitched files.
func AuditAccuracy(sourcePath, outputPath string) (*AccuracyReport, error) {
	cmdSource := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=nb_read_frames:format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		sourcePath,
	)
	var outSource bytes.Buffer
	cmdSource.Stdout = &outSource
	if err := cmdSource.Run(); err != nil {
		return nil, fmt.Errorf("failed to probe source frames: %w", err)
	}

	cmdOutput := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=nb_read_frames:format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		outputPath,
	)
	var outOutput bytes.Buffer
	cmdOutput.Stdout = &outOutput
	if err := cmdOutput.Run(); err != nil {
		return nil, fmt.Errorf("failed to probe output frames: %w", err)
	}

	sourceLines := strings.Split(strings.TrimSpace(outSource.String()), "\n")
	outputLines := strings.Split(strings.TrimSpace(outOutput.String()), "\n")

	report := &AccuracyReport{}

	if len(sourceLines) >= 2 {
		report.SourceFrames, _ = strconv.Atoi(sourceLines[0])
		report.SourceDuration, _ = strconv.ParseFloat(sourceLines[1], 64)
	}
	if len(outputLines) >= 2 {
		report.OutputFrames, _ = strconv.Atoi(outputLines[0])
		report.OutputDuration, _ = strconv.ParseFloat(outputLines[1], 64)
	}

	report.IsFrameExact = (report.SourceFrames == report.OutputFrames) && report.SourceFrames > 0
	return report, nil
}
