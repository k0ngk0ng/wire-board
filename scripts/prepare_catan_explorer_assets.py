#!/usr/bin/env python3
"""Extract the original four-color 2025 E&P ships, settlers and harbors.

Uses native PDF images and their soft alpha masks; no drawing or recoloring.
"""
import argparse
import hashlib
import json
from pathlib import Path
import pymupdf
from PIL import Image

SOURCE = 'catan-pirates-2025'
PIECES = {
    'settler-blue': 11909, 'settler-red': 11913,
    'settler-white': 11915, 'settler-orange': 11911,
    'ship-blue': 11917, 'ship-red': 11921,
    'ship-white': 11923, 'ship-orange': 11919,
    'harbor-blue': 11931, 'harbor-red': 11937,
    'harbor-white': 11939, 'harbor-orange': 11933,
}

def prepare(output, rules):
    root = Path(__file__).resolve().parent.parent
    source = json.loads((root / 'docs/board-expansion-rule-sources.json').read_text())[SOURCE]
    data = (rules / f'{SOURCE}.pdf').read_bytes()
    if hashlib.sha256(data).hexdigest() != source['sha256']:
        raise ValueError('Rulebook changed; review component references before extraction')
    target = output / 'catan/explorer'
    target.mkdir(parents=True, exist_ok=True)
    provenance = {}
    with pymupdf.open(stream=data, filetype='pdf') as doc:
        page = doc[3]
        for label in ('12 ships', '8 settlers', '16 harbor settlements'):
            if label not in page.get_text():
                raise ValueError(f'Wrong component page: {label}')
        rows = {row[0]: row for row in page.get_images(full=True)}
        for name, xref in PIECES.items():
            row = rows[xref]
            if not row[1]:
                raise ValueError(f'Missing original alpha mask: {name}')
            pix = pymupdf.Pixmap(doc, xref)
            if pix.colorspace.n > 3:
                pix = pymupdf.Pixmap(pymupdf.csRGB, pix)
            art = Image.frombytes('RGBA' if pix.alpha else 'RGB', (pix.width, pix.height), pix.samples)
            mask = pymupdf.Pixmap(doc, row[1])
            alpha = Image.frombytes('L', (mask.width, mask.height), mask.samples)
            if abs(alpha.width / alpha.height - art.width / art.height) > .04 or alpha.getextrema() != (0, 255):
                raise ValueError(f'Invalid native component mask: {name}')
            # The PDF stores masks at a higher sampling resolution than the
            # color image. Resample alpha onto the native color grid only.
            art.putalpha(alpha.resize(art.size, Image.Resampling.LANCZOS))
            path = target / f'{name}-v1.webp'
            art.save(path, lossless=True, method=6)
            provenance[name] = {'source': SOURCE, 'page': 4, 'method': 'native-image',
                                'object': xref, 'soft_mask': row[1], 'mask_size': list(alpha.size), 'size': list(art.size),
                                'bytes': path.stat().st_size, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
    (target / 'sources.json').write_text(json.dumps({'source': source, 'artwork': provenance}, ensure_ascii=False, indent=2) + '\n')
    print(f'Prepared {len(provenance)} original explorer components')

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path, required=True)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
