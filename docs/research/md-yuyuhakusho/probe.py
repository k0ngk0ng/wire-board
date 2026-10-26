#!/usr/bin/env python3
"""Headless libretro research replay; no ROM or extracted game assets included."""
import argparse
import ctypes as C
import hashlib
import json
from pathlib import Path
import platform
import struct
import zlib


class Info(C.Structure):
    _fields_ = [("name", C.c_char_p), ("version", C.c_char_p),
                ("extensions", C.c_char_p), ("fullpath", C.c_bool),
                ("block_extract", C.c_bool)]


class Game(C.Structure):
    _fields_ = [("path", C.c_char_p), ("data", C.c_void_p),
                ("size", C.c_size_t), ("meta", C.c_char_p)]


class Geometry(C.Structure):
    _fields_ = [("width", C.c_uint), ("height", C.c_uint),
                ("max_width", C.c_uint), ("max_height", C.c_uint),
                ("aspect", C.c_float)]


class Timing(C.Structure):
    _fields_ = [("fps", C.c_double), ("sample_rate", C.c_double)]


class AV(C.Structure):
    _fields_ = [("geometry", Geometry), ("timing", Timing)]


def digest(data):
    return hashlib.sha256(data).hexdigest()


def png(path, raw, width, height, pixel_format):
    rgb = bytearray()
    step = 4 if pixel_format == 1 else 2
    for y in range(height):
        rgb.append(0)
        for x in range(width):
            n = int.from_bytes(raw[(y * width + x) * step: (y * width + x + 1) * step], "little")
            if pixel_format == 1:
                color = ((n >> 16) & 255, (n >> 8) & 255, n & 255)
            elif pixel_format == 2:
                color = (((n >> 11) & 31) * 255 // 31, ((n >> 5) & 63) * 255 // 63, (n & 31) * 255 // 31)
            else:
                color = (((n >> 10) & 31) * 255 // 31, ((n >> 5) & 31) * 255 // 31, (n & 31) * 255 // 31)
            rgb.extend(color)
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    path.write_bytes(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
                     + chunk(b"IDAT", zlib.compress(rgb)) + chunk(b"IEND", b""))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--core", required=True, type=Path)
    ap.add_argument("--rom", required=True, type=Path)
    ap.add_argument("--inputs", required=True, type=Path)
    ap.add_argument("--out", required=True, type=Path)
    ap.add_argument("--frames", type=int, default=1800)
    ap.add_argument("--checkpoint", type=int, default=1200)
    ap.add_argument("--shots", default="180,360,600,900,1200,1500,1800")
    ap.add_argument("--teamplayer", action="store_true")
    args = ap.parse_args()
    root = Path(__file__).resolve().parents[3]
    out = args.out.resolve()
    if not out.is_relative_to(root):
        ap.error("output must remain inside this repository")
    if not 0 < args.checkpoint < args.frames:
        ap.error("checkpoint must be between 0 and frames")
    out.mkdir(parents=True, exist_ok=True)
    rom = args.rom.read_bytes()
    events = json.loads(args.inputs.read_text())
    shots = {int(x) for x in args.shots.split(",") if x}
    lib = C.CDLL(str(args.core.resolve()))
    def api(name, result, *parameters):
        f = getattr(lib, name)
        f.restype, f.argtypes = result, list(parameters)
        return f
    env_type = C.CFUNCTYPE(C.c_bool, C.c_uint, C.c_void_p)
    video_type = C.CFUNCTYPE(None, C.c_void_p, C.c_uint, C.c_uint, C.c_size_t)
    audio_type = C.CFUNCTYPE(None, C.c_int16, C.c_int16)
    batch_type = C.CFUNCTYPE(C.c_size_t, C.c_void_p, C.c_size_t)
    poll_type = C.CFUNCTYPE(None)
    input_type = C.CFUNCTYPE(C.c_int16, C.c_uint, C.c_uint, C.c_uint, C.c_uint)
    state = {"frame": 0, "format": 0, "image": b"", "width": 0, "height": 0, "video_calls": 0}
    directory = str(out).encode()
    @env_type
    def environment(command, data):
        if command in (9, 31, 30):  # system, save and assets directories
            C.cast(data, C.POINTER(C.c_char_p))[0] = directory
            return True
        if command == 10:  # SET_PIXEL_FORMAT
            fmt = C.cast(data, C.POINTER(C.c_int))[0]
            if fmt not in (0, 1, 2):
                return False
            state["format"] = fmt
            return True
        if command == 3:  # GET_CAN_DUPE
            C.cast(data, C.POINTER(C.c_bool))[0] = True
            return True
        if command == 17:  # GET_VARIABLE_UPDATE
            C.cast(data, C.POINTER(C.c_bool))[0] = False
            return True
        if command == (47 | 0x10000):  # GET_AUDIO_VIDEO_ENABLE, deterministic fast states
            C.cast(data, C.POINTER(C.c_int))[0] = 7
            return True
        if command == 15:  # GET_VARIABLE, use core defaults
            C.cast(data, C.POINTER(C.c_void_p))[1] = None
            return False
        return False
    @video_type
    def video(data, width, height, pitch):
        if data:
            row = width * (4 if state["format"] == 1 else 2)
            state["image"] = b"".join(C.string_at(data + y * pitch, row) for y in range(height))
            state["width"], state["height"] = width, height
            state["video_calls"] += 1
    @input_type
    def input_state(port, device, index, button):
        if device != 1 or index != 0:
            return 0
        mask = 0
        for event in events:
            if event["start"] <= state["frame"] <= event["end"] and event.get("port", 0) == port:
                for key in event["buttons"]:
                    mask |= 1 << key
        return mask if button == 256 else int(bool(mask & (1 << button)))
    audio = audio_type(lambda left, right: None)
    batch = batch_type(lambda data, frames: frames)
    poll = poll_type(lambda: None)
    for name, typ, cb in [("environment", env_type, environment), ("video_refresh", video_type, video),
                           ("audio_sample", audio_type, audio), ("audio_sample_batch", batch_type, batch),
                           ("input_poll", poll_type, poll), ("input_state", input_type, input_state)]:
        api("retro_set_" + name, None, typ)(cb)
    api_version = api("retro_api_version", C.c_uint)()
    assert api_version == 1, api_version
    info = Info()
    api("retro_get_system_info", None, C.POINTER(Info))(C.byref(info))
    api("retro_init", None)()
    data = C.create_string_buffer(rom)
    game = Game(str(args.rom.resolve()).encode(), C.cast(data, C.c_void_p), len(rom), None)
    assert api("retro_load_game", C.c_bool, C.POINTER(Game))(C.byref(game)), "ROM load failed"
    device = 1 + ((6 + 1) << 8) if args.teamplayer else 1 + ((1 + 1) << 8)
    controller = api("retro_set_controller_port_device", None, C.c_uint, C.c_uint)
    controller(0, device)
    controller(1, 1 + ((1 + 1) << 8))
    av = AV()
    api("retro_get_system_av_info", None, C.POINTER(AV))(C.byref(av))
    run = api("retro_run", None)
    ram_size = api("retro_get_memory_size", C.c_size_t, C.c_uint)(2)
    ram_pointer = api("retro_get_memory_data", C.c_void_p, C.c_uint)(2)
    assert ram_size == 65536 and ram_pointer, (ram_size, ram_pointer)
    size = api("retro_serialize_size", C.c_size_t)()
    save = api("retro_serialize", C.c_bool, C.c_void_p, C.c_size_t)
    restore = api("retro_unserialize", C.c_bool, C.c_void_p, C.c_size_t)
    checkpoint = C.create_string_buffer(size)
    records = []
    ram_samples = {}
    def tick(frame):
        state["frame"] = frame
        run()
        return {"frame": frame, "ram": digest(C.string_at(ram_pointer, ram_size)),
                "video": digest(state["image"]), "dimensions": [state["width"], state["height"]]}
    for frame in range(1, args.frames + 1):
        records.append(tick(frame))
        if args.checkpoint < frame <= args.checkpoint + 12:
            ram_samples[frame] = C.string_at(ram_pointer, ram_size)
        if frame == args.checkpoint:
            assert save(checkpoint, size), "serialize failed"
            (out / "checkpoint.bin").write_bytes(checkpoint.raw)
        if frame in shots:
            assert state["image"], "no video output"
            png(out / f"frame-{frame:05d}.png", state["image"], state["width"], state["height"], state["format"])
    assert restore(checkpoint, size), "unserialize failed"
    replay = []
    ram_difference_samples = []
    for frame in range(args.checkpoint + 1, args.frames + 1):
        replay.append(tick(frame))
        if frame in ram_samples:
            actual = C.string_at(ram_pointer, ram_size)
            diffs = [{"offset": hex(i), "expected": a, "actual": b}
                     for i, (a, b) in enumerate(zip(ram_samples[frame], actual)) if a != b]
            if diffs:
                ram_difference_samples.append({"frame": frame, "count": len(diffs), "bytes": diffs[:32]})
    expected = records[args.checkpoint:]
    mismatches = [{"expected": a, "actual": b} for a, b in zip(expected, replay) if a != b]
    report = {"scope": "original-ROM local emulator replay; not reconstructed game acceptance",
              "host": {"system": platform.system(), "machine": platform.machine()},
              "core": {"name": info.name.decode(), "version": info.version.decode(),
                       "sha256": digest(args.core.read_bytes()), "api": api_version},
              "rom_sha256": digest(rom), "inputs_sha256": digest(args.inputs.read_bytes()),
              "frames": args.frames, "checkpoint_frame": args.checkpoint, "replayed_frames": len(replay),
              "fps": av.timing.fps, "audio_sample_rate": av.timing.sample_rate,
              "ram_bytes": ram_size, "save_bytes": size, "pixel_format": state["format"],
              "teamplayer": args.teamplayer, "video_callbacks": state["video_calls"],
              "unique_video_frames": len({x["video"] for x in records}),
              "mismatch_count": len(mismatches), "mismatches": mismatches[:10],
              "ram_mismatch_frames": sum(a["ram"] != b["ram"] for a, b in zip(expected, replay)),
              "video_mismatch_frames": sum(a["video"] != b["video"] for a, b in zip(expected, replay)),
              "ram_difference_samples": ram_difference_samples,
              "pass": not mismatches and state["video_calls"] > 0,
              "limitations": ["No audio-output comparison", "No reconstructed rules or AI verified",
                              "RAM hashes use core-native byte layout; no CPU address interpretation"]}
    (out / "report.json").write_text(json.dumps(report, indent=2) + "\n")
    (out / "frames.json").write_text(json.dumps(records, separators=(",", ":")) + "\n")
    (out / "inputs.json").write_text(args.inputs.read_text())
    api("retro_unload_game", None)()
    api("retro_deinit", None)()
    print(json.dumps(report, indent=2))
    assert report["pass"], "replay diverged; inspect report"


if __name__ == "__main__":
    main()
