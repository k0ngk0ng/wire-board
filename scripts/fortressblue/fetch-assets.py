#!/usr/bin/env python3
"""Restore the published Fortress 2 BLUE fallback media for a clean checkout.

CI and fresh clones do not contain the gitignored v2 media directory. Fetch the
exact immutable objects recorded in assets/manifest.json so the build keeps a
same-origin fallback when the CDN is unavailable. Existing matching files are
left untouched.
"""
from __future__ import annotations

import concurrent.futures
import hashlib
import json
import sys
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "games/fortressblue/assets/manifest.json"
BASE = "https://cdn.ichenj.com/bga/v20261002-1"


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def fetch(url: str) -> bytes:
    last = None
    for attempt in range(4):
        try:
            request = urllib.request.Request(url, headers={"User-Agent": "wire-board-asset-fetch/1"})
            with urllib.request.urlopen(request, timeout=45) as response:
                return response.read()
        except Exception as error:  # noqa: BLE001 - retry timeouts and transient 5xx
            last = error
            if attempt < 3:
                time.sleep(1 + attempt * 2)
    raise RuntimeError(f"fetch failed after retries: {url}: {last}")


def main() -> None:
    manifest = json.loads(MANIFEST.read_text())
    entries = manifest["entries"]
    if not entries:
        raise SystemExit("empty asset manifest")
    def ensure(entry):
        target = ROOT / entry["output"]
        if target.is_file():
            data = target.read_bytes()
            if len(data) == entry["bytes"] and sha(data) == entry["sha256"]:
                return False
        url = BASE + entry["urlPath"]
        data = fetch(url)
        if len(data) != entry["bytes"] or sha(data) != entry["sha256"]:
            raise RuntimeError(f"asset mismatch: {url}")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        return True

    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        downloaded = sum(1 for changed in pool.map(ensure, entries) if changed)
    print(f"fallback media ready: {len(entries)} objects, downloaded {downloaded}")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"fetch-assets: {error}", file=sys.stderr)
        raise
