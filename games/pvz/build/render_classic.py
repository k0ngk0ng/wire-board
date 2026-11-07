"""Bake classic PC reanimation tracks into transparent lossless WebP sheets."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import shutil

NORMAL_HIDDEN = ("anim_cone", "anim_bucket", "anim_screendoor", "Zombie_flaghand", "Zombie_duckytube", "anim_tongue", "Zombie_mustache", "Zombie_outerarm_screendoor", "Zombie_innerarm_screendoor")


def matrix(value):
    if "_matrix" in value:
        return value["_matrix"]
    kx, ky = math.radians(value["kx"]), math.radians(value["ky"])
    return (math.cos(kx) * value["sx"], -math.sin(ky) * value["sy"],
            math.sin(kx) * value["sx"], math.cos(ky) * value["sy"], value["x"], value["y"])


def multiply(left, right):
    a, b, c, d, x, y = left
    e, f, g, h, u, v = right
    return (a * e + b * g, a * f + b * h, c * e + d * g, c * f + d * h,
            a * u + b * v + x, c * u + d * v + y)


def inverse(value):
    a, b, c, d, x, y = value
    determinant = a * d - b * c
    if abs(determinant) < .000001:
        raise ValueError("Singular attachment matrix")
    return (d / determinant, -b / determinant, -c / determinant, a / determinant,
            (b * y - d * x) / determinant, (c * x - a * y) / determinant)


def frame_layers(definition, kind, clip, index):
    """Compose independently animated shooter body and attached head."""
    simple = kind in ("peashooter", "repeater", "snowpea", "leftpeater", "gatlingpea")
    multi = kind in ("threepeater", "splitpea")
    if not ((simple and clip == "attack") or (kind == "gatlingpea" and clip == "idle") or (multi and clip in ("idle", "attack", "attack_front", "attack_rear"))):
        return [(track["name"], track["frames"][index]) for track in definition["tracks"]]
    body_start, body_end = clip_range(definition, "anim_idle")
    if simple:
        bone = "anim_stem" if any(t["name"] == "anim_stem" for t in definition["tracks"]) else "anim_idle"
        heads = [("anim_shooting" if clip == "attack" else "anim_head_idle", bone)]
    elif kind == "threepeater":
        heads = [("anim_shooting" + str(i) if clip == "attack" else "anim_head_idle" + str(i), "anim_head" + str(i)) for i in (1, 2, 3)]
    else:
        heads = [("anim_shooting" if clip in ("attack", "attack_front") else "anim_head_idle", "anim_idle"),
                 ("anim_splitpea_shooting" if clip in ("attack", "attack_rear") else "anim_splitpea_idle", "anim_idle")]
    origin = clip_range(definition, heads[0][0])[0] if clip != "idle" else body_start
    offset = index - origin
    body_index = body_start + offset % (body_end - body_start + 1)
    selected = {}
    for track in definition["tracks"]:
        value = track["frames"][body_index]
        if value["image"] and value["f"] >= 0:
            selected[track["name"]] = value
    for layer_name, bone_name in heads:
        start, end = clip_range(definition, layer_name)
        head_index = start + offset % (end - start + 1)
        bone = next((track for track in definition["tracks"] if track["name"] == bone_name), None)
        overlay = (1, 0, 0, 1, 0, 0)
        if bone is not None:
            overlay = multiply(matrix(bone["frames"][body_index]), inverse(matrix(bone["frames"][body_start])))
        for track in definition["tracks"]:
            value = track["frames"][head_index]
            if value["image"] and value["f"] >= 0:
                selected[track["name"]] = dict(value, _matrix=multiply(overlay, matrix(value)))
    return [(track["name"], selected.get(track["name"], track["frames"][index]))
            for track in definition["tracks"]]


def visible(name, kind):
    if kind in ("normal", "conehead", "buckethead"):
        if name.startswith("anim_cone"):
            return kind == "conehead"
        if name.startswith("anim_bucket"):
            return kind == "buckethead"
        if name.startswith("anim_hair") and kind != "normal":
            return False
        return not name.startswith(NORMAL_HIDDEN)
    return True


def clip_range(definition, name):
    track = next(track for track in definition["tracks"] if track["name"] == name)
    active = [index for index, frame in enumerate(track["frames"]) if frame["f"] >= 0]
    if not active:
        raise ValueError(f"Empty animation layer {name}")
    return min(active), max(active)


PROFILES = {
    "peashooter": ("PeaShooterSingle", {"idle": "anim_full_idle", "attack": "anim_shooting"}),
    "sunflower": ("SunFlower", {"idle": "anim_idle"}),
    "cherrybomb": ("CherryBomb", {"idle": "anim_idle", "attack": "anim_explode"}),
    "wallnut": ("Wallnut", {"idle": "anim_idle"}),
    "potatomine": ("PotatoMine", {"idle": "anim_armed", "unarmed": "anim_idle", "attack": "anim_rise"}),
    "snowpea": ("SnowPea", {"idle": "anim_full_idle", "attack": "anim_shooting"}),
    "repeater": ("PeaShooter", {"idle": "anim_full_idle", "attack": "anim_shooting"}),
    "tallnut": ("Tallnut", {"idle": "anim_idle"}),
    "normal": ("Zombie", {"idle": "anim_idle", "walk": "anim_walk", "attack": "anim_eat", "death": "anim_death"}),
    "conehead": ("Zombie", {"idle": "anim_idle", "walk": "anim_walk", "attack": "anim_eat", "death": "anim_death"}),
    "buckethead": ("Zombie", {"idle": "anim_idle", "walk": "anim_walk", "attack": "anim_eat", "death": "anim_death"}),
    "newspaper": ("Zombie_paper", {"idle": "anim_idle", "walk": "anim_walk", "attack": "anim_eat", "rage": "anim_walk_nopaper", "death": "anim_death"}),
    "pole": ("Zombie_polevaulter", {"idle": "anim_idle", "walk": "anim_run", "attack": "anim_eat", "jump": "anim_jump", "spent": "anim_walk", "death": "anim_death"}),
    "football": ("Zombie_football", {"idle": "anim_idle", "walk": "anim_walk", "attack": "anim_eat", "death": "anim_death"}),
    "mower": ("LawnMower", {"idle": "anim_normal"}),
    "sun": ("Sun", {"idle": None}),
}


def bake(source, output, version="classic-v1", profiles=None):
    from PIL import Image
    from reanim import decode
    output.mkdir(parents=True, exist_ok=False)
    (output / "sprites").mkdir()
    (output / "images").mkdir()
    (output / "audio").mkdir()
    images = {}
    for path in (source / "reanim").glob("*.png"):
        images["IMAGE_REANIM_" + path.stem.upper()] = Image.open(path).convert("RGBA")
    manifest = {"version": version, "sourceVersion": "PC 1.0.0.1051", "background": "images/background1.jpg",
                "backgroundCrop": {"x": 220, "y": 0, "width": 800, "height": 600},
                "lawn": {"x": 40, "y": 80, "width": 720, "height": 500}, "sprites": {}, "images": {}, "audio": {}}
    contact = Image.new("RGBA", (8 * 160, 2 * 200), (232, 230, 210, 255))
    for profile_index, (kind, (name, clips)) in enumerate((profiles or PROFILES).items()):
        definition = decode((source / "compiled/reanim" / (name + ".reanim.compiled")).read_bytes())
        track_frames = len(definition["tracks"][0]["frames"])
        ranges = {clip: (clip_range(definition, layer) if layer else (0, track_frames - 1)) for clip, layer in clips.items()}
        # Main visual anchor is the classic plant centre/ground or zombie hitbox centre/feet.
        anchor = (40, 80) if profile_index < 8 else (60, 115)
        if kind == "mower":
            anchor = (40, 80)
        if kind == "sun":
            anchor = (0, 0)
        bounds = []
        for clip, (start, end) in ranges.items():
            for index in range(start, end + 1):
                for track_name, value in frame_layers(definition, kind, clip, index):
                    image = images.get(value["image"])
                    if image is None or value["f"] < 0 or value["a"] <= 0 or not visible(track_name, kind):
                        continue
                    a, b, c, d, x, y = matrix(value)
                    bounds.extend((a * px + b * py + x, c * px + d * py + y) for px, py in ((0, 0), (image.width, 0), (0, image.height), image.size))
        min_x = math.floor(min(point[0] for point in bounds)) - 2
        min_y = math.floor(min(point[1] for point in bounds)) - 2
        width = math.ceil(max(point[0] for point in bounds)) - min_x + 2
        height = math.ceil(max(point[1] for point in bounds)) - min_y + 2
        rendered = {}
        for clip, (start, end) in ranges.items():
            count = end - start + 1
            columns = min(12, count)
            sheet = Image.new("RGBA", (width * columns, height * math.ceil(count / columns)))
            for index in range(start, end + 1):
                canvas = Image.new("RGBA", (width, height))
                for track_name, value in frame_layers(definition, kind, clip, index):
                    image = images.get(value["image"])
                    if image is None or value["f"] < 0 or value["a"] <= 0 or not visible(track_name, kind):
                        continue
                    a, b, c, d, x, y = matrix(value)
                    x, y = x - min_x, y - min_y
                    determinant = a * d - b * c
                    if abs(determinant) < .000001:
                        continue
                    inverse = (d / determinant, -b / determinant, (b * y - d * x) / determinant,
                               -c / determinant, a / determinant, (c * x - a * y) / determinant)
                    layer = image.transform((width, height), Image.Transform.AFFINE, inverse, Image.Resampling.BICUBIC)
                    if value["a"] < 1:
                        layer.putalpha(layer.getchannel("A").point(lambda alpha: int(alpha * max(0, value["a"]))))
                    canvas.alpha_composite(layer)
                cell = index - start
                sheet.alpha_composite(canvas, ((cell % columns) * width, (cell // columns) * height))
                if clip == "idle" and index == start:
                    preview = canvas.copy()
                    preview.thumbnail((150, 185))
                    contact.alpha_composite(preview, ((profile_index % 8) * 160 + (160 - preview.width) // 2,
                                                       (profile_index // 8) * 200 + 200 - preview.height))
            relative = "sprites/" + kind + "-" + clip + ".webp"
            sheet.save(output / relative, lossless=True, method=6)
            rendered[clip] = {"src": relative, "frameWidth": width, "frameHeight": height, "columns": columns,
                              "frames": count, "fps": definition["fps"], "anchorX": anchor[0] - min_x, "anchorY": anchor[1] - min_y,
                              "sourceFrames": [start, end], "loop": clip not in ("death", "jump")}
        manifest["sprites"][kind] = dict(rendered["idle"], animations=rendered)
        print(kind, width, height, "clips", ",".join(rendered), flush=True)
    for key, name in {"background": "background1.jpg", "shovel": "Shovel.png", "seedpacket": "SeedPacket_Larger.png",
                      "seedbank": "SeedBank.png", "projectile": "ProjectilePea.png", "snowProjectile": "ProjectileSnowPea.png", "shadow": "plantshadow.png"}.items():
        shutil.copyfile(source / "images" / name, output / "images" / name)
        manifest["images"][key] = "images/" + name
    for key, name in {"plant": "plant", "spawn": "groan", "shoot": "throw", "hit": "splat", "explosion": "cherrybomb", "mower": "lawnmower", "shovel": "shovel", "sun": "points"}.items():
        candidates = sorted((source / "sounds").glob(name + "*.ogg"))
        if not candidates:
            continue
        file = candidates[0]
        shutil.copyfile(file, output / "audio" / file.name)
        manifest["audio"][key] = "audio/" + file.name
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    file_manifest = [{"path": str(path.relative_to(output)), "bytes": path.stat().st_size, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
                     for path in sorted(output.rglob("*")) if path.is_file()]
    return manifest, file_manifest, contact


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--contact", type=Path, required=True)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--version", default="classic-v1")
    args = parser.parse_args()
    _, files, contact = bake(args.source, args.output, args.version)
    contact.save(args.contact)
    args.manifest.write_text(json.dumps({"version": args.version, "files": files}, indent=2) + "\n")


if __name__ == "__main__":
    main()
