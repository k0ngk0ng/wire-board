#!/usr/bin/env python3
"""Recheck retained Contra research against the original ROM; no game execution."""

import argparse
import hashlib
import json
import zipfile
import zlib
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
EVIDENCE = ROOT / "docs/research/nes-contra/evidence"
ROM_SHA256 = "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--rom-zip",
        type=Path,
        default=Path("/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip"),
    )
    args = parser.parse_args()
    sources = {}

    def read_json(path):
        raw = path.read_bytes()
        sources[str(path.relative_to(ROOT))] = hashlib.sha256(raw).hexdigest()
        return json.loads(raw)

    with zipfile.ZipFile(args.rom_zip) as archive:
        rom = archive.read("Contra (J).nes")
    digest = hashlib.sha256(rom).hexdigest()
    assert digest == ROM_SHA256, "Original ROM identity changed"
    assert len(rom) == 262160 and rom[:4] == b"NES\x1a"
    payload_sha1 = hashlib.sha1(rom[16:]).hexdigest()
    payload_crc32 = f"{zlib.crc32(rom[16:]):08x}"
    inspection = read_json(EVIDENCE / "rom-inspection.json")
    entry = next(
        x for x in inspection["reference"]["entries"]
        if 'name "Contra (Japan).unh"' in x
    )
    assert f"sha1 {payload_sha1.upper()}" in entry
    assert f"crc {payload_crc32.upper()}" in entry
    assert f"size {len(rom) - 16}" in entry

    index = read_json(EVIDENCE / "static-index.json")
    assert index["rom_sha256"] == digest
    banks = index["bank_manifest"]["banks"]
    assert len(banks) == 16
    for i, bank in enumerate(banks):
        assert bank["bank_8k"] == i and bank["bytes"] == 8192
        assert bank["rom_offset"] == 16 + i * 8192
        raw = rom[bank["rom_offset"]:bank["rom_offset"] + bank["bytes"]]
        assert hashlib.sha256(raw).hexdigest() == bank["sha256"]

    reset = read_json(EVIDENCE / "rea-reset-function.json")
    start = 16 + 15 * 8192 + (0xF80D - 0xE000)
    raw = rom[start:16 + 128 * 1024]
    assert len(raw) == 2035
    assert hashlib.sha256(raw).hexdigest() == reset["subject"]["digest"]["sha256"]
    assert reset["operation"] == "analyze_function"
    assert reset["parameters"]["procedure"] == "0xf80d"
    assert reset["provider"]["id"] == "ghidra"

    # Historical HTTP fixtures establish the reported contract, not a fresh run.
    fixtures = ROOT / ".local/coop-campaign/evidence"
    cooperative = []
    for players in (1, 2):
        for win in (True, False):
            name = f"TestCoopCampaignSharedHTTP-players-{players}-win-{str(win).lower()}.json"
            record = read_json(fixtures / name)
            assert record["passed"] is True
            results = [x["result"] for x in record["trace"] if x.get("result")]
            assert results, name
            for result in results:
                assert result["cooperative"] and result["rated"]
                assert result["ratingPolicy"] == "coop_campaign_v1"
                assert result["outcome"] == ("cleared" if win else "failed")
                assert len(result["players"]) == players
                assert all(p["ratingDelta"] == (20 if win else -10) for p in result["players"])
                assert all(p["won"] == win for p in result["players"])
            cooperative.append({"players": players, "win": win, "delta": 20 if win else -10})

    print(json.dumps({
        "scope": "ROM identity and retained evidence integrity; no new gameplay or E2E run",
        "checkedAt": datetime.now(timezone.utc).isoformat(),
        "passed": True,
        "rom": {"path": str(args.rom_zip), "member": "Contra (J).nes", "sha256": digest,
                "payloadSha1": payload_sha1, "payloadCrc32": payload_crc32},
        "referenceCommit": inspection["reference"]["commit"],
        "prgBanksChecked": len(banks),
        "rea": {"evidenceID": reset["evidence_id"], "provider": reset["provider"],
                "sliceFileOffset": start, "sliceBytes": len(raw),
                "source": "exported historical evidence; not a new analysis session"},
        "historicalCooperativeResults": cooperative,
        "sourceSHA256": sources,
        "limitations": [
            "Matches the retained DAT entry; does not fetch the reference database again.",
            "Does not rerun REA, HTTP fixtures, browser checks or the eight-stage campaign.",
            "Historical passing results do not certify the current uncommitted build.",
        ],
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
