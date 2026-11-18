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

def script(pc):
    return struct.unpack_from('<4H', SSS, OFFSETS[4]+pc*8)

def read_stage(prefix, event, label):
    return snapshot(BASE / f'{prefix}-reward-{event}-{label}-state-1.json', event)

records = []
for scene, event, suffix in [(102, 1845, ''), (103, 1883, ''), (103, 1883, '-renegotiated'), (104, 1911, '')]:
    prefix = f'regional-hidden-v10-{scene}{suffix}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 10
    assert result['candidate'] == 'audio' and result['developmentProfile'] == 'hp-9999' and result['authorityFixture']
    reward = next(row for row in result['claimed'] if row['event'] == event)
    source_path = BASE / (result['sourceCheckpoint'][:-5]+'-1.json')
    source_view, source_event, source_digest = snapshot(source_path, event)
    assert source_digest['sha256'] == result['sourceSHA256']
    phases = {}
    stages = []
    for label in ['unpicked', 'notice', 'collected', 'repeated']:
        view, row, info = read_stage(prefix, event, label)
        assert view['scene'] == scene and row[1:3] == original(event)[1:3] and row[7:10] == original(event)[7:10]
        assert result['proof']['Saved'][f'reward-{event}-{label}']
        phases[label] = (view, row)
        stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
    assert reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
    details = {}
    if event == 1845:
        assert source_event[6] == 2 and source_event[4] == 15780
        assert all(row[6] == 2 and row[8] == 387 and row[11] == 0 for view, row in phases.values())
        assert phases['unpicked'][1][4] == phases['notice'][1][4] == 15780
        assert phases['collected'][1][4] == phases['repeated'][1][4] == 15804
        assert script(15793) == (0x1f, 271, 0, 0) and script(15803)[0] == 1
        assert reward['originalMerchantCorpsePreserved'] and not reward['disappearsAfterPickup']
        assert [(g['Item'], g['Amount']) for g in reward['grants']] == [(271, 1)]
    elif event == 1883:
        assert source_event[4] == phases['unpicked'][1][4] == 21184 and source_event[6] == 2
        assert phases['collected'][1][6] == phases['repeated'][1][6] == 0
        expected_cash, expected_follower, expected_script = (1500, 83, 21230) if suffix else (1000, 82, 21184)
        assert phases['collected'][1][4] == phases['repeated'][1][4] == expected_script
        assert reward['originalEscortContract'] and reward['escortFollowerSprite'] == expected_follower
        assert reward['cashAfter']-reward['cashBefore'] == expected_cash
        for label in ['collected', 'repeated']:
            field = phases[label][0]['field']
            assert field['followers'] == 1 and [a[2] for a in field['actors'] if a[0] == 2] == [expected_follower]
        decision_view, _, info = read_stage(prefix, event, 'decision-1')
        menu = decision_view['menu']
        selected = menu['items'][menu['selected']]['label']
        assert selected == ('否' if suffix else '是') and reward['decisions'] == [selected]
        stages.append({'label': 'decision-1', 'state': info, 'selected': selected})
        assert script(21207) == (0xa, 21220, 0, 0)
        assert script(21216) == (0x1e, 1000, 0, 0) and script(21218) == (0x98, 82, 0, 0)
        assert script(21261) == (0x1e, 1500, 0, 0) and script(21263) == (0x98, 83, 0, 0)
        if suffix:
            for index, expected_entry in [(1, 21226), (2, 21230)]:
                view, row, info = read_stage(prefix, event, f'declined-field-{index}')
                assert view['phase'] == 'field' and row[6] == 2 and row[4] == expected_entry
                stages.append({'label': f'declined-field-{index}', 'state': info, 'nativeEntry': row[4]})
            assert reward['searchesBeforeReward'] == 3
        else:
            assert reward['searchesBeforeReward'] == 1
            final_view, warning, info = snapshot(BASE / f'{prefix}-state-1.json', 1884)
            assert warning[6] == 0 and final_view['field']['followers'] == 0
            details['originalSwordSaintWarningAndFollowerDeparture'] = info
        details.update({'cash': expected_cash, 'followerSprite': expected_follower, 'decision': selected, 'searchesBeforeReward': reward['searchesBeforeReward']})
    else:
        assert source_event[6] == 2 and source_event[4] == 21552
        assert phases['unpicked'][1][4] == phases['notice'][1][4] == 21552
        assert phases['collected'][1][4] == phases['repeated'][1][4] == 21558
        assert all(row[6] == 2 for view, row in phases.values())
        assert script(21552) == (0x16, 1910, 0, 2) and script(21556) == (0x1f, 262, 0, 0)
        _, before, before_info = snapshot(source_path, 1910)
        _, after, after_info = snapshot(BASE / f'{prefix}-reward-1911-repeated-state-1.json', 1910)
        assert before[8] == after[8] == 485 and before[11] == 1 and after[11] == 2
        assert reward['originalPoisonCorpseFrame'] == 2
        details['toadBody'] = {'beforeFrame': before[11], 'afterFrame': after[11], 'before': before_info, 'after': after_info}
    records.append({'event': event, 'scene': scene, 'variant': 'renegotiated' if suffix else 'direct', 'position': list(original(event)[1:3]), 'source': source_digest, 'sourceScript': source_event[4], 'sourceState': source_event[6], 'stages': stages, 'details': details, 'result': digest(result_path)})

reference = ROOT / '.local/pal/hidden-items-research/yangzhou-text-guide.md'
for quote in ['得定金1000文钱', '得定金1500文钱', '调查古董商的尸体，得到布包', '调查蛤蟆精，从肚子可取得五毒珠']:
    assert quote in reference.read_text()
report = {'passed': True, 'scope': 'read-only current-core native records and exact original script instructions: merchant corpse persists, both scholar money/choice/follower variants with rejection continuation and original departure, orb corpse frame and next entry; no native edits, no extra count for the alternate offer or reverified orb', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'reference': {'url': 'https://gamegene.cn/wiki/361', **digest(reference)}}
(ROOT / '.local/pal/verification/toad-original-rewards-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'nativeCases': len(records), 'scholarPrices': [1000, 1500], 'merchantCorpsePreserved': True, 'orbCorpseFrame': 2}, ensure_ascii=False))
