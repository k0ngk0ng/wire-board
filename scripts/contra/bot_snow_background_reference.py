#!/usr/bin/env python3
"""Independent Snow artwork oracle: pinned ROM bytes, never recipe code."""
import base64
import gzip
import hashlib
import json
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SHA = '6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def main():
    sources = {}

    def read(name):
        raw = (ROOT / name).read_bytes()
        sources[name] = digest(raw)
        return json.loads(gzip.decompress(raw) if name.endswith('.gz') else raw)

    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip') as z:
        rom = z.read('Contra (J).nes')
    assert digest(rom) == SHA
    catalog = read('docs/research/nes-contra/evidence/levels-catalog.json.gz')
    stage = catalog['stages'][4]
    tiles = next(t for t in catalog['tileSets'] if t['id'] == stage['tileSetId'])

    def verify(ref, raw):
        assert len(raw) == ref['length']
        assert digest(raw) == ref['sha256']
        assert rom[ref['romOffset']:ref['romOffset'] + len(raw)] == raw

    verify(tiles['tilesSource'], bytes(x for t in tiles['supertiles'] for x in t['tiles']))
    verify(tiles['attributesSource'], bytes(t['attribute'] for t in tiles['supertiles']))
    presets = {p['id']: p for p in catalog['chrPresets']}
    for i in (4, 15):
        verify(presets[i]['source'], bytes(presets[i]['pages']))
    alt = stage['alternate']
    assert alt['chrPreset'] == 15
    alt_raw = bytes(alt['collisionLimits'] + alt['cycleIndices'] + alt['paletteIndices'])
    verify(alt['source'], alt_raw)
    tables = read('docs/research/nes-contra/evidence/enemies-snow-tables.json')['tables']
    for t in tables.values():
        raw = bytes(t['bytes'])
        assert digest(raw) == t['sha256']
        assert rom[t['rom_offset']:t['rom_offset'] + len(raw)] == raw
    assert tables['snowTankTurrets']['bytes'] == [0x13, 0x12, 0x0f]
    assert tables['snowCarrierTop']['bytes'][::2] == [0x0b, 0x0a, 0x09, 0x0d]

    visual = read('games/contra/server/data/visual-catalog-v1.json.gz')
    rgb = base64.b64decode(visual['rgb'])
    assert len(rgb) == 192 and digest(rgb) == visual['catalog']['rgb']['sha256']
    # Verify the planar decode independently of the production visual exporter.
    for page in set(presets[4]['pages'] + presets[15]['pages']):
        decoded = bytearray(4096)
        for tile in range(64):
            for y in range(8):
                at = 16 + 128 * 1024 + page * 1024 + tile * 16 + y
                for x in range(8):
                    value = ((rom[at] >> (7-x)) & 1) | (((rom[at+8] >> (7-x)) & 1) << 1)
                    decoded[((tile//8)*8+y)*64 + (tile%8)*8+x] = value
        assert decoded == base64.b64decode(visual['pages'][str(page)])

    first = catalog['paletteColors'][0]
    color_base = first['source']['romOffset'] - first['id'] * 3
    colors = {i: list(rom[color_base+i*3:color_base+i*3+3]) for i in range(108)}
    patterns = tiles['supertiles'][tiles['updates']['baseSupertile']:]
    normal_chr, boss_chr = presets[4]['pages'], presets[15]['pages']
    tank_palettes = [[61, 62, i, 65, 0, 1, 34, 7] for i in sorted(set(tables['snowTankPalettes']['bytes']))]
    carrier_palette = alt['paletteIndices']

    def render(code, chr_pages, indices, backdrop=15):
        block = patterns[code]
        rgba = bytearray()
        for y in range(32):
            for x in range(32):
                tile = block['tiles'][(y//8)*4+x//8]
                page = chr_pages[4+(tile >> 6)]
                at = 16 + 128*1024 + page*1024 + (tile & 63)*16 + y % 8
                value = ((rom[at] >> (7-x % 8)) & 1) | (((rom[at+8] >> (7-x % 8)) & 1) << 1)
                group = (block['attribute'] >> ((y//16*2+x//16)*2)) & 3
                color = colors[indices[group]][value-1] if value else backdrop
                rgba.extend(rgb[color*3:color*3+3])
                rgba.append(255)
        return bytes(rgba)

    cases = []
    for code, anchor in [(15, [3, 12]), (18, [6, 22]), (19, [10, 27]), (9, [28, 16]), (10, [28, 16]), (11, [28, 16]), (13, [28, 16])]:
        carrier = code in [9, 10, 11, 13]
        chr_pages = boss_chr if carrier else normal_chr
        for palette in [carrier_palette] if carrier else tank_palettes:
            for backdrop in [15, 32, 22, 17]:
                cases.append({'code': code, 'kind': 20 if carrier else 18,
                              'family': 'snow-carrier' if carrier else 'snow-tank',
                              'anchor': anchor, 'chr': chr_pages, 'paletteIndices': palette,
                              'backdrop': backdrop, 'rgba': render(code, chr_pages, palette, backdrop).hex()})
    negatives = []
    for code in [16, 17, 20, 21, 22, 23, 24, 25, 26, 27]:
        carrier = code >= 22
        negatives.append({'reason': 'wheel/scenery/exit/erased, not full target artwork', 'code': code,
                          'rgba': render(code, boss_chr if carrier else normal_chr,
                                         carrier_palette if carrier else tank_palettes[0]).hex()})
    for code in [9, 10, 11, 13]:
        negatives.append({'reason': 'wrong CHR82 for Carrier; not a licensed complete appearance', 'code': code,
                          'rgba': render(code, normal_chr, carrier_palette).hex()})
    negatives.append({'reason': 'fully black disappearance', 'code': 27,
                      'rgba': (bytes(rgb[45:48]) + b'\xff').hex()*1024})
    original = read('games/contra/engine/testdata/original-enemies-snow-complex-cases.json.gz')
    assert original['source_is_original_execution'] and original['source']['rom_sha256'] == SHA
    observations = []
    for row in original['records']:
        for event in row['events']:
            if event['kind'] == 'palette' or event['kind'] == 'terrain' and event['afterUsed'] > event['used']:
                observations.append({'record': row['id'], 'event': event})
    rea = read('docs/research/nes-contra/evidence/enemies-snow-rea.json.gz')
    ids = [{'id': r['evidence_id'], 'parameters': r['parameters']}
           for r in rea.get('records', []) if r['operation'] == 'analyze_function']
    result = {'romSHA256': SHA, 'scope': 'immutable Snow artwork and legal full-color context; not live/dead inference or natural campaign',
              'sources': sources, 'rea': ids, 'patterns': patterns, 'visualCases': cases, 'negativeCases': negatives,
              'originalControlledObservations': observations,
              'limitations': ['Controlled carrier captures used an earlier Tank palette; actual Carrier appearance uses independently verified alternate header/CHR.',
                              'Partially faded colors remain unknown; intact artwork during destruction cannot establish hidden liveness.']}
    target = ROOT/'games/contra/engine/testdata/bot-snow-original-art.json.gz'
    target.write_bytes(gzip.compress(json.dumps(result, sort_keys=True, separators=(',', ':')).encode(), mtime=0))
    # Review sheet is an artifact, never used as input to the classifier.
    from PIL import Image, ImageDraw
    sheet = Image.new('RGB', (8*144, 5*160), (220, 220, 220))
    draw = ImageDraw.Draw(sheet)
    for code in range(len(patterns)):
        carrier = code in range(3, 15) or code in range(22, 28)
        raw = render(code, boss_chr if carrier else normal_chr, carrier_palette if carrier else tank_palettes[0])
        image = Image.frombytes('RGBA', (32, 32), raw)
        x, y = code % 8 * 144, code // 8 * 160
        sheet.paste(image.resize((128, 128), Image.Resampling.NEAREST), (x, y))
        draw.text((x, y+130), f'U{code:02X} attr{patterns[code]["attribute"]:02X}', fill=(0, 0, 0))
    selected = [c for c in cases if c['backdrop'] == 15 and c['paletteIndices'] in [tank_palettes[0], carrier_palette]]
    for col, case in enumerate(selected):
        x, y = col*144, 4*160
        image = Image.frombytes('RGBA', (32, 32), bytes.fromhex(case['rgba']))
        sheet.paste(image.resize((128, 128), Image.Resampling.NEAREST), (x, y))
        ax, ay = x+case['anchor'][0]*4, y+case['anchor'][1]*4
        draw.line((ax-6, ay, ax+6, ay), fill=(255, 0, 255), width=2)
        draw.line((ax, ay-6, ax, ay+6), fill=(255, 0, 255), width=2)
        draw.text((x, y+130), f'U{case["code"]:02X} {case["anchor"]}', fill=(0, 0, 0))
    path = ROOT/'docs/research/nes-contra/evidence/bot-snow-original-art.png'
    sheet.save(path)
    print(json.dumps({'fixture': str(target.relative_to(ROOT)), 'sha256': digest(target.read_bytes()),
                      'patterns': len(patterns), 'positive': len(cases), 'negative': len(negatives),
                      'controlledObservations': len(observations), 'reaFunctions': len(ids)}, indent=2))


if __name__ == '__main__':
    main()
