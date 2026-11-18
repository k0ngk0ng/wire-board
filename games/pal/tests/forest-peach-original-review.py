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


def visible_text(view, selected_only=False):
    return ''.join(chr(op['id']) for op in view.get('overlay', []) if op['kind'] == 0 and (not selected_only or 249 <= op['color'] <= 254))+(view.get('dialogue') or {}).get('text', '')


EXPECTED = [('187', [3150, 3136, 3137, 3147, 3148, 3149, 3146]), ('193-pre-lift', [3313, 3314, 3316, 3317]), ('193-post-lift', [3315])]
records = []
for suffix, events in EXPECTED:
    prefix = 'regional-hidden-v13-'+suffix
    path = BASE / f'{prefix}-result-1.json'
    result = json.loads(path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 13 and result['authorityFixture']
    assert [row['event'] for row in result['claimed']] == events
    for reward in result['claimed']:
        event = reward['event']
        phases, stages = {}, []
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            view, row, info = saved(prefix, f'reward-{event}-{label}', event)
            assert view['scene'] == result['scene'] and row[1:3] == original(event)[1:3] and row[7:10] == original(event)[7:10]
            assert view['phase'] != 'field' if label == 'notice' else view['phase'] == 'field'
            phases[label] = row
            stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
        assert phases['unpicked'][4] == original(event)[4] and phases['unpicked'][6] == original(event)[6]
        if original(event)[8] == 0:
            assert event in [3136, 3137] and script(25977) == (0x1f, 102, 0, 0)
            assert phases['collected'][4] == phases['repeated'][4] == 25979 and all(row[6] == 1 for row in phases.values())
        else:
            assert script(original(event)[4]) == (0x49, 65535, 0, 0)
            assert all(phases[label][6] == 0 for label in ['notice', 'collected', 'repeated']) and reward['disappearsAfterPickup']
        records.append({'event': event, 'scene': result['scene'], 'branch': suffix, 'stages': stages, 'result': digest(path)})

prefix = 'peach-quest-v1'
result_path = BASE / f'{prefix}-result-1.json'
quest = json.loads(result_path.read_bytes())
assert quest['passed'] and not quest['resumedTestRoute'] and quest['event'] == 2980 and quest['bossBattles'] == 1 and quest['forceInputs'] > 0
assert script(39824) == (0x95, 179, 39827, 0) and script(39828) == (0x25, 3134, 25168, 0) and script(39829) == (0x59, 181, 0, 0)
assert script(27203) == (0x49, 65535, 0, 0) and script(27241) == (7, 221, 41075, 0)
assert script(27246) == (0x1f, 261, 0, 0) and script(27412) == (0x59, 184, 0, 0)
assert script(27554) == (0x75, 1, 0, 0) and script(27674) == (0x1f, 269, 0, 0) and script(27675) == (0x59, 183, 0, 0)
assert script(27448) == (0x25, 2961, 25096, 0) and script(27507) == script(27533) == (0x75, 5, 1, 0)
stages = []
for label, event in [('wind-orb-wrong-use-after', 3134), ('mist-before', 3134), ('field-181', 3134), ('wooden-taoist-before', 2980), ('metal-orb-reading', 2980), ('gourd-return-solo', 2980), ('anu-rejoined', 2961), ('rescue-repeat-1', 2961), ('rescue-repeat-2', 2961), ('rescue-repeat-3', 2961)]:
    view, row, info = saved(prefix, label, event)
    if event == 3134:
        assert row[4] == (25168 if label == 'field-181' else 25164) and row[6] == 1
    elif event == 2980:
        assert row[6] == (1 if label == 'wooden-taoist-before' else 0)
    else:
        assert row[4] == 25096 and row[6] == 1
    if label == 'gourd-return-solo':
        assert view['scene'] == 183 and [p['role'] for p in view['party']] == [0]
    if label == 'anu-rejoined':
        assert [p['role'] for p in view['party']] == [4, 0]
    if label.startswith('rescue-repeat-'):
        assert view['scene'] == 183 and [p['role'] for p in view['party']] == [4, 0]
    stages.append({'label': label, 'event': event, 'state': info, 'nativeEvent': list(row), 'partyRoles': [p['role'] for p in view['party']]})
for label, item in [('wind-orb-wrong-use-item-selected', '风灵珠'), ('wind-orb-selected-in-fog', '风灵珠')]:
    view, _, info = saved(prefix, label, 3134)
    assert view['phase'] == 'menu' and view['menu']['kind'] == 5 and item in visible_text(view, True)
    stages.append({'label': label, 'state': info})
view, _, info = saved(prefix, 'wooden-taoist-battle-1', 2980)
assert view['phase'] == 'battle' and any(enemy['object'] == 474 for enemy in view['battle']['enemies'])
stages.append({'label': 'actual-original-wooden-taoist-battle', 'state': info})
view, _, info = saved(prefix, 'village-solo-in-progress', 2980)
assert view['scene'] == 184 and [p['role'] for p in view['party']] == [0]
stages.append({'label': 'village-solo-in-progress', 'state': info})
assert quest['inventoryChecks']['initial'] == quest['inventoryChecks']['wrong-wind-orb-retained'] == quest['inventoryChecks']['wind-orb-retained'] == {'风灵珠': 1, '金罡珠': 0, '寿葫芦': 0}
assert quest['inventoryChecks']['gifts-received'] == quest['finalInventory'] == {'风灵珠': 1, '金罡珠': 1, '寿葫芦': 1}
assert all(quest['inventoryChecks'][f'rescue-repeat-{n}'] == quest['finalInventory'] for n in [1, 2, 3])
records.append({'event': 2980, 'scene': 182, 'originalBattleFormation': 221, 'grants': [261, 269], 'closedVillageDoorEntry': 25096, 'stages': stages, 'result': digest(result_path)})
reference = ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md'
assert all(quote in reference.read_text() for quote in ['灵山山脚东侧有下列物品', '使用风灵珠驱散云雾', '打败木道人救出了村长', '得到了寿葫芦'])
report = {'passed': True, 'scope': 'read-only original native/script cross-check for 12 actual forest pickups across legal chapter roots plus complete original Peach touch quest, actual boss battle, wrong/correct wind-orb use, original temporary solo party and Anu reunion, closed return entrance and no repeated gifts; unreachable fungus classified separately; no native edits', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'reference': {'url': 'https://gamegene.cn/wiki/364', **digest(reference)}}
(ROOT / '.local/pal/verification/forest-peach-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'actualForestEvents': 12, 'originalPeachQuestEvents': 1, 'nativeCases': len(records), 'wrongWindOrbRetained': True, 'originalTemporarySolo': True}, ensure_ascii=False))


