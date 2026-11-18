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


EXPECTED = {176: [2915], 181: [2974, 2973, 2975, 2976, 2977], 190: [3188, 3189, 3191, 3192, 3193, 3194, 3195], 191: [3211, 3215, 3217, 3214, 3216, 3213], 192: list(range(3223, 3245))}
records = []
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v14-{scene}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 14 and result['authorityFixture'] and result['scene'] == scene
    assert [row['event'] for row in result['claimed']] == events
    for reward in result['claimed']:
        event = reward['event']
        phases, stages = {}, []
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            view, row, info = saved(prefix, f'reward-{event}-{label}', event)
            assert view['scene'] == scene and row[1:3] == original(event)[1:3] and row[7:10] == original(event)[7:10]
            assert view['phase'] != 'field' if label == 'notice' else view['phase'] == 'field'
            phases[label] = row
            stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
        assert phases['unpicked'][4] == original(event)[4] and phases['unpicked'][6] == original(event)[6]
        if original(event)[8] == 10:
            assert phases['unpicked'][10]*phases['unpicked'][9]+phases['unpicked'][11] == reward['closedFrame']
            assert all(phases[label][4] == 36509 and phases[label][10]*phases[label][9]+phases[label][11] == reward['openFrame'] == 1 for label in ['collected', 'repeated'])
        else:
            assert original(event)[8] == 0 and phases['collected'][6] == phases['repeated'][6] == 0
        records.append({'scene': scene, 'event': event, 'stages': stages, 'result': digest(result_path)})

version_contrast = []
for event in [3211, 3215]:
    prefix = 'regional-hidden-v14-191'
    result = json.loads((BASE / f'{prefix}-result-1.json').read_bytes())
    reward = next(row for row in result['claimed'] if row['event'] == event)
    assert reward['grants'] == [{'PC': 37674, 'Item': 152, 'Amount': 1, 'Cash': 0, 'Name': '傀儡虫'}]
    assert reward['repeatSearches'] >= 3 and reward['notification']
    _, current, info = saved(prefix, f'reward-{event}-collected', event)
    other = 3215 if event == 3211 else 3211
    _, other_row, _ = saved(prefix, f'reward-{event}-collected', other)
    assert current[4] == 36509 and other_row[4] == (37671 if event == 3211 else 36509)
    version_contrast.append({'event': event, 'nativeEntryAfter': current[4], 'otherEvent': other, 'otherEntry': other_row[4], 'state': info})

doors = []
for event, source_scene, target_scene in [(3197, 191, 176), (2890, 176, 192), (3220, 192, 176), (2892, 176, 192), (3222, 192, 176), (2889, 176, 192), (3219, 192, 176), (2891, 176, 192), (3221, 192, 176), (2888, 176, 192)]:
    original_entry = original(event)
    assert script(original_entry[4]) == (0x59, target_scene, 0, 0)
    view, row, info = saved('regional-hidden-v14-192', f'peach-door-{event}-arrived', event)
    assert view['scene'] == target_scene and view['phase'] == 'field' and row[4] == original_entry[4] and row[6] == 1
    doors.append({'event': event, 'sourceScene': source_scene, 'targetScene': target_scene, 'state': info})
for event in range(2901, 2909):
    _, before, _ = snapshot(BASE / 'peach-quest-v1-field-176-state-1.json', event)
    _, after_mission, _ = snapshot(BASE / 'peach-quest-v1-field-191-state-1.json', event)
    _, entered, _ = saved('regional-hidden-v14-192', 'peach-door-3197-arrived', event)
    assert before[6] == original(event)[6] and after_mission[6] == entered[6] == 0
    assert script(27046+event-2901) == (0x49, event, 0, 0)
gated_path = ROOT / '.local/pal/verification/peach-rewards-source/original-gated-village-state.json'
gated_checks = []
for event in [2901, 2902]:
    view, row, info = snapshot(gated_path, event)
    assert view['scene'] == 176 and row[6] == 2 and row[8] == 523
    activation_pc = 26840+event-2901
    assert script(activation_pc) == (0x49, event, 2, 0)
    gated_checks.append({'event': event, 'state': info, 'nativeEvent': list(row), 'activationPC': activation_pc})

for event, route in [(3227, [3220, 2892]), (3231, [3222, 2889]), (3236, [3219, 2891]), (3243, [3221, 2888])]:
    view, _, _ = saved('regional-hidden-v14-192', f'reward-{event}-unpicked', event)
    assert view['scene'] == 192
    assert all(next(row for row in doors if row['event'] == door)['targetScene'] in [176, 192] for door in route)

prefix = 'regional-hidden-v14-181'
result = json.loads((BASE / f'{prefix}-result-1.json').read_bytes())
assert struct.unpack_from('<7H', SSS, OFFSETS[2]+402*14)[3] == 41008
assert script(41008) == (0x1f, 104, 0, 0) and script(41010) == (0xffff, 13115, 0, 0)
battle_drops = []
for grant in result['battleItemGrants']:
    assert grant['enemy'] == 402 and grant['endScript'] == 41008 and grant['item'] == 104 and grant['amount'] > 0
    battle = grant['battle']
    start_view, _, start_info = saved(prefix, f'battle-{battle}', 2976)
    assert len([enemy for enemy in start_view['battle']['enemies'] if enemy['object'] == 402]) == grant['amount']
    path = BASE / f'{prefix}-battle-{battle}-fruit-drop-state-1.json'
    view, _, notice_info = snapshot(path, 2976)
    assert view['phase'] == 'battle' and view['battle']['phase'] == 1 and metadata(path)['view']['pc'] == 41010 and '捡到一颗鼠儿果' in all_text(view)
    battle_drops.append({**grant, 'battleState': start_info, 'noticeState': notice_info, 'nativePC': 41010})
assert sum(row['amount'] for row in battle_drops) == 3
assert result['inventory']['鼠儿果']-result['inventoryInitial'].get('鼠儿果', 0) == 2+sum(row['amount'] for row in battle_drops) == 5

reference = ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md'
assert all(quote in reference.read_text() for quote in ['民居一有酒x2', '民居三有酒x3', '傀儡虫（右边的拿不到）'])
report = {'passed': True, 'scope': '41 original actual Peach rewards through original south/north and five-room doors, native notices/entries/opening and repeat searches; source-correct removal of village gate obstacles including original fear-activated door barriers, two independently obtained worm boxes despite guide claim, and three original enemy-402 mouse-fruit end-script grants reconciled with actual inventory; no original edits', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'originalHouseDoorSequence': doors, 'originalBlockedHouseDoor': gated_checks, 'current98VersionContrast': {'referenceQuote': '傀儡虫（右边的拿不到）', 'eventsActuallyObtained': [3211, 3215], 'nativeChecks': version_contrast}, 'battleItemGrants': battle_drops, 'reference': {'url': 'https://gamegene.cn/wiki/364', **digest(reference)}}
(ROOT / '.local/pal/verification/peach-rewards-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'nativeCases': len(records), 'current98WormBoxesActuallyObtained': 2, 'originalBattleFruitDrops': 3, 'originalHouseDoorTransitions': len(doors)}, ensure_ascii=False))


