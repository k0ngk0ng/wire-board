import base64
from collections import deque
from compression import zstd
import gzip
import hashlib
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
BASE = ROOT / '.local/pal/verification/runtime-candidates/audio/development-health/hp-9999'
SOURCE = ROOT / '.local/pal/original-98/sss.mkf'
SSS = SOURCE.read_bytes()
assert hashlib.sha256(SSS).hexdigest() == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
OFFSETS = struct.unpack_from('<6I', SSS)
SCENES = [struct.unpack_from('<4H', SSS, at) for at in range(OFFSETS[1], OFFSETS[2], 8)]
DIRECTIONS = [(-16, 8), (-16, -8), (16, -8), (16, 8)]
CORE = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE

def current_events(path, target):
    data = path.read_bytes()
    state = json.loads(data)
    packed = base64.b64decode(state['native'], validate=True)
    raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
    assert raw[:8] == b'PALMEM01'
    size, memory_size = struct.unpack_from('<II', raw, 8)
    metadata = json.loads(raw[16:16+size])
    memory = raw[16+size:]
    assert len(memory) == memory_size <= 48*1024*1024
    assert metadata['buildID'] == CORE
    assert hashlib.sha256(memory + struct.pack('<I', metadata['stackPointer'])).hexdigest() == metadata['memoryID']
    original = SSS[OFFSETS[0]+(target-1)*32:OFFSETS[0]+target*32]
    needle = original[:8]
    at = memory.find(needle)
    candidates = []
    while at >= 0:
        base = at-(target-1)*32
        if base >= OFFSETS[0] and memory[base-OFFSETS[0]:base] != SSS[:OFFSETS[0]] and base+5077*32 <= len(memory):
            row = struct.unpack_from('<hHHhHHh9H', memory, at)
            if row[4:10] == struct.unpack('<hHHhHHh9H', original)[4:10]:
                candidates.append(base)
        at = memory.find(needle, at+1)
    assert len(candidates) == 1, (path.name, candidates)
    base = candidates[0]
    scene = state['public']['scene']
    events = [(index+1, struct.unpack_from('<hHHhHHh9H', memory, base+index*32)) for index in range(SCENES[scene-1][3], SCENES[scene][3])]
    return state['public'], events, hashlib.sha256(data).hexdigest()

def search_stances(row):
    target_x, target_y, mode = row[1], row[2], row[7]
    result = set()
    for dx, dy in DIRECTIONS:
        offsets = [(0, 0)]
        for step in range(4):
            offsets.extend([((step+1)*dx, (step+1)*dy), (step*dx, (step+2)*dy), ((step+2)*dx, step*dy)])
        for index, (x, y) in enumerate(offsets):
            if index < mode*6-4:
                result.add((target_x-x, target_y-y))
    return result

def review(name, target, portal_ids, optimistic=False):
    path = BASE / name
    view, events, digest = current_events(path, target)
    rows = dict(events)
    portals = [rows[index] for index in portal_ids if rows[index][6] > 0 and rows[index][0] == 0]
    blockers = [(row[1], row[2]) for _, row in events if row[6] >= 2]
    map_path = ROOT / f".local/pal/visual/pal/v1/maps/{view['map']}.bin.gz"
    ground = gzip.decompress(map_path.read_bytes())
    assert len(ground) == 65536 and view['phase'] == 'field'
    offset_x, offset_y = view['field']['offset']
    start = view['x'], view['y']
    goals = search_stances(rows[target])
    queue = deque([start])
    seen = {start}
    reachable_goals = []
    while queue:
        x, y = queue.popleft()
        if (x, y) in goals:
            reachable_goals.append([x, y])
        for dx, dy in DIRECTIONS:
            xx, yy = x+dx, y+dy
            if (xx, yy) in seen or xx < offset_x//32*32 or xx//32 >= 63-offset_x//32 or yy < offset_y//16*16 or yy >= 2048:
                continue
            if not optimistic and any(abs(xx-row[1])+2*abs(yy-row[2]) < (row[7]-4)*32+16 for row in portals):
                continue
            if not optimistic and any(abs(xx-a)+2*abs(yy-b) < 16 for a, b in blockers):
                continue
            index = ((yy//16*64+xx//32)*2+xx%32//16)*4
            if struct.unpack_from('<I', ground, index)[0] & 8192:
                continue
            seen.add((xx, yy))
            queue.append((xx, yy))
    return {'source': str(path.relative_to(ROOT)), 'sourceSHA256': digest, 'scene': view['scene'], 'map': view['map'], 'event': target, 'eventWorldXY': list(rows[target][1:3]), 'searchMode': rows[target][7], 'activeTrigger': rows[target][4], 'eventState': rows[target][6], 'eventFrame': rows[target][11], 'eventSprite': rows[target][8], 'position': list(start), 'partyOffset': [offset_x, offset_y], 'checkedWalkablePositions': len(seen), 'reachableSearchStances': reachable_goals, 'activePortals': [{'worldXY': list(row[1:3]), 'radius': (row[7]-4)*32+16} for row in portals], 'optimisticMapOnly': optimistic, 'mapSHA256': hashlib.sha256(map_path.read_bytes()).hexdigest(), 'normalPickupVerified': False}

bounty = review('yangzhou-freedom-complete-state-1.json', 1519, [1518])
bounty['classification'] = 'original inaccessible reward preserved; not counted as normally obtained'
bounty['guideViewportXY'] = [0x05b0, 0x0358]
assert [a+b for a, b in zip(bounty['guideViewportXY'], bounty['partyOffset'])] == bounty['eventWorldXY']
assert not bounty['reachableSearchStances']
capital = [review(f'capital-manor-step-{step}-state-1.json', 2173, [2161, 2162, 2163, 2164]) for step in [2, 13, 31, 42, 48]]
for row in capital:
    row['classification'] = 'reference study location confirmed; ordinary route remains unverified in checked states'
chemical = review('tower-upper-step-19-state-1.json', 2576, [2522], optimistic=True)
chemical['classification'] = 'original inaccessible reward preserved; explicit textual reference and disconnected static map'
assert not chemical['reachableSearchStances']
tower_pending = review('tower-rewards-167-failure-state-1.json', 2763, [2740], optimistic=True)
tower_pending['classification'] = 'no optimistic static route in checked state; further independent confirmation remains pending'
chemical_after = None
collection_path = BASE / 'tower-rewards-157-result-1.json'
if collection_path.is_file():
    collection = json.loads(collection_path.read_text())
    assert collection['passed'] and collection['originalRecordCount'] == 10
    assert len(collection['claimed']) == 9 and len(collection['preservedUnreachable']) == 1
    chemical_after = review('tower-rewards-157-state-1.json', 2576, [2522], optimistic=True)
    for key in ['eventWorldXY', 'searchMode', 'activeTrigger', 'eventState', 'eventFrame', 'eventSprite']:
        assert chemical_after[key] == chemical[key]
    assert not chemical_after['reachableSearchStances']
    chemical_after['classification'] = 'actual final native record unchanged after nine ordinary other pickups'
sources = []
for name, url in [('yangzhou-bounty-original-bug.md', 'https://www.palhero.net/?p=1210'), ('yangzhou-text-guide.md', 'https://gamegene.cn/wiki/361'), ('capital-text-guide.md', 'https://gamegene.cn/wiki/362'), ('tower-text-guide.md', 'https://gamegene.cn/wiki/363')]:
    path = ROOT / '.local/pal/hidden-items-research' / name
    assert path.is_file() and len(path.read_bytes()) > 100
    sources.append({'url': url, 'retainedText': str(path.relative_to(ROOT)), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
report = {'passed': True, 'diagnosticOnly': True, 'scope': 'read-only verified actual same-core snapshots, live-event table distinguished from original SSS copy, original search footprint and collision/portal graph; textual source classification, not successful pickup or all future story states', 'nativeBuildID': CORE, 'sssSHA256': hashlib.sha256(SSS).hexdigest(), 'bounty': bounty, 'capital': capital, 'chemical': chemical, 'chemicalAfterCollection': chemical_after, 'towerPending': tower_pending, 'textSources': sources, 'nativeEdits': False}
out = ROOT / '.local/pal/verification/hidden-text-route-review.json'
out.write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'diagnosticOnly': True, 'bountyClassification': bounty['classification'], 'capitalCheckedStates': len(capital)}, ensure_ascii=False))
