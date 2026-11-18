from collections import deque
import gzip
import hashlib
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
SSS = (ROOT / '.local/pal/original-98/sss.mkf').read_bytes()
OFFSETS = struct.unpack_from('<6I', SSS)
assert hashlib.sha256(SSS).hexdigest() == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
scene, map_id = 167, 143
first = struct.unpack_from('<4H', SSS, OFFSETS[1]+(scene-1)*8)[3]
last = struct.unpack_from('<4H', SSS, OFFSETS[1]+scene*8)[3]
events = {number: struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(number-1)*32) for number in range(first+1, last+1)}
data = gzip.decompress((ROOT / f'.local/pal/visual/pal/v1/maps/{map_id}.bin.gz').read_bytes())
assert len(data) == 65536
tiles = struct.unpack('<16384I', data)
free = {(x*32+half*16, y*16+half*8) for y in range(128) for x in range(64) for half in range(2) if not tiles[(y*64+x)*2+half] & 8192 and 160 <= x*32+half*16 < 1856 and 112 <= y*16+half*8 < 2048}
free -= {tuple(row[1:3]) for row in events.values() if row[6] == 2 and row[8] == 10}
exit_event = events[2740]
assert exit_event[1:3] == (976, 1720) and exit_event[7] == 5
free = {point for point in free if abs(point[0]-976)+2*abs(point[1]-1720) >= 48}
source_name = 'tower-upper-step-3-state.json'
source_path = BASE / (source_name[:-5]+'-1.json')
source_bytes = source_path.read_bytes()
source = json.loads(source_bytes)['public']
assert [member['role'] for member in source['party']] == [0, 2] and [source['scene'], source['x'], source['y']] == [167, 1024, 720]
current = (source['x'], source['y'])
catalog = json.loads((VERIFY / 'hidden-search-candidates.json').read_bytes())
remaining = [row for row in catalog['candidates'] if row['Scene'] == scene]
assert len(remaining) == 12
steps = []
while remaining:
    assert current in free
    walk, queue = {current: 0}, deque([current])
    while queue:
        x, y = queue.popleft()
        for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
            point = (x+dx, y+dy)
            if point in free and point not in walk:
                walk[point] = walk[(x, y)]+1
                queue.append(point)
    choices = []
    for row in remaining:
        x, y = row['X'], row['Y']
        assert row['TriggerMode'] == 1
        goals = [(x, y), (x-16, y-8), (x-16, y+8), (x+16, y-8), (x+16, y+8)]
        choices.extend((walk[point], row['Event'], point, row) for point in goals if point in walk)
    if not choices:
        break
    cost, event, stance, row = min(choices, key=lambda choice: choice[:3])
    steps.append({'event': event, 'scene': scene, 'position': [row['X'], row['Y']], 'searchStanceForPlanning': list(stance), 'portalsBefore': [], 'estimatedWalkingAndPortalCost': cost})
    remaining.remove(row)
    current = stance
case = {'scene': scene, 'sourceCheckpoint': source_name, 'sourceSHA256': hashlib.sha256(source_bytes).hexdigest(), 'sourcePosition': [source['scene'], source['x'], source['y']], 'events': [row['event'] for row in steps], 'unplannedEvents': [row['Event'] for row in remaining], 'steps': steps, 'portalSteps': 0, 'earthReturns': 0, 'estimatedWalkingAndPortalCost': sum(row['estimatedWalkingAndPortalCost'] for row in steps)}
report = {'passed': True, 'diagnosticOnly': True, 'originalDataEdits': False, 'scope': 'single original seventh tower floor, static normal walking and all 13 chest footprints including original false jade chest, reserved original exit; only input planning', 'sssSHA256': hashlib.sha256(SSS).hexdigest(), 'mapSHA256': hashlib.sha256(data).hexdigest(), 'case': case}
(VERIFY / 'tower-remainder-plan.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'diagnosticOnly': True, 'events': len(steps), 'unplanned': case['unplannedEvents'], 'walkingCost': case['estimatedWalkingAndPortalCost']}))
