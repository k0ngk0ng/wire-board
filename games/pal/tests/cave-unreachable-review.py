import base64
from collections import deque
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
SCRIPTS = [struct.unpack_from('<4H', SSS, at) for at in range(OFFSETS[4], OFFSETS[5], 8)]
CATALOG_PATH = ROOT / '.local/pal/verification/hidden-search-candidates.json'
CATALOG = json.loads(CATALOG_PATH.read_text())
assert CATALOG['passed'] and CATALOG['sssSHA256'] == SSS_HASH
CANDIDATES = {row['Event']: row for row in CATALOG['candidates']}
ROOTS = json.loads((ROOT / 'games/pal/tests/campaign-roots.json').read_text())['candidates']['audio']
assert ROOTS['coreSHA256'] == CORE and ROOTS['profile'] == 'hp-9999'
REFERENCE = ROOT / '.local/pal/hidden-items-research/suzhou-text-guide.md'
REFERENCE_TEXT = REFERENCE.read_text()
LOWER_IMAGE = ROOT / '.local/pal/hidden-items-research/cave-lower-map-reference.png'
assert LOWER_IMAGE.read_bytes()[:8] == b'\x89PNG\r\n\x1a\n'

def original(event):
    return struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(event-1)*32)

def semantic(row):
    return row[:13]+row[15:]

def snapshot(name):
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
    needle = SSS[OFFSETS[0]+234*32:OFFSETS[0]+234*32+8]
    starts = []
    at = memory.find(needle)
    while at >= 0:
        start = at-234*32
        if start >= OFFSETS[0] and start+5077*32 <= len(memory) and memory[start-OFFSETS[0]:start] != SSS[:OFFSETS[0]]:
            anchors = [235, 243, 261, 262, 264]
            if all(struct.unpack_from('<hHHhHHh9H', memory, start+(event-1)*32)[1:3] == original(event)[1:3] and struct.unpack_from('<hHHhHHh9H', memory, start+(event-1)*32)[8] == original(event)[8] for event in anchors):
                starts.append(start)
        at = memory.find(needle, at+1)
    assert len(starts) == 1, (name, starts)
    start = starts[0]
    events = {event: struct.unpack_from('<hHHhHHh9H', memory, start+(event-1)*32) for event in [666, 667, 670, 714, 715, 744]}
    return state['public'], events, {'path': str(path.relative_to(ROOT)), 'sha256': hashlib.sha256(data).hexdigest()}

def goals(row):
    points = set()
    for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
        offsets = [(0, 0)]
        x = y = 0
        for step in range(4):
            offsets.extend([(x+dx, y+dy), (x, y+dy*2), (x+dx*2, y)])
            x, y = x+dx, y+dy
        for index, (x, y) in enumerate(offsets):
            if index < row[7]*6-4:
                points.add((row[1]-x, row[2]-y))
    return points

records = []
groups = [(41, 'hidden-dragon-stair-4-state.json', [(666, '雄黄', '1.雄黄（拿不到）'), (667, '止血草', '2.止血草（拿不到）'), (670, '雄黄', '1.雄黄（拿不到）')]), (45, 'hidden-dragon-cave-party-state.json', [(714, '止血草', '2.止血草（拿不到）'), (715, '雄黄', '3.雄黄（拿不到）')]), (46, 'hidden-dragon-stair-2-state.json', [(744, '藤甲', '3.籐甲（拿不到）')])]
for scene, source_name, requested in groups:
    root = ROOTS['roots'][source_name]
    view, events, source = snapshot(source_name[:-5]+'-1.json')
    assert source['sha256'] == root['sha256'] and view['scene'] == scene
    report_path = BASE / (root['report'][:-5]+'-1.json')
    report_bytes = report_path.read_bytes()
    parent = json.loads(report_bytes)
    assert parent['passed'] and not parent.get('resumedTestRoute') and hashlib.sha256(report_bytes).hexdigest() == root['reportSHA256']
    offset = [view['x']-view['cameraX'], view['y']-view['cameraY']]
    assert offset == [160, 112]
    entrances = []
    transitions = [(pc, row) for pc, row in enumerate(SCRIPTS) if row[0] == 0x59 and row[1] == scene]
    for pc, row in transitions:
        for target in range(pc+1, min(pc+5, len(SCRIPTS))):
            entry = SCRIPTS[target]
            if entry[0] <= 3 or entry[0] == 0x59:
                break
            if entry[0] == 0x46:
                entrances.append({'scenePC': pc, 'positionPC': target, 'position': [entry[1]*32+entry[3]*16, entry[2]*16+entry[3]*8]})
                break
    assert len(entrances) == len(transitions) > 0
    map_id = CANDIDATES[requested[0][0]]['Map']
    assert map_id == view['map']
    map_path = ROOT / f'.local/pal/visual/pal/v1/maps/{map_id}.bin.gz'
    ground = gzip.decompress(map_path.read_bytes())
    assert len(ground) == 65536
    seeds = {tuple(row['position']) for row in entrances} | {(view['x'], view['y'])}
    seen = set(seeds)
    queue = deque(seeds)
    while queue:
        x, y = queue.popleft()
        for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
            a, b = x+dx, y+dy
            if (a, b) in seen or a < 160 or a >= 1856 or b < 112 or b >= 2048:
                continue
            index = ((b//16*64+a//32)*2+a%32//16)*4
            if struct.unpack_from('<I', ground, index)[0] & 8192:
                continue
            seen.add((a, b))
            queue.append((a, b))
    final_path = BASE / f'regional-hidden-v5-{scene}-state-1.json'
    result_path = BASE / f'regional-hidden-v5-{scene}-result-1.json'
    final = final_source = None
    if final_path.is_file() and result_path.is_file():
        result = json.loads(result_path.read_text())
        assert result['passed'] and result['verificationVersion'] == 5
        _, final, final_source = snapshot(final_path.name)
    for event, item, quote in requested:
        row, candidate = original(event), CANDIDATES[event]
        assert quote in REFERENCE_TEXT and candidate['Scene'] == scene and candidate['Map'] == map_id
        assert candidate['X'] == row[1] and candidate['Y'] == row[2] and candidate['Script'] == row[4] and candidate['Grants'][0]['Name'] == item
        assert semantic(events[event]) == semantic(row) and row[6] == 2 and row[8] == 10
        reachable = sorted(goals(row) & seen)
        assert not reachable, (event, reachable)
        if final is not None:
            assert semantic(final[event]) == semantic(row)
        records.append({'event': event, 'scene': scene, 'map': map_id, 'position': list(row[1:3]), 'item': item, 'script': row[4], 'searchMode': row[7], 'classification': 'original-unreachable-preserved', 'normalPickupVerified': False, 'nativeRecordPreservedBefore': True, 'nativeRecordPreservedAfter': final is not None, 'source': source, 'final': final_source, 'referenceQuote': quote, 'referenceLabel': '同图两处标号 1' if event in [666, 670] else None, 'allSceneTransitions': len(transitions), 'entrances': entrances, 'partyOffset': offset, 'optimisticWalkablePositions': len(seen), 'reachableSearchStances': reachable, 'mapSHA256': hashlib.sha256(ground).hexdigest()})

report = {'passed': True, 'readyForProgress': all(row['nativeRecordPreservedAfter'] for row in records), 'scope': 'read-only same-core original event records, every literal scene-change entrance in the original script table, native range bounds and original search footprint; optimistic graph ignores NPC and portal blockers; independent textual unreachable labels and lower-floor image with two label-1 objects; native records unchanged after accepted collection required for progress, no successful pickup claim', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'catalogSHA256': hashlib.sha256(CATALOG_PATH.read_bytes()).hexdigest(), 'nativeEdits': False, 'checkedEventFields': list(range(13))+[15], 'ignoredResourceCacheFields': [13, 14], 'records': records, 'reference': {'url': 'https://gamegene.cn/wiki/359', 'text': str(REFERENCE.relative_to(ROOT)), 'sha256': hashlib.sha256(REFERENCE.read_bytes()).hexdigest(), 'lowerImageURL': 'https://img.gamegene.cn/wiki/359-19.png', 'lowerImage': str(LOWER_IMAGE.relative_to(ROOT)), 'lowerImageSHA256': hashlib.sha256(LOWER_IMAGE.read_bytes()).hexdigest()}}
out = ROOT / '.local/pal/verification/cave-original-unreachable.json'
out.write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'readyForProgress': report['readyForProgress'], 'originalUnreachable': len(records), 'events': [row['event'] for row in records]}, ensure_ascii=False))
