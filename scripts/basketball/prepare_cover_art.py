#!/usr/bin/env python3
"""Prepare the sourced sports-character cutout; no image-generation model.

The input is the separately documented official dodgeball character illustration,
not a basketball box scan. Preserve its silhouettes and occlusion while removing
outer background and redrawing the held ball. Original media stays in .local.
Requires Pillow; output stays inside this checkout.
"""
import argparse
from collections import deque
import hashlib
import json
import math
from pathlib import Path
from PIL import Image, ImageChops, ImageDraw, ImageFilter

ROOT = Path(__file__).resolve().parents[2]


def prepare(source, output):
    pin = json.loads((ROOT / 'games/basketball/assets/cover-manifest.json').read_text())
    assert hashlib.sha256(source.read_bytes()).hexdigest() == pin['source']['sha256']
    im = Image.open(source).convert('RGB')
    assert im.size == (237, 264)
    w, h = im.size
    ink = im.convert('L').point(lambda v: 255 if v < 106 else 0).filter(ImageFilter.MaxFilter(3))
    blocked, outside, queue = ink.load(), set(), deque()
    for x in range(w): queue.extend([(x, 0), (x, h - 1)])
    for y in range(h): queue.extend([(0, y), (w - 1, y)])
    while queue:
        x, y = queue.popleft()
        if (x, y) in outside or blocked[x, y]: continue
        outside.add((x, y))
        for a, b in ((x - 1, y), (x + 1, y), (x, y - 1), (x, y + 1)):
            if 0 <= a < w and 0 <= b < h and (a, b) not in outside: queue.append((a, b))
    alpha = Image.new('L', im.size, 255)
    pixels = alpha.load()
    for x, y in outside: pixels[x, y] = 0
    im = im.convert('RGBA')
    im.putalpha(alpha.filter(ImageFilter.MinFilter(3)))
    mask = Image.new('L', (w, h))
    pixels, selected = im.load(), mask.load()
    for y in range(103, 184):
        for x in range(2, 69):
            r, g, b, a = pixels[x, y]
            if a and r > 85 and g > 65 and b < g * .82 and g > r * .60 and r > g * .93: selected[x, y] = 255
    mask = mask.filter(ImageFilter.MaxFilter(5)).filter(ImageFilter.MinFilter(5))
    clip = Image.new('L', (w, h))
    ImageDraw.Draw(clip).polygon([(42,107),(54,109),(65,114),(62,131),(56,149),(60,157),(60,176),(48,180),(32,180),(17,174),(8,161),(5,145),(10,128),(20,116),(31,109)], fill=255)
    scale = 4
    mask = ImageChops.multiply(mask, clip).resize((w * scale, h * scale), Image.Resampling.LANCZOS)
    layer = Image.new('RGBA', (w * scale, h * scale))
    pixels = layer.load()
    for y in range(103 * scale, 184 * scale):
        for x in range(2 * scale, 69 * scale):
            t = min(1, math.hypot(x / scale - 24, y / scale - 118) / 68)
            pixels[x, y] = tuple(round(a + (b - a) * t) for a, b in zip((231,163,82), (147,80,45))) + (255,)
    draw = ImageDraw.Draw(layer)
    for control in [((37,106),(28,130),(29,158),(43,183)), ((3,145),(23,136),(42,139),(63,152)), ((14,117),(29,114),(57,133),(62,164)), ((8,161),(21,166),(47,165),(62,158))]:
        points = []
        for n in range(100):
            t, u = n / 99, 1 - n / 99
            points.append(tuple((u ** 3 * control[0][a] + 3 * u * u * t * control[1][a] + 3 * u * t * t * control[2][a] + t ** 3 * control[3][a]) * scale for a in (0, 1)))
        draw.line(points, fill='#513a29', width=5)
    result = im.resize(layer.size, Image.Resampling.LANCZOS)
    result.paste(layer, (0, 0), mask)
    output.parent.mkdir(parents=True, exist_ok=True)
    result.resize(im.size, Image.Resampling.LANCZOS).save(output)
    digest = hashlib.sha256(output.read_bytes()).hexdigest()
    assert digest == pin['sha256'], 'Image differs from the selected/published cover pin'
    return {'sha256': digest, 'bytes': output.stat().st_size, 'dimensions': list(im.size)}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, default=Path('.local/basketball-character-references/dodge-kunio-riki.jpg'))
    parser.add_argument('--output', type=Path, default=Path('.local/basketball-cover/heroes.png'))
    args = parser.parse_args()
    source, output = (ROOT / args.source).resolve(), (ROOT / args.output).resolve()
    assert source.is_relative_to(ROOT) and output.is_relative_to(ROOT)
    print(json.dumps(prepare(source, output)))
