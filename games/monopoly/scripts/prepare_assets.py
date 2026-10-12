#!/usr/bin/env python3
"""Rebuild versioned Monopoly WebP/MP3 bundle from hash-pinned source files.

Run with --sources .local/monopoly/sources --rules .local/monopoly-research/c1009.pdf
and --output .local/monopoly/media. Requires Pillow, NumPy, PyMuPDF and ffmpeg.
Does not upload or write outside the specified output directory.
"""
import argparse
import hashlib
import io
import json
from pathlib import Path
import subprocess

import numpy as np
import pymupdf
from PIL import Image, ImageChops, ImageDraw, ImageOps

PDF_HASH = '8fc0456e50b6ad872628f904ca04aad5b7a57982a90047664c8620c221daf8be'
TOKEN_HASH = 'd25c489275e45564ba86c8fc31a8f42a601eb3f878d1722fe40202a14bd29b0c'
REMOTE = 'https://raw.githubusercontent.com/itaylayzer/Monopoly/be46b3c7c7ae1d1262d440a99029460fd535a85b/public/'
# Page two PDF-point rectangles, reviewed against the printed components.
SYMBOLS = {
    'go': (1548, 113, 1621, 161),
    'chance': (1548, 199, 1578, 252),
    'chest': (1596, 214.5, 1632, 247.5),
    'parking': (1573, 400, 1608, 435),
    'visiting': (1821, 69, 1850, 100),
    'jail': (1822, 190, 1857, 225),
    'tax': (1546, 323, 1579, 350),
    'station': (1150, 433, 1187, 464),
    'utility': (1315, 426, 1349, 464),
    'water': (1358, 429, 1396, 464),
}


def checked(path, expected):
    body = path.read_bytes()
    if hashlib.sha256(body).hexdigest() != expected:
        raise ValueError(f'Source changed: {path.name}')
    return body


def prepare(args):
    root = Path(__file__).resolve().parents[1]
    pins = json.loads((root / 'assets/audio-sources.json').read_text())
    doc = pymupdf.open(stream=checked(args.rules, PDF_HASH), filetype='pdf')
    tokens = Image.open(io.BytesIO(checked(args.sources / 'all.png', TOKEN_HASH))).convert('RGBA')
    for name, digest in pins.items():
        checked(args.sources / (name + '.mp3'), digest)
    output = args.output
    (output / 'images').mkdir(parents=True, exist_ok=True)
    (output / 'audio').mkdir(parents=True, exist_ok=True)
    media = []

    def save(name, im, source, square=False):
        im = im.convert('RGBA')
        if square:
            box = im.getbbox()
            if box: im = im.crop(box)
            im = ImageOps.contain(im, (224, 224), Image.Resampling.LANCZOS)
            canvas = Image.new('RGBA', (256, 256))
            canvas.alpha_composite(im, ((256-im.width)//2, (256-im.height)//2))
            im = canvas
        else:
            im.thumbnail((1200, 900), Image.Resampling.LANCZOS)
        path = output / 'images' / (name + '.webp')
        im.save(path, lossless=True, method=6)
        # Decode every prepared output rather than trusting a header.
        with Image.open(path) as check: check.load(); assert check.size == im.size
        media.append({'file': path.relative_to(output).as_posix(), 'mime': 'image/webp', 'size': list(im.size), 'source': source})

    for name, xref in [('cover', 42), ('house', 43), ('hotel', 44), ('logo', 45)]:
        raw = doc.extract_image(xref)
        im = Image.open(io.BytesIO(raw['image'])).convert('RGBA')
        rows = {row[0]: row for row in doc[0].get_images(full=True)}
        mask = rows[xref][1]
        if mask:
            mask_image = Image.open(io.BytesIO(doc.extract_image(mask)['image'])).convert('L')
            im.putalpha(mask_image.resize(im.size, Image.Resampling.BICUBIC))
        if name == 'logo':
            box = ImageChops.difference(im.convert('RGB'), Image.new('RGB', im.size, 'white')).getbbox()
            im = im.crop(box)
        save(name, im, {'document': 'C1009 UK 2022', 'page': 1, 'object': xref, 'mask': mask}, square=name in ['house', 'hotel'])
    for name, rect in SYMBOLS.items():
        pix = doc[1].get_pixmap(matrix=pymupdf.Matrix(5, 5), clip=pymupdf.Rect(rect), alpha=False)
        im = Image.frombytes('RGB', (pix.width, pix.height), pix.samples).convert('RGBA')
        pixels = np.array(im)
        # The printed pale-green square is a background, not part of the icon.
        background = (pixels[:,:,1] > 175) & (pixels[:,:,0] > 160) & (pixels[:,:,2] > 160) & (pixels[:,:,1] > pixels[:,:,0] + 8)
        pixels[:,:,3][background] = 0
        if name == 'go':
            # Remove clipped pieces of the enclosing diamond that touch a crop
            # edge; the GO lettering and arrow are separate interior components.
            from collections import deque
            h, w = pixels.shape[:2]
            queue = deque([(x, y) for x in range(w) for y in [0,h-1]] + [(x, y) for y in range(h) for x in [0,w-1]])
            while queue:
                x, y = queue.popleft()
                if not (0 <= x < w and 0 <= y < h) or not pixels[y,x,3]: continue
                pixels[y,x,3] = 0
                queue.extend([(x-1,y), (x+1,y), (x,y-1), (x,y+1)])
        save('symbol-' + name, Image.fromarray(pixels), {'document': 'C1009 UK 2022', 'page': 2, 'rectangle': rect, 'processing': 'crop vector art; remove pale-green background'}, square=True)
    for i in range(6):
        x, y = (i % 3) * 640, (i // 3) * 640
        im = tokens.crop((x, y, x+640, y+640))
        pixels = np.array(im)
        pixels[:,:,3][np.min(pixels[:,:,:3], axis=2) > 240] = 0
        save(f'token-{i+1}', Image.fromarray(pixels), {'url': REMOTE+'all.png', 'sha256': TOKEN_HASH, 'tile': i, 'description': 'Third-party classic token illustration, not an official metal-token photograph'}, square=True)
    for name, digest in pins.items():
        src, path = args.sources / (name + '.mp3'), output / 'audio' / (name + '.mp3')
        # Short cues, fixed sample rate, conservative normalized loudness, no metadata.
        duration = 2.4 if name == 'winning' else 1.4 if name in ['rolling', 'jail', 'dying'] else .9
        subprocess.run(['ffmpeg', '-hide_banner', '-loglevel', 'error', '-y', '-i', str(src), '-t', str(duration), '-af', f'loudnorm=I=-22:TP=-3:LRA=7,afade=t=out:st={duration-.15}:d=0.15', '-ar', '44100', '-ac', '1', '-codec:a', 'libmp3lame', '-b:a', '96k', '-map_metadata', '-1', str(path)], check=True)
        subprocess.run(['ffmpeg', '-v', 'error', '-xerror', '-i', str(path), '-f', 'null', '-'], check=True)
        probe = json.loads(subprocess.check_output(['ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'json', str(path)]))
        media.append({'file': path.relative_to(output).as_posix(), 'mime': 'audio/mpeg', 'duration': float(probe['format']['duration']), 'source': {'url': REMOTE+name+'.mp3', 'sha256': digest, 'processing': 'shortened, mono 44.1kHz, -22 LUFS, fade out'}})
    for row in media:
        body = (output / row['file']).read_bytes()
        row.update(bytes=len(body), sha256=hashlib.sha256(body).hexdigest())
    manifest = {'version': 'monopoly/v1', 'media': media, 'cardArt': 'Chance/chest reuse category symbols with live Chinese card text; these are not 32 full original card scans.'}
    (output / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2)+'\n')
    images = [r for r in media if r['mime'].startswith('image/')]
    sheet = Image.new('RGB', (800, ((len(images)+4)//5)*190), '#eff3e9')
    draw = ImageDraw.Draw(sheet)
    for i, row in enumerate(images):
        im = Image.open(output / row['file']).convert('RGBA');im.thumbnail((145,145))
        x,y=(i%5)*160,(i//5)*190
        sheet.paste(im,(x+(160-im.width)//2,y),im)
        draw.text((x+4,y+153),Path(row['file']).stem,fill='black')
    sheet.save(output / 'contact-sheet.png')
    print(f'Prepared and decoded {len(media)} media files, {sum(r["bytes"] for r in media)} bytes.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sources', type=Path, required=True)
    parser.add_argument('--rules', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    prepare(parser.parse_args())
