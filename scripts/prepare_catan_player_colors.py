#!/usr/bin/env python3
"""Add fifth/sixth player colors while retaining existing original piece shading.

Only blue-painted pixels are recolored. Inputs come from the deployment's
existing asset service; its address is supplied at runtime, never stored here.
"""
import argparse
import colorsys
import io
from pathlib import Path
import urllib.request
from PIL import Image


def prepare(output: Path, base: str):
    target = output / 'catan'
    target.mkdir(parents=True, exist_ok=True)
    for kind in ('settlement', 'city'):
        with urllib.request.urlopen(base.rstrip('/') + f'/catan/{kind}-blue-v1.webp', timeout=30) as response:
            original = Image.open(io.BytesIO(response.read())).convert('RGBA')
        for name, hue in (('purple', 0.80), ('green', 0.36)):
            result = original.copy()
            pixels = []
            for r, g, b, a in zip(*[iter(original.tobytes())] * 4):
                h, s, v = colorsys.rgb_to_hsv(r/255, g/255, b/255)
                if a and 0.48 <= h <= 0.72 and s > 0.15:
                    red, green, blue = colorsys.hsv_to_rgb(hue, s * 0.8, v * 0.8)
                    r, g, b = round(red*255), round(green*255), round(blue*255)
                pixels.append((r,g,b,a))
            result.putdata(pixels)
            result.save(target / f'{kind}-{name}-v1.webp', lossless=True, method=6)
    print('Prepared four shaded pieces; no temporary files retained.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('output', type=Path)
    parser.add_argument('--asset-base-url', required=True)
    args = parser.parse_args()
    prepare(args.output, args.asset_base_url)
