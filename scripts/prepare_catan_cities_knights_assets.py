#!/usr/bin/env python3
"""Extract Cities & Knights original components from pinned 2025 rulebooks.

Requires PyMuPDF and Pillow. Work stays in memory except final WebP/provenance.
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
    groups = {
        'catan-knights-2025': {
            'commodity-paper': (3184, (155, 221)),
            'commodity-cloth': (3185, (155, 221)),
            'commodity-coin': (3186, (155, 221)),
            'progress-back-science': (3249, (155, 220)),
            'progress-back-trade': (3238, (155, 220)),
            'progress-back-politics': (3259, (155, 220)),
            'metropolis-science': (11080, (90, 93)),
            'metropolis-trade': (11078, (90, 93)),
            'metropolis-politics': (11082, (90, 93)),
            'barbarian-ship': (3226, (96, 83)),
            'merchant': (3229, (76, 88)),
            'defender': (10937, (100, 100)),
        },
        'catan-knights-5-6-2025': {},
    }
    base = groups['catan-knights-2025']
    for color, objects in {
        'blue': (3323, 3188, 3196, 3205),
        'red': (3327, 3192, 3201, 3209),
        'white': (3329, 3194, 3203, 3211),
        'orange': (3325, 3190, 3199, 3207),
    }.items():
        base[f'wall-{color}'] = (objects[0], (104, 45))
        for level, (xref, size) in enumerate(zip(objects[1:], [(38, 75), (53, 74), (71, 77)]), 1):
            base[f'knight-{color}-{level}'] = (xref, size)
    extension = groups['catan-knights-5-6-2025']
    for color, objects in {
        'purple': [(2948, (94, 41)), (2936, (42, 83)), (2944, (62, 86)), (2940, (83, 89))],
        'green': [(2950, (94, 41)), (2938, (42, 84)), (2946, (62, 87)), (2942, (84, 91))],
    }.items():
        extension[f'wall-{color}'] = objects[0]
        for level, item in enumerate(objects[1:], 1):
            extension[f'knight-{color}-{level}'] = item
    target = output / 'catan/cities-knights'
    target.mkdir(parents=True, exist_ok=True)
    provenance = {}
    for key, objects in groups.items():
        source = sources[key]
        if rules_directory:
            data = (rules_directory / f'{key}.pdf').read_bytes()
        else:
            with urllib.request.urlopen(source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != source['sha256']:
            raise ValueError('Rulebook changed: verify before updating the pin')
        with pymupdf.open(stream=data, filetype='pdf') as doc:
            masks = {row[0]: row[1] for page in doc for row in page.get_images(full=True)}
            for name, (xref, size) in objects.items():
                pix = pymupdf.Pixmap(doc, xref)
                if pix.colorspace.n > 3:
                    pix = pymupdf.Pixmap(pymupdf.csRGB, pix)
                art = Image.frombytes('RGBA' if pix.alpha else 'RGB', (pix.width, pix.height), pix.samples).convert('RGBA')
                if art.size != size:
                    raise ValueError(f'Unexpected component dimensions: {name}')
                if masks.get(xref):
                    mask = pymupdf.Pixmap(doc, masks[xref])
                    alpha = Image.frombytes('L', (mask.width, mask.height), mask.samples)
                    art.putalpha(alpha.resize(art.size, Image.Resampling.LANCZOS))
                if not name.startswith(('commodity-', 'progress-back-')):
                    if not masks.get(xref):
                        raise ValueError(f'Missing piece transparency: {name}')
                    art = art.crop(art.getbbox())
                path = target / f'{name}-v1.webp'
                art.save(path, lossless=True, method=6)
                provenance[name] = {'source': key, 'object': xref, 'size': list(art.size), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
    (target / 'sources.json').write_text(json.dumps({'sources': {k: sources[k] for k in groups}, 'artwork': provenance}, ensure_ascii=False, indent=2) + '\n')
    print(f'Prepared {len(provenance)} original Cities & Knights components.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
