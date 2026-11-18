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


EXPECTED = {175: [2880, 2881, 2882, 2883, 2884, 2885, 2886, 2887], 177: [2918, 2919, 2920, 2921, 2922, 2923, 2924, 2932, 2925, 2926, 2927, 2928, 2929, 2930], 180: [2958, 2959], 185: [3036, 3037, 3038, 3039, 3040, 3041, 3042, 3043, 3031, 3030, 3028, 3029, 3032, 3033, 3034, 3035], 189: [3158, 3159, 3160]}
records = []
for scene, events in EXPECTED.items():
    prefix = f'regional-hidden-v12-{scene}'
    result_path = BASE / f'{prefix}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 12 and result['authorityFixture']
    assert [row['event'] for row in result['claimed']] == events
    for reward in result['claimed']:
        event, sprite = reward['event'], original(reward['event'])[8]
        assert reward['notification'] and reward['repeatSearches'] >= 3 and reward['repeatedFromSavedPose']
        stages, phases = [], {}
        for label in ['unpicked', 'notice', 'collected', 'repeated']:
            view, row, info = saved(prefix, f'reward-{event}-{label}', event)
            assert view['scene'] == scene and row[1:3] == original(event)[1:3] and row[7:10] == original(event)[7:10]
            assert view['phase'] != 'field' if label == 'notice' else view['phase'] == 'field'
            phases[label] = row
            stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
        assert phases['unpicked'][4] == original(event)[4] and phases['unpicked'][6] == original(event)[6]
        if sprite == 10:
            assert phases['unpicked'][10]*phases['unpicked'][9]+phases['unpicked'][11] == reward['closedFrame']
            assert all(phases[label][4] == 36509 and phases[label][10]*phases[label][9]+phases[label][11] == reward['openFrame'] == 1 and phases[label][6] == 2 for label in ['collected', 'repeated'])
        elif sprite in [68, 69, 70]:
            assert script(original(event)[4]) == (0x49, 65535, 0, 0)
            assert all(phases[label][6] == 0 for label in ['notice', 'collected', 'repeated'])
            assert reward['originalHerbDisappearance'] and reward['disappearsAfterPickup']
        elif event == 2932:
            assert script(25980) == (0x14, 1, 0, 0) and script(25984) == (0x1f, 291, 2, 0) and script(25985)[0] == 1
            assert phases['unpicked'][11] == 0 and all(phases[label][11] == 1 for label in ['notice', 'collected', 'repeated'])
            assert phases['collected'][4] == phases['repeated'][4] == 25986 and all(row[6] == 2 for row in phases.values())
            assert reward['originalBananaTreeFrame'] == 1 and not reward['disappearsAfterPickup']
        else:
            assert sprite == 0 and all(row[6] == 1 for row in phases.values())
        details = {}
        if event == 2886:
            assert script(28165)[0] == 1 and script(28168) == (0x1f, 268, 0, 0) and script(28169)[0] == 1 and script(28170)[0] == 0
            assert phases['collected'][4] == 28166 and phases['repeated'][4] == 28170
            for label, entry in [('vessel-notice', 28166), ('vessel-collected', 28170)]:
                view, row, info = saved(prefix, f'reward-{event}-{label}', event)
                assert row[4] == entry and row[6] == 1
                stages.append({'label': label, 'state': info, 'nativeEvent': list(row)})
            follow = reward['followup']
            assert (follow['PC'], follow['Item'], follow['Amount'], follow['afterRepeat']) == (28168, 268, 1, 1)
            assert follow['after']-follow['before'] == 1 and follow['noticeSeen'] and follow['noDuplicateSearches'] >= 3
            details['originalFollowupVessel'] = follow
        if event == 2922:
            assert script(28191) == (0x3e, 0, 0, 0) and script(28193) == (0x1f, 104, 0, 0) and script(28194)[0] == 0
            assert all(row[4] == 28191 and row[6] == 1 for row in phases.values())
            assert reward['originalRenewableHarvest'] and reward['inventoryAfter']-reward['inventoryBefore'] == 4
            assert len(reward['freshSearches']) == 3 and all(row['after']-row['before'] == 1 for row in reward['freshSearches'])
            for number in [1, 2, 3]:
                view, row, info = saved(prefix, f'reward-{event}-renewable-search-{number}', event)
                assert view['phase'] == 'field' and row[4] == 28191 and row[6] == 1
                stages.append({'label': f'renewable-search-{number}', 'state': info, 'nativeEvent': list(row)})
            details['originalRenewableHarvest'] = reward['freshSearches']
        records.append({'scene': scene, 'event': event, 'sprite': sprite, 'stages': stages, 'details': details, 'result': digest(result_path)})

path = BASE / 'regional-hidden-v12-180-mountain-climb-in-progress-state-1.json'
view, stair, stair_info = snapshot(path, 2953)
state = json.loads(path.read_bytes())
packed = base64.b64decode(state['native'], validate=True)
raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
size = struct.unpack_from('<I', raw, 8)[0]
metadata = json.loads(raw[16:16+size])
pc = metadata['view']['pc']
assert view['phase'] == 'script' and view['scene'] == 180 and 25931 <= pc <= 25954 and script(pc)[0] == 0x7f
assert stair[1:3] == (880, 1016) and stair[4] == 25949 and stair[6] == 1
reference = ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md'
assert all(quote in reference.read_text() for quote in ['鼠儿果（无限）', '碧血蚕x7、炼蛊皿', '香蕉x2', '灵山仙芝x2', '10000文钱x2'])
report = {'passed': True, 'scope': 'read-only original scripts and current native pickup continuation for 43 original event points, silkworm followup vessel, renewable fruit fresh searches, persistent picked banana frame, fungus/vine disappearance, two independent money chests and actual mountain stair camera script restoration; no native edits', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'nativeEdits': False, 'records': records, 'mountainStair': {'event': 2953, 'pc': pc, 'state': stair_info}, 'reference': {'url': 'https://gamegene.cn/wiki/364', **digest(reference)}}
(ROOT / '.local/pal/verification/sacred-rewards-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'nativeCases': len(records), 'originalRenewableHarvest': True, 'silkwormFollowup': True, 'bananaFrame': 1, 'mountainStairPC': pc}, ensure_ascii=False))


