import argparse
import gzip
import hashlib
import importlib.util
import json
import math
from pathlib import Path
import re
import struct
import subprocess
import wave


ROOT = Path(__file__).resolve().parents[2]
GUNS = ("p228", "scout", "xm1014", "mac10", "aug", "elite", "fiveseven", "ump45", "sg550", "galil", "famas", "usp", "glock18", "awp", "mp5navy", "m249", "m3", "m4a1", "tmp", "g3sg1", "deagle", "sg552", "ak47", "p90")
WEAPONS = GUNS + ("knife", "hegrenade", "flashbang", "smokegrenade", "c4")
BLENDS = {0: "opaque", 1: "additive", 2: "alpha", 3: "alpha-test"}


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


BUILDER = module("cs16_hud_builder", "build_assets.py")
FETCH = module("cs16_hud_fetch", "fetch_sources.py")


def parse_sprite(raw):
    if len(raw) < 42:
        raise ValueError("Truncated GoldSrc sprite header")
    ident, version, orientation, texture_format, radius, width, height, count, beam, sync = struct.unpack_from("<4siiifiiifi", raw)
    if ident != b"IDSP" or version != 2 or orientation not in range(5) or texture_format not in BLENDS or not math.isfinite(radius) or not math.isfinite(beam) or radius < 0 or sync not in (0, 1):
        raise ValueError("Unsupported GoldSrc sprite identity or format")
    if width < 1 or height < 1 or width > 2048 or height > 2048 or count < 1 or count > 256:
        raise ValueError("GoldSrc sprite dimensions or frame count exceed budget")
    colors = struct.unpack_from("<H", raw, 40)[0]
    if colors < 1 or colors > 256 or texture_format == 2 and colors != 256 or 42 + colors * 3 > len(raw):
        raise ValueError("GoldSrc sprite palette is invalid")
    palette = raw[42:42 + colors * 3]
    at = 42 + colors * 3
    frames = []
    total_pixels = 0
    for _ in range(count):
        if at + 20 > len(raw):
            raise ValueError("Truncated GoldSrc sprite frame")
        kind, origin_x, origin_y, frame_width, frame_height = struct.unpack_from("<5i", raw, at)
        at += 20
        if kind != 0:
            raise ValueError("Grouped GoldSrc HUD sprite frames are unsupported")
        pixels = frame_width * frame_height
        total_pixels += pixels
        if frame_width < 1 or frame_height < 1 or frame_width > width or frame_height > height or total_pixels > 8 * 1024 * 1024 or at + pixels > len(raw):
            raise ValueError("GoldSrc sprite frame exceeds its carrier")
        rgba = bytearray(pixels * 4)
        for index, value in enumerate(raw[at:at + pixels]):
            if value >= colors:
                raise ValueError("GoldSrc sprite palette index is out of bounds")
            red, green, blue = palette[value * 3:value * 3 + 3]
            alpha = 255
            if texture_format == 1:
                alpha = max(red, green, blue)
                if alpha:
                    red, green, blue = [round(channel * 255 / alpha) for channel in (red, green, blue)]
            elif texture_format == 2:
                red, green, blue = palette[765:768]
                alpha = value
            elif texture_format == 3 and value == 255:
                alpha = 0
            rgba[index * 4:index * 4 + 4] = bytes((red, green, blue, alpha))
        at += pixels
        frames.append({"origin": [origin_x, origin_y], "width": frame_width, "height": frame_height, "rgba": bytes(rgba)})
    if at != len(raw):
        raise ValueError("Unconsumed bytes after GoldSrc sprite frames")
    return {"textureFormat": texture_format, "orientation": orientation, "width": width, "height": height, "frames": frames}


def parse_hud_list(text):
    lines = text.splitlines()
    if not lines or not re.fullmatch(r"\s*\d+\s*", lines[0]):
        raise ValueError("GoldSrc HUD list count is missing")
    declared = int(lines[0])
    if declared < 1 or declared > 4096:
        raise ValueError("GoldSrc HUD list count exceeds budget")
    entries = []
    seen = set()
    for number, line in enumerate(lines[1:], 2):
        line = line.split("//", 1)[0].strip()
        if not line:
            continue
        fields = line.split()
        if len(fields) != 7 or not re.fullmatch(r"[A-Za-z0-9_]+", fields[0]) or not re.fullmatch(r"[A-Za-z0-9_]+", fields[2]):
            raise ValueError("Invalid GoldSrc HUD list row")
        try:
            resolution, x, y, width, height = [int(fields[index]) for index in (1, 3, 4, 5, 6)]
        except ValueError as error:
            raise ValueError("GoldSrc HUD list coordinates must be integers") from error
        if resolution not in (320, 640) or min(x, y) < 0 or min(width, height) < 1 or max(x + width, y + height) > 4096 or (resolution, fields[0]) in seen:
            raise ValueError("GoldSrc HUD definition is duplicated or out of range")
        seen.add((resolution, fields[0]))
        entries.append({"name": fields[0], "resolution": resolution, "sprite": fields[2], "rect": [x, y, width, height], "line": number})
    if len(entries) > declared:
        raise ValueError("GoldSrc HUD definitions exceed the declared count")
    return {"declared": declared, "entries": entries}


def crop_frame(frame, rect):
    x, y, width, height = rect
    if min(x, y) < 0 or min(width, height) < 1 or x + width > frame["width"] or y + height > frame["height"]:
        raise ValueError("HUD crop is outside its original sprite frame")
    return b"".join(frame["rgba"][((y + row) * frame["width"] + x) * 4:((y + row) * frame["width"] + x + width) * 4] for row in range(height))


def find_sprite(source, name):
    if not re.fullmatch(r"[A-Za-z0-9_]+", name):
        raise ValueError("Invalid original sprite name")
    for library in ("cstrike", "valve"):
        path = source / library / "sprites" / (name + ".spr")
        if path.is_file():
            FETCH.safe_components(path)
            return path
    raise ValueError("Required original sprite missing: " + name)


def atlas_regions(entries):
    unique = {}
    for entry in entries:
        width, height = entry["rect"][2:]
        key = (width, height, hashlib.sha256(entry["rgba"]).hexdigest())
        entry["atlasKey"] = key
        unique[key] = entry["rgba"]
    size = 256
    area = sum((key[0] + 2) * (key[1] + 2) for key in unique)
    while size * size < area * 2:
        size *= 2
    ordered = sorted(unique, key=lambda key: (-key[1], -key[0], key[2]))
    while True:
        placements = {}
        x = y = shelf = 0
        for key in ordered:
            width, height = key[:2]
            if x + width + 2 > size:
                y += shelf
                x = shelf = 0
            if y + height + 2 > size:
                break
            placements[key] = (x + 1, y + 1)
            x += width + 2
            shelf = max(shelf, height + 2)
        if len(placements) == len(unique):
            break
        size *= 2
        if size > 4096:
            raise ValueError("HUD atlas exceeds its fixed size budget")
    rgba = bytearray(size * size * 4)
    for key, (x, y) in placements.items():
        width, height = key[:2]
        for row in range(height):
            target = ((y + row) * size + x) * 4
            rgba[target:target + width * 4] = unique[key][row * width * 4:(row + 1) * width * 4]
    return size, bytes(rgba), placements


def hud_sound(source, output, key):
    if key not in ("items/9mmclip1.wav", "items/flashlight1.wav"):
        raise ValueError("Unsupported pinned HUD sound")
    path = source / "valve/sound" / key
    with wave.open(str(path), "rb") as original:
        channels = original.getnchannels()
        rate = original.getframerate()
        bits = original.getsampwidth() * 8
        frames = original.getnframes()
        pcm = original.readframes(frames)
    encoded = subprocess.run(["flac", "--silent", "--stdout", "--best", "--no-padding", "--no-seektable", str(path)], check=True, capture_output=True).stdout
    decoded = subprocess.run(["flac", "--silent", "--decode", "--stdout", "--force-raw-format", "--endian=little", "--sign=" + ("unsigned" if bits == 8 else "signed"), "-"], input=encoded, check=True, capture_output=True).stdout
    if decoded != pcm:
        raise ValueError("Original HUD sound PCM differs after lossless conversion")
    return {**BUILDER.write_resource(output, "sounds/" + str(Path(key).with_suffix(".flac")), encoded, "audio/flac"), "sourceSHA256": BUILDER.digest(path.read_bytes()), "pcmSHA256": BUILDER.digest(pcm), "channels": channels, "sampleRate": rate, "bits": bits, "frames": frames}


def defuser_model(source, output, models):
    descriptor = models.get("w-thighpack")
    if not isinstance(descriptor, dict):
        raise ValueError("The original converted defuse-kit world model is missing")
    source_path = source / "cstrike/models/w_thighpack.mdl"
    raw_source = source_path.read_bytes()
    if raw_source[:4] != b"IDST" or struct.unpack_from("<i", raw_source, 4)[0] != 10 or BUILDER.digest(raw_source) != descriptor.get("sourceSHA256"):
        raise ValueError("Defuse-kit model differs from its original MDL identity")
    carrier = (output / descriptor["file"]).read_bytes()
    if len(carrier) != descriptor["bytes"] or BUILDER.digest(carrier) != descriptor["sha256"]:
        raise ValueError("Defuse-kit GLB carrier differs from its recorded identity")
    decoded = gzip.decompress(carrier) if descriptor.get("encoding") == "gzip" else carrier
    if len(decoded) != descriptor["decodedBytes"] or BUILDER.digest(decoded) != descriptor["decodedSHA256"] or struct.unpack_from("<III", decoded) != (0x46546c67, 2, len(decoded)):
        raise ValueError("Defuse-kit decoded GLB differs from its recorded identity")
    length, kind = struct.unpack_from("<II", decoded, 12)
    if kind != 0x4e4f534a:
        raise ValueError("Defuse-kit GLB metadata is missing")
    document = json.loads(decoded[20:20 + length])
    if document["nodes"][0]["extras"]["sourceSHA256"] != descriptor["sourceSHA256"] or not document.get("images"):
        raise ValueError("Defuse-kit original model or embedded textures are missing")
    models["w-defuser"] = dict(descriptor)


def build_hud(source, output):
    source = Path(source).resolve()
    output = BUILDER.owned_output(output)
    output.mkdir(parents=True, exist_ok=True)
    metadata_path = ROOT / "games/cs16/assets/hud-sources.json"
    metadata_raw = metadata_path.read_bytes()
    metadata = json.loads(metadata_raw)
    for entry in FETCH.validate_manifest(metadata, "original"):
        if not FETCH.verify_file(source / entry["path"], entry):
            raise ValueError("Missing or changed pinned HUD source: " + entry["path"])
    manifest_path = output / "manifest.json"
    manifest = json.loads(manifest_path.read_text())
    if manifest.get("schema") != 1:
        raise ValueError("An existing CS16 public manifest is required")
    lists = []
    hud_path = source / "cstrike/sprites/hud.txt"
    native_hud = parse_hud_list(hud_path.read_text(encoding="latin1"))
    lists.append(("", native_hud))
    for weapon in WEAPONS:
        lists.append(("weapon-" + weapon + "/", parse_hud_list((source / "cstrike/sprites" / ("weapon_" + weapon + ".txt")).read_text(encoding="latin1"))))
    sprites = {}
    entries = []
    weapon_roles = {}
    skipped = []
    for prefix, definition in lists:
        for entry in definition["entries"]:
            path = find_sprite(source, entry["sprite"])
            if path not in sprites:
                raw = path.read_bytes()
                sprites[path] = (parse_sprite(raw), hashlib.sha256(raw).hexdigest())
            sprite, sha256 = sprites[path]
            if len(sprite["frames"]) != 1:
                raise ValueError("An original HUD bank must contain one static frame")
            try:
                cropped = crop_frame(sprite["frames"][0], entry["rect"])
            except ValueError:
                if not prefix or entry["resolution"] != 320:
                    raise
                skipped.append({"name": prefix + entry["name"], "resolution": 320, "sourceSprite": path.relative_to(source).as_posix(), "sourceRect": entry["rect"], "reason": "outside-original-frame"})
                continue
            entries.append({**entry, "key": prefix + entry["name"], "path": path.relative_to(source).as_posix(), "sourceSHA256": sha256, "textureFormat": sprite["textureFormat"], "rgba": cropped})
            if prefix and entry["resolution"] == 640:
                weapon_roles.setdefault(prefix[7:-1], {})[entry["name"]] = prefix + entry["name"]
    size, rgba, placements = atlas_regions(entries)
    atlas = BUILDER.write_resource(output, "hud/atlas.png", BUILDER.png_bytes(size, size, rgba), "image/png")
    definitions = {"320": {}, "640": {}}
    for entry in entries:
        x, y = placements[entry["atlasKey"]]
        definitions[str(entry["resolution"])][entry["key"]] = {"x": x, "y": y, "width": entry["rect"][2], "height": entry["rect"][3], "sourceSprite": entry["path"], "sourceRect": entry["rect"], "sourceSHA256": entry["sourceSHA256"], "blend": BLENDS[entry["textureFormat"]]}
    for weapon in GUNS:
        if "d_" + weapon not in definitions["640"] or "ammo" not in weapon_roles.get(weapon, {}):
            raise ValueError("An original gun death or ammunition sprite is missing: " + weapon)
    manifest["hud"] = {"schema": 1, "atlas": atlas, "width": size, "height": size, "defaultResolution": 640, "sprites": definitions, "weapons": weapon_roles, "killIcons": {key.removeprefix("d_"): key for key in definitions["640"] if key.startswith("d_")}, "source": {"repository": metadata["repository"], "commit": metadata["commit"], "catalogSHA256": BUILDER.digest(metadata_raw), "hudTextSHA256": BUILDER.digest(hud_path.read_bytes()), "declaredDefinitions": native_hud["declared"], "actualDefinitions": len(native_hud["entries"]), "spriteBanks": len(sprites), "skippedDefinitions": skipped}}
    for key in ("items/9mmclip1.wav", "items/flashlight1.wav"):
        manifest["sounds"][key] = hud_sound(source, output, key)
    defuser_model(source, output, manifest["models"])
    manifest_path.write_bytes(BUILDER.json_bytes(manifest) + b"\n")
    verification = ROOT / ".local/cs16/verification/hud"
    verification.mkdir(parents=True, exist_ok=True)
    (verification / "source-resources.json").write_bytes(BUILDER.json_bytes(BUILDER.resource_records(manifest, output)) + b"\n")
    return manifest


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=ROOT / ".local/cs16/research/original")
    parser.add_argument("--output", type=Path, default=ROOT / ".local/cs16/assets")
    arguments = parser.parse_args()
    manifest = build_hud(arguments.source, arguments.output)
    print(json.dumps({"passed": True, "atlas": manifest["hud"]["atlas"], "sprites": {key: len(value) for key, value in manifest["hud"]["sprites"].items()}, "weapons": len(manifest["hud"]["weapons"])}))


if __name__ == "__main__":
    main()
