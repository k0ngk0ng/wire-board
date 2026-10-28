"""Original fixed Alien's Lair artwork and legal visual context evidence.

Failures considered before recipes: wrong source table/CHR pages; treating
0x8x retained attributes as newly installed attributes; labeling closed jaws,
rubble or decoration as a live weak point; hiding same-picture ambiguity using
HP, routine, slot, timing or the current match's level; mutable global catalog.
This script never reads a live match and never derives expected art from recipes.
"""
from pathlib import Path
import gzip,hashlib,json,zipfile

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/contra-bot-aliens'
ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
def sha(data):return hashlib.sha256(data).hexdigest()

def main():
    from PIL import Image,ImageDraw
    OUT.mkdir(parents=True,exist_ok=True)
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as archive:rom=archive.read('Contra (J).nes')
    assert sha(rom)==ROM_SHA
    path=ROOT/'docs/research/nes-contra/evidence/levels-catalog.json.gz';packed=path.read_bytes();catalog=json.loads(gzip.decompress(packed));assert catalog['romSHA256']==ROM_SHA
    stage=catalog['stages'][7];tiles=next(t for t in catalog['tileSets']if t['id']==stage['tileSetId']);blocks=tiles['supertiles'];base=tiles['updates']['baseSupertile'];assert base==0x49
    sources=[{'path':str(path.relative_to(ROOT)),'sha256':sha(packed)}]
    def verify(ref,data):
        assert len(data)==ref['length']and sha(data)==ref['sha256']and rom[ref['romOffset']:ref['romOffset']+len(data)]==data
    verify(tiles['tilesSource'],bytes(v for block in blocks for v in block['tiles']))
    verify(tiles['attributesSource'],bytes(block['attribute']for block in blocks))
    verify(stage['header']['source'],bytes(stage['header']['bytes']))
    alternate=stage['alternate'];verify(alternate['source'],bytes(alternate['collisionLimits']+alternate['cycleIndices']+alternate['paletteIndices']))
    for item in stage['maps']['entries']:verify(item['source'],bytes.fromhex(item['encoded']))
    presets={p['id']:p for p in catalog['chrPresets']}
    for preset in (7,17):verify(presets[preset]['source'],bytes(presets[preset]['pages']))
    colors={p['id']:p for p in catalog['paletteColors']}
    for code in range(72,81):verify(colors[code]['source'],bytes(colors[code]['colors']))
    public=ROOT/'.local/contra-browser/build/public';manifest=json.loads((public/'manifest.json').read_text());rgbRef=manifest['rgb'];rgb=(public/rgbRef['url'].lstrip('/')).read_bytes();assert sha(rgb)==rgbRef['sha256']
    sources.append({'path':str((public/rgbRef['url'].lstrip('/')).relative_to(ROOT)),'sha256':sha(rgb),'role':'original calibrated NES RGB palette'})
    animationPath=ROOT/'docs/research/nes-contra/evidence/levels-chr-animation.json';animation=json.loads(animationPath.read_text());sources.append({'path':str(animationPath.relative_to(ROOT)),'sha256':sha(animationPath.read_bytes())})
    row=animation['rows'][7];raw=bytes([row['backgroundSlot']-4]+row['pages']);location=row['source'];assert sha(raw)==location['sha256']and rom[location['romOffset']:location['romOffset']+5]==raw
    companion=animation['secondaryLair'];offset=16+0x10000+companion['sourceCPU'];assert rom[offset:offset+4]==bytes(companion['pages'])
    chrBase=16+131072
    pages={}
    def page(number):
        if number not in pages:
            raw=rom[chrBase+number*1024:chrBase+(number+1)*1024];pixels=[]
            for y in range(64):
                for x in range(64):
                    tile=(y//8)*8+x//8;row=y&7;bit=7-(x&7)
                    pixels.append(((raw[tile*16+row]>>bit)&1)|(((raw[tile*16+8+row]>>bit)&1)<<1))
            pages[number]=bytes(pixels)
        return pages[number]
    def palette(indices):
        return [v for code in indices for v in [15]+colors[code]['colors']]
    def render(block,mapping,indices,attribute=None):
        result=Image.new('RGB',(32,32));pal=palette(indices);attr=block['attribute']if attribute is None else attribute
        for y in range(32):
            for x in range(32):
                tile=block['tiles'][(y//8)*4+x//8];local=tile&63;pixels=page(mapping[4+(tile>>6)])
                value=pixels[((local>>3)*8+y%8)*64+(local&7)*8+x%8];group=(attr>>((y//16*2+x//16)*2))&3
                color=pal[group*4+value]if value else 15;result.putpixel((x,y),tuple(rgb[color*3:color*3+3]))
        return result
    contexts=[]
    for alternate in (False,True):
        settings=stage['alternate']if alternate else stage['initialPaletteState'];mapping=presets[17 if alternate else 7]['pages'];cycles=sorted(set(settings['cycleIndices']))
        for phase in range(4):
            mapped=mapping.copy();mapped[row['backgroundSlot']]=row['pages'][phase]
            if not alternate:mapped[companion['slot']]=companion['pages'][phase]
            for cycle in cycles:
                indices=settings['paletteIndices'].copy();indices[3]=cycle
                contexts.append({'alternate':alternate,'mapping':mapped,'paletteIndices':indices,'sourcePhase':phase})
    patterns=[]
    for table in ('base','update'):
        for code in range(len(blocks)if table=='base'else tiles['updates']['count']):
            block=blocks[code if table=='base'else base+code]
            patterns.append({'table':table,'code':code,'tiles':block['tiles'],'attribute':block['attribute']})
    observations=[]
    for name in ('original-enemies-aliens-boss.json.gz','original-enemies-aliens-boss-guardian-sequence.json.gz','original-enemies-aliens-boss-heart-sequence.json.gz'):
        path=ROOT/'games/contra/engine/testdata'/name;raw=path.read_bytes();data=json.loads(gzip.decompress(raw));assert data['source_is_original_execution'];sources.append({'path':str(path.relative_to(ROOT)),'sha256':sha(raw)})
        # Calls establish legal update codes and ordering, not the live identity
        # or palette for a match. Controlled Heart runs used the initial scene.
        seen=set()
        for row in data['records']:
            for request in row['requests']:
                key=(request['code'],request['used']<64)
                if key not in seen:seen.add(key);observations.append({'source':name,'record':row['id'],'code':key[0],'acceptedAtEntry':key[1]})
    for name in ('enemies-aliens-boss-index.json','levels-chr-verification.json'):
        path=ROOT/'docs/research/nes-contra/evidence'/name;sources.append({'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())})
    # 0x8x requests retain the visible map attributes. These cells come from
    # the original Guardian and Heart map definitions, independently of recipes.
    retained=[]
    for mapIndex,positions in [(2,[(4,2),(5,2),(5,3)]),(10,[(5,2),(6,2),(5,3),(6,3)])]:
        m=stage['maps']['entries'][mapIndex]['supertiles']
        for x,y in positions:
            code=m[y*8+x];retained.append({'sourceMap':mapIndex,'cell':[x,y],'baseCode':code,'attribute':blocks[code]['attribute']})
    for alternate,title in [(False,'guardian'),(True,'heart')]:
        context=next(c for c in contexts if c['alternate']==alternate and c['sourcePhase']==3)
        image=Image.new('RGB',(12*96,5*100),(230,230,230));draw=ImageDraw.Draw(image)
        for code in range(51):
            block=blocks[base+code];attr=0xff if alternate and 4<=code<=15 else block['attribute']
            tile=render(block,context['mapping'],context['paletteIndices'],attr);x=(code%12)*96;y=(code//12)*100
            image.paste(tile.resize((64,64),Image.Resampling.NEAREST),(x+16,y));draw.text((x+6,y+66),f'U{code:02X}/A{attr:02X}',fill=(0,0,0))
        image.save(OUT/(title+'-updates.png'))
    # Complete fixed artwork panels retain their original scenery; updates
    # change patterns only. They are source-art reviews, not screenshots of play.
    def scene(mapIndex,context,name):
        alternate=context['alternate'];variant={'pulse0':0,'pulse1':1,'destroyed':2}.get(name,0)
        ids=stage['maps']['entries'][mapIndex]['supertiles'];grid=[blocks[i]for i in ids];grid=grid.copy()
        # Guardian's retained right-jaw cell is (5,2). Applying the original
        # B5CD offsets and B4EE origin to it gives these writes through R9,
        # before wall clearing: R7 erases row1, R8 updates rows0/1, R9 row2/0.
        replacements=([(4,2,0x11),(5,2,0x10)]if name=='closed'else[(4,2,0x13),(5,2,0x12),(5,3,0x14)]if name=='open'else[(4,2,0x03),(5,2,0x03),(5,3,0x03),(4,1,0x03),(5,1,0x03),(6,0,0x15),(5,0,0x16),(7,1,0x17),(6,1,0x03),(6,2,0x03),(4,0,0x03)])if not alternate else [(6,2,4+variant*4),(5,2,5+variant*4),(6,3,6+variant*4),(5,3,7+variant*4)]
        for x,y,code in replacements:grid[y*8+x]=blocks[base+code]
        frame=Image.new('RGB',(256,224))
        for n,block in enumerate(grid):frame.paste(render(block,context['mapping'],context['paletteIndices'],blocks[ids[n]]['attribute']),((n%8)*32,(n//8)*32))
        return frame
    for mapIndex,alternate,variants in [(2,False,['closed','open','destroyed']),(10,True,['pulse0','pulse1','destroyed'])]:
        context=next(c for c in contexts if c['alternate']==alternate and c['sourcePhase']==3)
        sheet=Image.new('RGB',(3*512,448+24),(230,230,230));draw=ImageDraw.Draw(sheet)
        for variant,name in enumerate(variants):
            frame=scene(mapIndex,context,name)
            sheet.paste(frame.resize((512,448),Image.Resampling.NEAREST),(variant*512,24));draw.text((variant*512+8,4),name,fill=(0,0,0))
        sheet.save(OUT/(('heart'if alternate else'guardian')+'-scene.png'))
    rasters=[];scenes=[]
    for i,context in enumerate(contexts):
        for code in range(51):
            attribute=255 if context['alternate']and 4<=code<=15 else blocks[base+code]['attribute']
            rgba=render(blocks[base+code],context['mapping'],context['paletteIndices'],attribute).convert('RGBA').tobytes()
            rasters.append({'context':i,'code':code,'retainedAttribute':attribute,'rgba':rgba.hex(),'sha256':sha(rgba)})
        selected=(not context['alternate']and context['paletteIndices'][3]==76)or(context['alternate']and context['sourcePhase']==3)
        if selected:
            for name in (['pulse0','pulse1','destroyed']if context['alternate']else['closed','open','destroyed']):
                rgba=scene(10 if context['alternate']else 2,context,name).convert('RGBA').tobytes()
                scenes.append({'context':i,'appearance':name,'rgba':rgba.hex(),'sha256':sha(rgba)})
    reaIndex=json.loads((ROOT/'docs/research/nes-contra/evidence/enemies-aliens-boss-index.json').read_text())
    rea=[{'address':f['address'],'evidenceID':f['evidenceID'],'session':f['session']}for f in reaIndex['functions']if f['address']in['0xb4a1','0xb50a','0xb522','0xb5ab','0xb6d7','0xbcd9','0xbd71']]
    chrSources=[{'page':n,'romOffset':chrBase+n*1024,'bytes':1024,'sha256':sha(rom[chrBase+n*1024:chrBase+(n+1)*1024])}for n in sorted(pages)]
    result={'version':1,'romSHA256':ROM_SHA,'sources':sources,'rea':rea,'chrSources':chrSources,'tileSet':tiles['id'],'updateBase':base,'patterns':patterns,'contexts':contexts,'retainedAttributes':retained,'requestObservations':observations,'rasters':rasters,'scenes':scenes,
            'limitations':['All contexts are global immutable art knowledge, never selected from a live hidden level.','Controlled requests establish drawing semantics; original Heart render context comes from the alternate header/map, not injected-Heart initial-scene captures.','An intact pattern can persist during defeat before graphic replacement; a single image does not prove remaining HP or current vulnerability.']}
    raw=json.dumps(result,sort_keys=True,separators=(',',':')).encode();fixture=ROOT/'games/contra/engine/testdata/bot-alien-original-art.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0))
    anchors=Image.new('RGB',(1280,344),(230,230,230));draw=ImageDraw.Draw(anchors)
    reviews=[(12,'open',(112,32,240,160),(168,88),'Guardian opening'),(22,'pulse0',(144,48,240,144),(184,104),'Heart palette 75'),(23,'pulse1',(144,48,240,144),(184,104),'Heart palette 80'),(22,'destroyed',(144,48,240,144),None,'Destroyed: excluded')]
    for i,(context,appearance,box,anchor,label)in enumerate(reviews):
        original=next(s for s in scenes if s['context']==context and s['appearance']==appearance)
        image=Image.frombytes('RGBA',(256,224),bytes.fromhex(original['rgba'])).convert('RGB')
        anchors.paste(image.crop(box).resize((288,288),Image.Resampling.NEAREST),(i*320+16,28));draw.text((i*320+8,5),label,fill=(0,0,0))
        if anchor:
            x=i*320+16+round((anchor[0]-box[0])*288/(box[2]-box[0]));y=28+round((anchor[1]-box[1])*288/(box[3]-box[1]))
            draw.line((x-6,y,x+6,y),fill=(255,255,0),width=2);draw.line((x,y-6,x,y+6),fill=(255,255,0),width=2);draw.text((i*320+8,320),str(anchor),fill=(0,0,0))
    anchors.save(OUT/'anchors.png')
    (OUT/'reference-summary.json').write_text(json.dumps({'patterns':len(patterns),'contexts':len(contexts),'rasters':len(rasters),'scenes':len(scenes),'fixtureSHA256':sha(fixture.read_bytes()),'retainedAttributes':retained},indent=2)+'\n')
    print((OUT/'reference-summary.json').read_text())

if __name__=='__main__':main()
