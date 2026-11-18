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


VERIFY = ROOT / '.local/pal/verification'
plan_path = VERIFY / 'trial-deep-rewards-plan.json'
plan = json.loads(plan_path.read_bytes())
assert plan['passed'] and plan['diagnosticOnly'] and not plan['originalDataEdits'] and plan['sssSHA256'] == SSS_HASH and plan['separatelyReviewedOriginalEvents'] == [4000]
records, routes, drops = [], [], []
for case in plan['cases']:
    scene = case['scene']
    prefix = f'regional-hidden-v18-{scene}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 18 and result['authorityFixture'] and result['scene'] == scene and result['routePlanSHA256'] == digest(plan_path)['sha256']
    assert [row['event'] for row in result['claimed']] == case['events'] and len(case['events']) == {223: 26, 224: 31, 226: 8, 227: 13}[scene]
    for reward in result['claimed']:
        event = reward['event']
        phases, stages = {}, []
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            view, row, info = saved(prefix, f'reward-{event}-{label}', event)
            assert view['scene'] == scene and row[1:3] == original(event)[1:3] and row[7:10] == original(event)[7:10]
            assert view['phase'] != 'field' if label == 'notice' else view['phase'] == 'field'
            assert [member['role'] for member in view['party']] == [0, 4] and all(member['controller'] == 0 for member in view['party'])
            phases[label] = row
            stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
        assert original(event)[8] in [10, 389] and phases['unpicked'][4] == original(event)[4] and phases['unpicked'][6] == original(event)[6] == 2
        assert phases['unpicked'][10]*phases['unpicked'][9]+phases['unpicked'][11] == reward['closedFrame'] == original(event)[10]*original(event)[9]+original(event)[11]
        assert all(phases[label][4] == 36509 and phases[label][10]*phases[label][9]+phases[label][11] == reward['openFrame'] == 1 for label in ['collected', 'repeated'])
        _, final_record, final_info = snapshot(BASE / f'{prefix}-state-1.json', event)
        assert final_record[1:3] == original(event)[1:3] and final_record[4] == 36509 and final_record[6] == 2 and final_record[8] == original(event)[8] and final_record[10]*final_record[9]+final_record[11] == 1
        records.append({'scene': scene, 'event': event, 'stages': stages, 'finalCaseState': {'state': final_info, 'nativeEvent': list(final_record)}, 'result': digest(result_path)})
    expected_legs = [leg for step in case['steps'] for leg in step['portalsBefore']]
    assert len(result['trialRoute']) == len(expected_legs)
    for index, (actual, expected) in enumerate(zip(result['trialRoute'], expected_legs), 1):
        assert actual['step'] == index and actual['kind'] == expected['kind'] and actual['sceneBefore'] == expected['scene'] and actual['sceneAfter'] == expected['destinationScene']
        if expected['kind'] == 'earth-return':
            assert expected['teleportScript'] == 30770 and script(30770) == (0x59, 216, 0, 0) and script(30771) == (0x46, 26, 46, 0)
            selected_view, _, selected_info = saved(prefix, f'trial-route-{index}-earth-selected', case['events'][0])
            returned_view, _, returned_info = saved(prefix, f'trial-route-{index}-earth-returned', case['events'][0])
            assert selected_view['phase'] == 'menu' and returned_view['scene'] == 216 and returned_view['phase'] == 'field' and [returned_view['x'], returned_view['y']] == actual['positionAfter']
            routes.append({'scene': scene, 'step': index, 'kind': expected['kind'], 'selected': selected_info, 'returned': returned_info, 'actualPosition': actual['positionAfter']})
        else:
            event = expected['event']
            view, row, info = saved(prefix, f'trial-route-{index}-door-{event}-arrived', event)
            assert view['scene'] == expected['destinationScene'] and view['phase'] == 'field' and row[4] == expected['script'] == original(event)[4] and row[6] == 1 and row[7] == original(event)[7] and [view['x'], view['y']] == actual['positionAfter']
            routes.append({'scene': scene, 'step': index, 'kind': expected['kind'], 'event': event, 'originalScript': row[4], 'chainedEvents': expected['chainedEvents'], 'state': info, 'actualPosition': actual['positionAfter']})
    for grant in result['battleItemGrants']:
        enemy, battle = grant['enemy'], grant['battle']
        assert struct.unpack_from('<7H', SSS, OFFSETS[2]+enemy*14)[3] == 42828 and grant['endScript'] == 42828 and grant['item'] == 152 and grant['name'] == '傀儡虫' and grant['amount'] > 0
        start, _, start_info = saved(prefix, f'battle-{battle}', case['events'][0])
        assert sum(row['object'] == enemy for row in start['battle']['enemies']) == grant['amount']
        notice, _, notice_info = saved(prefix, f'battle-{battle}-worm-drop', case['events'][0])
        assert '获得一只傀儡虫' in all_text(notice) and metadata(ROOT / notice_info['path'])['view']['pc'] == 42830
        drops.append({'scene': scene, **grant, 'battleState': start_info, 'noticeState': notice_info, 'nativePC': 42830})
    primary_worms = sum(grant['Amount'] for reward in result['claimed'] for grant in reward['grants'] if grant['Item'] == 152)
    assert result['inventory']['傀儡虫']-result['inventoryInitial'].get('傀儡虫', 0) == primary_worms+sum(row['amount'] for row in result['battleItemGrants']) and result['inventory']['土灵珠'] == result['inventoryInitial']['土灵珠'] == 1

unreachable_path = VERIFY / 'trial-deep-original-unreachable.json'
unreachable = json.loads(unreachable_path.read_bytes())
assert unreachable['passed'] and unreachable['readyForProgress'] and not unreachable['nativeEdits'] and unreachable['nativeBuildID'] == CORE and unreachable['records'][0]['event'] == 4000
reference = ROOT / '.local/pal/hidden-items-research/past-trial-text-guide.md'
assert all(quote in reference.read_text() for quote in ['试炼窟第六层', '试炼窟第七层', '传送层上层', '传送层下层', '9.隐蛊（拿不到）'])
report = {'passed': True, 'scope': '78 actual original deep trial chest pickups through original doors/ropes/chained and same-scene teleport routes, retained-inventory earth return and original normal worm end-script ledger; final whole-branch chest state retention, separately source/map/native/actual-priority confirmed original unclaimable record 4000; no original edits, not bottom cave or all hidden rewards', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'routePlan': digest(plan_path), 'routes': routes, 'battleItemGrants': drops, 'unreachable': digest(unreachable_path), 'reference': {'url': 'https://gamegene.cn/wiki/365', **digest(reference)}}
(VERIFY / 'trial-deep-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'newChestPickups': len(records), 'routeActions': len(routes), 'normalWormDropEntries': len(drops), 'originalUnreachablePreserved': 1}, ensure_ascii=False))

