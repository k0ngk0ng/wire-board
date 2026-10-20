#!/usr/bin/env python3
"""Extract indexed sound resources from the preserved 2002 Crazy Arcade client.

Validation inventory (before implementation): truncated count/name/record;
unsafe resource name; out-of-bounds or overlapping ranges; unexpected trailing
index bytes; wrong source digest; WAV/MP3 bytes that don't match their format.
The client ZIP is inventoried/extracted with REA first; this parser follows the
source-bound Fx.idx offsets and never executes the Windows binary.
"""
import argparse, hashlib, json, struct
from pathlib import Path

root = Path(__file__).resolve().parents[2]
p = argparse.ArgumentParser()
p.add_argument("--client-dir", type=Path, required=True)
p.add_argument("--out", type=Path, required=True)
a = p.parse_args()
out = a.out.resolve()
if not out.is_relative_to(root):
    p.error("output must remain inside the repository")
index = (a.client_dir / "Fx.idx").read_bytes()
data = (a.client_dir / "Fx.idd").read_bytes()
# Same archive used by the retained REA extraction Evidence, not another version.
expected_index = "6be28522015c5a4b0f4b8ab86a989819d1b0b83f676566d4b99cc4c8d15aba7c"
if hashlib.sha256(index).hexdigest() != expected_index:
    p.error("Fx.idx does not match the reviewed 2002-03-13 artifact")
if hashlib.sha256(data).hexdigest() != "3554b3cda62313366d07776c48c203ddc95092b18b753b4d0986fab19ed664b2":
    p.error("Fx.idd does not match the reviewed artifact")
count = struct.unpack_from("<I", index)[0]
position = 4
entries = []
last_end = 0
for ordinal in range(count):
    length = struct.unpack_from("<I", index, position)[0]
    position += 4
    if not 0 < length < 1024 or position + length + 8 > len(index):
        p.error("invalid resource record")
    name = index[position:position + length].rstrip(b"\0").decode("cp949", errors="replace")
    position += length
    offset, size = struct.unpack_from("<II", index, position)
    position += 8
    if ".." in Path(name).parts or Path(name).is_absolute() or "\\" in name:
        p.error("unsafe resource name")
    if offset < last_end or offset + size > len(data):
        p.error("overlapping or out-of-bounds resource")
    last_end = offset + size
    entries.append({"name": name, "offset": offset, "size": size})
if position != len(index):
    p.error("unexpected trailing index bytes")
out.mkdir(parents=True, exist_ok=True)
resources = []
for entry in entries:
    name = entry["name"]
    if not name.startswith(("Wave/", "MP3/")):
        continue
    payload = data[entry["offset"]:entry["offset"] + entry["size"]]
    is_wave = name.startswith("Wave/")
    if is_wave and not (payload.startswith(b"RIFF") and payload[8:12] == b"WAVE"):
        p.error("invalid WAV signature")
    if not is_wave and not (payload.startswith(b"ID3") or payload[0] == 255 and payload[1] & 224 == 224):
        p.error("invalid MP3 signature")
    target = out / (name + (".wav" if is_wave else ".mp3"))
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(payload)
    resources.append({**entry, "sha256": hashlib.sha256(payload).hexdigest(), "output": str(target.relative_to(root))})
manifest = {"source": "https://archive.org/download/crazy_arcade_setups/2002.03.13./Crazy%20Arcade.zip", "indexSha256": hashlib.sha256(index).hexdigest(), "dataSha256": hashlib.sha256(data).hexdigest(), "indexedResources": count, "resources": resources}
(out / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
print(json.dumps({"indexed": count, "extractedAudio": len(resources), "manifest": str(out / "manifest.json")}))
