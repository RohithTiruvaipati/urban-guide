package com.renderfarm.orchestrator.model;

public class XmlTimelineClip {
    private int clipIndex;
    private String name;
    private String sourceFilePath;
    private double timelineStartSec;
    private double timelineEndSec;
    private double sourceInSec;
    private double sourceOutSec;
    private double durationSec;

    public XmlTimelineClip() {}

    public XmlTimelineClip(int clipIndex, String name, String sourceFilePath,
                           double timelineStartSec, double timelineEndSec,
                           double sourceInSec, double sourceOutSec, double durationSec) {
        this.clipIndex = clipIndex;
        this.name = name;
        this.sourceFilePath = sourceFilePath;
        this.timelineStartSec = timelineStartSec;
        this.timelineEndSec = timelineEndSec;
        this.sourceInSec = sourceInSec;
        this.sourceOutSec = sourceOutSec;
        this.durationSec = durationSec;
    }

    public int getClipIndex() { return clipIndex; }
    public void setClipIndex(int clipIndex) { this.clipIndex = clipIndex; }

    public String getName() { return name; }
    public void setName(String name) { this.name = name; }

    public String getSourceFilePath() { return sourceFilePath; }
    public void setSourceFilePath(String sourceFilePath) { this.sourceFilePath = sourceFilePath; }

    public double getTimelineStartSec() { return timelineStartSec; }
    public void setTimelineStartSec(double timelineStartSec) { this.timelineStartSec = timelineStartSec; }

    public double getTimelineEndSec() { return timelineEndSec; }
    public void setTimelineEndSec(double timelineEndSec) { this.timelineEndSec = timelineEndSec; }

    public double getSourceInSec() { return sourceInSec; }
    public void setSourceInSec(double sourceInSec) { this.sourceInSec = sourceInSec; }

    public double getSourceOutSec() { return sourceOutSec; }
    public void setSourceOutSec(double sourceOutSec) { this.sourceOutSec = sourceOutSec; }

    public double getDurationSec() { return durationSec; }
    public void setDurationSec(double durationSec) { this.durationSec = durationSec; }

    @Override
    public String toString() {
        return "XmlTimelineClip{" +
                "clipIndex=" + clipIndex +
                ", name='" + name + '\'' +
                ", sourceFilePath='" + sourceFilePath + '\'' +
                ", timelineStartSec=" + timelineStartSec +
                ", timelineEndSec=" + timelineEndSec +
                ", sourceInSec=" + sourceInSec +
                ", sourceOutSec=" + sourceOutSec +
                ", durationSec=" + durationSec +
                '}';
    }
}
