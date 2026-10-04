#!/usr/bin/env python3
"""Extract original Seafarers pieces and terrain from pinned official rulebooks.

Requires PyMuPDF and Pillow. PDFs are verified before reading object references.
All extraction is in memory; only final artwork and provenance are written.
"""
import argparse
import hashlib
import json
from pathlib import Path
import urllib.request

import pymupdf
from PIL import Image


def prepare(output: Path, rules_directory: Path | None):
    root = Path(__file__).resolve().parent.parent
    sources = json.loads((root / 'docs/board-expansion-rule-sources.json').read_text())
    target = output / 'catan/seafarers'
    target.mkdir(parents=True, exist_ok=True)
    # Object IDs, dimensions and crop bounds verified against the component pages.
    groups = {
        'catan-seafarers-2025': {
            'ship-white': (21433, (67, 55), None),
            'ship-orange': (21435, (67, 55), None),
            'ship-blue': (21437, (67, 55), None),
            'ship-red': (21439, (67, 55), None),
            'pirate': (21545, (106, 91), None),
            'terrain-sea': (21402, (444, 507), None),
            'terrain-gold': (21453, (444, 482), (54, 48, 389, 434)),
        },
        'catan-seafarers-5-6-2025': {
            'ship-purple': (19406, (97, 80), None),
            'ship-green': (19428, (97, 80), None),
        },
    }
    provenance = {}
    for key, objects in groups.items():
        source = sources[key]
        if rules_directory:
            data = (rules_directory / f'{key}.pdf').read_bytes()
        else:
            with urllib.request.urlopen(source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != source['sha256']:
            raise ValueError('Rulebook changed: verify the source before updating the pin')
        with pymupdf.open(stream=data, filetype='pdf') as doc:
            masks = {row[0]: row[1] for row in doc[0].get_images(full=True)}
            for name, (xref, dimensions, crop) in objects.items():
                pix = pymupdf.Pixmap(doc, xref)
                if pix.colorspace.n > 3:
                    pix = pymupdf.Pixmap(pymupdf.csRGB, pix)
                art = Image.frombytes('RGBA' if pix.alpha else 'RGB',
                                      (pix.width, pix.height), pix.samples).convert('RGBA')
                if dimensions and art.size != dimensions:
                    raise ValueError(f'Unexpected source dimensions: {name}')
                if masks.get(xref):
                    pixmask = pymupdf.Pixmap(doc, masks[xref])
                    mask = Image.frombytes('L', (pixmask.width, pixmask.height), pixmask.samples)
                    art.putalpha(mask.resize(art.size, Image.Resampling.LANCZOS))
                if crop:
                    art = art.crop(crop)
                if not name.startswith('terrain-'):
                    if not masks.get(xref):
                        raise ValueError(f'Missing piece transparency: {name}')
                    art = art.crop(art.getbbox())
                art.save(target / f'{name}-v1.webp', lossless=True, method=6)
                provenance[name] = {'source': key, 'object': xref, 'crop': crop}
    (target / 'sources.json').write_text(json.dumps({
        'sources': {key: sources[key] for key in groups},
        'artwork': provenance,
    }, ensure_ascii=False, indent=2) + '\n')
    print('Prepared nine original Seafarers images; no scratch files retained.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
