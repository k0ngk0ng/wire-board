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


records = []
for scene, event in [(101, 1818), (109, 2035), (138, 2336)]:
    prefix = f'regional-hidden-v11-{scene}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 11 and result['authorityFixture']
    reward = result['claimed'][0]
    assert reward['event'] == event and reward['notification'] and reward['repeatSearches'] >= 3
    phases, stages = {}, []
    for label in ['unpicked', 'notice', 'collected', 'repeated']:
        view, row, info = saved(prefix, f'reward-{event}-{label}', event)
        assert view['scene'] == scene and row[1:3] == original(event)[1:3] and row[7:10] == original(event)[7:10]
        assert result['proof']['Saved'][f'reward-{event}-{label}']
        phases[label] = row
        stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
    if event == 1818:
        assert phases['unpicked'][4] == phases['notice'][4] == 19833
        assert phases['collected'][4] == phases['repeated'][4] == 19852
        assert all(row[6] == 2 and row[8] == 134 for row in phases.values())
        assert script(19836) == (0xa, 19829, 0, 0) and script(19843) == (0x1f, 287, 0, 0) and script(19851)[0] == 1
        view, _, info = saved(prefix, 'reward-1818-decision-1', event)
        selected(view, '是')
        stages.append({'label': 'decision', 'state': info})
    elif event == 2035:
        assert phases['unpicked'][6] == 2 and all(phases[label][6] == 0 for label in ['notice', 'collected', 'repeated'])
        assert all(row[4] == 21569 and row[8] == 308 for row in phases.values())
        assert script(21569) == (0x49, 65535, 0, 0) and script(21573) == (0x1f, 109, 0, 0)
    else:
        assert phases['unpicked'][6] == phases['notice'][6] == 1
        assert phases['collected'][6] == phases['repeated'][6] == 0
        assert all(row[4] == 38454 and row[8] == 0 for row in phases.values())
        assert script(38457) == (0x1f, 255, 0, 0) and script(38458) == (0x49, 65535, 0, 0)
    records.append({'scene': scene, 'event': event, 'position': list(original(event)[1:3]), 'stages': stages, 'result': digest(result_path)})

prefix = 'capital-funding-v1'
result_path = BASE / f'{prefix}-result-1.json'
funding = json.loads(result_path.read_bytes())
assert funding['passed'] and funding['grants'] == 3 and funding['shopQuantity'] == 7
assert script(19258) == script(19278) == script(19297) == (0x1e, 30000, 0, 0)
assert script(19266) == (0x1e, 35536, 19269, 0) and script(19286) == (0x1e, 35536, 19289, 0)
assert script(19267) == script(19287) == (0x1e, 30000, 0, 0)
assert script(19272) == (0xa, 19261, 0, 0) and script(19292) == (0xa, 19281, 0, 0)
assert script(19265)[0] == script(19285)[0] == 1 and script(19301)[0] == 8 and script(19305)[0] == 0
assert script(16339) == (0x26, 13, 0, 0)
stages = []
for step in funding['steps']:
    label = step['label']
    if label.startswith('spend-'):
        for buy in step['purchases']:
            item_view, shop, info = saved(prefix, buy['label']+'-item-selected', 2324)
            assert item_view['scene'] == 136 and shop[4] == 16334 and shop[6] == 2
            selected(item_view, '金童剑')
            assert item_view['menu']['selected'] == 183 and item_view['cash'] == buy['cashBefore']
            choice_view, _, choice_info = saved(prefix, buy['label']+'-choice', 2324)
            selected(choice_view, '是')
            after_view, _, after_info = saved(prefix, buy['label']+'-after', 2324)
            assert after_view['cash'] == buy['cashAfter'] == buy['cashBefore']-12000
            stages.append({'label': buy['label'], 'selectedItem': info, 'confirmation': choice_info, 'after': after_info, 'cashBefore': buy['cashBefore'], 'cashAfter': buy['cashAfter']})
        continue
    before_view, before_row, before_info = saved(prefix, label+'-before', 2003)
    after_view, after_row, after_info = saved(prefix, label+'-after', 2003)
    assert before_view['scene'] == after_view['scene'] == 109 and before_view['phase'] == after_view['phase'] == 'field'
    assert before_row[6] == after_row[6] == 2 and before_row[8] == after_row[8] == 81
    assert before_view['cash'] == step['cashBefore'] and after_view['cash'] == step['cashAfter']
    assert after_view['cash']-before_view['cash'] == step['delta']
    grant = {'grant-1': (19253, 19266), 'grant-2': (19266, 19286), 'grant-3': (19286, 19302)}.get(label)
    if grant:
        assert (before_row[4], after_row[4]) == grant and step['delta'] == 30000
        reading_view, _, reading_info = saved(prefix, label+'-reading', 2003)
        assert reading_view['cash'] == step['cashAfter'] and reading_view['phase'] != 'field'
    else:
        assert before_row[4] == after_row[4] and step['delta'] == 0
        if label.startswith('funding-exhausted-'):
            assert after_row[4] == 19302 and after_view['cash'] < 30000
    entry = {'label': label, 'before': before_info, 'after': after_info, 'entryBefore': before_row[4], 'entryAfter': after_row[4], 'cashBefore': step['cashBefore'], 'cashAfter': step['cashAfter']}
    if step['choice']:
        view, _, info = saved(prefix, label+'-choice', 2003)
        selected(view, step['choice'])
        entry['choice'] = info
    if grant:
        entry['reading'] = reading_info
    stages.append(entry)
records.append({'event': 2003, 'grants': 3, 'maxFunding': 90000, 'shopQuantity': 7, 'shopSpending': 84000, 'stages': stages, 'result': digest(result_path)})

for event, scene, suffix, price, item, pc, entry in [(2086, 112, 'sugar', 30, 80, 17182, 17179), (2225, 128, 'wine', 100, 86, 16225, 16220)]:
    prefix = f'capital-merchant-v1-{suffix}'
    result_path = BASE / f'{prefix}-result-1.json'
    value = json.loads(result_path.read_bytes())
    assert value['passed'] and value['event'] == event and value['price'] == price and value['purchases'] == 2
    assert script(pc)[0:2] == (0x1e, 65536-price) and script(pc+1) == (0x1f, item, 0, 0)
    stages = []
    for step in value['steps']:
        entry_stages = []
        for label in ['before', 'choice', 'after']:
            view, row, info = saved(prefix, step['label']+'-'+label, event)
            assert row[1:3] == original(event)[1:3] and row[4] == entry and row[6] == 2 and row[7:10] == original(event)[7:10]
            assert view['scene'] == scene and view['cash'] == step['cashAfter' if label == 'after' else 'cashBefore']
            if label == 'choice':
                selected(view, step['choice'])
            else:
                assert view['phase'] == 'field'
            entry_stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
        stages.append({'label': step['label'], 'choice': step['choice'], 'delta': step['delta'], 'states': entry_stages})
    records.append({'event': event, 'scene': scene, 'price': price, 'item': item, 'repeatedPurchaseAllowed': True, 'stages': stages, 'result': digest(result_path)})

references = []
for filename, url, quotes in [('capital-text-guide.md', 'https://gamegene.cn/wiki/362', ['帮他送情书', '必须从北边小路走过来捡取', '古董商家有银针', '钱低于30000时', '最后一次给钱', '酒100']), ('yangzhou-text-guide.md', 'https://gamegene.cn/wiki/361', ['糖葫芦每个30文钱'])]:
    path = ROOT / '.local/pal/hidden-items-research' / filename
    assert all(quote in path.read_text() for quote in quotes)
    references.append({'url': url, **digest(path)})
report = {'passed': True, 'scope': 'read-only original event/script and current-core native continuation checks for three one-time pickups, capped funding, real weapon purchases, repeatable paid merchants and original confirmation selections; no edits to native state', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'references': references}
(ROOT / '.local/pal/verification/capital-side-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'nativeCases': len(records), 'maxFunding': 90000, 'actualWeaponSpending': 84000, 'paidMerchantPrices': [30, 100]}, ensure_ascii=False))
