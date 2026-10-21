#!/usr/bin/env python3
"""Game-scoped delivery checks; no ROM or image extraction required in CI."""
import argparse
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--dist', action='store_true')
args = parser.parse_args()
subprocess.run(['node', '--test', 'scripts/basketball/asset-build.test.mjs', 'scripts/basketball/asset-base.test.mjs'], cwd=root, check=True)
if args.dist:
    subprocess.run(['node', 'games/basketball/build/verify-assets.mjs', 'web/dist'], cwd=root, check=True)
