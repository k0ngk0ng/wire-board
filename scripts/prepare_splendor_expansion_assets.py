#!/usr/bin/env python3
"""Extract pinned official expansion artwork. Requires Poppler and Pillow.

Temporary downloads and extracted images live under the output directory and
are removed on success or failure. No deployment configuration is embedded.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import urllib.request

from PIL import Image, ImageDraw


def prepare(output: Path, rules_directory: Path | None):
    source_file = Path(__file__).resolve().parent.parent / 'docs/board-expansion-rule-sources.json'
    sources = json.loads(source_file.read_text())
    geometry = json.loads(Path(__file__).with_name('splendor_asset_geometry.json').read_text())
    source = sources['splendor-silk-road']
    output.mkdir(parents=True, exist_ok=True)
    targets = output / 'splendor/expansions'
    targets.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='.splendor-art-', dir=output) as temp:
        scratch = Path(temp)
        pdf = scratch / 'silk-road.pdf'
        if rules_directory:
            data = (rules_directory / 'splendor-silk-road.pdf').read_bytes()
        else:
            with urllib.request.urlopen(source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != source['sha256']:
            raise ValueError('Official PDF changed; review rules and artwork before updating the pin')
        pdf.write_bytes(data)
        subprocess.run(['pdfimages', '-f', '2', '-l', '2', '-png', str(pdf), str(scratch / 'image')], check=True)
        # PDF object order differs from text order. These indices are visually
        # verified against each power: red/white, black, white, blue/black, green.
        for post, index in enumerate((27, 41, 30, 33, 36), start=1):
            art = Image.open(scratch / f'image-{index:03}.png').convert('RGBA')
            alpha = Image.open(scratch / f'image-{index+1:03}.png').convert('L')
            if art.size != (206, 138) or alpha.size != art.size:
                raise ValueError(f'Unexpected embedded image for trading post {post}')
            art.putalpha(alpha)
            art.save(targets / f'post-{post}.webp', quality=94, method=6)
        orient_source = sources['splendor-sun-never-sets']
        if rules_directory:
            data = (rules_directory / 'splendor-sun-never-sets.pdf').read_bytes()
        else:
            with urllib.request.urlopen(orient_source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != orient_source['sha256']:
            raise ValueError('Official Orient PDF changed; review before updating the pin')
        pdf = scratch / 'sun-never-sets.pdf'
        pdf.write_bytes(data)
        subprocess.run(['pdfimages', '-f', '2', '-l', '2', '-png', str(pdf), str(scratch / 'orient')], check=True)
        # Original symbols only, not complete card illustrations. Avoid the
        # double-white and sacrifice-black examples for other card colors.
        for name, index, size in (
            ('orient', 2, (66, 66)), ('orient-gold', 18, (168, 87)),
            ('orient-copy', 21, (100, 100)), ('orient-free-1', 27, (100, 100)),
            ('orient-free-2', 36, (100, 100)),
        ):
            art = Image.open(scratch / f'orient-{index:03}.png').convert('RGBA')
            alpha = Image.open(scratch / f'orient-{index+1:03}.png').convert('L')
            if art.size != size or alpha.size != size:
                raise ValueError(f'Unexpected embedded Orient symbol {name}')
            art.putalpha(alpha)
            art.save(targets / f'{name}.webp', quality=94, method=6)
        # The six-deck setup illustration includes three complete Orient backs.
        # Rectify only the printed face, excluding the illustrated deck thickness.
        layout = Image.open(scratch / 'orient-010.png').convert('RGBA')
        alpha = Image.open(scratch / 'orient-011.png').convert('L')
        if layout.size != (1377, 1033) or alpha.size != layout.size:
            raise ValueError('Unexpected Orient setup illustration')
        layout.putalpha(alpha)
        for tier, quad in geometry['orientBacks']['quads'].items():
            art = layout.transform(tuple(geometry['orientBacks']['size']), Image.Transform.QUAD,
                tuple(v for point in quad for v in point), Image.Resampling.BICUBIC)
            art.save(targets / f'orient-back-{tier}.webp', quality=94, method=6)
        # Original painted wooden pieces; masks only remove photographic scenery.
        # Keep highlights, windows and surface texture, without recoloring tokens.
        piece_source = sources['splendor-strongholds-photo']
        if rules_directory:
            data = (rules_directory / 'sun-photo05.jpg').read_bytes()
        else:
            with urllib.request.urlopen(piece_source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != piece_source['sha256']:
            raise ValueError('Official stronghold photograph changed')
        photo = scratch / 'strongholds.jpg'
        photo.write_bytes(data)
        pieces = Image.open(photo).convert('RGBA')
        if pieces.size != (1920, 1280):
            raise ValueError('Unexpected stronghold photo dimensions')
        for color, points in geometry['strongholdOutlines'].items():
            mask = Image.new('L', pieces.size)
            ImageDraw.Draw(mask).polygon([tuple(point) for point in points], fill=255)
            art = pieces.copy()
            art.putalpha(mask)
            art = art.crop(mask.getbbox())
            art.thumbnail((144, 144), Image.Resampling.LANCZOS)
            canvas = Image.new('RGBA', (160, 160))
            canvas.alpha_composite(art, ((160 - art.width) // 2, 152 - art.height))
            canvas.save(targets / f'stronghold-{color}.webp', quality=94, method=6)
        # Use only the city illustrations: printed costs in this promotional
        # photograph are not a source of final rules or reverse-side pairing.
        city_source = sources['splendor-cities-photo']
        if rules_directory:
            data = (rules_directory / 'silk-02.webp').read_bytes()
        else:
            with urllib.request.urlopen(city_source['url'], timeout=45) as response:
                data = response.read()
        if hashlib.sha256(data).hexdigest() != city_source['sha256']:
            raise ValueError('Official city photograph changed')
        photo = scratch / 'cities.webp'
        photo.write_bytes(data)
        cities = Image.open(photo).convert('RGB')
        if cities.size != (1500, 1000):
            raise ValueError('Unexpected city photo dimensions')
        city_geometry = geometry['cityIllustrations']
        for tile, quad in city_geometry['quads'].items():
            art = cities.transform(tuple(city_geometry['rectifiedSize']), Image.Transform.QUAD,
                tuple(v for point in quad for v in point), Image.Resampling.BICUBIC)
            art = art.crop(tuple(city_geometry['illustrationCrop']))
            art.save(targets / f'city-{tile}.webp', quality=94, method=6)
    (targets / 'sources.json').write_text(json.dumps({
        'rules': 'split-box-2025',
        'tradingPosts': source,
        'orientSymbols': orient_source,
        'orientBacks': orient_source,
        'strongholdPieces': piece_source,
        'cityIllustrations': city_source,
        'note': 'Page 2 embedded trading-post tiles and original Orient effect symbols; not full Orient card illustrations. Rectified three Orient backs from the page 2 setup illustration; four un-recolored wooden pieces from the pinned publisher photograph. Seven cropped city illustrations exclude printed costs and are not evidence of final rules or paired sides. Geometry is recorded in scripts/splendor_asset_geometry.json. Promotional prototype images differ.',
    }, ensure_ascii=False, indent=2) + '\n')
    print('Prepared five trading-post tiles, five Orient symbols, three Orient backs, four stronghold pieces and seven city illustrations; temporary files removed.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--rules-directory', type=Path)
    args = parser.parse_args()
    prepare(args.output, args.rules_directory)
