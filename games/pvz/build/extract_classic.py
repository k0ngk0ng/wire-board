"""Read the classic PC PopCap PAK without running its Windows executable."""
import argparse
import hashlib
import json
from pathlib import Path
import struct


def read_pak(encoded):
    data = bytes(value ^ 0xf7 for value in encoded)
    if len(data) < 9 or struct.unpack_from("<II", data) != (0xBAC04AC0, 0):
        raise ValueError("Unsupported PopCap PAK signature/version")
    at, entries, seen = 8, [], set()
    while True:
        if at >= len(data):
            raise ValueError("Truncated PAK file table")
        flag = data[at]
        at += 1
        if flag == 0x80:
            break
        if flag != 0 or at >= len(data):
            raise ValueError("Unsupported PAK entry flag")
        length = data[at]
        at += 1
        if not length or at + length + 12 > len(data):
            raise ValueError("Truncated PAK entry")
        name = data[at:at + length].decode("utf8").replace("\\", "/")
        at += length
        if any(part in ("", ".", "..") for part in name.split("/")) or ":" in name or "\x00" in name:
            raise ValueError("Unsafe PAK member path")
        if name.casefold() in seen:
            raise ValueError("Duplicate PAK member")
        seen.add(name.casefold())
        size, timestamp = struct.unpack_from("<IQ", data, at)
        at += 12
        entries.append({"path": name, "size": size, "filetime": timestamp})
    for entry in entries:
        end = at + entry["size"]
        if end > len(data):
            raise ValueError("Truncated PAK member data")
        entry.update(offset=at, data=data[at:end])
        at = end
    if at != len(data):
        raise ValueError("Unexplained trailing PAK data")
    return entries


def extract(encoded, output):
    entries = read_pak(encoded)
    output.mkdir(parents=True, exist_ok=False)
    manifest = []
    for entry in entries:
        target = output / entry["path"]
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(entry["data"])
        manifest.append({key: value for key, value in entry.items() if key != "data"} | {
            "sha256": hashlib.sha256(entry["data"]).hexdigest(),
        })
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("pak", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--manifest", type=Path, required=True)
    args = parser.parse_args()
    encoded = args.pak.read_bytes()
    entries = extract(encoded, args.output)
    args.manifest.write_text(json.dumps({
        "source": str(args.pak), "sha256": hashlib.sha256(encoded).hexdigest(),
        "members": entries,
    }, indent=2) + "\n")
    print(f"Extracted {len(entries)} members into {args.output}")


if __name__ == "__main__":
    main()
