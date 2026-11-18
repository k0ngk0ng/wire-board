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


VERIFY = ROOT / '.local/pal/verification'
reference_path = ROOT / '.local/pal/hidden-items-research/past-trial-text-guide.md'
quote = '9.隐蛊（拿不到）'
assert quote in reference_path.read_text()
target = 4000
position = tuple(original(target)[1:3])
assert position == (1040, 1448) and original(target)[6:9] == (2, 1, 10)
shadow_pairs = [(3949, 3954), (3950, 3955)]
shadows = []
for first, following in shadow_pairs:
    a, b = original(first), original(following)
    assert a[0] == b[0] == 0 and a[1:3] == b[1:3] and a[5] == b[5] == 0 and a[6:9] == b[6:9] == (1, 4, 0)
    assert first < following and script(a[4])[0] == script(b[4])[0] == 0x59 and script(a[4])[1] == script(b[4])[1] == 227
    shadows.append({'firstEvent': first, 'shadowedEvent': following, 'position': list(a[1:3]), 'firstScript': a[4], 'shadowedScript': b[4]})
state_writes = []
for pc in range((OFFSETS[5]-OFFSETS[4])//8):
    op = script(pc)
    if op[0] in [0x49, 0x40, 0x24] and op[1] in [3949, 3950, 3954, 3955]:
        state_writes.append({'PC': pc, 'instruction': list(op)})
assert state_writes == []
entrances = []
for pc in range((OFFSETS[5]-OFFSETS[4])//8):
    if script(pc)[:2] != (0x59, 227):
        continue
    placement = script(pc+1)
    assert placement[0] == 0x46 and placement[3] in [0, 1]
    matches = [event for event in range(1, 5078) if original(event)[4] == pc]
    assert len(matches) == 1
    event = matches[0]
    entrances.append({'event': event, 'script': pc, 'position': [placement[1]*32+placement[3]*16, placement[2]*16+placement[3]*8], 'shadowedBy': 3949 if event == 3954 else 3950 if event == 3955 else None})
assert len(entrances) == 28
map_path = ROOT / '.local/pal/visual/pal/v1/maps/224.bin.gz'
data = gzip.decompress(map_path.read_bytes())
assert len(data) == 65536
free = set()
for y in range(128):
    for x in range(64):
        for half in range(2):
            at = (x*32+half*16, y*16+half*8)
            if 160 <= at[0] < 1856 and 112 <= at[1] < 2048 and not struct.unpack_from('<I', data, ((y*64+x)*2+half)*4)[0] & 8192:
                free.add(at)


def reachable(including_shadowed):
    seen = {tuple(row['position']) for row in entrances if including_shadowed or row['shadowedBy'] is None}
    queue = deque(seen)
    while queue:
        x, y = queue.popleft()
        for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
            at = (x+dx, y+dy)
            if at in free and at not in seen:
                seen.add(at)
                queue.append(at)
    return seen


goals = [position]+[(position[0]+dx, position[1]+dy) for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]]
actual_graph, impossible_graph = reachable(False), reachable(True)
assert len(actual_graph) == 2733 and len(impossible_graph) == 2853 and not any(at in actual_graph for at in goals) and all(at in impossible_graph for at in goals)
native_stages = []
for label, path in [('before', BASE / 'sacred-medicine-trial-entry-state-1.json'), ('after', BASE / 'regional-hidden-v18-227-state-1.json')]:
    view, row, info = snapshot(path, target)
    assert row[1:13] == original(target)[1:13] and row[6] == 2 and row[8] == 10
    native_stages.append({'label': label, 'state': info, 'nativeEvent': list(row), 'scene': view['scene']})
    for first, following in shadow_pairs:
        for event in [first, following]:
            _, current, _ = snapshot(path, event)
            assert current[0] == 0 and current[1:9] == original(event)[1:9]

reference_source = Path('/Users/jason/Sources/github.com/sdlpal/sdlpal/play.c')
reference_bytes = reference_source.read_bytes()
reference_text = reference_bytes.decode()
ascending = 'wEventObjectID <= gpGlobals->g.rgScene[gpGlobals->wNumScene].wEventObjectIndex;'
run = 'p->wTriggerScript = PAL_RunTriggerScript(p->wTriggerScript, wEventObjectID);'
assert ascending in reference_text and 'wEventObjectID++)' in reference_text and run in reference_text
tail = reference_text[reference_text.index(run):]
assert tail.index('if (gpGlobals->fEnteringScene)') < tail.index('return;')
order_reference = {'sourcePath': str(reference_source), 'sha256': hashlib.sha256(reference_bytes).hexdigest(), 'ascendingEventLoop': ascending, 'runTrigger': run, 'sceneSwitchReturnsBeforeLaterEvents': True}
witness_path = VERIFY / 'trial-deep-priority-result.json'
witness = json.loads(witness_path.read_bytes())
assert witness['passed'] and witness['nativeBuildID'] == CORE and not witness['nativeEdits']
assert witness['firstEvent'] == 3949 and witness['shadowedEvent'] == 3954 and witness['actualDestinationScene'] == 227 and witness['actualDestination'] == [1488, 1272]
priority_native = []
for label, expected_scene in [('before-first-overlap', 226), ('first-overlap-arrived', 227)]:
    view, first, info = saved('trial-priority-v1', label, 3949)
    _, following, _ = saved('trial-priority-v1', label, 3954)
    assert view['scene'] == expected_scene and first[0] == following[0] == 0 and first[1:9] == original(3949)[1:9] and following[1:9] == original(3954)[1:9]
    if expected_scene == 227:
        assert [view['x'], view['y']] == [1488, 1272]
    priority_native.append({'label': label, 'state': info, 'firstEvent': list(first), 'shadowedEvent': list(following), 'scene': view['scene'], 'position': [view['x'], view['y']]})
report = {'passed': True, 'readyForProgress': True, 'scope': 'one original lower teleport-layer invisibility-worm chest left unclaimed; explicit independent guide, both original overlapping portal pairs and first-trigger/return semantics, actual same-core first portal witness, all 26 real incoming placements under an optimistic map ignoring every dynamic collision and all further teleport triggers; two shadowed impossible entries separately shown to connect the otherwise isolated region; original records unchanged', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'reference': {'url': 'https://gamegene.cn/wiki/365', 'quote': quote, **digest(reference_path)}, 'originalTriggerOrderReference': order_reference, 'priorityWitness': digest(witness_path), 'records': [{'event': target, 'scene': 227, 'map': 224, 'position': list(position), 'classification': 'original-unreachable-preserved', 'actuallyObtained': False, 'normalPickupVerified': False, 'nativeRecordPreservedBefore': True, 'nativeRecordPreservedAfter': True, 'nativeStages': native_stages, 'reachableSearchStances': [], 'searchStances': [list(at) for at in goals], 'allRawIncomingPlacements': 28, 'allActuallyTriggerableIncomingPlacements': 26, 'referenceQuote': quote}], 'shadowedPortals': shadows, 'explicitStateModeAutoScriptWrites': state_writes, 'entrances': entrances, 'graph': {'mapSHA256': hashlib.sha256(data).hexdigest(), 'optimisticWalkablePositions': len(actual_graph), 'impossibleShadowedEntranceWalkablePositions': len(impossible_graph), 'ignoresAllDynamicCollisions': True, 'evenAllowsWalkingAcrossAllTeleportTriggerAreas': True, 'partyOffset': [160, 112], 'hypotheticalShadowedSearchStances': [list(at) for at in goals if at in impossible_graph]}}
report['priorityNativeChecks'] = priority_native
(VERIFY / 'trial-deep-original-unreachable.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'originalUnreachablePreserved': 1, 'event': target, 'triggerableEntrances': 26, 'optimisticWalkablePositions': 2733}))

