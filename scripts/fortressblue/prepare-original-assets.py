#!/usr/bin/env python3
"""Build the Fortress 2 BLUE media pack from the locally acquired web client.

The source client is not committed. This script reads the extracted `.i16`
files, decodes the original LH5 + per-row span sprite codec, writes immutable
WebP/OGG media under the gitignored public directory, and emits the committed
map catalog / asset manifest / provenance records.

Frozen v2 builder. Existing v2/v3 data is never overwritten by main.
Restore published media with fetch-assets.py; generate current tank animations
with prepare-animation-assets.py. verify-original-source.py imports these
legacy codec/build functions into a repository-local temporary directory.
"""
from __future__ import annotations

import base64
import hashlib
import io
import json
import math
import os
import shutil
import struct
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
LOCAL = ROOT / ".local/fortressblue-original"
OUT = ROOT / "games/fortressblue/assets/public/fortressblue/v2"
MAPS_DIR = ROOT / "games/fortressblue/maps"
ASSETS = ROOT / "games/fortressblue/assets"
TANKS_JSON = ROOT / "games/fortressblue/data/tanks.json"
sys.path.insert(0, str(LOCAL / "tools/py"))
from PIL import Image  # noqa: E402
from lhafile.lhafile import LhaInfo, lzhlib  # noqa: E402

TRANSPARENT = 0xF81F
CELL = 4  # collision cell size in source pixels
WORLD_HEIGHT = 960

# Cocos lobby map metadata: Cocos id -> (English/Chinese name, game_map_id).
MAP_ROWS = [
    (1, "热血之都", "The legend of Sabuk", 16),
    (2, "玩具工厂", "The Toy Factory", 17),
    (3, "风车森林", "The windmill Forest", 1),
    (4, "蔚蓝云海", "The Blue Sea of clouds", 2),
    (5, "梦幻大陆", "The fantastic land", 3),
    (6, "钢铁工厂", "The factory", 4),
    (7, "海盗码头", "The Pirate wharf", 5),
    (8, "奇幻仙境", "The heaven", 6),
    (9, "天空之城", "The sky", 7),
    (10, "独木桥", "The single log bridge", 8),
    (11, "古老埃及", "The sphinx", 9),
    (12, "神圣殿堂", "The templer", 10),
    (13, "神秘洞穴", "The cave", 11),
    (14, "草原坟场", "The Grassland Cemetery", 12),
    (15, "冰封溪谷", "The valley of city", 13),
    (16, "海盗舰队", "The pirate ship", 14),
    (17, "风华绝代", "The beauty", 15),
]

TANK_NAMES = {
    1: "大碗", 2: "叉叉", 3: "老土", 4: "甲壳虫", 5: "公爵", 6: "飞火",
    7: "流星", 8: "小妖", 9: "雷霆", 10: "闪灵", 11: "水鬼", 12: "海怪",
    13: "暗影", 14: "萨满",
}
# Values copied from the official Cocos configs/tanks.json (2026-10-20).
TANK_STATS = {
    1: dict(hp=950, defense=100, speed=50, primary=300, special=150, angle=90, range=45, skills=[3]),
    2: dict(hp=940, defense=99, speed=57, primary=330, special=100, angle=80, range=47, skills=[2]),
    3: dict(hp=920, defense=89, speed=48, primary=140, special=420, angle=45, range=43, skills=[]),
    4: dict(hp=1000, defense=118, speed=60, primary=230, special=150, angle=40, range=47, skills=[]),
    5: dict(hp=1000, defense=123, speed=60, primary=240, special=153, angle=40, range=49, skills=[2]),
    6: dict(hp=1100, defense=117, speed=57, primary=120, special=90, angle=45, range=60, skills=[]),
    7: dict(hp=1050, defense=115, speed=55, primary=175, special=60, angle=50, range=53, skills=[]),
    8: dict(hp=990, defense=95, speed=80, primary=220, special=99, angle=36, range=50, skills=[4]),
    9: dict(hp=1150, defense=115, speed=56, primary=220, special=320, angle=40, range=48, skills=[1]),
    10: dict(hp=970, defense=91, speed=70, primary=200, special=270, angle=55, range=60, skills=[4]),
    11: dict(hp=1000, defense=105, speed=54, primary=230, special=190, angle=50, range=47, skills=[2]),
    12: dict(hp=985, defense=92, speed=80, primary=220, special=166, angle=40, range=55, skills=[4]),
    13: dict(hp=990, defense=98, speed=62, primary=210, special=140, angle=50, range=50, skills=[]),
    14: dict(hp=1010, defense=104, speed=60, primary=220, special=150, angle=50, range=50, skills=[]),
}
SKILL_NAMES = {1: "爆炸", 2: "毒", 3: "火", 4: "雷"}

# Original item list frames (itemlist!.i16) and their server-side effects.
# id, name, description, effect, amount, turns, frame index
ITEM_ROWS = [
    (0, "小补丸", "恢复 180 生命", "heal_self", 180, 0, 0),
    (1, "大补丸", "恢复 300 生命", "heal_self", 300, 0, 1),
    (2, "能量补充弹", "恢复 70 移动力", "fuel", 70, 0, 2),
    (3, "团补丸", "全体队友恢复 120 生命", "heal_team", 120, 0, 3),
    (4, "防护罩", "2 回合内受到的爆炸伤害减半", "shield_self", 0, 2, 4),
    (25, "威力增强弹", "下一发炮弹伤害提高 40%", "power_self", 40, 1, 25),
    (26, "风向转换器", "立即反转当前风向", "reverse_wind", 0, 0, 26),
    (35, "穿风弹", "下一发炮弹不受风力影响", "no_wind", 0, 1, 35),
]


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def lha(payload: bytes, raw: int) -> bytes:
    info = LhaInfo()
    info.compress_type = b"-lh5-"
    info.compress_size = len(payload)
    info.file_size = raw
    info.CRC = 0
    info.filename = "x"
    out = io.BytesIO()
    session = lzhlib.LZHDecodeSession(io.BytesIO(payload), out, info)
    while not session.do_next():
        pass
    return out.getvalue()


def decode_span(words, w: int, h: int):
    pixels = [0] * (w * h)
    alpha = [0] * (w * h)
    i = 0
    for y in range(h):
        if i + 2 > len(words):
            break
        _row_bytes, spans = words[i], words[i + 1]
        i += 2
        for _ in range(spans):
            if i + 2 > len(words):
                break
            x, n = words[i], words[i + 1]
            i += 2
            vals = words[i:i + n]
            i += n
            for k, v in enumerate(vals):
                xx = x + k
                if 0 <= xx < w:
                    pixels[y * w + xx] = v
                    alpha[y * w + xx] = 0 if v == TRANSPARENT else 255
    return pixels, alpha


def to_image(pixels, alpha, w: int, h: int) -> Image.Image:
    r = bytes(((v >> 11) & 31) * 255 // 31 for v in pixels)
    g = bytes(((v >> 5) & 63) * 255 // 63 for v in pixels)
    b = bytes((v & 31) * 255 // 31 for v in pixels)
    return Image.merge(
        "RGBA",
        (
            Image.frombytes("L", (w, h), r),
            Image.frombytes("L", (w, h), g),
            Image.frombytes("L", (w, h), b),
            Image.frombytes("L", (w, h), bytes(alpha)),
        ),
    )


def decode_block(data: bytes, raw: int, typ: int, w: int, h: int):
    packed = lha(data, raw)
    words = list(struct.unpack_from("<%dH" % (len(packed) // 2), packed))
    if typ == 1:
        pixels = words[: w * h]
        alpha = [0 if v == TRANSPARENT else 255 for v in pixels]
        return to_image(pixels, alpha, w, h)
    pixels, alpha = decode_span(words, w, h)
    return to_image(pixels, alpha, w, h)


def frames(path: Path):
    src = path.read_bytes()
    if src[:8] == b"ENC_I16\0":
        raise RuntimeError(f"encrypted i16 not supported: {path.name}")
    count = struct.unpack_from("<I", src, 0)[0]
    off = 4
    result = []
    for _ in range(count):
        if off + 24 > len(src):
            break
        typ, w, h = struct.unpack_from("<III", src, off)
        ox, oy = struct.unpack_from("<hh", src, off + 12)
        raw, comp = struct.unpack_from("<II", src, off + 16)
        body = decode_block(src[off + 24:off + 24 + comp], raw, typ, w, h)
        end = off + 24 + comp
        flag = struct.unpack_from("<I", src, end)[0] if end + 4 <= len(src) else 0
        other = None
        if flag:
            sx, sy = struct.unpack_from("<hh", src, end + 4)
            scomp = struct.unpack_from("<I", src, end + 8)[0]
            other = (decode_block(src[end + 12:end + 12 + scomp], flag, typ, w, h), sx, sy)
            off = end + 12 + scomp
        else:
            off = end + 4
        result.append(dict(type=typ, w=w, h=h, ox=ox, oy=oy, image=body, other=other))
    return result


def crop_anchor(image: Image.Image, ox: int, oy: int):
    """Crop to alpha bounds and return the anchor position within the crop."""
    bbox = image.getchannel("A").getbbox()
    if not bbox:
        return image, 0, 0
    left, top, right, bottom = bbox
    return image.crop(bbox), -ox - left, -oy - top


def save_webp(image: Image.Image, path: Path, quality: int = 92, lossless: bool = False):
    path.parent.mkdir(parents=True, exist_ok=True)
    if lossless:
        image.save(path, "WEBP", lossless=True, quality=100, method=6)
    else:
        image.save(path, "WEBP", quality=quality, alpha_quality=100, method=6)


def rgba_at(image: Image.Image, x: int, y: int):
    if x < 0 or y < 0 or x >= image.width or y >= image.height:
        return (0, 0, 0, 0)
    return image.getpixel((x, y))


def build_mask(fg: Image.Image, off_y: int):
    """4 px collision cells over the 960 px world canvas."""
    cols = math.ceil(fg.width / CELL)
    rows = math.ceil(WORLD_HEIGHT / CELL)
    rows_fg = math.ceil(fg.height / CELL)
    bits = bytearray((cols * rows + 7) // 8)

    def solid(cx, cy):
        x0, y0 = cx * CELL, cy * CELL - off_y
        for yy in range(y0, y0 + CELL):
            if yy < 0 or yy >= fg.height:
                continue
            for xx in range(x0, x0 + CELL):
                if xx < 0 or xx >= fg.width:
                    continue
                if fg.getpixel((xx, yy))[3] > 0:
                    return True
        return False

    for cy in range(rows):
        for cx in range(cols):
            if solid(cx, cy):
                idx = cy * cols + cx
                bits[idx >> 3] |= 1 << (7 - (idx & 7))
    return cols, rows, base64.b64encode(bytes(bits)).decode("ascii")


def derive_bullet_mask(src_bits: bytes, cols: int, rows: int) -> bytes:
    """Keep only foreground components that reach the lower quarter of the map.

    The original foreground layer contains floating rails and platforms. Tanks
    collide with all of them, but treating every decorative rail as shell
    cover made most shots stop after a quarter of the map. Shells use this
    ground-like subset; tank movement and spawns keep the full mask.
    """
    def get(idx: int) -> bool:
        return src_bits[idx >> 3] & (1 << (7 - (idx & 7))) != 0

    split = int(rows * 0.72)
    out = bytearray(len(src_bits))
    seen = bytearray(cols * rows)
    for sy in range(rows):
        for sx in range(cols):
            start = sy * cols + sx
            if seen[start] or not get(start):
                continue
            queue = [start]
            seen[start] = 1
            cells = []
            low = 0
            while queue:
                idx = queue.pop()
                cells.append(idx)
                x, y = idx % cols, idx // cols
                low = max(low, y)
                for nx, ny in ((x + 1, y), (x - 1, y), (x, y + 1), (x, y - 1)):
                    if nx < 0 or ny < 0 or nx >= cols or ny >= rows:
                        continue
                    ni = ny * cols + nx
                    if not seen[ni] and get(ni):
                        seen[ni] = 1
                        queue.append(ni)
            if low >= split:
                for idx in cells:
                    out[idx >> 3] |= 1 << (7 - (idx & 7))
    if not any(out):
        return src_bits
    return bytes(out)


def safe_ground(mask_bits: bytes, cols: int, rows: int, x: int, want_team: int):
    """Find a column with solid ground and enough headroom."""
    order = [x]
    for d in range(CELL, cols * CELL // 2, CELL):
        order.extend([x - d, x + d])
    best = None
    for xx in order:
        cx = xx // CELL
        if cx < 0 or cx >= cols:
            continue
        # Find the highest solid cell with a clear 5-cell column above.
        for cy in range(rows - 1, 3, -1):
            idx = cy * cols + cx
            if mask_bits[idx >> 3] & (1 << (7 - (idx & 7))):
                clear = True
                for up in range(1, 6):
                    j = (cy - up) * cols + cx
                    if mask_bits[j >> 3] & (1 << (7 - (j & 7))):
                        clear = False
                        break
                if clear:
                    candidate = (xx, cy * CELL)
                    if best is None:
                        best = candidate
                    elif want_team == 0 and candidate[0] < best[0]:
                        best = candidate
                    elif want_team == 1 and candidate[0] > best[0]:
                        best = candidate
                    break
        if best and abs(best[0] - x) <= cols * CELL // 4:
            break
    return best


def spawn_points(mask_bits: bytes, cols: int, rows: int, players: int):
    team_size = players // 2
    out = []
    for team in range(2):
        base_x = int(cols * CELL * (0.14 if team == 0 else 0.86))
        spacing = int(cols * CELL * 0.07)
        for i in range(team_size):
            want = base_x + (i - (team_size - 1) / 2) * spacing
            got = safe_ground(mask_bits, cols, rows, int(want), team)
            if not got:
                got = (int(want), WORLD_HEIGHT - 80)
            out.append({"x": got[0], "y": got[1], "team": team, "seat": team * team_size + i})
    return out


def build_map(m: dict):
    stage = m["stage"]
    fg_path = LOCAL / "downloaded" / f"stageN_{stage}f.i16"
    bg_candidates = [LOCAL / "downloaded" / f"stageN_{stage}_0b.i16", LOCAL / "downloaded" / f"stageN_{stage}b.i16"]
    bg_path = next((p for p in bg_candidates if p.exists()), None)
    if not fg_path.exists():
        raise SystemExit(f"missing foreground: {fg_path}")
    fg_frames = frames(fg_path)
    fg = fg_frames[0]["image"]
    if fg_frames[0]["type"] != 1:
        raise SystemExit(f"unexpected foreground type for stage {stage}")
    off_y = fg_frames[0]["oy"]
    bg = None
    if bg_path:
        bg = frames(bg_path)[0]["image"]
    # Save full-res layers.
    fg_out = OUT / "maps" / f"{m['id']}-fg.webp"
    save_webp(fg, fg_out, quality=92)
    bg_out = OUT / "maps" / f"{m['id']}-bg.webp"
    if bg is not None:
        # Stretch the backdrop to the full 960 px world height; foreground is
        # positioned at its original canvas offset.
        bg_canvas = bg.resize((fg.width, WORLD_HEIGHT), Image.LANCZOS)
        save_webp(bg_canvas, bg_out, quality=90)
    # Thumbnail for the map picker: background + foreground.
    thumb = Image.new("RGBA", (fg.width, WORLD_HEIGHT), (0, 0, 0, 255))
    if bg is not None:
        thumb.alpha_composite(bg.resize((fg.width, WORLD_HEIGHT), Image.LANCZOS), (0, 0))
    thumb.alpha_composite(fg, (0, off_y))
    thumb.thumbnail((480, 300), Image.LANCZOS)
    thumb_out = OUT / "maps" / f"{m['id']}-thumb.webp"
    save_webp(thumb, thumb_out, quality=85)
    mask_cols, mask_rows, mask_b64 = build_mask(fg, off_y)
    mask_bits = base64.b64decode(mask_b64)
    bullet_b64 = base64.b64encode(derive_bullet_mask(mask_bits, mask_cols, mask_rows)).decode("ascii")
    spawns = spawn_points(mask_bits, mask_cols, mask_rows, 8)
    music = LOCAL / "analysis" / "music" / f"stage{stage:02d}.wav"
    if not music.exists():
        music = LOCAL / "analysis" / "music" / "stage15.wav"
    music_out = OUT / "audio" / "maps" / f"{stage:02d}.mp3"
    music_out.parent.mkdir(parents=True, exist_ok=True)
    if music.exists():
        subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", str(music), "-c:a", "libmp3lame", "-b:a", "96k", "-ac", "1", str(music_out)], check=True)
    # The two foreground frames are a gentle animated overlay (clouds/water).
    overlay = None
    if len(fg_frames) > 1:
        overlay = fg_frames[1]["image"]
        overlay_out = OUT / "maps" / f"{m['id']}-fg2.webp"
        save_webp(overlay, overlay_out, quality=92)
    return {
        "id": m["id"],
        "name": m["name"],
        "nameEn": m["nameEn"],
        "stage": stage,
        "width": fg.width,
        "height": WORLD_HEIGHT,
        "offY": off_y,
        "fg": f"/fortressblue/v2/maps/{m['id']}-fg.webp",
        "fg2": f"/fortressblue/v2/maps/{m['id']}-fg2.webp" if overlay else None,
        "bg": f"/fortressblue/v2/maps/{m['id']}-bg.webp" if bg is not None else None,
        "thumb": f"/fortressblue/v2/maps/{m['id']}-thumb.webp",
        "music": f"/fortressblue/v2/audio/maps/{stage:02d}.mp3" if music.exists() else None,
        "maskCols": mask_cols,
        "maskRows": mask_rows,
        "mask": mask_b64,
        "bulletMask": bullet_b64,
        "spawns": spawns,
        "fgSha256": sha256(fg_out),
        "bgSha256": sha256(bg_out) if bg is not None else None,
    }


def build_tank(tank_id: int):
    path = LOCAL / "downloaded" / f"char{tank_id:02d}.i16"
    if not path.exists():
        return None
    all_frames = frames(path)
    # Frame 0 = idle, 8 = firing, 18 = destroyed, 28 = moving. Both blocks are
    # left/right facing variants; use the right-facing (second block).
    def pick(index, state):
        f = all_frames[index]
        img, sx, sy = f["other"] if f["other"] else (f["image"], f["ox"], f["oy"])
        cropped, ax, ay = crop_anchor(img, sx, sy)
        out = OUT / "tanks" / f"{tank_id}-{state}.webp"
        save_webp(cropped, out, quality=95, lossless=True)
        return f"/fortressblue/v2/tanks/{tank_id}-{state}.webp", cropped.width, cropped.height, ax, ay
    states = {}
    for idx, state in ((0, "idle"), (8, "fire"), (18, "die"), (28, "move")):
        if idx < len(all_frames):
            url, w, h, ax, ay = pick(idx, state)
            states[state] = {"url": url, "w": w, "h": h, "ax": ax, "ay": ay}
    # Bullet.
    bpath = LOCAL / "downloaded" / f"bullet{tank_id:02d}.i16"
    bullet = None
    if bpath.exists():
        f = frames(bpath)[0]
        img, ax, ay = crop_anchor(f["image"], f["ox"], f["oy"])
        out = OUT / "projectiles" / f"{tank_id}.webp"
        save_webp(img, out, quality=95, lossless=True)
        bullet = {"url": f"/fortressblue/v2/projectiles/{tank_id}.webp", "w": img.width, "h": img.height, "ax": ax, "ay": ay}
    return states, bullet


def build_explosion():
    src = LOCAL / "downloaded" / "explo.i16"
    fs = frames(src)
    cell_w = max(f["image"].width for f in fs)
    cell_h = max(f["image"].height for f in fs)
    sheet = Image.new("RGBA", (cell_w * len(fs), cell_h), (0, 0, 0, 0))
    metas = []
    for i, f in enumerate(fs):
        x = i * cell_w + (cell_w - f["image"].width) // 2
        y = (cell_h - f["image"].height) // 2
        sheet.alpha_composite(f["image"], (x, y))
        metas.append({"x": x, "y": y, "ox": f["ox"], "oy": f["oy"]})
    out = OUT / "effects" / "explosion.webp"
    save_webp(sheet, out, quality=92)
    meta = OUT / "effects" / "explosion.json"
    meta.write_text(json.dumps({"cellW": cell_w, "cellH": cell_h, "frames": metas}, ensure_ascii=False, indent=1))
    return {"url": "/fortressblue/v2/effects/explosion.webp", "cellW": cell_w, "cellH": cell_h, "count": len(fs)}


def build_portraits():
    path = LOCAL / "extracted/package/Asset/tank.i16"
    fs = frames(path)
    out = {}
    for index, f in enumerate(fs):
        tank_id = index + 1
        if tank_id > 13:
            tank_id = 14
        img, ax, ay = crop_anchor(f["image"], f["ox"], f["oy"])
        target = OUT / "tanks" / f"{tank_id}-portrait.webp"
        save_webp(img, target, quality=92)
        out[str(tank_id)] = {"url": f"/fortressblue/v2/tanks/{tank_id}-portrait.webp", "w": img.width, "h": img.height, "ax": ax, "ay": ay}
    return out


def build_audio():
    sfx_map = {
        "fire": "Fire1-1.wav", "explode": "Bomb1-1.wav", "move": "move1.wav",
        "unmove": "Unmove1.wav", "select": "Select1.wav", "turn": "Turn.wav",
        "win": "Warring.wav", "lose": "Enemy.wav", "hit": "BoxLanding.wav",
    }
    out = {}
    for name, src in sfx_map.items():
        p = LOCAL / "extracted/package/Asset" / src
        if not p.exists():
            continue
        target = OUT / "audio" / f"{name}.mp3"
        target.parent.mkdir(parents=True, exist_ok=True)
        subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", str(p), "-c:a", "libmp3lame", "-b:a", "96k", "-ac", "1", str(target)], check=True)
        out[name] = f"/fortressblue/v2/audio/{name}.mp3"
    return out


def build_items():
    src = LOCAL / "extracted/package/Asset/itemlist!.i16"
    if not src.exists():
        raise SystemExit(f"missing item list: {src}")
    sheets = frames(src)
    out_dir = OUT / "items"
    out_dir.mkdir(parents=True, exist_ok=True)
    items = []
    for item_id, name, description, effect, amount, turns, frame in ITEM_ROWS:
        if frame >= len(sheets):
            raise SystemExit(f"item frame {frame} missing for {name}")
        image = sheets[frame]["image"]
        out = out_dir / f"{item_id}.webp"
        save_webp(image, out, quality=95, lossless=True)
        items.append({
            "id": item_id,
            "name": name,
            "description": description,
            "effect": effect,
            "amount": amount,
            "turns": turns,
            "icon": f"/fortressblue/v2/items/{item_id}.webp",
            "w": image.width,
            "h": image.height,
        })
    # Pure media builder: provenance checks redirect OUT and must not mutate active rules.
    return items


def main():
    if (ASSETS / "manifest.json").exists() or (ROOT / "games/fortressblue/data/animations.json").exists() or (OUT.exists() and any(OUT.iterdir())):
        raise SystemExit("v2 is immutable and active data may be v3; use fetch-assets.py for published v2, prepare-animation-assets.py for v3, or verify-original-source.py for isolated provenance checks")
    if not LOCAL.exists():
        raise SystemExit(f"source assets not found: {LOCAL}")
    OUT.mkdir(parents=True, exist_ok=True)
    MAPS_DIR.mkdir(parents=True, exist_ok=True)
    (ROOT / "games/fortressblue/data").mkdir(parents=True, exist_ok=True)
    maps = []
    for cocos_id, name, name_en, game_map_id in MAP_ROWS:
        item = {"id": f"stage-{game_map_id}", "cocosId": cocos_id, "stage": game_map_id, "name": name, "nameEn": name_en}
        built = build_map(item)
        maps.append(built)
        print("map", built["id"], built["name"], built["width"], built["offY"], flush=True)
    tanks = {}
    for tank_id in range(1, 15):
        if tank_id not in TANK_STATS:
            continue
        built = build_tank(tank_id)
        if not built:
            continue
        states, bullet = built
        stats = TANK_STATS[tank_id]
        tanks[str(tank_id)] = {
            "id": tank_id,
            "name": TANK_NAMES[tank_id],
            "hp": stats["hp"], "defense": stats["defense"], "speed": stats["speed"],
            "primary": stats["primary"], "special": stats["special"],
            "angle": stats["angle"], "range": stats["range"],
            "skills": stats["skills"], "skillName": SKILL_NAMES.get(stats["skills"][0], "爆炸") if stats["skills"] else None,
            "sprites": states, "bullet": bullet,
        }
        print("tank", tank_id, flush=True)
    explosion = build_explosion()
    audio = build_audio()
    portraits = build_portraits()
    items = build_items()
    (ROOT / "games/fortressblue/data/items.json").write_text(json.dumps({"items": items}, ensure_ascii=False, indent=1) + "\n")
    TANKS_JSON.write_text(json.dumps({"tanks": tanks, "portraits": portraits, "explosion": explosion, "audio": audio}, ensure_ascii=False, indent=1) + "\n")
    (MAPS_DIR / "catalog.json").write_text(json.dumps({"version": 2, "worldHeight": WORLD_HEIGHT, "cell": CELL, "maps": maps}, ensure_ascii=False) + "\n")
    # The client needs the same public collision mask to draw an accurate aim
    # preview. It is not hidden information: the server derives it from the
    # visible foreground layer and sends every crater after each explosion.
    client_maps = [{k: v for k, v in m.items() if k not in ("fgSha256", "bgSha256")} for m in maps]
    (MAPS_DIR / "client.json").write_text(json.dumps({"version": 2, "worldHeight": WORLD_HEIGHT, "cell": CELL, "maps": client_maps}, ensure_ascii=False, indent=1) + "\n")
    # Emit a manifest for the uploader/verifier.
    entries = []
    for p in sorted(OUT.rglob("*")):
        if p.is_file() and p.suffix in (".webp", ".mp3"):
            rel = p.relative_to(OUT).as_posix()
            mime = "image/webp" if p.suffix == ".webp" else "audio/mpeg"
            entries.append({"file": rel, "urlPath": "/fortressblue/v2/" + rel, "output": p.relative_to(ROOT).as_posix(), "sha256": sha256(p), "bytes": p.stat().st_size, "mime": mime})
    manifest = {
        "version": "fortressblue/v2",
        "prefix": "bga/v20261002-1/fortressblue/v2/",
        "generator": "scripts/fortressblue/prepare-original-assets.py",
        "generatorSha256": sha256(Path(__file__).resolve()),
        "license": "原版素材来自官方网页客户端公开资源包；版权归 CCR / 原权利方所有，仅经用户授权在私有平台使用，原始媒体不进入 Git。",
        "entries": entries,
    }
    (ASSETS / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    sources = {
        "source": "https://game.fortress2blue.com/（官方网页客户端 tankclient_emsc.c906c.data.00-02 + Download/ 资源）",
        "acquired": "2026-10-20",
        "method": "Emscripten file package 拆分；LH5 解压；逐行 span RLE 解出 16-bit RGB565；官方 Cocos 大厅配置与运行时下载资源补齐",
        "license": "版权归 CCR / 原权利方所有。仅按用户明确授权用于本私有平台；原始媒体不提交 Git，不再分发。",
        "hashes": {
            "tankclient_emsc.c906c.data.00": sha256(LOCAL / "game/tankclient_emsc.c906c.data.00"),
            "tankclient_emsc.c906c.data.01": sha256(LOCAL / "game/tankclient_emsc.c906c.data.01"),
            "tankclient_emsc.c906c.data.02": sha256(LOCAL / "game/tankclient_emsc.c906c.data.02"),
            "tankclient_emsc.eeb2b.wasm": sha256(LOCAL / "game/tankclient_emsc.eeb2b.wasm"),
            "tankclient_emsc.9485d.js": sha256(LOCAL / "game/tankclient_emsc.9485d.js"),
        },
    }
    (ASSETS / "sources.json").write_text(json.dumps(sources, ensure_ascii=False, indent=2) + "\n")
    print("wrote", len(maps), "maps,", len(tanks), "tanks,", len(items), "items,", len(entries), "assets", flush=True)


if __name__ == "__main__":
    main()
