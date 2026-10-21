#!/usr/bin/env python3
"""Strict, deterministic extraction of the retained 2002 Fx map/tile resources.

Failure inventory was committed to FORMAT-PLAN.md before implementation. Never
execute CA.exe. Unresolved raw fields are retained, not assigned invented rules.
"""
import argparse
import hashlib
import json
import re
import struct
import zlib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SOURCE_HASHES = {
    'client-2002/Fx.idd': '3554b3cda62313366d07776c48c203ddc95092b18b753b4d0986fab19ed664b2',
    'client-2002/Fx.idx': '6be28522015c5a4b0f4b8ab86a989819d1b0b83f676566d4b99cc4c8d15aba7c',
    'client-2002/CA.exe': '194a86b9e95ff9c2f18eff19030c3f726aa62df3dddbce1dcc4df87ed8fd5b8a',
    '2002-fx-index.json': 'c4eb4ef9b5ae88f6d640db29af7cb5a262d19531905ab4b0cac087ce973bf4b4',
}

def sha(data):
    return hashlib.sha256(data).hexdigest()

class Reader:
    def __init__(self, data):
        self.data, self.pos = data, 0
    def take(self, n):
        assert n >= 0 and self.pos + n <= len(self.data), ('truncated', self.pos, n, len(self.data))
        b = self.data[self.pos:self.pos+n]
        self.pos += n
        return b
    def ints(self, n=1):
        return list(struct.unpack('<' + str(n) + 'i', self.take(n*4)))
    def integer(self):
        return self.ints()[0]
    def string(self, allow_empty=False):
        n = self.integer()
        assert (0 if allow_empty else 1) <= n < 256, ('name length', n)
        return self.take(n).decode('ascii')
    def done(self):
        assert self.pos == len(self.data), ('unconsumed', self.pos, len(self.data))

def png(w, h, rgba):
    assert len(rgba) == w*h*4
    def chunk(kind, b):
        return struct.pack('>I', len(b))+kind+b+struct.pack('>I', zlib.crc32(kind+b)&0xffffffff)
    rows = b''.join(b'\0'+rgba[y*w*4:(y+1)*w*4] for y in range(h))
    return b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>2I5B', w,h,8,6,0,0,0))+chunk(b'IDAT',zlib.compress(rows))+chunk(b'IEND',b'')

def rgb565(word):
    r,g,b = (word>>11)&31, (word>>5)&63, word&31
    return bytes(((r<<3)|(r>>2), (g<<2)|(g>>4), (b<<3)|(b>>2), 255))

def decode_sprite(b):
    r = Reader(b)
    n, fmt, w, h = r.ints(4)
    assert fmt == 1 and 0 < n < 1000 and 0 < w < 4096 and 0 < h < 4096
    frames = [r.ints(7) for _ in range(n)]
    assert all(0 <= f[0] < f[2] <= w and 0 <= f[1] < f[3] <= h for f in frames)
    size = r.integer()
    assert size == len(b)-r.pos
    rgba = bytearray()
    while r.pos < len(b):
        opcode = r.take(1)[0]
        kind, count = opcode>>6, opcode&63
        assert kind < 3 and count > 0
        if kind == 0:
            rgba.extend(b''.join(rgb565(x) for x in struct.unpack('<'+str(count)+'H',r.take(count*2))))
        elif kind == 1:
            rgba.extend(rgb565(struct.unpack('<H',r.take(2))[0])*count)
        else:
            rgba.extend(bytes(count*4))
        assert len(rgba) <= w*h*4
    assert len(rgba) == w*h*4
    r.done()
    return dict(width=w,height=h,framesRaw=frames,pixelEncoding='RGB565 RLE'), bytes(rgba)

def extract(source_dir, out, asset_dir):
    for file,expected in SOURCE_HASHES.items():
        assert sha((source_dir/file).read_bytes()) == expected, ('source identity changed', file)
    raw = (source_dir/'client-2002/Fx.idd').read_bytes()
    idx = json.loads((source_dir/'2002-fx-index.json').read_text())
    lookup = {e['name']: e for e in idx}
    assert len(idx) == len(lookup) == 1164
    end = 0
    for e in sorted(idx,key=lambda e:e['offset']):
        assert end <= e['offset'] <= e['offset']+e['size'] <= len(raw)
        end = e['offset']+e['size']
    def member(name):
        candidates = [k for k in lookup if k.casefold() == name.casefold()]
        assert len(candidates) == 1, ('resource casefold ambiguity', name, candidates)
        e = lookup[candidates[0]]
        b = raw[e['offset']:e['offset']+e['size']]
        return b, dict(e,sha256=sha(b))
    table, table_source = member('ObjectTable/ObjectTable')
    r = Reader(table)
    count = r.integer()
    assert count == 490
    objects = [dict(id=i,name=r.string(),bodyOffset=r.integer()) for i in range(count)]
    bodies = [o for o in objects if o['bodyOffset'] >= 0]
    assert [o['bodyOffset'] for o in bodies] == sorted(set(o['bodyOffset'] for o in bodies))
    assert bodies[0]['bodyOffset'] == r.pos
    for i,o in enumerate(bodies):
        end = bodies[i+1]['bodyOffset'] if i+1 < len(bodies) else len(table)
        b = table[o['bodyOffset']:end]
        q = Reader(b)
        n = q.integer()
        assert 0 < n < 1000
        sequences=[]
        for _ in range(n):
            name, resource, frames = q.string(), q.string(allow_empty=True), q.integer()
            assert 0 <= frames < 10000
            sequences.append(dict(name=name,resource=resource,framePairsRaw=[q.ints(2) for _ in range(frames)]))
        q.done()
        o.update(bodySize=len(b),bodySha256=sha(b),sequences=sequences)
    info, info_source = member('TileInfo/TileInfo')
    r=Reader(info)
    n=r.integer()
    assert n==388
    infos={}
    for i in range(n):
        start=r.pos
        name=r.string()
        fields=r.ints(11)
        assert name not in infos and fields[0] in range(4)
        infos[name]=dict(ordinal=i,recordOffset=start,fieldsRaw=fields)
    r.done()
    maps=[]
    used=set()
    for name in sorted(lookup):
        if not name.startswith('Map/'):
            continue
        b,src=member(name)
        r=Reader(b)
        w,h,n,field12,field16=r.ints(5)
        assert 0 < w <= 15 and 0 < h <= 13 and 0 < n <= 8
        spawns=[r.ints(2) for _ in range(n)]
        assert all(0 <= x < w and 0 <= y < h for x,y in spawns)
        assert len(set(map(tuple,spawns)))==n
        link_offset=r.pos
        links_count=r.integer()
        assert 0 <= links_count <= 10
        links=[r.ints(4) for _ in range(links_count)]
        assert all(0 <= p[0] < w and 0 <= p[1] < h and 0 <= p[2] < w and 0 <= p[3] < h for p in links)
        cells_offset=r.pos
        cells=[r.ints(2) for _ in range(w*h)]
        assert all(-1 <= v < count for cell in cells for v in cell)
        assert all(v==-1 or objects[v]['name'] in infos for cell in cells for v in cell)
        used.update(v for cell in cells for v in cell if v>=0)
        items_offset=r.pos
        item_n=r.integer()
        assert 0 <= item_n < 100
        items=[dict(name=r.string(),weightRaw=r.integer()) for _ in range(item_n)]
        assert all(i['name'].startswith('Item/') and i['weightRaw']>=0 for i in items)
        r.done()
        source_name=name.split('/')[1]
        family=re.sub(r'\d+$','',source_name)
        maps.append(dict(id='classic2002-'+source_name.lower(),sourceName=name,source=src,family=family,
            sourceNumber=int(source_name[len(family):]),width=w,height=h,spawns=spawns,
            ground=[[cells[y*w+x][0] for x in range(w)] for y in range(h)],
            objects=[[cells[y*w+x][1] for x in range(w)] for y in range(h)],
            metadata=dict(headerFieldsRaw=[w,h,n,field12,field16],spawnOffset=20,linkOffset=link_offset,
                pairedCoordinatesRaw=links,cellsOffset=cells_offset,itemWeightsOffset=items_offset,itemWeights=items)))
    assert len(maps)==112
    tiles=[]
    resources=set()
    for id_ in sorted(used):
        o=objects[id_]
        assert o['bodyOffset']>=0
        inf=infos[o['name']]
        fields=inf['fieldsRaw']
        typ=fields[0]
        tile=dict(o,tileInfo=inf,category=['ground','static','box','passable'][typ])
        if typ==2:
            assert fields[2] in (0,1)
            tile['movable']=fields[2]!=0
        if typ in (0,1):
            tile['footprintRaw']=fields[1:3]
        tiles.append(tile)
        resources.update(s['resource'] for s in o['sequences'])
    sprites=[]
    decoded={}
    asset_dir.mkdir(parents=True,exist_ok=True)
    for name in sorted(resources):
        b,src=member(name)
        meta,pixels=decode_sprite(b)
        decoded[name]=(meta,pixels)
        file=re.sub('[^a-zA-Z0-9_-]','_',name)+'.png'
        data=png(meta['width'],meta['height'],pixels)
        (asset_dir/file).write_bytes(data)
        sprites.append(dict(name=name,source=src,**meta,file=file,sha256=sha(data),rgbaSha256=sha(pixels)))
    sprite_by_name={s['name']:s for s in sprites}
    for t in tiles:
        for s in t['sequences']:
            assert all(0 <= pair[0] < len(sprite_by_name[s['resource']]['framesRaw']) for pair in s['framePairsRaw'])
    # Resource previews use the original frame rectangle and anchor at tile bottom.
    preview_dir=asset_dir.parent/'previews'
    preview_dir.mkdir(parents=True,exist_ok=True)
    tile_by_id={t['id']:t for t in tiles}
    for m in maps:
        w,h=m['width']*40,m['height']*40
        pixels=bytearray(w*h*4)
        for layer in ['ground','objects']:
            for y,row in enumerate(m[layer]):
                for x,id_ in enumerate(row):
                    if id_<0:
                        continue
                    t=tile_by_id[id_]
                    seq=next((q for q in t['sequences'] if q['name']=='Default'),None)
                    assert seq is not None, ('missing default pose',t['name'])
                    meta,src=decoded[seq['resource']]
                    f=meta['framesRaw'][seq['framePairsRaw'][0][0]]
                    for sy in range(f[1],f[3]):
                        dy=y*40+39+sy-f[1]-f[5]
                        if not 0 <= dy < h:
                            continue
                        for sx in range(f[0],f[2]):
                            dx=x*40+sx-f[0]-f[4]
                            if 0 <= dx < w:
                                si=(sy*meta['width']+sx)*4
                                if src[si+3]:
                                    di=(dy*w+dx)*4
                                    pixels[di:di+4]=src[si:si+4]
        data=png(w,h,pixels)
        file=m['id']+'.png'
        (preview_dir/file).write_bytes(data)
        m['preview']=dict(file=file,width=w,height=h,sha256=sha(data),rendering='original default sprite rectangles, cell=40, tile bottom anchor; static reconstruction, not runtime screenshot')
    attr,attr_source=member('Attr/BomberAttr')
    r=Reader(attr)
    characters=[]
    while r.pos<len(attr):
        start=r.pos
        name=r.string()
        f=r.ints(10)
        characters.append(dict(name=name,recordOffset=start,fieldsRaw=f,initialPixelsPerTick=f[1],maximumPixelsPerTick=f[2]))
    r.done()
    assert len(characters)==10
    sources=[]
    for file in ['client-2002/Fx.idd','client-2002/Fx.idx','client-2002/CA.exe','2002-fx-index.json']:
        b=(source_dir/file).read_bytes()
        sources.append(dict(path=file,size=len(b),sha256=sha(b)))
    catalog=dict(schemaVersion=1,edition='2002 retained client; source identifiers do not assert later regional map numbering',
        sourceFiles=sources,characterAttributes=dict(source=attr_source,tickMS=30,cellPixels=40,characters=characters),objectTableSource=table_source,tileInfoSource=info_source,maps=maps,tiles=tiles,sprites=sprites,
        limitations=['Raw header fields at offsets 12 and 16 retained without invented meanings.',
            'Paired ground-coordinate links are observed in map loader; gameplay timing not dynamically observed.',
            'TileInfo fields are type-dependent; unassigned words may contain stale editor memory.',
            'No claim of complete historical map library outside this exact Fx.idd.',
            'Static analysis and resource decoding do not establish a live original gameplay run.'])
    out.parent.mkdir(parents=True,exist_ok=True)
    out.write_text(json.dumps(catalog,ensure_ascii=False,separators=(',',':'))+'\n')
    print(json.dumps(dict(catalog=str(out.relative_to(ROOT)),maps=len(maps),tiles=len(tiles),sprites=len(sprites),sha256=sha(out.read_bytes()))))

if __name__=='__main__':
    ap=argparse.ArgumentParser()
    ap.add_argument('--source',type=Path,default=ROOT/'.local/bubbleparty-original')
    ap.add_argument('--out',type=Path,default=ROOT/'games/bubbleparty/data/classic-2002.json')
    ap.add_argument('--assets',type=Path,default=ROOT/'games/bubbleparty/assets/public/bubbleparty/v4/classic/sprites')
    args=ap.parse_args()
    assert args.out.resolve().is_relative_to(ROOT) and args.assets.resolve().is_relative_to(ROOT)
    extract(args.source.resolve(),args.out.resolve(),args.assets.resolve())
