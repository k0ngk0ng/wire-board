#!/usr/bin/env python3
"""Extract the original 2025 CATAN for Two trade token from a pinned rulebook.
The printed token is circular; apply a circular clip in the interface.
"""
import argparse
import hashlib
import json
from pathlib import Path
import urllib.request

import pymupdf
from PIL import Image


ARTWORK = {
    'trade-token': (7, 44950, (105, 104)),
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
        raise ValueError('Rulebook changed; review art before updating the pin')
    target = output / 'catan/two'
    target.mkdir(parents=True, exist_ok=True)
    provenance = {}
    with pymupdf.open(stream=data, filetype='pdf') as doc:
        for name, (page, xref, size) in ARTWORK.items():
            rows = {r[0]: r for r in doc[page - 1].get_images(full=True)}
            if xref not in rows:
                raise ValueError(f'Artwork missing from verified page: {name}')
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
                art.putalpha(mask.resize(size, Image.Resampling.BICUBIC))
            path = target / f'{name}-v1.webp'
            art.save(path, lossless=True, method=6)
            provenance[name] = {
                'source': key, 'page': page, 'object': xref,
                'soft_mask': mask_xref or None,
                'soft_mask_size': list(mask_size) if mask_size else None,
                'size': list(size), 'file': f'catan/two/{path.name}',
                'bytes': path.stat().st_size,
                'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            }
    manifest = {
        'sources': {key: source}, 'artwork': provenance,
        'scope': 'Original 2025 trade token, visually verified on printed page 7. Native size and colors retained; renderer applies the printed circular clipping.',
    }
    (target / 'sources.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    print(f'Prepared {len(provenance)} original CATAN for Two images; no temporary downloads retained.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
