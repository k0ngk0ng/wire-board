#!/usr/bin/env python3
"""Extract original Barbarian Attack pieces and complete card faces.

Pinned 2025 rulebooks only. The castle and cards are page composites: their
printed dice/text are PDF vectors and must not be lost by extracting only the
underlying image. No artwork is recolored or redrawn.
"""
import argparse
import hashlib
import json
from pathlib import Path
import urllib.request

import pymupdf
from PIL import Image, ImageDraw, ImageChops

BASE = 'catan-traders-2025'
EXTENDED = 'catan-traders-5-6-2025'
PIECES = {
    'knight-blue': (BASE, 15, 44980, (73, 92)),
    'knight-orange': (BASE, 15, 44987, (73, 92)),
    'knight-red': (BASE, 15, 44979, (73, 92)),
    'knight-white': (BASE, 15, 44992, (73, 92)),
    'knight-purple': (EXTENDED, 8, 6605, (59, 74)),
    'knight-green': (EXTENDED, 8, 6607, (59, 74)),
    'barbarian': (BASE, 15, 44954, (62, 98)),
    'card-back': (BASE, 15, 23140, (173, 246)),
}
# Full card frame bounds, independently matched to their named illustrations.
# Multiple frames reuse nested image objects; choosing an xref alone is unsafe.
CARDS = {
    'capture': (100.6680145, 264.8504028, 165.9013977, 358.0322876),
    'knighthood': (165.6093292, 264.8504028, 230.8427124, 358.0322876),
    'swift_knight': (230.5506287, 264.8504028, 295.7840271, 358.0322876),
    'treason': (295.4919434, 264.8504028, 360.7253418, 358.0322876),
}
CASTLE = [
    (394.9598, 165.3157), (434.3398, 187.9717),
    (434.3398, 233.2577), (394.9598, 255.8587),
    (355.7128, 233.2577), (355.7128, 187.9717),
]


def composite(page, bounds, size, polygon=None):
    # Render vector text/dice at 4x, then reduce. Pixel origin may be rounded by
    # MuPDF; use Pixmap.x/y when registering the alpha polygon, not clip.x0/y0.
    pix = page.get_pixmap(matrix=pymupdf.Matrix(4, 4), clip=pymupdf.Rect(bounds), alpha=False)
    art = Image.frombytes('RGB', (pix.width, pix.height), pix.samples).convert('RGBA')
    if polygon:
        mask = Image.new('L', art.size, 0)
        ImageDraw.Draw(mask).polygon([(x*4-pix.x, y*4-pix.y) for x, y in polygon], fill=255)
        art.putalpha(mask)
    return art.resize(size, Image.Resampling.LANCZOS)


def prepare(output: Path, rules_directory: Path | None):
    root = Path(__file__).resolve().parent.parent
    catalog = json.loads((root / 'docs/board-expansion-rule-sources.json').read_text())
    target = output / 'catan/attack'
    target.mkdir(parents=True, exist_ok=True)
    docs, sources, provenance = {}, {}, {}
    try:
        for key in (BASE, EXTENDED):
            source = catalog[key]
            if rules_directory:
                data = (rules_directory / f'{key}.pdf').read_bytes()
            else:
                with urllib.request.urlopen(source['url'], timeout=45) as response:
                    data = response.read()
            if hashlib.sha256(data).hexdigest() != source['sha256']:
                raise ValueError('Rulebook changed; review art before changing the source pin')
            sources[key] = source
            docs[key] = pymupdf.open(stream=data, filetype='pdf')

        def save(name, art, source):
            path = target / f'{name}-v1.webp'
            art.save(path, lossless=True, method=6)
            provenance[name] = {**source, 'size': list(art.size),
                'file': f'catan/attack/{path.name}', 'bytes': path.stat().st_size,
                'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}

        for name, (key, page, xref, size) in PIECES.items():
            doc = docs[key]
            rows = {r[0]: r for r in doc[page-1].get_images(full=True)}
            if xref not in rows or tuple(rows[xref][2:4]) != size:
                raise ValueError(f'Original component no longer matches: {name}')
            pix = pymupdf.Pixmap(doc, xref)
            if pix.colorspace.n > 3:
                pix = pymupdf.Pixmap(pymupdf.csRGB, pix)
            art = Image.frombytes('RGBA' if pix.alpha else 'RGB', size, pix.samples)
            mask_xref = rows[xref][1]
            if mask_xref:
                mask = pymupdf.Pixmap(doc, mask_xref)
                art.putalpha(Image.frombytes('L', (mask.width, mask.height), mask.samples)
                             .resize(size, Image.Resampling.BICUBIC))
            if name != 'card-back' and (not mask_xref or art.getchannel('A').getextrema() != (0, 255)):
                raise ValueError(f'Missing piece transparency: {name}')
            save(name, art, {'source': key, 'page': page, 'object': xref,
                             'soft_mask': mask_xref or None, 'method': 'native-image'})

        page = docs[BASE][14]
        bounds = (min(x for x, _ in CASTLE), min(y for _, y in CASTLE),
                  max(x for x, _ in CASTLE), max(y for _, y in CASTLE))
        save('castle', composite(page, bounds, (208, 240), CASTLE),
             {'source': BASE, 'page': 15, 'clip': bounds, 'alpha_polygon': CASTLE,
              'method': 'page-composite', 'orientation': 'upright; purple 1/6 upper-left/lower-right, green 2/5 upper-right/lower-left, brown 3/4 vertical edges'})
        for name, bounds in CARDS.items():
            printed = page.get_text(clip=pymupdf.Rect(bounds)).lower()
            if name.replace('_', ' ') not in printed:
                raise ValueError(f'Wrong card region: {name}')
            art = composite(page, bounds, (196, 280))
            mask_xref = 23161 if name == 'treason' else 45631
            mask = pymupdf.Pixmap(docs[BASE], mask_xref)
            alpha = Image.frombytes('L', (mask.width, mask.height), mask.samples)
            # Frame masks contain transparent windows for the separate art.
            # Fill those enclosed holes, preserving only exterior transparency.
            region = alpha.point(lambda v: 255 if v else 0)
            for x in range(region.width):
                for y in (0, region.height-1):
                    if region.getpixel((x, y)) == 0:
                        ImageDraw.floodfill(region, (x, y), 128)
            for y in range(region.height):
                for x in (0, region.width-1):
                    if region.getpixel((x, y)) == 0:
                        ImageDraw.floodfill(region, (x, y), 128)
            alpha = ImageChops.lighter(alpha, region.point(lambda v: 255 if v == 0 else 0))
            art.putalpha(alpha.resize(art.size, Image.Resampling.BICUBIC))
            if art.getpixel((98, 100))[3] != 255:
                raise ValueError(f'Card illustration was cut out: {name}')
            save(f'card-{name}', art,
                 {'source': BASE, 'page': 15, 'clip': bounds, 'method': 'page-composite',
                  'soft_mask': mask_xref,
                  'language': 'original English; Chinese explanation provided in the UI'})
        (target / 'sources.json').write_text(json.dumps({
            'sources': sources, 'artwork': provenance,
            'scope': 'Six original player-color knights, barbarian, complete castle with printed loss-direction dice, four original English card faces and card back. No recoloring; native piece resolution. Castle kept upright on both map sizes.'
        }, ensure_ascii=False, indent=2) + '\n')
    finally:
        for doc in docs.values():
            doc.close()
    print(f'Prepared {len(provenance)} original Barbarian Attack images; no downloaded temporary files retained.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
