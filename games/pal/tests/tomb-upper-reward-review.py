from collections import deque
import ctypes
import gzip
import hashlib
import json
from pathlib import Path
import struct
import sys


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
OUT = VERIFY / 'last-hidden-v1'
ARCHIVE = VERIFY / 'win98-reference-v1'
RES = ROOT / '.local/pal/visual/pal/v1'
sys.path.insert(0, str(OUT / 'python'))
from PIL import Image, ImageChops, ImageFilter, ImageOps, ImageStat


DIRS = [(-16,8), (-16,-8), (16,-8), (16,8)]
SOURCES = []


def read(path, sha=None):
    data = path.read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    assert sha is None or digest == sha, path
    SOURCES.append({'path': path.relative_to(ROOT).as_posix(), 'sha256': digest})
    return data


def archive(data, scene):
    o = struct.unpack_from('<6I',data)
    scripts = list(struct.iter_unpack('<4H',data[o[4]:o[5]]))
    events = [None, *struct.iter_unpack('<hHHhHHh9H',data[o[0]:o[1]])]
    maps = list(struct.iter_unpack('<4H',data[o[1]:o[2]]))
    map_id, start, teleport, first = maps[scene-1]
    assert map_id == 55
    places = []
    for pc,row in enumerate(scripts):
        if row[:2] == (0x59, scene):
            assert scripts[pc+1][0] == 0x46
            _,x,y,h = scripts[pc+1]
            places.append({'transitionPC': pc, 'placementPC': pc+1, 'worldXY': [x*32+h*16, y*16+h*8]})
    return scripts, events, places, start, teleport, range(first+1, maps[scene][3]+1)


def decode(raw, method):
    a,b = struct.unpack_from('<2I',raw,55*4)
    func = getattr(ctypes.CDLL(str(ARCHIVE/'map-decoder.dylib')), method)
    func.argtypes = [ctypes.c_void_p,ctypes.c_void_p,ctypes.c_int]
    func.restype = ctypes.c_int
    buf = ctypes.create_string_buffer(65536)
    assert func(ctypes.create_string_buffer(raw[a:b]+bytes(8)),buf,65536)==65536
    return buf.raw


def limited(point):
    return 160 <= point[0] < 1856 and 112 <= point[1] < 2048


def walk(raw, places, permit_padding=False):
    tiles = struct.unpack('<16384I',raw)
    parents = dict.fromkeys(tuple(row['worldXY']) for row in places)
    queue = deque(parents)
    while queue:
        x,y = queue.popleft()
        for dx,dy in DIRS:
            a,b = position = x+dx,y+dy
            if position in parents or not (0 <= a < 2048 and 0 <= b < 2048) or (not permit_padding and not limited(position)):
                continue
            assert a%32//16 == b%16//8
            if not tiles[((b//16)*64+a//32)*2+a%32//16] & 8192:
                parents[position] = (x,y)
                queue.append(position)
    return parents


def poses(seen, x, y):
    return sorted(seen.keys() & {(x,y), *((x+dx,y+dy) for dx,dy in DIRS)})


def render(raw):
    palette = json.loads(read(RES / 'palettes.json'))[0][0]
    atlas_path = RES / 'atlases/gop/55.png'
    read(atlas_path)
    atlas = Image.open(atlas_path).convert('RGBA')
    index = atlas.getchannel('R')
    atlas = Image.merge('RGBA',(*(index.point([row[i] for row in palette]) for i in range(3)),atlas.getchannel('A')))
    frames = []
    for rect in json.loads(read(RES/'atlases/gop/55.json'))['frames']:
        frames.append(atlas.crop((rect['x'],rect['y'],rect['x']+rect['width'],rect['y']+rect['height'])) if rect and rect['width'] else None)
    surface = Image.new('RGBA',(2048,2048))
    tiles = struct.unpack('<16384I',raw)
    for layer in range(2):
        for y in range(128):
            for h in range(2):
                for x in range(64):
                    tile = tiles[(y*64+x)*2+h]>>(layer*16)
                    frame = ((tile&255)|((tile>>4)&256))-layer
                    if 0 <= frame < len(frames) and frames[frame] is not None:
                        surface.alpha_composite(frames[frame],(x*32+h*16-16,y*16+h*8-8))
    return surface.convert('RGB')


def edge(image):
    return ImageOps.autocontrast(image.convert('L').filter(ImageFilter.FIND_EDGES),cutoff=5)


def map_registration(rendered, published):
    limits = (310,460,1380,1320)
    ref = edge(published.crop(limits)).resize((268,215))
    candidates = []
    for dx in range(121,145):
        for dy in range(119,141):
            x0,y0,x1,y1 = limits
            here = edge(rendered.crop((x0+dx,y0+dy,x1+dx,y1+dy))).resize((268,215))
            diff = ImageChops.difference(ref,here)
            candidates.append((ImageStat.Stat(diff).mean[0],dx,dy))
    best,next_best = sorted(candidates)[:2]
    assert best[1:] == (128,128) and best[0] < 11 and next_best[0]-best[0] > .05
    return list(best[1:]), best[0], next_best[0]


def mark(image, offset, events, number, parents):
    _,x,y = events[number][:3]
    px,py = x-offset[0],y-offset[1]
    tile = image.crop((px-48,py-36,px+48,py+36))
    red = sum(r-g>32 and r-b>32 for r,g,b in tile.get_flattened_data())
    return {'event': number,'worldXY': [x,y],'pixels': [px,py],'redAnnotationPixels': red,'textureVariance': ImageStat.Stat(tile).var[0],'reachableOriginalSearchStances': poses(parents,x,y)}


sss_path = ROOT/'.local/pal/original-98/sss.mkf'
dos_path = ARCHIVE/'fan-dos-sss.mkf'
script,event,entry,start,teleport,selected = archive(read(sss_path,'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'),61)
dos_script,dos_event,dos_entries,_,_,_ = archive(read(dos_path,'38252bae8dabaebe9c780f00e8e6829bce56f89d862ecf9d44cc7b135047ed5d'),60)
win_tiles = decode(read(ROOT/'.local/pal/original-98/map.mkf','bef63043ffcbe9b6de6cbe295891134d7cea74952cb6701b5566049d0eb015e2'),'YJ2_Decompress')
dos_tiles = decode(read(ARCHIVE/'fan-dos-map.mkf','1f1fdf71257e3769d1ee3494336a8ae3a5d7690dce710803fa357bbc48023d88'),'YJ1_Decompress')
assert win_tiles == gzip.decompress(read(RES/'maps/55.bin.gz'))
assert len(entry)==len(dos_entries)==12 and all(limited(p['worldXY']) for p in entry)
assert script[start:start+4]==[(0x4a,39,0,0),(0x43,78,0,0),(0x45,40,0,0),(0,0,0,0)]
assert script[teleport:teleport+6]==[(0x35,0,0,0),(0x71,0,0,0),(0x59,65,0,0),(0x46,19,15,0),(0x50,0,0,0),(0,0,0,0)]
assert not any(op in (0x6d,0x99) and target==61 for op,target,_,_ in script)
assert [event[n][8] for n in selected if event[n][8]==10]==[10]*7 and event[1172][1:3]==dos_event[1197][1:3]==(768,1840)
assert script[event[1172][4]+3]==dos_script[dos_event[1197][4]+3]==(0x1f,100,0,0)
original_default = walk(win_tiles,entry)
original_padding = walk(win_tiles,entry,True)
dos_walk = walk(dos_tiles,dos_entries)
assert len(original_default)==6465 and not poses(original_default,768,1840) and poses(dos_walk,768,1840)
end = poses(original_padding,768,1840)[0]
trace = [end]
while original_padding[trace[-1]] is not None:
    trace.append(original_padding[trace[-1]])
trace.reverse()
out_of_bounds = next(index for index,p in enumerate(trace) if not limited(p))
assert out_of_bounds>0
snapshot_review = json.loads(read(VERIFY/'blackwater-extra-records-pending.json','6e072b83e03e808464c8a94fad5e0d616d1c65bdd939de6d38f3cdc398191bbb'))
record = next(r for r in snapshot_review['records'] if r['event']==1172)
assert record['originalRecordUnchanged'] and not record['normalPickupVerified'] and record['sameSceneSearchStances'] == []
for label in ['before','after']:
    state = json.loads(read(ROOT/record[label]['path'],record[label]['sha256']))
    assert state['public']['field']['offset']==[160,112]
assert state['public']['scene']==61
page = read(OUT/'palhero-maps-index.html','ad5119b1a96eff040b04fe40ffe8c1f4fef9923c5402a5157538f1d27ff9c988').decode('utf-8')
assert '《仙剑98柔情篇》迷宫地图' in page and 'strategies_pal98maps-017.jpg' in page
independent = OUT/'palhero-tomb-upper.jpg'
read(independent,'7488a105ca773b61b65af4c83a2e0ef94ac0d4e712861774cd658803368017d9')
published = Image.open(independent).convert('RGB')
assert published.size == (1727,1817)
offset, score, next_score = map_registration(render(win_tiles),published)
controls = [mark(published,offset,event,n,original_default) for n in range(1168,1175) if n!=1172]
target = mark(published,offset,event,1172,original_default)
assert target['pixels']==[640,1712] and target['redAnnotationPixels']==0 and target['textureVariance']<100
assert all(r['redAnnotationPixels']>=300 and r['textureVariance']>500 and r['reachableOriginalSearchStances'] for r in controls)
report = {'passed': True,'readyForProgress': True,'nativeEdits': False,'actuallyObtained': False,'classification': 'original-unreachable-preserved','event': 1172,'scene': 61,'map': 55,'position': [768,1840],'grant': 100,'scope': 'Windows active bounds and every raw entrance, ignoring dynamic blockers and all outward portals; separately published 98 map is position-registered against six known reachable rewards, DOS is comparison only','originalEntries': entry,'bounds': {'partyOffset': [160,112],'leftInclusive': 160,'rightExclusive': 1856,'minimumY': 112,'mapYLimit': 2048},'reachableOriginalSearchStances': poses(original_default,768,1840),'illegalPaddedRoute': {'firstOutOfBounds': trace[out_of_bounds],'previousPosition': trace[out_of_bounds-1],'entry': trace[0],'targetSearchStance': end,'totalSteps': len(trace)-1},'patchedDOS': {'event': 1197,'samePosition': True,'reachableSearchStances': poses(dos_walk,768,1840)},'teleport': {'sceneScript': teleport,'exitScene': 65},'independentMap': {'guideURL': 'https://www.palhero.net/?p=1243','imageURL': 'https://dl.palhero.net/palhero/maps/strategies_pal98maps-017.jpg','offset': offset,'registrationScore': score,'secondScore': next_score,'target': target,'controls': controls},'unchangedSnapshot': record,'sources': SOURCES}
(OUT/'tomb-upper-reward.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'passed': True,'event': 1172,'originalAccessible': False,'independentControls': len(controls),'firstOutOfBounds': trace[out_of_bounds]},ensure_ascii=False))
