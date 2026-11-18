import base64
from compression import zstd
import gzip
import hashlib
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
GLOBAL = ROOT / '.local/pal/verification/runtime-candidates/audio/development-health/hp-9999'
BASE = GLOBAL / 'scopes/misc-rewards-v1'
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


def first_search(view, rows):
    assert view['phase'] == 'field'
    direction = view['field']['direction']
    dx, dy = [(-16, 8), (-16, -8), (16, -8), (16, 8)][direction]
    x, y = view['x'], view['y']
    points = [(x, y)]
    for step in range(4):
        points.extend([(x+dx, y+dy), (x, y+dy*2), (x+dx*2, y)])
        x, y = x+dx, y+dy
    first = struct.unpack_from('<4H', SSS, OFFSETS[1]+(view['scene']-1)*8)[3]+1
    last = struct.unpack_from('<4H', SSS, OFFSETS[1]+view['scene']*8)[3]
    for index, point in enumerate(points):
        for event in range(first, last+1):
            row = rows[event]
            if row[6] <= 0 or row[7] >= 4 or row[7]*6-4 <= index:
                continue
            if point[0]//32 == row[1]//32 and point[1]//16 == row[2]//16 and (point[0] % 32 != 0) == (row[1] % 32 != 0):
                return event
    return 0


VERIFY = ROOT / '.local/pal/verification'
specs = {
    129: (6, 92, 25, 0, 3747, 3759, 3760, 3751, 3742, 3746, '来喔'),
    438: (24, 80, 20, 20, 7567, 7568, 7570, 7569, 7573, 7566, '买糖葫芦吗'),
    846: (51, 75, 100, 0, 11017, 11020, 11021, 11019, 11012, 11012, '买糯米吗'),
    847: (51, 75, 100, 0, 11017, 11020, 11021, 11019, 11012, 11012, '买糯米吗'),
}
merchants = []
for event, (scene, item, price, decline_cost, entry, charge_pc, grant_pc, choice_pc, insufficient, declined, receipt) in specs.items():
    prefix = f'misc-market-v1-{event}'
    path = BASE / f'{prefix}-result-1.json'
    result = json.loads(path.read_bytes())
    assert result['passed'] and not result['nativeEdits'] and result['scene'] == scene and result['event'] == event and result['price'] == price and result['declineCost'] == decline_cost
    assert original(event)[4] == entry and script(charge_pc) == (0x1e, (-price) & 65535, insufficient, 0) and script(grant_pc) == (0x1f, item, 0, 0) and script(choice_pc) == (0xa, declined, 0, 0)
    assert [step['choice'] for step in result['steps']] == ['否', '是', '是', '否']
    assert result['proof']['Battles'] == 0 and result['inventoryAfter'].get(result['item'], 0)-result['inventoryBefore'].get(result['item'], 0) == 2
    steps = []
    cash = result['cashInitial']
    for step in result['steps']:
        label = step['label']
        assert step['cashBefore'] == cash and step['delta'] == -(price if step['choice'] == '是' else decline_cost) and step['cashAfter'] == step['cashBefore']+step['delta']
        stages = []
        for stage in ['before', 'search-pose', 'reading', 'choice', 'after']:
            stage_path = BASE / f'{prefix}-{label}-{stage}-state-1.json'
            view, row, info = snapshot(stage_path, event)
            assert view['scene'] == scene and row[4] == entry and row[6] == 2 and row[8] == original(event)[8]
            if stage in ['before', 'search-pose']:
                assert view['phase'] == 'field' and view['cash'] == step['cashBefore']
            if stage == 'search-pose':
                assert first_search(view, native_events(stage_path)) == event
            if stage == 'reading':
                assert receipt in text(view)+(view.get('dialogue') or {}).get('text', '')
            if stage == 'choice':
                selected(view, step['choice'])
                assert view['cash'] == step['cashBefore']-decline_cost
                assert metadata(stage_path)['view']['pc'] == choice_pc
            if stage == 'after':
                assert view['phase'] == 'field' and view['cash'] == step['cashAfter']
            stages.append({'label': stage, 'cash': view['cash'], 'nativePC': metadata(stage_path)['view']['pc'], 'state': info})
        cash = step['cashAfter']
        steps.append({'label': label, 'choice': step['choice'], 'stages': stages})
    assert cash == result['cashFinal'] == result['cashInitial']-2*price-2*decline_cost
    merchants.append({'event': event, 'scene': scene, 'item': item, 'price': price, 'declineCost': decline_cost, 'chargePC': charge_pc, 'choicePC': choice_pc, 'grantPC': grant_pc, 'rawOriginalChargeBeforeChoice': charge_pc < choice_pc, 'actualSearchTargetVerified': True, 'steps': steps, 'insufficientFundsGameplayCovered': False, 'insufficientFundsOriginalBranchPreserved': True, 'result': digest(path)})

pickups = []
for event, scene, item, grant_pc, labels in [
    (399, 22, 173, 6178, ['unpicked', 'notice', 'collected', 'repeated']),
    (798, 49, 285, 10418, ['unpicked', 'notice', 'collected', 'repeated']),
]:
    prefix = f'misc-pickup-v1-{event}'
    path = BASE / f'{prefix}-result-1.json'
    result = json.loads(path.read_bytes())
    assert result['passed'] and not result['nativeEdits'] and result['event'] == event and result['itemID'] == item and result['repeatSearches'] == 3
    assert result['amountBefore'] == 0 and result['amountAfter'] == result['amountAfterRepeat'] == 1 and script(grant_pc) == (0x1f, item, 0, 0)
    stages = []
    for label in labels:
        view, row, info = saved(prefix, label, event)
        assert view['scene'] == scene and row[1:3] == original(event)[1:3] and row[4] == original(event)[4] and row[8] == original(event)[8]
        assert row[6] == (1 if event == 399 and label in ['unpicked', 'notice'] else 2 if event == 798 and label == 'unpicked' else 0)
        if label == 'notice':
            assert result['message'] in all_text(view)
        stages.append({'label': label, 'nativeEvent': list(row), 'state': info})
    source_view, _, source_info = saved(prefix, 'source', event)
    final_view, _, final_info = snapshot(BASE / f'{prefix}-state-1.json', event)
    assert final_view['cash']-source_view['cash'] == result['allRouteRewardCash'] == sum(row['rewardCash'] for row in result['battleCashChanges'])
    assert result['proof']['Battles'] == result['proof']['Wins'] and result['proof']['ScriptedFinishes'] == 0
    if event == 399:
        assert script(6020) == (0x49, 399, 1, 0)
        assert source_view['phase'] == 'dialogue' and [member['role'] for member in source_view['party']] == [0]
        recovery, _, recovery_info = saved(prefix, 'original-recovery-complete', event)
        assert recovery['phase'] == 'field' and [member['role'] for member in recovery['party']] == [0, 1] and recovery['party'][0]['hp'] > 0
        stages.append({'label': 'original-recovery-complete', 'state': recovery_info})
    else:
        for portal, destination in [(871, 50), (801, 49)]:
            view, _, info = saved(prefix, f'door-{portal}-arrived', event)
            entry = original(portal)[4]
            assert script(entry) == (0x59, destination, 0, 0) and view['scene'] == destination
            stages.append({'label': f'door-{portal}-arrived', 'state': info})
    pickups.append({'event': event, 'scene': scene, 'item': item, 'grantPC': grant_pc, 'stages': stages, 'sourceState': source_info, 'finalState': final_info, 'result': digest(path)})

fixture_path = BASE / 'market-harbor-entry-1.json'
fixture = json.loads(fixture_path.read_bytes())
assert fixture['passed'] and fixture['generatedFromNewGame'] and not fixture['resumedTestRoute'] and not fixture['nativeEdits'] and fixture['cash'] == 550 and fixture['scene'] == 6
assert fixture['stateSHA256'] == digest(BASE / 'market-harbor-entry-state-1.json')['sha256']
parents = []
for entry in fixture['parents']:
    path = BASE / (entry['name'][:-5]+'-1.json')
    value = json.loads(path.read_bytes())
    assert value['passed'] and digest(path)['sha256'] == entry['sha256']
    parents.append(digest(path))
assert len(parents) == 5
view, _, fixture_info = snapshot(BASE / 'market-harbor-entry-state-1.json', 129)
assert view['scene'] == 6 and [member['role'] for member in view['party']] == [0] and view['cash'] == 550
source_code = ROOT / 'games/pal/native/upstream/play.c'
assert 'p->wTriggerScript = PAL_RunTriggerScript(p->wTriggerScript, k + 1);' in source_code.read_text()
report = {'passed': True, 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'scope': 'four actual original merchant events with native first-search target and selected choice, exact pre-choice or post-choice cash timing and repeated purchases; two original pickups from real story continuations, original Ling recovery and original trap location; new-game harbor fixture independently archived; insufficient-funds gameplay combinations not added to this batch', 'merchants': merchants, 'pickups': pickups, 'harborFixture': {'result': digest(fixture_path), 'state': fixture_info, 'parents': parents}, 'originalSearchSource': digest(source_code)}
(VERIFY / 'misc-rewards-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'merchants': len(merchants), 'actualPurchases': 8, 'actualDeclines': 8, 'ordinaryPickups': len(pickups), 'searchTargetsVerified': 16, 'originalSugarDeclineChargePreserved': True}, ensure_ascii=False))

