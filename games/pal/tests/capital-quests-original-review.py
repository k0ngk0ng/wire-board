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


assert script(39722) == (0x81, 2166, 1, 39726) and script(39723) == (0x25, 2166, 19879, 0)
assert script(39724) == (0x20, 287, 0, 0) and script(19888) == (0xa, 19917, 0, 0)
assert script(19906) == (0x1f, 288, 0, 0) and script(19907)[0] == 1
assert script(39742) == (0x81, 1818, 1, 39745) and script(39743) == (0x25, 1818, 19923, 0)
assert script(39744)[0] == 0
assert script(19925) == (0x49, 1814, 0, 0) and script(19926) == (0x49, 1815, 2, 0)
assert script(39745) == (0x81, 1815, 1, 38780) and script(39746) == (0x25, 1815, 19942, 0)
assert script(39747) == (0x20, 288, 0, 0) and script(19960) == (0x25, 2166, 19966, 0)
records = []
for variant, choice in [('accept', '是'), ('decline', '否')]:
    prefix = f'capital-quests-v1-letter-{variant}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['choice'] == choice and result['completeChain']
    assert result['reconsideredAfterDeclines'] == (variant == 'decline')
    stages = []
    checks = [('letter-delivery-before', 2166, 19873), ('letter-delivery-after', 2166, 19908 if variant == 'accept' else 19879), ('jade-hui-after', 1818, 19938), ('jade-de-after', 1815, 19962), ('xiang-ending-1-before', 2166, 19966), ('xiang-ending-3-after', 2166, 19976)]
    if variant == 'decline':
        checks.extend((f'xiang-declined-repeat-{n}-after', 2166, 19879) for n in [1, 2, 3])
        checks.append(('xiang-reconsidered-after', 2166, 19908))
    for label, event, entry in checks:
        view, row, info = saved(prefix, label, event)
        assert view['phase'] == 'field' and row[4] == entry and row[6] == 2
        assert row[1:3] == original(event)[1:3] and row[8] == original(event)[8]
        stages.append({'label': label, 'event': event, 'entry': entry, 'nativeEvent': list(row), 'state': info})
    for event, expected_state in [(1814, 0), (1815, 2)]:
        _, row, info = saved(prefix, 'jade-hui-after', event)
        assert row[6] == expected_state
        stages.append({'label': 'original-guard-switch', 'event': event, 'nativeEvent': list(row), 'state': info})
    view, _, info = saved(prefix, 'letter-delivery-choice', 2166)
    selected(view, choice)
    stages.append({'label': 'delivery-choice', 'state': info})
    for item, label in [('情书', 'letter-wrong-use'), ('情书', 'letter-delivery'), ('玉佩', 'jade-wrong-use'), ('玉佩', 'jade-hui'), ('玉佩', 'jade-de')]:
        view, _, info = saved(prefix, label+'-item-selected', 2166)
        assert view['phase'] == 'menu' and view['menu']['kind'] == 5 and item in text(view, True)
        stages.append({'label': label+'-item-selected', 'state': info})
    if variant == 'decline':
        for label, expected in [(f'xiang-declined-repeat-{n}', '否') for n in [1, 2, 3]]+[('xiang-reconsidered', '是')]:
            view, _, info = saved(prefix, label+'-choice', 2166)
            selected(view, expected)
            stages.append({'label': label+'-choice', 'state': info})
    assert result['inventoryChecks']['letter-wrong-use-retained'] == {'情书': 1, '玉佩': 0}
    assert result['inventoryChecks']['jade-wrong-use-retained'] == result['inventoryChecks']['jade-hui-retained'] == {'情书': 0, '玉佩': 1}
    assert result['inventoryChecks']['jade-returned'] == result['finalInventory'] == {'情书': 0, '玉佩': 0}
    records.append({'event': 2166, 'variant': variant, 'stages': stages, 'result': digest(result_path)})

prefix = 'capital-quests-v1-bundle'
result_path = BASE / f'{prefix}-result-1.json'
result = json.loads(result_path.read_bytes())
assert result['passed'] and result['sourceCorpseEvent'] == 1845 and result['learnedMagic'] == 384
assert script(39715) == (0x81, 1783, 1, 39718) and script(39718) == (0x81, 2333, 1, 38780)
assert script(39719) == (0x25, 2333, 16538, 0) and script(39720) == (0x20, 271, 0, 0)
assert script(16609) == (0x1f, 270, 0, 0) and script(16622) == (0x55, 384, 1, 0) and script(16623)[0] == 1
stages = []
for label, event, entry in [('corpse-bundle-before', 1845, 15780), ('corpse-bundle-after', 1845, 15804), ('bundle-delivery-before', 2333, 16524), ('bundle-delivery-after', 2333, 16624), ('song-repeat-3-after', 2333, 16624)]:
    view, row, info = saved(prefix, label, event)
    assert view['phase'] == 'field' and row[4] == entry and row[6] == 2 and row[8] == original(event)[8]
    assert row[1:3] == original(event)[1:3]
    stages.append({'label': label, 'event': event, 'nativeEvent': list(row), 'state': info})
for label, token in [('gourd-grant-reading', '获得紫金葫芦'), ('gourd-spell-reading', '得到灵葫咒')]:
    view, _, info = saved(prefix, label, 2333)
    visible = text(view, include_black=True)+(view.get('dialogue') or {}).get('text', '')
    assert token in visible and view['phase'] != 'field'
    stages.append({'label': label, 'state': info})
for label in ['learned', 'repeated']:
    view, _, info = saved(prefix, label+'-gourd-spell-visible', 2333)
    assert view['phase'] == 'menu' and view['menu']['kind'] == 10 and view['menu']['role'] == 0 and '灵葫咒' in text(view)
    stages.append({'label': label+'-gourd-spell-visible', 'state': info})
view, _, info = saved(prefix, 'initial-spell-menu', 2333)
assert view['menu']['role'] == 0 and '灵葫咒' not in text(view)
stages.append({'label': 'initial-spell-menu', 'state': info})
assert result['inventoryChecks']['corpse-bundle-collected'] == result['inventoryChecks']['bundle-wrong-use-retained'] == {'布包': 1, '紫金葫芦': 0}
assert result['inventoryChecks']['gourd-received'] == result['finalInventory'] == {'布包': 0, '紫金葫芦': 1}
records.append({'event': 2333, 'sourceCorpseEvent': 1845, 'learnedMagic': 384, 'stages': stages, 'result': digest(result_path)})
reference = ROOT / '.local/pal/hidden-items-research/capital-text-guide.md'
assert all(quote in reference.read_text() for quote in ['将情书（使用）交给阿香', '将玉佩（使用）交给守卫阿德', '原来她喜欢的是少爷', '（使用）交给宋氏', '学会了灵葫咒'])
report = {'passed': True, 'scope': 'read-only original item-use scripts, current native entries, guard activation switch, exact menu selections, original item menu inventory checks, gourd gift and real Li learned spell menu after restoration; direct and declined/reconsidered letter branches, no native edits', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'reference': {'url': 'https://gamegene.cn/wiki/362', **digest(reference)}}
(ROOT / '.local/pal/verification/capital-quests-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'nativeCases': len(records), 'conditionalEvents': [2166, 2333], 'originalGuardSwitch': True, 'learnedMagic': 384}, ensure_ascii=False))


