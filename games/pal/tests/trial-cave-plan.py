import collections
import gzip
import hashlib
import heapq
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
ORIGINAL = ROOT / '.local/pal/original-98'
SSS = (ORIGINAL / 'sss.mkf').read_bytes()
DATA = (ORIGINAL / 'data.mkf').read_bytes()
SCENE_OFFSET = struct.unpack_from('<I', SSS, 4)[0]
EVENT_OFFSET = struct.unpack_from('<I', SSS, 0)[0]
SCRIPT_START, SCRIPT_END = struct.unpack_from('<II', SSS, 16)
SCRIPTS = [struct.unpack_from('<4H', SSS, i) for i in range(SCRIPT_START, SCRIPT_END, 8)]
OBJECT_START, OBJECT_END = struct.unpack_from('<II', SSS, 8)
OBJECTS = [struct.unpack_from('<7H', SSS, i) for i in range(OBJECT_START, OBJECT_END, 14)]
TEAM_START, TEAM_END = struct.unpack_from('<II', DATA, 8)
TEAMS = [struct.unpack_from('<5H', DATA, i) for i in range(TEAM_START, TEAM_END, 10)]
SCENES = {}
GROUPS = {}
MAP_HASHES = {}

def distance(a, b):
    return abs(a[0] - b[0]) + abs(a[1] - b[1]) * 2

def search_reaches(point, target, mode):
    same = lambda p: (p[0] // 32, p[1] // 16, p[0] % 32 != 0) == (target[0] // 32, target[1] // 16, target[0] % 32 != 0)
    for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
        x, y = point
        positions = [(x, y)]
        for _ in range(4):
            positions.extend([(x + dx, y + dy), (x, y + dy * 2), (x + dx * 2, y)])
            x, y = x + dx, y + dy
        if any(same(p) for p in positions[:mode * 6 - 4]):
            return True
    return False

for scene in range(216, 228):
    map_id, on_enter, teleport, first = struct.unpack_from('<4H', SSS, SCENE_OFFSET + (scene - 1) * 8)
    last = struct.unpack_from('<4H', SSS, SCENE_OFFSET + scene * 8)[3]
    binary = gzip.decompress((ROOT / '.local/pal/visual/pal/v1/maps' / (str(map_id) + '.bin.gz')).read_bytes())
    MAP_HASHES[map_id] = hashlib.sha256(binary).hexdigest()
    tiles = struct.unpack('<16384I', binary)
    cells = {(x * 32 + half * 16, y * 16 + half * 8) for y in range(128) for x in range(64) for half in range(2) if not tiles[(y * 64 + x) * 2 + half] & 8192 and 160 <= x * 32 + half * 16 < 1856 and y * 16 + half * 8 >= 112}
    groups = {}
    count = 0
    for cell in sorted(cells):
        if cell in groups:
            continue
        queue = collections.deque([cell])
        groups[cell] = count
        while queue:
            x, y = queue.popleft()
            for dx, dy in [(-16, 8), (-16, -8), (16, -8), (16, 8)]:
                point = (x + dx, y + dy)
                if point in cells and point not in groups:
                    groups[point] = count
                    queue.append(point)
        count += 1
    GROUPS[scene] = groups
    events = []
    for i in range(first, last):
        values = struct.unpack_from('<16H', SSS, EVENT_OFFSET + i * 32)
        x, y, script, state, mode, sprite = values[1], values[2], values[4], values[6], values[7], values[8]
        if not script or not 0 < state < 32768 or mode == 0:
            continue
        operation, a, b, c = SCRIPTS[script]
        record = {'scene': scene, 'event': i + 1, 'x': x, 'y': y, 'script': script, 'search': mode < 4, 'mode': mode}
        radius = (mode - 4) * 32 + 16 if mode >= 4 else 256
        record['components'] = sorted({groups[point] for point in cells if distance(point, (x, y)) < radius and (mode >= 4 or search_reaches(point, (x, y), mode))})
        if operation == 7 and sprite and mode >= 4:
            if scene > 221:
                continue
            record['kind'] = 'encounter'
            record['wormCandidates'] = sum(1 for enemy in TEAMS[a] if enemy not in [0, 65535] and OBJECTS[enemy][3] == 42828)
            if not record['wormCandidates']:
                continue
        elif operation == 89 and not sprite and a in range(216, 228):
            position = SCRIPTS[script + 1]
            if position[0] != 70:
                raise AssertionError(('unsupported original portal placement', record))
            record.update(kind='portal', destinationScene=a, destination=[position[1] * 32 + position[3] * 16, position[2] * 16 + position[3] * 8])
        else:
            continue
        events.append(record)
    SCENES[scene] = events

ORIGINAL_DESTINATIONS = {event['event']: (event['destinationScene'], tuple(event['destination'])) for events in SCENES.values() for event in events if event['kind'] == 'portal'}
for events in SCENES.values():
    for event in events:
        if event['kind'] != 'portal':
            continue
        scene, point = ORIGINAL_DESTINATIONS[event['event']]
        seen = set()
        chained = []
        while True:
            key = (scene, point)
            if key in seen:
                raise AssertionError(('original automatic portal cycle', event['event'], chained))
            seen.add(key)
            following = next((candidate for candidate in SCENES[scene] if candidate['kind'] == 'portal' and not candidate['search'] and distance(point, (candidate['x'], candidate['y'])) < (candidate['mode'] - 4) * 32 + 16), None)
            if following is None:
                break
            chained.append(following['event'])
            scene, point = ORIGINAL_DESTINATIONS[following['event']]
        event.update(destinationScene=scene, destination=list(point), chainedEvents=chained)

EDGES = collections.defaultdict(list)
CANDIDATES = []
for scene, events in SCENES.items():
    for event in events:
        if event['kind'] == 'encounter':
            CANDIDATES.append(event)
            continue
        destination = tuple(event['destination'])
        group = GROUPS[event['destinationScene']].get(destination)
        if group is None:
            continue
        target = (event['destinationScene'], group)
        for group in event['components']:
            EDGES[(scene, group)].append((target, event))

current = (216, GROUPS[216][(1168, 360)])
position = (1168, 360)
remaining = list(CANDIDATES)
steps = []
potential = 0
while remaining:
    distances = {current: (0, position, [])}
    queue = [(0, current)]
    while queue:
        cost, node = heapq.heappop(queue)
        if cost != distances[node][0]:
            continue
        _, at, path = distances[node]
        for target, event in EDGES[node]:
            value = cost + distance(at, (event['x'], event['y'])) + 256
            if target not in distances or value < distances[target][0]:
                distances[target] = (value, tuple(event['destination']), path + [event])
                heapq.heappush(queue, (value, target))
    best = None
    for event in remaining:
        for group in event['components']:
            node = (event['scene'], group)
            if node not in distances:
                continue
            cost, at, path = distances[node]
            choice = (cost + distance(at, (event['x'], event['y'])), event['event'], node, event, path)
            if best is None or choice[:2] < best[:2]:
                best = choice
    if best is None:
        break
    _, _, current, event, path = best
    steps.extend(path)
    steps.append(event)
    remaining.remove(event)
    position = (event['x'], event['y'])
    potential += event['wormCandidates']

assert potential >= 36
out = ROOT / 'games/pal/tests/testdata/trial-worm-route.json'
out.parent.mkdir(parents=True, exist_ok=True)
out.write_text(json.dumps({'scope': 'static original test walkthrough candidates only; actual movement, original battles and inventory must be verified, never production bot policy', 'sssSHA256': hashlib.sha256(SSS).hexdigest(), 'dataSHA256': hashlib.sha256(DATA).hexdigest(), 'mapSHA256': MAP_HASHES, 'start': [216, 1168, 360], 'estimatedWormCandidates': potential, 'steps': steps}, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'steps': len(steps), 'encounters': sum(step['kind'] == 'encounter' for step in steps), 'potential': potential, 'path': str(out.relative_to(ROOT))}))
