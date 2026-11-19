import argparse
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser()
parser.add_argument('--execute', action='store_true')
parser.add_argument('--remake', action='store_true')
args = parser.parse_args()
manifest = root / 'games/racing/assets' / ('manifest-v2.json' if args.remake else 'manifest.json')
pack = root / '.local' / ('racing-pack') / 'racing' / 'v1'
command = [sys.executable, '-B', str(root / 'scripts/resource_store.py'), 'publish', '--game', 'racing', '--manifest', str(manifest), '--pack', str(pack)]
if args.execute:
    command.append('--execute')
subprocess.run(command, cwd=root, check=True)
