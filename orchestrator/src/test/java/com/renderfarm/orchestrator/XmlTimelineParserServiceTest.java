package com.renderfarm.orchestrator;

import com.renderfarm.orchestrator.model.XmlTimelineClip;
import com.renderfarm.orchestrator.service.XmlTimelineParserService;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class XmlTimelineParserServiceTest {

    private final XmlTimelineParserService parser = new XmlTimelineParserService();

    @Test
    void testParseXmlTimeline_PremiereXmeml(@TempDir Path tempDir) throws Exception {
        // Create dummy video files
        Path clip1 = Files.createFile(tempDir.resolve("SceneA.mp4"));
        Path clip2 = Files.createFile(tempDir.resolve("SceneB.mp4"));

        String xmlContent = """
                <?xml version="1.0" encoding="UTF-8"?>
                <!DOCTYPE xmeml>
                <xmeml version="4">
                  <sequence id="sequence-1">
                    <name>Test Sequence</name>
                    <duration>600</duration>
                    <rate>
                      <timebase>30</timebase>
                      <ntsc>FALSE</ntsc>
                    </rate>
                    <media>
                      <video>
                        <track>
                          <clipitem id="clipitem-1">
                            <name>SceneA.mp4</name>
                            <duration>300</duration>
                            <rate>
                              <timebase>30</timebase>
                            </rate>
                            <start>0</start>
                            <end>150</end>
                            <in>30</in>
                            <out>180</out>
                            <file id="file-1">
                              <name>SceneA.mp4</name>
                              <pathurl>file://localhost%s</pathurl>
                            </file>
                          </clipitem>
                          <clipitem id="clipitem-2">
                            <name>SceneB.mp4</name>
                            <duration>300</duration>
                            <rate>
                              <timebase>30</timebase>
                            </rate>
                            <start>150</start>
                            <end>300</end>
                            <in>60</in>
                            <out>210</out>
                            <file id="file-2">
                              <name>SceneB.mp4</name>
                              <pathurl>file://localhost%s</pathurl>
                            </file>
                          </clipitem>
                        </track>
                      </video>
                    </media>
                  </sequence>
                </xmeml>
                """.formatted(clip1.toAbsolutePath().toString(), clip2.toAbsolutePath().toString());

        Path xmlFile = tempDir.resolve("timeline.xml");
        Files.writeString(xmlFile, xmlContent);

        List<XmlTimelineClip> clips = parser.parseXmlTimeline(xmlFile.toAbsolutePath().toString());

        assertNotNull(clips);
        assertEquals(2, clips.size());

        // Clip 1: in=30, out=180 at 30fps -> in=1.0s, out=6.0s, duration=5.0s, start=0.0s, end=5.0s
        XmlTimelineClip c1 = clips.get(0);
        assertEquals(0, c1.getClipIndex());
        assertEquals("SceneA.mp4", c1.getName());
        assertEquals(1.0, c1.getSourceInSec(), 0.001);
        assertEquals(6.0, c1.getSourceOutSec(), 0.001);
        assertEquals(5.0, c1.getDurationSec(), 0.001);
        assertEquals(0.0, c1.getTimelineStartSec(), 0.001);
        assertEquals(5.0, c1.getTimelineEndSec(), 0.001);

        // Clip 2: in=60, out=210 at 30fps -> in=2.0s, out=7.0s, duration=5.0s, start=5.0s, end=10.0s
        XmlTimelineClip c2 = clips.get(1);
        assertEquals(1, c2.getClipIndex());
        assertEquals("SceneB.mp4", c2.getName());
        assertEquals(2.0, c2.getSourceInSec(), 0.001);
        assertEquals(7.0, c2.getSourceOutSec(), 0.001);
        assertEquals(5.0, c2.getDurationSec(), 0.001);
        assertEquals(5.0, c2.getTimelineStartSec(), 0.001);
        assertEquals(10.0, c2.getTimelineEndSec(), 0.001);
    }
}
