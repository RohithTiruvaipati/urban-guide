package com.renderfarm.orchestrator.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.renderfarm.orchestrator.model.ChunkRange;
import com.renderfarm.orchestrator.model.Keyframe;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

@Service
public class KeyframeProbeService {

    private static final Logger log = LoggerFactory.getLogger(KeyframeProbeService.class);
    private final ObjectMapper objectMapper = new ObjectMapper();

    public double probeDuration(String filePath) {
        try {
            ProcessBuilder pb = new ProcessBuilder(
                    "ffprobe",
                    "-v", "error",
                    "-show_entries", "format=duration",
                    "-of", "default=noprint_wrappers=1:nokey=1",
                    filePath
            );
            Process process = pb.start();
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8))) {
                String line = reader.readLine();
                if (line != null) {
                    return Double.parseDouble(line.trim());
                }
            }
            process.waitFor();
        } catch (Exception e) {
            log.error("Failed to probe duration for {}: {}", filePath, e.getMessage());
        }
        return 0.0;
    }

    public List<Keyframe> probeKeyframes(String filePath) {
        List<Keyframe> keyframes = new ArrayList<>();
        try {
            ProcessBuilder pb = new ProcessBuilder(
                    "ffprobe",
                    "-v", "error",
                    "-select_streams", "v:0",
                    "-show_entries", "frame=pts_time,pict_type,key_frame",
                    "-of", "json",
                    filePath
            );
            Process process = pb.start();
            JsonNode root = objectMapper.readTree(process.getInputStream());
            process.waitFor();

            JsonNode framesNode = root.get("frames");
            if (framesNode != null && framesNode.isArray()) {
                for (JsonNode frame : framesNode) {
                    boolean isKeyframe = (frame.has("key_frame") && frame.get("key_frame").asInt() == 1) ||
                            (frame.has("pict_type") && "I".equalsIgnoreCase(frame.get("pict_type").asText()));

                    if (isKeyframe && frame.has("pts_time")) {
                        double pts = frame.get("pts_time").asDouble();
                        keyframes.add(new Keyframe(pts, "I"));
                    }
                }
            }
        } catch (Exception e) {
            log.error("Failed to probe keyframes for {}: {}", filePath, e.getMessage());
        }
        return keyframes;
    }

    public List<ChunkRange> calculateGOPChunks(double totalDuration, List<Keyframe> keyframes, double targetChunkSec) {
        List<ChunkRange> chunks = new ArrayList<>();
        if (targetChunkSec <= 0 || totalDuration <= 0 || keyframes.isEmpty()) {
            chunks.add(new ChunkRange(0, 0.0, totalDuration, totalDuration));
            return chunks;
        }

        List<Double> splitPoints = new ArrayList<>();
        splitPoints.add(0.0);

        double currentBoundary = targetChunkSec;
        double lastAssigned = 0.0;

        while (currentBoundary < totalDuration - (targetChunkSec * 0.25)) {
            Keyframe bestKf = null;
            double minDiff = Double.MAX_VALUE;

            for (Keyframe kf : keyframes) {
                if (kf.getPtsTime() <= lastAssigned || kf.getPtsTime() >= totalDuration) {
                    continue;
                }
                double diff = Math.abs(kf.getPtsTime() - currentBoundary);
                if (diff < minDiff) {
                    minDiff = diff;
                    bestKf = kf;
                }
            }

            if (bestKf != null && bestKf.getPtsTime() > lastAssigned) {
                splitPoints.add(bestKf.getPtsTime());
                lastAssigned = bestKf.getPtsTime();
                currentBoundary = bestKf.getPtsTime() + targetChunkSec;
            } else {
                break;
            }
        }

        splitPoints.add(totalDuration);

        for (int i = 0; i < splitPoints.size() - 1; i++) {
            double start = splitPoints.get(i);
            double end = splitPoints.get(i + 1);
            chunks.add(new ChunkRange(i, start, end, end - start));
        }

        return chunks;
    }
}
