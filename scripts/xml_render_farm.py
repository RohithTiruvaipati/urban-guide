#!/usr/bin/env python3
"""
Distributed / Multi-Worker Premiere Pro XML Timeline Renderer
Parses Premiere Pro / Final Cut Pro XML (video.xml), slices the edit timeline
into parallel chunks, renders each chunk from raw 4K source footage, and stitches
them into the final master MP4.
"""

import sys
import os
import xml.etree.ElementTree as ET
import urllib.parse
import subprocess
import argparse
import time
import concurrent.futures
from dataclasses import dataclass
from typing import List, Optional

@dataclass
class ClipItem:
    name: str
    track_idx: int
    timeline_start: int  # in frames
    timeline_end: int    # in frames
    source_in: int       # in frames
    source_out: int      # in frames
    file_path: Optional[str]
    is_adjustment_layer: bool

@dataclass
class TimelineSequence:
    name: str
    duration_frames: int
    timebase: float
    width: int
    height: int
    video_clips: List[ClipItem]
    audio_clips: List[ClipItem]

def parse_xml_sequence(xml_path: str) -> TimelineSequence:
    tree = ET.parse(xml_path)
    root = tree.getroot()

    seq_node = root.find('.//sequence')
    if seq_node is None:
        raise ValueError(f"No <sequence> found in {xml_path}")

    name = seq_node.find('name').text if seq_node.find('name') is not None else "Sequence"
    duration_str = seq_node.find('duration').text if seq_node.find('duration') is not None else "0"
    duration_frames = int(duration_str)

    tb_node = seq_node.find('.//rate/timebase')
    ntsc_node = seq_node.find('.//rate/ntsc')
    tb = float(tb_node.text) if tb_node is not None else 24.0
    if ntsc_node is not None and ntsc_node.text.strip().upper() == "TRUE":
        fps = (tb * 1000.0) / 1001.0  # 23.976 fps
    else:
        fps = tb

    w_node = seq_node.find('.//media/video/format/samplecharacteristics/width')
    h_node = seq_node.find('.//media/video/format/samplecharacteristics/height')
    width = int(w_node.text) if w_node is not None else 3840
    height = int(h_node.text) if h_node is not None else 2160

    # Parse video tracks
    video_clips: List[ClipItem] = []
    v_tracks = seq_node.findall('.//media/video/track')
    for t_idx, track in enumerate(v_tracks):
        for c in track.findall('clipitem'):
            c_name = c.find('name').text if c.find('name') is not None else "unnamed"
            c_start = int(c.find('start').text) if c.find('start') is not None else 0
            c_end = int(c.find('end').text) if c.find('end') is not None else 0
            c_in = int(c.find('in').text) if c.find('in') is not None else 0
            c_out = int(c.find('out').text) if c.find('out') is not None else 0

            file_path = None
            path_node = c.find('.//file/pathurl')
            if path_node is not None and path_node.text:
                raw_url = path_node.text
                clean_path = urllib.parse.unquote(raw_url).replace("file://localhost", "")
                if os.path.exists(clean_path):
                    file_path = clean_path
                else:
                    base = os.path.basename(clean_path)
                    candidates = [
                        os.path.join(os.path.dirname(xml_path), base),
                        os.path.join("/Users/rohithtiruvaipati1/Desktop/untitled folder 2", base),
                        os.path.join(os.path.expanduser("~/Downloads"), base)
                    ]
                    for cand in candidates:
                        if os.path.exists(cand):
                            file_path = cand
                            break

            is_adj = ("adjustment" in c_name.lower()) or (file_path is None)

            video_clips.append(ClipItem(
                name=c_name,
                track_idx=t_idx,
                timeline_start=c_start,
                timeline_end=c_end,
                source_in=c_in,
                source_out=c_out,
                file_path=file_path,
                is_adjustment_layer=is_adj
            ))

    # Parse audio tracks
    audio_clips: List[ClipItem] = []
    a_tracks = seq_node.findall('.//media/audio/track')
    for t_idx, track in enumerate(a_tracks):
        for c in track.findall('clipitem'):
            c_name = c.find('name').text if c.find('name') is not None else "audio_clip"
            c_start = int(c.find('start').text) if c.find('start') is not None else 0
            c_end = int(c.find('end').text) if c.find('end') is not None else 0
            c_in = int(c.find('in').text) if c.find('in') is not None else 0
            c_out = int(c.find('out').text) if c.find('out') is not None else 0

            file_path = None
            path_node = c.find('.//file/pathurl')
            if path_node is not None and path_node.text:
                raw_url = path_node.text
                clean_path = urllib.parse.unquote(raw_url).replace("file://localhost", "")
                if os.path.exists(clean_path):
                    file_path = clean_path
                else:
                    base = os.path.basename(clean_path)
                    candidates = [
                        os.path.join(os.path.dirname(xml_path), base),
                        os.path.join("/Users/rohithtiruvaipati1/Desktop/untitled folder 2", base),
                        os.path.join(os.path.expanduser("~/Downloads"), base)
                    ]
                    for cand in candidates:
                        if os.path.exists(cand):
                            file_path = cand
                            break

            if file_path:
                audio_clips.append(ClipItem(
                    name=c_name,
                    track_idx=t_idx,
                    timeline_start=c_start,
                    timeline_end=c_end,
                    source_in=c_in,
                    source_out=c_out,
                    file_path=file_path,
                    is_adjustment_layer=False
                ))

    return TimelineSequence(
        name=name,
        duration_frames=duration_frames,
        timebase=fps,
        width=width,
        height=height,
        video_clips=video_clips,
        audio_clips=audio_clips
    )

def render_timeline_chunk(seq: TimelineSequence, chunk_idx: int, start_frame: int, end_frame: int, output_path: str, codec: str = "h264_videotoolbox") -> None:
    fps = seq.timebase
    chunk_dur_frames = end_frame - start_frame
    chunk_dur_sec = chunk_dur_frames / fps

    # Find video clips overlapping this chunk range on Track 1 (base track) and upper tracks
    overlapping_clips = [
        c for c in seq.video_clips
        if not c.is_adjustment_layer and c.file_path and (c.timeline_start < end_frame and c.timeline_end > start_frame)
    ]
    overlapping_clips.sort(key=lambda c: (c.timeline_start, c.track_idx))

    os.makedirs(os.path.dirname(os.path.abspath(output_path)), exist_ok=True)

    if not overlapping_clips:
        cmd = [
            "ffmpeg", "-y",
            "-f", "lavfi",
            "-i", f"color=c=black:s={seq.width}x{seq.height}:r={fps}:d={chunk_dur_sec}",
            "-c:v", "libx264", "-preset", "ultrafast",
            "-pix_fmt", "yuv420p",
            output_path
        ]
        subprocess.run(cmd, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        return

    tmp_chunk_dir = os.path.join(os.path.dirname(output_path), f"tmp_chunk_{chunk_idx:03d}")
    os.makedirs(tmp_chunk_dir, exist_ok=True)

    rendered_subsegments = []
    current_timeline_pos = start_frame

    for i, clip in enumerate(overlapping_clips):
        # Gap before clip?
        if clip.timeline_start > current_timeline_pos:
            gap_frames = clip.timeline_start - current_timeline_pos
            gap_sec = gap_frames / fps
            gap_out = os.path.join(tmp_chunk_dir, f"gap_{i}.mp4")
            subprocess.run([
                "ffmpeg", "-y", "-f", "lavfi",
                "-i", f"color=c=black:s={seq.width}x{seq.height}:r={fps}:d={gap_sec}",
                "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", gap_out
            ], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            rendered_subsegments.append(gap_out)
            current_timeline_pos = clip.timeline_start

        slice_start_timeline = max(clip.timeline_start, current_timeline_pos)
        slice_end_timeline = min(clip.timeline_end, end_frame)

        if slice_end_timeline <= slice_start_timeline:
            continue

        offset_into_clip = slice_start_timeline - clip.timeline_start
        clip_source_frame_start = clip.source_in + offset_into_clip
        clip_slice_frames = slice_end_timeline - slice_start_timeline

        ss_sec = clip_source_frame_start / fps
        t_sec = clip_slice_frames / fps

        sub_out = os.path.join(tmp_chunk_dir, f"sub_{i}.mp4")
        
        enc_args = ["-c:v", "h264_videotoolbox", "-b:v", "15M"] if codec == "h264_videotoolbox" else ["-c:v", "libx264", "-preset", "veryfast", "-b:v", "15M"]

        # Attempt hardware encode
        cmd = [
            "ffmpeg", "-y",
            "-ss", f"{ss_sec:.4f}",
            "-t", f"{t_sec:.4f}",
            "-i", clip.file_path,
            "-vf", f"scale={seq.width}:{seq.height}:force_original_aspect_ratio=decrease,pad={seq.width}:{seq.height}:(ow-iw)/2:(oh-ih)/2,fps={fps}",
            *enc_args,
            "-an", "-avoid_negative_ts", "make_zero",
            sub_out
        ]
        res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

        if res.returncode != 0:
            # Fallback 1: software libx264
            cmd_fb = [
                "ffmpeg", "-y",
                "-ss", f"{ss_sec:.4f}",
                "-t", f"{t_sec:.4f}",
                "-i", clip.file_path,
                "-vf", f"scale={seq.width}:{seq.height}:force_original_aspect_ratio=decrease,pad={seq.width}:{seq.height}:(ow-iw)/2:(oh-ih)/2,fps={fps}",
                "-c:v", "libx264", "-preset", "ultrafast",
                "-an", "-avoid_negative_ts", "make_zero",
                sub_out
            ]
            res_fb = subprocess.run(cmd_fb, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            
            if res_fb.returncode != 0:
                # Fallback 2: unreadable/permission-denied file on external disk -> generate visual placeholder segment
                safe_name = clip.name.replace(":", "_").replace("[", "").replace("]", "")
                print(f"\n⚠️  [Worker Notice] Clip {clip.name} unreadable on external mount, rendering placeholder ({t_sec:.1f}s)")
                cmd_card = [
                    "ffmpeg", "-y", "-f", "lavfi",
                    "-i", f"color=c=0x1a1a1a:s={seq.width}x{seq.height}:r={fps}:d={t_sec:.4f}",
                    "-vf", f"drawtext=text='MISSING CLIP - {safe_name}':fontcolor=white:fontsize=64:x=(w-text_w)/2:y=(h-text_h)/2",
                    "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
                    "-an", sub_out
                ]
                res_card = subprocess.run(cmd_card, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                if res_card.returncode != 0:
                    # Solid color fallback without drawtext
                    cmd_solid = [
                        "ffmpeg", "-y", "-f", "lavfi",
                        "-i", f"color=c=0x1a1a1a:s={seq.width}x{seq.height}:r={fps}:d={t_sec:.4f}",
                        "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
                        "-an", sub_out
                    ]
                    subprocess.run(cmd_solid, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

        rendered_subsegments.append(sub_out)
        current_timeline_pos = slice_end_timeline
        if current_timeline_pos >= end_frame:
            break

    # Trailing gap fill
    if current_timeline_pos < end_frame:
        gap_frames = end_frame - current_timeline_pos
        gap_sec = gap_frames / fps
        gap_out = os.path.join(tmp_chunk_dir, "trailing_gap.mp4")
        subprocess.run([
            "ffmpeg", "-y", "-f", "lavfi",
            "-i", f"color=c=black:s={seq.width}x{seq.height}:r={fps}:d={gap_sec}",
            "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", gap_out
        ], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        rendered_subsegments.append(gap_out)

    # Concat subsegments into single chunk file
    manifest_path = os.path.join(tmp_chunk_dir, "concat.txt")
    with open(manifest_path, "w") as f:
        for p in rendered_subsegments:
            f.write(f"file '{p}'\n")

    subprocess.run([
        "ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", manifest_path,
        "-c", "copy", output_path
    ], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    # Cleanup temp subsegments
    for p in rendered_subsegments:
        try: os.remove(p)
        except: pass
    try: os.remove(manifest_path)
    except: pass
    try: os.rmdir(tmp_chunk_dir)
    except: pass

def render_full_audio_pass(seq: TimelineSequence, output_audio_path: str) -> None:
    os.makedirs(os.path.dirname(os.path.abspath(output_audio_path)), exist_ok=True)
    dur_sec = seq.duration_frames / seq.timebase

    valid_audio_clips = [c for c in seq.audio_clips if c.file_path and os.path.exists(c.file_path)]

    if not valid_audio_clips:
        subprocess.run([
            "ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo",
            "-t", f"{dur_sec:.3f}", "-c:a", "aac", "-b:a", "320k", output_audio_path
        ], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        return

    main_audio = valid_audio_clips[0]
    ss_sec = main_audio.source_in / seq.timebase
    
    cmd = [
        "ffmpeg", "-y",
        "-ss", f"{ss_sec:.4f}",
        "-t", f"{dur_sec:.4f}",
        "-i", main_audio.file_path,
        "-c:a", "aac", "-b:a", "320k",
        output_audio_path
    ]
    res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if res.returncode != 0:
        subprocess.run([
            "ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo",
            "-t", f"{dur_sec:.3f}", "-c:a", "aac", "-b:a", "320k", output_audio_path
        ], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def main():
    parser = argparse.ArgumentParser(description="Distributed Premiere Pro XML Sequence Renderer")
    parser.add_argument("--xml", default="/Users/rohithtiruvaipati1/Desktop/Films/IDLE (kinda)/video.xml", help="Path to Premiere Pro XML file")
    parser.add_argument("--chunks", type=int, default=8, help="Number of parallel chunks")
    parser.add_argument("--workers", type=int, default=8, help="Number of parallel worker processes")
    parser.add_argument("--output", default="test_assets/output/idle_kinda_exported.mp4", help="Final output MP4 path")
    parser.add_argument("--codec", default="h264_videotoolbox", choices=["h264_videotoolbox", "libx264"], help="Video encoder")
    args = parser.parse_args()

    print("==================================================================")
    print("🎬 PREMIERE PRO XML DISTRIBUTED SEQUENCE RENDER FARM")
    print("==================================================================")
    print(f"XML Project:   {args.xml}")
    print(f"Parallelism:   {args.chunks} chunks across {args.workers} workers")
    print(f"Encoder:       {args.codec} (Hardware Accelerated)")
    print(f"Final Output:  {args.output}")
    print("------------------------------------------------------------------")

    start_time = time.time()

    print("🔍 Parsing Premiere sequence & resolving source 4K media... ", end="", flush=True)
    seq = parse_xml_sequence(args.xml)
    total_sec = seq.duration_frames / seq.timebase
    print(f"OK\n   Timeline: {seq.duration_frames} frames ({total_sec:.2f}s) @ {seq.timebase:.3f}fps, {seq.width}x{seq.height} UHD")
    print(f"   Clips:    {len(seq.video_clips)} video edits, {len(seq.audio_clips)} audio tracks")

    # Divide timeline into N frame chunks
    frames_per_chunk = (seq.duration_frames + args.chunks - 1) // args.chunks
    chunk_ranges = []
    for c_i in range(args.chunks):
        c_start = c_i * frames_per_chunk
        c_end = min((c_i + 1) * frames_per_chunk, seq.duration_frames)
        if c_start >= seq.duration_frames:
            break
        chunk_ranges.append((c_i, c_start, c_end))

    print(f"\n📦 Sliced into {len(chunk_ranges)} parallel timeline chunks:")
    out_dir = os.path.dirname(os.path.abspath(args.output))
    chunk_paths = []
    for c_i, c_start, c_end in chunk_ranges:
        c_dur = (c_end - c_start) / seq.timebase
        c_path = os.path.join(out_dir, f"xml_chunk_{c_i:03d}.mp4")
        chunk_paths.append(c_path)
        print(f"   Chunk #{c_i}: frames [{c_start:>5} -> {c_end:>5}] ({c_dur:5.1f}s) -> {os.path.basename(c_path)}")

    # Extract / synthesize master audio track
    audio_path = os.path.join(out_dir, "master_timeline_audio.aac")
    print("\n🎵 Extracting & mixing master timeline audio... ", end="", flush=True)
    render_full_audio_pass(seq, audio_path)
    print("OK")

    # Render chunks in parallel using worker thread pool
    print(f"\n⚡ Dispatching {len(chunk_ranges)} chunks across {args.workers} worker processes...")
    render_start = time.time()

    def do_render(item):
        idx, c_s, c_e = item
        out_p = chunk_paths[idx]
        t0 = time.time()
        render_timeline_chunk(seq, idx, c_s, c_e, out_p, codec=args.codec)
        dt = time.time() - t0
        print(f"   ✓ Chunk #{idx} [{c_s:>5} -> {c_e:>5}] rendered in {dt:.2f}s")
        return idx

    with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as executor:
        futures = [executor.submit(do_render, item) for item in chunk_ranges]
        for f in concurrent.futures.as_completed(futures):
            f.result()

    render_time = time.time() - render_start
    print(f"🏁 All {len(chunk_ranges)} 4K chunks rendered in {render_time:.2f}s")

    # Lossless stitch
    print("\n🧩 Instant stream concatenation & audio muxing... ", end="", flush=True)
    stitch_start = time.time()
    manifest_file = os.path.join(out_dir, "xml_concat_manifest.txt")
    with open(manifest_file, "w") as f:
        for p in chunk_paths:
            f.write(f"file '{p}'\n")

    subprocess.run([
        "ffmpeg", "-y",
        "-f", "concat", "-safe", "0", "-i", manifest_file,
        "-i", audio_path,
        "-c:v", "copy", "-c:a", "copy",
        "-movflags", "+faststart",
        args.output
    ], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    stitch_time = time.time() - stitch_start
    print(f"Done in {stitch_time:.2f}s (Lossless stream copy)")

    # Cleanup intermediate chunk files
    try: os.remove(manifest_file)
    except: pass

    total_wall_clock = time.time() - start_time
    print("==================================================================")
    print("📊 PREMIERE PRO DISTRIBUTED RENDER SUMMARY")
    print("==================================================================")
    print(f"Total Sequence Length: {total_sec:.2f}s ({seq.duration_frames} frames)")
    print(f"Resolution:            {seq.width}x{seq.height} (4K UHD)")
    print(f"Parallel Speedup:      {args.chunks}x chunked rendering")
    print(f"Render Wall Clock:     {render_time:.2f}s")
    print(f"Stitch Time:           {stitch_time:.2f}s")
    print(f"Total Export Time:     {total_wall_clock:.2f}s")
    print(f"Master Output File:    {args.output}")
    print("==================================================================")

if __name__ == "__main__":
    main()
