package com.renderfarm.orchestrator.model;

public class ChunkRange {
    private int chunkIndex;
    private double startSec;
    private double endSec;
    private double duration;

    public ChunkRange() {}

    public ChunkRange(int chunkIndex, double startSec, double endSec, double duration) {
        this.chunkIndex = chunkIndex;
        this.startSec = startSec;
        this.endSec = endSec;
        this.duration = duration;
    }

    public int getChunkIndex() { return chunkIndex; }
    public void setChunkIndex(int chunkIndex) { this.chunkIndex = chunkIndex; }

    public double getStartSec() { return startSec; }
    public void setStartSec(double startSec) { this.startSec = startSec; }

    public double getEndSec() { return endSec; }
    public void setEndSec(double endSec) { this.endSec = endSec; }

    public double getDuration() { return duration; }
    public void setDuration(double duration) { this.duration = duration; }
}
