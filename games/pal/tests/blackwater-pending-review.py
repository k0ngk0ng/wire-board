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
assert hashlib.sha256(SSS).hexdigest() == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
assert hashlib.sha256((ROOT / '.local/pal/runtime-candidates/audio/hp-9999/rules.wasm').read_bytes()).hexdigest() == CORE
OFFSETS = struct.unpack_from('<6I', SSS)
SCRIPTS = [struct.unpack_from('<4H', SSS, at) for at in range(OFFSETS[4], OFFSETS[5], 8)]

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
    assert row[:13]+row[15:] == original(event)[:13]+original(event)[15:]
    return state['public'], digest(path)

records = []
for scene, map_id, event, source_name in [(61, 55, 1172, 'ghost-tomb-waypoint-31-state-1.json'), (63, 56, 1196, 'ghost-tomb-waypoint-2-state-1.json')]:
    row = original(event)
    assert row[7] == 1 and row[8] == 10
    entrances = []
    transitions = [(pc, entry) for pc, entry in enumerate(SCRIPTS) if entry[0] == 0x59 and entry[1] == scene]
    for pc, entry in transitions:
        for position_pc in range(pc+1, min(pc+5, len(SCRIPTS))):
            position = SCRIPTS[position_pc]
            if position[0] <= 3 or position[0] == 0x59:
                break
            if position[0] == 0x46:
                entrances.append({'scenePC': pc, 'positionPC': position_pc, 'position': [position[1]*32+position[3]*16, position[2]*16+position[3]*8]})
                break
    assert len(transitions) == len(entrances) == (12 if scene == 61 else 3)
    map_path = ROOT / f'.local/pal/visual/pal/v1/maps/{map_id}.bin.gz'
    ground = gzip.decompress(map_path.read_bytes())
    assert len(ground) == 65536
    scene_row = struct.unpack_from('<4H', SSS, OFFSETS[1]+(scene-1)*8)
    next_scene = struct.unpack_from('<4H', SSS, OFFSETS[1]+scene*8)
    assert scene_row[0] == map_id
    portals = []
    for candidate in range(scene_row[3]+1, next_scene[3]+1):
        item = original(candidate)
        if item[6] > 0 and item[7] >= 4 and item[8] == 0:
            portals.append({'event': candidate, 'position': list(item[1:3]), 'radiusExclusive': (item[7]-4)*32+16})
    assert len(portals) == (12 if scene == 61 else 3)

    def walk(absorb_exits):
        seeds = {tuple(entry['position']) for entry in entrances}
        queue, seen = deque(seeds), set(seeds)
        while queue:
            x, y = queue.popleft()
            for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
                a, b = x+dx, y+dy
                if (a, b) in seen or not (160 <= a < 1856 and 112 <= b < 2048):
                    continue
                if struct.unpack_from('<I', ground, ((b//16*64+a//32)*2+a%32//16)*4)[0] & 8192:
                    continue
                if absorb_exits and any(abs(a-portal['position'][0])+2*abs(b-portal['position'][1]) < portal['radiusExclusive'] for portal in portals):
                    continue
                seen.add((a, b))
                queue.append((a, b))
        return seen

    goals = {(row[1]+dx, row[2]+dy) for dx, dy in [(0, 0), (-16, 8), (-16, -8), (16, 8), (16, -8)]}
    optimistic, bounded = walk(False), walk(True)
    assert not goals & bounded
    if scene == 61:
        assert not goals & optimistic
    else:
        assert goals & optimistic
    before_view, before = snapshot(BASE / source_name, event)
    final_view, final = snapshot(BASE / f'regional-hidden-v7-{scene}-state-1.json', event)
    assert final_view['scene'] == scene and final_view['map'] == map_id
    result_path = BASE / f'regional-hidden-v7-{scene}-result-1.json'
    result = json.loads(result_path.read_bytes())
    assert result['passed'] and result['verificationVersion'] == 7 and event not in [entry['event'] for entry in result['claimed']]
    records.append({'event': event, 'scene': scene, 'map': map_id, 'position': list(row[1:3]), 'script': row[4], 'classification': 'pending-independent-evidence-and-route-review', 'normalPickupVerified': False, 'readyForProgress': False, 'originalRecordUnchanged': True, 'beforeScene': before_view['scene'], 'before': before, 'after': final, 'result': digest(result_path), 'allSceneTransitions': len(transitions), 'entrances': entrances, 'portals': portals, 'optimisticWalkablePositions': len(optimistic), 'sameSceneWalkablePositions': len(bounded), 'ignoringExitSearchStances': sorted(goals & optimistic), 'sameSceneSearchStances': sorted(goals & bounded), 'mapFile': digest(map_path), 'originalTileDWORD': struct.unpack_from('<I', ground, ((row[2]//16*64+row[1]//32)*2+row[1]%32//16)*4)[0]})

report = {'passed': True, 'diagnosticOnly': True, 'readyForProgress': False, 'scope': 'read-only unchanged original candidate records before and after accepted regional collections; every literal entrance, optimistic map with and without absorbing original touch exits; no independent explicit unreachable attribution yet, no successful pickup claim or progress increment', 'nativeBuildID': CORE, 'nativeEdits': False, 'records': records}
(ROOT / '.local/pal/verification/blackwater-extra-records-pending.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'diagnosticOnly': True, 'readyForProgress': False, 'pendingEvents': [row['event'] for row in records]}, ensure_ascii=False))
