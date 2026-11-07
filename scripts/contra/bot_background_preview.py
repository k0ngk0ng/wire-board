#!/usr/bin/env python3
"""Render immutable recovered supertiles for recipe review, not live rule state."""
from pathlib import Path
import hashlib
import json
from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[2]
PUBLIC = ROOT / '.local/contra-browser/build/public'
OUT = ROOT / '.local/contra-background-vision'
MANIFEST = json.loads((PUBLIC / 'manifest.json').read_text())

def asset(value):
    path = PUBLIC / value['url'].lstrip('/')
    path.resolve().relative_to(PUBLIC.resolve())
    raw = path.read_bytes()
    assert len(raw) == value['bytes']
    assert hashlib.sha256(raw).hexdigest() == value['sha256']
    return raw

RGB = asset(MANIFEST['rgb'])
PAGES = {int(k): asset(v) for k, v in MANIFEST['pages'].items()}

def make(level, variant, alternate, title):
    data = json.loads((ROOT / f'games/contra/engine/leveldata/stage-{level + 1}.json').read_text())
    stage = data['stages'][0]
    sources = {x['id']: x for x in data['tileSets']}
    presets = {x['id']: x['pages'] for x in data['chrPresets']}
    colors = {x['id']: x['colors'] for x in data['paletteColors']}
    settings = stage['initialPaletteState']
    preset = stage['chrPreset']
    tileset = sources[stage['tileSetId']]
    if alternate:
        settings = stage['alternate']
        preset = stage['alternate']['chrPreset']
    if variant == 'boss':
        tileset = sources[stage['baseBoss']['tileSetId']]
        settings = stage['alternate']
        preset = stage['baseBoss']['chrPreset']
    mapping = presets[preset]
    palette = []
    for group in settings['paletteIndices']:
        palette.extend([15] + colors[group])
    size = (96 * 12, 98 * ((len(tileset['supertiles']) + 11)//12))
    sheet = Image.new('RGB', size, (225, 225, 225))
    draw = ImageDraw.Draw(sheet)
    for index, block in enumerate(tileset['supertiles']):
        tile = Image.new('RGB', (32, 32))
        for y in range(32):
            for x in range(32):
                tx, ty = x//8, y//8
                code = block['tiles'][ty*4+tx]
                page = PAGES[mapping[4+(code >> 6)]]
                code &= 63
                at = ((code >> 3)*8 + y%8)*64 + (code & 7)*8 + x%8
                value = page[at]
                group = (block['attribute'] >> ((tx//2 + 2*(ty//2))*2)) & 3
                color = palette[group*4+value] if value else palette[0]
                tile.putpixel((x, y), tuple(RGB[(color&63)*3:(color&63)*3+3]))
        px, py = (index % 12)*96, (index // 12)*98
        sheet.paste(tile.resize((64,64), Image.Resampling.NEAREST), (px+16, py))
        label = f'B{index:02X}'
        if index >= tileset['updateBase']:
            label += f'/U{index-tileset["updateBase"]:02X}'
        draw.text((px+12,py+66), label, fill=(0,0,0))
    OUT.mkdir(parents=True, exist_ok=True)
    path = OUT / f'{title}.png'
    sheet.save(path)
    print(path.relative_to(ROOT))

if __name__ == '__main__':
    make(0, 'regular', True, 'jungle-boss-supertiles')
    make(1, 'regular', False, 'base-room-supertiles')
    make(1, 'boss', False, 'base-boss-supertiles')

# The 16x16 Base source is independently captured from the original animation
# table. 0x8x calls retain palette group 3; use each stage's public color cycles.
if __name__ == '__main__':
    import gzip
    reference = json.loads(gzip.decompress((ROOT/'games/contra/engine/testdata/bot-background-original-art.json.gz').read_bytes()))
    patterns = {p['code']: p['tiles'] for p in reference['patterns'] if p['level']==1 and p['table']=='tile-animation'}
    data = json.loads((ROOT/'games/contra/engine/leveldata/stage-2.json').read_text())
    stage=data['stages'][0]
    mapping=next(p['pages'] for p in data['chrPresets'] if p['id']==stage['chrPreset'])
    colors={p['id']:p['colors'] for p in data['paletteColors']}
    cycles=list(dict.fromkeys(stage['initialPaletteState']['cycleIndices']))
    image=Image.new('RGB',(11*88,len(cycles)*100),(225,225,225));draw=ImageDraw.Draw(image)
    for row,group in enumerate(cycles):
        palette=[15]+colors[group]
        for code,tiles in sorted(patterns.items()):
            tile=Image.new('RGB',(16,16))
            for y in range(16):
                for x in range(16):
                    value=tiles[(y//8)*2+x//8];page=PAGES[mapping[4+(value>>6)]];value&=63
                    index=page[((value>>3)*8+y%8)*64+(value&7)*8+x%8];rgb=palette[index]*3
                    tile.putpixel((x,y),tuple(RGB[rgb:rgb+3]))
            image.paste(tile.resize((64,64),Image.Resampling.NEAREST),(code*88+12,row*100))
            draw.text((code*88+5,row*100+68),f'A{code:02X} P{group:02X}',fill=(0,0,0))
    path=OUT/'base-animations-palette-cycles.png';image.save(path);print(path.relative_to(ROOT))
