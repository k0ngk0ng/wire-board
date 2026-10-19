#!/usr/bin/env python3
"""Build the Battle City sprite atlas from the user-provided ROM.

The ROM is never redistributed: this script reads it from .local, writes the
derived pack under .local/battlecity-pack, and only installs the pack into
games/battlecity/assets/public (gitignored) on explicit request.

Frames follow games/battlecity/CONTRACT.md section 11 exactly.  Pixels are
decoded from the CHR bank without scaling; team recolouring maps the three
sprite colour indices to the documented warm/cool palettes.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import struct
import subprocess
import sys
import wave
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[2]
DEFAULT_ROM = ROOT / ".local/battlecity/rom/Battle City (J).nes"
DEFAULT_OUT = ROOT / ".local/battlecity-pack/battlecity/v1"
INSTALL = ROOT / "games/battlecity/assets/public/battlecity/v1"
ROM_SHA256 = "a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c"
CHR_OFFSET = 0x4010
FCEUX_PALETTE_SHA256 = "bbb0355bcadc4be771efeb81930b09b17d97dcb8c70167a652902a789158c5f8"

# FCEUX 2.6.6 default palette (FCEUX.pal), 64 NES colours.
PALETTE = [
    (116, 116, 116), (36, 24, 140), (0, 0, 168), (68, 0, 156),
    (140, 0, 116), (168, 0, 16), (164, 0, 0), (124, 8, 0),
    (64, 44, 0), (0, 68, 0), (0, 80, 0), (0, 60, 20),
    (24, 60, 92), (0, 0, 0), (0, 0, 0), (0, 0, 0),
    (188, 188, 188), (0, 112, 236), (32, 56, 236), (128, 0, 240),
    (188, 0, 188), (228, 0, 88), (216, 40, 0), (200, 76, 12),
    (136, 112, 0), (0, 148, 0), (0, 168, 0), (0, 144, 56),
    (0, 128, 136), (0, 0, 0), (0, 0, 0), (0, 0, 0),
    (252, 252, 252), (60, 188, 252), (92, 148, 252), (204, 136, 252),
    (244, 120, 252), (252, 116, 180), (252, 116, 96), (252, 152, 56),
    (240, 188, 60), (128, 208, 16), (76, 220, 72), (88, 248, 152),
    (0, 232, 216), (120, 120, 120), (0, 0, 0), (0, 0, 0),
    (252, 252, 252), (168, 228, 252), (196, 212, 252), (212, 200, 252),
    (252, 196, 252), (252, 196, 216), (252, 188, 176), (252, 216, 168),
    (252, 228, 160), (224, 252, 160), (168, 240, 188), (176, 252, 204),
    (156, 252, 240), (196, 196, 196), (0, 0, 0), (0, 0, 0),
]

# Captured gameplay palettes (PPU $3F00-$3F1F).
BG0 = [0x0F, 0x17, 0x06, 0x00]  # brick, eagle
BG1 = [0x0F, 0x12, 0x3C, 0x12]  # water
BG2 = [0x0F, 0x29, 0x09, 0x0B]  # trees
BG3 = [0x0F, 0x00, 0x10, 0x20]  # steel, ice
SPRITE2 = [0x0F, 0x0C, 0x10, 0x20]  # powerups, shield, bullets
SPRITE3 = [0x0F, 0x04, 0x16, 0x20]  # explosions
TEAM_TOP = [0x0F, 0x06, 0x17, 0x27]  # warm red/orange
TEAM_BOTTOM = [0x0F, 0x0C, 0x12, 0x21]  # cool blue/cyan

TERRAIN = {
    "brick": (0x0F, BG0),
    "steel": (0x10, BG3),
    "water": (0x12, BG1),
    "trees": (0x22, BG2),
    "ice": (0x21, BG3),
}
BASE_TILES = {
    "base-top": ((0xC8, 0xCA, 0xC9, 0xCB), BG0),
    "base-bottom": ((0xC8, 0xCA, 0xC9, 0xCB), BG0),
    "base-destroyed": ((0xCC, 0xCE, 0xCD, 0xCF), BG0),
}
# Each class owns 16 CHR tiles: two orientations x two tread frames. The
# 8x16 OAM pair uses TL=base, TR=base+2, BL=base+1, BR=base+3. The original
# game flips these two orientations for the opposite directions.
CLASSES = {
    "balanced": {"up": 0x20, "left": 0x28},
    "basic": {"left": 0xA8, "up": 0xB0},
    "fast": {"left": 0xB8, "up": 0xC0},
    "power": {"left": 0xC8, "up": 0xD0},
    "armor": {"left": 0xD8, "up": 0xE0},
}
DIRECTIONS = ("up", "right", "down", "left")
POWERUPS = {
    "powerup-helmet": 128,
    "powerup-clock": 132,
    "powerup-shovel": 136,
    "powerup-star": 140,
    "powerup-grenade": 144,
    "powerup-tank": 148,
}
AUDIO_KEYS = [
    "bgm-battle", "bgm-start", "sfx-fire", "sfx-brick", "sfx-steel",
    "sfx-explode", "sfx-powerup", "sfx-shield", "sfx-base", "sfx-win", "sfx-lose",
]
BGM_KEYS = {"bgm-battle", "bgm-start"}


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def decode_tile(chr_data: bytes, tile: int, bank: int) -> list[list[int]]:
    start = (0x1000 if bank else 0) + tile * 16
    raw = chr_data[start : start + 16]
    if len(raw) != 16:
        raise ValueError(f"tile ${tile:02X} outside CHR")
    return [
        [((raw[y] >> (7 - x)) & 1) | (((raw[y + 8] >> (7 - x)) & 1) << 1) for x in range(8)]
        for y in range(8)
    ]


def sprite16(chr_data: bytes, base: int, palette: list[int], bank: int = 1) -> Image.Image:
    """8x16 pair composition used by $DA2B/$DA7B: TL=base, TR=base+2, BL=base+1, BR=base+3."""
    image = Image.new("RGBA", (16, 16), (0, 0, 0, 0))
    for tile, ox, oy in ((base, 0, 0), (base + 2, 8, 0), (base + 1, 0, 8), (base + 3, 8, 8)):
        for y, row in enumerate(decode_tile(chr_data, tile, bank)):
            for x, value in enumerate(row):
                if value:
                    image.putpixel((ox + x, oy + y), PALETTE[palette[value] & 0x3F] + (255,))
    return image


def bullet6(chr_data: bytes, palette: list[int]) -> Image.Image:
    """Canonical upward bullet (3x4 core) centred in the contract's 6x6 frame.

    The original 8x16 sprite uses bank 1, base $B1: top tile 176, bottom 177.
    Up/down and left/right art are exact 180-degree rotations of each other,
    so the game can rotate this single frame for all four directions.
    """
    image = Image.new("RGBA", (6, 6), (0, 0, 0, 0))
    tiles = (decode_tile(chr_data, 176, 1), decode_tile(chr_data, 177, 1))
    for ty, row in enumerate(tiles[0]):
        for x, value in enumerate(row):
            if 3 <= x <= 5 and 6 <= ty <= 7:
                image.putpixel((x - 1, ty - 5), PALETTE[palette[value] & 0x3F] + (255,))
    for ty, row in enumerate(tiles[1]):
        for x, value in enumerate(row):
            if 3 <= x <= 5 and ty <= 1:
                image.putpixel((x - 1, ty + 3), PALETTE[palette[value] & 0x3F] + (255,))
    return image


def terrain8(chr_data: bytes, tile: int, palette: list[int]) -> Image.Image:
    image = Image.new("RGBA", (8, 8), (0, 0, 0, 255))
    for y, row in enumerate(decode_tile(chr_data, tile, 1)):
        for x, value in enumerate(row):
            image.putpixel((x, y), PALETTE[palette[value] & 0x3F] + (255,))
    return image


def base16(chr_data: bytes, tiles: tuple[int, int, int, int], palette: list[int]) -> Image.Image:
    image = Image.new("RGBA", (16, 16), (0, 0, 0, 255))
    for tile, ox, oy in zip(tiles, (0, 8, 0, 8), (0, 0, 8, 8)):
        for y, row in enumerate(decode_tile(chr_data, tile, 1)):
            for x, value in enumerate(row):
                image.putpixel((ox + x, oy + y), PALETTE[palette[value] & 0x3F] + (255,))
    return image


def build_frames(chr_data: bytes) -> dict[str, tuple[Image.Image, dict]]:
    frames: dict[str, tuple[Image.Image, dict]] = {}
    for name, (tile, palette) in TERRAIN.items():
        frames[name] = (
            terrain8(chr_data, tile, palette),
            {"category": "terrain", "opaque": True, "palette": palette, "tiles": [tile]},
        )
    for name, (tiles, palette) in BASE_TILES.items():
        frames[name] = (
            base16(chr_data, tiles, palette),
            {"category": "base", "opaque": True, "palette": palette, "tiles": list(tiles)},
        )
    for cls, bases in CLASSES.items():
        for team, palette in (("top", TEAM_TOP), ("bottom", TEAM_BOTTOM)):
            up = {
                frame: sprite16(chr_data, bases["up"] + frame * 4, palette, bank=0)
                for frame in (0, 1)
            }
            left = {
                frame: sprite16(chr_data, bases["left"] + frame * 4, palette, bank=0)
                for frame in (0, 1)
            }
            oriented = {
                "up": up,
                "down": {
                    frame: up[frame].transpose(Image.Transpose.FLIP_TOP_BOTTOM)
                    for frame in (0, 1)
                },
                "left": left,
                "right": {
                    frame: left[frame].transpose(Image.Transpose.FLIP_LEFT_RIGHT)
                    for frame in (0, 1)
                },
            }
            for direction in DIRECTIONS:
                source = bases["up"] if direction in ("up", "down") else bases["left"]
                for frame in (0, 1):
                    base = source + frame * 4
                    name = f"tank-{cls}-{team}-{direction}-{frame}"
                    frames[name] = (
                        # Tank OAM bytes are even, selecting pattern table $0000
                        # (CHR bank 0); effects/items use odd bytes (bank 1).
                        oriented[direction][frame],
                        {
                            "category": "tank",
                            "class": cls,
                            "team": team,
                            "direction": direction,
                            "frame": frame,
                            "opaque": False,
                            "palette": palette,
                            "chr_bank": 0,
                            "flip": {
                                "up": "none", "down": "vertical",
                                "left": "none", "right": "horizontal",
                            }[direction],
                            "tiles": [base, base + 1, base + 2, base + 3],
                        },
                    )
    for team, palette in (("top", TEAM_TOP), ("bottom", TEAM_BOTTOM)):
        frames[f"bullet-{team}"] = (
            bullet6(chr_data, palette),
            {"category": "bullet", "team": team, "opaque": False, "palette": palette, "tiles": [176, 177]},
        )
    for index, base in enumerate((240, 244, 248)):
        frames[f"explosion-{index}"] = (
            sprite16(chr_data, base, SPRITE3),
            {"category": "explosion", "opaque": False, "palette": SPRITE3,
             "tiles": [base, base + 1, base + 2, base + 3]},
        )
    for index, base in enumerate((40, 44)):
        frames[f"shield-{index}"] = (
            sprite16(chr_data, base, SPRITE2),
            {"category": "shield", "opaque": False, "palette": SPRITE2,
             "tiles": [base, base + 1, base + 2, base + 3]},
        )
    for name, base in POWERUPS.items():
        frames[name] = (
            sprite16(chr_data, base, SPRITE2),
            {"category": "powerup", "opaque": False, "palette": SPRITE2,
             "tiles": [base, base + 1, base + 2, base + 3]},
        )
    return frames


def pack(frames: dict[str, tuple[Image.Image, dict]], padding: int = 2, max_width: int = 512):
    ordered = sorted(frames.items(), key=lambda item: (-item[1][0].height, -item[1][0].width, item[0]))
    x = y = row_height = 0
    placements: dict[str, dict] = {}
    for name, (image, _meta) in ordered:
        if x and x + image.width + padding > max_width:
            x = 0
            y += row_height + padding
            row_height = 0
        placements[name] = {"x": x, "y": y, "w": image.width, "h": image.height}
        x += image.width + padding
        row_height = max(row_height, image.height)
    width = max(p["x"] + p["w"] for p in placements.values())
    height = max(p["y"] + p["h"] for p in placements.values())
    atlas = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    for name, (image, _meta) in frames.items():
        pos = placements[name]
        atlas.alpha_composite(image, (pos["x"], pos["y"]))
    return atlas, placements


def encode_audio(masters: Path, out: Path) -> dict:
    """Encode the 11 contract audio keys from WAV masters and validate them."""
    out.mkdir(parents=True, exist_ok=True)
    records = {}
    for key in AUDIO_KEYS:
        source = masters / f"{key}.wav"
        if not source.is_file():
            raise SystemExit(f"missing audio master {source}")
        with wave.open(str(source), "rb") as wav:
            channels, rate, frames = wav.getnchannels(), wav.getframerate(), wav.getnframes()
            samples = wav.readframes(frames)
        duration = frames / rate
        if frames == 0 or not any(samples):
            raise SystemExit(f"{key}: silent master")
        target = out / f"{key}.mp3"
        bitrate = "128k" if key in BGM_KEYS else "96k"
        channels_out = 2 if key in BGM_KEYS else 1
        subprocess.run(
            [
                "ffmpeg", "-y", "-loglevel", "error", "-i", str(source),
                "-ac", str(channels_out), "-codec:a", "libmp3lame", "-b:a", bitrate,
                "-map_metadata", "-1", str(target),
            ],
            check=True,
        )
        probe = subprocess.run(
            ["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", str(target)],
            check=True, capture_output=True, text=True,
        )
        records[key] = {
            "file": target.name,
            "bytes": target.stat().st_size,
            "sha256": sha256(target.read_bytes()),
            "duration_s": round(float(probe.stdout.strip()), 3),
            "source_channels": channels,
            "source_rate": rate,
            "bitrate": bitrate,
            "channels": channels_out,
        }
    return records


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--rom", type=Path, default=DEFAULT_ROM)
    parser.add_argument("--out", type=Path, default=DEFAULT_OUT)
    parser.add_argument("--masters", type=Path, help="directory with <key>.wav audio masters")
    parser.add_argument("--install-local", action="store_true")
    args = parser.parse_args()

    rom = args.rom.read_bytes()
    if sha256(rom) != ROM_SHA256:
        raise SystemExit(f"unexpected ROM: {args.rom}")
    if len(rom) < CHR_OFFSET + 8192:
        raise SystemExit("ROM has no complete CHR bank")
    chr_data = rom[CHR_OFFSET : CHR_OFFSET + 8192]
    frames = build_frames(chr_data)
    expected = (
        set(TERRAIN) | set(BASE_TILES)
        | {f"tank-{c}-{t}-{d}-{f}" for c in CLASSES for t in ("top", "bottom")
           for d in DIRECTIONS for f in (0, 1)}
        | {"bullet-top", "bullet-bottom"}
        | {f"explosion-{i}" for i in range(3)}
        | {f"shield-{i}" for i in range(2)}
        | set(POWERUPS)
    )
    if set(frames) != expected:
        raise SystemExit(f"frame set mismatch: {sorted(set(frames) ^ expected)}")

    atlas, placements = pack(frames)
    args.out.mkdir(parents=True, exist_ok=True)
    atlas_path = args.out / "atlas.webp"
    atlas.save(atlas_path, "WEBP", lossless=True, quality=100, method=6, exact=True)
    check = Image.open(atlas_path).convert("RGBA")
    if check.size != atlas.size or check.tobytes() != atlas.tobytes():
        raise SystemExit("lossless WebP round-trip changed pixels")

    records = {}
    for name, (image, meta) in sorted(frames.items()):
        record = {"x": placements[name]["x"], "y": placements[name]["y"],
                  "w": image.width, "h": image.height, **meta}
        records[name] = record
    metadata = {
        "version": 1,
        "image": atlas_path.name,
        "width": atlas.width,
        "height": atlas.height,
        "image_bytes": atlas_path.stat().st_size,
        "image_sha256": sha256(atlas_path.read_bytes()),
        "frames": records,
        "teams": {"top": TEAM_TOP, "bottom": TEAM_BOTTOM},
        "palette": {"name": "FCEUX.pal", "sha256": FCEUX_PALETTE_SHA256},
        "source": {
            "rom_sha256": ROM_SHA256,
            "chr_offset": CHR_OFFSET,
            "chr_bytes": len(chr_data),
            "generator": "scripts/battlecity/prepare_assets.py",
            "generator_sha256": sha256(Path(__file__).read_bytes()),
            "layout": "TL=base, TR=base+2, BL=base+1, BR=base+3 (8x16 pairs)",
            "tank_base": "class_base + direction_offset + frame*4; group strides 0x00/0x80/0xA0/0xC0/0xE0",
        },
    }
    if args.masters:
        metadata["audio"] = encode_audio(args.masters, args.out / "audio")
    if metadata.get("audio") and set(metadata["audio"]) != set(AUDIO_KEYS):
        raise SystemExit("audio key set mismatch")
    (args.out / "atlas.json").write_text(json.dumps(metadata, indent=2) + "\n")

    if args.install_local:
        INSTALL.mkdir(parents=True, exist_ok=True)
        for item in args.out.iterdir():
            if item.is_file():
                shutil.copy2(item, INSTALL / item.name)
        if (args.out / "audio").is_dir():
            shutil.copytree(args.out / "audio", INSTALL / "audio", dirs_exist_ok=True)
        print(f"installed to {INSTALL}")

    print(json.dumps({
        "out": str(args.out),
        "frames": len(records),
        "atlas": [atlas.width, atlas.height],
        "bytes": atlas_path.stat().st_size,
        "sha256": metadata["image_sha256"],
        "audio_keys": len(metadata.get("audio", {})),
    }, indent=2))


if __name__ == "__main__":
    main()
