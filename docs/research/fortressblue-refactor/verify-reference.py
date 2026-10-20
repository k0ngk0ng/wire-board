#!/usr/bin/env python3
"""Verify retained official reference artifacts without network access."""
import hashlib
import json
from pathlib import Path
import subprocess


root = Path(__file__).resolve().parents[3]
evidence = root / ".local/fr/evidence"
manifest = json.loads((evidence / "evidence.json").read_text())
failures = []
for entry in manifest["files"]:
    path = evidence / entry["file"]
    if not path.is_file():
        failures.append(f"missing: {entry['file']}")
    elif hashlib.sha256(path.read_bytes()).hexdigest() != entry["sha256"]:
        failures.append(f"digest mismatch: {entry['file']}")
for name in ("steam-gameplay.mp4", "steam-pv.mp4", "shot-camera-impact-23-34.mp4"):
    result = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", str(evidence / name), "-f", "null", "-"],
        capture_output=True, text=True,
    )
    if result.returncode or result.stderr.strip():
        failures.append(f"decode failed: {name}: {result.stderr.strip()}")
result = {
    "passed": not failures,
    "files_checked": len(manifest["files"]),
    "video_decodes_checked": 3,
    "failures": failures,
}
(evidence / "verification.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
print(json.dumps(result, ensure_ascii=False, indent=2))
raise SystemExit(bool(failures))
