#!/usr/bin/env python3
"""Read local Contra archives; retain identities, a pinned DAT excerpt and REA input.

Verification boundaries: misleading archive names, invalid ZIP CRC, missing or
duplicate NES members, bad/truncated headers, unexpected trainers/trailing data,
header-only hash differences, modified payloads, and an incorrect reset mapping.
No source archive is modified. Only --output receives files; raw bytes stay local.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import urllib.request
import zipfile
import zlib

COMMIT = "12c21cf79f1e20eff6ac68b80875816c717d0a7a"
DAT_URL = (f"https://raw.githubusercontent.com/libretro/libretro-database/{COMMIT}/"
           "metadat/no-intro/Nintendo%20-%20Nintendo%20Entertainment%20System.dat")
CANDIDATES = ["03/魂斗罗日版(完全版).zip", "20/魂斗罗美版.zip",
              "42/魂斗罗(中文).zip", "06/魂斗罗.zip", "24/魂斗罗.zip"]
EXPECTED = {
    "Japan": "be9dd65be8db897978dd34533dd3a037784a8ee9",
    "USA": "979494e7869ac7ab4815fdbd1dc99f893f713fbf",
}


def digest(data):
    return {"sha256": hashlib.sha256(data).hexdigest(),
            "sha1": hashlib.sha1(data).hexdigest(),
            "crc32": f"{zlib.crc32(data):08x}", "bytes": len(data)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--rom-root", type=Path, default=Path("/Users/jason/Documents/nes/roms"))
    parser.add_argument("--output", type=Path, default=Path(".local/nes-contra-research"))
    args = parser.parse_args()
    out = args.output.resolve()
    if not out.is_relative_to(Path.cwd().resolve()):
        raise SystemExit("output must remain within the current workspace")
    out.mkdir(parents=True, exist_ok=True)
    dat = urllib.request.urlopen(DAT_URL, timeout=30).read()
    blocks = [b for b in re.findall(r"game \(\n.*?\n\)", dat.decode(), re.S)
              if re.search(r'name "Contra \((Japan|USA)\)"', b)]
    for region, sha1 in EXPECTED.items():
        assert any(f'Contra ({region}).unh' in b and sha1.upper() in b for b in blocks)
    report = {"reference": {"url": DAT_URL, "commit": COMMIT, **digest(dat), "entries": blocks},
              "archives": [], "limitations": [
                  "DAT match identifies PRG+CHR bytes, not cartridge ownership or distribution rights.",
                  "Reset slice is a static fixed-bank hypothesis; no mapper or NES runtime is emulated.",
                  "No gameplay, all-stage completion, timing or bot behavior is verified by this probe."]}
    for rel in CANDIDATES:
        path = args.rom_root / rel
        before = digest(path.read_bytes())
        entry = {"path": str(path), **before, "members": []}
        with zipfile.ZipFile(path) as archive:
            assert archive.testzip() is None, rel
            for member in archive.infolist():
                if member.is_dir():
                    continue
                raw = archive.read(member)
                item = {"name": member.filename, **digest(raw)}
                if raw[:4] == b"NES\x1a":
                    h = raw[:16]
                    assert len(h) == 16 and not h[6] & 4
                    prg_len, chr_len = h[4] * 16384, h[5] * 8192
                    assert len(raw) == 16 + prg_len + chr_len
                    prg, payload = raw[16:16 + prg_len], raw[16:]
                    item.update(header=h.hex(), mapper=(h[6] >> 4) | (h[7] & 240),
                                prg_bytes=prg_len, chr_bytes=chr_len, payload=digest(payload),
                                vectors={name: f"0x{int.from_bytes(prg[-6+i*2:][:2], 'little'):04x}"
                                         for i, name in enumerate(["nmi", "reset", "irq"])})
                    item["matched_original_region"] = next(
                        (region for region, sha in EXPECTED.items() if sha == item["payload"]["sha1"]), None)
                    if rel == CANDIDATES[1]:
                        assert item["matched_original_region"] == "USA" and item["mapper"] == 2
                    if rel == CANDIDATES[0]:
                        assert item["matched_original_region"] == "Japan" and item["mapper"] == 23
                        reset = int(item["vectors"]["reset"], 16)
                        assert reset == 0xF80D and prg_len == 131072
                        offset = prg_len - (0x10000 - reset)
                        sample = prg[offset:]
                        (out / "contra-j-reset-f80d.bin").write_bytes(sample)
                        report["rea_sample"] = {"path": "contra-j-reset-f80d.bin", **digest(sample),
                                                "rom_offset": 16 + offset, "prg_offset": offset,
                                                "cpu_start": "0xf80d", "cpu_end": "0xffff",
                                                "mapping": "last 8 KiB PRG bank at 0xe000; reset-vector slice"}
                entry["members"].append(item)
        assert sum("mapper" in item for item in entry["members"]) == (1 if rel in CANDIDATES[:3] else 0)
        assert digest(path.read_bytes()) == before, f"source changed: {path}"
        report["archives"].append(entry)
    report["passed"] = True
    (out / "rom-inspection.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"passed": True, "output": str(out / "rom-inspection.json"),
                      "sample": report["rea_sample"]}, ensure_ascii=False))


if __name__ == "__main__":
    main()
