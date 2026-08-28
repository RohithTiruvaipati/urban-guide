package com.renderfarm.orchestrator.model;

import java.time.Instant;

public class JobState {
    private String id;
    private JobStatus status;
    private int totalChunks;
    private int completedChunks;
    private String sourcePath;
    private String audioPath;
    private String finalOutputPath;
    private String codec;
    private String preset;
    private String bitrate;
    private Instant createdAt;
    private Instant updatedAt;
    private Long renderDurationMs;
    private Long totalDurationMs;
    private String errorMessage;

    public JobState() {}

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public JobStatus getStatus() { return status; }
    public void setStatus(JobStatus status) { this.status = status; }

    public int getTotalChunks() { return totalChunks; }
    public void setTotalChunks(int totalChunks) { this.totalChunks = totalChunks; }

    public int getCompletedChunks() { return completedChunks; }
    public void setCompletedChunks(int completedChunks) { this.completedChunks = completedChunks; }

    public String getSourcePath() { return sourcePath; }
    public void setSourcePath(String sourcePath) { this.sourcePath = sourcePath; }

    public String getAudioPath() { return audioPath; }
    public void setAudioPath(String audioPath) { this.audioPath = audioPath; }

    public String getFinalOutputPath() { return finalOutputPath; }
    public void setFinalOutputPath(String finalOutputPath) { this.finalOutputPath = finalOutputPath; }

    public String getCodec() { return codec; }
    public void setCodec(String codec) { this.codec = codec; }

    public String getPreset() { return preset; }
    public void setPreset(String preset) { this.preset = preset; }

    public String getBitrate() { return bitrate; }
    public void setBitrate(String bitrate) { this.bitrate = bitrate; }

    public Instant getCreatedAt() { return createdAt; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }

    public Instant getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }

    public Long getRenderDurationMs() { return renderDurationMs; }
    public void setRenderDurationMs(Long renderDurationMs) { this.renderDurationMs = renderDurationMs; }

    public Long getTotalDurationMs() { return totalDurationMs; }
    public void setTotalDurationMs(Long totalDurationMs) { this.totalDurationMs = totalDurationMs; }

    public String getErrorMessage() { return errorMessage; }
    public void setErrorMessage(String errorMessage) { this.errorMessage = errorMessage; }
}
