package transcoder

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildProxyPath(t *testing.T) {
	tests := []struct {
		source   string
		outDir   string
		codec    string
		expected string
	}{
		{"/media/raw/A001_C001_08293M.MOV", "/media/proxies", "prores", "/media/proxies/A001_C001_08293M_Proxy.mov"},
		{"/media/raw/Clip_4K.mp4", "/media/proxies", "h264", "/media/proxies/Clip_4K_Proxy.mp4"},
		{"/footage/Interview.mxf", "./out", "prores", "out/Interview_Proxy.mov"},
	}

	for _, tt := range tests {
		got := BuildProxyPath(tt.source, tt.outDir, tt.codec)
		if filepath.Clean(got) != filepath.Clean(tt.expected) {
			t.Errorf("BuildProxyPath(%s, %s, %s) = %s, expected %s",
				tt.source, tt.outDir, tt.codec, got, tt.expected)
		}
	}
}

func TestTranscodeAndVerifyProxy_ProRes(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed, skipping integration test")
	}

	tmpDir, err := os.MkdirTemp("", "proxy_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create synthetic 4K test clip (2s, 30fps, with 2 audio channels)
	sourceClip := filepath.Join(tmpDir, "A001_C001_RAW.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=size=3840x2160:rate=30:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=1000:duration=2",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k",
		sourceClip,
	)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to generate synthetic 4K source: %v", err)
	}

	proxyClip := BuildProxyPath(sourceClip, tmpDir, "prores")

	// Transcode to ProRes Proxy
	if err := TranscodeProxy(sourceClip, proxyClip, "prores", "1080p"); err != nil {
		t.Fatalf("TranscodeProxy failed: %v", err)
	}

	// Verify compliance for Premiere Pro
	report, err := VerifyProxy(sourceClip, proxyClip)
	if err != nil {
		t.Fatalf("VerifyProxy failed: %v", err)
	}

	if !report.IsAttachable {
		t.Errorf("expected proxy to be Premiere-attachable, report: %+v", report)
	}
	if !report.FrameMatch {
		t.Errorf("frame count mismatch: source=%d, proxy=%d", report.SourceFrames, report.ProxyFrames)
	}
	if !report.AudioMatch {
		t.Errorf("audio channel mismatch between source and proxy")
	}
}
