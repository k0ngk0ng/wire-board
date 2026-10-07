#!/usr/bin/env python3
"""Extract original 2025 E&P player pieces and mission artwork.

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
    'crew-blue': 11925, 'crew-red': 11954,
    'crew-white': 11976, 'crew-orange': 11935,
    'pirate-blue': 11996, 'pirate-red': 12002,
    'pirate-white': 12004, 'pirate-orange': 12000,
    'settler-blue': 11909, 'settler-red': 11913,
    'settler-white': 11915, 'settler-orange': 11911,
    'ship-blue': 11917, 'ship-red': 11921,
    'ship-white': 11923, 'ship-orange': 11919,
    'harbor-blue': 11931, 'harbor-red': 11937,
    'harbor-white': 11939, 'harbor-orange': 11933,
}

MISSION_ART = {'fish': (3, 11814), 'council': (3, 11816), 'fish-shoal': (2, 5201),
    'spice': (3, 11822), 'farm-swift': (2, 5191), 'farm-gold': (2, 5186),
    'farm-pirate-5': (2, 5179), 'farm-pirate-4': (2, 5226)}

EXTENSION_PIECES = {
    'crew-purple': 3650, 'crew-green': 3652,
    'settler-purple': 3654, 'settler-green': 3656,
    'harbor-purple': 3658, 'harbor-green': 3660,
    'ship-purple': 3664, 'ship-green': 3666,
    'pirate-purple': 3672, 'pirate-green': 3674,
}

def prepare(output, rules, six_player=False):
    root = Path(__file__).resolve().parent.parent
    source_key = 'catan-pirates-5-6-2025' if six_player else SOURCE
    source = json.loads((root / 'docs/board-expansion-rule-sources.json').read_text())[source_key]
    data = (rules / f'{source_key}.pdf').read_bytes()
    if hashlib.sha256(data).hexdigest() != source['sha256']:
        raise ValueError('Rulebook changed; review component references before extraction')
    target = output / 'catan/explorer'
    target.mkdir(parents=True, exist_ok=True)
    provenance = {}
    with pymupdf.open(stream=data, filetype='pdf') as doc:
        if six_player:
            artwork = {name: (1, xref) for name, xref in EXTENSION_PIECES.items()}
        else:
            page = doc[3]
            for label in ('12 ships', '8 settlers', '16 harbor settlements'):
                if label not in page.get_text():
                    raise ValueError(f'Wrong component page: {label}')
            artwork = {name: (3, xref) for name, xref in PIECES.items()} | MISSION_ART
        for name, (page_index, xref) in artwork.items():
            rows = {row[0]: row for row in doc[page_index].get_images(full=True)}
            row = rows[xref]
            if not row[1]:
                raise ValueError(f'Missing original alpha mask: {name}')
            pix = pymupdf.Pixmap(doc, xref)
            if pix.colorspace.n > 3:
                pix = pymupdf.Pixmap(pymupdf.csRGB, pix)
            art = Image.frombytes('RGBA' if pix.alpha else 'RGB', (pix.width, pix.height), pix.samples)
            mask = pymupdf.Pixmap(doc, row[1])
            alpha = Image.frombytes('L', (mask.width, mask.height), mask.samples)
            # The council is a full rectangular sea painting, clipped to a hex
            # by the board; its native mask is intentionally fully opaque.
            expected_alpha = (255, 255) if name == 'council' else (0, 255)
            if abs(alpha.width / alpha.height - art.width / art.height) > .04 or alpha.getextrema() != expected_alpha:
                raise ValueError(f'Invalid native component mask: {name}')
            # The PDF stores masks at a higher sampling resolution than the
            # color image. Resample alpha onto the native color grid only.
            art.putalpha(alpha.resize(art.size, Image.Resampling.LANCZOS))
            path = target / f'{name}-v1.webp'
            art.save(path, lossless=True, method=6)
            provenance[name] = {'source': source_key, 'page': page_index + 1, 'method': 'native-image',
                                'object': xref, 'soft_mask': row[1], 'mask_size': list(alpha.size), 'size': list(art.size),
                                'bytes': path.stat().st_size, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
    (target / 'sources.json').write_text(json.dumps({'source': source, 'artwork': provenance}, ensure_ascii=False, indent=2) + '\n')
    print(f'Prepared {len(provenance)} original explorer components')

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path, required=True)
    parser.add_argument('--six-player', action='store_true', help='Extract the original green and purple extension pieces')
    args = parser.parse_args()
    prepare(args.output, args.rules_directory, args.six_player)
