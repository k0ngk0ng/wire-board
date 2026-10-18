#!/usr/bin/env python3
"""Verify the SPF2T manifest and available local/CDN objects.

A clean checkout has no commercial media (it is OSS-only), so --dist passes
with externalOnly=true when no local fallback exists. If any local fallback
exists, every manifest entry must be present and match.
"""
import argparse
import hashlib
import json
import subprocess
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
META = ROOT / "games/spf2t/assets/manifest.json"
DEFAULT_PACK = ROOT / ".local/spf2t-pack"
CDN = "https://cdn.ichenj.com/bga/v20261002-1"


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def check_media(path):
    head = path.read_bytes()[:16]
    suffix = path.suffix.lower()
    if suffix == ".webp":
        assert head[:4] == b"RIFF" and head[8:12] == b"WEBP", f"invalid webp {path}"
    elif suffix == ".mp3":
        assert head[:3] == b"ID3" or (head[0] == 0xFF and head[1] & 0xE0 == 0xE0), f"invalid mp3 {path}"
    else:
        raise AssertionError(f"unexpected media {path}")


def verify_media(path, entry):
    check_media(path)
    assert path.stat().st_size == entry["bytes"], f"size {path}"
    assert sha(path) == entry["sha256"], f"sha256 {path}"
    if entry["kind"] == "portrait":
        result = subprocess.run(
            ["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0", str(path)],
            check=True, text=True, capture_output=True,
        )
        width, height = result.stdout.strip().split(",")[:2]
        assert int(width) == entry["width"] and int(height) == entry["height"], f"dimensions {path}"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist", action="store_true")
    parser.add_argument("--oss", action="store_true")
    parser.add_argument("--pack", type=Path, default=DEFAULT_PACK)
    args = parser.parse_args()
    manifest = json.loads(META.read_text())
    assert manifest["prefix"] == "bga/v20261002-1/spf2t/v1/"
    assert manifest["cdnBase"] == CDN
    assert manifest["entries"], "empty manifest"
    objects = []
    local_present = 0
    for entry in manifest["entries"]:
        relative = entry["urlPath"].lstrip("/")
        pack_path = args.pack / relative
        local_path = ROOT / "games/spf2t/assets/public" / relative
        dist_path = ROOT / "web/dist" / relative
        candidates = []
        if pack_path.exists():
            candidates.append(pack_path)
        if local_path.exists():
            candidates.append(local_path)
        if args.dist and dist_path.exists():
            candidates.append(dist_path)
        if candidates:
            local_present += 1
            for path in candidates:
                verify_media(path, entry)
        if args.oss:
            with urllib.request.urlopen(CDN + entry["urlPath"], timeout=40) as response:
                data = response.read()
                assert len(data) == entry["bytes"], f"bytes {entry['urlPath']}"
                assert hashlib.sha256(data).hexdigest() == entry["sha256"], f"sha {entry['urlPath']}"
                content = response.headers.get_content_type()
                expected = "image/webp" if entry["urlPath"].endswith(".webp") else "audio/mpeg"
                assert content == expected, f"mime {entry['urlPath']}: {content}"
        objects.append({"path": entry["urlPath"], "bytes": entry["bytes"], "sha256": entry["sha256"], "local": bool(candidates)})
    external_only = local_present == 0
    if args.dist and 0 < local_present < len(manifest["entries"]):
        raise AssertionError("partial local fallback; install the complete pack or none of it")
    out = ROOT / ".local/spf2t"
    out.mkdir(parents=True, exist_ok=True)
    name = "oss-verify.json" if args.oss else "assets-verify.json"
    (out / name).write_text(
        json.dumps(
            {"passed": True, "dist": args.dist, "oss": args.oss, "externalOnly": external_only, "objects": objects},
            ensure_ascii=False,
            indent=2,
        )
        + "\n"
    )
    print(f"Verified {len(objects)} objects (externalOnly={external_only})")


if __name__ == "__main__":
    main()
