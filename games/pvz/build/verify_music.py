"""Repeat the final FLAC/PCM/intro/loop verification without loading BASS."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess


def read_streaminfo(body):
    """Read the mandatory first FLAC metadata block without an external probe.

    Published stems have a known, nonzero sample count. Full decoded frame
    counts and PCM hashes are checked independently below with FFmpeg.
    """
    if len(body) < 42 or body[:4] != b"fLaC" or body[4] & 0x7f != 0 or int.from_bytes(body[5:8], "big") != 34:
        raise ValueError("Invalid FLAC STREAMINFO framing")
    minimum, maximum = int.from_bytes(body[8:10], "big"), int.from_bytes(body[10:12], "big")
    packed = int.from_bytes(body[18:26], "big")
    info = {"sampleRate": packed >> 44, "channels": ((packed >> 41) & 7) + 1,
            "bitsPerSample": ((packed >> 36) & 31) + 1, "totalSamples": packed & ((1 << 36) - 1)}
    if minimum < 16 or maximum < minimum or not 1 <= info["sampleRate"] <= 655350 or not 4 <= info["bitsPerSample"] <= 32 or info["totalSamples"] == 0:
        raise ValueError("Invalid FLAC STREAMINFO values")
    return info


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("games/pvz/assets/public/pvz/classic-v2"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    manifest = json.loads((args.root / "manifest.json").read_text())
    music = manifest["music"]
    assert {"menu", "day", "night", "pool", "fog", "roof", "roofnight", "boss", "chooseSeeds"} <= music.keys()
    assert music["roofnight"]["startOrder"] == 158
    tracks = {track["src"]: track for value in music.values() for track in value["tracks"]}
    records = []
    for name, track in sorted(tracks.items()):
        path = args.root / name
        body = path.read_bytes()
        assert path.suffix == ".flac" and body[:4] == b"fLaC", name
        assert len(body) == track["bytes"] and hashlib.sha256(body).hexdigest() == track["sha256"], name
        info = read_streaminfo(body)
        assert info == {"sampleRate": track["sampleRate"], "channels": track["channels"],
                        "bitsPerSample": track["bitsPerSample"], "totalSamples": track["pcmFrames"]}, name
        assert info["sampleRate"] == 44100 and info["channels"] == 2 and info["bitsPerSample"] == 16, name
        pcm = subprocess.check_output(["ffmpeg", "-v", "error", "-i", str(path), "-f", "s16le", "-"])
        assert len(pcm) == track["pcmFrames"] * 4 and hashlib.sha256(pcm).hexdigest() == track["pcm16leSHA256"], name
        assert 0 <= track["introPCMFrames"] < track["pcmFrames"], name
        assert track["loopStartSeconds"] == track["introPCMFrames"] / 44100, name
        assert track["loopEndSeconds"] == track["pcmFrames"] / 44100, name
        assert track["loopPCMFrames"] == track["pcmFrames"] - track["introPCMFrames"], name
        if track.get("intentionalSilence"):
            assert track["sourceOrder"] == 92 and track["role"] == "drums" and not any(pcm), name
        else:
            assert any(pcm), name
        records.append({"src": name, "sha256": track["sha256"], "pcm16leSHA256": track["pcm16leSHA256"],
                        "frames": track["pcmFrames"], "introFrames": track["introPCMFrames"], "loopFrames": track["loopPCMFrames"],
                        "losslessRoundTrip": True, "intentionalSilence": track.get("intentionalSilence", False)})
    args.output.write_text(json.dumps({"status": "pass", "themes": list(music), "tracks": records,
                                      "scope": "original theme stems and sample-exact loop segments; not frame-equivalent adaptive mixing"}, indent=2) + "\n")
    print(f"Verified {len(records)} original FLAC stems, exact PCM digests and intro/loop metadata")


if __name__ == "__main__":
    main()
