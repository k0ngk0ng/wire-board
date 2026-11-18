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


def native_spells(path):
    state = json.loads(path.read_bytes())
    packed = base64.b64decode(state['native'], validate=True)
    raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
    size = struct.unpack_from('<I', raw, 8)[0]
    meta = json.loads(raw[16:16+size])
    memory = raw[16+size:]
    assert meta['buildID'] == CORE and hashlib.sha256(memory+struct.pack('<I', meta['stackPointer'])).hexdigest() == meta['memoryID']
    names = struct.pack('<6H', 36, 37, 38, 40, 39, 41)
    starts, at = [], memory.find(names)
    while at >= 0:
        start = at-36
        if start >= 0 and start+900 <= len(memory) and all(struct.unpack_from('<H', memory, start+108+member['role']*2)[0] == member['hp'] and struct.unpack_from('<H', memory, start+84+member['role']*2)[0] == member['maxHP'] for member in state['public']['party']):
            starts.append(start)
        at = memory.find(names, at+1)
    assert len(starts) == 1, (path, starts)
    spells = {role: [struct.unpack_from('<H', memory, starts[0]+384+index*12+role*2)[0] for index in range(32)] for role in range(6)}
    return spells


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


def text(view, selected_only=False, include_black=False):
    chars = []
    def add(code, color):
        if (color or include_black) and (not selected_only or 249 <= color <= 254):
            chars.append(chr(code))
    if 'raster' not in view:
        for op in view['overlay']:
            if op['kind'] == 0:
                add(op['id'], op['color'])
    else:
        raster, seen = view['raster'], set()
        def visit(index):
            if index <= 0 or index > len(raster['nodes']) or index in seen:
                return
            seen.add(index)
            node = raster['nodes'][index-1]
            visit(node[1])
            visit(node[2])
            if node[0] == 2:
                add(node[5], node[8])
        visit(raster['root'])
    return ''.join(chars)


VERIFY = ROOT / '.local/pal/verification'
EXPECTED = [3607, 3645, 3641, 3642, 3643, 3644]
catalog = {row['Event']: row for row in json.loads((VERIFY / 'hidden-search-candidates.json').read_bytes())['candidates']}
assert script(39856) == (0x84, 3607, 2, 39656) and script(31286) == (0x83, 3608, 0, 31286)
assert script(31547) == (0x79, 37, 31550, 0) and script(31548) == (0x55, 389, 5, 0) and script(31550) == (0x55, 389, 2, 0)
records, branch_checks = [], []
for branch, roles, fire_role in [('two', [0, 4], 4), ('three', [1, 0, 4], 1)]:
    prefix = f'trial-bottom-v1-{branch}'
    path = BASE / f'{prefix}-result-1.json'
    result = json.loads(path.read_bytes())
    assert result['passed'] and not result['nativeEdits'] and result['nativeBuildID'] == CORE and result['roles'] == roles and result['fireRole'] == fire_role
    assert [row['event'] for row in result['claimed']] == EXPECTED and [row['event'] for row in result['gods']] == list(range(3636, 3641))
    route_checks = []
    for index, leg in enumerate(result['route'], 1):
        original_leg = leg['originalLeg']
        event = original_leg['event']
        row = original(event)
        assert original_leg['position'] == list(row[1:3]) and original_leg['mode'] == row[7] and original_leg['script'] == row[4]
        assert script(row[4]) == (0x59, original_leg['rawDestinationScene'], 0, 0)
        placement = script(row[4]+1)
        assert placement[0] == 0x46 and original_leg['rawDestination'] == [placement[1]*32+placement[3]*16, placement[2]*16+placement[3]*8]
        view, _, info = saved(prefix, f'trial-route-{index}-door-{event}-arrived', event)
        assert view['phase'] == 'field' and view['scene'] == original_leg['destinationScene'] and [view['x'], view['y']] == leg['positionAfter']
        assert [member['role'] for member in view['party']] == roles
        route_checks.append({'event': event, 'scene': view['scene'], 'position': leg['positionAfter'], 'state': info})
    source_events = native_events(BASE / f'{prefix}-source-state-1.json')
    final_events = native_events(BASE / f'{prefix}-complete-state-1.json')
    old_chests = [event for event, row in source_events.items() if row[8] in [10, 389] and row[4] == 36509 and row[6] == 2]
    for event in old_chests:
        before_row, after_row = source_events[event], final_events[event]
        assert before_row[1:5] == after_row[1:5] and before_row[6:13] == after_row[6:13], (branch, event)
    for reward in result['claimed']:
        event, stages = reward['event'], []
        point = catalog[event]
        assert reward['position'] == [point['X'], point['Y']] and reward['grants'] == point['Grants'] and reward['notification'] and reward['repeatSearches'] == 3
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            view, row, info = saved(prefix, f'reward-{event}-{label}', event)
            assert view['scene'] == 214 and row[1:3] == original(event)[1:3] and row[8] == original(event)[8]
            assert row[6] == (2 if event != 3607 or label == 'unpicked' else 0)
            if label == 'unpicked':
                assert row[4] == original(event)[4] and row[10]*row[9]+row[11] == 0
            elif event != 3607:
                assert row[10]*row[9]+row[11] == 1
                if label in ['collected', 'repeated']:
                    assert row[4] == 36509
            if label == 'notice':
                assert point['Messages'][0] in all_text(view)
            stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
        records.append({'branch': branch, 'event': event, 'stages': stages, 'result': digest(path)})
    boat_checks = []
    for label in ['reed-invalid-selected', 'reed-invalid-finished', 'reed-bank-selected', 'reed-bank-placed', 'reed-first-crossed']:
        view, row, info = saved(prefix, label, 3607)
        _, shore, _ = saved(prefix, label, 3608)
        if label.endswith('selected'):
            assert view['phase'] == 'menu' and view['menu']['kind'] == 5 and '芦苇漂' in text(view, True)
        if label in ['reed-invalid-selected', 'reed-invalid-finished', 'reed-bank-selected']:
            assert row[1:3] == original(3607)[1:3] and row[6] == 0 and shore[6] == 2
        elif label == 'reed-bank-placed':
            assert row[1:3] == (272, 1448) and row[6] == 2 and row[7] == 1 and shore[6] == 0
        else:
            assert view['phase'] == 'field' and [view['x'], view['y']] == [512, 1568] and row[1:3] == (496, 1560) and row[6] == 1 and row[7] == 4 and row[4] == 31296
        boat_checks.append({'label': label, 'nativeEvent': list(row), 'originalShoreEvent': list(shore), 'state': info})
    transits = []
    for label in result['proof']['Saved']:
        if not label.startswith('boat-transit-'):
            continue
        view, _, info = saved(prefix, label, 3607)
        meta = metadata(BASE / f'{prefix}-{label}-state-1.json')
        assert view['phase'] == 'script' and view['scene'] == 214 and meta['event']['kind'] == 4 and meta['view']['pc'] > 0
        assert [(member['role'], member['controller']) for member in view['party']] == [(role, 0) for role in roles]
        transits.append({'label': label, 'position': [view['x'], view['y']], 'nativePC': meta['view']['pc'], 'state': info})
    assert len(transits) == result['boatSaves'] > 0
    god_checks = []
    source_path = BASE / f'{prefix}-before-gods-role-{roles[0]}-menu-state-1.json'
    before = native_spells(source_path)
    after = native_spells(BASE / f'{prefix}-complete-state-1.json')
    targets = {3636: (4, 315), 3637: (0, 320), 3638: (4, 325), 3639: (fire_role, 389), 3640: (0, 336)}
    for event, (role, magic) in targets.items():
        stages = []
        for label in ['unpicked', 'notice', 'learned', 'repeated']:
            view, row, info = saved(prefix, f'god-{event}-{label}', event)
            assert row[1:3] == original(event)[1:3] and row[8] == 0 and row[7] == 1 and row[6] == (1 if label == 'unpicked' else 0)
            if label in ['learned', 'repeated']:
                assert native_spells(BASE / f'{prefix}-god-{event}-{label}-state-1.json')[role].count(magic) == 1
            stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
        assert magic not in before[role] and after[role].count(magic) == 1
        name = {315: '风神', 320: '雷神', 325: '雪妖', 389: '火神', 336: '山神'}[magic]
        view, _, info = saved(prefix, f'after-gods-role-{role}-{name}', event)
        assert view['phase'] == 'menu' and view['menu']['kind'] == 10 and view['menu']['role'] == role and name in all_text(view)
        god_checks.append({'event': event, 'role': role, 'magic': magic, 'nativeLearnedExactlyOnce': True, 'stages': stages, 'visibleMenu': info})
    other_fire_role = 1 if fire_role == 4 else 4
    assert after[other_fire_role].count(389) == before[other_fire_role].count(389)
    drops = []
    for drop in result['battleItemGrants']:
        battle_path = BASE / f"{prefix}-battle-{drop['battle']}-state-1.json"
        battle_view, _, battle_info = snapshot(battle_path, 3607)
        objects = [enemy['object'] for enemy in battle_view['battle']['enemies']]
        assert objects.count(drop['enemy']) == drop['amount']
        assert struct.unpack_from('<7H', SSS, OFFSETS[2]+drop['enemy']*14)[3] == 42828 and script(42828) == (0x1f, 152, 0, 0)
        drop_path = BASE / f"{prefix}-battle-{drop['battle']}-worm-drop-state-1.json"
        drop_view, _, drop_info = snapshot(drop_path, 3607)
        assert metadata(drop_path)['view']['pc'] == 42830 and '获得一只傀儡虫' in all_text(drop_view)
        drops.append({**drop, 'battleState': battle_info, 'noticeState': drop_info, 'nativePC': 42830})
    for event in EXPECTED[1:]:
        _, row, _ = saved(prefix, 'complete', event)
        assert row[4] == 36509 and row[6] == 2 and row[10]*row[9]+row[11] == 1
    branch_checks.append({'branch': branch, 'roles': roles, 'fireRole': fire_role, 'nativeSpellsBefore': before, 'nativeSpellsAfter': after, 'boats': boat_checks, 'transits': transits, 'gods': god_checks, 'route': route_checks, 'priorClaimedChestStatesRetained': old_chests, 'battleItemGrants': drops, 'result': digest(path)})
reference = ROOT / '.local/pal/hidden-items-research/past-trial-text-guide.md'
assert all(quote in reference.read_text() for quote in ['试炼窟底层有魅影神靴、玄冥宝刀、冥蛇杖、无尘剑、金罡珠', '灵儿习得火神，逍遥习得雷神、山神，阿奴习得风神、雪妖'])
report = {'passed': True, 'nativeBuildID': CORE, 'nativeEdits': False, 'sssSHA256': SSS_HASH, 'scope': 'six original bottom item events through actual menus, exact blocked-front refusal and original bank placement, actual boat rides and fresh-runtime continuation, five original god magics and two genuine party fire branches; read-only native event and complete role magic tables', 'records': records, 'branches': branch_checks, 'reference': {'url': 'https://gamegene.cn/wiki/365', **digest(reference)}}
(VERIFY / 'trial-bottom-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'uniqueItemEvents': len(EXPECTED), 'actualBranchPickups': len(records), 'godBranches': len(branch_checks)}, ensure_ascii=False))


