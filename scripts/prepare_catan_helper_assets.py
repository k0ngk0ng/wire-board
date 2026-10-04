#!/usr/bin/env python3
"""Extract twelve original Helpers portraits from the pinned official rulebook.

Requires Poppler and Pillow. All scratch files stay inside output and are cleaned.
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
    root = Path(__file__).resolve().parent.parent
    source = json.loads((root / 'docs/board-expansion-rule-sources.json').read_text())['catan-helpers']
    output.mkdir(parents=True, exist_ok=True)
    target = output / 'catan/helpers'
    target.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='.helper-art-', dir=output) as temporary:
        scratch = Path(temporary)
        if rules_directory:
            data = (rules_directory / 'catan-helpers.pdf').read_bytes()
        else:
            with urllib.request.urlopen(source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != source['sha256']:
            raise ValueError('Rulebook changed: verify the source before updating the pin')
        pdf = scratch / 'helpers.pdf'
        pdf.write_bytes(data)
        listing = subprocess.check_output(['pdfimages', '-f', '6', '-l', '11', '-list', str(pdf)], text=True)
        objects = {}
        for line in listing.splitlines():
            row = line.split()
            if len(row) > 11 and row[2] == 'image' and row[5] == 'cmyk':
                objects[int(row[10])] = int(row[1])
        subprocess.run(['pdfimages', '-f', '6', '-l', '11', '-png', str(pdf), str(scratch / 'image')], check=True)
        # Two named portraits per detailed-rules page, verified in printed order.
        for helper, obj in enumerate((125, 133, 159, 168, 195, 203, 230, 239, 265, 278, 300, 313), 1):
            index = objects[obj]
            art = Image.open(scratch / f'image-{index:03}.png').convert('RGBA')
            mask = Image.open(scratch / f'image-{index+1:03}.png').convert('L')
            if not 110 <= art.width <= 120 or not 90 <= art.height <= 110:
                raise ValueError(f'Unexpected portrait for helper {helper}')
            art = art.resize(mask.size, Image.Resampling.LANCZOS)
            art.putalpha(mask)
            art.save(target / f'helper-{helper}.webp', quality=94, method=6)
    (target / 'sources.json').write_text(json.dumps({'source': source, 'portraits': 'Detailed rules pages 6–11, printed helper IDs 1–12'}, ensure_ascii=False, indent=2)+'\n')
    print('Prepared twelve official helper portraits; scratch files removed.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
