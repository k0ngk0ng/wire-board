import base64
from compression import zstd
import gzip
import hashlib
import json
from pathlib import Path
import struct
from collections import deque

ROOT = Path(__file__).resolve().parents[3]
BASE = ROOT / '.local/pal/verification/runtime-candidates/audio/development-health/hp-9999'
CORE = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
SSS = (ROOT / '.local/pal/original-98/sss.mkf').read_bytes()
SSS_HASH = hashlib.sha256(SSS).hexdigest()
assert SSS_HASH == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE
OFFSETS = struct.unpack_from('<6I', SSS)


def original(event):
    return struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(event-1)*32)


def script(pc):
    return struct.unpack_from('<4H', SSS, OFFSETS[4]+pc*8)


def digest(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}


def snapshot(path, event):
    state = json.loads(path.read_bytes())
    packed = base64.b64decode(state['native'], validate=True)
    raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
    assert raw[:8] == b'PALMEM01'
    size, memory_size = struct.unpack_from('<II', raw, 8)
    metadata = json.loads(raw[16:16+size])
    memory = raw[16+size:]
    assert len(memory) == memory_size <= 48*1024*1024 and metadata['buildID'] == CORE
    assert hashlib.sha256(memory+struct.pack('<I', metadata['stackPointer'])).hexdigest() == metadata['memoryID']
    needle = SSS[OFFSETS[0]+234*32:OFFSETS[0]+234*32+8]
    starts = []
    at = memory.find(needle)
    while at >= 0:
        start = at-234*32
        if start >= OFFSETS[0] and start+5077*32 <= len(memory) and memory[start-OFFSETS[0]:start] != SSS[:OFFSETS[0]]:
            if all(struct.unpack_from('<hHHhHHh9H', memory, start+(anchor-1)*32)[1:3] == original(anchor)[1:3] and struct.unpack_from('<hHHhHHh9H', memory, start+(anchor-1)*32)[8] == original(anchor)[8] for anchor in [235, 243, 261, 262, 264]):
                starts.append(start)
        at = memory.find(needle, at+1)
    assert len(starts) == 1, (str(path), starts)
    row = struct.unpack_from('<hHHhHHh9H', memory, starts[0]+(event-1)*32)
    return state['public'], row, digest(path)


def saved(prefix, label, event):
    return snapshot(BASE / f'{prefix}-{label}-state-1.json', event)


def selected(view, label):
    assert view['phase'] == 'menu'
    menu = view['menu']
    assert any(item['label'] == label and item['value'] == menu['selected'] and item['enabled'] for item in menu['items'])


def scene_events(scene):
    first = struct.unpack_from('<4H', SSS, OFFSETS[1]+(scene-1)*8)[3]
    end = struct.unpack_from('<4H', SSS, OFFSETS[1]+scene*8)[3]
    return [(event, original(event)) for event in range(first+1, end+1)]


def goals(event):
    row = original(event)
    assert row[7] == 1
    return [tuple(row[1:3])]+[(row[1]-dx, row[2]-dy) for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]]


def graph(scene, map_id):
    data = gzip.decompress((ROOT / f'.local/pal/visual/pal/v1/maps/{map_id}.bin.gz').read_bytes())
    assert len(data) == 65536
    entrances, exits = [], []
    for pc in range((OFFSETS[5]-OFFSETS[4])//8):
        if script(pc)[:2] == (0x59, scene):
            placement = script(pc+1)
            assert placement[0] == 0x46 and placement[3] <= 1
            entrances.append({'scenePC': pc, 'position': [placement[1]*32+placement[3]*16, placement[2]*16+placement[3]*8]})
    for event, row in scene_events(scene):
        if row[6] > 0 and row[7] >= 4 and row[8] == 0 and script(row[4])[0] == 0x59:
            exits.append({'event': event, 'entry': list(row[1:3]), 'radius': (row[7]-4)*32+16, 'script': row[4]})
    assert len(entrances) == len(exits) == 2
    seeds = [tuple(row['position']) for row in entrances]
    seen, queue = set(seeds), deque(seeds)
    while queue:
        x, y = queue.popleft()
        for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
            a, b = x+dx, y+dy
            if (a, b) in seen or a < 160 or a >= 1856 or b < 112 or b >= 2048:
                continue
            index = ((b//16*64+a//32)*2+a%32//16)*4
            if struct.unpack_from('<I', data, index)[0] & 8192:
                continue
            seen.add((a, b))
            queue.append((a, b))
    return seen, {'scene': scene, 'map': map_id, 'mapSHA256': hashlib.sha256(data).hexdigest(), 'entrances': entrances, 'exits': exits, 'optimisticWalkablePositions': len(seen), 'ignoresAllDynamicCollisions': True, 'evenAllowsWalkingAcrossSceneExitAreas': True, 'partyOffset': [160, 112]}


records, graphs = [], []
reference = ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md'
for scene, map_id, event, quote in [(190, 200, 3190, '2.女娲石（拿不到）'), (191, 159, 3212, '2.蜂王蜜（拿不到）')]:
    seen, plan = graph(scene, map_id)
    assert not any(at in seen for at in goals(event)) and quote in reference.read_text()
    before_path = BASE / f'peach-quest-v1-field-{scene}-state-1.json'
    after_path = BASE / f'regional-hidden-v14-{scene}-state-1.json'
    before_view, before, before_info = snapshot(before_path, event)
    after_view, after, after_info = snapshot(after_path, event)
    assert before[1:13] == after[1:13] == original(event)[1:13] and before[6] == 2 and before[8] == 10
    assert before_view['field']['offset'] == after_view['field']['offset'] == [160, 112]
    for exit_row in plan['exits']:
        for path in [before_path, after_path]:
            _, row, _ = snapshot(path, exit_row['event'])
            assert row[4] == exit_row['script'] and row[6] == 1 and row[7] == original(exit_row['event'])[7]
    result_path = BASE / f'regional-hidden-v14-{scene}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 14 and event not in {row['event'] for row in result['claimed']}
    graphs.append(plan)
    records.append({'event': event, 'scene': scene, 'map': map_id, 'position': list(original(event)[1:3]), 'classification': 'original-unreachable-preserved', 'actuallyObtained': False, 'normalPickupVerified': False, 'nativeRecordPreservedBefore': True, 'nativeRecordPreservedAfter': True, 'nativeEventBefore': list(before), 'nativeEventAfter': list(after), 'before': before_info, 'after': after_info, 'allSceneTransitions': len(plan['entrances']), 'reachableSearchStances': [], 'searchStances': [list(at) for at in goals(event)], 'referenceQuote': quote, 'otherSceneRewards': digest(result_path)})

movement_path = ROOT / '.local/pal/verification/forest-original-unreachable.json'
movement = json.loads(movement_path.read_bytes())
assert movement['passed'] and movement['nativeBuildID'] == CORE and not movement['nativeEdits']
assert movement['originalMovementReference']['actualPartyOffset'] == [160, 112]
report = {'passed': True, 'readyForProgress': True, 'scope': 'two original Peach reward records kept unclaimed, with explicit independent text, exact native before/after records and all incoming original scene placements under optimistic static map reachability even allowing exit-area bypass; dynamic actors ignored; no original edits or false pickups', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'graphs': graphs, 'originalMovementReference': digest(movement_path), 'reference': {'url': 'https://gamegene.cn/wiki/364', **digest(reference)}}
(ROOT / '.local/pal/verification/peach-original-unreachable.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'originalUnreachablePreserved': len(records), 'events': [row['event'] for row in records]}, ensure_ascii=False))


