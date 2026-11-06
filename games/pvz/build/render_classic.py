"""Bake classic PC reanimation tracks into transparent lossless WebP sheets."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import shutil

NORMAL_HIDDEN = ("anim_cone", "anim_bucket", "anim_screendoor", "Zombie_flaghand", "Zombie_duckytube", "anim_tongue", "Zombie_mustache", "Zombie_outerarm_screendoor", "Zombie_innerarm_screendoor")


def matrix(value):
    kx, ky = math.radians(value["kx"]), math.radians(value["ky"])
    return (math.cos(kx) * value["sx"], -math.sin(ky) * value["sy"],
            math.sin(kx) * value["sx"], math.cos(ky) * value["sy"], value["x"], value["y"])


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


def bake(source, output):
    from PIL import Image
    from reanim import decode
    output.mkdir(parents=True, exist_ok=False)
    (output / "sprites").mkdir()
    (output / "images").mkdir()
    (output / "audio").mkdir()
    images = {}
    for path in (source / "reanim").glob("*.png"):
        images["IMAGE_REANIM_" + path.stem.upper()] = Image.open(path).convert("RGBA")
    manifest = {"version": "classic-v1", "sourceVersion": "PC 1.0.0.1051", "background": "images/background1.jpg",
                "backgroundCrop": {"x": 220, "y": 0, "width": 800, "height": 600},
                "lawn": {"x": 40, "y": 80, "width": 720, "height": 500}, "sprites": {}, "images": {}, "audio": {}}
    contact = Image.new("RGBA", (8 * 160, 2 * 200), (232, 230, 210, 255))
    for profile_index, (kind, (name, clips)) in enumerate(PROFILES.items()):
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
        for start, end in ranges.values():
            for index in range(start, end + 1):
                for track in definition["tracks"]:
                    value = track["frames"][index]
                    image = images.get(value["image"])
                    if image is None or value["f"] < 0 or value["a"] <= 0 or not visible(track["name"], kind):
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
                for track in definition["tracks"]:
                    value = track["frames"][index]
                    image = images.get(value["image"])
                    if image is None or value["f"] < 0 or value["a"] <= 0 or not visible(track["name"], kind):
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
    args = parser.parse_args()
    _, files, contact = bake(args.source, args.output)
    contact.save(args.contact)
    args.manifest.write_text(json.dumps({"version": "classic-v1", "files": files}, indent=2) + "\n")


if __name__ == "__main__":
    main()
