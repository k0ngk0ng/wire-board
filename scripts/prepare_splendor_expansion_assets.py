#!/usr/bin/env python3
"""Extract pinned official expansion artwork. Requires Poppler and Pillow.

Temporary downloads and extracted images live under the output directory and
are removed on success or failure. No deployment configuration is embedded.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import urllib.request

from PIL import Image


def prepare(output: Path, rules_directory: Path | None):
    source_file = Path(__file__).resolve().parent.parent / 'docs/board-expansion-rule-sources.json'
    sources = json.loads(source_file.read_text())
    source = sources['splendor-silk-road']
    output.mkdir(parents=True, exist_ok=True)
    targets = output / 'splendor/expansions'
    targets.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='.splendor-art-', dir=output) as temp:
        scratch = Path(temp)
        pdf = scratch / 'silk-road.pdf'
        if rules_directory:
            data = (rules_directory / 'splendor-silk-road.pdf').read_bytes()
        else:
            with urllib.request.urlopen(source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != source['sha256']:
            raise ValueError('Official PDF changed; review rules and artwork before updating the pin')
        pdf.write_bytes(data)
        subprocess.run(['pdfimages', '-f', '2', '-l', '2', '-png', str(pdf), str(scratch / 'image')], check=True)
        # PDF object order differs from text order. These indices are visually
        # verified against each power: red/white, black, white, blue/black, green.
        for post, index in enumerate((27, 41, 30, 33, 36), start=1):
            art = Image.open(scratch / f'image-{index:03}.png').convert('RGBA')
            alpha = Image.open(scratch / f'image-{index+1:03}.png').convert('L')
            if art.size != (206, 138) or alpha.size != art.size:
                raise ValueError(f'Unexpected embedded image for trading post {post}')
            art.putalpha(alpha)
            art.save(targets / f'post-{post}.webp', quality=94, method=6)
        orient_source = sources['splendor-sun-never-sets']
        if rules_directory:
            data = (rules_directory / 'splendor-sun-never-sets.pdf').read_bytes()
        else:
            with urllib.request.urlopen(orient_source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != orient_source['sha256']:
            raise ValueError('Official Orient PDF changed; review before updating the pin')
        pdf = scratch / 'sun-never-sets.pdf'
        pdf.write_bytes(data)
        subprocess.run(['pdfimages', '-f', '2', '-l', '2', '-png', str(pdf), str(scratch / 'orient')], check=True)
        # Original symbols only, not complete card illustrations. Avoid the
        # double-white and sacrifice-black examples for other card colors.
        for name, index, size in (
            ('orient', 2, (66, 66)), ('orient-gold', 18, (168, 87)),
            ('orient-copy', 21, (100, 100)), ('orient-free-1', 27, (100, 100)),
            ('orient-free-2', 36, (100, 100)),
        ):
            art = Image.open(scratch / f'orient-{index:03}.png').convert('RGBA')
            alpha = Image.open(scratch / f'orient-{index+1:03}.png').convert('L')
            if art.size != size or alpha.size != size:
                raise ValueError(f'Unexpected embedded Orient symbol {name}')
            art.putalpha(alpha)
            art.save(targets / f'{name}.webp', quality=94, method=6)
    (targets / 'sources.json').write_text(json.dumps({
        'rules': 'split-box-2025',
        'tradingPosts': source,
        'orientSymbols': orient_source,
        'note': 'Page 2 embedded trading-post tiles and original Orient effect symbols; not full Orient card illustrations. Matched visually to rule descriptions; promotional prototype images differ.',
    }, ensure_ascii=False, indent=2) + '\n')
    print('Prepared five official trading-post tiles and five Orient symbols; temporary files removed.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
