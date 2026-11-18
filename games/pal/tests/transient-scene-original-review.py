import base64
from compression import zstd
import gzip
import hashlib
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999/scopes/transient-scene-v1'
CORE = '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
SSS_PATH = ROOT / '.local/pal/original-98/sss.mkf'
SSS = SSS_PATH.read_bytes()
assert hashlib.sha256(SSS).hexdigest() == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
OFFSETS = struct.unpack_from('<6I', SSS)
SCRIPTS = [struct.unpack_from('<4H', SSS, at) for at in range(OFFSETS[4], OFFSETS[5], 8)]


def digest(path):
    data = path.read_bytes()
    return {'path': str(path.relative_to(ROOT)), 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}


def original(event):
    return struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(event-1)*32)


def scene(number):
    return struct.unpack_from('<4H', SSS, OFFSETS[1]+(number-1)*8)


def snapshot(path):
    state = json.loads(path.read_bytes())
    packed = base64.b64decode(state['native'], validate=True)
    raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
    assert raw[:8] == b'PALMEM01'
    size, memory_size = struct.unpack_from('<II', raw, 8)
    meta, memory = json.loads(raw[16:16+size]), raw[16+size:]
    assert len(memory) == memory_size <= 48*1024*1024 and meta['buildID'] == CORE
    assert hashlib.sha256(memory+struct.pack('<I', meta['stackPointer'])).hexdigest() == meta['memoryID']
    needle = SSS[OFFSETS[0]+234*32:OFFSETS[0]+234*32+8]
    starts, at = [], memory.find(needle)
    while at >= 0:
        start = at-234*32
        if start >= OFFSETS[0] and start+5077*32 <= len(memory) and memory[start-OFFSETS[0]:start] != SSS[:OFFSETS[0]]:
            if all(struct.unpack_from('<hHHhHHh9H', memory, start+(anchor-1)*32)[1:3] == original(anchor)[1:3] for anchor in [235, 243, 261, 262, 264]):
                starts.append(start)
        at = memory.find(needle, at+1)
    assert len(starts) == 1, (path.name, starts)
    events = {}
    for event in [2932, 2947]:
        row = struct.unpack_from('<hHHhHHh9H', memory, starts[0]+(event-1)*32)
        assert row[:13]+row[15:] == original(event)[:13]+original(event)[15:], (path.name, event, row)
        events[str(event)] = list(row)
    return {'state': digest(path), 'scene': state['public']['scene'], 'phase': state['public']['phase'], 'pc': meta['view']['pc'], 'events': events}


entrances = [pc for pc, row in enumerate(SCRIPTS) if row[:2] == (0x59, 178)]
assert entrances == [27947]
assert not [(pc, row) for pc, row in enumerate(SCRIPTS) if row[:2] == (0x6d, 178)]
assert scene(175)[1] == 27775 and scene(178) == (201, 27955, 0, 2944) and scene(177)[0] == 201
assert SCRIPTS[27936] == (0x59, 194, 0, 0) and SCRIPTS[27937] == (1, 0, 0, 0)
assert SCRIPTS[27947] == (0x59, 178, 0, 0) and SCRIPTS[27949] == (1, 0, 0, 0)
entry = [{'pc': pc, 'instruction': list(SCRIPTS[pc])} for pc in range(27955, 28014)]
safe = {0x46, 0x9, 0x16, 0x5, 0x3c, 0x3d, 0xffff, 0x8e, 0x24, 0x50, 0x49}
assert all(row['instruction'][0] in safe for row in entry[:-2])
assert all(row['instruction'][2] == 0 for row in entry if row['instruction'][0] == 9)
assert SCRIPTS[28012] == (0x59, 175, 0, 0) and SCRIPTS[28013] == (1, 0, 0, 0)
autos = [{'root': root, 'instructions': [list(SCRIPTS[pc]) for pc in range(root, root+3)]} for root in [28020, 28023]]
assert autos[0]['instructions'] == [[0x11, 44, 44, 1], [0xf, 1, 0, 0], [0, 0, 0, 0]]
assert autos[1]['instructions'] == [[0xf, 0, 0, 0], [0x7c, 38, 49, 0], [0, 0, 0, 0]]
assert [(row['pc'], row['instruction'][1:3]) for row in entry if row['instruction'][0] == 0x24] == [(28005, [2946, 28023])]
assert original(2947) == original(2932) and original(2947)[1:3] == (1264, 712)
assert original(2947)[4:9] == (25980, 0, 2, 1, 519) and SCRIPTS[25984] == (0x1f, 291, 2, 0)
result_path = BASE / 'transient-scene-v1-result-1.json'
result = json.loads(result_path.read_bytes())
assert result['passed'] and result['diagnosticOnly'] and not result['readyForProgress'] and not result['normalPickupVerified']
assert len(result['observed']) > 0 and {row['phase'] for row in result['observed']} <= {'script', 'dialogue'}
assert len(result['directionProbes']) == 4 and all(row['inputRejected'] and row['timerCalls'] == 8 for row in result['directionProbes'])
stages = []
for label in result['proof']['Saved']:
    row = snapshot(BASE / f'transient-scene-v1-{label}-state-1.json')
    row['label'] = label
    if label == 'entered' or label == 'first-dialogue' or label.startswith('direction-'):
        assert row['scene'] == 178 and row['phase'] in ['script', 'dialogue']
    stages.append(row)
assert len(stages) == result['proof']['Saves'] and result['finalScene'] == 175 and result['bananaFinalAmount'] == 0
roots_path = ROOT / 'games/pal/tests/campaign-roots.json'
roots = json.loads(roots_path.read_bytes())['candidates']['audio']
root = roots['roots'][result['sourceCheckpoint']]
source = BASE / 'campaign-tower-bottom-state-1.json'
parent = BASE / 'campaign-tower-bottom-1.json'
assert digest(source)['sha256'] == root['sha256'] == result['sourceSHA256'] and digest(parent)['sha256'] == root['reportSHA256']
assert json.loads(parent.read_bytes())['passed'] and not json.loads(parent.read_bytes())['resumedTestRoute']
assert digest(ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm')['sha256'] == roots['coreSHA256'] == CORE
guide = ROOT / '.local/pal/hidden-items-research/sacred-tree-text-guide.md'
assert '[物品]圣姑家外有下列物品（香蕉为过关物品，一定要拿）' in guide.read_text()
report = {'passed': True, 'diagnosticOnly': True, 'readyForProgress': False, 'event': 2947, 'scene': 178, 'map': 201, 'nativeBuildID': CORE, 'nativeEdits': False, 'normalPickupVerified': False, 'classification': 'scripted-visit-confirmed-independent-attribution-pending', 'independentUnreachableAttribution': False, 'allLiteralSceneEntrances': entrances, 'sceneEntryScriptWrites': [], 'entryAlwaysLeavesBeforeReturning': True, 'entryInstructions': entry, 'originalAutoscripts': autos, 'distinctDuplicateEvent': 2932, 'originalRecord': list(original(2947)), 'nativeStages': stages, 'result': digest(result_path), 'parentState': digest(source), 'parentReport': digest(parent), 'reference': {'url': 'https://gamegene.cn/wiki/364', **digest(guide), 'scope': 'ordinary accessible exterior banana described; does not independently classify duplicate transient event 2947'}, 'sss': digest(SSS_PATH), 'referenceEngine': [digest(ROOT / ('games/pal/native/upstream/'+name)) for name in ['play.c', 'script.c']]}
(VERIFY / 'transient-scene-original-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'diagnosticOnly': True, 'readyForProgress': False, 'event': 2947, 'nativeStages': len(stages), 'progressIncrement': 0}))
