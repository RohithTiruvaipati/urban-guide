package com.renderfarm.orchestrator.model;

public class JobRequest {
    private String sourcePath;
    private Double targetChunkSec;
    private String codec;
    private String preset;
    private String bitrate;
    private Integer crf;
    private String videoFilter;
    private String outputFilename;

    public JobRequest() {}

    public JobRequest(String sourcePath, Double targetChunkSec, String codec, String preset, String bitrate, Integer crf, String videoFilter, String outputFilename) {
        this.sourcePath = sourcePath;
        this.targetChunkSec = targetChunkSec;
        this.codec = codec;
        this.preset = preset;
        this.bitrate = bitrate;
        this.crf = crf;
        this.videoFilter = videoFilter;
        this.outputFilename = outputFilename;
    }

    public String getSourcePath() { return sourcePath; }
    public void setSourcePath(String sourcePath) { this.sourcePath = sourcePath; }

    public Double getTargetChunkSec() { return targetChunkSec; }
    public void setTargetChunkSec(Double targetChunkSec) { this.targetChunkSec = targetChunkSec; }

    public String getCodec() { return codec; }
    public void setCodec(String codec) { this.codec = codec; }

    public String getPreset() { return preset; }
    public void setPreset(String preset) { this.preset = preset; }

    public String getBitrate() { return bitrate; }
    public void setBitrate(String bitrate) { this.bitrate = bitrate; }

    public Integer getCrf() { return crf; }
    public void setCrf(Integer crf) { this.crf = crf; }

    public String getVideoFilter() { return videoFilter; }
    public void setVideoFilter(String videoFilter) { this.videoFilter = videoFilter; }

    public String getOutputFilename() { return outputFilename; }
    public void setOutputFilename(String outputFilename) { this.outputFilename = outputFilename; }
}
