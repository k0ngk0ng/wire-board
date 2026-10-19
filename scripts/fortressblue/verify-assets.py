#!/usr/bin/env python3
"""Verify the original fortressblue WAV bank and its built/local copies."""
from __future__ import annotations

import argparse
import array
import hashlib
import json
import wave
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "games/fortressblue/assets/manifest.json"
LOCAL = ROOT / "games/fortressblue/assets/public"
DIST = ROOT / "web/dist"


def check(path: Path, entry: dict):
    data = path.read_bytes()
    assert len(data) == entry["bytes"], f"bytes {path}"
    assert hashlib.sha256(data).hexdigest() == entry["sha256"], f"sha {path}"
    with wave.open(str(path), "rb") as audio:
        assert audio.getnchannels() == 1, path
        assert audio.getsampwidth() == 2, path
        assert audio.getframerate() == 44100, path
        frames = audio.readframes(audio.getnframes())
        duration = audio.getnframes() / audio.getframerate()
        assert duration >= 0.08, (path, duration)
    samples = array.array("h", frames)
    peak = max(abs(v) for v in samples)
    assert peak > 64, f"silent {path}"
    assert peak < 32767, f"clipped {path}"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist", action="store_true")
    args = parser.parse_args()
    manifest = json.loads(MANIFEST.read_text())
    entries = manifest["entries"]
    assert entries and manifest["license"].startswith("项目原创")
    for entry in entries:
        rel = entry["output"].removeprefix("games/fortressblue/assets/public/")
        check(LOCAL / rel, entry)
        if args.dist:
            check(DIST / rel, entry)
    out = ROOT / ".local/fortressblue"
    out.mkdir(parents=True, exist_ok=True)
    (out / "assets-verify.json").write_text(
        json.dumps(
            {
                "passed": True,
                "entries": len(entries),
                "bytes": sum(e["bytes"] for e in entries),
                "dist": args.dist,
            },
            indent=2,
        )
        + "\n"
    )
    print(f"verified {len(entries)} original WAV files")


if __name__ == "__main__":
    main()
