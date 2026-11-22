import argparse
import hashlib
import importlib.util
import json
import math
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
NAMES = ("blood", "bloodspray", "fexplo", "eexplo")
SPEC = importlib.util.spec_from_file_location("cs16_original_effect_sprite", Path(__file__).with_name("build_hud.py"))
SPRITES = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SPRITES)


def effect_atlas(sprite):
    count = len(sprite["frames"])
    columns = math.isqrt(count)
    if columns * columns < count:
        columns += 1
    rows = (count + columns - 1) // columns
    cell_width = sprite["width"] + 2
    cell_height = sprite["height"] + 2
    width = columns * cell_width
    height = rows * cell_height
    rgba = bytearray(width * height * 4)
    frames = []
    evidence = []
    for index, source in enumerate(sprite["frames"]):
        x = (index % columns) * cell_width + 1
        y = (index // columns) * cell_height + 1
        for row in range(source["height"]):
            start = ((y + row) * width + x) * 4
            count_bytes = source["width"] * 4
            rgba[start:start + count_bytes] = source["rgba"][row * count_bytes:(row + 1) * count_bytes]
        frames.append({"x": x, "y": y, "width": source["width"], "height": source["height"], "origin": source["origin"]})
        alpha = source["rgba"][3::4]
        evidence.append({"frame": index, "width": source["width"], "height": source["height"], "origin": source["origin"], "rgbaSHA256": hashlib.sha256(source["rgba"]).hexdigest(), "transparentPixels": alpha.count(0), "opaquePixels": alpha.count(255), "partialPixels": sum(0 < value < 255 for value in alpha)})
    return width, height, bytes(rgba), frames, evidence


def build_effects(source, output):
    source = Path(source).resolve()
    output = SPRITES.BUILDER.owned_output(output)
    output.mkdir(parents=True, exist_ok=True)
    catalog_path = ROOT / "games/cs16/assets/effects-sources.json"
    catalog = json.loads(catalog_path.read_text())
    for entry in SPRITES.FETCH.validate_manifest(catalog, "original"):
        if not SPRITES.FETCH.verify_file(source / entry["path"], entry):
            raise ValueError("Original combat sprite source identity changed: " + entry["path"])
    manifest_path = output / "manifest.json"
    manifest = json.loads(manifest_path.read_text())
    if manifest.get("schema") != 1:
        raise ValueError("An existing CS16 asset manifest is required")
    palette = (source / "cstrike/gfx/palette.lmp").read_bytes()
    if len(palette) != 768:
        raise ValueError("Original engine color palette must contain 256 RGB entries")
    values = {}
    evidence = []
    for name in NAMES:
        source_path = source / "valve/sprites" / (name + ".spr")
        raw = source_path.read_bytes()
        sprite = SPRITES.parse_sprite(raw)
        width, height, rgba, frames, frame_evidence = effect_atlas(sprite)
        atlas = SPRITES.BUILDER.write_resource(output, "effects/" + name + ".png", SPRITES.BUILDER.png_bytes(width, height, rgba), "image/png")
        values[name] = {"atlas": atlas, "width": width, "height": height, "frames": frames, "textureFormat": sprite["textureFormat"], "orientation": sprite["orientation"], "blend": "additive" if sprite["textureFormat"] == 1 else "normal", "sourceFile": source_path.relative_to(source).as_posix(), "sourceBytes": len(raw), "sourceSHA256": hashlib.sha256(raw).hexdigest()}
        if name in ("blood", "bloodspray"):
            values[name].update({"tint": list(palette[247 * 3:248 * 3]), "paletteIndex": 247, "paletteSourceSHA256": hashlib.sha256(palette).hexdigest()})
        evidence.append({"name": name, "sourceFile": values[name]["sourceFile"], "sourceBytes": len(raw), "sourceSHA256": values[name]["sourceSHA256"], "textureFormat": sprite["textureFormat"], "orientation": sprite["orientation"], "atlas": atlas, "tint": values[name].get("tint"), "paletteIndex": values[name].get("paletteIndex"), "frames": frame_evidence})
    manifest["effects"] = values
    manifest_path.write_bytes(SPRITES.BUILDER.json_bytes(manifest) + b"\n")
    verification = ROOT / ".local/cs16/verification/combat-effect-assets"
    verification.mkdir(parents=True, exist_ok=True)
    (verification / "frames.json").write_text(json.dumps({"sourceRepository": catalog["repository"], "sourceCommit": catalog["commit"], "sourceCatalogSHA256": hashlib.sha256(catalog_path.read_bytes()).hexdigest(), "sprites": evidence}, indent=2) + "\n")
    (verification / "source-resources.json").write_bytes(SPRITES.BUILDER.json_bytes(SPRITES.BUILDER.resource_records(manifest, output)) + b"\n")
    return manifest


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=ROOT / ".local/cs16/research/original")
    parser.add_argument("--output", type=Path, default=ROOT / ".local/cs16/assets")
    arguments = parser.parse_args()
    manifest = build_effects(arguments.source, arguments.output)
    print(json.dumps({"passed": True, "effects": {name: {"frames": len(value["frames"]), "bytes": value["atlas"]["bytes"], "sha256": value["atlas"]["sha256"]} for name, value in manifest["effects"].items()}}))


if __name__ == "__main__":
    main()
