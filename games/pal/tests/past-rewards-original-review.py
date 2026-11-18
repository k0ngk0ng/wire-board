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


EXPECTED = {263: list(range(4580, 4587)), 264: list(range(4601, 4607)), 265: list(range(4643, 4647)), 253: [4445, 4444, 4446, 4447, 4448], 251: list(range(4415, 4423))}
records, water_battles, water_fields = [], [], []
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v16-{scene}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 16 and result['authorityFixture'] and result['scene'] == scene
    assert [row['event'] for row in result['claimed']] == events
    for reward in result['claimed']:
        event = reward['event']
        phases, stages = {}, []
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            view, row, info = saved(prefix, f'reward-{event}-{label}', event)
            assert view['scene'] == scene and row[1:3] == original(event)[1:3] and row[7:10] == original(event)[7:10]
            assert view['phase'] != 'field' if label == 'notice' else view['phase'] == 'field'
            assert [member['role'] for member in view['party']] == ([3, 0] if scene in [251, 253] else [0]) and all(member['controller'] == 0 for member in view['party'])
            phases[label] = row
            stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
        assert original(event)[8] == 10 and phases['unpicked'][4] == original(event)[4] and phases['unpicked'][6] == original(event)[6] == 2
        assert phases['unpicked'][10]*phases['unpicked'][9]+phases['unpicked'][11] == reward['closedFrame'] == original(event)[10]*original(event)[9]+original(event)[11]
        assert all(phases[label][4] == 36509 and phases[label][10]*phases[label][9]+phases[label][11] == reward['openFrame'] == 1 for label in ['collected', 'repeated'])
        records.append({'scene': scene, 'event': event, 'stages': stages, 'result': digest(result_path)})
    if scene in [251, 253]:
        for label, path in [('before', BASE / (result['sourceCheckpoint'][:-5]+'-1.json')), ('after', BASE / f'{prefix}-state-1.json')]:
            view, chase, info = snapshot(path, 4410 if scene == 251 else 4441)
            assert view['scene'] == scene and chase[6] > 0 and chase[7] in [0, 7] and chase[8] == 582 and chase[4] == 33673
            assert script(33673) == (0x40, 65535, 0, 0) and script(33674) == (0x24, 65535, 33644, 0) and script(33666) == (2, 33644, 4, 0) and script(33667) == (0x40, 65535, 7, 0)
            assert 33644 <= chase[5] <= 33667 if chase[7] == 0 else 33641 <= chase[5] <= 33671
            water_fields.append({'scene': scene, 'stage': label, 'chaseEvent': 4410 if scene == 251 else 4441, 'state': info, 'nativeEvent': list(chase), 'originalPostBattleCooldown': chase[7] == 0, 'reactivation': {'PC': 33667, 'instruction': list(script(33667))}})
            if scene == 251:
                _, ending, _ = snapshot(path, 4412)
                assert ending[4:9] == original(4412)[4:9]
        for label in result['proof']['Saved']:
            if label.startswith('battle-'):
                view, _, info = saved(prefix, label, events[0])
                assert view['battle'] is not None and [member['role'] for member in view['party']] == [3, 0]
                if any(enemy['object'] == 547 for enemy in view['battle']['enemies']):
                    water_battles.append({'scene': scene, 'label': label, 'state': info})

switches = []
for event in [4628, 4630, 4632]:
    before_view, before, before_info = saved('regional-hidden-v16-265', f'past-monster-{event}-before', event)
    after_view, after, after_info = saved('regional-hidden-v16-265', f'past-monster-{event}-opened', event)
    assert before_view['scene'] == after_view['scene'] == 265 and before[6] == 2 and after[6] == 1 and before[8] == after[8] == 615
    assert before[1:3] == after[1:3] == original(event)[1:3] and before[4] == after[4] == original(event)[4]
    assert script(original(event)[4]) == (0x16, event, 0, 1) and script(original(event)[4]+1) == (0x49, event, 1, 0)
    switches.append({'event': event, 'before': before_info, 'after': after_info, 'nativeBefore': list(before), 'nativeAfter': list(after), 'originalScript': original(event)[4]})
view, entry, door_info = saved('regional-hidden-v16-265', 'past-door-4564-arrived', 4564)
assert script(original(4564)[4]) == (0x59, 265, 0, 0) and view['scene'] == 265 and entry[4] == original(4564)[4] and entry[6] == 1

main_path = BASE / 'campaign-temple-past-1.json'
main = json.loads(main_path.read_bytes())
assert main['passed'] and not main['resumedTestRoute'] and main['candidate'] == 'audio' and main['developmentProfile'] == 'hp-9999'
checks, proof = main['checks'], main['proof']
assert checks['StaffRepeatInputs'] == 3 and checks['InventoryWithStaff']['天蛇杖'] == 1 and checks['InventoryBefore'].get('天蛇杖', 0) == checks['InventoryAfter'].get('天蛇杖', 0) == 0
assert proof['Battles'] == proof['Wins'] == 22 and proof['ScriptedFinishes'] == 0 and proof['Saves'] == len(proof['Saved']) == 164 and all(proof['Saved'].values())
replay_path = ROOT / '.local/pal/verification/audio-campaign-replay.json'
replay = json.loads(replay_path.read_bytes())
assert replay['passed'] and replay['coreSHA256'] == CORE and replay['noRelabelledNativeMemory']
accepted = next(row for row in replay['proof'] if row['stage'] == 20)
assert accepted['path'] == digest(main_path)['path'] and accepted['sha256'] == digest(main_path)['sha256'] and accepted['source'] == main
assert script(34080) == (0x49, 65535, 0, 0) and script(34082) == (0x1f, 195, 0, 0)
staff_stages = []
collected_view = None
for label in ['field-266-step-19', 'staff-reading', 'staff-collected', 'staff-no-duplication', 'queen-love-reading', 'field-254-step-30']:
    assert proof['Saved'][label]
    path = BASE / f'temple-past-{label}-state-1.json'
    view, row, info = snapshot(path, 4649)
    assert row[1:3] == original(4649)[1:3] and row[4] == 34080 and row[7] == 2 and row[8] == 612
    assert row[6] == (1 if label == 'field-266-step-19' else 0)
    if label == 'staff-reading':
        assert '天蛇杖' in all_text(view)
    if label == 'staff-collected':
        collected_view = view
    if label == 'staff-no-duplication':
        assert view['phase'] == 'field' and view['scene'] == 266 and [view['x'], view['y'], view['field']['direction']] == [collected_view['x'], collected_view['y'], collected_view['field']['direction']]
    staff_stages.append({'label': label, 'state': info, 'nativeEvent': list(row), 'scene': view['scene'], 'phase': view['phase'], 'roles': [p['role'] for p in view['party']]})

data = (ROOT / '.local/pal/original-98/data.mkf').read_bytes()
data_offsets = struct.unpack_from('<17I', data)
spell_costs = []
for object_id, name, expected_cost in [(348, '天剑', 44), (320, '雷神', 68)]:
    magic = struct.unpack_from('<7H', SSS, OFFSETS[2]+object_id*14)[0]
    cost = struct.unpack_from('<H', data, data_offsets[4]+magic*32+24)[0]
    assert cost == expected_cost
    spell_costs.append({'object': object_id, 'name': name, 'magic': magic, 'mpCost': cost})
reference = ROOT / '.local/pal/hidden-items-research/past-trial-text-guide.md'
assert all(quote in reference.read_text() for quote in ['魔兽培养室里有圣灵符x2、女娲石、步云靴', '水底迷宫前段有八仙石', '水底迷宫后段有雪蛤蟆', '找到天蛇杖'])
report = {'passed': True, 'scope': '30 actual original past underground and water chest pickups with four native saved stages, original breeding switches and optional doorway, retained original water chase and enabled sacrifice boundary, original magic costs preserved; separately revalidated existing full same-core staff pickup/no-duplication/delivery story evidence, not newly executed main story; no original edits', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'originalBreedingSwitches': switches, 'breedingDoor': {'event': 4564, 'sourceScene': 262, 'targetScene': 265, 'state': door_info}, 'originalWaterBattleSnapshots': water_battles, 'originalWaterChasePreserved': water_fields, 'originalWaterSpellCosts': spell_costs, 'originalDataSHA256': hashlib.sha256(data).hexdigest(), 'staff': {'event': 4649, 'reusedExistingStoryEvidence': True, 'report': digest(main_path), 'acceptedCampaignReplay': digest(replay_path), 'originalEntry': 34080, 'grantPC': 34082, 'item': 195, 'nativeStages': staff_stages, 'existingFreshRuntimeSaves': 164, 'existingOrdinaryBattles': 22, 'newGameplayRuns': 0}, 'reference': {'url': 'https://gamegene.cn/wiki/365', **digest(reference)}}
(ROOT / '.local/pal/verification/past-rewards-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'newPickups': len(records), 'originalBreedingSwitches': len(switches), 'newWaterBeastBattleSnapshots': len(water_battles), 'revalidatedExistingStaffChains': 1}, ensure_ascii=False))

