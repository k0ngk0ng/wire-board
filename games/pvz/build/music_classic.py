"""Render original MO3 scene stems with sample-exact intro/loop metadata."""
import argparse
import ctypes as C
import hashlib
import json
from pathlib import Path
import subprocess

RATE = 44100
LIBRARY_SHA256 = "e81fb7b4d0009ba6343fbfcd840620704dcb840686d405b9734cd37150d31974"
BASS_FLAGS = 0x228204  # DECODE | PRESCAN | original POSRESET | RAMP | LOOP


def music_definitions():
    all_channels = list(range(30))
    result = {}
    for key, title, order in (("menu", "Crazy Dave Main Theme", 152), ("day", "Grasswalk", 0),
                              ("night", "Moongrains", 48), ("pool", "Watery Graves", 94),
                              ("fog", "Rigor Mormist", 125), ("roof", "Graze the Roof", 184),
                              ("boss", "Brainiac Maniac", 158), ("chooseSeeds", "Choose Your Seeds", 122)):
        result[key] = {"title": title, "startOrder": order, "channels": {"main": list(all_channels)}}
    for key, main, drums, hats in (("day", range(24), range(24, 27), [27]),
                                   ("pool", range(18), range(18, 29), list(range(18, 25)) + [29]),
                                   ("fog", range(16), range(16, 23), [23]),
                                   ("roof", range(18), range(18, 21), [21])):
        result[key]["channels"] = {"main": list(main), "drums": list(drums), "hihats": list(hats)}
        result[key]["drumsInitiallyMuted"] = True
    result["night"].update(drumsOrder=92, drumsInitiallyMuted=False)
    result["night"]["channels"]["drums"] = list(all_channels)
    result["nightBurstEven"] = {"title": "Moongrains — original even-order burst", "startOrder": 76, "channels": {"main": list(all_channels)}, "optional": True}
    result["nightBurstOdd"] = {"title": "Moongrains — original odd-order burst", "startOrder": 77, "channels": {"main": list(all_channels)}, "optional": True}
    return result


def loop_seconds(start, end, sample_rate):
    if sample_rate <= 0 or start < 0 or end <= start:
        raise ValueError("Invalid source PCM intro/loop interval")
    return start / sample_rate, end / sample_rate


def intentional_silence(scene, role, order, pcm):
    # The original nighttime drum handle waits at silent order 92 until the
    # burst controller jumps it to 76/77. This is an observed source cue.
    return scene == "night" and role == "drums" and order == 92 and bool(pcm) and not any(pcm)


class ChannelInfo(C.Structure):
    _fields_ = [(name, C.c_uint32) for name in ("freq", "chans", "flags", "ctype", "origres", "plugin", "sample")] + [("filename", C.c_char_p)]


class BassDecoder:
    def __init__(self, library):
        if hashlib.sha256(library.read_bytes()).hexdigest() != LIBRARY_SHA256:
            raise ValueError("Unverified offline BASS decoder library")
        self.lib = C.CDLL(str(library.resolve()))
        uint, uint64, pointer = C.c_uint32, C.c_uint64, C.c_void_p
        signatures = {
            "BASS_GetVersion": (uint, []), "BASS_ErrorGetCode": (C.c_int, []),
            "BASS_Init": (C.c_int, [C.c_int, uint, uint, pointer, pointer]), "BASS_Free": (C.c_int, []),
            "BASS_MusicLoad": (uint, [uint, pointer, uint64, uint, uint, uint]), "BASS_MusicFree": (C.c_int, [uint]),
            "BASS_ChannelGetInfo": (C.c_int, [uint, C.POINTER(ChannelInfo)]),
            "BASS_ChannelGetData": (uint, [uint, pointer, uint]), "BASS_ChannelGetLength": (uint64, [uint, uint]),
            "BASS_ChannelSetAttribute": (C.c_int, [uint, uint, C.c_float]),
            "BASS_ChannelSetPosition": (C.c_int, [uint, uint64, uint]),
        }
        for name, (result, arguments) in signatures.items():
            function = getattr(self.lib, name)
            function.restype, function.argtypes = result, arguments
        if not self.lib.BASS_Init(0, RATE, 0, None, None):
            raise RuntimeError(f"Offline decoder init failed: {self.lib.BASS_ErrorGetCode()}")

    def close(self):
        self.lib.BASS_Free()

    def render(self, path, order, channels, frames, block_frames):
        if not 0 <= order < 236 or not channels or any(not 0 <= value < 30 for value in channels):
            raise ValueError("Invalid original music order/channel mask")
        if not 0 < frames <= RATE * 400:
            raise ValueError("Unbounded music render")
        encoded = str(path.resolve()).encode()
        handle = self.lib.BASS_MusicLoad(0, C.cast(C.c_char_p(encoded), C.c_void_p), 0, 0, BASS_FLAGS, RATE)
        if not handle:
            raise RuntimeError(f"MO3 load failed: {self.lib.BASS_ErrorGetCode()}")
        try:
            info = ChannelInfo()
            if not self.lib.BASS_ChannelGetInfo(handle, C.byref(info)) or (info.freq, info.chans) != (RATE, 2):
                raise RuntimeError("Unexpected decoder PCM format")
            for channel in range(30):
                if not self.lib.BASS_ChannelSetAttribute(handle, 0x200 + channel, 1.0 if channel in channels else 0.0):
                    raise RuntimeError("Cannot restore original channel mute preset")
            if not self.lib.BASS_ChannelSetPosition(handle, order, 1):
                raise RuntimeError("Cannot seek to original order")
            remaining = frames * 4
            chunks = []
            while remaining:
                length = min(remaining, block_frames * 4)
                buffer = C.create_string_buffer(length)
                size = self.lib.BASS_ChannelGetData(handle, buffer, length)
                if size != length:
                    raise RuntimeError(f"Truncated PCM render: {size}/{length}")
                chunks.append(buffer.raw[:size])
                remaining -= size
            return b"".join(chunks)
        finally:
            self.lib.BASS_MusicFree(handle)


def render_assets(source, output, library, loop_path, cache, report_path, only=None):
    loop_data = json.loads(loop_path.read_text())
    if loop_data["librarySHA256"] != LIBRARY_SHA256 or loop_data["nativeBASSflags"] != BASS_FLAGS:
        raise ValueError("Loop measurement and decoder profile differ")
    for name, data in loop_data["sources"].items():
        if hashlib.sha256((source / name).read_bytes()).hexdigest() != data["sha256"]:
            raise ValueError("Original MO3 bytes changed")
    music_folder = output / "music"
    music_folder.mkdir(exist_ok=True)
    cache.mkdir(parents=True, exist_ok=True)
    manifest = json.loads((output / "manifest.json").read_text())
    active_path = loop_path.with_name("night-active-loops.json")
    active = json.loads(active_path.read_text())["records"]
    for key, order in (("nightBurstEven", 76), ("nightBurstOdd", 77)):
        loop_data["themes"][key] = {"main": dict(active[str(order)], enabledChannels=list(range(30)))}
    music = manifest.get("music", {})
    prior_records = json.loads(report_path.read_text()).get("tracks", []) if report_path.exists() else []
    records = {track["src"]: track for track in prior_records}
    decoder = BassDecoder(library)
    try:
        for scene, definition in music_definitions().items():
            if only and scene not in only:
                continue
            tracks = []
            for role, enabled in definition["channels"].items():
                scan_role = "hihat" if role == "hihats" else role
                measured = loop_data["themes"][scene][scan_role]
                if not measured["verifiedExactBoundaryAcrossBlockSizes"]:
                    raise ValueError("Unverified source loop boundary")
                order = definition["drumsOrder"] if scene == "night" and role == "drums" else definition["startOrder"]
                if order != measured["startOrder"] or enabled != measured["enabledChannels"]:
                    raise ValueError("REA channel/order preset and measurement differ")
                path = source / measured["sourceFile"]
                end, intro = measured["firstCyclePCMFrames"], measured["introPCMFrames"]
                begin_seconds, end_seconds = loop_seconds(intro, end, RATE)
                pcm = decoder.render(path, order, enabled, end, 65536)
                repeat = decoder.render(path, order, enabled, end, 1024)
                silent_cue = intentional_silence(scene, role, order, pcm)
                if pcm != repeat or not any(pcm) and not silent_cue:
                    raise ValueError(f"Non-repeatable or silent original music: {scene}/{role}")
                pcm_path = cache / (scene + "-" + role + ".s16le")
                pcm_path.write_bytes(pcm)
                relative = "music/" + scene.lower() + "-" + role + ".flac"
                target = output / relative
                subprocess.run(["ffmpeg", "-v", "error", "-y", "-f", "s16le", "-ar", str(RATE), "-ac", "2", "-i", str(pcm_path),
                                "-c:a", "flac", "-compression_level", "8", str(target)], check=True)
                decoded = subprocess.check_output(["ffmpeg", "-v", "error", "-i", str(target), "-f", "s16le", "-"])
                if decoded != pcm:
                    raise ValueError(f"Lossy music conversion: {scene}/{role}")
                body = target.read_bytes()
                initially_muted = role != "main" and definition.get("drumsInitiallyMuted", True)
                track = {"key": scene.lower() + "-" + role, "role": role, "src": relative, "loopStartSeconds": begin_seconds,
                         "loopEndSeconds": end_seconds, "durationSeconds": end_seconds, "sampleRate": RATE, "channels": 2, "bitsPerSample": 16,
                         "pcmFrames": end, "introPCMFrames": intro, "loopPCMFrames": end - intro,
                         "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest(), "pcm16leSHA256": hashlib.sha256(pcm).hexdigest(),
                         "sourceFile": measured["sourceFile"], "sourceSHA256": measured["sourceSHA256"], "sourceOrder": order,
                         "loopOrder": measured["loopOrder"], "loopRow": measured["loopRow"], "enabledChannels": enabled,
                         "initiallyMuted": initially_muted, "required": role == "main" and not definition.get("optional", False)}
                if silent_cue:
                    track.update(intentionalSilence=True, sourceCue="night-drums-off-order-92", activeBurstOrders=[76, 77])
                tracks.append(track)
                records[relative] = dict(track, scene=scene, repeatableAcrossPCMBlockSizes=True, losslessRoundTrip=True)
                print(scene, role, "frames", end, "bytes", len(body), flush=True)
            music[scene] = {"title": definition["title"], "startOrder": definition["startOrder"], "tracks": tracks,
                            "mixingScope": "original scene stems and initial mute preset; not frame-equivalent dynamic mixing"}
    finally:
        decoder.close()
    music["roofnight"] = music["boss"]
    for value in music.values():
        for track in value["tracks"]:
            if track.get("initiallyMuted") or track.get("intentionalSilence"):
                track["required"] = False
    music["night"]["variants"] = {"even": music["nightBurstEven"], "odd": music["nightBurstOdd"]}
    for value in music.values():
        for track in value["tracks"]:
            if track["src"] in records:
                records[track["src"]].update(track)
    manifest["music"] = music
    for scene in manifest["maps"]:
        # Music is listed separately so the game can show real scene-load progress.
        manifest["music"][scene]["resources"] = [track["src"] for track in music[scene]["tracks"] if track["required"]]
        manifest["music"][scene]["optionalResources"] = [track["src"] for track in music[scene]["tracks"] if not track["required"]]
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    report_path.write_text(json.dumps({"status": "pass", "decoder": {"librarySHA256": LIBRARY_SHA256, "version": "BASS 2.4.18.3", "flags": hex(BASS_FLAGS), "device": 0},
                                       "loopBoundarySource": str(loop_path), "sourceMusicSetupEvidence": "ev_8344638e4d53ac63b893680a4fc1e0c1c22fcfc46f258e359d3c13daaf293c8d",
                                       "sourceMusicPlayEvidence": "ev_6e6d9ef1435e122ae803c42485febc4dce09ca0747b9ea1f5a8ac3f0f6c8199a", "tracks": list(records.values())}, indent=2) + "\n")
    files = [{"path": str(path.relative_to(output)), "bytes": path.stat().st_size, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
             for path in sorted(output.rglob("*")) if path.is_file()]
    (output.parents[2] / "manifest-v2.json").write_text(json.dumps({"version": "classic-v2", "files": files}, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=Path(".local/pvz-research/extracted/sounds"))
    parser.add_argument("--output", type=Path, default=Path("games/pvz/assets/public/pvz/classic-v2"))
    parser.add_argument("--library", type=Path, default=Path(".local/pvz-music/tools/bass24-osx/libbass.dylib"))
    parser.add_argument("--loops", type=Path, default=Path("docs/research/pvz/evidence/music/theme-loops-v2.json"))
    parser.add_argument("--cache", type=Path, default=Path(".local/pvz-music/rendered"))
    parser.add_argument("--report", type=Path, default=Path("docs/research/pvz/evidence/music-render.json"))
    parser.add_argument("--only", nargs="+", help="Render selected new themes while retaining prior verified assets")
    args = parser.parse_args()
    render_assets(args.source, args.output, args.library, args.loops, args.cache, args.report, args.only)


if __name__ == "__main__":
    main()
