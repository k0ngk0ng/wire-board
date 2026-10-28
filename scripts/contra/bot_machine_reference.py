"""Read pinned Japanese machine artwork before declaring visual recipes.

Failure inventory: treating carts' OAM as background; guessing an actor from a
shared/erased image; wrong animation stride/height; inferring HP/timer/slot/live
level; selecting CHR 0 for a palette-only alternate; missing retained palette
quadrants; deriving expected art from the recipe under test.
"""
from pathlib import Path
import gzip,hashlib,json,zipfile
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-machine-vision'
ROM_PATH=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def read(name):
    p=ROOT/name;raw=p.read_bytes();return json.loads(gzip.decompress(raw)if name.endswith('.gz')else raw),{'path':name,'sha256':sha(raw)}

def build():
    archive=sha(ROM_PATH.read_bytes())
    with zipfile.ZipFile(ROM_PATH)as z:rom=z.read('Contra (J).nes')
    assert sha(rom)==SHA and archive==sha(ROM_PATH.read_bytes())
    catalog,source=read('docs/research/nes-contra/evidence/levels-catalog.json.gz')
    graphics,gs=read('games/contra/engine/testdata/original-enemy-machines-graphics-controlled.json.gz')
    assert catalog['romSHA256']==SHA and graphics['source_is_original_execution']
    sources=[source,gs];patterns=[];sets={t['id']:t for t in catalog['tileSets']}
    for p in catalog['chrPresets']:
        ref=p['source'];assert list(rom[ref['romOffset']:ref['romOffset']+ref['length']])==p['pages']
    for level in [5,6]:
        s=catalog['stages'][level];h=s['header'];ref=h['source'];assert list(rom[ref['romOffset']:ref['romOffset']+ref['length']])==h['bytes']
        assert s['initialPaletteState']['paletteIndices']==h['bytes'][16:24]
        alt=s['alternate'];ref=alt['source'];assert list(rom[ref['romOffset']+7:ref['romOffset']+15])==alt['paletteIndices']
        ts=sets[catalog['stages'][level]['tileSetId']]
        for field,data in [('tilesSource',bytes(v for b in ts['supertiles']for v in b['tiles'])),('attributesSource',bytes(b['attribute']for b in ts['supertiles']))]:
            p=ts[field];assert sha(data)==p['sha256'] and rom[p['romOffset']:p['romOffset']+p['length']]==data
        for table in ['base','update']:
            first=0 if table=='base'else ts['updates']['baseSupertile']
            for code,b in enumerate(ts['supertiles'][first:]):patterns.append({'level':level,'table':table,'code':code,'columns':4,'rows':4,'tiles':b['tiles'],'attribute':b['attribute']})
        d=graphics['definitions'][str(level)];raw=bytes.fromhex(d['raw']);assert sha(raw)==d['sha256']and rom[16+d['prgOffset']:16+d['prgOffset']+len(raw)]==raw
        for code in range(len(raw)//5):
            start=code*5;header=raw[start];rows=(header&7)if header&128 else 2
            assert rows and start+1+rows*2<=len(raw)
            patterns.append({'level':level,'table':'tile-animation','code':code,'columns':2,'rows':rows,'tiles':list(raw[start+1:start+1+rows*2]),'attribute':header})
    observations=[]
    natural,ns=read('games/contra/engine/testdata/original-enemy-machines-graphics-natural.json.gz');sources.append(ns)
    for r in natural['records']:
        if r['carry']:continue
        b=r['before'];level=b['level'];ts=sets[catalog['stages'][level]['tileSetId']]['supertiles'];mapped=bytes.fromhex(b['supertiles'])
        sy=r['y']+b['scrollY'];py=(sy+(16 if sy>=240 else 0))&255
        sx=r['x']+b['scrollX'];px=sx&255;page=(b['control']&1)^(sx>=256)
        tx,ty=(px&248)//8,(py&248)//8
        pattern=next(p for p in patterns if p['level']==level and p['table']=='tile-animation'and p['code']==(r['code']&127))
        groups=[]
        for dy in range(pattern['rows']):
            for dx in range(2):
                x,y=tx+dx,ty+dy;index=page*64+(y//4)*8+x//4
                assert index<len(mapped)
                attr=ts[mapped[index]]['attribute'];groups.append((attr>>(((y&2)|((x>>1)&1))*2))&3)
        observations.append({'record':r['index'],'level':level,'code':r['code']&127,'x':r['x'],'y':r['y'],'groups':groups})
    rea,rs=read('docs/research/nes-contra/evidence/enemies-machines-rea.json.gz');sources.append(rs)
    spawns,ss=read('docs/research/nes-contra/evidence/enemies-all-stage-data.json');sources.append(ss)
    by={(p['level'],p['table'],p['code']):p for p in patterns}
    maps={level:{m['index']:m['supertiles']for m in catalog['stages'][level]['maps']['entries']}for level in[5,6]}
    def groups_at(level,tx,ty,columns,rows):
        result=[]
        for dy in range(rows):
            for dx in range(columns):
                x,y=tx+dx,ty+dy;index=y//4*8+(x%32)//4
                assert 0<=y<28 and x//32 in maps[level]
                code=maps[level][x//32][index];attr=by[(level,'base',code)]['attribute']
                result.append((attr>>(((y&2)|((x>>1)&1))*2))&3)
        return result
    flames=[];launchers=[];claw_origins=[];claw_path=[]
    for level in[5,6]:
        for script in spawns['stages'][level]['scripts']:
            raw=script['bytes'];at=0
            assert list(rom[script['romOffset']:script['romOffset']+len(raw)])==raw
            while raw[at]!=255:
                trigger=raw[at]&254;kind=raw[at+1]&63;n=(raw[at+1]>>6)+1
                for position in raw[at+2:at+2+n]:
                    x=script['screen']*256+trigger+240;y=position&240
                    if level==5 and kind in[16,17,18]:
                        for segment in range([5,9,13,15][position&3]):
                            dx=(-8*segment if kind==17 else 8*segment)if kind!=16 else 0;dy=8*segment if kind==16 else 0
                            tx=(x-7+dx)//8;ty=((y+8+dy-1+224+16)%256)//8
                            groups=groups_at(level,tx,ty,2,2);assert groups==[3]*4
                            flames.append({'screen':script['screen'],'trigger':trigger,'kind':kind,'position':position,'segment':segment,'groups':groups})
                    if level==6 and kind==23:
                        groups=groups_at(level,(x-4)//8,(y-16)//8,2,2);assert groups==[2]*4
                        launchers.append({'screen':script['screen'],'trigger':trigger,'groups':groups})
                    if level==6 and kind==16:
                        claw_origins.append([x//256,(x%256)//8,(y-16)//8])
                        for segment in range([4,3,8,3][position&3]):
                            groups=groups_at(level,x//8,(y-16)//8+segment,2,3);assert groups==[0]*6
                            claw_path.append({'screen':script['screen'],'trigger':trigger,'position':position,'segment':segment,'groups':groups})
                at+=2+n
    claw_aliases={}
    for code in[0,2,4,6]:
        p=by[(6,'tile-animation',code)];hits=[]
        for screen,blocks in maps[6].items():
            tiles=[];groups=[]
            for y in range(28):
                for x in range(32):
                    b=by[(6,'base',blocks[y//4*8+x//4])];tiles.append(b['tiles'][(y%4)*4+x%4]);groups.append((b['attribute']>>(((y&2)|((x>>1)&1))*2))&3)
            for y in range(29-p['rows']):
                for x in range(31):
                    idx=[(y+dy)*32+x+dx for dy in range(p['rows'])for dx in range(2)]
                    if[tiles[i]for i in idx]==p['tiles']and all(groups[i]==0 for i in idx):hits.append([screen,x,y])
        claw_aliases[str(code)]=hits
    assert sorted(claw_aliases['0'])==sorted(claw_origins)
    assert all([s,x,y+1]in claw_origins for s,x,y in claw_aliases['6'])
    assert list(rom[16+0x1d04e:16+0x1d052])==[15,32,22,17]
    appearances=[];presets={p['id']:p['pages']for p in catalog['chrPresets']}
    for level,alternate in[(5,False),(6,False),(6,True)]:
        stage=catalog['stages'][level];h=stage['alternate']if alternate else stage['initialPaletteState'];mapping=presets[stage['chrPreset']]
        palettes=[h['paletteIndices']]
        for phase in range(4):
            p=h['paletteIndices'].copy();p[3]=h['cycleIndices'][phase]
            if not alternate:p[2]=[4,92,4,93][phase]
            if p not in palettes:palettes.append(p)
        appearances.append({'level':level,'alternate':alternate,'chr':mapping,'palettes':palettes,'backdrops':[15,32,22,17]})
    manifest=json.loads((ROOT/'.local/contra-browser/build/public/manifest.json').read_text())
    rgbRef=manifest['rgb'];rgbPath=ROOT/'.local/contra-browser/build/public'/rgbRef['url'].lstrip('/');rgb=rgbPath.read_bytes();assert len(rgb)==192 and sha(rgb)==rgbRef['sha256']
    sources.append({'path':str(rgbPath.relative_to(ROOT)),'sha256':sha(rgb)})
    result={'version':1,'romSHA256':SHA,'archiveSHA256':archive,'sources':sources,'patterns':patterns,'clawPaletteObservations':observations,
            'stages':[catalog['stages'][i]for i in[5,6]],'chrPresets':catalog['chrPresets'],'reaEvidenceIds':[x['evidence']['evidence_id']for x in rea['records']],
            'flamePaletteProof':flames,'launcherPaletteProof':launchers,'clawStaticImages':claw_aliases,'clawSpawnAnchors':claw_origins,'clawPathPaletteProof':claw_path,'appearances':appearances,
            'scope':'Immutable artwork and explicitly labeled original observations; no live World or recipe implementation input.'}
    # Source-based review selection, prepared before the Go declarations. These
    # complete images are decoded directly from the original planar CHR bytes.
    selection=[(5,False,'tile-animation',[1,2],3,18,8,8),(5,False,'tile-animation',[3],3,18,4,8),
               (5,False,'tile-animation',[4,5],3,17,8,8),(5,False,'tile-animation',[6],3,17,12,8),
               (5,False,'tile-animation',[7,8],3,16,8,8),(5,False,'tile-animation',[9],3,16,8,4),
               (6,False,'tile-animation',[0,2,4],0,16,8,8),(6,False,'tile-animation',[6],0,16,8,16),
               (6,True,'tile-animation',[9,10],2,23,8,8),(6,True,'base',[0x5e],3,22,16,16)]
    def pixels(p,mapping,indices,backdrop,group):
        palette=[]
        for index in indices:palette.extend([15,*rom[16+0x1d0e3+index*3:16+0x1d0e6+index*3]])
        out=bytearray()
        for y in range(p['rows']*8):
            for x in range(p['columns']*8):
                tile=p['tiles'][(y//8)*p['columns']+x//8];at=16+131072+mapping[4+(tile>>6)]*1024+(tile&63)*16;bit=7-x%8
                value=((rom[at+y%8]>>bit)&1)|(((rom[at+y%8+8]>>bit)&1)<<1)
                g=group if p['table']=='tile-animation'else(p['attribute']>>((y//16*2+x//16)*2))&3
                color=palette[g*4+value]if value else backdrop
                out.extend(rgb[color*3:color*3+3]);out.append(255)
        return out.hex()
    cases=[]
    for lv,alt,table,codes,group,kind,ax,ay in selection:
        a=next(a for a in appearances if a['level']==lv and a['alternate']==alt)
        for code in codes:
            p=by[(lv,table,code)]
            for indices in a['palettes']:
                for backdrop in a['backdrops']:
                    cases.append({'level':lv,'alternate':alt,'table':table,'code':code,'kind':kind,'width':p['columns']*8,'height':p['rows']*8,'anchorX':ax,'anchorY':ay,'group':group,
                                  'chr':a['chr'],'paletteIndices':indices,'backdrop':backdrop,'rgba':pixels(p,a['chr'],indices,backdrop,group)})
    negatives=[]
    for lv,alt,table,codes,group in[(5,False,'tile-animation',[0,10,11,12,13],3),(6,False,'tile-animation',[1,3,5,7],0),
                                    (6,True,'tile-animation',[8,11],2),(6,True,'update',[5],0),(6,False,'tile-animation',[0],1),(5,False,'tile-animation',[2],1)]:
        a=next(a for a in appearances if a['level']==lv and a['alternate']==alt)
        for code in codes:
            p=by[(lv,table,code)];negatives.append({'level':lv,'alternate':alt,'table':table,'code':code,'width':p['columns']*8,'height':p['rows']*8,'rgba':pixels(p,a['chr'],a['palettes'][0],15,group)})
    result['visualCases']=cases;result['negativeCases']=negatives
    return result,rom,rgb

def render(data,rom,rgb):
    from PIL import Image,ImageDraw
    OUT.mkdir(parents=True,exist_ok=True)
    chr=rom[16+131072:];presets={p['id']:p['pages']for p in data['chrPresets']}
    def pixel(tile,x,y,mapping):
        at=mapping[4+(tile>>6)]*1024+(tile&63)*16;bit=7-x
        return((chr[at+y]>>bit)&1)|(((chr[at+y+8]>>bit)&1)<<1)
    def palette(settings):
        raw=[]
        for p in settings['paletteIndices']:
            start=16+0x1d0e3+p*3;raw.extend([15,*rom[start:start+3]])
        return raw
    def tile_image(p,mapping,colors,group=None):
        im=Image.new('RGB',(p['columns']*8,p['rows']*8))
        for y in range(im.height):
            for x in range(im.width):
                value=pixel(p['tiles'][(y//8)*p['columns']+x//8],x%8,y%8,mapping)
                g=group if group is not None else(p['attribute']>>((y//16*2+x//16)*2))&3
                color=colors[g*4+value]if value else 15
                im.putpixel((x,y),tuple(rgb[color*3:color*3+3]))
        return im
    paths=[]
    for stage in data['stages']:
        level=stage['stage']-1
        for alternate in [False,True]:
            settings=stage['alternate']if alternate else stage['initialPaletteState']
            preset=(stage['alternate']['chrPreset']or stage['chrPreset'])if alternate else stage['chrPreset']
            mapping=presets[preset];colors=palette(settings)
            pats=[p for p in data['patterns']if p['level']==level and p['table']=='base']
            im=Image.new('RGB',(96*12,98*((len(pats)+11)//12)),(225,225,225));draw=ImageDraw.Draw(im)
            update=next(p['code']for p in pats if p['tiles']==next(q['tiles']for q in data['patterns']if q['level']==level and q['table']=='update'and q['code']==0))
            for i,p in enumerate(pats):
                x,y=(i%12)*96,(i//12)*98;im.paste(tile_image(p,mapping,colors).resize((64,64),Image.Resampling.NEAREST),(x+16,y))
                label=f'B{i:02X}'+(f'/U{i-update:02X}'if i>=update else'');draw.text((x+10,y+68),label,fill=(0,0,0))
            target=OUT/f'stage-{level+1}-{"alternate"if alternate else"regular"}-supertiles.png';im.save(target);paths.append(target)
            anim=[p for p in data['patterns']if p['level']==level and p['table']=='tile-animation']
            im=Image.new('RGB',(88*len(anim),4*170),(225,225,225));draw=ImageDraw.Draw(im)
            for group in range(4):
                for i,p in enumerate(anim):
                    tile=tile_image(p,mapping,colors,group);x,y=i*88,group*170
                    im.paste(tile.resize((tile.width*3,tile.height*3),Image.Resampling.NEAREST),(x+20,y))
                    draw.text((x+6,y+146),f'A{i:02X} G{group}',fill=(0,0,0))
            target=OUT/f'stage-{level+1}-{"alternate"if alternate else"regular"}-animations.png';im.save(target);paths.append(target)
            if alternate:
                last=14 if level==6 else 12
                m=next(m for m in stage['maps']['entries']if m['index']==last)
                im=Image.new('RGB',(256,224));by={p['code']:p for p in pats}
                for i,code in enumerate(m['supertiles']):im.paste(tile_image(by[code],mapping,colors),((i%8)*32,(i//8)*32))
                target=OUT/f'stage-{level+1}-boss-map-{last}.png';im.resize((768,672),Image.Resampling.NEAREST).save(target);paths.append(target)
    return paths

if __name__=='__main__':
    data,rom,rgb=build();OUT.mkdir(parents=True,exist_ok=True)
    target=ROOT/'games/contra/engine/testdata/bot-machine-original-art.json.gz';target.write_bytes(gzip.compress(json.dumps(data,sort_keys=True,separators=(',',':')).encode(),mtime=0))
    paths=render(data,rom,rgb)
    print(json.dumps({'patterns':len(data['patterns']),'observations':len(data['clawPaletteObservations']),'fixtureSHA256':sha(target.read_bytes()),'sheets':[str(p.relative_to(ROOT))for p in paths]}))
