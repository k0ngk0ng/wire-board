import base64
from compression import zstd
import gzip
import hashlib
import json
from pathlib import Path
import struct

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


def all_text(view):
    chars = []
    if view.get('raster'):
        raster, seen = view['raster'], set()
        def visit(index):
            if index <= 0 or index > len(raster['nodes']) or index in seen:
                return
            seen.add(index)
            node = raster['nodes'][index-1]
            visit(node[1])
            visit(node[2])
            if node[0] == 2:
                chars.append(chr(node[5]))
        visit(raster['root'])
    else:
        chars.extend(chr(op['id']) for op in view.get('overlay') or [] if op['kind'] == 0)
    return ''.join(chars)+(view.get('dialogue') or {}).get('text', '')


def metadata(path):
    state = json.loads(path.read_bytes())
    packed = base64.b64decode(state['native'], validate=True)
    raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
    size = struct.unpack_from('<I', raw, 8)[0]
    return json.loads(raw[16:16+size])


from collections import deque

VERIFY = ROOT / '.local/pal/verification'
reference = ROOT / '.local/pal/hidden-items-research/final-maze-text-guide.md'
quotes = {4846: '4.金蚕王（拿不到）5.孟婆汤（拿不到）', 4847: '4.金蚕王（拿不到）5.孟婆汤（拿不到）', 4917: '7.三尸蛊（同一处的幻蛊拿不到）'}
assert all(quote in reference.read_text() for quote in quotes.values())
explicit_writes = []
for pc in range((OFFSETS[5]-OFFSETS[4])//8):
    op = script(pc)
    if op[0] in [0x49, 0x40, 0x24, 0x25, 0x12] and op[1] in [4846, 4847, 4906, 4917]:
        explicit_writes.append({'PC': pc, 'instruction': list(op)})
assert explicit_writes == []


def optimistic_graph(scene, map_id):
    data = gzip.decompress((ROOT / f'.local/pal/visual/pal/v1/maps/{map_id}.bin.gz').read_bytes())
    assert len(data) == 65536
    free = {(x*32+half*16, y*16+half*8) for y in range(128) for x in range(64) for half in range(2) if not struct.unpack_from('<I', data, ((y*64+x)*2+half)*4)[0] & 8192}
    entrances = []
    for pc in range((OFFSETS[5]-OFFSETS[4])//8-1):
        if script(pc)[:2] != (0x59, scene):
            continue
        placement = script(pc+1)
        assert placement[0] == 0x46 and placement[3] in [0, 1]
        owners = [event for event in range(1, 5078) if original(event)[4] == pc]
        assert owners
        entrances.append({'script': pc, 'events': owners, 'position': [placement[1]*32+placement[3]*16, placement[2]*16+placement[3]*8]})
    seen = {tuple(row['position']) for row in entrances}
    queue = deque(seen)
    while queue:
        x, y = queue.popleft()
        for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
            point = (x+dx, y+dy)
            if point in free and point not in seen:
                seen.add(point)
                queue.append(point)
    return seen, {'scene': scene, 'map': map_id, 'mapSHA256': hashlib.sha256(data).hexdigest(), 'allRawIncomingPlacements': entrances, 'optimisticWalkablePositions': len(seen), 'ignoresAllDynamicCollisions': True, 'allowsAllExitTriggerAreas': True, 'evenAllowsMapPaddingOutsidePlayerViewportBounds': True}


reachable, graph = optimistic_graph(287, 204)
assert len(graph['allRawIncomingPlacements']) == 5 and len(reachable) == 1019
records = []
for event in [4846, 4847]:
    row = original(event)
    x, y = row[1:3]
    stances = [(x, y), (x-16, y-8), (x-16, y+8), (x+16, y-8), (x+16, y+8)]
    assert not set(stances) & reachable
    stages = []
    for label, path in [('before', BASE / 'finale-field-286-step-6-state-1.json'), ('after', BASE / 'regional-hidden-v19-287-state-1.json')]:
        view, current, info = snapshot(path, event)
        assert current[1:13] == row[1:13] and current[6] == 2
        stages.append({'label': label, 'state': info, 'nativeEvent': list(current), 'scene': view['scene']})
    records.append({'event': event, 'scene': 287, 'map': 204, 'position': [x, y], 'classification': 'original-unreachable-preserved', 'actuallyObtained': False, 'normalPickupVerified': False, 'nativeRecordPreservedBefore': True, 'nativeRecordPreservedAfter': True, 'reachableSearchStances': [], 'searchStances': [list(point) for point in stances], 'referenceQuote': quotes[event], 'nativeStages': stages})
first, shadow = original(4906), original(4917)
assert first[1:3] == shadow[1:3] == (544, 1616) and first[6:9] == shadow[6:9] == (2, 1, 10)
assert first[4] == 37899 and shadow[4] == 37989 and first[4] != shadow[4]
source = ROOT / 'games/pal/native/upstream/play.c'
text = source.read_text()
assert 'k < gpGlobals->g.rgScene[gpGlobals->wNumScene].wEventObjectIndex; k++)' in text and 'p->wTriggerScript = PAL_RunTriggerScript(p->wTriggerScript, k + 1);' in text
assert 'return; // don\'t go further' in text
stages = []
for label, path in [('before', BASE / 'finale-field-289-step-4-state-1.json'), ('after', BASE / 'regional-hidden-v19-289-state-1.json')]:
    view, a, info = snapshot(path, 4906)
    _, b, _ = snapshot(path, 4917)
    assert a[1:3] == b[1:3] == first[1:3] and a[6:9] == b[6:9] == (2, 1, 10) and b[1:13] == shadow[1:13]
    assert a[4] == (37899 if label == 'before' else 36509) and a[11] == (0 if label == 'before' else 1)
    stages.append({'label': label, 'state': info, 'firstNativeEvent': list(a), 'nativeEvent': list(b), 'scene': view['scene']})
result_path = BASE / 'regional-hidden-v19-289-result-1.json'
result = json.loads(result_path.read_bytes())
claimed = next(row for row in result['claimed'] if row['event'] == 4906)
assert result['passed'] and claimed['repeatSearches'] == 3 and claimed['notification'] and all(row['event'] != 4917 for row in result['claimed'])
records.append({'event': 4917, 'scene': 289, 'map': 206, 'position': [544, 1616], 'classification': 'original-unreachable-preserved', 'actuallyObtained': False, 'normalPickupVerified': False, 'nativeRecordPreservedBefore': True, 'nativeRecordPreservedAfter': True, 'reachableSearchStances': [], 'reason': 'earlier same-position one-time chest remains searchable with original empty-chest continuation; original ascending search returns before the later event', 'firstEvent': 4906, 'referenceQuote': quotes[4917], 'nativeStages': stages, 'actualFirstChestResult': digest(result_path)})
unverified = []
reachable_back, back_graph = optimistic_graph(294, 192)
assert len(back_graph['allRawIncomingPlacements']) == 2 and len(reachable_back) == 2103
for event in [5071, 5072, 5073, 5074, 5077]:
    row = original(event)
    x, y = row[1:3]
    stances = [(x, y), (x-16, y-8), (x-16, y+8), (x+16, y-8), (x+16, y+8)]
    assert not set(stances) & reachable_back
    stages = []
    for label, path in [('before', BASE / 'finale-field-294-step-2-state-1.json'), ('after', BASE / 'regional-hidden-v19-294-state-1.json')]:
        view, current, info = snapshot(path, event)
        assert current[1:13] == row[1:13] and current[6] == 1
        stages.append({'label': label, 'state': info, 'nativeEvent': list(current)})
    unverified.append({'event': event, 'scene': 294, 'position': [x, y], 'readyForProgress': False, 'actuallyObtained': False, 'normalPickupVerified': False, 'reachableSearchStances': [], 'nativeStages': stages, 'reason': 'all raw entrances optimistic original map graph disconnected; independent specific classification evidence remains incomplete'})
report = {'passed': True, 'readyForProgress': True, 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'scope': 'three independently guide-confirmed original unclaimable chests with all-entrance optimistic map and current native state or actual earlier same-coordinate chest witness; five additional back-abyss records stay unprocessed', 'reference': {'url': 'https://gamegene.cn/wiki/366', **digest(reference)}, 'originalSearchReference': digest(source), 'explicitStateModePositionOrScriptWrites': explicit_writes, 'records': records, 'graph': graph, 'unverifiedRecords': unverified, 'unverifiedBackAbyssGraph': back_graph}
(VERIFY / 'final-maze-original-unreachable.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'originalUnreachablePreserved': len(records), 'stillUnverified': len(unverified)}, ensure_ascii=False))
