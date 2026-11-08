#!/usr/bin/env python3
"""Read existing StoneAge tables/indices; emit rule and URL metadata only.

Never copies, downloads or uploads a bitmap/audio file. The source repository
is read-only. Run from Wire Board with --source pointing to the StoneAge repo.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import struct

ROOT = Path(__file__).resolve().parents[3]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--source', type=Path, required=True)
    args = parser.parse_args()
    source = args.source.resolve()
    data = source / 'server/legacy/source/2.5/gmsv/data'
    assets = source / 'client/web/assets/original'
    sprites = json.loads((assets / 'sprites.json').read_text())['sprites']
    manifest = json.loads((assets / 'manifest.json').read_text())
    used = set()

    def rows(name):
        return [line.decode('gb18030', errors='strict').split(',')
                for line in (data / name).read_bytes().splitlines()
                if line.strip() and not line.lstrip().startswith(b'#')]

    def complete(graphic):
        if str(graphic) not in sprites:
            return False
        available = {(a['direction'], a['action']) for a in sprites[str(graphic)]['actions'] if a['frames']}
        return all((d, a) in available for d in (3, 7) for a in (0, 1, 2, 3, 4, 10))

    pets = []
    graphics = set()
    for row in rows('enemybase.txt'):
        graphic = int(row[36])
        if row[37].strip() != '1' or graphic in graphics or not complete(graphic):
            continue
        # Capture species weights/elements; equal-budget statistics are derived
        # by the independent server, not copied from random grown MMORPG pets.
        pets.append(dict(id=int(row[6]), name=row[0].strip(), graphic=graphic,
                         weights=[int(float(v)) for v in row[9:13]],
                         elements=[int(v) for v in row[15:19]]))
        graphics.add(graphic)
    # Spread across the entire verified classic collection, keeping familiar
    # early species as well as later families. Deduplicate by graphic above.
    selected = pets[:24] + [p for i, p in enumerate(pets[24:]) if i % 3 == 0][:48]
    selected_by_graphic = {p['graphic']: p for p in pets}
    native = (source / 'server/legacy/source/2.5/gmsv/char/char_base.c').read_bytes().decode('gb18030')
    table = native.split('ridePetTable[296] =', 1)[1].split('};', 1)[0]
    ride_maps = {}
    for line in table.splitlines():
        if line.strip().startswith('//'):
            continue
        match = re.search(r'\{\s*(\d+),\s*(\d+),\s*(\d+),\s*(\d+)', line)
        if not match:
            continue
        graphic, avatar, pet, _ = map(int, match.groups())
        if avatar < 100240 and complete(graphic) and pet in selected_by_graphic:
            ride_maps.setdefault(pet, {})[avatar] = graphic
    rides = []
    for pet, variants in ride_maps.items():
        item = dict(selected_by_graphic[pet])
        # Two verified rider appearances for every mount, including originals.
        item['riders'] = [dict(avatar=k, graphic=v) for k, v in sorted(variants.items())[:2]]
        rides.append(item)
        used.update(v['graphic'] for v in item['riders'])
        used.update(v['avatar'] + kind for v in item['riders'] for kind in range(5)
                    if complete(v['avatar'] + kind))
    assert len(rides) >= 8, 'insufficient complete mounted animations'

    items = rows('itemset.txt')
    by_id = {int(r[16]): r for r in items}

    def equipment(row, slot, variant):
        image = str(int(row[17]))
        bitmap = manifest['bitmaps'].get(image)
        if not bitmap:
            bitmap = manifest['bitmaps'].get(str(manifest['bitmap_aliases'].get(image)))
        assert bitmap, (row[0], image)
        return dict(id=int(row[16]), name=row[0].strip(), slot=slot,
                    weaponType=int(row[19]) if slot == 'weapon' else -1,
                    icon=bitmap['file'], variant=variant,
                    sourceDescription=row[2].strip())

    gear = []
    # Use actual original tiered equipment, preserving original names/icons.
    # Arena stat budgets and attached magic are explicit in catalog.go.
    for kind in (0, 1, 2, 3, 4, 17, 18, 19):
        choices = [r for r in items if int(r[19]) == kind and int(r[16]) % 100 in (40, 50, 60)]
        for i, row in enumerate(choices[:3]):
            gear.append(equipment(row, 'weapon', i))
    for kind, slot in ((6, 'hat'), (7, 'clothes')):
        choices = [r for r in items if int(r[19]) == kind and int(r[16]) % 100 in (30, 40, 50, 60, 70, 80)]
        for i, row in enumerate(choices[:8]):
            gear.append(equipment(row, slot, i))
    choices = [r for r in items if int(r[19]) in (8, 9, 10, 11, 13, 15)]
    for i, row in enumerate(choices[:18]):
        gear.append(equipment(row, 'accessory', i))
    assert sum(g['slot'] == 'hat' for g in gear) >= 6
    assert sum(g['slot'] == 'clothes' for g in gear) >= 6

    used.update(p['graphic'] for p in selected + rides)
    art = {}
    resources = {}
    all_files = set()
    for graphic in sorted(used):
        assert complete(graphic), graphic
        actions = []
        for action in sprites[str(graphic)]['actions']:
            if action['direction'] not in (3, 7) or action['action'] not in (0, 1, 2, 3, 4, 10):
                continue
            frames = []
            for frame in action['frames']:
                path = assets / frame['file']
                payload = path.read_bytes()
                assert payload[:8] == b'\x89PNG\r\n\x1a\n', path
                width, height = struct.unpack('>II', payload[16:24])
                frames.append(dict(file=frame['file'], x=frame['xoffset'], y=frame['yoffset'],
                                   width=width, height=height, sound=frame.get('sound', 0)))
                all_files.add(frame['file'])
            actions.append(dict(direction=action['direction'], action=action['action'],
                                ticks=action['frame_ms'], frames=frames))
        art[str(graphic)] = actions
    all_files.update(g['icon'] for g in gear)
    all_files.add('battle/battle_00.png')
    ui={name:dict(normal=f'bitmaps/bitmap_{normal}.png',pressed=f'bitmaps/bitmap_{pressed}.png')
        for name,normal,pressed in [('attack',8684,8685),('magic',8686,8687),('guard',8692,8693),('items',8694,8695)]}
    all_files.update(file for pair in ui.values() for file in pair.values())
    for name in sorted(all_files):
        payload = (assets / name).read_bytes()
        resources[name] = dict(bytes=len(payload), sha256=hashlib.sha256(payload).hexdigest())
    provenance = {str(p.relative_to(source)): hashlib.sha256(p.read_bytes()).hexdigest()
                  for p in [data / n for n in ('enemybase.txt', 'petskill.txt', 'itemset.txt')] +
                  [assets / 'sprites.json', assets / 'manifest.json',
                   source / 'server/legacy/source/2.5/gmsv/char/char_base.c',
                   source / 'client/web/runtimeassets/index.html']}
    out = ROOT / 'games/stoneage'
    (out / 'server').mkdir(parents=True, exist_ok=True)
    (out / 'web').mkdir(parents=True, exist_ok=True)
    (out / 'server/catalog_source.json').write_text(json.dumps(dict(pets=selected, rides=rides, equipment=gear), ensure_ascii=False, separators=(',', ':')) + '\n')
    (out / 'web/art.json').write_text(json.dumps(dict(sprites=art, resources=resources, ui=ui), ensure_ascii=False, separators=(',', ':')) + '\n')
    (out / 'source-manifest.json').write_text(json.dumps(dict(format=1, root='https://cdn.ichenj.com/stoneage', inputs=provenance,
        counts=dict(pets=len(selected), rides=len(rides), equipment=len(gear), sprites=len(art), bitmaps=len(resources)),
        assetsCopied=0, assetsUploaded=0), ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(dict(pets=len(selected), rides=len(rides), equipment=len(gear), sprites=len(art), bitmapReferences=len(resources), assetsCopied=0)))
    from expand_avatars import expand
    expand(source)


if __name__ == '__main__':
    main()
