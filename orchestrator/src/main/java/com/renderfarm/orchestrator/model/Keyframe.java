package com.renderfarm.orchestrator.model;

public class Keyframe {
    private double ptsTime;
    private String pictType;

    public Keyframe() {}

    public Keyframe(double ptsTime, String pictType) {
        this.ptsTime = ptsTime;
        this.pictType = pictType;
    }

    public double getPtsTime() { return ptsTime; }
    public void setPtsTime(double ptsTime) { this.ptsTime = ptsTime; }

    public String getPictType() { return pictType; }
    public void setPictType(String pictType) { this.pictType = pictType; }
}
