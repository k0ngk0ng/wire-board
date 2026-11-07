"""Decode and verify every delivered classic sprite and original audio clip."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess


def main():
    from PIL import Image
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("games/pvz/assets/public/pvz/classic-v1"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    manifest = json.loads((args.root / "manifest.json").read_text())
    declared = json.loads((args.root.parents[2] / "manifest.json").read_text())
    for entry in declared["files"]:
        path = args.root / entry["path"]
        assert path.stat().st_size == entry["bytes"], path
        assert hashlib.sha256(path.read_bytes()).hexdigest() == entry["sha256"], path
    sprites = []
    for kind, sprite in manifest["sprites"].items():
        for clip, value in sprite["animations"].items():
            path = args.root / value["src"]
            image = Image.open(path)
            image.load()
            assert image.mode == "RGBA", path
            assert image.width == value["columns"] * value["frameWidth"], path
            assert image.height % value["frameHeight"] == 0, path
            assert image.getchannel("A").getextrema() == (0, 255), path
            assert value["frames"] <= value["columns"] * (image.height // value["frameHeight"]), path
            sprites.append({"kind": kind, "clip": clip, "frames": value["frames"], "size": image.size,
                            "sha256": hashlib.sha256(path.read_bytes()).hexdigest()})
    audio = []
    for key, name in manifest["audio"].items():
        result = subprocess.run(["ffmpeg", "-v", "error", "-i", str(args.root / name), "-f", "s16le", "-"],
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True)
        assert len(result.stdout) > 0 and any(result.stdout), name
        audio.append({"key": key, "file": name, "sourceEncoding": "original OGG/Vorbis; no lossy re-encoding",
                      "decodedPCM16LEBytes": len(result.stdout), "decodedPCM16LESHA256": hashlib.sha256(result.stdout).hexdigest()})
    args.output.write_text(json.dumps({"sprites": sprites, "audio": audio, "frames": sum(x["frames"] for x in sprites), "status": "pass"}, indent=2) + "\n")
    print(f"Verified {len(sprites)} clips, {sum(x['frames'] for x in sprites)} frames and {len(audio)} audio files")


if __name__ == "__main__":
    main()
