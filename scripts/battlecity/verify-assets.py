#!/usr/bin/env python3
"""Verify battlecity's local/CDN asset pack against the committed manifest."""
from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
META = ROOT / "games/battlecity/assets/manifest.json"
PACK = ROOT / ".local/battlecity-pack/battlecity/v1"
LOCAL = ROOT / "games/battlecity/assets/public/battlecity/v1"
DIST = ROOT / "web/dist/battlecity/v1"
PREFIX = "bga/v20261002-1/battlecity/v1/"
CDN = "https://cdn.ichenj.com/bga/v20261002-1"
EXPECTED_AUDIO = {
    "bgm-battle", "bgm-start", "sfx-fire", "sfx-brick", "sfx-steel",
    "sfx-explode", "sfx-powerup", "sfx-shield", "sfx-base", "sfx-win", "sfx-lose",
}


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def media_signature(path: Path, kind: str) -> None:
    head = path.read_bytes()[:16]
    if kind == "atlas":
        assert head[:4] == b"RIFF" and head[8:12] == b"WEBP", f"invalid webp {path}"
    elif kind in ("bgm", "sfx"):
        assert head[:3] == b"ID3" or (head[0] == 0xFF and head[1] & 0xE0 == 0xE0), f"invalid mp3 {path}"
    elif kind == "atlas-meta":
        json.loads(path.read_text())


def audio_check(path: Path, want: float) -> None:
    probe = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", str(path)],
        check=True, capture_output=True, text=True,
    )
    got = float(probe.stdout.strip())
    assert abs(got - want) < 0.08, f"duration {path}: {got} vs {want}"
    raw = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", str(path), "-f", "s16le", "-ac", "1", "-ar", "44100", "-"],
        check=True, capture_output=True,
    ).stdout
    assert raw, f"empty audio {path}"
    import array
    samples = array.array("h", raw)
    if len(samples) % 2:
        samples = samples[:-1]
    peak = max(abs(v) for v in samples)
    assert peak > 64, f"silent audio {path}"
    assert peak < 32767, f"clipped audio {path}"


def choose(entry: dict, args) -> list[Path]:
    rel = entry["file"]
    candidates = []
    for root in (args.pack, LOCAL):
        p = root / rel
        if p.exists():
            candidates.append(p)
    if args.dist:
        p = DIST / rel
        if p.exists():
            candidates.append(p)
    return candidates


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist", action="store_true")
    parser.add_argument("--oss", action="store_true")
    parser.add_argument("--pack", type=Path, default=PACK)
    args = parser.parse_args()
    manifest = json.loads(META.read_text())
    assert manifest["prefix"] == PREFIX
    assert manifest["cdnBase"] == CDN
    assert manifest["entries"], "empty manifest"
    total = 0
    local_present = 0
    for entry in manifest["entries"]:
        candidates = choose(entry, args)
        if candidates:
            local_present += 1
            for path in candidates:
                data = path.read_bytes()
                assert len(data) == entry["bytes"], f"bytes {path}"
                assert sha(data) == entry["sha256"], f"sha256 {path}"
                media_signature(path, entry["kind"])
                if entry["kind"] in ("bgm", "sfx"):
                    audio_check(path, entry["duration"])
        if args.oss:
            with urllib.request.urlopen(CDN + entry["urlPath"], timeout=45) as response:
                data = response.read()
                assert len(data) == entry["bytes"], f"bytes {entry['urlPath']}"
                assert sha(data) == entry["sha256"], f"sha {entry['urlPath']}"
                expected = {
                    "atlas": "image/webp", "atlas-meta": "application/json",
                    "bgm": "audio/mpeg", "sfx": "audio/mpeg",
                }[entry["kind"]]
                assert response.headers.get_content_type() == expected, f"mime {entry['urlPath']}"
        total += entry["bytes"]

    atlas_entry = next(e for e in manifest["entries"] if e["kind"] == "atlas")
    meta_entry = next(e for e in manifest["entries"] if e["kind"] == "atlas-meta")
    atlas_path = args.pack / atlas_entry["file"]
    if not atlas_path.exists():
        atlas_path = LOCAL / atlas_entry["file"]
    if atlas_path.exists():
        probe = subprocess.run(
            ["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", str(atlas_path)],
            check=True, capture_output=True, text=True,
        )
        width, height = [int(v) for v in probe.stdout.strip().split(",")[:2]]
        assert (width, height) == (atlas_entry["width"], atlas_entry["height"]), (width, height)
        meta = json.loads((atlas_path.parent / meta_entry["file"]).read_text())
        assert len(meta["frames"]) == atlas_entry["frames"]
        for name, frame in meta["frames"].items():
            assert 0 <= frame["x"] and 0 <= frame["y"], name
            assert frame["x"] + frame["w"] <= width, name
            assert frame["y"] + frame["h"] <= height, name
        assert set(meta.get("audio", {})) == EXPECTED_AUDIO, set(meta.get("audio", {}))
        for name in ("brick", "steel", "water", "trees", "ice", "base-top", "base-bottom",
                     "powerup-star", "tank-balanced-top-up-0", "tank-armor-bottom-left-1"):
            assert name in meta["frames"], f"missing frame {name}"

    # Corrected native four-direction atlas is independent of unchanged v1 audio.
    v2 = json.loads((META.parent / "manifest-v2.json").read_text())
    assert v2["version"] == "battlecity/v2"
    assert v2["generatorSha256"] == sha((ROOT / v2["generator"]).read_bytes())
    corrected = json.loads((META.parent / "atlas.json").read_text())
    assert corrected["version"] == 2 and len(corrected["frames"]) == 101
    v2_present = 0
    for entry in v2["entries"]:
        assert entry["urlPath"] == "/battlecity/v2/" + entry["file"]
        paths = [ROOT / ".local/battlecity-pack/battlecity/v2" / entry["file"],
                 ROOT / "games/battlecity/assets/public" / entry["urlPath"].lstrip("/")]
        if args.dist:
            paths.append(ROOT / "web/dist" / entry["urlPath"].lstrip("/"))
        found = [path for path in paths if path.exists()]
        v2_present += bool(found)
        for path in found:
            assert path.stat().st_size == entry["bytes"] and sha(path.read_bytes()) == entry["sha256"], path
            media_signature(path, entry["kind"])
        if args.oss:
            with urllib.request.urlopen(CDN + entry["urlPath"], timeout=45) as response:
                data = response.read()
                assert len(data) == entry["bytes"] and sha(data) == entry["sha256"], entry["urlPath"]
                assert response.headers.get_content_type() == {"atlas": "image/webp", "atlas-meta": "application/json"}[entry["kind"]]
    assert v2_present in (0, 2), "partial v2 atlas fallback"
    assert sha((META.parent / "atlas.json").read_bytes()) == next(e["sha256"] for e in v2["entries"] if e["kind"] == "atlas-meta")

    external_only = local_present == 0
    if args.dist and 0 < local_present < len(manifest["entries"]):
        raise AssertionError("partial local fallback; install the complete pack or none")
    out = ROOT / ".local/battlecity"
    out.mkdir(parents=True, exist_ok=True)
    name = "oss-verify.json" if args.oss else "assets-verify.json"
    (out / name).write_text(json.dumps({
        "passed": True, "dist": args.dist, "oss": args.oss,
        "externalOnly": external_only, "entries": len(manifest["entries"]), "bytes": total,
        "correctedAtlasEntries": len(v2["entries"]), "correctedAtlasLocalEntries": v2_present,
    }, indent=2) + "\n")
    print(f"Verified {len(manifest['entries'])} objects, {total} bytes (externalOnly={external_only})")


if __name__ == "__main__":
    main()
