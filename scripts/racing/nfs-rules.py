#!/usr/bin/env python3
"""Freeze numeric driving/collision rules from verified original-scene exports.

No GLB, texture, audio, executable, or other client payload enters server data.
Every original VROAD point and width is retained; all lengths become decimetres.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import math
from pathlib import Path


TRACKS = ("france", "coastal", "uk", "park")


def number(value: float, scale: float = 1) -> float:
    value = float(value) * scale
    if not math.isfinite(value):
        raise ValueError("non-finite source geometry")
    return round(value, 8)


def hull(points: list[list[float]]) -> list[tuple[float, float]]:
    points = sorted(set((number(p[0], 10), number(p[2], 10)) for p in points))
    if len(points) < 3:
        return points
    def cross(a, b, c):
        return (b[0]-a[0])*(c[1]-a[1])-(b[1]-a[1])*(c[0]-a[0])
    lower, upper = [], []
    for p in points:
        while len(lower) > 1 and cross(lower[-2], lower[-1], p) <= 1e-10:
            lower.pop()
        lower.append(p)
    for p in reversed(points):
        while len(upper) > 1 and cross(upper[-2], upper[-1], p) <= 1e-10:
            upper.pop()
        upper.append(p)
    return lower[:-1] + upper[:-1]


def convert(route: dict) -> dict:
    digest = route["geometryHash"]
    if len(digest) != 64 or any(c not in "0123456789abcdef" for c in digest):
        raise ValueError("source geometry must have a complete immutable hash")
    points, road, barriers = [], [], []
    for p in route["points"]:
        points.append({"x": number(p["x"], 10), "y": number(p["z"], 10)})
        r = {"height": number(p["y"], 10), "left": number(p["leftWidth"], 10), "right": number(p["rightWidth"], 10)}
        if r["left"] <= 0 or r["right"] <= 0:
            raise ValueError("nonpositive original paved-road width")
        for k in ("nx", "ny", "nz", "fx", "fy", "fz", "rx", "ry", "rz"):
            r[k] = number(p[k])
        r.update(surface=int(p["surface"]), segment=int(p["segment"]))
        road.append(r)
    for source in route["barriers"]:
        if source["kind"] == "wall":
            a, b = source["a"], source["b"]
            bottom = min(a[1], b[1])
            barriers.append({"kind": "wall", "a": {"x": number(a[0], 10), "y": number(a[2], 10)}, "b": {"x": number(b[0], 10), "y": number(b[2], 10)}, "bottom": number(bottom, 10), "top": number(bottom+source["height"], 10), "segment": int(source["segment"])})
        elif source["kind"] == "object" and source.get("collisionType") != 120:
            vertices = source.get("vertices", [])
            polygon = hull(vertices)
            if not polygon:
                continue
            bottom, top = min(v[1] for v in vertices), max(v[1] for v in vertices)
            for i, a in enumerate(polygon):
                b = polygon[(i+1) % len(polygon)]
                barriers.append({"kind": "object", "a": {"x": a[0], "y": a[1]}, "b": {"x": b[0], "y": b[1]}, "bottom": number(bottom, 10), "top": number(top, 10), "segment": int(source.get("segment", -1))})
    maximum = route["bounds"]["max"]
    return {"version": 3, "id": route["id"], "name": route["name"], "geometry": digest, "width": number(maximum[0]+30, 10), "height": number(maximum[2]+30, 10), "roadWidth": number(sum(r["left"]+r["right"] for r in road)/len(road)), "length": number(route["length"], 10), "points": points, "road": road, "barriers": barriers}


def write(path: Path, value, immutable=False):
    payload = json.dumps(value, ensure_ascii=False, separators=(",", ":"), allow_nan=False).encode()+b"\n"
    if immutable and path.exists() and path.read_bytes() != payload:
        raise ValueError(f"refusing to change immutable rule data: {path}")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(payload)
    return hashlib.sha256(payload).hexdigest(), len(payload)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=Path("games/racing/assets/public/racing/v3"))
    parser.add_argument("--out", type=Path, default=Path("games/racing/server/data"))
    args = parser.parse_args()
    root = Path.cwd().resolve()
    if not args.out.resolve().is_relative_to(root):
        raise ValueError("output must stay in the current workspace")
    index = []
    for id in TRACKS:
        source = args.source/"tracks"/(id+".route.json")
        rules = convert(json.loads(source.read_text()))
        sha, count = write(args.out/"rules"/(rules["geometry"]+".json"), rules, immutable=True)
        index.append({"id": id, "geometry": rules["geometry"], "ruleSha256": sha, "bytes": count, "sourceSha256": hashlib.sha256(source.read_bytes()).hexdigest(), "points": len(rules["points"]), "barriers": len(rules["barriers"])})
    cars = json.loads((args.source/"cars.json").read_text())["cars"]
    bodies = {id.lower(): dict(zip(("width", "height", "length"), map(number, car["dimensions"]))) for id, car in cars.items()}
    write(args.out/"car-bodies.json", bodies)
    write(args.out/"index.json", index)
    print(json.dumps(index, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
