#!/usr/bin/env python3
"""Publish battlecity's immutable asset pack to OSS.

Default is a dry run.  --execute performs immutable PUTs after verifying the
local pack and refusing to overwrite any existing object with different bytes.
"""
from __future__ import annotations

import argparse
import base64
import hashlib
import hmac
import json
import urllib.error
import urllib.request
from email.utils import formatdate
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BUCKET = "stoneage-assets"
ENDPOINT = f"https://{BUCKET}.oss-cn-shanghai.aliyuncs.com"
PREFIX = "bga/v20261002-1"
CDN = "https://cdn.ichenj.com/" + PREFIX
PACK = ROOT / ".local/battlecity-pack/battlecity/v1"
MANIFEST = ROOT / "games/battlecity/assets/manifest.json"
KEY = (ROOT / "config/secrets/oss-access-key-id").read_text().strip()
SECRET = (ROOT / "config/secrets/oss-access-key-secret").read_text().strip()


def request(method, path, body=None, mime=""):
    date = formatdate(usegmt=True)
    md5 = base64.b64encode(hashlib.md5(body).digest()).decode() if body else ""
    extra = "x-oss-forbid-overwrite:true\n" if method == "PUT" else ""
    canonical = f"{method}\n{md5}\n{mime}\n{date}\n{extra}/{BUCKET}/{path}"
    signature = base64.b64encode(hmac.new(SECRET.encode(), canonical.encode(), hashlib.sha1).digest()).decode()
    headers = {"Date": date, "Authorization": f"OSS {KEY}:{signature}"}
    if method == "PUT":
        headers.update({
            "Content-MD5": md5,
            "Content-Type": mime,
            "x-oss-forbid-overwrite": "true",
            "Cache-Control": "public, max-age=31536000, immutable",
        })
    return urllib.request.urlopen(
        urllib.request.Request(ENDPOINT + "/" + path, data=body, method=method, headers=headers),
        timeout=45,
    )


def mime_for(kind: str) -> str:
    return {"atlas": "image/webp", "atlas-meta": "application/json", "bgm": "audio/mpeg", "sfx": "audio/mpeg"}[kind]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--execute", action="store_true", help="perform immutable uploads")
    parser.add_argument("--atlas-v2", action="store_true", help="publish corrected atlas only")
    args = parser.parse_args()
    version = "v2" if args.atlas_v2 else "v1"
    manifest = json.loads((MANIFEST.with_name("manifest-v2.json") if args.atlas_v2 else MANIFEST).read_text())
    pack = PACK.parent / version
    assert manifest["prefix"] == PREFIX + "/battlecity/" + version + "/"
    # Confirm bucket and credentials against an existing immutable object.
    known = json.loads((ROOT / "games/snooker/assets/manifest.json").read_text())["entries"][0]
    with request("GET", PREFIX + known["urlPath"]) as response:
        assert hashlib.sha256(response.read()).hexdigest() == known["sha256"]
    results = []
    for entry in manifest["entries"]:
        source = pack / entry["file"]
        data = source.read_bytes()
        assert hashlib.sha256(data).hexdigest() == entry["sha256"], f"local hash {entry['file']}"
        assert len(data) == entry["bytes"], f"local size {entry['file']}"
        path = PREFIX + entry["urlPath"]
        mime = mime_for(entry["kind"])
        try:
            with request("GET", path) as response:
                existing = response.read()
            assert hashlib.sha256(existing).hexdigest() == entry["sha256"], "immutable collision " + path
            status = "existing"
        except urllib.error.HTTPError as error:
            if error.code != 404:
                raise
            status = "to-upload"
            if args.execute:
                with request("PUT", path, data, mime) as response:
                    assert response.status == 200
                status = "uploaded"
        if args.execute:
            with request("GET", path) as response:
                assert hashlib.sha256(response.read()).hexdigest() == entry["sha256"]
            with urllib.request.urlopen(CDN + entry["urlPath"], timeout=45) as response:
                assert hashlib.sha256(response.read()).hexdigest() == entry["sha256"]
                assert response.headers.get_content_type() == mime
        results.append({"path": entry["urlPath"], "sha256": entry["sha256"], "bytes": len(data), "status": status})
        print(status, entry["urlPath"], flush=True)
    report_prefix = "oss-v2" if args.atlas_v2 else "oss"
    outside = ROOT / ".local/battlecity" / f"{report_prefix}-{'publish' if args.execute else 'plan'}.json"
    outside.parent.mkdir(parents=True, exist_ok=True)
    outside.write_text(json.dumps({"executed": args.execute, "objects": results}, indent=2) + "\n")
    print(f"{'published' if args.execute else 'planned'} {len(results)} immutable objects")


if __name__ == "__main__":
    main()
