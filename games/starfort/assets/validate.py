#!/usr/bin/env python3
"""Validate delivered media independently of the SVG/audio generator."""
import argparse
import array
import hashlib
import json
from pathlib import Path
import wave
import xml.etree.ElementTree as ET

from PIL import Image, ImageChops

ROOT = Path(__file__).resolve().parent
PUBLIC = ROOT / "public/starfort/v1"
TOWERS = ("arrow", "cannon", "frost", "arcane")
ENEMIES = ("slime", "runner", "armored", "swarm", "shield", "elite", "boss")
SOUNDS = ("build", "upgrade", "sell", "arrow", "cannon", "frost", "arcane", "hit", "leak", "wave", "victory", "defeat")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    expected = {f"tower-{kind}-{level}": (192, 224) for kind in TOWERS for level in (1, 2, 3)}
    expected.update({f"enemy-{kind}-{pose}": ((160, 160) if kind == "boss" else (128, 128)) for kind in ENEMIES for pose in (0, 1, 2)})
    expected.update({f"base-{state}": (224, 224) for state in ("intact", "damaged")})
    expected.update({f"projectile-{kind}": (64, 64) for kind in TOWERS})
    expected.update({f"effect-{kind}": (128, 128) for kind in ("burst", "frost", "arcane")})
    expected["coin"] = (64, 64)
    atlas_meta = json.loads((PUBLIC / "atlas.json").read_text())
    atlas = Image.open(PUBLIC / "atlas.webp").convert("RGBA")
    assert atlas.size == (atlas_meta["meta"]["width"], atlas_meta["meta"]["height"])
    assert set(expected) == set(atlas_meta["frames"]), "atlas missing/unexpected frames"
    checks = []
    digests = {}
    for name, size in {**expected, "map": (960, 640)}.items():
        svg = ET.parse(ROOT / "src" / f"{name}.svg").getroot()
        for element in svg.iter():
            assert element.tag.split("}")[-1] not in ("image", "script", "foreignObject")
            assert not any(key.endswith("href") for key in element.attrib), "external asset reference"
        image = Image.open(PUBLIC / f"{name}.webp").convert("RGBA")
        image.load()
        assert image.size == size, (name, image.size, size)
        assert len(image.getcolors(size[0] * size[1]) or []) > 24, f"{name}: insufficient visual detail"
        bounds = image.getchannel("A").getbbox()
        assert bounds is not None, f"{name}: empty image"
        if name != "map":
            assert image.getchannel("A").getextrema() == (0, 255), f"{name}: missing alpha"
            assert bounds[0] > 0 and bounds[1] > 0 and bounds[2] < size[0] and bounds[3] < size[1], f"{name}: clipped artwork {bounds}"
            frame = atlas_meta["frames"][name]
            x, y, w, h = (frame[key] for key in ("x", "y", "w", "h"))
            assert (w, h) == size and x >= 0 and y >= 0 and x + w <= atlas.width and y + h <= atlas.height
            assert ImageChops.difference(image, atlas.crop((x, y, x + w, y + h))).getbbox() is None, f"{name}: atlas mismatch"
        digests[name] = hashlib.sha256(image.tobytes()).hexdigest()
        checks.append({"asset": name, "size": size, "alphaBounds": bounds})
    for kind in TOWERS:
        assert len({digests[f"tower-{kind}-{level}"] for level in (1, 2, 3)}) == 3
    for kind in ENEMIES:
        assert len({digests[f"enemy-{kind}-{pose}"] for pose in (0, 1, 2)}) == 3
    for sound in SOUNDS:
        with wave.open(str(PUBLIC / "audio" / f"{sound}.wav")) as wav:
            assert wav.getnchannels() == 1 and wav.getsampwidth() == 2 and wav.getframerate() == 22050
            samples = array.array("h", wav.readframes(wav.getnframes()))
            peak = max(map(abs, samples))
            duration = len(samples) / wav.getframerate()
            assert 0.04 <= duration <= 2.5 and 100 < peak < 32000, (sound, duration, peak)
            assert abs(samples[0]) < 150 and abs(samples[-1]) < 150, f"{sound}: audible edge click"
            checks.append({"asset": "audio/" + sound, "seconds": duration, "peak": peak})
    manifest = json.loads((ROOT / "manifest.json").read_text())
    actual = {str(path.relative_to(PUBLIC)) for path in PUBLIC.rglob("*") if path.is_file()}
    assert actual == {item["path"] for item in manifest["files"]}, "manifest coverage"
    for item in manifest["files"]:
        content = (PUBLIC / item["path"]).read_bytes()
        assert len(content) == item["bytes"] and hashlib.sha256(content).hexdigest() == item["sha256"], item["path"]
    result = {"status": "passed", "checks": checks, "files": len(actual)}
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"status": result["status"], "checks": len(checks), "files": len(actual)}))


if __name__ == "__main__":
    main()
