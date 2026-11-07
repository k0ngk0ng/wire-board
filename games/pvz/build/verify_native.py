"""Verify the complete published native-layer asset pipeline and semantic poses."""
import argparse
import gzip
import hashlib
import json
import math
from pathlib import Path


def main():
    from PIL import Image
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("games/pvz/assets/public/pvz/classic-v2"))
    parser.add_argument("--catalogue", type=Path, default=Path("docs/research/pvz/evidence/full-catalog.json"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    root = args.root
    manifest = json.loads((root / "manifest.json").read_text())
    declared = json.loads((root.parents[2] / "manifest-v2.json").read_text())
    catalogue = json.loads(args.catalogue.read_text())
    for entry in declared["files"]:
        path = root / entry["path"]
        assert path.stat().st_size == entry["bytes"], path
        assert hashlib.sha256(path.read_bytes()).hexdigest() == entry["sha256"], path
    for group in ("byUnit", "byScene", "byPreview"):
        for key, paths in manifest[group].items():
            for path in paths:
                assert (root / path).is_file(), (group, key, path)
    decoded_images = {}
    records, payloads = [], {}
    for unit in catalogue["plants"] + catalogue["zombies"]:
        key = unit["id"]
        sprite = manifest["sprites"][key]
        data = json.loads(gzip.decompress((root / sprite["native"]["src"]).read_bytes()))
        assert data["matrixOrder"] == "canvas-a-b-c-d-e-f"
        assert set(sprite["animations"]) <= set(data["clips"]), key
        for clip, value in data["clips"].items():
            assert value["frames"] and any(value["frames"]), (key, clip)
            for frame in value["frames"]:
                for layer in frame:
                    assert len(layer["m"]) == 6 and all(math.isfinite(x) for x in layer["m"]), (key, clip)
                    assert 0 <= layer["alpha"] <= 1, (key, clip)
                    path = layer["src"]
                    if path not in decoded_images:
                        image = Image.open(root / path)
                        image.load()
                        assert image.mode in ("RGB", "RGBA"), path
                        assert image.convert("RGBA").getchannel("A").getbbox() is not None, path
                        decoded_images[path] = image.size
        preview = Image.open(root / sprite["preview"]["src"])
        assert preview.getchannel("A").getbbox() is not None, key
        records.append({"kind": key, "clips": len(data["clips"]), "compressedBytes": (root / sprite["native"]["src"]).stat().st_size, "previewSize": preview.size})
        payloads[key] = data
    body = {"backleaf", "frontleaf", "stalk_bottom", "stalk_top"}
    for key in ("peashooter", "snowpea", "repeater", "gatlingpea", "splitpea"):
        for clip in ("idle", "attack"):
            for index, frame in enumerate(payloads[key]["clips"][clip]["frames"]):
                assert body <= {x["sourceLayer"] for x in frame}, (key, clip, index, "missing stalk/leaves")
    for clip in ("idle", "attack"):
        for frame in payloads["threepeater"]["clips"][clip]["frames"]:
            assert {"anim_face1", "anim_face2", "anim_face3"} <= {x["sourceLayer"] for x in frame}, "missing threepeater head"
    for key, layer in (("wallnut", "anim_face"), ("tallnut", "anim_idle"), ("garlic", "anim_face"), ("pumpkin", "Pumpkin_front")):
        original = next(x["src"] for x in payloads[key]["clips"]["idle"]["frames"][0] if x["sourceLayer"] == layer)
        damaged = next(x["src"] for x in payloads[key]["clips"]["damage2"]["frames"][0] if x["sourceLayer"] == layer)
        assert original != damaged, (key, "missing original damaged artwork")
    sled = payloads["bobsled"]["clips"]["sledding"]["frames"][0]
    layers = {x["sourceLayer"] for x in sled}
    assert {"sled/back", "sled/front"} <= layers, "missing bobsled vehicle"
    assert all(any(name.startswith(f"passenger{slot}/") for name in layers) for slot in range(4)), "missing bobsled passengers"
    assert manifest["maps"]["roof"]["roofSlope"] == [100, 80, 60, 40, 20, 0, 0, 0, 0]
    for key in ("pool", "fog"):
        image = Image.open(root / manifest["maps"][key]["background"])
        r, g, b, *_ = image.getpixel((620, 350))
        assert b > r and not (r > 240 and g > 240), (key, "missing original water surface")
    args.output.write_text(json.dumps({"status": "pass", "scope": "asset pipeline; not original Windows runtime equivalence",
                                       "units": records, "decodedSharedImages": len(decoded_images), "declaredFiles": len(declared["files"]),
                                       "semanticChecks": ["shooter-body", "multihead-body", "original-damage-stages", "bobsled-team", "pool-water", "roof-slope"]}, indent=2) + "\n")
    print(f"Verified {len(records)} units, {len(decoded_images)} shared original images and semantic poses")


if __name__ == "__main__":
    main()
