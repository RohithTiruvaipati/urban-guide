package com.renderfarm.orchestrator.model;

import java.util.Map;

public class JobStatusResponse {
    private String jobId;
    private JobStatus status;
    private int totalChunks;
    private int completedChunks;
    private double progressPercent;
    private String sourcePath;
    private String finalOutputPath;
    private Long renderDurationMs;
    private Long totalDurationMs;
    private Map<String, Object> chunks;
    private String errorMessage;

    public JobStatusResponse() {}

    public JobStatusResponse(String jobId, JobStatus status, int totalChunks, int completedChunks,
                             double progressPercent, String sourcePath, String finalOutputPath,
                             Long renderDurationMs, Long totalDurationMs, Map<String, Object> chunks, String errorMessage) {
        this.jobId = jobId;
        this.status = status;
        this.totalChunks = totalChunks;
        this.completedChunks = completedChunks;
        this.progressPercent = progressPercent;
        this.sourcePath = sourcePath;
        this.finalOutputPath = finalOutputPath;
        this.renderDurationMs = renderDurationMs;
        this.totalDurationMs = totalDurationMs;
        this.chunks = chunks;
        this.errorMessage = errorMessage;
    }

    public String getJobId() { return jobId; }
    public void setJobId(String jobId) { this.jobId = jobId; }

    public JobStatus getStatus() { return status; }
    public void setStatus(JobStatus status) { this.status = status; }

    public int getTotalChunks() { return totalChunks; }
    public void setTotalChunks(int totalChunks) { this.totalChunks = totalChunks; }

    public int getCompletedChunks() { return completedChunks; }
    public void setCompletedChunks(int completedChunks) { this.completedChunks = completedChunks; }

    public double getProgressPercent() { return progressPercent; }
    public void setProgressPercent(double progressPercent) { this.progressPercent = progressPercent; }

    public String getSourcePath() { return sourcePath; }
    public void setSourcePath(String sourcePath) { this.sourcePath = sourcePath; }

    public String getFinalOutputPath() { return finalOutputPath; }
    public void setFinalOutputPath(String finalOutputPath) { this.finalOutputPath = finalOutputPath; }

    public Long getRenderDurationMs() { return renderDurationMs; }
    public void setRenderDurationMs(Long renderDurationMs) { this.renderDurationMs = renderDurationMs; }

    public Long getTotalDurationMs() { return totalDurationMs; }
    public void setTotalDurationMs(Long totalDurationMs) { this.totalDurationMs = totalDurationMs; }

    public Map<String, Object> getChunks() { return chunks; }
    public void setChunks(Map<String, Object> chunks) { this.chunks = chunks; }

    public String getErrorMessage() { return errorMessage; }
    public void setErrorMessage(String errorMessage) { this.errorMessage = errorMessage; }
}
