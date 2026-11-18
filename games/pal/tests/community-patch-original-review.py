import base64
from collections import deque
from compression import zstd
import ctypes
import gzip
import hashlib
import json
from pathlib import Path
import struct


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SCOPE = VERIFY / 'win98-reference-v1'
ORIGINAL = ROOT / '.local/pal/original-98'
NATIVE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999'
REF = 'https://raw.githubusercontent.com/kunruiw1991/pal98-rouqing/ec8dfcb15eb78ce2e851332650b88c723e8eb760/'
QUOTES = {145: '修改了盛渔村市场一个得不到的水果的坐标', 191: '修改了皇甫英家一个得不到的糯米糕的坐标', 1196: '修改了黑水镇民居的一个地图外的金刚符宝箱的坐标', 2173: '修改了尚书府正厅一个地图外的十里香的坐标', 2763: '修改了锁妖塔七层一个地图外的还神丹宝箱的坐标', 3537: '修改了大理城北民居乙的一个得不到的布靴的坐标'}
ABYSS_QUOTE = '1 修正了无底深渊地图C部分物品得不到的问题'
DIRS = [(-16, 8), (-16, -8), (16, -8), (16, 8)]
CASES = {145: (145, 5), 191: (191, 9), 1196: (1221, 62), 2173: (2248, 118), 2763: (2894, 166), 3537: (3748, 207), **{event: (event + 255, 293) for event in range(5071, 5078)}}


def sha(data):
    return hashlib.sha256(data).hexdigest()


def bound(path, expected):
    data = path.read_bytes()
    assert sha(data) == expected, path
    return data


def rel(path):
    return path.relative_to(ROOT).as_posix()


def parse(data):
    offsets = struct.unpack_from('<6I', data)
    return {
        'events': [None, *struct.iter_unpack('<hHHhHHh9H', data[offsets[0]:offsets[1]])],
        'scenes': list(struct.iter_unpack('<4H', data[offsets[1]:offsets[2]])),
        'scripts': list(struct.iter_unpack('<4H', data[offsets[4]:offsets[5]])),
    }


def terrain(data, method, map_id):
    a, b = struct.unpack_from('<2I', data, 4 * map_id)
    compressed = data[a:b]
    assert (compressed[:4] == b'YJ_1') == (method == 'YJ1_Decompress')
    decoder = getattr(ctypes.CDLL(str(SCOPE / 'map-decoder.dylib')), method)
    decoder.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_int]
    decoder.restype = ctypes.c_int
    buffer = ctypes.create_string_buffer(65536)
    assert decoder(ctypes.create_string_buffer(compressed + bytes(8)), buffer, 65536) == 65536
    return buffer.raw


def reachable(image, scripts, scene, padding=True):
    seeds = []
    for pc, row in enumerate(scripts):
        if row[:2] == (0x59, scene):
            positions = [i for i in [pc - 1, pc + 1] if scripts[i][0] in [0x46, 0x70, 0x7a, 0x7b]]
            assert positions, (scene, pc)
            _, x, y, half = scripts[positions[-1]]
            seeds.append((x*32+half*16, y*16+half*8))
    assert seeds
    seen = set(seeds)
    queue = deque(seeds)
    tiles = struct.unpack('<16384I', image)
    while queue:
        x, y = queue.popleft()
        for dx, dy in DIRS:
            a, b = x+dx, y+dy
            if (a,b) in seen or not (0 if padding else 160) <= a < (2048 if padding else 1856) or not (0 if padding else 112) <= b < 2048:
                continue
            if not tiles[(b//16*64+a//32)*2+a%32//16] & 8192:
                seen.add((a,b))
                queue.append((a,b))
    return seen


def stances(seen, coord):
    x, y = coord
    return sorted(seen & {(x, y), *((x+dx, y+dy) for dx, dy in DIRS)})


def grant(data, event, scene):
    row = data['events'][event]
    assert data['scenes'][scene - 1][3] < event <= data['scenes'][scene][3]
    assert row[6] > 0 and row[7] == 1
    code = data['scripts'][row[4]:row[4]+4]
    assert code[1][0] == 0x3e and code[2][0] == 0xffff and code[3][0] == 0x1f and code[3][2:] == (0, 0)
    return code[3][1]


def original_state(path, event, event_row):
    data = path.read_bytes()
    packed = base64.b64decode(json.loads(data)['native'], validate=True)
    raw = zstd.decompress(packed) if packed[:4] == bytes.fromhex('28b52ffd') else gzip.decompress(packed)
    assert raw[:8] == b'PALMEM01'
    size, mem_size = struct.unpack_from('<II', raw, 8)
    meta = json.loads(raw[16:16 + size])
    memory = raw[16 + size:]
    assert len(memory) == mem_size <= 48*1024*1024 and sha(memory + struct.pack('<I', meta['stackPointer'])) == meta['memoryID']
    assert meta['buildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
    anchor = WIN[0:24]
    at = memory.find(WIN[24+234*32:24+234*32+8])
    starts = []
    while at >= 0:
        start = at - 234*32
        if start >= 24 and start + 5077*32 <= len(memory) and memory[start-24:start] != anchor:
            if all(struct.unpack_from('<HH', memory, start+(index-1)*32+2) == WIN_DATA['events'][index][1:3] for index in [235, 243, 261, 262, 264]):
                starts.append(start)
        at = memory.find(WIN[24+234*32:24+234*32+8], at + 1)
    assert len(starts) == 1, path
    got = struct.unpack_from('<hHHhHHh9H', memory, starts[0]+(event-1)*32)
    assert got[:14] + got[15:] == event_row[:14] + event_row[15:], (path.name, got, event_row)
    return {'path': rel(path), 'sha256': sha(data), 'eventRow': got}


WIN = bound(ORIGINAL / 'sss.mkf', 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756')
DOS = bound(SCOPE / 'fan-dos-sss.mkf', '38252bae8dabaebe9c780f00e8e6829bce56f89d862ecf9d44cc7b135047ed5d')
WIN_MAP = bound(ORIGINAL / 'map.mkf', 'bef63043ffcbe9b6de6cbe295891134d7cea74952cb6701b5566049d0eb015e2')
DOS_MAP = bound(SCOPE / 'fan-dos-map.mkf', '1f1fdf71257e3769d1ee3494336a8ae3a5d7690dce710803fa357bbc48023d88')
WIN_DATA, DOS_DATA = parse(WIN), parse(DOS)
raw_note = SCOPE / 'dos-readme.zh.txt'
note = SCOPE / 'dos-readme.zh.txt.utf8'
text = bound(raw_note, '80752d9475c0daadc6bbc664c52ad2bd7aed096dd5db2bc337f066f877369911').decode('cp936').replace('\r\n', '\n')
assert text == note.read_text()
catalog = json.loads((VERIFY / 'hidden-search-candidates.json').read_bytes())
items = {row['Event']: row for row in catalog['candidates']}
earlier = json.loads((VERIFY / 'hidden-entry-recheck-v1/entry-routes.json').read_bytes())
earlier_routes = {row['event']: row['routeVariants'] for scene in earlier['scenes'] for row in scene['records']}
assert earlier['passed'] and earlier['sssSHA256'] == sha(WIN) and not earlier['progressIncrement']
snapshot_paths = {
    145: [NATIVE / 'regional-hidden-6-unverified-145-state-1.json'],
    191: [NATIVE / 'regional-hidden-10-unverified-191-state-1.json'],
    1196: [NATIVE / 'regional-hidden-v7-63-state-1.json'],
    2173: [NATIVE / 'capital-manor-step-2-state-1.json', NATIVE / 'capital-manor-step-48-state-1.json'],
    2763: [NATIVE / 'scopes/tower-remainder-v1' / f'{name}-state-1.json' for name in ['tower-upper-step-3', 'regional-hidden-v20-167']],
    3537: [VERIFY / 'dali-rewards-source/original-boots-attempt-state.json', NATIVE / 'regional-hidden-v15-208-state-1.json'],
}
patch_before, patch_after = [NATIVE / f'{name}-state-1.json' for name in ['finale-field-294-step-2', 'regional-hidden-v19-294']]
results, controls, graphs = [], [], {}
for event, (dos_event, dos_scene) in CASES.items():
    win = items[event]
    scene, map_id = win['Scene'], win['Map']
    assert WIN_DATA['scenes'][scene - 1][0] == DOS_DATA['scenes'][dos_scene - 1][0] == map_id
    key = (map_id, scene, dos_scene)
    if key not in graphs:
        decoded_win = terrain(WIN_MAP, 'YJ2_Decompress', map_id)
        decoded_dos = terrain(DOS_MAP, 'YJ1_Decompress', map_id)
        assert decoded_win == gzip.decompress((ROOT / f'.local/pal/visual/pal/v1/maps/{map_id}.bin.gz').read_bytes())
        graphs[key] = decoded_win, decoded_dos, reachable(decoded_win, WIN_DATA['scripts'], scene), reachable(decoded_dos, DOS_DATA['scripts'], dos_scene, False)
    decoded_win, decoded_dos, win_walk, dos_walk = graphs[key]
    original = WIN_DATA['events'][event]
    patched = DOS_DATA['events'][dos_event]
    assert list(original[1:3]) == [win['X'], win['Y']] and original[8] == patched[8]
    prize = grant(WIN_DATA, event, scene)
    assert prize == grant(DOS_DATA, dos_event, dos_scene) == win['Grants'][0]['Item']
    previous = stances(win_walk, original[1:3])
    after = stances(dos_walk, patched[1:3])
    assert after, event
    if event in {5075, 5076}:
        assert previous
        controls.append({'event': event, 'winSearchStances': previous, 'dosSearchStances': after})
        continue
    quote = QUOTES.get(event, ABYSS_QUOTE)
    assert quote in text and not earlier_routes[event]['preMoveDoors']['searchFound']
    if event in QUOTES:
        assert original[1:3] != patched[1:3]
    else:
        assert not previous and map_id == 192
    if event == 3537:
        assert not earlier_routes[event]['preMoveDoorsAllowObservedActors']['searchFound']
    checks = [original_state(path, event, original) for path in snapshot_paths.get(event, [patch_before, patch_after])]
    results.append({'event': event, 'scene': scene, 'map': map_id, 'position': original[1:3], 'grant': prize, 'quote': quote, 'classification': 'original-unreachable-preserved', 'actuallyObtained': False, 'originalEvent': original, 'originalSnapshots': checks, 'originalPreMoveDoorRoute': False, 'originalTerrainSearchStances': previous, 'patchedScene': dos_scene, 'patchedEvent': dos_event, 'patchedPosition': patched[1:3], 'differsInPosition': original[1:3] != patched[1:3], 'patchedSearchStances': after, 'winTileSHA256': sha(decoded_win), 'dosTileSHA256': sha(decoded_dos)})

assert len(results) == 11 and {row['event'] for row in results} == set(CASES) - {5075, 5076}
report = {'passed': True, 'readyForProgress': True, 'originalDataEdits': False, 'scopeStartsWithOriginalWindows98': True, 'scopeArchivesAreComparisonOnly': True, 'actuallyObtained': 0, 'scope': 'unaltered Windows original saved event rows and original routes, independently documented community DOS fixes, exact same-map/sprite/item correspondence, optimistic patch terrain connectivity only', 'source': {'url': REF + 'Dos_98/Readme/readme.txt', 'rawPath': rel(raw_note), 'rawSHA256': sha(raw_note.read_bytes()), 'decodedPath': rel(note), 'decodedSHA256': sha(note.read_bytes()), 'comparisonSSS': {'url': REF + 'Dos_98/SSS.MKF', 'sha256': sha(DOS)}, 'comparisonMap': {'url': REF + 'Dos_98/MAP.MKF', 'sha256': sha(DOS_MAP)}}, 'originalSSS': sha(WIN), 'originalMap': sha(WIN_MAP), 'records': results, 'abyssControls': controls, 'unresolved': [{'event': 1172, 'cause': 'same reward coordinate in comparison; changed original geometry but no scoped independent statement'}, {'event': 2947, 'cause': 'scripted visit confirmed earlier; current textual sources do not specifically identify the duplicate banana event'}]}
(SCOPE / 'community-patch-review.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'passed': True, 'originalRecordsPreserved': len(results), 'normalPickups': 0}, ensure_ascii=False))
