package com.renderfarm.orchestrator.service;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

import java.io.BufferedReader;
import java.io.File;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

@Service
public class StitcherService {

    private static final Logger log = LoggerFactory.getLogger(StitcherService.class);

    public boolean extractAudio(String sourcePath, String audioOutputPath) {
        try {
            File parent = new File(audioOutputPath).getParentFile();
            if (parent != null) {
                parent.mkdirs();
            }

            ProcessBuilder pb = new ProcessBuilder(
                    "ffmpeg", "-y",
                    "-i", sourcePath,
                    "-vn",
                    "-c:a", "copy",
                    audioOutputPath
            );
            Process process = pb.start();
            int exit = process.waitFor();
            return exit == 0;
        } catch (Exception e) {
            log.error("Failed to extract audio from {}: {}", sourcePath, e.getMessage());
            return false;
        }
    }

    public boolean stitchChunks(List<String> chunkPaths, String audioPath, String finalOutputPath) {
        Path manifestPath = null;
        try {
            File parent = new File(finalOutputPath).getParentFile();
            if (parent != null) {
                parent.mkdirs();
            }

            manifestPath = Files.createTempFile("concat_manifest_", ".txt");
            StringBuilder sb = new StringBuilder();
            for (String chunk : chunkPaths) {
                sb.append("file '").append(new File(chunk).getAbsolutePath()).append("'\n");
            }
            Files.writeString(manifestPath, sb.toString(), StandardCharsets.UTF_8);

            List<String> command = new ArrayList<>();
            command.add("ffmpeg");
            command.add("-y");
            command.add("-f");
            command.add("concat");
            command.add("-safe");
            command.add("0");
            command.add("-i");
            command.add(manifestPath.toAbsolutePath().toString());

            boolean hasAudio = audioPath != null && new File(audioPath).exists() && new File(audioPath).length() > 0;
            if (hasAudio) {
                command.add("-i");
                command.add(audioPath);
            }

            command.add("-c:v");
            command.add("copy");

            if (hasAudio) {
                command.add("-c:a");
                command.add("copy");
            }

            command.add("-movflags");
            command.add("+faststart");
            command.add(finalOutputPath);

            ProcessBuilder pb = new ProcessBuilder(command);
            Process process = pb.start();
            int exit = process.waitFor();

            if (exit != 0) {
                log.error("ffmpeg concat exited with code {}", exit);
                return false;
            }
            return true;
        } catch (Exception e) {
            log.error("Failed to stitch chunks: {}", e.getMessage());
            return false;
        } finally {
            if (manifestPath != null) {
                try {
                    Files.deleteIfExists(manifestPath);
                } catch (Exception ignored) {}
            }
        }
    }

    public long probeFrameCount(String filePath) {
        try {
            ProcessBuilder pb = new ProcessBuilder(
                    "ffprobe",
                    "-v", "error",
                    "-select_streams", "v:0",
                    "-count_frames",
                    "-show_entries", "stream=nb_read_frames",
                    "-of", "default=noprint_wrappers=1:nokey=1",
                    filePath
            );
            Process process = pb.start();
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8))) {
                String line = reader.readLine();
                if (line != null) {
                    return Long.parseLong(line.trim());
                }
            }
            process.waitFor();
        } catch (Exception e) {
            log.error("Failed to probe frame count for {}: {}", filePath, e.getMessage());
        }
        return -1;
    }
}
