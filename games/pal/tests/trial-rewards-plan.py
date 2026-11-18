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
for scene in range(215, 228):
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


source_name = 'sacred-medicine-trial-entry-state.json'
source_path = BASE / (source_name[:-5]+'-1.json')
source_raw = source_path.read_bytes()
source = json.loads(source_raw)['public']
start = (source['scene'], source['x'], source['y'])
assert start == (216, 1168, 360) and [row['role'] for row in source['party']] == [0, 4]
catalog = json.loads((VERIFY / 'hidden-search-candidates.json').read_bytes())
cases = []
for target_scene, expected_count in [(220, 36), (221, 22)]:
    remaining = [row for row in catalog['candidates'] if row['Scene'] == target_scene]
    assert len(remaining) == expected_count
    current, steps, pending_legs, last_escape_count = start, [], [], -1
    while remaining:
        paths = reachable(current)
        best = None
        for point in remaining:
            target_goals = goals(target_scene, [point['X'], point['Y']], point['TriggerMode'])
            for node, (cost, route) in paths.items():
                if node[0] != target_scene:
                    continue
                walk = walking(target_scene, (node[1], node[2]))
                available = [(walk[goal], goal) for goal in target_goals if goal in walk]
                if not available:
                    continue
                distance_to_reward, standing = min(available)
                choice = (cost+distance_to_reward, point['Event'], standing, point, route)
                if best is None or choice[:3] < best[:3]:
                    best = choice
        if best is None:
            assert len(remaining) != last_escape_count and current[0] in range(217, 228), ('original portal graph still lacks a reward after an original earth return', target_scene, [row['Event'] for row in remaining], current)
            teleport = struct.unpack_from('<4H', SSS, OFFSETS[1]+(current[0]-1)*8)[2]
            assert teleport == 30770 and instruction(teleport) == (0x59, 216, 0, 0) and instruction(teleport+1) == (0x46, 26, 46, 0)
            pending_legs.append({'kind': 'earth-return', 'scene': current[0], 'teleportScript': teleport, 'destinationScene': 216, 'destination': [832, 736]})
            current, last_escape_count = (216, 832, 736), len(remaining)
            continue
        cost, number, standing, point, route = best
        portals = pending_legs+[{**{key: value for key, value in portal.items() if key != 'approach'}, 'kind': 'portal'} for portal in route]
        pending_legs = []
        steps.append({'event': number, 'scene': target_scene, 'position': [point['X'], point['Y']], 'searchStanceForPlanning': list(standing), 'portalsBefore': portals, 'estimatedWalkingAndPortalCost': cost})
        remaining.remove(point)
        current = (target_scene, *standing)
    cases.append({'scene': target_scene, 'sourceCheckpoint': source_name, 'events': [row['event'] for row in steps], 'steps': steps, 'portalSteps': sum(leg['kind'] == 'portal' for row in steps for leg in row['portalsBefore']), 'earthReturns': sum(leg['kind'] == 'earth-return' for row in steps for leg in row['portalsBefore']), 'estimatedWalkingAndPortalCost': sum(row['estimatedWalkingAndPortalCost'] for row in steps)})

report = {'passed': True, 'diagnosticOnly': True, 'scope': 'original static walking cells, permanent chest footprints, native automatic portal trigger boundaries and original search ropes; original chained placements, input planning only, not executed gameplay or progress acceptance', 'sssSHA256': hashlib.sha256(SSS).hexdigest(), 'sourceCheckpoint': source_name, 'sourceSHA256': hashlib.sha256(source_raw).hexdigest(), 'start': list(start), 'mapSHA256': MAP_HASHES, 'cases': cases, 'originalDataEdits': False}
(VERIFY / 'trial-rewards-plan.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'diagnosticOnly': True, 'cases': [{'scene': row['scene'], 'events': len(row['events']), 'portalSteps': row['portalSteps'], 'earthReturns': row['earthReturns'], 'walkingCost': row['estimatedWalkingAndPortalCost']} for row in cases]}))
