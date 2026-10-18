#!/usr/bin/env python3
"""Publish immutable SPF2T media. Verify existing bytes; never overwrite."""
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
PACK = ROOT / ".local/spf2t-pack/spf2t/v1"
MANIFEST = ROOT / "games/spf2t/assets/manifest.json"
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
        headers.update(
            {
                "Content-MD5": md5,
                "Content-Type": mime,
                "x-oss-forbid-overwrite": "true",
                "Cache-Control": "public, max-age=31536000, immutable",
            }
        )
    return urllib.request.urlopen(
        urllib.request.Request(ENDPOINT + "/" + path, data=body, method=method, headers=headers),
        timeout=45,
    )


def main():
    manifest = json.loads(MANIFEST.read_text())
    assert manifest["prefix"] == "bga/v20261002-1/spf2t/v1/"
    # Confirm the bucket and credentials against an existing immutable object.
    known = json.loads((ROOT / "games/snooker/assets/manifest.json").read_text())["entries"][0]
    with request("GET", PREFIX + known["urlPath"]) as response:
        assert hashlib.sha256(response.read()).hexdigest() == known["sha256"]
    results = []
    for entry in manifest["entries"]:
        source = PACK / entry["file"]
        data = source.read_bytes()
        assert hashlib.sha256(data).hexdigest() == entry["sha256"], f"local hash {entry['file']}"
        assert len(data) == entry["bytes"], f"local size {entry['file']}"
        path = PREFIX + entry["urlPath"]
        mime = "image/webp" if entry["urlPath"].endswith(".webp") else "audio/mpeg"
        try:
            with request("GET", path) as response:
                assert hashlib.sha256(response.read()).hexdigest() == entry["sha256"], "immutable collision " + path
        except urllib.error.HTTPError as error:
            if error.code != 404:
                raise
            with request("PUT", path, data, mime) as response:
                assert response.status == 200
        with request("GET", path) as response:
            assert hashlib.sha256(response.read()).hexdigest() == entry["sha256"]
        with urllib.request.urlopen(CDN + entry["urlPath"], timeout=45) as response:
            assert hashlib.sha256(response.read()).hexdigest() == entry["sha256"]
            assert response.headers.get_content_type() == mime
        results.append({"path": entry["urlPath"], "sha256": entry["sha256"], "bytes": len(data), "cdnVerified": True})
        print("Verified", entry["urlPath"], flush=True)
    out = ROOT / ".local/spf2t/oss-publish.json"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps({"passed": True, "objects": results}, indent=2) + "\n")
    print(f"Published and verified {len(results)} immutable objects")


if __name__ == "__main__":
    main()
