#!/usr/bin/env python3
"""Pin public background artwork to the original cartridge and retained traces."""
from pathlib import Path
import collections
import gzip
import hashlib
import json
import zipfile

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA = '6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
sha = lambda b: hashlib.sha256(b).hexdigest()

def read(name):
    p = ROOT / name
    raw = p.read_bytes()
    value = gzip.decompress(raw) if name.endswith('.gz') else raw
    return json.loads(value), {'path': name, 'sha256': sha(raw)}

with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip') as z:
    candidates = [z.read(n) for n in z.namelist() if n.lower().endswith('.nes')]
rom = next(b for b in candidates if sha(b) == ROM_SHA)
catalog, catalog_source = read('docs/research/nes-contra/evidence/levels-catalog.json.gz')
assert catalog['romSHA256'] == ROM_SHA
patterns = []
sources = [catalog_source]
sets = {t['id']: t for t in catalog['tileSets']}
for level in [0, 1, 2, 3]:
    stage = catalog['stages'][level]
    choices = [(0, stage['tileSetId'])]
    if level in [1, 3]:
        choices.append((1, 'b68a-be8a'))
    for variant, name in choices:
        tiles = sets[name]
        for field, raw in [('tilesSource', bytes(x for t in tiles['supertiles'] for x in t['tiles'])),
                           ('attributesSource', bytes(t['attribute'] for t in tiles['supertiles']))]:
            ref = tiles[field]
            assert sha(raw) == ref['sha256']
            assert rom[ref['romOffset']:ref['romOffset']+ref['length']] == raw
        for table in ['base', 'update']:
            start = 0 if table == 'base' else tiles['updates']['baseSupertile']
            count = len(tiles['supertiles']) if table == 'base' else tiles['updates']['count']
            for code in range(count):
                t = tiles['supertiles'][start+code]
                patterns.append({'level': level, 'variant': variant, 'table': table, 'code': code,
                                 'width': 4, 'height': 4, 'tiles': t['tiles'], 'attribute': t['attribute']})
graphics, graphics_source = read('games/contra/engine/testdata/original-enemy-machines-graphics-controlled.json.gz')
sources.append(graphics_source)
assert graphics['source_is_original_execution']
for level in [1, 3]:
    definition = graphics['definitions'][str(level)]
    raw = bytes.fromhex(definition['raw'])
    assert sha(raw) == definition['sha256']
    assert rom[16+definition['prgOffset']:16+definition['prgOffset']+len(raw)] == raw
    assert len(raw) == 11*5
    for code in range(11):
        row = raw[code*5:code*5+5]
        patterns.append({'level': level, 'variant': 0, 'table': 'tile-animation', 'code': code,
                         'width': 2, 'height': 2, 'tiles': list(row[1:]), 'attribute': row[0]})
observations = []
for name in ['enemies-base-2-room.json.gz', 'enemies-base-4-room.json.gz']:
    capture, ref = read('docs/research/nes-contra/evidence/'+name)
    sources.append(ref)
    assert capture['source_is_original_execution'] and capture['source']['rom_sha256'] == ROM_SHA
    level = capture['stage']-1
    ts = sets[catalog['stages'][level]['tileSetId']]['supertiles']
    unique = {}
    for r in capture['records']:
        if r['type'] not in [19, 20]:
            continue
        w = r['before']['world']
        mapped = bytes.fromhex(r['before']['supertiles'])
        for q in r['requests']:
            if q['kind'] != 'tiles' or q['used'] >= 80:
                continue
            sy = q['y']+w[21]
            py = (sy+(16 if sy>=240 else 0)) & 255
            px = (q['x']+w[22]) & 255
            page = (w[23]&1) ^ (1 if q['x']+w[22]>=256 else 0)
            tx, ty = (px&248)//8, (py&248)//8
            at = page*64+(ty//4)*8+tx//4
            attr = ts[mapped[at]]['attribute']
            group = (attr >> (((ty&2)|((tx>>1)&1))*2)) & 3
            actor = r['before']['pool'][r['slot']]
            anchor = [actor[2]-q['x']+((q['x']+w[22])&7), actor[1]-q['y']+((q['y']+w[21])&7)]
            assert group == 3 and anchor == [8, 8]
            key = (level, r['type'], q['code']&127)
            unique.setdefault(key, {'source': ref['path'], 'record': r['id'], 'level': level,
                                   'visualOwner': r['type'], 'code': q['code']&127,
                                   'paletteGroup': group, 'anchor': anchor})
    observations.extend(unique.values())
rea = []
for name in ['enemies-base-rea.json.gz','enemies-shooters-rea.json.gz','enemies-shooters-bossdoor-rea.json.gz']:
    records, ref = read('docs/research/nes-contra/evidence/'+name)
    sources.append(ref)
    rea.append({'source': ref['path'], 'records': len(records.get('records',[])), 'romSHA256': ROM_SHA})
result = {'version':1,'romSHA256':ROM_SHA,'scope':'immutable ROM artwork byte identity and original visible update context, not a natural campaign or runtime oracle for the classifier','sources':sources,'rea':rea,'patterns':patterns,'basePaletteObservations':observations}
out = ROOT/'games/contra/engine/testdata/bot-background-original-art.json.gz'
raw = json.dumps(result,sort_keys=True,separators=(',',':')).encode()
out.write_bytes(gzip.compress(raw,mtime=0))
print(json.dumps({'path':str(out.relative_to(ROOT)),'sha256':sha(out.read_bytes()),'patterns':len(patterns),'paletteObservations':len(observations)},indent=2))
