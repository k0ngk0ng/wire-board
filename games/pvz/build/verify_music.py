"""Repeat the final FLAC/PCM/intro/loop verification without loading BASS."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess


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
        info = json.loads(subprocess.check_output(["ffprobe", "-v", "error", "-show_streams", "-of", "json", str(path)]))["streams"][0]
        assert info["codec_name"] == "flac" and int(info["sample_rate"]) == track["sampleRate"] == 44100, name
        assert info["channels"] == track["channels"] == 2 and int(info["bits_per_raw_sample"]) == track["bitsPerSample"] == 16, name
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
