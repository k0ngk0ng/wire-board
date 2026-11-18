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


EXPECTED = {208: list(range(3513, 3536))+[3539, 3536, 3538, 3540, 3541, 3542, 3543, 3544, 3545, 3546], 210: [3576, 3577, 3578, 3579]}
records = []
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v15-{scene}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 15 and result['authorityFixture'] and result['scene'] == scene
    assert [row['event'] for row in result['claimed']] == events
    assert result['proof']['Battles'] == 0 and not result['combatAllowed']
    for reward in result['claimed']:
        event = reward['event']
        phases, stages = {}, []
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            view, row, info = saved(prefix, f'reward-{event}-{label}', event)
            assert view['scene'] == scene and row[1:6] == original(event)[1:6] and row[7:13] == original(event)[7:13]
            assert view['phase'] != 'field' if label == 'notice' else view['phase'] == 'field'
            assert [member['role'] for member in view['party']] == [4, 0] and all(member['controller'] == 0 for member in view['party'])
            phases[label] = row
            stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
        assert original(event)[8] == 0 and phases['unpicked'][6] == original(event)[6] == 1 and phases['collected'][6] == phases['repeated'][6] == 0
        records.append({'scene': scene, 'event': event, 'stages': stages, 'result': digest(result_path)})

door_routes = {208: [(3466, 207, 208), (3499, 208, 207), (3463, 207, 208), (3496, 208, 207), (3464, 207, 208), (3497, 208, 207), (3465, 207, 208), (3498, 208, 207), (3470, 207, 208), (3503, 208, 207), (3469, 207, 208), (3502, 208, 207), (3468, 207, 208)], 210: [(3460, 207, 209), (3551, 209, 210)]}
doors = []
for scene, route in door_routes.items():
    for event, source_scene, target_scene in route:
        entry = original(event)
        assert script(entry[4]) == (0x59, target_scene, 0, 0)
        placement = script(entry[4]+1)
        assert placement[0] == 0x46 and placement[3] in [0, 1]
        view, row, info = saved(f'regional-hidden-v15-{scene}', f'dali-door-{event}-arrived', event)
        assert view['scene'] == target_scene and view['phase'] == 'field' and row[4] == entry[4] and row[6] == 1
        assert [view['x'], view['y']] == [placement[1]*32+placement[3]*16, placement[2]*16+placement[3]*8]
        doors.append({'collectionScene': scene, 'event': event, 'sourceScene': source_scene, 'targetScene': target_scene, 'originalPlacement': [view['x'], view['y']], 'state': info})

main_path = BASE / 'campaign-dali-kirin-1.json'
main = json.loads(main_path.read_bytes())
assert main['passed'] and not main['resumedTestRoute'] and main['candidate'] == 'audio' and main['developmentProfile'] == 'hp-9999'
checks, proof = main['checks'], main['proof']
assert checks['KirinObserved'] and checks['KirinAcceptedSaved'] and checks['RepeatInputs'] == 3 and checks['RewardOrder'] == ['获得火灵珠', '获得火眼麒麟角']
assert checks['InventoryBefore'] == {'风灵珠': 1} and checks['InventoryAfter'] == {'火灵珠': 1, '火眼麒麟角': 1, '风灵珠': 1}
assert proof['Battles'] == proof['Wins'] == 43 and proof['ScriptedFinishes'] == 0 and proof['Saves'] == len(proof['Saved']) == 103 and all(proof['Saved'].values())
replay_path = ROOT / '.local/pal/verification/audio-campaign-replay.json'
replay = json.loads(replay_path.read_bytes())
assert replay['passed'] and replay['coreSHA256'] == CORE and replay['noRelabelledNativeMemory']
accepted = next(row for row in replay['proof'] if row['stage'] == 19)
assert accepted['path'] == digest(main_path)['path'] and accepted['sha256'] == digest(main_path)['sha256'] and accepted['source'] == main
assert script(29972) == (7, 224, 41075, 0) and script(29976) == (0x1f, 266, 0, 0) and script(30093) == (0x1f, 276, 0, 0)
assert script(29983) == (0x49, 65535, 0, 0)
kirin_stages = []
collected_view, collected_row = None, None
for label in ['field-202-step-7', 'kirin-awakens-reading', 'kirin-accepted', 'sacred-reward-1', 'sacred-reward-2', 'sacred-items-collected', 'ended-kirin-confirm-1', 'ended-kirin-confirm-2', 'ended-kirin-confirm-3', 'returned-213', 'returned-207']:
    assert proof['Saved'][label]
    path = BASE / f'dali-kirin-{label}-state-1.json'
    view, row, info = snapshot(path, 3354)
    assert row[1:3] == original(3354)[1:3] and row[4] == 29953 and row[7] == 2 and row[8] == 478
    if label in ['field-202-step-7', 'kirin-awakens-reading', 'kirin-accepted', 'sacred-reward-1']:
        assert row[6] == 2 and [p['role'] for p in view['party']] == [4, 0]
    else:
        assert row[6] == 0
    if label == 'kirin-accepted':
        assert view['phase'] == 'battle' and any(enemy['object'] == 463 for enemy in view['battle']['enemies']) and any(player['state'] == 2 for player in view['battle']['players'])
    if label in ['sacred-reward-1', 'sacred-reward-2']:
        assert view['phase'] == 'script' and checks['RewardOrder'][int(label[-1])-1] in all_text(view)
        assert metadata(path)['view']['pc'] == (29975 if label.endswith('1') else 30092)
    if label == 'sacred-reward-2':
        assert [p['role'] for p in view['party']] == [0]
    if label == 'sacred-items-collected':
        collected_view, collected_row = view, row
    if label.startswith('ended-kirin-confirm-'):
        assert view['phase'] == 'field' and view['scene'] == 202 and [view['x'], view['y'], view['field']['direction']] == [collected_view['x'], collected_view['y'], collected_view['field']['direction']] and row[1:13] == collected_row[1:13]
        assert [p['role'] for p in view['party']] == [4, 0]
    kirin_stages.append({'label': label, 'state': info, 'nativeEvent': list(row), 'scene': view['scene'], 'phase': view['phase'], 'roles': [p['role'] for p in view['party']]})

reference = ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md'
assert all(quote in reference.read_text() for quote in ['白苗民居一有酒x4', '汉人民居二有籐甲、300文钱、灵葫仙丹、蜂王蜜', '火眼麒麟角'])
pending_stages = []
for label, path in [('before', BASE / 'campaign-dali-kirin-state-1.json'), ('attempt', ROOT / '.local/pal/verification/dali-rewards-source/original-boots-attempt-state.json'), ('after', BASE / 'regional-hidden-v15-208-state-1.json')]:
    _, row, info = snapshot(path, 3537)
    assert row[1:13] == original(3537)[1:13] and row[6] == 1 and row[8] == 0
    pending_stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
attempt_log = ROOT / '.local/pal/verification/dali-rewards-source/original-boots-attempt-run.txt'
assert 'all original regional search stances blocked 3537' in attempt_log.read_text()
report = {'passed': True, 'scope': '37 actual original Dali pickups through 15 original door transitions, hidden events disabled independently with all four native saved stages; boots 3537 left pending and unchanged; separately revalidated existing complete same-core Kirin story evidence with two sacred rewards, original temporary solo and three saved repeat confirmations, not newly executed Kirin gameplay; no original edits', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'originalDoorRoutes': doors, 'pendingBoots': {'event': 3537, 'scene': 208, 'position': [1456, 344], 'classification': 'pending-original-search-blocked', 'readyForProgress': False, 'actuallyObtained': False, 'independentClassificationReferenceMissing': True, 'nativeStages': pending_stages, 'failedRun': digest(attempt_log)}, 'kirin': {'event': 3354, 'reusedExistingStoryEvidence': True, 'report': digest(main_path), 'acceptedCampaignReplay': digest(replay_path), 'originalEntry': 29953, 'battlePC': 29972, 'battleFormation': 224, 'fireOrbGrantPC': 29976, 'fireOrbItem': 266, 'hornGrantPC': 30093, 'hornItem': 276, 'nativeStages': kirin_stages, 'existingFreshRuntimeSaves': 103, 'existingOrdinaryBattles': 43, 'newGameplayRuns': 0}, 'reference': {'url': 'https://gamegene.cn/wiki/364', **digest(reference)}}
(ROOT / '.local/pal/verification/dali-rewards-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'newPickups': len(records), 'originalDoorTransitions': len(doors), 'revalidatedExistingKirinRewardChains': 1, 'newKirinGameplayRuns': 0}, ensure_ascii=False))

