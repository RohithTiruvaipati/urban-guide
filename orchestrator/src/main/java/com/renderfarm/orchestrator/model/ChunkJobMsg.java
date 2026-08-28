package com.renderfarm.orchestrator.model;

public class ChunkJobMsg {
    private String jobId;
    private int chunkIndex;
    private int totalChunks;
    private String sourcePath;
    private String outputPath;
    private double startSec;
    private double endSec;
    private String codec;
    private String preset;
    private String bitrate;
    private String videoFilter;
    private boolean avoidNegativeTs;

    public ChunkJobMsg() {}

    public ChunkJobMsg(String jobId, int chunkIndex, int totalChunks, String sourcePath, String outputPath,
                       double startSec, double endSec, String codec, String preset, String bitrate,
                       String videoFilter, boolean avoidNegativeTs) {
        this.jobId = jobId;
        this.chunkIndex = chunkIndex;
        this.totalChunks = totalChunks;
        this.sourcePath = sourcePath;
        this.outputPath = outputPath;
        this.startSec = startSec;
        this.endSec = endSec;
        this.codec = codec;
        this.preset = preset;
        this.bitrate = bitrate;
        this.videoFilter = videoFilter;
        this.avoidNegativeTs = avoidNegativeTs;
    }

    public String getJobId() { return jobId; }
    public void setJobId(String jobId) { this.jobId = jobId; }

    public int getChunkIndex() { return chunkIndex; }
    public void setChunkIndex(int chunkIndex) { this.chunkIndex = chunkIndex; }

    public int getTotalChunks() { return totalChunks; }
    public void setTotalChunks(int totalChunks) { this.totalChunks = totalChunks; }

    public String getSourcePath() { return sourcePath; }
    public void setSourcePath(String sourcePath) { this.sourcePath = sourcePath; }

    public String getOutputPath() { return outputPath; }
    public void setOutputPath(String outputPath) { this.outputPath = outputPath; }

    public double getStartSec() { return startSec; }
    public void setStartSec(double startSec) { this.startSec = startSec; }

    public double getEndSec() { return endSec; }
    public void setEndSec(double endSec) { this.endSec = endSec; }

    public String getCodec() { return codec; }
    public void setCodec(String codec) { this.codec = codec; }

    public String getPreset() { return preset; }
    public void setPreset(String preset) { this.preset = preset; }

    public String getBitrate() { return bitrate; }
    public void setBitrate(String bitrate) { this.bitrate = bitrate; }

    public String getVideoFilter() { return videoFilter; }
    public void setVideoFilter(String videoFilter) { this.videoFilter = videoFilter; }

    public boolean isAvoidNegativeTs() { return avoidNegativeTs; }
    public void setAvoidNegativeTs(boolean avoidNegativeTs) { this.avoidNegativeTs = avoidNegativeTs; }
}
