#!/usr/bin/env python3
"""Generate and validate the 12 symmetric 13x13 battlecity maps.

Map cells: 0 empty, 1 brick, 2 steel, 3 water, 4 trees, 5 ice.
Bases live at (6,0)/(6,12); the 5-cell brick ring around each base is fixed.
"""
from __future__ import annotations

import json
import sys
from collections import deque
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "games/battlecity/maps/catalog.json"
COLS = ROWS = 13
BASE_TOP = (6, 0)
BASE_BOTTOM = (6, 12)
SPAWNS_TOP = [(2, 2), (6, 2), (10, 2)]
SPAWNS_BOTTOM = [(x, 10) for x, _ in SPAWNS_TOP]


def ring(cx: int, cy: int) -> list[tuple[int, int]]:
    if cy == 0:
        return [(cx - 1, 0), (cx + 1, 0), (cx - 1, 1), (cx, 1), (cx + 1, 1)]
    return [(cx - 1, 12), (cx + 1, 12), (cx - 1, 11), (cx, 11), (cx + 1, 11)]


def blank() -> list[list[int]]:
    return [[0] * COLS for _ in range(ROWS)]


def put(grid, x, y, w, h, code):
    for yy in range(y, y + h):
        for xx in range(x, x + w):
            if 0 <= xx < COLS and 0 <= yy < ROWS:
                grid[yy][xx] = code


def symmetric(grid, x, y, w, h, code):
    put(grid, x, y, w, h, code)
    put(grid, COLS - x - w, ROWS - y - h, w, h, code)


def build(shapes) -> list[list[int]]:
    grid = blank()
    for x, y, w, h, code in shapes:
        symmetric(grid, x, y, w, h, code)
    # Base cells remain empty; the server places the base object.
    grid[BASE_TOP[1]][BASE_TOP[0]] = 0
    grid[BASE_BOTTOM[1]][BASE_BOTTOM[0]] = 0
    for x, y in ring(*BASE_TOP) + ring(*BASE_BOTTOM):
        grid[y][x] = 1
    # All three spawn cells and their forward cell stay clear.  A brick in
    # front is allowed, but water/steel must not trap a spawning tank.
    for x, y in SPAWNS_TOP:
        grid[y][x] = 0
        if grid[y + 1][x] in (2, 3):
            grid[y + 1][x] = 0
    for x, y in SPAWNS_BOTTOM:
        grid[y][x] = 0
        if grid[y - 1][x] in (2, 3):
            grid[y - 1][x] = 0
    return grid


def reachable(grid, start, target) -> bool:
    q = deque([start])
    seen = {start}
    while q:
        x, y = q.popleft()
        if (x, y) == target:
            return True
        for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
            nx, ny = x + dx, y + dy
            if not (0 <= nx < COLS and 0 <= ny < ROWS) or (nx, ny) in seen:
                continue
            if grid[ny][nx] in (2, 3):
                continue
            seen.add((nx, ny))
            q.append((nx, ny))
    return False


def validate(mid, name, grid) -> list[str]:
    errors = []
    if len(grid) != ROWS or any(len(row) != COLS for row in grid):
        errors.append("dimensions")
    for y in range(ROWS):
        for x in range(COLS):
            if grid[y][x] != grid[ROWS - 1 - y][COLS - 1 - x]:
                errors.append(f"asymmetry at {x},{y}")
                break
    if grid[BASE_TOP[1]][BASE_TOP[0]] != 0 or grid[BASE_BOTTOM[1]][BASE_BOTTOM[0]] != 0:
        errors.append("base cell not empty")
    for x, y in ring(*BASE_TOP) + ring(*BASE_BOTTOM):
        if grid[y][x] != 1:
            errors.append(f"ring not brick at {x},{y}")
    for spawns, base in ((SPAWNS_TOP, BASE_BOTTOM), (SPAWNS_BOTTOM, BASE_TOP)):
        for sx, sy in spawns:
            if grid[sy][sx] != 0:
                errors.append(f"spawn not empty at {sx},{sy}")
            if not reachable(grid, (sx, sy), base):
                errors.append(f"spawn {sx},{sy} cannot reach {base}")
    return errors


MAPS = [
    ("classic-01", "中央大道", [
        (2, 3, 1, 7, 1), (4, 4, 1, 5, 1), (8, 4, 1, 5, 1), (5, 6, 3, 1, 2),
        (1, 7, 2, 1, 5), (10, 5, 2, 1, 5),
    ]),
    ("classic-02", "双桥", [
        (2, 4, 2, 5, 3), (6, 3, 1, 2, 1), (6, 8, 1, 2, 1), (1, 2, 3, 1, 1),
        (9, 10, 3, 1, 1), (4, 6, 5, 1, 4),
    ]),
    ("classic-03", "砖墙迷宫", [
        (2, 2, 1, 3, 1), (4, 3, 1, 2, 1), (6, 4, 1, 5, 1), (8, 2, 1, 4, 1),
        (1, 6, 3, 1, 1), (9, 6, 3, 1, 1), (2, 8, 2, 1, 4),
    ]),
    ("classic-04", "冰封湖面", [
        (2, 3, 3, 3, 5), (7, 3, 4, 2, 5), (4, 7, 5, 2, 5), (1, 6, 1, 2, 1),
        (10, 5, 1, 2, 1), (6, 6, 1, 1, 2),
    ]),
    ("classic-05", "林间伏击", [
        (1, 3, 4, 2, 4), (8, 3, 4, 2, 4), (3, 6, 2, 4, 4), (8, 6, 2, 4, 4),
        (5, 5, 3, 1, 1), (6, 7, 1, 2, 1),
    ]),
    ("classic-06", "钢铁防线", [
        (2, 3, 1, 6, 2), (4, 4, 1, 4, 1), (8, 4, 1, 4, 1), (5, 6, 3, 1, 2),
        (0, 7, 2, 1, 1), (11, 5, 2, 1, 1),
    ]),
    ("classic-07", "水泽要塞", [
        (1, 3, 3, 3, 3), (9, 7, 3, 3, 3), (4, 5, 1, 3, 1), (8, 5, 1, 3, 1),
        (6, 6, 1, 1, 2), (2, 8, 2, 1, 4),
    ]),
    ("classic-08", "十字路口", [
        (6, 2, 1, 9, 1), (2, 6, 9, 1, 1), (5, 5, 3, 3, 2), (1, 3, 1, 2, 4),
        (11, 8, 1, 2, 4),
    ]),
    ("classic-09", "环形堡垒", [
        (3, 3, 7, 1, 1), (3, 9, 7, 1, 1), (3, 3, 1, 7, 1), (9, 3, 1, 7, 1),
        (5, 5, 3, 3, 2), (1, 1, 2, 1, 3), (10, 11, 2, 1, 3),
    ]),
    ("classic-10", "废墟街区", [
        (2, 3, 2, 2, 1), (4, 5, 2, 1, 1), (8, 3, 2, 2, 1), (7, 6, 2, 2, 1),
        (1, 8, 2, 1, 4), (10, 4, 2, 1, 5), (6, 7, 1, 3, 1),
    ]),
    ("classic-11", "双子走廊", [
        (3, 3, 1, 8, 1), (9, 3, 1, 8, 1), (4, 3, 1, 2, 3), (8, 8, 1, 2, 3),
        (5, 6, 3, 1, 5), (0, 6, 2, 1, 4),
    ]),
    ("classic-12", "开阔战场", [
        (2, 4, 1, 2, 1), (10, 4, 1, 2, 1), (4, 3, 2, 1, 1), (7, 9, 2, 1, 1),
        (1, 7, 2, 2, 5), (10, 4, 2, 2, 5), (6, 6, 1, 1, 2),
    ]),
]


def main() -> None:
    maps = []
    bad = False
    for mid, name, shapes in MAPS:
        grid = build(shapes)
        errors = validate(mid, name, grid)
        if errors:
            bad = True
            print(f"{mid} {name}: {'; '.join(errors)}", file=sys.stderr)
        maps.append({"id": mid, "name": name, "cells": ["".join(str(v) for v in row) for row in grid]})
    if bad:
        raise SystemExit("map validation failed")
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps({"version": 1, "maps": maps}, ensure_ascii=False, indent=2) + "\n")
    print(f"wrote {len(maps)} maps to {OUT.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
