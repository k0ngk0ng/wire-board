"""Deliver complete composed poses as shared original images plus gzip matrices."""
import argparse
import gzip
import hashlib
import json
import math
from pathlib import Path
import re

from classic_v2 import Builder, augment
from render_classic import clip_range, matrix


def canvas_matrix(value):
    a, b, c, d, x, y = value
    return [a, c, b, d, x, y]


def asset_stem(value):
    raw = value.removeprefix("IMAGE_REANIM_").lower()
    if re.fullmatch(r"[a-z0-9._-]+", raw):
        return raw
    slug = re.sub(r"[^a-z0-9._-]+", "-", raw).strip("-.") or "layer"
    return slug + "-" + hashlib.sha256(value.encode()).hexdigest()[:10]


def layer_bounds(value, width, height):
    a, b, c, d, x, y = value
    corners = [(a * px + c * py + x, b * px + d * py + y) for px, py in ((0, 0), (width, 0), (0, height), (width, height))]
    return min(p[0] for p in corners), min(p[1] for p in corners), max(p[0] for p in corners), max(p[1] for p in corners)


def semantic_aliases(kind):
    result = {"walking": "walk", "eating": "attack", "vaulting": "jump", "flying": "idle", "escaping": "walk",
              "jumping": "jumpdown", "asleep": "sleep", "scared": "scaredidle", "small": "idle", "grown": "bigidle",
              "biting": "bite", "digesting": "chew", "recovering": "swallow", "charging": "charge", "ready": "idle",
              "high": "idlehigh", "low": "idle", "glowing": "idle", "reflecting": "block"}
    result.update({
        "snorkel": {"diving": "swim", "submerged": "swim", "surface": "uptoeat", "entering-water": "jumpinpool", "snorkel-surfacing": "uptoeat", "snorkel-submerging": "uptoeat"},
        "dolphin": {"entering-water": "jumpinpool"},
        "digger": {"digging": "drill", "emerging": "landing", "rising": "landing"},
        "bungee": {"descending": "drop", "grabbing": "grab", "rising": "raise", "bungee-grab": "grab"},
        "dancing": {"dancing": "moonwalk", "summoning": "armraise"},
        "backupdancer": {"dancing": "armraise", "summoning": "armraise"},
        "pogo": {"bouncing": "pogo"}, "bobsled": {},
        "jackinthebox": {"jackbox-popping": "pop"},
        "zomboni": {"driving": "idle"}, "catapult": {"driving": "walk", "catapult-shooting": "shoot"},
        "gargantuar": {"throwing": "throw", "smashing": "smash"}, "redeye": {"throwing": "throw", "smashing": "smash"},
        "squashhead": {}, "ladder": {"ladder-placing": "placeladder", "climbing": "walk"},
        "gravebuster": {"eating": "idle"},
        "chomper": {"biting": "attack"}, "tanglekelp": {"grabbing": "attack"}, "umbrellaleaf": {"reflecting": "attack"},
        "imitater": {"imitater-explode": "attack"},
        "zomboss": {"boss-enter": "enter", "boss-idle": "idle", "boss-head": "head_idle", "boss-summon": "spawn_1", "boss-stomp": "stomp_1", "boss-rv": "RV_1", "boss-bungee": "bungee_1_enter",
                    "boss-head-enter": "head_enter", "boss-head-idle-before": "head_idle", "boss-head-spit": "head_attack_1", "boss-head-idle-after": "head_idle", "boss-head-leave": "head_leave"},
    }.get(kind, {}))
    return result


def looping_marker(marker):
    return not any(key in (marker or "") for key in ("death", "jump", "explode", "stomp", "pop", "grab", "throw", "charge", "enter", "leave", "drop", "raise", "landing", "uptoeat", "placeladder", "smash", "shoot", "spawn_", "bite", "swallow", "block", "rise", "lower", "grow", "form", "crumble"))


def runtime_fps(kind, clip, declared):
    if kind in ("bossfireball", "bossiceball") and clip in ("idle", "role"):
        return 2
    if kind == "imitater" and clip in ("attack", "explode"):
        return 26
    if kind == "snorkel":
        if clip == "jumpinpool":
            return 16
        if clip == "uptoeat":
            return 24
    if kind == "bungee" and clip in ("attack", "grab"):
        return 24
    if kind == "bungee" and clip in ("raise",):
        return 36
    if kind in ("gargantuar", "redeye"):
        if clip == "throw":
            return 24
        if clip in ("attack", "smash"):
            return 16
    if kind == "pole" and clip == "jump":
        return 24
    if kind == "dolphin":
        if clip == "jumpinpool":
            return 16
        if clip in ("dolphinjump", "jump"):
            return 10
    if kind == "ladder" and clip == "placeladder":
        return 24
    if kind == "zomboss":
        if clip == "RV_1":
            return 16
        if any(key in clip for key in ("spawn_", "head_attack_", "stomp_", "bungee_", "head_idle")):
            return 12
        if clip == "idle":
            return 6
    return declared


def convert(source, output, catalogue, audit_path):
    builder = Builder(source, output, catalogue)
    if not (output / "manifest.json").exists():
        contact = builder.build(previews_only=True)
        (audit_path.parent).mkdir(parents=True, exist_ok=True)
        contact.save(audit_path.with_suffix(".png"))
    builder.manifest = json.loads((output / "manifest.json").read_text())
    augment(builder)
    manifest = builder.manifest
    units = builder.units + [{"id": key, "originalId": -1, "reanim": name, "group": "effect"}
                             for key, name in (("goldcoin", "Coin_gold"), ("silvercoin", "Coin_silver"), ("diamond", "Diamond"))]
    (output / "native").mkdir(exist_ok=True)
    (output / "layers").mkdir(exist_ok=True)
    original_urls = {kind: list(urls) for kind, urls in manifest["byUnit"].items()}
    owner = {url: "silvercoin" if kind == "coin" else kind for kind, value in manifest["sprites"].items() for url in {x["src"] for x in value["animations"].values()}}
    image_files, own_urls, audit, renamed = {}, {}, [], {}
    for unit in units:
        kind, d = unit["id"], builder.definition(unit["reanim"])
        payload = {"version": 1, "kind": kind, "matrixOrder": "canvas-a-b-c-d-e-f", "clips": {}}
        used, all_bounds = set(), []
        for clip, marker in builder.profiles(unit).items():
            start, end = clip_range(d, marker) if marker is not None else (0, len(d["tracks"][0]["frames"]) - 1)
            frames, bounds = [], []
            for index in range(start, end + 1):
                layers = []
                for name, value in builder.compose(unit, clip, index):
                    symbol = value["image"]
                    image_key = (kind + "-" if kind == "explodeonut" else "") + symbol
                    if image_key not in image_files:
                        image = builder.images[symbol]
                        if kind == "explodeonut":
                            image = image.copy()
                            r, g, b, a = image.split()
                            image = builder.Image.merge("RGBA", (r, g.point(lambda v: v * 64 // 255), b.point(lambda v: v * 64 // 255), a))
                        relative = "layers/" + asset_stem(image_key) + ".webp"
                        renamed["layers/" + image_key.removeprefix("IMAGE_REANIM_").lower() + ".webp"] = relative
                        image.save(output / relative, lossless=True, method=6)
                        image_files[image_key] = (relative, image.width, image.height)
                    relative, width, height = image_files[image_key]
                    transform = canvas_matrix(matrix(value))
                    bounds.append(layer_bounds(transform, width, height))
                    used.add(relative)
                    layers.append({"src": relative, "m": transform, "alpha": value["a"], "sourceLayer": name})
                frames.append(layers)
            if not bounds:
                raise ValueError(f"No original layers for {kind}/{clip}")
            rect = {"x": math.floor(min(x[0] for x in bounds)) - 2, "y": math.floor(min(x[1] for x in bounds)) - 2}
            rect.update(width=math.ceil(max(x[2] for x in bounds)) - rect["x"] + 2,
                        height=math.ceil(max(x[3] for x in bounds)) - rect["y"] + 2)
            loop = looping_marker(marker)
            payload["clips"][clip] = {"fps": runtime_fps(kind, clip, d["fps"]), "loop": loop, "frames": frames, "bounds": rect,
                                     "sourceFrames": [start, end]}
            all_bounds.append(rect)
        for alias, original in semantic_aliases(kind).items():
            if original in payload["clips"]:
                payload["clips"][alias] = payload["clips"][original]
        if kind == "snorkel" and "snorkel-submerging" in payload["clips"]:
            original = payload["clips"]["uptoeat"]
            payload["clips"]["snorkel-submerging"] = dict(original, frames=list(reversed(original["frames"])), reverseSource=True)
        # Boss-summoning data is state-dependent; retain every source row separately.
        bounds = {"x": min(x["x"] for x in all_bounds), "y": min(x["y"] for x in all_bounds)}
        bounds.update(width=max(x["x"] + x["width"] for x in all_bounds) - bounds["x"],
                      height=max(x["y"] + x["height"] for x in all_bounds) - bounds["y"])
        anchor = (40, 80) if kind in builder.plants or kind in ("mower", "poolcleaner", "roofcleaner") else (60, 115)
        if unit.get("group") == "effect" and kind not in ("mower", "poolcleaner", "roofcleaner") or kind == "zomboss":
            anchor = (0, 0)
        payload.update(anchorX=anchor[0], anchorY=anchor[1], bounds=bounds)
        relative = "native/" + kind + ".pvzr"
        encoded = json.dumps(payload, separators=(",", ":")).encode()
        (output / relative).write_bytes(gzip.compress(encoded, compresslevel=9, mtime=0))
        animations = {key: {"src": relative, "native": True, "frames": len(value["frames"]), "fps": value["fps"], "loop": value["loop"], "bounds": value["bounds"]}
                      for key, value in payload["clips"].items()}
        old = manifest["sprites"][kind]
        manifest["sprites"][kind] = {"src": relative, "native": {"src": relative, "compression": "gzip", "format": "pvz-reanimation-json-v1", "images": sorted(used)},
                                       "anchorX": anchor[0], "anchorY": anchor[1], "bounds": bounds,
                                       "animations": animations, "preview": old["preview"], "originalId": old["originalId"], "group": old["group"]}
        if kind == "zomboss":
            manifest["sprites"][kind]["placement"] = "scene"
        if kind == "bungee":
            manifest["sprites"][kind]["attachments"] = {"plantFoot": {"x": -20, "y": -5}, "spikeFoot": {"x": -20, "y": -39}, "roofWestAdjustmentY": 10, "space": "relative-to-native-foot", "sourceEvidenceID": "ev_ffd7e3929c2a6a7306d820e1c3163d7b289d01b86352fe226456c9841e139d96"}
        own_urls[kind] = {relative, *used}
        audit.append({"kind": kind, "clips": len(payload["clips"]), "images": len(used), "jsonBytes": len(encoded), "gzipBytes": (output / relative).stat().st_size,
                      "bounds": bounds, "anchor": anchor})
        print(kind, "native", (output / relative).stat().st_size, "images", len(used), flush=True)

    def expand(urls, seen=None):
        result = set()
        for url in urls:
            kind = owner.get(url)
            if kind is None and url.startswith("native/"):
                candidate = Path(url).name.split(".")[0]
                if candidate in own_urls:
                    kind = candidate
            if kind in own_urls:
                result.update(own_urls[kind])
            else:
                result.add(renamed.get(url, url))
        return result

    for kind, urls in original_urls.items():
        manifest["byUnit"][kind] = sorted(expand(urls) | own_urls.get(kind, set()))
    for scene, urls in manifest["byScene"].items():
        manifest["byScene"][scene] = sorted(expand(urls))
    for key, kind in (("firepea", "firepea"), ("fume", "fumepuff")):
        manifest["projectiles"][key] = manifest["sprites"][kind]
    for alias, original in {"icepea": "snowpea", "spore": "fume", "needle": "spike"}.items():
        manifest["projectiles"][alias] = manifest["projectiles"][original]
    manifest["projectiles"]["bossfire"] = manifest["sprites"]["bossfireball"]
    manifest["projectiles"]["bossice"] = manifest["sprites"]["bossiceball"]
    manifest["sprites"]["coin"] = manifest["sprites"]["silvercoin"]
    manifest["byUnit"]["coin"] = manifest["byUnit"]["silvercoin"]
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    # These unpublished rasters are this builder's redundant intermediate output.
    referenced = set().union(*(set(x) for x in manifest["byUnit"].values()), *(set(x) for x in manifest["byScene"].values()))
    deleted = []
    for folder in ("sprites", "native", "layers"):
        for path in (output / folder).iterdir():
            if path.is_file() and str(path.relative_to(output)) not in referenced:
                deleted.append(str(path.relative_to(output)))
                path.unlink()
    audit_path.write_text(json.dumps({"version": "classic-v2", "matrixOrder": "canvas-a-b-c-d-e-f", "units": audit, "removedUnpublishedIntermediateRasters": deleted}, indent=2) + "\n")
    files = [{"path": str(path.relative_to(output)), "bytes": path.stat().st_size, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
             for path in sorted(output.rglob("*")) if path.is_file()]
    (output.parents[2] / "manifest-v2.json").write_text(json.dumps({"version": "classic-v2", "files": files}, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--catalogue", type=Path, default=Path("docs/research/pvz/evidence/full-catalog.json"))
    parser.add_argument("--audit", type=Path, required=True)
    args = parser.parse_args()
    convert(args.source, args.output, json.loads(args.catalogue.read_text()), args.audit)


if __name__ == "__main__":
    main()
