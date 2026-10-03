#!/usr/bin/env python3
"""Convert the verified classic DotA icon collection, preserving original colors."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=Path("docs/research/dota1/assets.json"))
    parser.add_argument("--output", type=Path, default=Path("web/public/dota/v1"))
    args = parser.parse_args()
    manifest = json.loads(args.manifest.read_text())
    args.output.mkdir(parents=True, exist_ok=True)
    sources = []
    for asset in manifest["assets"]:
        source = args.manifest.parent / asset["file"]
        if hashlib.sha256(source.read_bytes()).hexdigest() != asset["sha256"]:
            raise ValueError(f"Source hash mismatch: {asset['id']}")
        destination = args.output / (asset["id"] + ".webp")
        subprocess.run(["cwebp", "-quiet", "-lossless", str(source), "-o", str(destination)], check=True)
        sources.append({"id": asset["id"], "source": asset["source"], "source_page": asset["source_page"],
                        "source_sha256": asset["sha256"], "file": destination.name,
                        "sha256": hashlib.sha256(destination.read_bytes()).hexdigest()})
    (args.output / "sources.json").write_text(json.dumps({"collection": "Classic Warcraft III / DotA", "assets": sources}, ensure_ascii=False, indent=2) + "\n")
    print(f"Prepared {len(sources)} lossless icons ({sum(p.stat().st_size for p in args.output.glob('*.webp')):,} bytes).")


if __name__ == "__main__":
    main()
