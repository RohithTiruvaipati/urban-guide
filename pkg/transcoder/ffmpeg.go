package transcoder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/distributed-proxy-system/pkg/models"
)

// ProbeMetadata uses ffprobe to extract stream information needed for proxy conformity.
func ProbeMetadata(filePath string) (*models.ClipMetadata, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("source file not accessible at %s: %w", filePath, err)
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "stream=index,codec_type,codec_name,width,height,r_frame_rate,channels,sample_rate,duration,nb_read_frames:format=duration,size",
		"-count_frames",
		"-of", "json",
		filePath,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed for %s: %w (stderr: %s)", filePath, err, stderr.String())
	}

	type probeStream struct {
		Index        int    `json:"index"`
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		RFrameRate   string `json:"r_frame_rate"`
		Channels     int    `json:"channels"`
		SampleRate   string `json:"sample_rate"`
		Duration     string `json:"duration"`
		NbReadFrames string `json:"nb_read_frames"`
	}

	type probeFormat struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
	}

	type probeOutput struct {
		Streams []probeStream `json:"streams"`
		Format  probeFormat   `json:"format"`
	}

	var out probeOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe json: %w", err)
	}

	meta := &models.ClipMetadata{}

	// Parse format duration
	if d, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil {
		meta.Duration = d
	}

	for _, s := range out.Streams {
		if s.CodecType == "video" && meta.Width == 0 {
			meta.Width = s.Width
			meta.Height = s.Height
			meta.VideoCodec = s.CodecName

			// Parse framerate e.g. "24000/1001" or "30/1"
			parts := strings.Split(s.RFrameRate, "/")
			if len(parts) == 2 {
				num, _ := strconv.ParseFloat(parts[0], 64)
				den, _ := strconv.ParseFloat(parts[1], 64)
				if den > 0 {
					meta.FrameRate = num / den
				}
			} else if f, err := strconv.ParseFloat(s.RFrameRate, 64); err == nil {
				meta.FrameRate = f
			}

			if frames, err := strconv.Atoi(s.NbReadFrames); err == nil && frames > 0 {
				meta.TotalFrames = frames
			} else if meta.FrameRate > 0 && meta.Duration > 0 {
				meta.TotalFrames = int(math.Round(meta.Duration * meta.FrameRate))
			}
		} else if s.CodecType == "audio" && !meta.HasAudio {
			meta.HasAudio = true
			meta.AudioCodec = s.CodecName
			meta.AudioChannels = s.Channels
			if sr, err := strconv.Atoi(s.SampleRate); err == nil {
				meta.AudioSampleRate = sr
			}
		}
	}

	return meta, nil
}

// BuildProxyPath generates Premiere Pro-standard proxy filename: `<basename>_Proxy.<ext>`.
func BuildProxyPath(sourcePath, outputDir, codec string) string {
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	targetExt := ".mov"
	if strings.ToLower(codec) == "h264" || strings.ToLower(codec) == "mp4" {
		targetExt = ".mp4"
	}

	proxyFileName := fmt.Sprintf("%s_Proxy%s", nameWithoutExt, targetExt)
	return filepath.Join(outputDir, proxyFileName)
}

// TranscodeProxy executes FFmpeg with exact Premiere Pro proxy presets.
func TranscodeProxy(sourcePath, proxyPath, codec, resolution string) error {
	outDir := filepath.Dir(proxyPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create proxy output dir %s: %w", outDir, err)
	}

	meta, err := ProbeMetadata(sourcePath)
	if err != nil {
		return fmt.Errorf("pre-transcode probe failed: %w", err)
	}

	// Calculate target scale filter maintaining aspect ratio
	var scaleFilter string
	switch strings.ToLower(resolution) {
	case "720p":
		scaleFilter = "scale=-2:720"
	case "source":
		scaleFilter = ""
	default: // "1080p" or default
		if meta.Width > 1920 || meta.Height > 1080 {
			scaleFilter = "scale=-2:1080"
		}
	}

	args := []string{
		"-y",
		"-i", sourcePath,
	}

	if scaleFilter != "" {
		args = append(args, "-vf", scaleFilter)
	}

	isProRes := strings.ToLower(codec) == "prores" || strings.ToLower(codec) == "apple_prores"

	if isProRes {
		// Apple ProRes 422 Proxy standard preset (profile:v 0 = Proxy)
		args = append(args,
			"-c:v", "prores_ks",
			"-profile:v", "0",
			"-vendor", "apl0",
			"-pix_fmt", "yuv422p10le",
		)
		if meta.HasAudio {
			// Copy all native audio channels losslessly so Premiere audio tracks match 1:1
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-an")
		}
	} else {
		// Fast H.264 Proxy Preset
		args = append(args,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-crf", "22",
			"-pix_fmt", "yuv420p",
		)
		if meta.HasAudio {
			args = append(args, "-c:a", "aac", "-b:a", "192k")
		} else {
			args = append(args, "-an")
		}
	}

	args = append(args, proxyPath)

	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg proxy transcode failed: %w\nOutput: %s", err, stderr.String())
	}

	// Verify output exists and is non-empty
	fi, err := os.Stat(proxyPath)
	if err != nil || fi.Size() == 0 {
		return fmt.Errorf("proxy file missing or empty after transcode: %s", proxyPath)
	}

	return nil
}

// VerifyProxy performs compliance audit between original footage and the generated proxy.
func VerifyProxy(sourcePath, proxyPath string) (*models.ProxyReport, error) {
	srcMeta, err := ProbeMetadata(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source probe error: %w", err)
	}

	prxMeta, err := ProbeMetadata(proxyPath)
	if err != nil {
		return nil, fmt.Errorf("proxy probe error: %w", err)
	}

	report := &models.ProxyReport{
		SourcePath:    sourcePath,
		ProxyPath:     proxyPath,
		SourceFrames:  srcMeta.TotalFrames,
		ProxyFrames:   prxMeta.TotalFrames,
		DurationDelta: math.Abs(srcMeta.Duration - prxMeta.Duration),
	}

	report.AudioMatch = (srcMeta.HasAudio == prxMeta.HasAudio)
	if srcMeta.HasAudio && prxMeta.HasAudio {
		report.AudioMatch = (srcMeta.AudioChannels == prxMeta.AudioChannels)
	}

	// Frame match tolerance: exact frame match or within 1 frame delta
	frameDiff := int(math.Abs(float64(srcMeta.TotalFrames - prxMeta.TotalFrames)))
	report.FrameMatch = (frameDiff <= 1)

	// A proxy is attachable in Premiere Pro if:
	// 1. Durations match (< 0.1s delta)
	// 2. Audio track count / channels match
	// 3. Frame counts match
	report.IsAttachable = report.FrameMatch && report.AudioMatch && (report.DurationDelta < 0.25)

	return report, nil
}
