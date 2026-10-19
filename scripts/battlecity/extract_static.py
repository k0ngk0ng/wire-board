#!/usr/bin/env python3
"""Decode Battle City's static NROM CHR ROM and palette tables.

This script only reads the user-provided ROM and writes research artifacts to
the git-ignored .local workspace.  It does not execute or emulate the ROM.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[2]
DEFAULT_ROM = ROOT / ".local/battlecity/rom/Battle City (J).nes"
OUT = ROOT / ".local/battlecity/extract"
ROM_SHA256 = "a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c"

# A conventional 2C02 palette used only for research contact sheets.  Runtime
# colors are captured from FCEUX's own gui.parsecolor table.
NES_RGB = [
    (84, 84, 84), (0, 30, 116), (8, 16, 144), (48, 0, 136),
    (68, 0, 100), (92, 0, 48), (84, 4, 0), (60, 24, 0),
    (32, 42, 0), (8, 58, 0), (0, 64, 0), (0, 60, 0),
    (0, 50, 60), (0, 0, 0), (0, 0, 0), (0, 0, 0),
    (152, 150, 152), (8, 76, 196), (48, 50, 236), (92, 30, 228),
    (136, 20, 176), (160, 20, 100), (152, 34, 32), (120, 60, 0),
    (84, 90, 0), (40, 114, 0), (8, 124, 0), (0, 118, 40),
    (0, 102, 120), (0, 0, 0), (0, 0, 0), (0, 0, 0),
    (236, 238, 236), (76, 154, 236), (120, 124, 236), (176, 98, 236),
    (228, 84, 236), (236, 88, 180), (236, 106, 100), (212, 136, 32),
    (160, 170, 0), (116, 196, 0), (76, 208, 32), (56, 204, 108),
    (56, 180, 204), (60, 60, 60), (0, 0, 0), (0, 0, 0),
    (236, 238, 236), (168, 204, 236), (188, 188, 236), (212, 178, 236),
    (236, 174, 236), (236, 174, 212), (236, 180, 176), (228, 196, 144),
    (204, 210, 120), (180, 222, 120), (168, 226, 144), (152, 226, 180),
    (160, 214, 228), (160, 162, 160), (0, 0, 0), (0, 0, 0),
]
RAMP = [(0, 0, 0, 0), (96, 96, 96, 255), (176, 176, 176, 255), (255, 255, 255, 255)]


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def decode_tile(data: bytes) -> list[list[int]]:
    if len(data) != 16:
        raise ValueError("NES 2bpp tile must be 16 bytes")
    return [
        [((data[y] >> (7 - x)) & 1) | (((data[y + 8] >> (7 - x)) & 1) << 1) for x in range(8)]
        for y in range(8)
    ]


def parse_rom(data: bytes) -> dict:
    if data[:4] != b"NES\x1a":
        raise ValueError("not an iNES ROM")
    prg_banks, chr_banks = data[4], data[5]
    if prg_banks != 1 or chr_banks != 1:
        raise ValueError("expected NROM-128 with one PRG and one CHR bank")
    if len(data) != 16 + 1 * 16384 + 1 * 8192:
        raise ValueError("unexpected ROM length")
    return {
        "prg_bytes": prg_banks * 16384,
        "chr_bytes": chr_banks * 8192,
        "mapper": (data[6] >> 4) | (data[7] & 0xF0),
        "mirroring": "horizontal" if data[6] & 1 else "vertical",
        "prg_offset": 16,
        "chr_offset": 16 + prg_banks * 16384,
    }


def prg_byte(prg: bytes, address: int) -> int:
    # NROM-128 mirrors $C000-$FFFF onto $8000-$BFFF.
    return prg[(address - 0x8000) & 0x3FFF]


def read_bytes(prg: bytes, address: int, count: int) -> bytes:
    return bytes(prg_byte(prg, address + i) for i in range(count))


def render_tiles(chr_data: bytes) -> Image.Image:
    image = Image.new("RGBA", (16 * 8, 32 * 8), (0, 0, 0, 0))
    for tile in range(512):
        pixels = decode_tile(chr_data[tile * 16 : (tile + 1) * 16])
        x0, y0 = (tile % 16) * 8, (tile // 16) * 8
        for y, row in enumerate(pixels):
            for x, index in enumerate(row):
                image.putpixel((x0 + x, y0 + y), RAMP[index])
    return image


def render_labeled(chr_data: bytes) -> Image.Image:
    scale = 4
    sheet = render_tiles(chr_data).resize((16 * 8 * scale, 32 * 8 * scale), Image.Resampling.NEAREST)
    draw = ImageDraw.Draw(sheet)
    for tile in range(512):
        x0, y0 = (tile % 16) * 8 * scale, (tile // 16) * 8 * scale
        draw.rectangle([x0, y0, x0 + 8 * scale - 1, y0 + 8 * scale - 1], outline=(50, 50, 50, 255))
        draw.text((x0 + 1, y0 + 1), f"{tile:03d}", fill=(255, 80, 80, 255))
    return sheet


def palette_preview(prg: bytes, colors: bytes, path: Path) -> None:
    image = Image.new("RGB", (16 * 16, 4 * 16), (0, 0, 0))
    draw = ImageDraw.Draw(image)
    for i, index in enumerate(colors):
        rgb = NES_RGB[index & 0x3F]
        x, y = (i % 16) * 16, (i // 16) * 16
        draw.rectangle([x, y, x + 15, y + 15], fill=rgb)
        draw.text((x + 1, y + 1), f"{index:02X}", fill=(0, 0, 0) if sum(rgb) > 300 else (255, 255, 255))
    image.save(path)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("rom", type=Path, nargs="?", default=DEFAULT_ROM)
    args = parser.parse_args()
    data = args.rom.read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    if digest != ROM_SHA256:
        raise SystemExit(f"unexpected ROM sha256: {digest}")
    meta = parse_rom(data)
    prg = data[meta["prg_offset"] : meta["prg_offset"] + meta["prg_bytes"]]
    chr_data = data[meta["chr_offset"] : meta["chr_offset"] + meta["chr_bytes"]]

    OUT.mkdir(parents=True, exist_ok=True)
    render_tiles(chr_data).save(OUT / "chr-tiles.png")
    render_labeled(chr_data).save(OUT / "chr-tiles-labeled.png")

    bg_palette = read_bytes(prg, 0xD565, 16)
    bg_palette_2 = read_bytes(prg, 0xD575, 16)
    sprite_palette = read_bytes(prg, 0xD555, 16)
    palette_preview(prg, bg_palette, OUT / "bg-palette.png")
    palette_preview(prg, sprite_palette, OUT / "sprite-palette.png")

    unique = len({chr_data[i * 16 : (i + 1) * 16] for i in range(512)})
    report = {
        "rom": {"path": str(args.rom.relative_to(ROOT)), "sha256": digest, **meta},
        "chr": {"tiles": 512, "unique": unique, "offset": meta["chr_offset"]},
        "palettes": {
            "background0": {"cpu": "D565", "prg_offset": (0xD565 - 0x8000) & 0x3FFF, "bytes": list(bg_palette)},
            "background1": {"cpu": "D575", "prg_offset": (0xD575 - 0x8000) & 0x3FFF, "bytes": list(bg_palette_2)},
            "sprites": {"cpu": "D555", "prg_offset": (0xD555 - 0x8000) & 0x3FFF, "bytes": list(sprite_palette)},
        },
        "artifacts": ["chr-tiles.png", "chr-tiles-labeled.png", "bg-palette.png", "sprite-palette.png"],
        "note": "Preview RGB is a conventional 2C02 table; final colors come from FCEUX gui.parsecolor.",
    }
    (OUT / "static-report.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
