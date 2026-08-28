package com.renderfarm.orchestrator.model;

public class ChunkResultMsg {
    private String jobId;
    private int chunkIndex;
    private String status;
    private String outputPath;
    private long durationMs;
    private String workerId;
    private String error;

    public ChunkResultMsg() {}

    public ChunkResultMsg(String jobId, int chunkIndex, String status, String outputPath, long durationMs, String workerId, String error) {
        this.jobId = jobId;
        this.chunkIndex = chunkIndex;
        this.status = status;
        this.outputPath = outputPath;
        this.durationMs = durationMs;
        this.workerId = workerId;
        this.error = error;
    }

    public String getJobId() { return jobId; }
    public void setJobId(String jobId) { this.jobId = jobId; }

    public int getChunkIndex() { return chunkIndex; }
    public void setChunkIndex(int chunkIndex) { this.chunkIndex = chunkIndex; }

    public String getStatus() { return status; }
    public void setStatus(String status) { this.status = status; }

    public String getOutputPath() { return outputPath; }
    public void setOutputPath(String outputPath) { this.outputPath = outputPath; }

    public long getDurationMs() { return durationMs; }
    public void setDurationMs(long durationMs) { this.durationMs = durationMs; }

    public String getWorkerId() { return workerId; }
    public void setWorkerId(String workerId) { this.workerId = workerId; }

    public String getError() { return error; }
    public void setError(String error) { this.error = error; }
}
