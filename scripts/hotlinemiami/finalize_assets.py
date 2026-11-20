import hashlib
import io
import json
import shutil
import subprocess
import sys
import wave
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / ".local/hotlinemiami/assets"
RESEARCH = ROOT / ".local/hotlinemiami/research"
sys.path.insert(0, str(ROOT / ".local/hotlinemiami/tools/python"))
sys.path.insert(0, str(ROOT / "scripts/hotlinemiami"))
from PIL import Image, ImageDraw
from build_maps import METADATA, ORIGINAL, read_object_data, read_rooms, read_sprite_data, recover_maps
from extract_assets import read_wad


def character_definitions():
    return [
        {"id": "jacket", "name": "Jacket", "sprite": "sprPWalkUnarmed", "portraitSprite": "sprMasksBig", "portraitFrame": 0, "overlay": "sprRoosterMask", "overlayFrame": 1, "poses": {"pistol": "sprPWalkSilencer", "uzi": "sprPWalkUzi", "m16": "sprPWalkM16", "shotgun": "sprPWalkShotgun", "knife": "sprPWalkKnife", "bat": "sprPWalkBat", "katana": "sprPWalkSword"}},
        {"id": "biker", "name": "Biker", "sprite": "sprBoss2Walk", "portraitSprite": "sprBikerHurt", "portraitFrame": 0},
        {"id": "bodyguard", "name": "保镖", "sprite": "bodyguard-walk", "sourceSprite": "sprBodyguardAttack", "portraitSprite": "sprBodyguardFace", "portraitFrame": 0, "dead": "sprBodyguardDead", "adaptation": "原版保镖上身与专属腿部组合为八帧通用行走，未补绘像素"},
        {"id": "father", "name": "教父", "sprite": "father-walk", "sourceSprite": "sprBoss4Idle", "portraitSprite": "sprBoss4Face", "portraitFrame": 0, "attack": "sprBoss4Shoot1", "dead": "sprBoss4Die", "adaptation": "原版教父上身与通用腿部组合为八帧通用行走，未补绘像素"},
        {"id": "richter", "name": "Richter", "sprite": "sprAssassinWalk", "portraitSprite": "sprAssassinRatFace", "portraitFrame": 0, "overlay": "sprRatMask_dupe1", "overlayFrame": 1, "dead": "sprAssassinDead"},
    ]


def weapon_definitions():
    return [{"id": identity, "name": name, "sourceFrame": frame} for identity, name, frame in [("pistol", "手枪", 7), ("uzi", "Uzi", 9), ("m16", "M16", 0), ("shotgun", "霰弹枪", 1), ("knife", "小刀", 3), ("bat", "棒球棒", 4), ("katana", "武士刀", 8)]]


def load_source_manifest():
    RESEARCH.mkdir(parents=True, exist_ok=True)
    current = json.loads((ASSETS / "manifest.json").read_text())
    source_path = RESEARCH / "full-atlas-manifest.json"
    if len(current.get("images", {})) > 100:
        source_path.write_text(json.dumps(current, ensure_ascii=False, separators=(",", ":")) + "\n")
        return current
    return json.loads(source_path.read_text())


def register_image(manifest, identity, image):
    relative = "runtime/" + identity + ".png"
    target = ASSETS / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    image.save(target, format="PNG", compress_level=9)
    payload = target.read_bytes()
    manifest["images"][identity] = {"file": relative, "width": image.width, "height": image.height, "bytes": len(payload), "sha256": hashlib.sha256(payload).hexdigest()}


def source_frame(source, name, frame_index=0):
    sprite = source["sprites"][name]
    frame = sprite["frames"][frame_index]
    image = Image.open(ASSETS / source["images"][sprite["image"]]["file"]).convert("RGBA")
    return image.crop((frame["x"], frame["y"], frame["x"] + frame["w"], frame["y"] + frame["h"]))


def pack_sprites(source, target, selected):
    sheet = Image.new("RGBA", (1024, 4096))
    x = y = line_height = 0
    for name in sorted(selected):
        original = source["sprites"][name]
        sprite = {key: value for key, value in original.items() if key not in ["frames", "image"]}
        sprite.update({"image": "sprites", "frames": []})
        for index, frame in enumerate(original["frames"]):
            if x + frame["w"] > sheet.width:
                x = 0
                y += line_height
                line_height = 0
            if y + frame["h"] > sheet.height:
                raise ValueError("runtime sprite atlas exceeds budget")
            sheet.alpha_composite(source_frame(source, name, index), (x, y))
            sprite["frames"].append({**frame, "x": x, "y": y})
            x += frame["w"]
            line_height = max(line_height, frame["h"])
        target["sprites"][name] = sprite
    register_image(target, "sprites", sheet.crop((0, 0, 1024, y + line_height)))


def add_adapted_walks(source, manifest):
    definitions = [("bodyguard-walk", "sprBodyguardAttack", 3, "sprBodyguardLegs"), ("father-walk", "sprBoss4Idle", 0, "sprLegs")]
    for identity, upper_name, upper_frame, legs_name in definitions:
        upper = source_frame(source, upper_name, upper_frame)
        upper_metadata = source["sprites"][upper_name]
        origin_x = upper_metadata["originX"]
        origin_y = upper_metadata["originY"]
        upper = upper.crop((origin_x - 16, origin_y - 16, origin_x + 16, origin_y + 16))
        sheet = Image.new("RGBA", (256, 32))
        legs_metadata = source["sprites"][legs_name]
        frames = []
        for index in range(8):
            frame = Image.new("RGBA", (32, 32))
            legs = source_frame(source, legs_name, index)
            frame.alpha_composite(legs, (16 - legs_metadata["originX"], 16 - legs_metadata["originY"]))
            frame.alpha_composite(upper, (0, 0))
            sheet.alpha_composite(frame, (index * 32, 0))
            frames.append({"x": index * 32, "y": 0, "w": 32, "h": 32, "originX": 16, "originY": 16})
        register_image(manifest, identity, sheet)
        manifest["sprites"][identity] = {"image": identity, "width": 32, "height": 32, "originX": 16, "originY": 16, "frames": frames, "adaptation": {"upper": upper_name, "upperFrame": upper_frame, "legs": legs_name, "operation": "unchanged source pixels cropped and composited"}}


def map_background(source, room, objects, sprites):
    image = Image.new("RGBA", (room["width"], room["height"]), (10, 8, 18, 255))
    backgrounds = {0: source_frame(source, "background0"), 1: source_frame(source, "background1")}
    foreground = []
    for x, y, background_id, source_x, source_y, width, height, _ in sorted(room["tiles"], key=lambda tile: -tile[7]):
        if background_id not in backgrounds:
            raise ValueError("selected room uses unsupported tile palette")
        tile = backgrounds[background_id].crop((source_x, source_y, source_x + width, source_y + height))
        if background_id == 1:
            foreground.append((tile, x, y))
        else:
            image.alpha_composite(tile, (x, y))
    for instance, object_id, x, y, _ in sorted(room["objects"], key=lambda item: -objects[item[1]][2]):
        object_data = objects[object_id]
        sprite = sprites.get(object_data[1])
        if not sprite or not sprite["name"].startswith("Enviroment/") or object_data[6] == 0:
            continue
        if any(value in sprite["name"] for value in ["/Walls/sprDoor", "GlassPanel", "/Walls/sprWindow"]):
            continue
        name = sprite["name"].split("/")[-1]
        if name not in source["sprites"]:
            raise ValueError("map sprite absent: " + name)
        frame = source_frame(source, name)
        fields = sprite["fields"]
        image.alpha_composite(frame, (x - fields[3], y - fields[4]))
    for tile, x, y in foreground:
        image.alpha_composite(tile, (x, y))
    return image


def fixed_rules(arena):
    rules = {key: value for key, value in arena.items() if key not in ["walls", "doors", "spawns", "weaponSpawns"]}
    rules["width"] *= 256
    rules["height"] *= 256
    rules["coordinateScale"] = 256
    for key in ["walls", "doors", "spawns", "weaponSpawns"]:
        rules[key] = [{field: value * 256 if field in ["x", "y", "w", "h"] else value for field, value in item.items()} for item in arena[key]]
    return rules


def add_audio(manifest):
    names = {"pistol": "sndSilencer.wav", "uzi": "sndUzi.wav", "m16": "sndM16.wav", "shotgun": "sndShotgun.wav", "knife": "sndSwing1.wav", "bat": "sndSwing2.wav", "katana": "sndSwing1.wav", "pickup": "sndPickupWeapon.wav", "throw": "sndThrow.wav", "down": "sndGrunt.wav", "kill": "sndHit.wav", "door": "sndDoorOpen.wav"}
    entries = read_wad((ORIGINAL / "selected/HotlineMiami_GL.wad").read_bytes())
    manifest["sounds"] = {}
    ffmpeg = shutil.which("ffmpeg")
    ffprobe = shutil.which("ffprobe")
    if not ffmpeg or not ffprobe:
        raise ValueError("ffmpeg and ffprobe are required for verified lossless audio")
    for identity, name in names.items():
        matches = [(key, payload) for key, payload in entries.items() if key.endswith("/" + name)]
        if not matches:
            continue
        source, payload = matches[0]
        with wave.open(io.BytesIO(payload), "rb") as wav:
            channels = wav.getnchannels()
            sample_rate = wav.getframerate()
            sample_width = wav.getsampwidth()
            frame_count = wav.getnframes()
            pcm = wav.readframes(frame_count)
        source_pcm_digest = hashlib.sha256(pcm).hexdigest()
        extension = ".wav"
        original_digest = hashlib.sha256(payload).hexdigest()
        if sample_width >= 2:
            result = subprocess.run([ffmpeg, "-v", "error", "-i", "pipe:0", "-c:a", "flac", "-f", "flac", "pipe:1"], input=payload, capture_output=True, check=True)
            if len(result.stdout) < len(payload):
                payload = result.stdout
                extension = ".flac"
        codec = {1: "pcm_u8", 2: "pcm_s16le", 3: "pcm_s24le", 4: "pcm_s32le"}.get(sample_width)
        if not codec:
            raise ValueError("unsupported PCM bit depth")
        raw_format = {1: "u8", 2: "s16le", 3: "s24le", 4: "s32le"}[sample_width]
        decoded = subprocess.run([ffmpeg, "-v", "error", "-i", "pipe:0", "-c:a", codec, "-f", raw_format, "pipe:1"], input=payload, capture_output=True, check=True).stdout
        probe = json.loads(subprocess.run([ffprobe, "-v", "error", "-show_streams", "-select_streams", "a:0", "-of", "json", "pipe:0"], input=payload, capture_output=True, check=True).stdout)["streams"][0]
        if decoded != pcm or int(probe["sample_rate"]) != sample_rate or probe["channels"] != channels:
            raise ValueError("lossless audio roundtrip mismatch")
        relative = "runtime/" + identity + extension
        (ASSETS / relative).write_bytes(payload)
        manifest["sounds"][identity] = {"file": relative, "bytes": len(payload), "sha256": hashlib.sha256(payload).hexdigest(), "source": source, "sourceSHA256": original_digest, "sourcePCMsha256": source_pcm_digest, "decodedPCMsha256": hashlib.sha256(decoded).hexdigest(), "sampleRate": sample_rate, "channels": channels, "bitDepth": sample_width * 8, "frames": frame_count}


def add_music(manifest):
    installer = ORIGINAL / "gog_hotline_miami_2.0.0.4.sh"
    source = "data/noarch/game/Hydrogen.ogg"
    with zipfile.ZipFile(installer) as archive:
        payload = archive.read(source)
    relative = "runtime/music.ogg"
    (ASSETS / relative).write_bytes(payload)
    manifest["music"] = {"file": relative, "bytes": len(payload), "sha256": hashlib.sha256(payload).hexdigest(), "mime": "audio/ogg", "source": source, "encoding": "original unchanged Ogg stream"}


def contact_sheet(manifest, source):
    sheet = Image.new("RGB", (1200, 900), (24, 19, 30))
    draw = ImageDraw.Draw(sheet)
    for index, arena in enumerate(manifest["maps"]):
        preview = Image.open(ASSETS / manifest["images"][arena["preview"]]["file"]).convert("RGBA")
        preview.thumbnail((380, 280))
        sheet.paste(preview, (index * 400 + 10, 28), preview)
        draw.text((index * 400 + 10, 8), arena["id"] + " / native room " + str(arena["rules"]["sourceRoomId"]), fill="white")
    for index, character in enumerate(manifest["characters"]):
        portrait = Image.open(ASSETS / manifest["images"][character["portrait"]]["file"]).convert("RGBA")
        portrait.thumbnail((180, 150))
        sheet.paste(portrait, (index * 240 + 20, 330), portrait)
        if character["sprite"] in source["sprites"]:
            sprite = source_frame(source, character["sprite"])
        else:
            definition = manifest["sprites"][character["sprite"]]
            frame = definition["frames"][0]
            texture = Image.open(ASSETS / manifest["images"][definition["image"]]["file"])
            sprite = texture.crop((frame["x"], frame["y"], frame["x"] + frame["w"], frame["y"] + frame["h"]))
        if character.get("overlay"):
            overlay = source_frame(source, character["overlay"], character.get("overlayFrame", 0))
            sprite.alpha_composite(overlay, ((sprite.width - overlay.width) // 2, (sprite.height - overlay.height) // 2))
        sprite = sprite.resize((sprite.width * 3, sprite.height * 3), Image.Resampling.NEAREST)
        sheet.paste(sprite, (index * 240 + 20, 520), sprite)
        draw.text((index * 240 + 20, 310), character["id"], fill="white")
    for index, weapon in enumerate(manifest["weapons"]):
        icon = Image.open(ASSETS / manifest["images"][weapon["icon"]]["file"]).convert("RGBA")
        sheet.paste(icon, (index * 170 + 20, 765), icon)
        draw.text((index * 170 + 20, 740), weapon["id"], fill="white")
    output = ROOT / ".local/hotlinemiami/verification/assets/contact-sheet.png"
    output.parent.mkdir(parents=True, exist_ok=True)
    sheet.save(output)


def create_runtime_assets():
    source = load_source_manifest()
    manifest = {"version": 1, "sourceSHA256": source["sourceSHA256"], "images": {}, "sprites": {}, "characters": [], "weapons": [], "maps": []}
    characters = character_definitions()
    selected = {character.get("sourceSprite", character["sprite"]) for character in characters} | {character["overlay"] for character in characters if character.get("overlay")}
    selected.update(name for name in ["sprPWalkSilencer", "sprPWalkUzi", "sprPWalkM16", "sprPWalkShotgun", "sprPWalkKnife", "sprPWalkBat", "sprPWalkSword", "sprPAttackSilencer", "sprPAttackUzi", "sprPAttackM16", "sprPAttackShotgun", "sprPAttackKnife", "sprPAttackBat", "sprPAttackSword", "sprPFrontShotgun", "sprAssassinDead", "sprBodyguardDead", "sprBoss1Dead", "sprBoss4Die", "sprBoss4Shoot1", "sprWeapons", "sprLegs", "sprBodyguardLegs"] if name in source["sprites"])
    pack_sprites(source, manifest, selected)
    add_adapted_walks(source, manifest)
    for character in characters:
        portrait = source_frame(source, character["portraitSprite"], character["portraitFrame"])
        bounds = portrait.getbbox()
        if bounds:
            portrait = portrait.crop(bounds)
        ratio = max(1, 120 // max(portrait.width, portrait.height))
        portrait = portrait.resize((portrait.width * ratio, portrait.height * ratio), Image.Resampling.NEAREST)
        portrait_id = "portrait-" + character["id"]
        register_image(manifest, portrait_id, portrait)
        manifest["characters"].append({**character, "portrait": portrait_id})
    for weapon in weapon_definitions():
        icon = source_frame(source, "sprWeapons", weapon["sourceFrame"])
        bounds = icon.getbbox()
        if not bounds:
            raise ValueError("selected weapon icon is empty")
        icon = icon.crop(bounds)
        ratio = max(1, 56 // max(icon.width, icon.height))
        icon = icon.resize((icon.width * ratio, icon.height * ratio), Image.Resampling.NEAREST)
        padded = Image.new("RGBA", (64, 64))
        padded.alpha_composite(icon, ((64 - icon.width) // 2, (64 - icon.height) // 2))
        identity = "weapon-" + weapon["id"]
        register_image(manifest, identity, padded)
        manifest["sprites"][identity] = {"image": identity, "frames": [{"x": 0, "y": 0, "w": 64, "h": 64, "originX": 32, "originY": 32}], "width": 64, "height": 64, "originX": 32, "originY": 32}
        manifest["weapons"].append({**weapon, "sprite": identity, "icon": identity})
    rooms = read_rooms((METADATA / "hotline_rooms.bin").read_bytes())
    objects = read_object_data()
    sprites = read_sprite_data()
    for arena in recover_maps():
        image = map_background(source, rooms[arena["sourceRoomId"]], objects, sprites)
        identity = "map-" + arena["id"]
        preview_id = "preview-" + arena["id"]
        register_image(manifest, identity, image)
        preview = image.resize((320, image.height * 320 // image.width), Image.Resampling.NEAREST)
        register_image(manifest, preview_id, preview)
        manifest["maps"].append({"id": arena["id"], "name": arena["name"], "width": image.width, "height": image.height, "image": identity, "preview": preview_id, "rules": fixed_rules(arena)})
    add_audio(manifest)
    add_music(manifest)
    (ASSETS / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, separators=(",", ":")) + "\n")
    contact_sheet(manifest, source)
    return manifest


if __name__ == "__main__":
    manifest = create_runtime_assets()
    print(json.dumps({"maps": len(manifest["maps"]), "characters": len(manifest["characters"]), "weapons": len(manifest["weapons"]), "images": len(manifest["images"]), "bytes": sum(image["bytes"] for image in manifest["images"].values())}, ensure_ascii=False))
