#!/usr/bin/env python3
"""Verify and copy the original city illustrations without re-encoding.

Requires Pillow. Downloads stay in memory. The atlas contains no conditions
and does not establish which physical faces are paired on a city tile.
"""
import argparse
import hashlib
import io
import json
from pathlib import Path
import urllib.request

from PIL import Image

URL = 'https://x.boardgamearena.net/data/themereleases/current/games/splendor/250604-1803/img/cities_bg.png'
SHA256 = 'cb53db4ef984b9dbe16adcd838c16909dac6163064f14f50d710ca16cc992480'


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
        if art.format != 'WEBP' or art.size != (360, 1310):
            raise ValueError('Unexpected atlas format or dimensions')
        art.load()
    relative = 'splendor/expansions/cities-art-v1.webp'
    target = output / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(data)
    (output / 'city-atlas-provenance.json').write_text(json.dumps({
        'source': URL, 'sha256': SHA256, 'bytes': len(data), 'output': relative,
        'format': 'WEBP', 'size': [360, 1310],
        'transformation': 'none; original bytes, source suffix corrected',
        'display_crop': {'x': 5, 'width': 350, 'height': 172},
        'cities': [
            {'tile': 1, 'name': 'Madrid', 'top': 570},
            {'tile': 2, 'name': 'Amboise', 'top': 8},
            {'tile': 3, 'name': 'Timbuktu', 'top': 194},
            {'tile': 4, 'name': 'Delhi', 'top': 1130},
            {'tile': 5, 'name': 'Samarkand', 'top': 756},
            {'tile': 6, 'name': 'Seoul', 'top': 944},
            {'tile': 7, 'name': 'Krakow', 'top': 381},
        ],
        'overlays': 'Conditions are rendered separately from game state; absent from atlas',
        'evidence_limit': 'Illustrations only, not physical face pairing or condition evidence',
    }, indent=2) + '\n')
    print(f'Prepared {relative}: {len(data)} bytes, original encoding')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--source', type=Path, help='Previously downloaded original atlas')
    args = parser.parse_args()
    prepare(args.output, args.source)
