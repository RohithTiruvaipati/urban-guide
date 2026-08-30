package com.renderfarm.orchestrator.service;

import com.renderfarm.orchestrator.model.XmlTimelineClip;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.w3c.dom.Document;
import org.w3c.dom.Element;
import org.w3c.dom.NodeList;

import javax.xml.parsers.DocumentBuilder;
import javax.xml.parsers.DocumentBuilderFactory;
import java.io.File;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;

@Service
public class XmlTimelineParserService {

    private static final Logger log = LoggerFactory.getLogger(XmlTimelineParserService.class);

    public List<XmlTimelineClip> parseXmlTimeline(String xmlFilePath) {
        List<XmlTimelineClip> clips = new ArrayList<>();
        File xmlFile = new File(xmlFilePath);
        if (!xmlFile.exists()) {
            log.error("XML timeline file does not exist: {}", xmlFilePath);
            return clips;
        }

        try {
            DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
            factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", false);
            factory.setFeature("http://xml.org/sax/features/external-general-entities", false);
            factory.setFeature("http://xml.org/sax/features/external-parameter-entities", false);
            DocumentBuilder builder = factory.newDocumentBuilder();
            Document doc = builder.parse(xmlFile);
            doc.getDocumentElement().normalize();

            // Extract default sequence timebase (framerate)
            double timebase = 30.0;
            NodeList rateNodes = doc.getElementsByTagName("rate");
            if (rateNodes.getLength() > 0) {
                Element rateElem = (Element) rateNodes.item(0);
                NodeList tbNodes = rateElem.getElementsByTagName("timebase");
                if (tbNodes.getLength() > 0) {
                    try {
                        timebase = Double.parseDouble(tbNodes.item(0).getTextContent().trim());
                    } catch (NumberFormatException ignored) {}
                }
            }

            log.info("🎞️ Parsing XML timeline [{}] with base framerate: {} fps", xmlFile.getName(), timebase);

            // Traverse video clipitems
            NodeList clipItems = doc.getElementsByTagName("clipitem");
            int clipCounter = 0;

            for (int i = 0; i < clipItems.getLength(); i++) {
                Element clipElem = (Element) clipItems.item(i);

                // Check if this clipitem is under a video track (or has file/video media)
                Element fileElem = getFirstChildElement(clipElem, "file");
                if (fileElem == null) {
                    continue;
                }

                String pathUrl = getElementText(fileElem, "pathurl");
                String fileName = getElementText(fileElem, "name");
                String clipName = getElementText(clipElem, "name");

                String resolvedPath = resolveFilePath(pathUrl, fileName, xmlFile.getParentFile());
                if (resolvedPath == null || !new File(resolvedPath).exists()) {
                    log.warn("⚠️ Media file for clip [{}] not found at '{}' (skipping)", clipName, resolvedPath);
                    // Still record if resolvedPath exists or as fallback
                    if (resolvedPath == null) continue;
                }

                // Check clip-specific timebase if overridden
                double clipTimebase = timebase;
                Element clipRateElem = getFirstChildElement(clipElem, "rate");
                if (clipRateElem != null) {
                    String tbStr = getElementText(clipRateElem, "timebase");
                    if (!tbStr.isBlank()) {
                        try {
                            clipTimebase = Double.parseDouble(tbStr.trim());
                        } catch (NumberFormatException ignored) {}
                    }
                }

                long inFrame = parseLongSafe(getElementText(clipElem, "in"), 0);
                long outFrame = parseLongSafe(getElementText(clipElem, "out"), 0);
                long startFrame = parseLongSafe(getElementText(clipElem, "start"), 0);
                long endFrame = parseLongSafe(getElementText(clipElem, "end"), 0);

                if (outFrame <= inFrame) {
                    long durationFrames = parseLongSafe(getElementText(clipElem, "duration"), 0);
                    if (durationFrames > 0) {
                        outFrame = inFrame + durationFrames;
                    }
                }

                double sourceInSec = inFrame / clipTimebase;
                double sourceOutSec = outFrame / clipTimebase;
                double durationSec = (outFrame - inFrame) / clipTimebase;
                double timelineStartSec = startFrame / clipTimebase;
                double timelineEndSec = endFrame / clipTimebase;

                if (durationSec <= 0) {
                    continue;
                }

                XmlTimelineClip clip = new XmlTimelineClip(
                        clipCounter++,
                        clipName.isBlank() ? fileName : clipName,
                        resolvedPath,
                        timelineStartSec,
                        timelineEndSec,
                        sourceInSec,
                        sourceOutSec,
                        durationSec
                );

                clips.add(clip);
            }

            // Sort clips by timeline start timestamp
            clips.sort(Comparator.comparingDouble(XmlTimelineClip::getTimelineStartSec));

            // Re-index clips in chronological timeline order
            for (int idx = 0; idx < clips.size(); idx++) {
                clips.get(idx).setClipIndex(idx);
            }

            log.info("✅ Successfully parsed {} timeline cuts from XML [{}]", clips.size(), xmlFile.getName());

        } catch (Exception e) {
            log.error("Failed to parse XML timeline file {}: {}", xmlFilePath, e.getMessage(), e);
        }

        return clips;
    }

    private String resolveFilePath(String pathUrl, String fileName, File baseDir) {
        if (pathUrl != null && !pathUrl.isBlank()) {
            try {
                String decoded = URLDecoder.decode(pathUrl, StandardCharsets.UTF_8);
                if (decoded.startsWith("file://localhost/")) {
                    decoded = decoded.substring("file://localhost".length());
                } else if (decoded.startsWith("file:///")) {
                    decoded = decoded.substring("file://".length());
                } else if (decoded.startsWith("file:/")) {
                    decoded = decoded.substring("file:".length());
                }

                File f = new File(decoded);
                if (f.exists()) {
                    return f.getAbsolutePath();
                }

                // If absolute path does not exist, try resolving filename relative to XML base dir
                if (baseDir != null && f.getName() != null) {
                    File relativeFile = new File(baseDir, f.getName());
                    if (relativeFile.exists()) {
                        return relativeFile.getAbsolutePath();
                    }
                }
                return decoded;
            } catch (Exception e) {
                log.warn("Error decoding pathurl '{}': {}", pathUrl, e.getMessage());
            }
        }

        if (fileName != null && !fileName.isBlank() && baseDir != null) {
            File relativeFile = new File(baseDir, fileName);
            if (relativeFile.exists()) {
                return relativeFile.getAbsolutePath();
            }
        }

        return null;
    }

    private Element getFirstChildElement(Element parent, String tagName) {
        NodeList list = parent.getElementsByTagName(tagName);
        if (list.getLength() > 0) {
            return (Element) list.item(0);
        }
        return null;
    }

    private String getElementText(Element parent, String tagName) {
        Element child = getFirstChildElement(parent, tagName);
        if (child != null) {
            return child.getTextContent().trim();
        }
        return "";
    }

    private long parseLongSafe(String str, long defaultVal) {
        if (str == null || str.isBlank()) return defaultVal;
        try {
            return Long.parseLong(str.trim());
        } catch (NumberFormatException e) {
            return defaultVal;
        }
    }
}
