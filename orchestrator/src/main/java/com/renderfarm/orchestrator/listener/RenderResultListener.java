package com.renderfarm.orchestrator.listener;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.renderfarm.orchestrator.model.ChunkResultMsg;
import com.renderfarm.orchestrator.model.JobStatus;
import com.renderfarm.orchestrator.service.JobOrchestratorService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

@Component
public class RenderResultListener {

    private static final Logger log = LoggerFactory.getLogger(RenderResultListener.class);

    private final RedisTemplate<String, Object> redisTemplate;
    private final JobOrchestratorService jobOrchestratorService;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public RenderResultListener(RedisTemplate<String, Object> redisTemplate, JobOrchestratorService jobOrchestratorService) {
        this.redisTemplate = redisTemplate;
        this.jobOrchestratorService = jobOrchestratorService;
    }

    @KafkaListener(
            topics = "${render-farm.topics.results:render.results}",
            groupId = "${spring.kafka.consumer.group-id:orchestrator-group}"
    )
    public void onChunkResult(String payload) {
        ChunkResultMsg result;
        try {
            result = objectMapper.readValue(payload, ChunkResultMsg.class);
        } catch (Exception e) {
            log.error("Failed to parse chunk result json: {}", payload, e);
            return;
        }

        String jobId = result.getJobId();
        int chunkIndex = result.getChunkIndex();

        log.info("📥 Received chunk result for Job [{}] Chunk #{}: status={}, worker={}, duration={}ms",
                jobId, chunkIndex, result.getStatus(), result.getWorkerId(), result.getDurationMs());

        String jobKey = "job:" + jobId;
        String chunksKey = "job:" + jobId + ":chunks";

        try {
            // Update individual chunk state as JSON string
            Map<String, Object> chunkUpdate = new HashMap<>();
            chunkUpdate.put("index", chunkIndex);
            chunkUpdate.put("status", "SUCCESS".equalsIgnoreCase(result.getStatus()) ? "DONE" : "FAILED");
            chunkUpdate.put("workerId", result.getWorkerId());
            chunkUpdate.put("durationMs", result.getDurationMs());
            chunkUpdate.put("outputPath", result.getOutputPath());
            chunkUpdate.put("error", result.getError());

            String chunkJson = objectMapper.writeValueAsString(chunkUpdate);
            redisTemplate.opsForHash().put(chunksKey, String.valueOf(chunkIndex), chunkJson);

            // Increment completed count safely
            Map<Object, Object> jobMeta = redisTemplate.opsForHash().entries(jobKey);
            int currentCompleted = 0;
            if (jobMeta.containsKey("completedChunks")) {
                currentCompleted = Integer.parseInt(String.valueOf(jobMeta.get("completedChunks")));
            }
            int newCompleted = currentCompleted + 1;
            int totalChunks = Integer.parseInt(String.valueOf(jobMeta.getOrDefault("totalChunks", "0")));

            redisTemplate.opsForHash().put(jobKey, "completedChunks", String.valueOf(newCompleted));
            redisTemplate.opsForHash().put(jobKey, "updatedAt", Instant.now().toString());

            log.info("📊 Job [{}] Progress: {}/{} chunks completed", jobId, newCompleted, totalChunks);

            if ("FAILED".equalsIgnoreCase(result.getStatus())) {
                redisTemplate.opsForHash().put(jobKey, "status", JobStatus.FAILED.name());
                redisTemplate.opsForHash().put(jobKey, "errorMessage", "Chunk " + chunkIndex + " failed: " + result.getError());
                return;
            }

            // Trigger stitch if all chunks are complete
            if (newCompleted >= totalChunks && totalChunks > 0) {
                jobOrchestratorService.completeJobStitch(jobId);
            }
        } catch (Exception e) {
            log.error("Failed to process chunk result for Job [{}]: {}", jobId, e.getMessage(), e);
        }
    }
}
