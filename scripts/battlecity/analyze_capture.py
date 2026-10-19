#!/usr/bin/env python3
"""Inspect a captured Battle City PPU/OAM frame and render the playfield.

Research helper only; reads .local captures and writes previews there.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[2]
NES_RGB = [
    (84,84,84),(0,30,116),(8,16,144),(48,0,136),(68,0,100),(92,0,48),(84,4,0),(60,24,0),
    (32,42,0),(8,58,0),(0,64,0),(0,60,0),(0,50,60),(0,0,0),(0,0,0),(0,0,0),
    (152,150,152),(8,76,196),(48,50,236),(92,30,228),(136,20,176),(160,20,100),(152,34,32),(120,60,0),
    (84,90,0),(40,114,0),(8,124,0),(0,118,40),(0,102,120),(0,0,0),(0,0,0),(0,0,0),
    (236,238,236),(76,154,236),(120,124,236),(176,98,236),(228,84,236),(236,88,180),(236,106,100),(212,136,32),
    (160,170,0),(116,196,0),(76,208,32),(56,204,108),(56,180,204),(60,60,60),(0,0,0),(0,0,0),
    (236,238,236),(168,204,236),(188,188,236),(212,178,236),(236,174,236),(236,174,212),(236,180,176),(228,196,144),
    (204,210,120),(180,222,120),(168,226,144),(152,226,180),(160,214,228),(160,162,160),(0,0,0),(0,0,0),
]


def decode_tile(data: bytes):
    return [[((data[y] >> (7 - x)) & 1) | (((data[y + 8] >> (7 - x)) & 1) << 1) for x in range(8)] for y in range(8)]


def render_background(ppu: bytes, name: bytes, ctrl: int, palette: bytes) -> Image.Image:
    image = Image.new("RGB", (256, 240))
    bank = 0x1000 if ctrl & 0x10 else 0
    for ty in range(30):
        for tx in range(32):
            tile = name[ty * 32 + tx]
            pixels = decode_tile(ppu[bank + tile * 16 : bank + (tile + 1) * 16])
            attr = name[0x3C0 + (ty // 4) * 8 + tx // 4]
            shift = ((ty % 4) // 2) * 4 + ((tx % 4) // 2) * 2
            pal = (attr >> shift) & 3
            for dy, row in enumerate(pixels):
                for dx, index in enumerate(row):
                    color = palette[(pal * 4 + index) if index else 0] & 0x3F
                    image.putpixel((tx * 8 + dx, ty * 8 + dy), NES_RGB[color])
    return image


def render_sprites(ppu: bytes, oam: bytes, ctrl: int, palette: bytes) -> Image.Image:
    image = Image.new("RGBA", (256, 240), (0, 0, 0, 0))
    tall = bool(ctrl & 0x20)
    bank = 0x1000 if ctrl & 0x08 else 0
    height = 16 if tall else 8
    for i in range(64):
        y, tile, attr, x = oam[i * 4 : i * 4 + 4]
        y += 1
        start = ((tile & 1) * 0x1000 + (tile & 0xFE) * 16) if tall else bank + tile * 16
        rows = decode_tile(ppu[start : start + 16])
        if tall:
            rows += decode_tile(ppu[start + 16 : start + 32])
        if attr & 0x80:
            rows = rows[::-1]
        if attr & 0x40:
            rows = [row[::-1] for row in rows]
        for dy, row in enumerate(rows):
            for dx, index in enumerate(row):
                if not index:
                    continue
                color = palette[0x10 + (attr & 3) * 4 + index] & 0x3F
                px, py = x + dx, y + dy
                if 0 <= px < 256 and 0 <= py < 240:
                    image.putpixel((px, py), NES_RGB[color] + (255,))
    return image


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("frame", type=int)
    parser.add_argument("--dir", type=Path, default=ROOT / ".local/battlecity/reference/probe-start")
    args = parser.parse_args()
    base = args.dir / f"frame-{args.frame:04d}"
    ppu = (base.with_name(base.name + "-ppu.bin")).read_bytes()
    name = (base.with_name(base.name + "-name.bin")).read_bytes()
    oam = (base.with_name(base.name + "-oam.bin")).read_bytes()
    palette = (base.with_name(base.name + "-palette.bin")).read_bytes()
    ctrl = (base.with_name(base.name + "-ctrl.bin")).read_bytes()[0]
    out = args.dir / "analysis"
    out.mkdir(exist_ok=True)
    render_background(ppu, name, ctrl, palette).save(out / f"bg-{args.frame}.png")
    render_sprites(ppu, oam, ctrl, palette).save(out / f"sprites-{args.frame}.png")
    # Playfield is a 13x13 cell board at nametable offset (2,2), each cell 2x2 tiles.
    grid = []
    for cy in range(13):
        row = []
        for cx in range(13):
            tiles = []
            for sy in range(2):
                for sx in range(2):
                    tx, ty = 2 + cx * 2 + sx, 2 + cy * 2 + sy
                    tiles.append(name[ty * 32 + tx])
            row.append(tiles)
        grid.append(row)
    sprites = []
    for i in range(64):
        y, tile, attr, x = oam[i * 4 : i * 4 + 4]
        if y == 0xF8:
            continue
        sprites.append({"index": i, "x": x, "y": y + 1, "tile": tile, "attr": attr})
    report = {
        "ctrl": ctrl,
        "palette": list(palette),
        "playfield_tiles": grid,
        "sprites": sprites,
    }
    (out / f"frame-{args.frame}.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"ctrl": ctrl, "sprites": len(sprites), "out": str(out)}, indent=2))


if __name__ == "__main__":
    main()
