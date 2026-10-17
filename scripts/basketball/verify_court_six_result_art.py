"""Compare six result palettes to captured PPU patterns and original $9526.

Palette 15 has natural-result screenshot evidence in accept_renderer.py.
Other variants below are explicit static recolorings using independently executed
original palette buffers, not claims of naturally reached result screenshots.
"""
from hashlib import sha256
import json
from pathlib import Path
from PIL import Image, ImageDraw
from graphics import decode_tile, render_background
from palette_fade import fade_nes_index
from result_art import palette_for_court
from static_sprites import SpriteROM

ROOT = Path(__file__).resolve().parents[2]
ART = ROOT / '.local/basketball'
OUT = ART / 'rea/court-six'


def main():
    rom_bytes = (ROOT / '.local/nes-basket/basketball.nes').read_bytes()
    rom = SpriteROM(rom_bytes)
    oracle = json.loads((OUT / 'result-palette-oracle.json').read_text())
    assert oracle['source_rom_sha256'] == sha256(rom_bytes).hexdigest()
    assert oracle['original_source_sha256'] == sha256(rom.prg[4*16384+0x1526:4*16384+0x162f]).hexdigest()
    assert len(oracle['cases']) == 120
    full = {(c['paletteID'], tuple(c['teams']), c['flags']): c['colors']
            for c in oracle['cases'] if c['level'] == 4}
    for case in oracle['cases']:
        expected = [fade_nes_index(v, case['level'])
                    for v in full[case['paletteID'], tuple(case['teams']), case['flags']]]
        assert expected == case['colors']
    capture = json.loads((OUT / 'result-capture.json').read_text())
    directory = ROOT / '.local/nes-basket/runs' / capture['run']
    reference = directory.parent / 'court-six-result-reference'
    assert (directory / 'ram.bin').read_bytes() == (reference / 'ram.bin').read_bytes()
    for frame in capture['frames']:
        name = f"frame-{frame['frame']:05d}.png"
        assert (directory / name).read_bytes() == (reference / name).read_bytes()
    stem = f"ppu-{capture['frames'][0]['emulator_frame']:05d}"
    field = (directory / (stem + '-field.bin')).read_bytes()
    hud = (directory / (stem + '-ppu.bin')).read_bytes()
    colors = (directory / 'nes-colors.rgb').read_bytes()
    records = json.loads((ART / 'results/manifest.json').read_text())['courts'][6]
    variants = {7: records, **{r['paletteID']: r for r in records['paletteVariants']}}
    reports = []
    for palette_id in [7, 11, 12, 13, 14, 15]:
        palette = full[palette_id, (0, 1), 0]
        assert list(palette_for_court(rom, 6, palette_id)) == palette[:16]
        mapped = bytearray(field)
        mapped[0x2000:0x2400] = field[0x2800:0x2c00]
        mapped[0x3f00:0x3f20] = bytes(palette)
        expected = {'background': render_background(mapped, colors).crop((0, 0, 256, 240)).convert('RGBA')}
        mapped = bytearray(hud)
        mapped[0x3f00:0x3f20] = bytes(palette)
        template = render_background(mapped, colors).crop((0, 0, 256, 48)).convert('RGBA')
        for x in [64, 144]:
            ImageDraw.Draw(template).rectangle((x, 8, x+47, 31), fill=(0, 0, 0, 0))
        expected['hud'] = template
        tiles = Image.new('RGBA', (128, 512))
        digits = Image.new('RGBA', (160, 24))

        def tile(image, left, top, tile_id, group):
            for y, row in enumerate(decode_tile(hud[tile_id*16:(tile_id+1)*16])):
                for x, index in enumerate(row):
                    color = palette[group*4+index if index else 0]
                    image.putpixel((left+x, top+y), tuple(colors[color*3:color*3+3]) + (255,))

        for group in range(4):
            for tile_id in range(256):
                tile(tiles, tile_id % 16 * 8, (group*16+tile_id//16)*8, tile_id, group)
        for digit, base in enumerate(rom.prg[4*16384+0x3d82:4*16384+0x3d8c]):
            for y in range(3):
                for x in range(2):
                    tile(digits, digit*16+x*8, y*8, base+y*16+x, 0)
        expected.update(hudTiles=tiles, digits=digits)
        for role, image in expected.items():
            entry = variants[palette_id][role]
            file = ART / 'results' / entry['file']
            assert sha256(file.read_bytes()).hexdigest() == entry['sha256']
            actual = Image.open(file).convert('RGBA')
            assert actual.size == image.size
            assert actual.tobytes() == image.tobytes(), (palette_id, role)
            reports.append({'paletteID': palette_id, 'role': role,
                            'pixels': image.width*image.height, 'sha256': entry['sha256']})
    report = {'status': 'passed', 'scope': 'captured original PPU pattern/table decoding with original palette-function buffers; palette15 additionally has natural screenshot evidence',
              'originalFunctionCalls': 120, 'palettes': [7, 11, 12, 13, 14, 15],
              'images': len(reports), 'pixels': sum(r['pixels'] for r in reports),
              'accepted_for_release': False, 'imagesDetail': reports}
    (OUT / 'result-art-verification.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({k: v for k, v in report.items() if k != 'imagesDetail'}))


if __name__ == '__main__':
    main()
