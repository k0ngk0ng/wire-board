import argparse
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser()
parser.add_argument('--execute', action='store_true')
parser.add_argument('--atlas-v2', action='store_true')
args = parser.parse_args()
manifest = root / 'games/battlecity/assets' / ('manifest-v2.json' if args.atlas_v2 else 'manifest.json')
pack = root / '.local' / ('battlecity-pack') / 'battlecity' / ('v2' if args.atlas_v2 else 'v1')
command = [sys.executable, '-B', str(root / 'scripts/resource_store.py'), 'publish', '--game', 'battlecity', '--manifest', str(manifest), '--pack', str(pack)]
if args.execute:
    command.append('--execute')
subprocess.run(command, cwd=root, check=True)
