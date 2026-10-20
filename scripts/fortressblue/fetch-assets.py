#!/usr/bin/env python3
"""Restore the published Fortress 2 BLUE fallback media for a clean checkout.

CI and fresh clones do not contain the gitignored v2 media directory. Fetch the
exact immutable objects recorded in assets/manifest.json so the build keeps a
same-origin fallback when the CDN is unavailable. Existing matching files are
left untouched.
"""
from __future__ import annotations

import hashlib
import json
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "games/fortressblue/assets/manifest.json"
BASE = "https://cdn.ichenj.com/bga/v20261002-1"


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def main() -> None:
    manifest = json.loads(MANIFEST.read_text())
    entries = manifest["entries"]
    if not entries:
        raise SystemExit("empty asset manifest")
    downloaded = 0
    for entry in entries:
        target = ROOT / entry["output"]
        if target.is_file():
            data = target.read_bytes()
            if len(data) == entry["bytes"] and sha(data) == entry["sha256"]:
                continue
        url = BASE + entry["urlPath"]
        with urllib.request.urlopen(url, timeout=90) as response:
            data = response.read()
        if len(data) != entry["bytes"] or sha(data) != entry["sha256"]:
            raise SystemExit(f"asset mismatch: {url}")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        downloaded += 1
    print(f"fallback media ready: {len(entries)} objects, downloaded {downloaded}")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"fetch-assets: {error}", file=sys.stderr)
        raise
