package com.renderfarm.orchestrator;

import com.renderfarm.orchestrator.model.ChunkRange;
import com.renderfarm.orchestrator.model.Keyframe;
import com.renderfarm.orchestrator.service.KeyframeProbeService;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class KeyframeProbeServiceTest {

    private final KeyframeProbeService probeService = new KeyframeProbeService();

    @Test
    void testCalculateGOPChunks_InvariantChecks() {
        List<Keyframe> keyframes = List.of(
                new Keyframe(0.0, "I"),
                new Keyframe(2.0, "I"),
                new Keyframe(4.0, "I"),
                new Keyframe(6.0, "I"),
                new Keyframe(8.0, "I"),
                new Keyframe(10.0, "I")
        );

        double totalDuration = 10.0;
        double targetChunkSec = 3.0;

        List<ChunkRange> chunks = probeService.calculateGOPChunks(totalDuration, keyframes, targetChunkSec);

        assertNotNull(chunks);
        assertFalse(chunks.isEmpty());

        // Invariant 1: Start at 0.0
        assertEquals(0.0, chunks.get(0).getStartSec(), 0.0001);

        // Invariant 2: Continuous boundaries (no gaps or overlaps)
        for (int i = 0; i < chunks.size() - 1; i++) {
            assertEquals(chunks.get(i).getEndSec(), chunks.get(i + 1).getStartSec(), 0.0001,
                    "Discontinuity between chunk " + i + " and " + (i + 1));
        }

        // Invariant 3: End at total duration
        assertEquals(totalDuration, chunks.get(chunks.size() - 1).getEndSec(), 0.0001);
    }
}
