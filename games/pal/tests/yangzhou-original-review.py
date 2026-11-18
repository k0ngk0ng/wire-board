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

def stable(row):
    return tuple(row[index] for index in list(range(4))+list(range(5, 6))+list(range(7, 11))+[12, 15])

records = []
for scene, event, source in [(87, 1631, 'yangzhou-step-22-state-1.json'), (93, 1704, 'yangzhou-step-47-state-1.json'), (97, 1779, 'yangzhou-step-24-state-1.json')]:
    source_view, source_event, source_digest = snapshot(BASE / source, event)
    stages = []
    rows = {}
    for label in ['unpicked', 'notice', 'collected', 'repeated']:
        path = BASE / f'regional-hidden-v9-{scene}-reward-{event}-{label}-state-1.json'
        view, row, info = snapshot(path, event)
        assert view['scene'] == scene and stable(row) == stable(original(event))
        rows[label] = row
        stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
    result_path = BASE / f'regional-hidden-v9-{scene}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 9 and result['scene'] == scene
    assert result['proof']['Battles'] == result['proof']['Wins'] and result['proof']['ScriptedFinishes'] == 0
    report_row = next(row for row in result['emptyChestAudits'] if row['event'] == event) if event == 1704 else next(row for row in result['claimed'] if row['event'] == event)
    assert report_row['repeatSearches'] >= 3 and report_row['repeatedFromSavedPose']
    for label in rows:
        assert result['proof']['Saved'][f'reward-{event}-{label}']
    if event == 1631:
        assert original(event)[6] == 0 and source_event[6] == rows['unpicked'][6] == 2
        assert script(13628) == (0x49, 1631, 2, 0)
        assert source_event[4] == rows['unpicked'][4] == rows['collected'][4] == rows['repeated'][4] == 14041
        assert rows['notice'][6] == rows['collected'][6] == rows['repeated'][6] == 0
        assert script(14041) == (0x49, 0xffff, 0, 0) and script(14042)[0] == 5 and script(14045) == (0x1f, 271, 0, 0)
        assert report_row['originalQuestBundleDisappearance'] and report_row['disappearsAfterPickup']
        assert [(g['Item'], g['Amount']) for g in report_row['grants']] == [(271, 1)]
    elif event == 1704:
        assert source_event[4] == rows['unpicked'][4] == 37479 and rows['unpicked'][11] == 0
        assert rows['notice'][4] == 37479 and rows['notice'][11] == 1
        assert rows['collected'][4] == rows['repeated'][4] == 36509
        assert rows['collected'][11] == rows['repeated'][11] == 1
        assert all(row[6] == 2 for row in rows.values())
        assert script(37482) == (0x1f, 0, 0, 0) and script(37483) == (2, 36509, 0, 0)
        assert report_row['originalZeroItemChest'] and not report_row['countedCandidate'] and report_row['grants'] == []
        assert report_row['inventoryBefore'] == report_row['inventoryAfter']
        assert report_row['inventoryBefore'].get('紫菁玉蓉膏', 0) == report_row['inventoryAfter'].get('紫菁玉蓉膏', 0) == 0
        assert report_row['cashBefore'] == report_row['cashAfter']
    else:
        assert original(event)[6] == source_event[6] == rows['unpicked'][6] == 1
        assert rows['collected'][6] == rows['repeated'][6] == 0
        assert script(38272) == (0x49, 0xffff, 0, 0)
        assert source_event[4] == 38268 and all(row[4] == 38268 for row in rows.values())
        assert [(g['Item'], g['Amount']) for g in report_row['grants']] == [(84, 1)]
        assert report_row['invisible'] and not report_row['disappearsAfterPickup']
    records.append({'event': event, 'scene': scene, 'position': list(original(event)[1:3]), 'originalState': original(event)[6], 'sourceState': source_event[6], 'sourceScene': source_view['scene'], 'source': source_digest, 'stages': stages, 'result': digest(result_path), 'actuallyAwarded': event != 1704, 'candidateCounted': event != 1704})

message_path = ROOT / '.local/pal/original-98/m.msg'
message_data = message_path.read_bytes()
a, b = struct.unpack_from('<II', SSS, OFFSETS[3]+12347*4)
message = message_data[a:b].decode('gbk')
assert message == '获得紫青玉蓉膏'
reference = ROOT / '.local/pal/hidden-items-research/yangzhou-text-guide.md'
quote = '其中有个宝箱虽显示获得紫青玉蓉膏，但实际上并未获得任何物品（某些98版已修正此错误）。'
assert quote in reference.read_text()
catalog = json.loads((ROOT / '.local/pal/verification/hidden-search-candidates.json').read_bytes())
assert catalog['passed'] and catalog['sssSHA256'] == SSS_HASH
assert 1704 not in {row['Event'] for row in catalog['candidates']}
report = {'passed': True, 'scope': 'read-only actual same-core original event table across unpicked/notice/collected/repeated snapshots; original quest bundle activation and disappearance, zero-item chest prompt/opening/next-script with unchanged checked inventory and cash, actual Windows hidden garlic state closure; original resources unchanged, zero-item chest is not a successful item grant or progress candidate', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'checkedStableFields': list(range(4))+[5]+list(range(7, 11))+[12, 15], 'ignoredResourceCacheFields': [13, 14], 'records': records, 'zeroItemMessage': message, 'messageSource': digest(message_path), 'reference': {'url': 'https://gamegene.cn/wiki/361', 'quote': quote, **digest(reference)}}
(ROOT / '.local/pal/verification/yangzhou-original-rewards-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'actualPickupsReviewed': 2, 'zeroItemChestPreserved': True, 'zeroItemCandidateCounted': False}, ensure_ascii=False))
