from collections import deque
import gzip
import hashlib
import heapq
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
SSS = (ROOT / '.local/pal/original-98/sss.mkf').read_bytes()
OFFSETS = struct.unpack_from('<6I', SSS)
assert hashlib.sha256(SSS).hexdigest() == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'


def instruction(pc):
    return struct.unpack_from('<4H', SSS, OFFSETS[4]+pc*8)


def event(number):
    return struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(number-1)*32)


def distance(a, b):
    return abs(a[0]-b[0])+2*abs(a[1]-b[1])


def search_reaches(at, target, mode):
    for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
        x, y = at
        positions = [(x, y)]
        for _ in range(4):
            positions.extend([(x+dx, y+dy), (x, y+dy*2), (x+dx*2, y)])
            x, y = x+dx, y+dy
        if target in positions[:mode*6-4]:
            return True
    return False


PORTALS, MAPS, CELLS, RAW_CELLS, MAP_HASHES = {}, {}, {}, {}, {}
for scene in range(214, 228):
    map_id, _, _, first = struct.unpack_from('<4H', SSS, OFFSETS[1]+(scene-1)*8)
    last = struct.unpack_from('<4H', SSS, OFFSETS[1]+scene*8)[3]
    records = [(number, event(number)) for number in range(first+1, last+1)]
    data = gzip.decompress((ROOT / f'.local/pal/visual/pal/v1/maps/{map_id}.bin.gz').read_bytes())
    assert len(data) == 65536
    MAPS[scene] = map_id
    MAP_HASHES[map_id] = hashlib.sha256(data).hexdigest()
    tiles = struct.unpack('<16384I', data)
    cells = {(x*32+half*16, y*16+half*8) for y in range(128) for x in range(64) for half in range(2) if not tiles[(y*64+x)*2+half] & 8192 and 160 <= x*32+half*16 < 1856 and 112 <= y*16+half*8 < 2048}
    boxes = {tuple(row[1:3]) for _, row in records if row[6] == 2 and row[8] in [10, 389]}
    cells -= boxes
    RAW_CELLS[scene] = cells
    portals = []
    for number, row in records:
        if row[6] <= 0 or row[8] != 0 or not 0 < row[7] <= 8 or not row[4] or instruction(row[4])[0] != 0x59:
            continue
        entry, placement = instruction(row[4]), instruction(row[4]+1)
        assert placement[0] == 0x46 and placement[3] in [0, 1]
        portals.append({'scene': scene, 'event': number, 'position': list(row[1:3]), 'mode': row[7], 'script': row[4], 'rawDestinationScene': entry[1], 'rawDestination': [placement[1]*32+placement[3]*16, placement[2]*16+placement[3]*8], 'search': row[7] < 4})
    PORTALS[scene] = portals
    auto = [row for row in portals if not row['search']]
    CELLS[scene] = {point for point in cells if not any(distance(point, row['position']) < (row['mode']-4)*32+16 for row in auto)}


def goals(scene, position, mode):
    if mode == 1:
        x, y = position
        return [point for point in [(x, y), (x-16, y-8), (x-16, y+8), (x+16, y-8), (x+16, y+8)] if point in CELLS[scene]]
    return [point for point in CELLS[scene] if distance(point, position) <= 256 and search_reaches(point, tuple(position), mode)]


for scene, portals in PORTALS.items():
    for row in portals:
        if row['search']:
            row['approach'] = goals(scene, row['position'], row['mode'])
        else:
            approach = []
            for x, y in CELLS[scene]:
                for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
                    at = (x+dx, y+dy)
                    first = next((candidate for candidate in portals if not candidate['search'] and distance(at, candidate['position']) < (candidate['mode']-4)*32+16), None)
                    if at in RAW_CELLS[scene] and first is row:
                        approach.append((x, y))
                        break
            row['approach'] = approach
        target_scene, target = row['rawDestinationScene'], tuple(row['rawDestination'])
        chain, seen = [], set()
        while target_scene in PORTALS:
            key = (target_scene, target)
            assert key not in seen, ('original automatic portal cycle', row['event'], chain)
            seen.add(key)
            following = next((candidate for candidate in PORTALS[target_scene] if not candidate['search'] and distance(target, candidate['position']) < (candidate['mode']-4)*32+16), None)
            if following is None:
                break
            chain.append(following['event'])
            target_scene, target = following['rawDestinationScene'], tuple(following['rawDestination'])
        row['destinationScene'], row['destination'], row['chainedEvents'] = target_scene, list(target), chain

DISTANCES = {}


def walking(scene, origin):
    key = (scene, origin)
    if key in DISTANCES:
        return DISTANCES[key]
    assert origin in CELLS[scene], ('not a stable original walking position', key)
    result, queue = {origin: 0}, deque([origin])
    while queue:
        x, y = queue.popleft()
        for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
            point = (x+dx, y+dy)
            if point in CELLS[scene] and point not in result:
                result[point] = result[(x, y)]+1
                queue.append(point)
    DISTANCES[key] = result
    return result


def reachable(start):
    paths, queue = {start: (0, [])}, [(0, start)]
    while queue:
        cost, node = heapq.heappop(queue)
        if paths[node][0] != cost:
            continue
        scene, x, y = node
        walk = walking(scene, (x, y))
        for portal in PORTALS[scene]:
            destination_scene, destination = portal['destinationScene'], tuple(portal['destination'])
            if destination_scene not in CELLS or destination not in CELLS[destination_scene]:
                continue
            steps = min((walk.get(point, 1000000) for point in portal['approach']), default=1000000)
            if steps >= 1000000:
                continue
            target = (destination_scene, *destination)
            following_cost = cost+steps+2
            if target not in paths or following_cost < paths[target][0]:
                paths[target] = (following_cost, paths[node][1]+[portal])
                heapq.heappush(queue, (following_cost, target))
    return paths


branches = []
for name, source_name, parent_name in [('two', 'regional-hidden-v18-227-state.json', 'regional-hidden-v18-227-result.json'), ('three', 'prayer-field-229-phase-0-step-4-orb-0-state.json', 'campaign-rain-prayer.json')]:
    source_path = BASE / (source_name[:-5]+'-1.json')
    parent_path = BASE / (parent_name[:-5]+'-1.json')
    parent = json.loads(parent_path.read_bytes())
    assert parent['passed'] and not parent.get('resumedTestRoute')
    source = json.loads(source_path.read_bytes())['public']
    expected_roles = [0, 4] if name == 'two' else [1, 0, 4]
    assert [row['role'] for row in source['party']] == expected_roles
    root = (source['scene'], source['x'], source['y'])
    initial_legs = []
    if name == 'three':
        e = event(4018)
        jump, placement = instruction(e[4]), instruction(e[4]+1)
        assert jump == (0x59, 215, 0, 0) and placement == (0x46, 23, 11, 1)
        initial_legs.append({'kind': 'portal', 'scene': 229, 'event': 4018, 'position': list(e[1:3]), 'mode': e[7], 'script': e[4], 'search': False, 'rawDestinationScene': 215, 'rawDestination': [752, 184], 'destinationScene': 215, 'destination': [752, 184], 'chainedEvents': []})
        root = (215, 752, 184)
    candidates = [(cost, node, route) for node, (cost, route) in reachable(root).items() if node[0] == 214]
    assert candidates
    cost, landing, route = min(candidates, key=lambda row: row[:2])
    legs = initial_legs+[{**{key: value for key, value in leg.items() if key != 'approach'}, 'kind': 'portal'} for leg in route]
    branches.append({'branch': name, 'sourceCheckpoint': source_name, 'sourceSHA256': hashlib.sha256(source_path.read_bytes()).hexdigest(), 'sourceReport': parent_name, 'sourceReportSHA256': hashlib.sha256(parent_path.read_bytes()).hexdigest(), 'sourcePosition': [source['scene'], source['x'], source['y']], 'roles': expected_roles, 'legs': legs, 'bottomLanding': list(landing), 'estimatedWalkingAndPortalCost': cost})
report = {'passed': True, 'diagnosticOnly': True, 'scope': 'ordinary original route to bottom cave from two separate genuinely earned party states; no gameplay or pickup acceptance; no native edits', 'sssSHA256': hashlib.sha256(SSS).hexdigest(), 'mapSHA256': MAP_HASHES, 'branches': branches}
(VERIFY / 'trial-bottom-plan.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'diagnosticOnly': True, 'branches': [{'branch': row['branch'], 'roles': row['roles'], 'legs': len(row['legs']), 'walkingCost': row['estimatedWalkingAndPortalCost']} for row in branches]}))

