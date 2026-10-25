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
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "games/fortressblue/assets/manifest.json"
BASE = "https://cdn.ichenj.com/bga/v20261002-1"
TRANSFER_TIMEOUT = 45
DOWNLOAD_ATTEMPTS = 4
RETRY_DELAY = 1


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def fetch(url: str) -> bytes:
    last = None
    for attempt in range(DOWNLOAD_ATTEMPTS):
        try:
            # A socket idle timeout does not bound a slowly trickling body.
            # curl bounds the complete transfer and uses the reachable IPv4
            # route consistently on hosted runners. Bytes still pass the
            # manifest length and SHA-256 checks before being written.
            result = subprocess.run(
                ["curl", "--ipv4", "--fail", "--location", "--silent", "--show-error",
                 "--connect-timeout", str(min(10, TRANSFER_TIMEOUT)),
                 "--max-time", str(TRANSFER_TIMEOUT),
                 "--user-agent", "wire-board-asset-fetch/1", "--url", url],
                capture_output=True, timeout=TRANSFER_TIMEOUT + 2,
            )
            if result.returncode:
                raise RuntimeError(result.stderr.decode(errors="replace").strip())
            return result.stdout
        except Exception as error:  # noqa: BLE001 - retry timeouts and transient 5xx
            last = error
            print(f"download attempt {attempt + 1}/{DOWNLOAD_ATTEMPTS}: {url}: {error}", file=sys.stderr, flush=True)
            if attempt + 1 < DOWNLOAD_ATTEMPTS:
                time.sleep(RETRY_DELAY * (1 + attempt * 2))
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

    downloaded = 0
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        pending = {pool.submit(ensure, entry) for entry in entries}
        try:
            for done, future in enumerate(concurrent.futures.as_completed(pending), 1):
                downloaded += bool(future.result())
                print(f"fallback media checked: {done}/{len(entries)}", flush=True)
        except Exception:
            for future in pending:
                future.cancel()
            raise
    print(f"fallback media ready: {len(entries)} objects, downloaded {downloaded}")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"fetch-assets: {error}", file=sys.stderr)
        raise
