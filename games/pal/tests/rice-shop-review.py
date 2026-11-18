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
SSS_PATH = ROOT / '.local/pal/original-98/sss.mkf'
SSS = SSS_PATH.read_bytes()
SSS_HASH = hashlib.sha256(SSS).hexdigest()
assert SSS_HASH == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE
OFFSETS = struct.unpack_from('<6I', SSS)
SCRIPTS = [struct.unpack_from('<4H', SSS, at) for at in range(OFFSETS[4], OFFSETS[5], 8)]
EVENTS = list(range(861, 871))
CATALOG_PATH = ROOT / '.local/pal/verification/hidden-search-candidates.json'
CATALOG = json.loads(CATALOG_PATH.read_text())
assert CATALOG['passed'] and CATALOG['sssSHA256'] == SSS_HASH
CANDIDATES = {row['Event']: row for row in CATALOG['candidates']}
REFERENCE = ROOT / '.local/pal/hidden-items-research/white-river-text-guide.md'
QUOTE = '[物品]骆记米行里的东西都无法拿到（都在屋外黑色部份）。'
assert QUOTE in REFERENCE.read_text()

def digest(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}

def original(event):
    return struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(event-1)*32)

def semantic(row):
    return row[:13]+row[15:]

def snapshot(path):
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
        if start >= OFFSETS[0] and start+(OFFSETS[1]-OFFSETS[0]) <= len(memory) and memory[start-OFFSETS[0]:start] != SSS[:OFFSETS[0]]:
            if all(struct.unpack_from('<hHHhHHh9H', memory, start+(event-1)*32)[1:3] == original(event)[1:3] and struct.unpack_from('<hHHhHHh9H', memory, start+(event-1)*32)[8] == original(event)[8] for event in [235, 243, 261, 262, 264]):
                starts.append(start)
        at = memory.find(needle, at+1)
    assert len(starts) == 1, (str(path), starts)
    records = {event: struct.unpack_from('<hHHhHHh9H', memory, starts[0]+(event-1)*32) for event in [844]+EVENTS}
    return state['public'], records, digest(path)

def search_stances(row):
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

roots = json.loads((ROOT / 'games/pal/tests/campaign-roots.json').read_text())['candidates']['audio']
assert roots['coreSHA256'] == CORE and roots['profile'] == 'hp-9999'
root = roots['roots']['campaign-hidden-dragon-state.json']
before_view, before, before_source = snapshot(BASE / 'campaign-hidden-dragon-state-1.json')
assert before_source['sha256'] == root['sha256'] and before_view['scene'] == 49
parent_path = BASE / (root['report'][:-5]+'-1.json')
parent = json.loads(parent_path.read_text())
assert parent['passed'] and not parent.get('resumedTestRoute') and digest(parent_path)['sha256'] == root['reportSHA256']
result_path = BASE / 'regional-hidden-v6-51-result-1.json'
result = json.loads(result_path.read_text())
assert result['passed'] and result['verificationVersion'] == 6 and result['auditOnly']
assert result['auditedEvents'] == EVENTS and result['claimed'] == [] and result['scene'] == 51
assert result['candidate'] == 'audio' and result['developmentProfile'] == 'hp-9999' and result['authorityFixture']
assert result['sourceCheckpoint'] == 'campaign-hidden-dragon-state.json' and result['sourceSHA256'] == root['sha256']
assert result['proof']['Saves'] >= 1 and result['proof']['Saved']['rice-shop-records-preserved']
assert result['proof']['Battles'] == result['proof']['ScriptedFinishes'] == 0
assert result['cashInitial'] == result['cashFinal'] == before_view['cash']
assert result['partyInitial'] == result['party'] == before_view['party']
saved_view, saved, saved_source = snapshot(BASE / 'regional-hidden-v6-51-rice-shop-records-preserved-state-1.json')
final_view, final, final_source = snapshot(BASE / 'regional-hidden-v6-51-state-1.json')
for view, records in [(saved_view, saved), (final_view, final)]:
    assert view['scene'] == 51 and view['map'] == 58 and [view['x'], view['y']] == [816, 920]
    assert [view['x']-view['cameraX'], view['y']-view['cameraY']] == [160, 112]
    assert view['party'] == before_view['party'] and view['cash'] == before_view['cash']
    assert not any(actor[0] == 1 and actor[1] in EVENTS for actor in view['field']['actors'])
for event in [844]+EVENTS:
    assert semantic(before[event]) == semantic(saved[event]) == semantic(final[event]) == semantic(original(event)), event

entrances = []
transitions = [(pc, row) for pc, row in enumerate(SCRIPTS) if row[0] == 0x59 and row[1] == 51]
for pc, row in transitions:
    for position_pc in range(pc+1, min(pc+5, len(SCRIPTS))):
        entry = SCRIPTS[position_pc]
        if entry[0] <= 3 or entry[0] == 0x59:
            break
        if entry[0] == 0x46:
            entrances.append({'scenePC': pc, 'positionPC': position_pc, 'position': [entry[1]*32+entry[3]*16, entry[2]*16+entry[3]*8]})
            break
assert len(transitions) == len(entrances) == 1
assert entrances == [{'scenePC': 9675, 'positionPC': 9676, 'position': [816, 920]}]
door = original(844)
assert door[1:3] == (768, 944) and door[4:9] == (9679, 0, 1, 5, 0)
scene_row = struct.unpack_from('<4H', SSS, OFFSETS[1]+50*8)
next_scene = struct.unpack_from('<4H', SSS, OFFSETS[1]+51*8)
assert scene_row == (58, 0, 0, 843) and next_scene[3] == 870
touch_events = [event for event in range(scene_row[3]+1, next_scene[3]+1) if original(event)[6] > 0 and original(event)[7] >= 4]
assert touch_events == [844]
assert SCRIPTS[9679:9683] == [(0x59, 50, 0, 0), (0x46, 28, 53, 0), (0x50, 0, 0, 0), (0, 0, 0, 0)]
radius = (door[7]-4)*32+16
assert radius == 48
references = [{'pc': pc, 'instruction': list(row)} for pc, row in enumerate(SCRIPTS) if any(operand in [844, 844+0x9000] for operand in row[1:])]
assert references == [{'pc': 2404, 'instruction': [0xffff, 844, 0, 0]}]
map_path = ROOT / '.local/pal/visual/pal/v1/maps/58.bin.gz'
ground = gzip.decompress(map_path.read_bytes())
assert len(ground) == 65536
visual_release = json.loads((ROOT / 'games/pal/assets/visual-release.json').read_text())
visual_catalog = visual_release['catalogs']['997f43b90cbcb6432137d46dfa29ad5c5cfcfbf8a2d151aff933a3a9cfae2c1c']
assert visual_catalog['cdnVerified']
assert visual_catalog['files']['maps/58.bin.gz']['sha256'] == digest(map_path)['sha256']

def tile(x, y):
    return struct.unpack_from('<I', ground, ((y//16*64+x//32)*2+x%32//16)*4)[0]

def walk(exit_absorbs):
    seeds = {tuple(entry['position']) for entry in entrances}
    seen = set(seeds)
    queue = deque(seeds)
    while queue:
        x, y = queue.popleft()
        for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
            a, b = x+dx, y+dy
            if (a, b) in seen or not (160 <= a < 1856 and 112 <= b < 2048) or tile(a, b) & 8192:
                continue
            if exit_absorbs and abs(a-door[1])+2*abs(b-door[2]) < radius:
                continue
            seen.add((a, b))
            queue.append((a, b))
    return seen

optimistic, same_scene = walk(False), walk(True)
assert len(optimistic) == 12414 and len(same_scene) == 342
records = []
for event in EVENTS:
    row, candidate = original(event), CANDIDATES[event]
    assert candidate['Scene'] == 51 and candidate['Map'] == 58 and candidate['X'] == row[1] and candidate['Y'] == row[2]
    assert candidate['Script'] == row[4] and row[6:9] == (1, 1, 0) and candidate['Sprite'] == 0
    assert tile(row[1], row[2]) == 0
    naive_goals = sorted(search_stances(row) & optimistic)
    reachable = sorted(search_stances(row) & same_scene)
    assert naive_goals and not reachable, event
    records.append({'event': event, 'scene': 51, 'map': 58, 'position': list(row[1:3]), 'grants': candidate['Grants'], 'script': row[4], 'searchMode': row[7], 'classification': 'original-unreachable-preserved', 'normalPickupVerified': False, 'nativeRecordPreservedBefore': True, 'nativeRecordPreservedAfter': True, 'originalTileDWORD': 0, 'reachableSearchStances': reachable, 'ignoringExitSearchStances': naive_goals, 'referenceQuote': QUOTE})

replay_path = ROOT / '.local/pal/verification/audio-campaign-replay.json'
replay = json.loads(replay_path.read_text())
assert replay['passed'] and replay['coreSHA256'] == CORE
later_doors, missing_finals = [], []
for stage in replay['proof']:
    path = ROOT / stage['path']
    assert digest(path)['sha256'] == stage['sha256']
    state_path = path.with_name(path.name[:-7]+'-state-1.json')
    if not state_path.is_file():
        missing_finals.append(stage['stage'])
        continue
    view, state_events, source = snapshot(state_path)
    assert semantic(state_events[844]) == semantic(door), (stage['stage'], state_events[844])
    later_doors.append({'stage': stage['stage'], 'state': source, 'doorUnchanged': True})

report = {'passed': True, 'readyForProgress': True, 'scope': 'independent textual black-area evidence, all literal scene-change entrances, original active exit as absorbing scene boundary, same-core original event records preserved after fresh-runtime save; optimistic comparison ignores NPCs and falsely traverses the exit; literal operand scan is auxiliary evidence, not full symbolic execution; no successful pickup claim', 'nativeBuildID': CORE, 'sssSHA256': SSS_HASH, 'catalogSHA256': digest(CATALOG_PATH)['sha256'], 'nativeEdits': False, 'checkedEventFields': list(range(13))+[15], 'ignoredResourceCacheFields': [13, 14], 'records': records, 'allSceneTransitions': len(transitions), 'entrances': entrances, 'exit': {'event': 844, 'position': list(door[1:3]), 'state': door[6], 'triggerMode': door[7], 'radiusExclusive': radius, 'script': door[4], 'instructions': [list(row) for row in SCRIPTS[9679:9683]], 'literalOperandReferences': references, 'laterStageChecks': later_doors, 'stagesWithoutFinalSnapshot': missing_finals}, 'graph': {'map': digest(map_path), 'uncompressedSHA256': hashlib.sha256(ground).hexdigest(), 'ignoringExitPositions': len(optimistic), 'sameScenePositions': len(same_scene), 'bounds': [160, 1856, 112, 2048], 'partyOffset': [160, 112]}, 'snapshots': [before_source, saved_source, final_source], 'result': digest(result_path), 'parent': digest(parent_path), 'reference': {'url': 'https://gamegene.cn/wiki/360', 'quote': QUOTE, **digest(REFERENCE)}}
out = ROOT / '.local/pal/verification/rice-shop-original-unreachable.json'
out.write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'readyForProgress': True, 'originalUnreachable': len(records), 'sameScenePositions': len(same_scene), 'laterStageDoorsChecked': len(later_doors)}, ensure_ascii=False))
