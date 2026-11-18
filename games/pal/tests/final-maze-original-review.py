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


VERIFY = ROOT / '.local/pal/verification'
plan_path = VERIFY / 'final-maze-rewards-plan.json'
plan = json.loads(plan_path.read_bytes())
assert plan['passed'] and plan['diagnosticOnly'] and not plan['originalDataEdits'] and plan['sssSHA256'] == SSS_HASH
assert sum(len(case['events']) for case in plan['cases']) == 48
catalog = {row['Event']: row for row in json.loads((VERIFY / 'hidden-search-candidates.json').read_bytes())['candidates']}
records, routes, drops, appearances, retained = [], [], [], [], []
for case in plan['cases']:
    scene = case['scene']
    prefix = f'regional-hidden-v19-{scene}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 19 and result['authorityFixture'] and result['scene'] == scene and result['routePlanSHA256'] == digest(plan_path)['sha256']
    assert result['sourceCheckpoint'] == case['sourceCheckpoint'] and result['sourceSHA256'] == case['sourceSHA256']
    assert [row['event'] for row in result['claimed']] == case['events'] and [row['role'] for row in result['party']] == [1, 0, 4]
    for reward in result['claimed']:
        event, point = reward['event'], catalog[reward['event']]
        assert reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] == 3 and reward['position'] == [point['X'], point['Y']]
        stages = []
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            view, row, info = saved(prefix, f'reward-{event}-{label}', event)
            assert view['scene'] == scene and row[1:3] == original(event)[1:3] and row[7:10] == original(event)[7:10]
            assert [member['role'] for member in view['party']] == [1, 0, 4] and all(member['controller'] == 0 for member in view['party'])
            if label == 'unpicked':
                assert row[4] == original(event)[4] and row[6] == original(event)[6]
            elif label in ['collected', 'repeated']:
                if point['Sprite']:
                    assert row[4] == 36509 and row[6] == 2 and row[10]*row[9]+row[11] == 1
                else:
                    assert row[6] == 0
            if label == 'notice':
                assert point['Messages'][0] in all_text(view)
            stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
        _, final_row, final_info = snapshot(BASE / f'{prefix}-state-1.json', event)
        assert final_row[1:3] == original(event)[1:3] and final_row[8] == original(event)[8]
        assert final_row[4] == 36509 and final_row[6] == 2 and final_row[10]*final_row[9]+final_row[11] == 1 if point['Sprite'] else final_row[6] == 0
        records.append({'scene': scene, 'event': event, 'stages': stages, 'finalCaseState': {'state': final_info, 'nativeEvent': list(final_row)}, 'result': digest(result_path)})
    legs = [leg for step in case['steps'] for leg in step['portalsBefore']]
    assert len(result['trialRoute']) == len(legs) == case['portalSteps']
    for index, (actual, expected) in enumerate(zip(result['trialRoute'], legs), 1):
        assert actual['originalLeg'] == {**expected, 'chainedEvents': expected.get('chainedEvents'), 'teleportScript': 0}
        event = expected['event']
        view, _, info = saved(prefix, f'trial-route-{index}-door-{event}-arrived', event)
        assert view['scene'] == expected['destinationScene'] and [view['x'], view['y']] == actual['positionAfter']
        routes.append({'scene': scene, 'event': event, 'positionAfter': actual['positionAfter'], 'state': info})
    source_path = BASE / (case['sourceCheckpoint'][:-5]+'-1.json')
    source_events = native_events(source_path)
    final_events = native_events(BASE / f'{prefix}-state-1.json')
    old_chests = [event for event, row in source_events.items() if row[8] in [10, 389] and row[4] == 36509 and row[6] == 2]
    for event in old_chests:
        a, b = source_events[event], final_events[event]
        assert a[1:5] == b[1:5] and a[6:13] == b[6:13], (scene, event)
    retained.append({'scene': scene, 'source': digest(source_path), 'final': digest(BASE / f'{prefix}-state-1.json'), 'priorClaimedChests': old_chests})
    for event in range(1, 5078):
        if original(event)[8] not in [10, 389] or original(event)[4] == 0:
            continue
        if event not in [row['event'] for row in result['claimed']]:
            continue
        before, row, _ = snapshot(source_path, event)
        assert row[4] == original(event)[4] and row[6] == original(event)[6]
    observed = result.get('wormAppearances', [])
    for entry in observed:
        path = BASE / f"{prefix}-{entry['label']}-state-1.json"
        view, _, info = snapshot(path, case['events'][0])
        assert view['battle']['enemies'][entry['slot']]['object'] == entry['enemy'] and struct.unpack_from('<7H', SSS, OFFSETS[2]+entry['enemy']*14)[3] == 42828
        appearances.append({'scene': scene, **entry, 'state': info})
    for grant in result['battleItemGrants']:
        battle, enemy = grant['battle'], grant['enemy']
        assert struct.unpack_from('<7H', SSS, OFFSETS[2]+enemy*14)[3] == 42828 and grant['amount'] > 0 and grant['item'] == 152 and grant['name'] == '傀儡虫'
        start, _, start_info = saved(prefix, f'battle-{battle}', case['events'][0])
        count = sum(row['object'] == enemy for row in start['battle']['enemies']) if not observed else sum(row['battle'] == battle and row['enemy'] == enemy for row in observed)
        assert count == grant['amount']
        notice, _, notice_info = saved(prefix, f'battle-{battle}-worm-drop', case['events'][0])
        assert '获得一只傀儡虫' in all_text(notice) and metadata(ROOT / notice_info['path'])['view']['pc'] == 42830
        drops.append({'scene': scene, **grant, 'battleState': start_info, 'noticeState': notice_info, 'nativePC': 42830})
    collected_worms = sum(grant['Amount'] for reward in result['claimed'] for grant in reward['grants'] if grant['Item'] == 152)
    assert result['inventory']['傀儡虫']-result['inventoryInitial'].get('傀儡虫', 0) == collected_worms+sum(row['amount'] for row in result['battleItemGrants'])
    assert result['inventory']['土灵珠'] == result['inventoryInitial']['土灵珠'] == 1 and len(result['battleCashChanges']) == result['proof']['Battles']

empty_path = BASE / 'final-maze-empty-v1-result-1.json'
empty = json.loads(empty_path.read_bytes())
assert empty['passed'] and empty['event'] == 5049 and empty['originalItemID'] == 0 and empty['originalGrantPC'] == 37482 and empty['repeatSearches'] == 3 and empty['itemBefore'] == empty['itemAfter']
assert script(37482) == (0x1f, 0, 0, 0) and original(5049)[4] == 37479
empty_stages = []
for label in ['unpicked', 'notice', 'collected', 'repeated']:
    view, row, info = saved('final-maze-empty-v1', label, 5049)
    assert view['scene'] == 293 and row[1:3] == (752, 904) and row[6] == 2 and row[8] == 10
    assert row[4] == (37479 if label in ['unpicked', 'notice'] else 36509) and row[11] == (0 if label == 'unpicked' else 1)
    if label == 'notice':
        assert '获得紫青玉蓉膏' in all_text(view)
    empty_stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
unreachable_path = VERIFY / 'final-maze-original-unreachable.json'
unreachable = json.loads(unreachable_path.read_bytes())
assert unreachable['passed'] and len(unreachable['records']) == 3 and len(unreachable['unverifiedRecords']) == 5
reference = ROOT / '.local/pal/hidden-items-research/final-maze-text-guide.md'
assert '紫青玉蓉膏（伪）' in reference.read_text()
report = {'passed': True, 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'scope': '48 original final maze rewards, independent same-script chest state, original nine-grub quantity, two back-abyss wine pickups, normal initial and summoned worm rewards with actual public slot evidence, actual empty false-jade chest and three original preserved chests; five unclassified original records stay pending', 'records': records, 'routes': routes, 'battleItemGrants': drops, 'wormAppearances': appearances, 'retainedPriorChests': retained, 'emptyChest': {'result': digest(empty_path), 'nativeStages': empty_stages}, 'unreachable': digest(unreachable_path), 'reference': {'url': 'https://gamegene.cn/wiki/366', **digest(reference)}}
(VERIFY / 'final-maze-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'actualPickups': len(records), 'normalWormDropEntries': len(drops), 'actualWormAppearances': len(appearances), 'emptyChestAudited': 1, 'originalUnreachablePreserved': 3, 'stillPending': 5}, ensure_ascii=False))


