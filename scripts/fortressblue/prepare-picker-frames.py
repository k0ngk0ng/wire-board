#!/usr/bin/env python3
"""Derive preview-only transparent bounds without modifying published atlases."""
import argparse
import hashlib
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / '.local/fortressblue-original/tools/py'))
from PIL import Image

parser = argparse.ArgumentParser()
parser.add_argument('--check', action='store_true')
args = parser.parse_args()
catalog = json.loads((ROOT / 'games/fortressblue/data/animations.json').read_text())
result = {}
for key, tank in catalog['tanks'].items():
    index = tank['actions']['idle']['right'][0]
    frame = tank['frames'][index]
    path = ROOT / ('games/fortressblue/assets/public' + tank['atlases'][frame['atlas']])
    with Image.open(path) as atlas:
        image = atlas.crop((frame['x'], frame['y'], frame['x']+frame['w'], frame['y']+frame['h']))
        left, top, right, bottom = image.getbbox()
    result[key] = {'frame': index, 'x': left, 'y': top, 'w': right-left, 'h': bottom-top,
                   'atlasSHA256': hashlib.sha256(path.read_bytes()).hexdigest()}
output = ROOT / 'games/fortressblue/assets/picker-frames.json'
content = json.dumps(result, ensure_ascii=False, indent=2) + '\n'
if args.check:
    assert output.read_text() == content, 'Preview bounds differ from original atlas pixels'
else:
    output.write_text(content)
print('Verified 14 preview bounds; published media unchanged')
