import base64
from compression import zstd
import gzip
import hashlib
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
GLOBAL = ROOT / '.local/pal/verification/runtime-candidates/audio/development-health/hp-9999'
BASE = GLOBAL / 'scopes/tower-remainder-v1'
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


def native_events(path):
    state = json.loads(path.read_bytes())
    packed = base64.b64decode(state['native'], validate=True)
    raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
    size = struct.unpack_from('<I', raw, 8)[0]
    meta = json.loads(raw[16:16+size])
    memory = raw[16+size:]
    assert meta['buildID'] == CORE and hashlib.sha256(memory+struct.pack('<I', meta['stackPointer'])).hexdigest() == meta['memoryID']
    needle = SSS[OFFSETS[0]+234*32:OFFSETS[0]+234*32+8]
    starts, at = [], memory.find(needle)
    while at >= 0:
        start = at-234*32
        if start >= OFFSETS[0] and start+5077*32 <= len(memory) and memory[start-OFFSETS[0]:start] != SSS[:OFFSETS[0]] and all(struct.unpack_from('<hHHhHHh9H', memory, start+(anchor-1)*32)[1:3] == original(anchor)[1:3] for anchor in [235, 243, 261, 262, 264]):
            starts.append(start)
        at = memory.find(needle, at+1)
    assert len(starts) == 1
    return {event: struct.unpack_from('<hHHhHHh9H', memory, starts[0]+(event-1)*32) for event in range(1, 5078)}


from collections import deque

VERIFY = ROOT / '.local/pal/verification'
plan_path = VERIFY / 'tower-remainder-plan.json'
plan = json.loads(plan_path.read_bytes())
case = plan['case']
assert plan['passed'] and plan['diagnosticOnly'] and not plan['originalDataEdits'] and len(case['events']) == 11 and case['unplannedEvents'] == [2763]
path = BASE / 'regional-hidden-v20-167-result-1.json'
result = json.loads(path.read_bytes())
assert result['passed'] and result['verificationVersion'] == 20 and result['scene'] == 167 and result['sourceCheckpoint'] == case['sourceCheckpoint'] and result['sourceSHA256'] == case['sourceSHA256']
assert [row['event'] for row in result['claimed']] == case['events'] and result['routePlanSHA256'] == digest(plan_path)['sha256']
catalog = {row['Event']: row for row in json.loads((VERIFY / 'hidden-search-candidates.json').read_bytes())['candidates']}
records = []
for reward in result['claimed']:
    event, stages = reward['event'], []
    assert reward['grants'] == catalog[event]['Grants'] and reward['position'] == [catalog[event]['X'], catalog[event]['Y']]
    for label in ['unpicked', 'notice', 'collected', 'repeated']:
        view, row, info = saved('regional-hidden-v20-167', f'reward-{event}-{label}', event)
        assert view['scene'] == 167 and [member['role'] for member in view['party']] == [0, 2] and row[1:3] == original(event)[1:3] and row[6:10] == original(event)[6:10]
        assert all(member['controller'] == 0 for member in view['party'])
        assert row[4] == original(event)[4] if label in ['unpicked', 'notice'] else row[4] == 36509
        assert row[10]*row[9]+row[11] == (0 if label == 'unpicked' else 1)
        if label == 'notice':
            assert all(message in all_text(view) for message in catalog[event]['Messages'])
        stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
    _, final, final_info = snapshot(BASE / 'regional-hidden-v20-167-state-1.json', event)
    assert final[4] == 36509 and final[6] == 2 and final[10]*final[9]+final[11] == 1
    records.append({'event': event, 'scene': 167, 'position': list(original(event)[1:3]), 'stages': stages, 'final': {'state': final_info, 'nativeEvent': list(final)}, 'result': digest(path)})
before_events = native_events(BASE / 'tower-upper-step-3-state-1.json')
after_events = native_events(BASE / 'regional-hidden-v20-167-state-1.json')
retained = [event for event, row in before_events.items() if row[8] in [10, 389] and row[4] == 36509 and row[6] == 2]
for event in retained:
    a, b = before_events[event], after_events[event]
    assert a[1:5] == b[1:5] and a[6:13] == b[6:13]

empty_path = BASE / 'tower-remainder-empty-v1-result-1.json'
empty = json.loads(empty_path.read_bytes())
assert empty['passed'] and empty['event'] == 2764 and empty['scene'] == 167 and empty['originalItemID'] == 0 and empty['itemBefore'] == empty['itemAfter'] and empty['repeatSearches'] == 3
assert original(2764)[4] == 37479 and script(37482) == (0x1f, 0, 0, 0)
empty_stages = []
for label in ['unpicked', 'notice', 'collected', 'repeated']:
    view, row, info = saved('tower-remainder-empty-v1', label, 2764)
    assert view['scene'] == 167 and row[1:3] == (336, 616) and row[6] == 2 and row[8] == 10
    assert row[4] == (37479 if label in ['unpicked', 'notice'] else 36509) and row[11] == (0 if label == 'unpicked' else 1)
    if label == 'notice':
        assert '获得紫青玉蓉膏' in all_text(view)
    empty_stages.append({'label': label, 'nativeEvent': list(row), 'state': info})

stories = []
for event, labels, parent_name, item, item_id, pc in [
    (2677, ['shu-tower-step-5', 'shu-tower-jade-reading', 'shu-tower-step-6'], 'campaign-shu-tower-1.json', '玉佛珠', 274, 22938),
    (2408, ['tower-upper-step-4', 'tower-upper-sword-reading', 'tower-upper-七星剑-verified', 'tower-upper-七星剑-repeated'], 'campaign-tower-upper-1.json', '七星剑', 186, 23345),
]:
    parent_path = GLOBAL / parent_name
    parent = json.loads(parent_path.read_bytes())
    assert parent['passed'] and not parent['resumedTestRoute'] and parent['candidate'] == 'audio' and parent['developmentProfile'] == 'hp-9999' and script(pc) == (0x1f, item_id, 0, 0)
    stages = []
    for label in labels:
        path = GLOBAL / f'{label}-state-1.json'
        view, row, info = snapshot(path, event)
        assert row[1:3] == original(event)[1:3] and row[8] == original(event)[8]
        if event == 2677:
            assert row[6] == (1 if label == labels[0] else 0)
            if label == 'shu-tower-jade-reading':
                assert '李逍遥收起玉佛珠' in all_text(view)
        else:
            assert row[6] == 2 and row[4] == (23340 if label in labels[:2] else 23499)
            if label == 'tower-upper-sword-reading':
                assert '七星剑' in all_text(view)
        stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
    repeat_path = BASE / f'shu-reward-repeat-v1-{event}-result-1.json'
    repeat = json.loads(repeat_path.read_bytes())
    assert repeat['passed'] and repeat['existingEarnedBranch'] and repeat['repeatSearches'] == 3 and repeat['inventoryBefore'][item] == repeat['inventoryAfter'][item] == 1
    for label in ['before', 'repeat-1', 'repeat-2', 'repeat-3']:
        view, row, info = saved(f'shu-reward-repeat-v1-{event}', label, event)
        assert view['phase'] == 'field' and row[6] == (0 if event == 2677 else 2) and row[4] == (22921 if event == 2677 else 23499)
        stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
    if event == 2677:
        assert parent['checks']['InventoryBefore'].get(item, 0) == 0 and parent['checks']['InventoryAfterGifts'][item] == 1 and parent['finalKeyItems'][item] == 1
    else:
        assert parent['checks']['InventoryBefore'].get(item, 0) == 0 and parent['checks']['InventoryAfterSword'][item] == 1 and parent['checks']['RepeatSwordSearches'] == 3 and repeat['inventoryAfter']['天师道袍'] == 1
    stories.append({'event': event, 'item': item, 'grantPC': pc, 'originalItemID': item_id, 'reusedCompleteStory': digest(parent_path), 'newRepeatResult': digest(repeat_path), 'nativeStages': stages})
assert script(23385) == (7, 163, 41075, 0) and script(42420) == (0x1f, 216, 0, 0)
enemy = struct.unpack_from('<7H', SSS, OFFSETS[2]+494*14)
assert enemy[3] == 42420
battle_path = GLOBAL / 'tower-upper-battle-7-state-1.json'
battle, _, battle_info = snapshot(battle_path, 2408)
assert 494 in [row['object'] for row in battle['battle']['enemies']]
stories[1]['originalBattleFollowup'] = {'formation': 163, 'enemy': 494, 'endScript': 42420, 'item': 216, 'name': '天师道袍', 'amount': 1, 'battleState': battle_info}
alias = json.loads((VERIFY / 'tower-remainder-source/seven-star-ascii-alias.json').read_bytes())
origin, alias_path = (ROOT / alias['original']).resolve(), (ROOT / alias['alias']).resolve()
assert origin.is_absolute() and origin.is_relative_to(ROOT) and alias_path.is_relative_to(ROOT) and origin.read_bytes() == alias_path.read_bytes() and digest(origin)['sha256'] == alias['sha256']

data = gzip.decompress((ROOT / '.local/pal/visual/pal/v1/maps/143.bin.gz').read_bytes())
free = {(x*32+half*16, y*16+half*8) for y in range(128) for x in range(64) for half in range(2) if not struct.unpack_from('<I', data, ((y*64+x)*2+half)*4)[0] & 8192}
entrances = []
for pc in range((OFFSETS[5]-OFFSETS[4])//8-1):
    if script(pc)[:2] == (0x59, 167):
        p = script(pc+1)
        assert p[0] == 0x46
        entrances.append({'script': pc, 'position': [p[1]*32+p[3]*16, p[2]*16+p[3]*8]})
assert entrances == [{'script': 22413, 'position': [1024, 720]}]
seen, queue = {(1024, 720)}, deque([(1024, 720)])
while queue:
    x, y = queue.popleft()
    for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
        point = (x+dx, y+dy)
        if point in free and point not in seen:
            seen.add(point)
            queue.append(point)
assert len(seen) == 5726
x, y = original(2763)[1:3]
stances = [(x, y), (x-16, y-8), (x-16, y+8), (x+16, y-8), (x+16, y+8)]
assert not set(stances) & seen
pending_stages = []
for label, path in [('before', BASE / 'tower-upper-step-3-state-1.json'), ('after', BASE / 'regional-hidden-v20-167-state-1.json')]:
    view, row, info = snapshot(path, 2763)
    assert row[1:13] == original(2763)[1:13] and row[6] == 2
    pending_stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
reference = ROOT / '.local/pal/hidden-items-research/tower-current-text-guide.md'
assert all(quote in reference.read_text() for quote in ['锁妖塔第七层', '紫青玉蓉膏（伪）', '打败剑老头，得七星剑、天师道袍'])
report = {'passed': True, 'nativeBuildID': CORE, 'nativeEdits': False, 'sssSHA256': SSS_HASH, 'scope': '11 new actual seventh-floor chests and original false jade chest, prior same-core complete jade bead and seven-star/robe story revalidated plus three new actual repeated searches each; remaining elixir chest stays unclassified and pending', 'records': records, 'retainedPriorChests': retained, 'emptyChest': {'result': digest(empty_path), 'nativeStages': empty_stages}, 'storyRewards': stories, 'sourceAlias': {'original': digest(origin), 'alias': digest(alias_path), 'nativeIdentityEdits': False}, 'pendingOriginalEvent': {'event': 2763, 'scene': 167, 'map': 143, 'position': [x, y], 'actuallyObtained': False, 'readyForProgress': False, 'reachableSearchStances': [], 'allRawIncomingPlacements': entrances, 'optimisticWalkablePositions': len(seen), 'ignoresAllDynamicCollisions': True, 'allowsAllExitTriggerAreas': True, 'evenAllowsMapPaddingOutsidePlayerViewportBounds': True, 'mapSHA256': hashlib.sha256(data).hexdigest(), 'nativeStages': pending_stages}, 'reference': {'url': 'https://gamegene.cn/wiki/363', **digest(reference)}}
(VERIFY / 'tower-remainder-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'newChestPickups': len(records), 'reusedStoryRewards': len(stories), 'falseChestAudited': 1, 'stillPending': 1}, ensure_ascii=False))
