#!/usr/bin/env python3
"""Extract original event illustrations and deck back from pinned 2025 T&B.

Requires PyMuPDF and Pillow. Images are artwork, not verified numbered faces
or a 36-card catalogue. Downloads and decoding stay in memory.
"""
import argparse
import hashlib
import json
from pathlib import Path
import urllib.request

import pymupdf
from PIL import Image


# Page numbers are printed/PDF pages, one-based. Visually matched to the
# adjacent event titles, including New Year (which has no production number).
ARTWORK = {
    'back': (4, 44946, (173, 247)),
    'beautiful_day': (5, 6797, (203, 177)),
    'calm_seas': (5, 6798, (208, 177)),
    'conflict': (5, 6796, (208, 172)),
    'earthquake': (5, 6801, (208, 172)),
    'epidemic': (5, 6799, (203, 177)),
    'good_neighbors': (5, 6800, (203, 177)),
    'helpful_neighbor': (6, 6863, (208, 172)),
    'new_year': (6, 6860, (208, 177)),
    'plentiful_year': (6, 6861, (203, 177)),
    'robber_attacks': (6, 6862, (208, 177)),
    'robber_flees': (6, 6859, (203, 177)),
    'tournament': (6, 6858, (208, 172)),
    'trade_advantage': (6, 6857, (208, 177)),
}


def prepare(output: Path, rules_directory: Path | None):
    root = Path(__file__).resolve().parent.parent
    key = 'catan-traders-2025'
    source = json.loads((root / 'docs/board-expansion-rule-sources.json').read_text())[key]
    if rules_directory:
        data = (rules_directory / f'{key}.pdf').read_bytes()
    else:
        with urllib.request.urlopen(source['url'], timeout=45) as response:
            data = response.read()
    if hashlib.sha256(data).hexdigest() != source['sha256']:
        raise ValueError('Rulebook changed; review source and artwork before updating the pin')
    target = output / 'catan/events'
    target.mkdir(parents=True, exist_ok=True)
    provenance = {}
    with pymupdf.open(stream=data, filetype='pdf') as doc:
        for name, (page, xref, size) in ARTWORK.items():
            if xref not in {row[0] for row in doc[page - 1].get_images(full=True)}:
                raise ValueError(f'Artwork is no longer on its verified page: {name}')
            pix = pymupdf.Pixmap(doc, xref)
            if pix.colorspace.n > 3:
                pix = pymupdf.Pixmap(pymupdf.csRGB, pix)
            art = Image.frombytes('RGBA' if pix.alpha else 'RGB', (pix.width, pix.height), pix.samples)
            if art.size != size:
                raise ValueError(f'Unexpected embedded dimensions: {name}')
            path = target / f'{name}-v1.webp'
            # Preserve native aspect ratio and detail; do not stretch, upscale
            # or paint a fabricated production number into the original art.
            art.save(path, lossless=True, method=6)
            provenance[name] = {
                'source': key, 'page': page, 'object': xref,
                'size': list(size), 'file': f'catan/events/{path.name}',
                'bytes': path.stat().st_size,
                'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            }
    manifest = {
        'sources': {key: source}, 'artwork': provenance,
        'scope': '13 event illustrations and one card back; NOT a verified 36-card production/event catalogue or complete numbered card faces.',
    }
    (target / 'sources.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    print(f'Prepared {len(provenance)} original event images; no temporary downloads retained.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
