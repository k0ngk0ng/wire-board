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
assert hashlib.sha256(SSS).hexdigest() == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
OFFSETS = struct.unpack_from('<6I', SSS)
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE

def read_events(name):
    path = BASE / name
    data = path.read_bytes()
    state = json.loads(data)
    packed = base64.b64decode(state['native'], validate=True)
    raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
    assert raw[:8] == b'PALMEM01'
    size, memory_size = struct.unpack_from('<II', raw, 8)
    metadata = json.loads(raw[16:16+size])
    memory = raw[16+size:]
    assert len(memory) == memory_size <= 48*1024*1024 and metadata['buildID'] == CORE
    assert hashlib.sha256(memory+struct.pack('<I', metadata['stackPointer'])).hexdigest() == metadata['memoryID']
    original = SSS[OFFSETS[0]+234*32:OFFSETS[0]+235*32]
    needle, candidates = original[:8], []
    at = memory.find(needle)
    while at >= 0:
        start = at-234*32
        if start >= OFFSETS[0] and start+5077*32 <= len(memory) and memory[start-OFFSETS[0]:start] != SSS[:OFFSETS[0]]:
            row = struct.unpack_from('<hHHhHHh9H', memory, at)
            if row[4:10] == struct.unpack('<hHHhHHh9H', original)[4:10]:
                anchors_match = True
                for event in [243, 261, 262, 264]:
                    actual = struct.unpack_from('<hHHhHHh9H', memory, start+(event-1)*32)
                    expected = struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(event-1)*32)
                    anchors_match = anchors_match and actual[1:3] == expected[1:3] and actual[8] == expected[8]
                if anchors_match:
                    candidates.append(start)
        at = memory.find(needle, at+1)
    assert len(candidates) == 1, candidates
    start = candidates[0]
    rows = {event: struct.unpack_from('<hHHhHHh9H', memory, start+(event-1)*32) for event in [235, 238, 239, 240, 241, 242, 243, 260, 261, 262, 264]}
    return state['public'], rows, {'path': str(path.relative_to(ROOT)), 'sha256': hashlib.sha256(data).hexdigest()}

before_view, before, before_source = read_events('hidden-island-entry-state-1.json')
gate_view, gate, gate_source = read_events('regional-hidden-v4-18-lotus-K-gate-opened-state-1.json')
picked_view, picked, picked_source = read_events('regional-hidden-v4-18-reward-264-collected-state-1.json')
final_view, final, final_source = read_events('regional-hidden-v4-18-reward-264-repeated-state-1.json')
assert before_view['scene'] == 15 and gate_view['scene'] == picked_view['scene'] == final_view['scene'] == 18
assert before[260][1:3] == (1296, 584) and gate[260][1:3] == (1280, 560)
assert before[261][6] == 2 and gate[261][6] == 0 and before[262][6] == 0 and gate[262][6] == 2
for pc, expected in [(4187, (73, 261, 0, 0)), (4188, (73, 261, 0, 0)), (4189, (73, 262, 2, 0))]:
    assert struct.unpack_from('<4H', SSS, OFFSETS[4]+pc*8) == expected
for event in [238, 239, 240, 241, 242, 243]:
    assert before[event][6] == gate[event][6] == picked[event][6] == final[event][6] == 2
for rows in [gate, picked, final]:
    assert rows[264][1:3] == before[264][1:3] == (1408, 432)
    assert rows[264][6:11] == before[264][6:11]
assert before[264][11] == 0 and picked[264][11] == final[264][11] == 1
assert gate[264][4] == 5167 and picked[264][4] == final[264][4] == 36509
result_path = BASE / 'regional-hidden-v4-18-result-1.json'
result = json.loads(result_path.read_text())
assert result['passed'] and result['verificationVersion'] == 4
chest = next(row for row in result['claimed'] if row['event'] == 264)
assert chest['repeatSearches'] == 3 and chest['notification'] and chest['openFrame'] == 1
assert [(g['Item'], g['Amount'], g['Name']) for g in chest['grants']] == [(67, 1, '风灵符'), (68, 1, '雷灵符'), (69, 1, '水灵符'), (70, 1, '火灵符'), (71, 1, '土灵符')]
assert all(result['inventory'][name] == 1 for name in ['风灵符', '雷灵符', '水灵符', '火灵符', '土灵符'])
reference = ROOT / '.local/pal/hidden-items-research/island-text-guide.md'
reference_image = ROOT / '.local/pal/hidden-items-research/island-lotus-map-reference.png'
assert 'K，拿灵符' in reference.read_text() and reference_image.is_file()
report = {'passed': True, 'scope': 'read-only actual same-core snapshots and original SSS prove ordinary K-lotus trigger removed its original blocker, without moving the chest or changing the map; linked actual five-talisman pickup and saved no-duplication proof; no statue was broken', 'nativeBuildID': CORE, 'nativeEdits': False, 'normalPickupVerified': True, 'event': 264, 'position': [1408, 432], 'lotus': {'event': 260, 'before': list(before[260][1:3]), 'after': list(gate[260][1:3])}, 'gate': {'event': 261, 'beforeState': before[261][6], 'afterState': gate[261][6], 'replacementEvent': 262, 'replacementBeforeState': before[262][6], 'replacementAfterState': gate[262][6]}, 'statuesIntact': [238, 239, 240, 241, 242, 243], 'snapshots': [before_source, gate_source, picked_source, final_source], 'result': {'path': str(result_path.relative_to(ROOT)), 'sha256': hashlib.sha256(result_path.read_bytes()).hexdigest()}, 'reference': {'url': 'https://gamegene.cn/wiki/358', 'quote': '(4)G-H-I-J，打石像；K，拿灵符', 'text': str(reference.relative_to(ROOT)), 'sha256': hashlib.sha256(reference.read_bytes()).hexdigest(), 'imageURL': 'https://img.gamegene.cn/wiki/358-11.png', 'image': str(reference_image.relative_to(ROOT)), 'imageSHA256': hashlib.sha256(reference_image.read_bytes()).hexdigest()}}
out = ROOT / '.local/pal/verification/lotus-route-review.json'
out.write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'normalPickupVerified': True, 'event': 264, 'statuesIntact': 6, 'gate': report['gate']}, ensure_ascii=False))
