#!/usr/bin/env python3
"""Verify the original Fortress 2 BLUE media pack (local, dist and CDN)."""
from __future__ import annotations

import argparse
import base64
import hashlib
import importlib.util
import json
import shutil
import subprocess
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "games/fortressblue/assets/manifest.json"
CATALOG = ROOT / "games/fortressblue/maps/catalog.json"
TANKS = ROOT / "games/fortressblue/data/tanks.json"
ITEMS = ROOT / "games/fortressblue/data/items.json"
LOCAL = ROOT / "games/fortressblue/assets/public"
DIST = ROOT / "web/dist"
CDN = "https://cdn.ichenj.com/bga/v20261002-1"


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def media_check(path: Path, entry: dict):
    data = path.read_bytes()
    assert len(data) == entry["bytes"], f"bytes {path}"
    assert sha(data) == entry["sha256"], f"sha {path}"
    suffix = path.suffix.lower()
    if suffix == ".webp":
        assert data[:4] == b"RIFF" and data[8:12] == b"WEBP", f"webp {path}"
    elif suffix == ".mp3":
        assert data[:3] == b"ID3" or (data[0] == 0xFF and data[1] & 0xE0 == 0xE0), f"mp3 {path}"
        probe_bin = shutil.which("ffprobe")
        if probe_bin:
            probe = subprocess.run(
                [probe_bin, "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(path)],
                capture_output=True, text=True, check=True,
            )
            duration = float((probe.stdout or "0").strip() or 0)
            minimum = 1.0 if "audio/maps/" in entry["file"] else 0.05
            assert duration >= minimum, f"duration {path}: {duration}"
        else:
            # Minimal CI runners do not ship ffprobe. The immutable object is
            # already byte/SHA verified; keep a conservative size floor so a
            # truncated or empty clip still fails.
            minimum_bytes = 400_000 if "audio/maps/" in entry["file"] else 1_000
            assert len(data) >= minimum_bytes, f"short media {path}: {len(data)}"
    else:
        raise AssertionError(f"unsupported media {path}")


def catalog_check():
    catalog = json.loads(CATALOG.read_text())
    assert catalog["version"] == 2 and len(catalog["maps"]) == 17
    for item in catalog["maps"]:
        bits = base64.b64decode(item["mask"])
        assert len(bits) == (item["maskCols"] * item["maskRows"] + 7) // 8, item["id"]
        bullet = base64.b64decode(item["bulletMask"])
        assert len(bullet) == len(bits) and any(bullet), item["id"]
        assert len(item["spawns"]) == 8, item["id"]
        for spawn in item["spawns"]:
            assert 0 <= spawn["x"] < item["width"] and 0 <= spawn["y"] <= item["height"], (item["id"], spawn)
    tanks = json.loads(TANKS.read_text())
    assert len(tanks["tanks"]) == 14 and tanks["explosion"]["count"] == 28
    for tank in tanks["tanks"].values():
        assert tank["hp"] > 0 and tank["primary"] > 0 and tank["sprites"].get("idle"), tank
        assert tank["bullet"] and tank["bullet"]["url"].endswith(".webp"), tank
    items = json.loads(ITEMS.read_text())["items"]
    assert [item["id"] for item in items] == [0, 1, 2, 3, 4, 25, 26, 35]
    for item in items:
        assert item["name"] and item["effect"] and item["icon"].endswith(".webp"), item


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist", action="store_true")
    parser.add_argument("--oss", action="store_true")
    args = parser.parse_args()
    manifest = json.loads(MANIFEST.read_text())
    entries = manifest["entries"]
    assert entries and manifest["license"].startswith("原版素材")
    catalog_check()
    v3_spec = importlib.util.spec_from_file_location("verify_v3", ROOT / "scripts/fortressblue/verify-v3-assets.py")
    v3_module = importlib.util.module_from_spec(v3_spec)
    v3_spec.loader.exec_module(v3_module)
    v3_result = v3_module.verify(dist=args.dist)
    v4_spec = importlib.util.spec_from_file_location("verify_supplies", ROOT / "scripts/fortressblue/verify-supply-assets.py")
    v4_module = importlib.util.module_from_spec(v4_spec)
    v4_spec.loader.exec_module(v4_module)
    v4_result = v4_module.verify(dist=args.dist)
    for entry in entries:
        rel = entry["output"].removeprefix("games/fortressblue/assets/public/")
        local = LOCAL / rel
        if local.is_file():
            media_check(local, entry)
        elif not args.dist:
            raise FileNotFoundError(local)
        if args.dist:
            media_check(DIST / rel, entry)
        if args.oss:
            with urllib.request.urlopen(CDN + entry["urlPath"], timeout=60) as response:
                data = response.read()
            assert sha(data) == entry["sha256"], f"cdn sha {entry['urlPath']}"
            assert len(data) == entry["bytes"], f"cdn bytes {entry['urlPath']}"
    out = ROOT / ".local/fortressblue"
    out.mkdir(parents=True, exist_ok=True)
    (out / "assets-verify.json").write_text(
        json.dumps({"passed": True, "entries": len(entries), "bytes": sum(e["bytes"] for e in entries),
                    "dist": args.dist, "oss": args.oss, "v3": v3_result, "v4": v4_result}, indent=2) + "\n"
    )
    print(f"verified {len(entries)} published v2 media files and {v3_result['entries']} v3 atlases / {v3_result['frames']} frames; supply v4 {v4_result['atlases']} atlases")


if __name__ == "__main__":
    main()
