"""Verify the tetris original audio manifest, WAV headers, local fallback and dist bytes."""
import argparse
import hashlib
import json
import struct
import urllib.request
import sys
from pathlib import Path

root = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(root / "scripts"))
from resource_store import configuration, published_url
parser = argparse.ArgumentParser()
parser.add_argument("--dist", action="store_true")
parser.add_argument("--oss", action="store_true")
args = parser.parse_args()

manifest_path = root / "games/tetris/assets/manifest.json"
manifest = json.loads(manifest_path.read_text())
sha = lambda data: hashlib.sha256(data).hexdigest()

assert manifest["version"] == "tetris/v1"
assert manifest["generator"] == "scripts/tetris/prepare-audio.mjs"
assert manifest["license"].startswith("项目原创")
assert manifest["prefix"] == "bga/tetris/"
assert len(manifest["entries"]) == 9
assert sha((root / manifest["generator"]).read_bytes()) == manifest["generatorSha256"]

def check_wav(data):
    assert data[:4] == b"RIFF" and data[8:12] == b"WAVE", "not a WAV"
    channels, rate = struct.unpack_from("<HI", data, 22)
    bits = struct.unpack_from("<H", data, 34)[0]
    assert channels == 1 and rate == 44100 and bits == 16, (channels, rate, bits)
    assert struct.unpack_from("<I", data, 4)[0] == len(data) - 8
    assert struct.unpack_from("<I", data, 40)[0] == len(data) - 44
    assert any(data[44:]), "silent sample"

results = []
for entry in manifest["entries"]:
    target = published_url(entry, "tetris", configuration()["CDN_ORIGIN"])
    data = (root / entry["output"]).read_bytes()
    assert sha(data) == entry["sha256"], entry["file"]
    assert len(data) == entry["bytes"], entry["file"]
    assert entry["urlPath"] == f"/tetris/v1/audio/{entry['file']}"
    assert (root / entry["output"]).relative_to(
        root / "games/tetris/assets/public"
    ).as_posix() == entry["urlPath"].lstrip("/")
    check_wav(data)
    if args.dist:
        packaged = root / "web/dist" / entry["urlPath"].lstrip("/")
        assert packaged.exists(), f"missing dist fallback {packaged}"
        assert sha(packaged.read_bytes()) == entry["sha256"], packaged
    if args.oss:
        url = target
        with urllib.request.urlopen(url, timeout=40) as response:
            assert sha(response.read()) == entry["sha256"], url
    results.append(
        {"path": entry["urlPath"], "sha256": entry["sha256"], "passed": True}
    )

out = root / ".local/tetris"
out.mkdir(parents=True, exist_ok=True)
name = "oss-verify.json" if args.oss else "assets-verify.json"
(out / name).write_text(
    json.dumps({"passed": True, "dist": args.dist, "objects": results}, indent=2) + "\n"
)
print(f"Verified {len(results)} original audio/fallback objects")
