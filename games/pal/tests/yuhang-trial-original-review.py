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
plan_path = VERIFY / 'trial-rewards-plan.json'
plan = json.loads(plan_path.read_bytes())
assert plan['passed'] and plan['diagnosticOnly'] and not plan['originalDataEdits'] and plan['sssSHA256'] == SSS_HASH
records, routes, drops = [], [], []
for case in plan['cases']:
    scene = case['scene']
    prefix = f'regional-hidden-v17-{scene}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 17 and result['authorityFixture'] and result['scene'] == scene and result['routePlanSHA256'] == digest(plan_path)['sha256']
    assert [row['event'] for row in result['claimed']] == case['events'] and len(case['events']) == (36 if scene == 220 else 22)
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
        assert original(event)[8] == 10 and phases['unpicked'][4] == original(event)[4] and phases['unpicked'][6] == original(event)[6] == 2
        assert phases['unpicked'][10]*phases['unpicked'][9]+phases['unpicked'][11] == reward['closedFrame'] == original(event)[10]*original(event)[9]+original(event)[11]
        assert all(phases[label][4] == 36509 and phases[label][10]*phases[label][9]+phases[label][11] == reward['openFrame'] == 1 for label in ['collected', 'repeated'])
        _, final_record, final_info = snapshot(BASE / f'{prefix}-state-1.json', event)
        assert final_record[1:3] == original(event)[1:3] and final_record[4] == 36509 and final_record[6] == 2 and final_record[10]*final_record[9]+final_record[11] == 1
        records.append({'scene': scene, 'event': event, 'stages': stages, 'finalCaseState': {'state': final_info, 'nativeEvent': list(final_record)}, 'result': digest(result_path)})
    expected_legs = [leg for step in case['steps'] for leg in step['portalsBefore']]
    assert len(result['trialRoute']) == len(expected_legs)
    for index, (actual, expected) in enumerate(zip(result['trialRoute'], expected_legs), 1):
        assert actual['step'] == index and actual['kind'] == expected['kind'] and actual['sceneBefore'] == expected['scene'] and actual['sceneAfter'] == expected['destinationScene']
        if expected['kind'] == 'earth-return':
            assert expected['teleportScript'] == 30770 and script(30770) == (0x59, 216, 0, 0) and script(30771) == (0x46, 26, 46, 0)
            selected_view, _, selected_info = saved(prefix, f'trial-route-{index}-earth-selected', 3749)
            returned_view, _, returned_info = saved(prefix, f'trial-route-{index}-earth-returned', 3749)
            assert selected_view['phase'] == 'menu' and selected_view['scene'] == expected['scene'] and returned_view['scene'] == 216 and returned_view['phase'] == 'field'
            assert [returned_view['x'], returned_view['y']] == actual['positionAfter']
            routes.append({'scene': scene, 'step': index, 'kind': expected['kind'], 'selected': selected_info, 'returned': returned_info, 'actualPosition': actual['positionAfter']})
        else:
            event = expected['event']
            view, row, info = saved(prefix, f'trial-route-{index}-door-{event}-arrived', event)
            assert view['scene'] == expected['destinationScene'] and view['phase'] == 'field' and row[4] == expected['script'] == original(event)[4] and row[6] == 1 and row[7] == original(event)[7]
            assert [view['x'], view['y']] == actual['positionAfter']
            routes.append({'scene': scene, 'step': index, 'kind': expected['kind'], 'event': event, 'originalScript': row[4], 'chainedEvents': expected['chainedEvents'], 'state': info, 'actualPosition': actual['positionAfter']})
    for grant in result['battleItemGrants']:
        enemy, battle = grant['enemy'], grant['battle']
        assert struct.unpack_from('<7H', SSS, OFFSETS[2]+enemy*14)[3] == 42828 and grant['endScript'] == 42828 and grant['item'] == 152 and grant['name'] == '傀儡虫' and grant['amount'] > 0
        start, _, start_info = saved(prefix, f'battle-{battle}', case['events'][0])
        assert sum(row['object'] == enemy for row in start['battle']['enemies']) == grant['amount']
        notice, _, notice_info = saved(prefix, f'battle-{battle}-worm-drop', case['events'][0])
        assert '获得一只傀儡虫' in all_text(notice)
        notice_path = ROOT / notice_info['path']
        assert metadata(notice_path)['view']['pc'] == 42830
        drops.append({'scene': scene, **grant, 'battleState': start_info, 'noticeState': notice_info, 'nativePC': 42830})
    assert result['inventory']['傀儡虫']-result['inventoryInitial'].get('傀儡虫', 0) == sum(row['amount'] for row in result['battleItemGrants']) and result['inventory']['土灵珠'] == result['inventoryInitial']['土灵珠'] == 1

main_path = BASE / 'campaign-old-yuhang-1.json'
main = json.loads(main_path.read_bytes())
assert main['passed'] and not main['resumedTestRoute'] and main['candidate'] == 'audio' and main['developmentProfile'] == 'hp-9999'
checks, proof = main['checks'], main['proof']
assert checks['WrongUseChecked'] and checks['ClothUsed'] and checks['RepeatInputs'] == 3
assert checks['InventoryWithCloth']['凤纹手绢'] == checks['InventoryAfterWrongUse']['凤纹手绢'] == 1 and checks['InventoryAfterCloth'].get('凤纹手绢', 0) == 0
assert checks['InventoryBeforeExchange']['木剑'] == 1 and checks['InventoryAfter'].get('木剑', 0) == 0 and checks['InventoryAfter']['水灵珠'] == checks['InventoryAfter']['金凤凰蛋壳'] == 1
assert proof['Battles'] == proof['Wins'] == 8 and proof['ScriptedFinishes'] == 0 and proof['Saves'] == len(proof['Saved']) == 88 and all(proof['Saved'].values())
replay_path = VERIFY / 'audio-campaign-replay.json'
replay = json.loads(replay_path.read_bytes())
assert replay['passed'] and replay['coreSHA256'] == CORE and replay['noRelabelledNativeMemory']
accepted = next(row for row in replay['proof'] if row['stage'] == 21)
assert accepted['path'] == digest(main_path)['path'] and accepted['sha256'] == digest(main_path)['sha256'] and accepted['source'] == main
assert script(34476) == (0x1f, 292, 0, 0) and script(34918) == (0x20, 166, 0, 34912) and script(34927) == (0x1f, 265, 0, 0) and script(30511) == (0x1f, 275, 0, 0)
old_stages = []
for event, labels in [(4674, ['field-269-step-2', 'cloth-granted-reading', 'cloth-collected', 'cloth-selected-away', 'cloth-wrong-place-preserved', 'cloth-selected-at-merchant', 'cloth-recognized-reading', 'field-204-step-14']), (4663, ['field-267-step-12', 'exchange-not-yet-made', 'water-orb-reading', 'orb-return-transition', 'egg-shell-reading', 'returned-no-duplication'])]:
    for label in labels:
        assert proof['Saved'][label]
        path = BASE / f'old-yuhang-{label}-state-1.json'
        view, row, info = snapshot(path, event)
        assert row[8] == original(event)[8]
        if event == 4674 and label in ['cloth-collected', 'cloth-selected-away', 'cloth-wrong-place-preserved']:
            assert row[4] == 34482 and row[6] == 2 and row[1:3] == (848, 360)
        if event == 4663 and label in ['exchange-not-yet-made', 'water-orb-reading', 'orb-return-transition']:
            assert row[4] == 34913 and row[6] == 2 and row[1:3] == (1088, 1616)
        if label == 'water-orb-reading':
            assert '水灵珠' in all_text(view) and metadata(path)['view']['pc'] == 34926
        if label == 'egg-shell-reading':
            assert '金凤凰蛋壳' in all_text(view) and view['scene'] == 204
        old_stages.append({'event': event, 'label': label, 'state': info, 'nativeEvent': list(row), 'scene': view['scene'], 'phase': view['phase'], 'roles': [p['role'] for p in view['party']]})

repeat_path = BASE / 'old-yuhang-rewards-v1-result-1.json'
repeat = json.loads(repeat_path.read_bytes())
assert repeat['passed'] and repeat['event'] == 4674 and repeat['repeatConversations'] == 3 and not repeat['nativeEdits']
assert repeat['inventoryBefore'] == repeat['inventoryAfter'] == {'凤纹手绢': 1, '木剑': 1} and repeat['proof']['Saves'] == 4 and repeat['proof']['Battles'] == 0 and all(n > 0 for n in repeat['readingsPerRepeat'])
repeat_stages = []
for label in ['before-repeat', 'repeat-1', 'repeat-2', 'repeat-3']:
    view, row, info = saved('old-yuhang-rewards-v1', label, 4674)
    assert view['phase'] == 'field' and view['scene'] == 269 and row[4] == 34482 and row[6] == 2 and row[8] == 588 and row[1:3] == (848, 360)
    repeat_stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
assert all(script(pc)[0] != 0x1f for pc in range(34482, 34486)) and script(34485)[0] == 0
reference = ROOT / '.local/pal/hidden-items-research/past-trial-text-guide.md'
assert all(quote in reference.read_text() for quote in ['木剑跟他交换水灵珠', '给了逍遥一条凤纹手帕', '试炼窟第四层', '试炼窟第五层'])
report = {'passed': True, 'scope': '58 actual original fourth/fifth trial chests through original doors/chained falls and retained-inventory earth return, separate original enemy worm end-script accounting; existing complete old Yuhang cloth/use and wooden-sword/orb/egg chain revalidated, plus three new actual grandmother repeat dialogues; no original edits, not later trial floors or all hidden rewards', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'routePlan': digest(plan_path), 'routes': routes, 'battleItemGrants': drops, 'oldYuhang': {'events': [4674, 4663], 'reusedExistingStoryEvidence': True, 'report': digest(main_path), 'acceptedCampaignReplay': digest(replay_path), 'nativeStages': old_stages, 'existingFreshRuntimeSaves': 88, 'existingOrdinaryBattles': 8, 'newMainStoryRuns': 0, 'clothGrant': {'PC': 34476, 'Item': 292}, 'woodenSwordConsumption': {'PC': 34918, 'Item': 166}, 'waterOrbGrant': {'PC': 34927, 'Item': 265}, 'eggFollowup': {'PC': 30511, 'Item': 275}}, 'newClothRepeat': {'report': digest(repeat_path), 'nativeStages': repeat_stages, 'newFreshRuntimeSaves': 4, 'repeatConversations': 3}, 'reference': {'url': 'https://gamegene.cn/wiki/365', **digest(reference)}}
(VERIFY / 'yuhang-trial-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'newChestPickups': len(records), 'routeActions': len(routes), 'normalWormDropEntries': len(drops), 'newClothRepeatConversations': 3, 'revalidatedOldEventChains': 2}, ensure_ascii=False))

