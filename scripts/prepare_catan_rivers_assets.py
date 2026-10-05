#!/usr/bin/env python3
"""Extract original Rivers of CATAN art from the pinned 2025 rulebooks.

Requires PyMuPDF and Pillow. Keeps native image dimensions and colors. River
components remain whole: the board must transform and clip them to its hexes,
not stretch individual square crops over unrelated terrain.
"""
import argparse
import hashlib
import json
from pathlib import Path
import urllib.request

import pymupdf
from PIL import Image


# Matched visually against the component rows, not inferred from PDF object
# order. Several nested illustrations reuse art outside their clipping paths.
ARTWORK = {
    'river-long': ('catan-traders-2025', 11, 14600, (401, 1259)),
    'river-short': ('catan-traders-2025', 11, 14610, (393, 936)),
    'river-extended': ('catan-traders-5-6-2025', 6, 6074, (346, 824)),
    'bridge-blue': ('catan-traders-2025', 11, 45177, (103, 54)),
    'bridge-orange': ('catan-traders-2025', 11, 45179, (103, 54)),
    'bridge-white': ('catan-traders-2025', 11, 45183, (103, 54)),
    'bridge-red': ('catan-traders-2025', 11, 45195, (103, 54)),
    'bridge-purple': ('catan-traders-5-6-2025', 6, 6059, (83, 43)),
    'bridge-green': ('catan-traders-5-6-2025', 6, 6061, (83, 43)),
    'coin-5': ('catan-traders-2025', 11, 45369, (109, 109)),
    'coin-1': ('catan-traders-2025', 11, 45364, (91, 91)),
    'wealthiest': ('catan-traders-2025', 11, 14632, (261, 144)),
    'poor': ('catan-traders-2025', 11, 14640, (256, 139)),
}


def prepare(output: Path, rules_directory: Path | None):
    root = Path(__file__).resolve().parent.parent
    sources = json.loads((root / 'docs/board-expansion-rule-sources.json').read_text())
    target = output / 'catan/rivers'
    target.mkdir(parents=True, exist_ok=True)
    provenance, used_sources = {}, {}
    for key in sorted({row[0] for row in ARTWORK.values()}):
        source = sources[key]
        if rules_directory:
            data = (rules_directory / f'{key}.pdf').read_bytes()
        else:
            with urllib.request.urlopen(source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != source['sha256']:
            raise ValueError('Rulebook changed; review artwork before updating the pin')
        used_sources[key] = source
        with pymupdf.open(stream=data, filetype='pdf') as doc:
            for name, (book, page, xref, size) in ARTWORK.items():
                if book != key:
                    continue
                rows = {row[0]: row for row in doc[page - 1].get_images(full=True)}
                if xref not in rows:
                    raise ValueError(f'Artwork is no longer on its verified page: {name}')
                pix = pymupdf.Pixmap(doc, xref)
                if pix.colorspace.n > 3:
                    pix = pymupdf.Pixmap(pymupdf.csRGB, pix)
                art = Image.frombytes('RGBA' if pix.alpha else 'RGB',
                                      (pix.width, pix.height), pix.samples)
                if art.size != size:
                    raise ValueError(f'Unexpected native image size: {name}')
                mask_xref = rows[xref][1]
                mask_size = None
                if mask_xref:
                    mask_pix = pymupdf.Pixmap(doc, mask_xref)
                    mask_size = (mask_pix.width, mask_pix.height)
                    mask = Image.frombytes('L', mask_size, mask_pix.samples)
                    # Only the soft mask is resampled when its grid differs.
                    art.putalpha(mask.resize(size, Image.Resampling.BICUBIC))
                if name.startswith(('bridge-', 'coin-')):
                    if not mask_xref or art.getchannel('A').getextrema() != (0, 255):
                        raise ValueError(f'Missing piece transparency: {name}')
                path = target / f'{name}-v1.webp'
                art.save(path, lossless=True, method=6)
                provenance[name] = {
                    'source': key, 'page': page, 'object': xref,
                    'soft_mask': mask_xref or None,
                    'soft_mask_size': list(mask_size) if mask_size else None,
                    'size': list(size), 'file': f'catan/rivers/{path.name}',
                    'bytes': path.stat().st_size,
                    'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
                }
    manifest = {
        'sources': used_sources, 'artwork': provenance,
        'scope': 'Original three complete river components, six bridge colors, 1/5 gold coins and wealth status paintings. Printed text, scores and terrain numbers are separate runtime overlays. Native rectangular river art requires board hex clipping and source-to-mouth alignment.',
    }
    (target / 'sources.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    print(f'Prepared {len(provenance)} original river images; no temporary downloads retained.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
