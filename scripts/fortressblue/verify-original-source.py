#!/usr/bin/env python3
"""Re-decode the original Fortress 2 BLUE client and compare with published media.

This is a local provenance check, not a CI step: the official client media is
not committed. It reads `.local/fortressblue-original` (downloaded client and
the extracted `.i16` sprites), runs the same decoder as the asset builder, and
verifies that every map, tank, bullet and source-client hash matches the
committed catalog, tank data and manifest byte-for-byte.

Run from the repository root:
    python3 -B scripts/fortressblue/verify-original-source.py
"""
from __future__ import annotations

import hashlib
import importlib.util
import json
import shutil
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PREPARE = ROOT / "scripts/fortressblue/prepare-original-assets.py"
MANIFEST_PATH = ROOT / "games/fortressblue/assets/manifest.json"
CATALOG_PATH = ROOT / "games/fortressblue/maps/catalog.json"
TANKS_PATH = ROOT / "games/fortressblue/data/tanks.json"
SOURCES_PATH = ROOT / "games/fortressblue/assets/sources.json"
OUT = ROOT / ".local/fortressblue/asset-redecode.json"


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_prepare():
    spec = importlib.util.spec_from_file_location("prepare_original", PREPARE)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main() -> int:
    manifest = json.loads(MANIFEST_PATH.read_text())
    catalog = json.loads(CATALOG_PATH.read_text())
    tanks = json.loads(TANKS_PATH.read_text())
    sources = json.loads(SOURCES_PATH.read_text())["hashes"]
    prepare = load_prepare()
    if not prepare.LOCAL.is_dir():
        raise SystemExit(
            f"missing local source workspace: {prepare.LOCAL}\n"
            "download the official web client and extract .i16 files first"
        )

    by_file = {entry["file"]: entry for entry in manifest["entries"]}
    tmp = Path(tempfile.mkdtemp(prefix="fb-redecode-"))
    prepare.OUT = tmp
    checks: list[dict] = []

    def check(name: str, got, want) -> None:
        same = got == want
        checks.append(
            {
                "name": name,
                "ok": same,
                **({} if same else {"got": got, "want": want}),
            }
        )

    for name, want in sources.items():
        check(f"source {name}", digest(prepare.LOCAL / "game" / name), want)

    for meta in catalog["maps"]:
        item = {
            "id": meta["id"],
            "name": meta["name"],
            "nameEn": meta["nameEn"],
            "stage": meta["stage"],
        }
        built = prepare.build_map(item)
        check(f"map mask {meta['id']}", built["mask"], meta["mask"])
        check(f"map spawns {meta['id']}", built["spawns"], meta["spawns"])
        for suffix in ("fg", "bg", "fg2", "thumb"):
            rel = f"maps/{meta['id']}-{suffix}.webp"
            ours = tmp / "maps" / f"{meta['id']}-{suffix}.webp"
            if rel in by_file:
                check(f"{meta['id']} {suffix}", digest(ours), by_file[rel]["sha256"])
            else:
                check(f"{meta['id']} {suffix} absent", ours.exists(), False)

    for tank in tanks["tanks"].values():
        tank_id = tank["id"]
        built = prepare.build_tank(tank_id)
        if not built:
            check(f"tank {tank_id} source", False, True)
            continue
        states, bullet = built
        for state, sprite in states.items():
            rel = f"tanks/{tank_id}-{state}.webp"
            check(f"tank {tank_id} {state}", digest(tmp / rel), by_file[rel]["sha256"])
            check(
                f"tank {tank_id} {state} dims",
                [sprite["w"], sprite["h"]],
                [tank["sprites"][state]["w"], tank["sprites"][state]["h"]],
            )
        if bullet:
            rel = f"projectiles/{tank_id}.webp"
            check(f"bullet {tank_id}", digest(tmp / rel), by_file[rel]["sha256"])

    shutil.rmtree(tmp, ignore_errors=True)
    failed = [item for item in checks if not item["ok"]]
    result = {"passed": not failed, "checks": len(checks), "failed": failed}
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if not failed else 1


if __name__ == "__main__":
    sys.exit(main())
