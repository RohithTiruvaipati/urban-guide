package com.renderfarm.orchestrator.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.renderfarm.orchestrator.model.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.scheduling.annotation.Async;
import org.springframework.stereotype.Service;

import java.io.File;
import java.time.Instant;
import java.util.*;

@Service
public class JobOrchestratorService {

    private static final Logger log = LoggerFactory.getLogger(JobOrchestratorService.class);

    private final RedisTemplate<String, Object> redisTemplate;
    private final KafkaTemplate<String, Object> kafkaTemplate;
    private final KeyframeProbeService probeService;
    private final XmlTimelineParserService xmlParserService;
    private final StitcherService stitcherService;
    private final ObjectMapper objectMapper;

    @Value("${render-farm.topics.jobs:render.jobs}")
    private String jobsTopic;

    @Value("${render-farm.default-chunk-sec:5.0}")
    private double defaultChunkSec;

    @Value("${render-farm.storage-dir:./test_assets/storage}")
    private String storageDir;

    public JobOrchestratorService(RedisTemplate<String, Object> redisTemplate,
                                  KafkaTemplate<String, Object> kafkaTemplate,
                                  KeyframeProbeService probeService,
                                  XmlTimelineParserService xmlParserService,
                                  StitcherService stitcherService,
                                  ObjectMapper objectMapper) {
        this.redisTemplate = redisTemplate;
        this.kafkaTemplate = kafkaTemplate;
        this.probeService = probeService;
        this.xmlParserService = xmlParserService;
        this.stitcherService = stitcherService;
        this.objectMapper = objectMapper;
    }

    public JobStatusResponse submitJob(JobRequest request) {
        String jobId = "job-" + UUID.randomUUID().toString().substring(0, 8);
        double targetChunkSec = (request.getTargetChunkSec() != null && request.getTargetChunkSec() > 0)
                ? request.getTargetChunkSec()
                : defaultChunkSec;

        String codec = (request.getCodec() != null && !request.getCodec().isBlank()) ? request.getCodec() : "libx264";
        String preset = (request.getPreset() != null && !request.getPreset().isBlank()) ? request.getPreset() : "medium";
        Integer crf = request.getCrf();
        String bitrate = (request.getBitrate() != null && !request.getBitrate().isBlank()) ? request.getBitrate() : (crf == null ? "15M" : null);
        String videoFilter = (request.getVideoFilter() != null) ? request.getVideoFilter() : "hue=s=1.1,eq=contrast=1.05";

        File jobDir = new File(storageDir, jobId);
        jobDir.mkdirs();

        String finalFilename = (request.getOutputFilename() != null && !request.getOutputFilename().isBlank())
                ? request.getOutputFilename()
                : "rendered_output.mp4";
        String finalOutputPath = new File(jobDir, finalFilename).getAbsolutePath();

        boolean isXmlTimeline = request.getSourcePath() != null && request.getSourcePath().toLowerCase().endsWith(".xml");
        Instant now = Instant.now();
        String jobKey = "job:" + jobId;
        String chunksKey = "job:" + jobId + ":chunks";

        if (isXmlTimeline) {
            log.info("🎬 Submitting XML Timeline render job [{}] from project: {}", jobId, request.getSourcePath());
            List<XmlTimelineClip> xmlClips = xmlParserService.parseXmlTimeline(request.getSourcePath());
            if (xmlClips.isEmpty()) {
                throw new IllegalArgumentException("No valid video clips found in XML timeline: " + request.getSourcePath());
            }

            log.info("📦 XML Timeline [{}] partitioned into {} parallel clip cuts", jobId, xmlClips.size());

            // Initialize Redis state (no separate master audio needed since audio is rendered per clip)
            Map<String, Object> jobMeta = new HashMap<>();
            jobMeta.put("id", jobId);
            jobMeta.put("status", JobStatus.RUNNING.name());
            jobMeta.put("totalChunks", String.valueOf(xmlClips.size()));
            jobMeta.put("completedChunks", "0");
            jobMeta.put("sourcePath", request.getSourcePath());
            jobMeta.put("audioPath", "");
            jobMeta.put("finalOutputPath", finalOutputPath);
            jobMeta.put("codec", codec);
            jobMeta.put("preset", preset);
            if (bitrate != null) jobMeta.put("bitrate", bitrate);
            if (crf != null) jobMeta.put("crf", String.valueOf(crf));
            jobMeta.put("createdAt", now.toString());
            jobMeta.put("updatedAt", now.toString());

            redisTemplate.opsForHash().putAll(jobKey, jobMeta);

            // Publish each XML timeline cut as an independent parallel chunk
            for (XmlTimelineClip clip : xmlClips) {
                String chunkFileName = String.format("chunk_%03d.mp4", clip.getClipIndex());
                String chunkOutputPath = new File(jobDir, chunkFileName).getAbsolutePath();

                try {
                    Map<String, Object> chunkInfo = new HashMap<>();
                    chunkInfo.put("index", clip.getClipIndex());
                    chunkInfo.put("status", "PENDING");
                    chunkInfo.put("name", clip.getName());
                    chunkInfo.put("sourcePath", clip.getSourceFilePath());
                    chunkInfo.put("startSec", clip.getSourceInSec());
                    chunkInfo.put("endSec", clip.getSourceOutSec());
                    chunkInfo.put("duration", clip.getDurationSec());
                    chunkInfo.put("outputPath", chunkOutputPath);

                    String chunkJson = objectMapper.writeValueAsString(chunkInfo);
                    redisTemplate.opsForHash().put(chunksKey, String.valueOf(clip.getClipIndex()), chunkJson);
                } catch (Exception e) {
                    log.error("Failed to write initial chunk state for clip {}: {}", clip.getClipIndex(), e.getMessage());
                }

                ChunkJobMsg jobMsg = new ChunkJobMsg(
                        jobId,
                        clip.getClipIndex(),
                        xmlClips.size(),
                        clip.getSourceFilePath(),
                        chunkOutputPath,
                        clip.getSourceInSec(),
                        clip.getSourceOutSec(),
                        clip.getDurationSec(),
                        codec,
                        preset,
                        bitrate,
                        crf,
                        videoFilter,
                        true,
                        true // includeAudio for XML cuts
                );

                kafkaTemplate.send(jobsTopic, jobId + "-" + clip.getClipIndex(), jobMsg);
            }

            log.info("📤 Dispatched {} XML clip cuts to Kafka topic '{}'", xmlClips.size(), jobsTopic);
            return getJobStatus(jobId);
        }

        String audioPath = new File(jobDir, "master_audio.aac").getAbsolutePath();
        log.info("🚀 Submitting video transcode job [{}] for source: {}", jobId, request.getSourcePath());

        // Probe duration & keyframes
        double duration = probeService.probeDuration(request.getSourcePath());
        List<Keyframe> keyframes = probeService.probeKeyframes(request.getSourcePath());
        List<ChunkRange> chunks = probeService.calculateGOPChunks(duration, keyframes, targetChunkSec);

        log.info("📦 Job [{}] partitioned into {} GOP-aligned chunks (duration: {}s, targetChunk: {}s)",
                jobId, chunks.size(), duration, targetChunkSec);

        // Extract master audio track once
        stitcherService.extractAudio(request.getSourcePath(), audioPath);

        // Initialize Redis state
        Map<String, Object> jobMeta = new HashMap<>();
        jobMeta.put("id", jobId);
        jobMeta.put("status", JobStatus.RUNNING.name());
        jobMeta.put("totalChunks", String.valueOf(chunks.size()));
        jobMeta.put("completedChunks", "0");
        jobMeta.put("sourcePath", request.getSourcePath());
        jobMeta.put("audioPath", audioPath);
        jobMeta.put("finalOutputPath", finalOutputPath);
        jobMeta.put("codec", codec);
        jobMeta.put("preset", preset);
        if (bitrate != null) {
            jobMeta.put("bitrate", bitrate);
        }
        if (crf != null) {
            jobMeta.put("crf", String.valueOf(crf));
        }
        jobMeta.put("createdAt", now.toString());
        jobMeta.put("updatedAt", now.toString());

        redisTemplate.opsForHash().putAll(jobKey, jobMeta);

        // Publish chunks to Kafka & track in Redis
        for (ChunkRange chunk : chunks) {
            String chunkFileName = String.format("chunk_%03d.mp4", chunk.getChunkIndex());
            String chunkOutputPath = new File(jobDir, chunkFileName).getAbsolutePath();

            try {
                Map<String, Object> chunkInfo = new HashMap<>();
                chunkInfo.put("index", chunk.getChunkIndex());
                chunkInfo.put("status", "PENDING");
                chunkInfo.put("startSec", chunk.getStartSec());
                chunkInfo.put("endSec", chunk.getEndSec());
                chunkInfo.put("duration", chunk.getDuration());
                chunkInfo.put("outputPath", chunkOutputPath);

                String chunkJson = objectMapper.writeValueAsString(chunkInfo);
                redisTemplate.opsForHash().put(chunksKey, String.valueOf(chunk.getChunkIndex()), chunkJson);
            } catch (Exception e) {
                log.error("Failed to write initial chunk state for chunk {}: {}", chunk.getChunkIndex(), e.getMessage());
            }

            ChunkJobMsg jobMsg = new ChunkJobMsg(
                    jobId,
                    chunk.getChunkIndex(),
                    chunks.size(),
                    request.getSourcePath(),
                    chunkOutputPath,
                    chunk.getStartSec(),
                    chunk.getEndSec(),
                    chunk.getDuration(),
                    codec,
                    preset,
                    bitrate,
                    crf,
                    videoFilter,
                    true,
                    false // includeAudio is false because master audio is extracted once
            );

            kafkaTemplate.send(jobsTopic, jobId + "-" + chunk.getChunkIndex(), jobMsg);
        }

        log.info("📤 Dispatched {} chunks to Kafka topic '{}'", chunks.size(), jobsTopic);

        return getJobStatus(jobId);
    }

    public JobStatusResponse getJobStatus(String jobId) {
        String jobKey = "job:" + jobId;
        String chunksKey = "job:" + jobId + ":chunks";

        Map<Object, Object> rawMeta = redisTemplate.opsForHash().entries(jobKey);
        if (rawMeta.isEmpty()) {
            return null;
        }

        int totalChunks = Integer.parseInt(String.valueOf(rawMeta.getOrDefault("totalChunks", "0")));
        int completedChunks = Integer.parseInt(String.valueOf(rawMeta.getOrDefault("completedChunks", "0")));
        double progress = totalChunks > 0 ? ((double) completedChunks / totalChunks) * 100.0 : 0.0;

        Map<Object, Object> rawChunks = redisTemplate.opsForHash().entries(chunksKey);
        Map<String, Object> chunksMap = new HashMap<>();
        for (Map.Entry<Object, Object> entry : rawChunks.entrySet()) {
            try {
                Object val = entry.getValue();
                if (val instanceof String strVal && (strVal.startsWith("{") || strVal.startsWith("["))) {
                    chunksMap.put(String.valueOf(entry.getKey()), objectMapper.readValue(strVal, Map.class));
                } else {
                    chunksMap.put(String.valueOf(entry.getKey()), val);
                }
            } catch (Exception e) {
                chunksMap.put(String.valueOf(entry.getKey()), entry.getValue());
            }
        }

        JobStatus status = JobStatus.valueOf(String.valueOf(rawMeta.getOrDefault("status", "PENDING")));

        Long renderDurationMs = rawMeta.containsKey("renderDurationMs")
                ? Long.parseLong(String.valueOf(rawMeta.get("renderDurationMs")))
                : null;
        Long totalDurationMs = rawMeta.containsKey("totalDurationMs")
                ? Long.parseLong(String.valueOf(rawMeta.get("totalDurationMs")))
                : null;

        return new JobStatusResponse(
                jobId,
                status,
                totalChunks,
                completedChunks,
                Math.round(progress * 10.0) / 10.0,
                String.valueOf(rawMeta.get("sourcePath")),
                String.valueOf(rawMeta.get("finalOutputPath")),
                renderDurationMs,
                totalDurationMs,
                chunksMap,
                (String) rawMeta.get("errorMessage")
        );
    }

    @Async
    public void completeJobStitch(String jobId) {
        String jobKey = "job:" + jobId;
        String chunksKey = "job:" + jobId + ":chunks";

        Map<Object, Object> rawMeta = redisTemplate.opsForHash().entries(jobKey);
        if (rawMeta.isEmpty()) {
            return;
        }

        redisTemplate.opsForHash().put(jobKey, "status", JobStatus.STITCHING.name());
        redisTemplate.opsForHash().put(jobKey, "updatedAt", Instant.now().toString());

        log.info("🧩 All chunks completed for Job [{}]. Starting lossless stitch...", jobId);

        int totalChunks = Integer.parseInt(String.valueOf(rawMeta.get("totalChunks")));
        String audioPath = String.valueOf(rawMeta.get("audioPath"));
        String finalOutputPath = String.valueOf(rawMeta.get("finalOutputPath"));
        String sourcePath = String.valueOf(rawMeta.get("sourcePath"));

        List<String> chunkFiles = new ArrayList<>();
        Map<Object, Object> rawChunks = redisTemplate.opsForHash().entries(chunksKey);

        for (int i = 0; i < totalChunks; i++) {
            Object chunkObj = rawChunks.get(String.valueOf(i));
            if (chunkObj != null) {
                try {
                    JsonNode node = objectMapper.readTree(String.valueOf(chunkObj));
                    if (node.has("outputPath")) {
                        chunkFiles.add(node.get("outputPath").asText());
                    }
                } catch (Exception e) {
                    log.error("Failed to parse chunk object for chunk {}: {}", i, e.getMessage());
                }
            }
        }

        long stitchStart = System.currentTimeMillis();
        boolean success = stitcherService.stitchChunks(chunkFiles, audioPath, finalOutputPath);
        long stitchDuration = System.currentTimeMillis() - stitchStart;

        Instant now = Instant.now();
        Instant createdAt = Instant.parse(String.valueOf(rawMeta.get("createdAt")));
        long totalWallClock = now.toEpochMilli() - createdAt.toEpochMilli();

        if (success) {
            long finalFrames = stitcherService.probeFrameCount(finalOutputPath);
            long sourceFrames = stitcherService.probeFrameCount(sourcePath);

            log.info("✅ Job [{}] STITCHED SUCCESSFULLY in {}ms! (Source: {} frames, Final: {} frames)",
                    jobId, stitchDuration, sourceFrames, finalFrames);

            redisTemplate.opsForHash().put(jobKey, "status", JobStatus.COMPLETED.name());
            redisTemplate.opsForHash().put(jobKey, "totalDurationMs", String.valueOf(totalWallClock));
            redisTemplate.opsForHash().put(jobKey, "updatedAt", now.toString());
        } else {
            log.error("❌ Job [{}] stitching failed!", jobId);
            redisTemplate.opsForHash().put(jobKey, "status", JobStatus.FAILED.name());
            redisTemplate.opsForHash().put(jobKey, "errorMessage", "ffmpeg concat stitch failed");
            redisTemplate.opsForHash().put(jobKey, "updatedAt", now.toString());
        }
    }
}
