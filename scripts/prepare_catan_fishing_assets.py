#!/usr/bin/env python3
"""Extract original Fishing on CATAN components from pinned 2025 rulebooks.

Requires PyMuPDF and Pillow. Retains native image size and original artwork;
printed number overlays are game data, not baked into the extracted terrain.
"""
import argparse
import hashlib
import json
from pathlib import Path
import urllib.request

import pymupdf
from PIL import Image


# Visually matched to the component rows on printed pages 9 and 5. Other
# larger lake/ground XObjects on these pages belong to nested/clipped examples,
# so do not choose one solely by its image resolution or object ordering.
ARTWORK = {
    'lake': ('catan-traders-2025', 9, 11817, (223, 261)),
    'lake-extended': ('catan-traders-5-6-2025', 5, 5721, (175, 204)),
    'ground': ('catan-traders-2025', 9, 11823, (182, 233)),
    'token-1': ('catan-traders-2025', 9, 45313, (93, 93)),
    'token-2': ('catan-traders-2025', 9, 45330, (93, 93)),
    'token-3': ('catan-traders-2025', 9, 45315, (93, 93)),
    'boot': ('catan-traders-2025', 9, 45320, (93, 93)),
    'back': ('catan-traders-2025', 9, 45329, (93, 93)),
    'number': ('catan-traders-2025', 9, 45303, (53, 53)),
}


def prepare(output: Path, rules_directory: Path | None):
    root = Path(__file__).resolve().parent.parent
    sources = json.loads((root / 'docs/board-expansion-rule-sources.json').read_text())
    target = output / 'catan/fishing'
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
                art = Image.frombytes('RGBA' if pix.alpha else 'RGB', (pix.width, pix.height), pix.samples)
                if art.size != size:
                    raise ValueError(f'Unexpected native image size: {name}')
                mask_xref = rows[xref][1]
                mask_size = None
                if mask_xref:
                    mask_pix = pymupdf.Pixmap(doc, mask_xref)
                    mask_size = (mask_pix.width, mask_pix.height)
                    mask = Image.frombytes('L', mask_size, mask_pix.samples)
                    # PDF soft masks can have a different native pixel grid.
                    # Match that mask to the color image, never upscale the art.
                    art.putalpha(mask.resize(size, Image.Resampling.BICUBIC))
                path = target / f'{name}-v1.webp'
                art.save(path, lossless=True, method=6)
                provenance[name] = {
                    'source': key, 'page': page, 'object': xref,
                    'soft_mask': mask_xref or None,
                    'soft_mask_size': list(mask_size) if mask_size else None,
                    'size': list(size), 'file': f'catan/fishing/{path.name}',
                    'bytes': path.stat().st_size,
                    'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
                }
    manifest = {
        'sources': used_sources, 'artwork': provenance,
        'scope': 'Original lake/ground terrain, 1/2/3-fish tokens, boot, token back and blue number disk. Lake and ground production numbers are separate runtime overlays.',
    }
    (target / 'sources.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    print(f'Prepared {len(provenance)} original fishing images; no temporary downloads retained.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
