"""Build the complete classic PC unit/scene catalogue from authenticated PAK data."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import re
import shutil

from reanim import decode
from render_classic import clip_range, frame_layers, inverse, matrix, multiply

PLAIN = {"normal", "flag", "conehead", "buckethead", "screendoor", "duckytube", "peahead", "wallnuthead", "jalapenohead", "gatlinghead", "squashhead", "tallnuthead"}
HEADED = {"peahead", "wallnuthead", "jalapenohead", "gatlinghead", "squashhead", "tallnuthead"}


def scene_definitions():
    result = {}
    for kind, number in (("day", "1"), ("night", "2"), ("pool", "3"), ("fog", "4"), ("roof", "5"), ("roofnight", "6boss")):
        water, roof = kind in ("pool", "fog"), kind in ("roof", "roofnight")
        rows, row_height = (6, 85) if water else (5, 85 if roof else 100)
        value = {"background": "images/background" + number + ".webp", "backgroundCrop": {"x": 220, "y": 0, "width": 800, "height": 600},
                 "lawn": {"x": 40, "y": 70 if roof else 80, "width": 720, "height": rows * row_height},
                 "rows": rows, "columns": 9, "rowHeight": row_height, "night": kind in ("night", "fog", "roofnight")}
        if water:
            value.update(poolRows=[2, 3], water={"src": "images/pool_night.jpg" if kind == "fog" else "images/pool.jpg", "x": 34, "y": 278})
        if roof:
            value.update(roofSlope=[100, 80, 60, 40, 20, 0, 0, 0, 0], continuousSlope={"beforeX": 440, "factor": .25})
        result[kind] = value
    return result


def layer_visible(name, kind):
    if kind not in PLAIN:
        if kind == "football" and name.startswith("anim_hair"):
            return False
        return True
    if name.startswith("anim_cone"):
        return kind == "conehead"
    if name.startswith("anim_bucket"):
        return kind == "buckethead"
    if name.startswith("anim_screendoor"):
        return kind == "screendoor"
    if name.startswith("Zombie_flaghand"):
        return kind == "flag"
    if name.startswith("Zombie_duckytube"):
        return kind == "duckytube"
    if name.startswith(("anim_tongue", "Zombie_mustache")):
        return False
    if name.startswith(("Zombie_outerarm_screendoor", "Zombie_innerarm_screendoor")):
        return kind == "screendoor" or (kind == "flag" and name == "Zombie_innerarm_screendoor")
    if kind == "screendoor" and name.startswith(("Zombie_outerarm_hand", "Zombie_outerarm_lower", "Zombie_outerarm_upper", "anim_innerarm")):
        return False
    if kind == "flag" and name.startswith("anim_innerarm"):
        return False
    if kind in ("conehead", "buckethead") and name.startswith("anim_hair"):
        return False
    if kind in HEADED:
        if name.startswith(("anim_hair", "anim_head")):
            return False
        if kind in ("wallnuthead", "jalapenohead", "tallnuthead") and name.startswith("Zombie_tie"):
            return False
    return True


def image_overrides(kind, clip):
    stage = 1 if clip.startswith("damage1") else 2 if clip.startswith("damage2") else 0
    result = {}
    if kind == "redeye":
        result["anim_head1"] = "IMAGE_REANIM_ZOMBIE_GARGANTUAR_HEAD_REDEYE"
    if not stage:
        return result
    if kind in ("wallnut", "tallnut"):
        result["anim_face" if kind == "wallnut" else "anim_idle"] = "IMAGE_REANIM_" + kind.upper() + "_CRACKED" + str(stage)
    elif kind == "garlic":
        result["anim_face"] = "IMAGE_REANIM_GARLIC_BODY" + str(stage + 1)
    elif kind == "pumpkin":
        result["Pumpkin_front"] = "IMAGE_REANIM_PUMPKIN_DAMAGE" + ("1" if stage == 1 else "3")
    elif kind in ("conehead", "buckethead"):
        result["anim_cone" if kind == "conehead" else "anim_bucket"] = "IMAGE_REANIM_ZOMBIE_" + ("CONE" if kind == "conehead" else "BUCKET") + str(stage + 1)
    elif kind == "newspaper":
        result["Zombie_paper_paper"] = "IMAGE_REANIM_ZOMBIE_PAPER_PAPER" + str(stage + 1)
    return result


class Builder:
    def __init__(self, source, output, catalogue):
        from PIL import Image
        self.Image, self.source, self.output, self.catalogue = Image, source, output, catalogue
        self.definitions, self.images = {}, {}
        self.files = {str(path.relative_to(source)).casefold(): path for path in source.rglob("*") if path.is_file()}
        for path in (source / "reanim").iterdir():
            if path.suffix.lower() not in (".png", ".jpg") or path.stem.endswith("_"):
                continue
            image = Image.open(path).convert("RGBA")
            mask = self.files.get(("reanim/" + path.stem + "_.png").casefold())
            if path.suffix.lower() == ".jpg" and mask:
                image.putalpha(Image.open(mask).convert("L"))
            self.images["IMAGE_REANIM_" + path.stem.upper()] = image
        for name in ("Zombie_bobsled_inside", "Zombie_bobsled1", "Zombie_bobsled2", "Zombie_bobsled3", "Zombie_bobsled4"):
            self.images["IMAGE_" + name.upper()] = Image.open(self.source / "images" / (name + ".png")).convert("RGBA")
        self.plants = {unit["id"] for unit in catalogue["plants"]}
        self.units = catalogue["plants"] + catalogue["zombies"] + [
            {"id": kind, "originalId": -1, "reanim": name, "group": "effect"}
            for kind, name in (("mower", "LawnMower"), ("poolcleaner", "PoolCleaner"), ("roofcleaner", "RoofCleaner"),
                               ("sun", "Sun"), ("firepea", "FirePea"), ("fumepuff", "Puff"),
                               ("bossfireball", "Zombie_boss_fireball"), ("bossiceball", "Zombie_boss_iceball"),
                               ("jalapenofire", "fire"), ("charred", "Zombie_charred"))]
        self.manifest = {"version": "classic-v2", "sourceVersion": "PC 1.0.0.1051", "maps": scene_definitions(),
                         "sprites": {}, "previews": {}, "images": {}, "audio": {}, "byScene": {}, "byUnit": {}, "byPreview": {}}
        self.audit = []

    def definition(self, name):
        if name not in self.definitions:
            path = self.files[("compiled/reanim/" + name + ".reanim.compiled").casefold()]
            self.definitions[name] = decode(path.read_bytes())
        return self.definitions[name]

    def profiles(self, unit):
        d = self.definition(unit["reanim"])
        names = {t["name"] for t in d["tracks"]}
        kind = unit["id"]
        idle = next((x for x in ("anim_full_idle", "anim_idle", "anim_walk", "anim_drive", "anim_moonwalk") if x in names), None)
        if kind == "mower":
            idle = "anim_normal"
        idle = {"poolcleaner": "anim_water", "fumepuff": "anim_puff", "bossiceball": "anim_role", "bossfireball": "anim_role", "jalapenofire": "anim_flame"}.get(kind, idle)
        if idle is None and unit.get("group") != "effect":
            raise ValueError(f"No idle animation for {kind}")
        result = {"idle": idle}
        preferences = {"walk": ("anim_run" if kind == "pole" else "anim_walkdolphin" if kind == "dolphin" else "anim_ladderwalk" if kind == "ladder" else "anim_walk", "anim_drive", "anim_moonwalk"),
                       "attack": ("anim_shooting1" if kind == "threepeater" else "anim_shooting", "anim_shoot", "anim_laddereat" if kind == "ladder" else "anim_eat", "anim_bite", "anim_attack", "anim_smash", "anim_explode", "anim_grab", "anim_blow", "anim_attract", "anim_block"),
                       "death": ("anim_death",), "sleep": ("anim_sleep",), "rage": ("anim_walk_nopaper",), "spent": ("anim_walk",), "jump": ("anim_jump",)}
        if kind == "potatomine":
            result = {"idle": "anim_armed", "unarmed": "anim_idle"}
        for key, candidates in preferences.items():
            if key == "walk" and kind in self.plants or key == "spent" and kind not in ("pole", "dolphin", "ladder"):
                continue
            chosen = next((x for x in candidates if x in names), None)
            if chosen:
                result[key] = chosen
        # Preserve the remaining actual action markers, excluding image/attachment layers.
        action = re.compile(r"^anim_(?:.*(?:walk|eat|death|swim|jump|run|sleep|grow|rise|lower|scared|charge|armed|unarmed|grab|hold|raise|land|pop|dig|drill|dizzy|moonwalk|point|armraise|ride|smash|throw|shoot|spawn|stomp|bungee|enter|leave|head_attack|head_idle|RV|block|crumble|twitch|chew|swallow|look).*)$")
        for track in d["tracks"]:
            name = track["name"]
            if name in result.values() or not action.match(name) or any(x["image"] for x in track["frames"]):
                continue
            if kind in ("peashooter", "repeater", "snowpea", "gatlingpea", "splitpea", "threepeater", "leftpeater") and "head" in name:
                continue
            active = [x for x in track["frames"] if x["f"] >= 0]
            if active:
                result[name.removeprefix("anim_")] = name
        if kind in ("wallnut", "tallnut", "garlic", "pumpkin"):
            result.update(damage1=idle, damage2=idle)
        if kind in ("conehead", "buckethead", "newspaper"):
            for stage in (1, 2):
                for action_name in ("idle", "walk", "attack"):
                    if action_name in result:
                        result[f"damage{stage}_{action_name}"] = result[action_name]
        if kind == "bobsled":
            result.update(sledding="anim_idle", **{"sled-crashing": "anim_jump"})
        if kind == "squashhead":
            result.update(**{"squash-rising": "anim_walk2", "squash-smashing": "anim_walk2"})
        if kind == "sunshroom":
            result.update(grown="anim_bigidle", grown_asleep="anim_bigsleep")
        if kind == "cactus":
            result.update(high="anim_idlehigh", high_attack="anim_shootinghigh")
        if kind in ("bossfireball", "bossiceball"):
            result["form"] = "anim_form"
        return result

    def compose(self, unit, clip, index):
        kind, definition = unit["id"], self.definition(unit["reanim"])
        semantic_clip = "attack" if clip.startswith("shooting") and kind in ("peashooter", "snowpea", "repeater", "gatlingpea", "threepeater", "splitpea") else clip
        source = frame_layers(definition, kind, semantic_clip, index)
        overrides = image_overrides(kind, clip)
        layers = []
        for name, original in source:
            value = dict(original)
            if name in overrides:
                value["image"] = overrides[name]
            if kind == "garlic" and clip.startswith("damage2") and name.startswith("Garlic_stem"):
                continue
            if layer_visible(name, kind):
                layers.append((name, value))
        if kind == "bobsled" and clip in ("sledding", "sled-crashing"):
            vehicle = {"x": -76, "y": 15, "kx": 0, "ky": 0, "sx": 1, "sy": 1, "f": 0, "a": 1,
                       "image": "IMAGE_ZOMBIE_BOBSLED_INSIDE"}
            team = [("sled/back", vehicle)]
            for slot in range(4):
                for name, value in layers:
                    team.append((f"passenger{slot}/" + name, dict(value, _matrix=multiply((1, 0, 0, 1, slot * 50, 0), matrix(value)))))
            team.append(("sled/front", dict(vehicle, image="IMAGE_ZOMBIE_BOBSLED4" if clip == "sled-crashing" else "IMAGE_ZOMBIE_BOBSLED1")))
            return [(name, value) for name, value in team if value["image"] and value["f"] >= 0 and value["a"] > 0]
        attachments = []
        if kind == "flag":
            attachments.append(("Zombie_flaghand", "Zombie_FlagPole", "anim_flag", (1, 0, 0, 1, 0, 0)))
        heads = {"peahead": ("anim_head1", "PeaShooterSingle", "anim_head_idle", 65, -5, -1, 1),
                 "gatlinghead": ("anim_head1", "GatlingPea", "anim_head_idle", 65, -5, -1, 1),
                 "wallnuthead": ("Zombie_body", "Wallnut", "anim_idle", 50, 0, -.8, .8),
                 "tallnuthead": ("Zombie_body", "Tallnut", "anim_idle", 37, 0, -.8, .8),
                 "jalapenohead": ("Zombie_body", "Jalapeno", "anim_idle", 55, -5, -1, 1),
                 "squashhead": ("anim_head1", "Squash", "anim_idle", 55, -15, -.75, .75)}
        if kind in heads:
            bone, name, layer, x, y, sx, sy = heads[kind]
            if kind == "squashhead" and clip in ("squash-rising", "squash-smashing"):
                layer = "anim_jumpup" if clip == "squash-rising" else "anim_jumpdown"
            rotation = .2
            attachments.append((bone, name, layer, (math.cos(rotation) * sx, -math.sin(rotation) * sy, math.sin(rotation) * sx, math.cos(rotation) * sy, x, y)))
        if kind == "zomboss":
            attachments.append(("Boss_head2", "Zombie_Boss_driver", "anim_idle", (1.2, 0, 0, 1.2, 28, -84)))
        for bone_name, child_name, marker, offset in attachments:
            bone = next(t for t in definition["tracks"] if t["name"] == bone_name)
            current = bone["frames"][index]
            if kind == "zomboss" and current["f"] < 0:
                continue
            child = self.definition(child_name)
            child_names = {t["name"] for t in child["tracks"]}
            if marker not in child_names:
                marker = "anim_idle" if "anim_idle" in child_names else None
            start, end = clip_range(child, marker) if marker else (0, len(child["tracks"][0]["frames"]) - 1)
            if kind == "squashhead" and clip in ("squash-rising", "squash-smashing"):
                parent_start, _ = clip_range(definition, "anim_walk2")
                child_index = min(end, start + index - parent_start)
            else:
                child_index = start + index % (end - start + 1)
            overlay = multiply(multiply(matrix(current), inverse(matrix(bone["frames"][0]))), offset)
            for track in child["tracks"]:
                frame = track["frames"][child_index]
                if frame["image"] and frame["f"] >= 0:
                    layers.append((child_name + "/" + track["name"], dict(frame, _matrix=multiply(overlay, matrix(frame)))))
        return [(name, value) for name, value in layers if value["image"] and value["f"] >= 0 and value["a"] > 0]

    def render_clip(self, unit, clip, marker):
        Image, d = self.Image, self.definition(unit["reanim"])
        start, end = clip_range(d, marker) if marker is not None else (0, len(d["tracks"][0]["frames"]) - 1)
        frames = [self.compose(unit, clip, index) for index in range(start, end + 1)]
        points = []
        for layers in frames:
            for name, value in layers:
                if value["image"] not in self.images:
                    raise ValueError(f"Missing original image for {unit['id']}/{clip}/{name}: {value['image']}")
                image = self.images[value["image"]]
                a, b, c, e, x, y = matrix(value)
                points.extend((a * px + b * py + x, c * px + e * py + y) for px, py in ((0, 0), (image.width, 0), (0, image.height), image.size))
        if not points:
            raise ValueError(f"No complete image for {unit['id']}/{clip}")
        min_x, min_y = math.floor(min(x for x, _ in points)) - 2, math.floor(min(y for _, y in points)) - 2
        width, height = math.ceil(max(x for x, _ in points)) - min_x + 2, math.ceil(max(y for _, y in points)) - min_y + 2
        count, columns = len(frames), min(10, len(frames))
        sheet = Image.new("RGBA", (width * columns, height * math.ceil(count / columns)))
        if width * height * count > 140_000_000:
            raise ValueError(f"Unbounded atlas {unit['id']}/{clip}: {width}x{height}x{count}")
        first = None
        for i, layers in enumerate(frames):
            canvas = Image.new("RGBA", (width, height))
            for _, value in layers:
                a, b, c, e, x, y = matrix(value)
                transform = (a, b, c, e, x - min_x, y - min_y)
                try:
                    inv = inverse(transform)
                except ValueError:
                    continue
                aa, bb, cc, dd, xx, yy = inv
                layer = self.images[value["image"]].transform((width, height), Image.Transform.AFFINE, (aa, bb, xx, cc, dd, yy), Image.Resampling.BICUBIC)
                if value["a"] < 1:
                    alpha = max(0, value["a"])
                    layer.putalpha(layer.getchannel("A").point(lambda value: int(value * alpha)))
                canvas.alpha_composite(layer)
            if unit["id"] == "explodeonut":
                canvas.putdata([(r, g // 4, b // 4, a) for r, g, b, a in canvas.getdata()])
            sheet.alpha_composite(canvas, ((i % columns) * width, (i // columns) * height))
            if first is None and canvas.getbbox() is not None:
                first = canvas
        relative = "sprites/" + unit["id"] + "-" + clip + ".webp"
        sheet.save(self.output / relative, lossless=True, method=4)
        anchor = (40, 80) if unit["id"] in self.plants or unit["id"] in ("mower", "poolcleaner", "roofcleaner") else (60, 115)
        if unit.get("group") == "effect" and unit["id"] not in ("mower", "poolcleaner", "roofcleaner"):
            anchor = (0, 0)
        if unit["id"] == "zomboss":
            anchor = (0, 0)
        descriptor = {"src": relative, "frameWidth": width, "frameHeight": height, "columns": columns, "frames": count,
                      "fps": d["fps"], "anchorX": anchor[0] - min_x, "anchorY": anchor[1] - min_y,
                      "sourceFrames": [start, end], "loop": not any(s in clip for s in ("death", "jump", "explode", "stomp", "pop", "grab", "throw", "charge", "enter", "leave"))}
        self.audit.append({"kind": unit["id"], "clip": clip, "source": unit["reanim"], "sourceFrames": [start, end],
                           "frameLayers": [sorted(name for name, _ in layers) for layers in frames], "size": [width, height]})
        return descriptor, first

    def build(self, previews_only=False):
        self.output.mkdir(parents=True, exist_ok=False)
        for name in ("sprites", "previews", "images", "audio"):
            (self.output / name).mkdir()
        contact = self.Image.new("RGBA", (8 * 160, math.ceil(len(self.units) / 8) * 200), "#e8e6d2")
        for i, unit in enumerate(self.units):
            animations, idle_image = {}, None
            profiles = self.profiles(unit)
            if previews_only:
                profiles = {"idle": profiles["idle"]}
            for clip, marker in profiles.items():
                descriptor, image = self.render_clip(unit, clip, marker)
                animations[clip] = descriptor
                if clip == "idle":
                    idle_image = image
            bbox = idle_image.getbbox()
            if bbox is None:
                raise ValueError(f"Empty preview {unit['id']}")
            preview = idle_image.crop(bbox)
            preview.thumbnail((160, 180))
            preview_path = "previews/" + unit["id"] + ".webp"
            preview.save(self.output / preview_path, lossless=True, method=6)
            contact.alpha_composite(preview, ((i % 8) * 160 + (160 - preview.width) // 2, (i // 8) * 200 + 190 - preview.height))
            descriptor = {"src": preview_path, "width": preview.width, "height": preview.height}
            self.manifest["previews"][unit["id"]] = descriptor
            self.manifest["sprites"][unit["id"]] = dict(animations["idle"], animations=animations, preview=descriptor, originalId=unit["originalId"], group=unit.get("group", "plant" if unit["id"] in self.plants else "zombie"))
            if unit["id"] == "zomboss":
                self.manifest["sprites"][unit["id"]]["placement"] = "scene"
            self.manifest["byUnit"][unit["id"]] = sorted({x["src"] for x in animations.values()})
            self.manifest["byPreview"][unit["id"]] = [preview_path]
            print(unit["id"], "clips", len(animations), flush=True)
        # Software-rendered pool composition preserves original water artwork.
        for kind, scene in self.manifest["maps"].items():
            name = Path(scene["background"]).stem
            image = self.Image.open(self.source / "images" / (name + ".jpg")).convert("RGBA")
            if "water" in scene:
                water = scene["water"]
                image.alpha_composite(self.Image.open(self.source / water["src"]).convert("RGBA"), (220 + water["x"], water["y"]))
                shutil.copyfile(self.source / water["src"], self.output / water["src"])
            image.save(self.output / scene["background"], lossless=True, method=6)
            mower = "poolcleaner" if kind in ("pool", "fog") else "roofcleaner" if kind in ("roof", "roofnight") else "mower"
            scene["mower"] = mower
            self.manifest["byScene"][kind] = [scene["background"]] + self.manifest["byUnit"][mower] + self.manifest["byUnit"]["sun"]
        for key, name in {"shovel": "Shovel.png", "seedpacket": "SeedPacket_Larger.png", "seedbank": "SeedBank.png", "projectile": "ProjectilePea.png", "snowProjectile": "ProjectileSnowPea.png", "shadow": "plantshadow.png"}.items():
            shutil.copyfile(self.source / "images" / name, self.output / "images" / name)
            self.manifest["images"][key] = "images/" + name
        for path in sorted((self.source / "sounds").glob("*.ogg")):
            shutil.copyfile(path, self.output / "audio" / path.name)
            self.manifest["audio"][path.stem] = "audio/" + path.name
        for key, name in {"spawn": "groan", "shoot": "throw", "hit": "splat", "explosion": "cherrybomb", "sun": "points"}.items():
            self.manifest["audio"][key] = self.manifest["audio"][name]
        augment(self)
        self.manifest.update(background=self.manifest["maps"]["day"]["background"], backgroundCrop=self.manifest["maps"]["day"]["backgroundCrop"], lawn=self.manifest["maps"]["day"]["lawn"])
        (self.output / "manifest.json").write_text(json.dumps(self.manifest, indent=2) + "\n")
        return contact


def augment(builder):
    """Add small independent pickups, terrain and projectile dependencies."""
    Image, out, source, manifest = builder.Image, builder.output, builder.source, builder.manifest
    (out / "objects").mkdir(exist_ok=True)
    for kind, name in (("goldcoin", "Coin_gold"), ("silvercoin", "Coin_silver"), ("diamond", "Diamond")):
        unit = {"id": kind, "originalId": -1, "reanim": name, "group": "effect"}
        descriptor, image = builder.render_clip(unit, "idle", None)
        preview = image.crop(image.getbbox())
        preview.thumbnail((100, 100))
        preview_path = "previews/" + kind + ".webp"
        preview.save(out / preview_path, lossless=True)
        thumb = {"src": preview_path, "width": preview.width, "height": preview.height}
        manifest["sprites"][kind] = dict(descriptor, animations={"idle": descriptor}, preview=thumb, group="effect", originalId=-1)
        manifest["previews"][kind], manifest["byPreview"][kind], manifest["byUnit"][kind] = thumb, [preview_path], [descriptor["src"]]

    # Each projectile uses the actual original image, independently addressable.
    projectile_sources = {
        "pea": ("images/ProjectilePea.png", None), "snowpea": ("images/ProjectileSnowPea.png", None),
        "cabbage": ("reanim/Cabbagepult_cabbage.png", None), "kernel": ("reanim/Cornpult_kernal.png", None),
        "butter": ("reanim/Cornpult_butter.png", None), "melon": ("reanim/Melonpult_melon.png", None),
        "wintermelon": ("reanim/WinterMelon_projectile.png", None), "cob": ("reanim/CobCannon_cob.png", None),
        "star": ("images/Projectile_star.png", None), "spike": ("images/ProjectileCactus.png", None),
        "basketball": ("reanim/Zombie_catapult_basketball.png", None), "zombiepea": ("images/ProjectilePea.png", None),
    }
    manifest["projectiles"] = {}
    for kind, (relative, _) in projectile_sources.items():
        path = builder.files[relative.casefold()]
        image = Image.open(path).convert("RGBA")
        target = "objects/projectile-" + kind + ".webp"
        image.save(out / target, lossless=True)
        manifest["projectiles"][kind] = {"src": target, "frameWidth": image.width, "frameHeight": image.height,
                                        "columns": 1, "frames": 1, "anchorX": image.width / 2, "anchorY": image.height / 2}
    manifest["projectiles"]["snow"] = manifest["projectiles"]["snowpea"]
    manifest["projectiles"]["firepea"] = manifest["sprites"]["firepea"]
    manifest["projectiles"]["fume"] = manifest["sprites"]["fumepuff"]
    mapping = {"peashooter": ["pea"], "snowpea": ["snowpea"], "repeater": ["pea"], "threepeater": ["pea"],
               "splitpea": ["pea"], "gatlingpea": ["pea"], "leftpeater": ["pea"], "cactus": ["spike"],
               "cattail": ["spike"], "cabbagepult": ["cabbage"], "kernelpult": ["kernel", "butter"],
               "melonpult": ["melon"], "wintermelon": ["wintermelon"], "cobcannon": ["cob"], "starfruit": ["star"],
               "puffshroom": ["fume"], "fumeshroom": ["fume"], "gloomshroom": ["fume"], "seashroom": ["fume"],
               "scaredyshroom": ["fume"], "catapult": ["basketball"], "peahead": ["zombiepea"], "gatlinghead": ["zombiepea"]}
    for kind, names in mapping.items():
        manifest["byUnit"][kind] = sorted(set(manifest["byUnit"][kind]) | {manifest["projectiles"][x]["src"] for x in names})
    for kind in ("torchwood",):
        manifest["byUnit"][kind] += manifest["byUnit"]["firepea"]
    for kind in ("dancing",):
        manifest["byUnit"][kind] += manifest["byUnit"]["backupdancer"]
    for kind in ("gargantuar", "redeye"):
        manifest["byUnit"][kind] += manifest["byUnit"]["imp"]
    manifest["byUnit"]["zomboss"] += manifest["byUnit"]["bossfireball"] + manifest["byUnit"]["bossiceball"]

    terrain = {"grave": "images/Tombstones.jpg", "ladder": "reanim/Zombie_ladder_1.png", "crater": "images/crater.png",
               "ice": "images/ice.png", "fog": "images/fog_software.png", "butter": "reanim/Cornpult_butter_splat.png"}
    manifest["objects"] = {}
    for key, relative in terrain.items():
        path = builder.files[relative.casefold()]
        image = Image.open(path).convert("RGBA")
        if key == "grave":
            image.putalpha(Image.open(source / "images/Tombstones_.png").convert("L"))
            image = image.crop((0, 0, 86, 91))
        if key == "fog":
            image = image.crop((0, 0, image.width // 3, image.height))
        target = "objects/" + key + ".webp"
        image.save(out / target, lossless=True)
        manifest["images"][key] = target
        manifest["objects"][key] = {"src": target, "width": image.width, "height": image.height}
    for kind, scene in manifest["maps"].items():
        for pickup in ("goldcoin", "silvercoin", "diamond"):
            manifest["byScene"][kind] += manifest["byUnit"][pickup]
        if kind == "night":
            manifest["byScene"][kind].append(manifest["images"]["grave"])
        if kind == "fog":
            manifest["byScene"][kind].append(manifest["images"]["fog"])
        manifest["byScene"][kind] += [manifest["images"][x] for x in ("ladder", "crater", "ice", "butter")]
    for event, original in {"magnet": "magnetshroom", "swallow": "gulp", "kelp-grab": "slurp", "drown": "zombiesplash",
                            "squash-jump": "squash_hmm", "fog-clear": "blover", "freeze": "frozen", "umbrella": "throw2",
                            "torch": "firepea", "hypnotize": "mindcontrolled", "garlic": "yuck", "collect": "points"}.items():
        manifest["audio"][event] = manifest["audio"][original]
    aliases = {"snorkel": {"submerged": "swim", "surface": "uptoeat"}, "digger": {"digging": "drill", "rising": "landing"},
               "balloon": {"fly": "idle", "flying": "idle"}, "pole": {"vault": "jump"}, "newspaper": {"rage_attack": "eat_nopaper"}}
    for kind, names in aliases.items():
        animations = manifest["sprites"][kind]["animations"]
        for alias, original in names.items():
            if original in animations:
                animations[alias] = animations[original]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--catalogue", type=Path, default=Path("docs/research/pvz/evidence/full-catalog.json"))
    parser.add_argument("--contact", type=Path, required=True)
    parser.add_argument("--audit", type=Path, required=True)
    args = parser.parse_args()
    builder = Builder(args.source, args.output, json.loads(args.catalogue.read_text()))
    contact = builder.build()
    contact.save(args.contact)
    args.audit.write_text(json.dumps({"version": "classic-v2", "units": builder.audit}, indent=2) + "\n")
    files = [{"path": str(path.relative_to(args.output)), "bytes": path.stat().st_size, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
             for path in sorted(args.output.rglob("*")) if path.is_file()]
    (args.output.parent.parent.parent / "manifest-v2.json").write_text(json.dumps({"version": "classic-v2", "files": files}, indent=2) + "\n")


if __name__ == "__main__":
    main()
