#!/usr/bin/env python3
"""Validate and copy the pinned original Orient atlas, without re-encoding.

Requires Pillow. Downloads stay in memory; only the verified output and
provenance are written inside the caller-specified output directory.
"""
import argparse
import hashlib
import io
import json
from pathlib import Path
import urllib.request

from PIL import Image

URL = 'https://x.boardgamearena.net/data/themereleases/current/games/splendor/250604-1803/img/sns_cards.jpg'
SHA256 = '4802bd7ebb939cb17f35e45b64b4ca057c92848ba984714483922f4755c309a9'


def prepare(output: Path, source: Path | None):
    if source:
        data = source.read_bytes()
    else:
        request = urllib.request.Request(URL, headers={
            'User-Agent': 'Mozilla/5.0', 'Referer': 'https://en.boardgamearena.com/',
        })
        with urllib.request.urlopen(request, timeout=45) as response:
            data = response.read()
    if hashlib.sha256(data).hexdigest() != SHA256:
        raise ValueError('Original atlas changed; inspect before updating the pin')
    with Image.open(io.BytesIO(data)) as art:
        if art.format != 'WEBP' or art.size != (1235, 1715):
            raise ValueError('Unexpected atlas format or dimensions')
        art.load()
    relative = 'splendor/expansions/orient-cards-v1.webp'
    target = output / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(data)
    (output / 'orient-atlas-provenance.json').write_text(json.dumps({
        'source': URL, 'sha256': SHA256, 'bytes': len(data), 'output': relative,
        'format': 'WEBP', 'size': [1235, 1715], 'grid': [5, 5], 'cell': [247, 343],
        'transformation': 'none; original bytes, source suffix corrected',
        'rows': ['copy, gold, copy_cascade, empty, empty',
                 'double: black, red, white, blue, green',
                 'sacrifice: black, red, white, blue, green',
                 'cascade: black, red, white, blue, green',
                 'back tier 1, back tier 2, back tier 3, empty, empty'],
        'overlays': 'Costs, points and effects rendered from game state; absent from atlas',
    }, indent=2) + '\n')
    print(f'Prepared {relative}: {len(data)} bytes, original encoding')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--source', type=Path, help='Previously downloaded original atlas')
    args = parser.parse_args()
    prepare(args.output, args.source)
