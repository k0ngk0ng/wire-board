#!/usr/bin/env python3
"""Capture the original 13x13 stage maps as verifiable reference evidence.

For each stage the script drives FCEUX to load that stage, then stores the
framebuffer, nametable, attribute bytes, CHR, OAM, palette and RAM under
.local/battlecity/maps/stage-NN.  The playfield is decoded from the nametable
and cross-checked byte for byte against the ROM stage table before the compact
game terrain codes are written.

Research helper only.  Nothing here is used by the web game at runtime.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / ".local/battlecity"
ROM = BASE / "rom/Battle City (J).nes"
ROM_SHA256 = "a869aead5b6957fc62002ff9636e048cc34baf0100d629b07dc51aa18f220c0c"
CAPTURE = ROOT / "scripts/battlecity/capture_sprites.py"
OUT = BASE / "maps"

STAGE_TABLE = 0x307A  # PRG file offset, 35 stages x 91 bytes
TERRAIN_TABLE = 0x1ACB  # CPU $DACB, 16 terrain codes x 4 tiles (TL,TR,BL,BR)
STAGE_BYTES = 91
COLS = ROWS = 13

# ROM terrain nibble -> game terrain code (CONTRACT section 3).
ROM_TO_GAME = {
    **{code: 1 for code in range(0x0, 0x5)},  # brick (partial and full)
    **{code: 2 for code in range(0x5, 0xA)},  # steel (partial and full)
    0xA: 3,  # water
    0xB: 4,  # trees
    0xC: 5,  # ice
    **{code: 0 for code in range(0xD, 0x10)},  # empty
}

# Cells the ROM rewrites at runtime: the bottom base and its five-brick ring.
BASE_RING = {
    (5, 11), (6, 11), (7, 11),
    (5, 12), (6, 12), (7, 12),
}
# 1P mode has no top base, so the ROM clears the top base cell.  Stage 5 in
# particular ships a brick there; the captured nametable shows empty space.
BASE_CELLS = {(6, 0), (6, 12)}


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def stage_codes(prg: bytes, stage: int) -> list[list[int]]:
    base = STAGE_TABLE + (stage - 1) * STAGE_BYTES
    rows = []
    for y in range(ROWS):
        row: list[int] = []
        for b in range(7):
            value = prg[base + y * 7 + b]
            row.extend((value >> 4, value & 0x0F))
        rows.append(row[:COLS])
    return rows


def expected_tiles(prg: bytes, codes: list[list[int]]) -> list[list[list[int]]]:
    tiles = []
    for row in codes:
        tiles.append([list(prg[TERRAIN_TABLE + code * 4 : TERRAIN_TABLE + code * 4 + 4]) for code in row])
    return tiles


def runtime_tiles(nametable: bytes) -> list[list[list[int]]]:
    return [
        [
            [nametable[(2 + y * 2 + sy) * 32 + 2 + x * 2 + sx] for sy in range(2) for sx in range(2)]
            for x in range(COLS)
        ]
        for y in range(ROWS)
    ]


def capture_stage(stage: int, force: bool) -> Path:
    out = OUT / f"stage-{stage:02d}"
    done = out / "capture-complete.json"
    if done.is_file() and not force:
        print(f"stage {stage:02d}: reuse {out}")
        return out
    cmd = [
        sys.executable,
        "-B",
        str(CAPTURE),
        "--name",
        f"stage-{stage:02d}",
        "--frames",
        "620",
        "--captures",
        "500,560",
        "--script",
        "select-stage",
        "--stage",
        str(stage),
        "--out",
        str(out),
    ]
    subprocess.run(cmd, check=True)
    return out


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--stages", default="1-12")
    parser.add_argument("--force", action="store_true")
    args = parser.parse_args()
    if sha256(ROM) != ROM_SHA256:
        raise SystemExit("unexpected ROM")
    first, _, last = args.stages.partition("-")
    stages = list(range(int(first), int(last or first) + 1))
    prg = ROM.read_bytes()[16 : 16 + 16384]
    OUT.mkdir(parents=True, exist_ok=True)
    records = []
    for stage in stages:
        out = capture_stage(stage, args.force)
        nametable = (out / "frame-0500-name.bin").read_bytes()
        palette = (out / "frame-0500-palette.bin").read_bytes()
        ctrl = (out / "frame-0500-ctrl.bin").read_bytes()[0]
        oam = (out / "frame-0500-oam.bin").read_bytes()
        if len(nametable) != 0x1000 or len(oam) != 0x100 or len(palette) != 32:
            raise SystemExit(f"stage {stage}: short capture")
        if ctrl != 0xB0:
            raise SystemExit(f"stage {stage}: unexpected PPUCTRL {ctrl:#04x}")

        codes = stage_codes(prg, stage)
        expected = expected_tiles(prg, codes)
        actual = runtime_tiles(nametable)
        mismatch = [
            (x, y, codes[y][x], actual[y][x], expected[y][x])
            for y in range(ROWS)
            for x in range(COLS)
            if (x, y) not in BASE_RING and (x, y) not in BASE_CELLS and actual[y][x] != expected[y][x]
        ]
        if mismatch:
            raise SystemExit(f"stage {stage}: nametable mismatch {mismatch[:6]}")
        runtime_base = {"6,0": actual[0][6]}
        if actual[0][6] != [0, 0, 0, 0]:
            raise SystemExit(f"stage {stage}: top base cell not empty: {actual[0][6]}")
        for x, y in BASE_RING:
            runtime_base[f"{x},{y}"] = actual[y][x]
            if (x, y) == (6, 12):
                # Intact eagle: TL=C8, TR=CA, BL=C9, BR=CB, palette 0.
                if actual[y][x] != [0xC8, 0xCA, 0xC9, 0xCB]:
                    raise SystemExit(f"stage {stage}: unexpected base tiles {actual[y][x]}")
                tx, ty = 2 + x * 2, 2 + y * 2
                attr = nametable[0x3C0 + (ty // 4) * 8 + tx // 4]
                shift = ((ty % 4) // 2) * 4 + ((tx % 4) // 2) * 2
                if (attr >> shift) & 3:
                    raise SystemExit(f"stage {stage}: base not in palette 0")
            elif any(tile not in (0x00, 0x0F) for tile in actual[y][x]):
                raise SystemExit(f"stage {stage}: unexpected ring tile at {x},{y}: {actual[y][x]}")

        game_codes = [[ROM_TO_GAME[code] for code in row] for row in codes]
        for bx, by in ((6, 0), (6, 12)):
            game_codes[by][bx] = 0  # bases are objects, not terrain
        cells = ["".join(str(code) for code in row) for row in game_codes]
        rom_codes = ["".join(f"{code:X}" for code in row) for row in codes]
        record = {
            "stage": stage,
            "grid": cells,
            "rom_codes": rom_codes,
            "runtime_base_tiles": runtime_base,
            "evidence": {
                "dir": str(out.relative_to(ROOT)),
                "frames": [500, 560],
                "png_sha256": sha256(out / "frame-0500.png"),
                "nametable_sha256": sha256(out / "frame-0500-name.bin"),
                "attribute_sha256": sha256(out / "frame-0500-palette.bin"),
                "ppu_sha256": sha256(out / "frame-0500-ppu.bin"),
                "oam_sha256": sha256(out / "frame-0500-oam.bin"),
                "ram_sha256": sha256(out / "frame-0500-ram.bin"),
            },
        }
        records.append(record)
        print(f"stage {stage:02d}: verified 13x13, {sum(c != '0' for row in cells for c in row)} occupied cells")

    payload = {
        "version": 1,
        "source": {
            "rom": "Battle City (J).nes",
            "rom_sha256": ROM_SHA256,
            "stage_table_prg_offset": f"0x{STAGE_TABLE:04X}",
            "terrain_table_prg_offset": f"0x{TERRAIN_TABLE:04X}",
            "capture": "FCEUX 2.6.6, stage poked through RAM $85, frame 500",
            "generator": "scripts/battlecity/capture_maps.py",
            "generator_sha256": sha256(Path(__file__)),
        },
        "mapping": {"0-4": 1, "5-9": 2, "A": 3, "B": 4, "C": 5, "D-F": 0},
        "stages": records,
    }
    (OUT / "terrain-13x13.json").write_text(json.dumps(payload, indent=2) + "\n")
    (OUT / "README.md").write_text(
        "# Battle City map reference captures\n\n"
        f"ROM SHA-256: `{ROM_SHA256}`\n\n"
        "Each `stage-NN/` directory holds frame 500/560 screenshots, nametable, "
        "attribute table, CHR, OAM, palette and RAM dumps produced by FCEUX 2.6.6. "
        "`terrain-13x13.json` decodes the 13x13 playfield and cross-checks every "
        "terrain cell against the ROM table at PRG `0x307A`; only the runtime "
        "bottom-base ring is exempt. Terrain mapping: 0-4 brick, 5-9 steel, "
        "A water, B trees, C ice, D-F empty.\n\n"
        "Regenerate: `.local/board-art-env/bin/python -B scripts/battlecity/capture_maps.py`\n",
    )
    print(json.dumps({"stages": len(records), "out": str(OUT)}, indent=2))


if __name__ == "__main__":
    main()
