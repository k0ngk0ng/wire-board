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


def event_rows(scene):
    first = struct.unpack_from('<4H', SSS, OFFSETS[1]+(scene-1)*8)[3]
    end = struct.unpack_from('<4H', SSS, OFFSETS[1]+scene*8)[3]
    return [(event, original(event)) for event in range(first+1, end+1)]


def footprints(entry, radius):
    for x in range(entry[0]//16*16-radius, entry[0]+radius+1, 16):
        for y in range(entry[1]//8*8-radius//2, entry[1]+radius//2+1, 8):
            if x >= 0 and y >= 0 and (x//16-y//8)%2 == 0 and abs(x-entry[0])+2*abs(y-entry[1]) < radius:
                yield x, y


def graph(starts_override=None, lift_events=None):
    ground_path = ROOT / '.local/pal/visual/pal/v1/maps/151.bin.gz'
    ground = gzip.decompress(ground_path.read_bytes())
    assert len(ground) == 65536
    incoming, climbs, lifts, edges = [], [], [], {}
    for pc in range((len(SSS)-OFFSETS[4])//8):
        row = script(pc)
        if row[:2] != (0x59, 193):
            continue
        placement = script(pc+1)
        assert placement[0] == 0x46 and placement[3] <= 1
        incoming.append({'pc': pc, 'position': [placement[1]*32+placement[3]*16, placement[2]*16+placement[3]*8]})
    assert len(incoming) == 2 and {row['pc'] for row in incoming} == {25611, 25965}
    for event, row in event_rows(193):
        if row[6] <= 0 or row[7] < 4 or row[4] == 0:
            continue
        last, supported = None, True
        for pc in range(row[4], row[4]+16):
            command = script(pc)
            if command[0] <= 3:
                break
            if command[0] in [0x7a, 0x7b]:
                last = command
            elif command[0] not in [0x46, 0x7f, 0x9, 0x6e]:
                supported = False
        if supported and last is not None:
            assert last[3] <= 1
            climbs.append({'event': event, 'script': row[4], 'entry': list(row[1:3]), 'radius': (row[7]-4)*32+16, 'destination': [last[1]*32+last[3]*16, last[2]*16+last[3]*8]})
        if row[8] == 501:
            for pc in range(row[4], row[4]+32):
                command = script(pc)
                if command[0] <= 3:
                    break
                if command[0] == 0x44:
                    assert command[3] <= 1
                    lifts.append({'event': event, 'script': row[4], 'entry': list(row[1:3]), 'radius': (row[7]-4)*32+16, 'destination': [command[1]*32+command[3]*16, command[2]*16+command[3]*8]})
    assert len(climbs) == 52 and {row['event'] for row in lifts} == {3299, 3300}
    enabled_lifts = lifts if lift_events is None else [row for row in lifts if row['event'] in lift_events]
    for row in climbs+enabled_lifts:
        for at in footprints(row['entry'], row['radius']):
            edges.setdefault(at, set()).add(tuple(row['destination']))
    forced = {at for row in climbs for at in footprints(row['entry'], row['radius'])}
    starts = [tuple(row['position']) for row in incoming] if starts_override is None else starts_override
    seen, queue = set(starts), deque(starts)
    while queue:
        x, y = queue.popleft()
        following = [] if (x, y) in forced else [(x+dx, y+dy) for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]]
        following.extend(edges.get((x, y), []))
        landings = edges.get((x, y), set())
        for a, b in following:
            if (a, b) in seen or a < 160 or a//32 >= 58 or b < 112 or b >= 2048:
                continue
            index = ((b//16*64+a//32)*2+a%32//16)*4
            if (a, b) not in landings and struct.unpack_from('<I', ground, index)[0] & 8192:
                continue
            seen.add((a, b))
            queue.append((a, b))
    return seen, {'map': 151, 'mapSHA256': hashlib.sha256(ground).hexdigest(), 'incoming': incoming, 'climbs': climbs, 'lifts': lifts, 'optimisticWalkablePositions': len(seen), 'ignoresAllDynamicCollisions': True, 'allowsAllLiftsRegardlessOfUsedState': True, 'allowsWalkingAcrossInactivePlantAreas': True, 'allowsAllOverlappingClimbDestinations': True, 'retainsOriginalMandatoryClimbTriggers': True}


def search_stances(event):
    row = original(event)
    assert row[7] == 1
    return [tuple(row[1:3])]+[(row[1]-x, row[2]-y) for x, y in [(-16, 8), (-16, -8), (16, -8), (16, 8)]]


records = []
seen, plan = graph()
for event in [3313, 3314, 3315, 3316, 3317]:
    assert any(at in seen for at in search_stances(event))
event = 3318
goals = search_stances(event)
assert not any(at in seen for at in goals)
source = BASE / 'sacred-tree-monkey-3302-fed-state-1.json'
final = BASE / 'regional-hidden-v13-193-post-lift-state-1.json'
before_view, before, before_info = snapshot(source, event)
after_view, after, after_info = snapshot(final, event)
assert before[1:12] == after[1:12] == original(event)[1:12]
assert before[4] == 38189 and before[6] == 2 and before[8] == 70
assert script(38194) == (0x1f, 107, 0, 0)
result_paths = [BASE / f'regional-hidden-v13-193-{phase}-result-1.json' for phase in ['pre-lift', 'post-lift']]
results = [json.loads(path.read_bytes()) for path in result_paths]
assert all(result['passed'] for result in results)
claimed = [row for result in results for row in result['claimed']]
assert len(claimed) == 5 and {row['event'] for row in claimed} == {3313, 3314, 3315, 3316, 3317}
assert event not in {row['event'] for row in claimed}
other_source = BASE / 'sacred-tree-monkey-3301-fed-state-1.json'
other_final = BASE / 'regional-hidden-v13-193-pre-lift-state-1.json'
_, other_before, other_before_info = snapshot(other_source, event)
_, other_after, other_after_info = snapshot(other_final, event)
assert other_before[1:12] == other_after[1:12] == original(event)[1:12]
climb_preservation = []
for climb in plan['climbs']:
    native_rows = []
    for path in [source, final, other_source, other_final]:
        _, row, _ = snapshot(path, climb['event'])
        assert row[4] == climb['script'] and row[5] == 0 and row[6] == 1 and row[7] == original(climb['event'])[7]
        native_rows.append([row[4], row[5], row[6], row[7]])
    climb_preservation.append({'event': climb['event'], 'nativeTriggerStates': native_rows})
reference = ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md'
quote = '7.灵山仙芝（拿不到）'
assert quote in reference.read_text()
normal_fungus = next(row for row in claimed if row['event'] == 3314)
assert normal_fungus['grants'] == [{'PC': 38194, 'Item': 107, 'Amount': 1, 'Cash': 0, 'Name': '灵山仙芝'}] and normal_fungus['notification'] and normal_fungus['repeatSearches'] >= 3
record = {'event': event, 'scene': 193, 'map': 151, 'position': list(original(event)[1:3]), 'item': 107, 'classification': 'original-unreachable-preserved', 'normalPickupVerified': False, 'actuallyObtained': False, 'allSceneTransitions': len(plan['incoming']), 'reachableSearchStances': [], 'searchedWorldPositions': [list(at) for at in goals], 'nativeRecordPreservedBefore': True, 'nativeRecordPreservedAfter': True, 'before': before_info, 'after': after_info, 'otherBranchBefore': other_before_info, 'otherBranchAfter': other_after_info, 'nativeEventBefore': list(before), 'nativeEventAfter': list(after), 'independentReferenceQuote': quote, 'otherSameItemEventActuallyClaimed': 3314}
records.append(record)
scene_source = Path('/Users/jason/Sources/github.com/sdlpal/sdlpal/scene.c')
play_source = Path('/Users/jason/Sources/github.com/sdlpal/sdlpal/play.c')
range_check = 'x < blockX || x >= 2048/32 - blockX -1 || y < blockY || y >= 2048 - blockY -1'
walking_call = 'PAL_CheckObstacleWithRange(PAL_XY(xTarget, yTarget), TRUE, 0, TRUE)'
assert range_check in scene_source.read_text() and walking_call in scene_source.read_text()
touch_check = '(p->wTriggerMode - kTriggerTouchNear) * 32 + 16'
assert touch_check in play_source.read_text()
boundary_view, _, boundary_info = saved('regional-hidden-v13-193-pre-lift', 'reward-3314-collected', 3314)
assert boundary_view['field']['offset'] == [160, 112]
boundary_cells = []
for index, flags in enumerate(base64.b64decode(boundary_view['field']['cells'])[:289]):
    u, z = index%17-8, index//17-8
    x, y = boundary_view['x']+(u-z)*16, boundary_view['y']+(u+z)*8
    if 0 <= x < 160 and 112 <= y < 2048:
        assert flags & 1
        boundary_cells.append([x, y, flags])
assert boundary_cells
movement_reference = {'referenceSceneSourceSHA256': hashlib.sha256(scene_source.read_bytes()).hexdigest(), 'referencePlaySourceSHA256': hashlib.sha256(play_source.read_bytes()).hexdigest(), 'rangeCheck': range_check, 'playerWalkingCall': walking_call, 'mandatoryTouchCheck': touch_check, 'actualPartyOffset': [160, 112], 'actualNativeBlockedLeftBoundaryCells': boundary_cells, 'actualNativeBoundaryState': boundary_info, 'scope': 'read-only reference source plus actual current native collision cells; the optimistic map retains the original walking range, not an invented collection restriction'}
report = {'passed': True, 'readyForProgress': True, 'scope': 'original unreachable east-forest fungus preserved; explicit independent text plus optimistic original map graph with both incoming scene placements, all 52 persistent mandatory climbs and both optional plant rides regardless of consumed state, walking through inactive plant areas and all overlapping destinations; actual other same-item event verified, original native record unchanged; no claimed pickup or native edits', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'graph': plan, 'originalMovementReference': movement_reference, 'persistentClimbNativeChecks': climb_preservation, 'records': records, 'actualOtherRewards': [digest(path) for path in result_paths], 'reference': {'url': 'https://gamegene.cn/wiki/364', 'quote': quote, **digest(reference)}}
(ROOT / '.local/pal/verification/forest-original-unreachable.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'originalUnreachablePreserved': 1, 'ordinaryPickupsInSameScene': 5, 'originalClimbs': 52, 'originalPlantRides': 2, 'allIncomingScenes': 2, 'optimisticPositions': len(seen)}, ensure_ascii=False))


