"""Source-bound map data extraction, after REA loading-path inspection.

No execution/physics is reimplemented here. Original-loader captures separately
verify the decoded map entries and transition resource selections.
"""
from pathlib import Path
import hashlib
import zipfile

ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
ROM_PATH=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')


def digest(data):return hashlib.sha256(data).hexdigest()


def original():
    before=digest(ROM_PATH.read_bytes())
    with zipfile.ZipFile(ROM_PATH)as z:rom=z.read('Contra (J).nes')
    assert digest(rom)==ROM_SHA
    assert before==digest(ROM_PATH.read_bytes())
    return rom,before


def build():
    rom,archive_sha=original();prg=rom[16:16+131072]
    def word(offset):return int.from_bytes(prg[offset:offset+2],'little')
    def source(cpu,length,banks):
        cpu_base=0xc000 if banks[0]==14 else 0x8000
        offset=cpu-cpu_base+banks[0]*8192
        assert 0<=offset<len(prg) and offset+length<=len(prg)
        return {'cpu':cpu,'banks':banks,'prgOffset':offset,'romOffset':16+offset,
                'length':length,'sha256':digest(prg[offset:offset+length])}
    headers=[list(prg[0xa87b+i*32:0xa89b+i*32])for i in range(8)]
    map_tables=sorted({h[2]+h[3]*256 for h in headers}|{0x90e3})
    # Base 2 and Base 4 are adjacent tables sharing a data area beginning at
    # $82E6. A table's own first/minimum pointer cannot establish its boundary.
    boundaries=sorted(set(map_tables+[word(a)for a in map_tables]+[0xc000]))
    def map_entries(table,rows):
        end=min(a for a in boundaries if a>table);cursor=table;pointers=[]
        while cursor<end:
            pointer=word(cursor);assert 0x8000<=pointer<0xc000
            pointers.append(pointer);end=min(end,pointer);cursor+=2
        assert cursor==end and end>table and len(pointers)<=128
        entries=[];seen={}
        for index,pointer in enumerate(pointers):
            at=pointer;values=[];commands=[]
            while len(values)<rows*8:
                assert at<0xc000
                start=at;code=prg[at];at+=1
                if code<0x80:
                    values.append(code);commands.append({'offset':start-pointer,'literal':code})
                elif code<0xf0:
                    n=code&127;assert n and len(values)+n<=rows*8
                    value=prg[at];at+=1;values.extend([value]*n)
                    commands.append({'offset':start-pointer,'repeat':n,'value':value})
                else:
                    row=(code&15)*8;assert row+8<=len(values) and len(values)+8<=rows*8
                    values.extend(values[row:row+8]);commands.append({'offset':start-pointer,'copyRow':code&15})
            assert len(values)==rows*8 and at-pointer<256
            entries.append({'index':index,'aliasOf':seen.get(pointer),'pointer':pointer,
                            'encoded':prg[pointer:at].hex(),'supertiles':values,
                            'source':source(pointer,at-pointer,[4,5]),'commands':commands})
            seen.setdefault(pointer,index)
        return {'pointerTable':source(table,end-table,[4,5]),'mapRows':rows,
                'entryCount':len(entries),'distinctPointers':len(seen),'entries':entries}
    sets={}
    def tile_set(tiles,attrs,update_index):
        key=f'{tiles:04x}-{attrs:04x}'
        count,rem=divmod(attrs-tiles,16);assert rem==0 and 0<count<=128
        update_tiles=word(0x1c56e+update_index*4);update_attrs=word(0x1c570+update_index*4)
        base,remainder=divmod(update_tiles-tiles,16)
        assert remainder==0 and update_attrs-attrs==base and 0<=base<count
        data=[{'id':n,'tiles':list(prg[0x4000+tiles+n*16:0x4000+tiles+(n+1)*16]),
               'attribute':prg[0x4000+attrs+n]}for n in range(count)]
        value={'id':key,'supertiles':data,'tilesSource':source(tiles,count*16,[6,7]),
               'attributesSource':source(attrs,count,[6,7]),
               'updates':{'baseSupertile':base,'count':count-base,
                          'pointerTableEntry':source(0xc56e+update_index*4,4,[14,15]),
                          'tilesSource':source(update_tiles,(count-base)*16,[6,7]),
                          'attributesSource':source(update_attrs,count-base,[6,7])}}
        if key in sets:
            # Base 2 and 4 point to the same definitions through separate rows.
            assert sets[key]['supertiles']==data
        else:sets[key]=value
        return key
    def palette(raw):
        return {'collisionLimits':raw[:3],'cycleIndices':raw[3:7],'paletteIndices':raw[7:15]}
    boss_pointers=[word(0x1dd45+i*2)for i in range(3)]
    assert boss_pointers==[0x90e3,0xb68a,0xbe8a]
    boss_map=map_entries(boss_pointers[0],7)
    boss_tiles=tile_set(boss_pointers[1],boss_pointers[2],8)
    presets={};stages=[]
    for stage,h in enumerate(headers):
        maps=map_entries(h[2]+h[3]*256,8 if h[1] else 7)
        key=tile_set(h[4]+h[5]*256,h[6]+h[7]*256,stage)
        assert all(max(m['supertiles'])<len(sets[key]['supertiles'])for m in maps['entries'])
        alternate_chr=prg[0x1c78b+stage]
        indices={stage}
        if alternate_chr:indices.add(alternate_chr)
        if h[0]==1:indices.add(8+(stage>>1))
        for i in indices:
            pages=list(prg[0x1c793+i*8:0x1c79b+i*8]);assert max(pages)<128
            presets[i]={'id':i,'pages':pages,'source':source(0xc793+i*8,8,[14,15])}
        alternate=list(prg[0x1d05a+stage*15:0x1d069+stage*15])
        item={'stage':stage+1,'sceneType':h[0],'scrollType':h[1],
              'header':{'bytes':h,'source':source(0xa87b+stage*32,32,[4,5])},
              'maps':maps,'tileSetId':key,'initialPaletteState':palette(h[9:24]),'chrPreset':stage,
              'scrollSpanPixels':240 if h[1] else 256,
              'initialCollisionCheck':h[25],'stopScrollScreen':h[24],
              'alternate':{'triggerScreen':h[8],'mode':'base-boss'if h[0]==1 else 'scroll-boundary',
                           'chrPreset':alternate_chr if alternate_chr else None,
                           **palette(alternate),'source':source(0xd05a+stage*15,15,[14,15])}}
        if h[0]==1:
            item['perspectivePhases']=4
            item['baseBoss']={'sceneType':128,'mapIndex':stage>>1,'mapSetId':'base-boss','tileSetId':boss_tiles,
                              'chrPreset':8+(stage>>1),'pointerSwitchSource':source(0xdd45,6,[14,15]),
                              'trigger':{'screen':h[8],'perspectivePhase':3},'enterAtScreen':h[24]}
        stages.append(item)
    assert [s['maps']['entryCount']for s in stages]==[14,24,10,32,22,14,16,12]
    assert boss_map['entryCount']==2
    color_ids=sorted({n for s in stages for p in [s['initialPaletteState'],s['alternate']]
                      for field in ['cycleIndices','paletteIndices']for n in p[field]})
    color_base=word(0x1d0e1)
    colors=[{'id':n,'colors':list(prg[0x10000+color_base+n*3:0x10000+color_base+n*3+3]),
             'source':source(color_base+n*3,3,[14,15])}for n in color_ids]
    return {'schema':1,'romSHA256':ROM_SHA,'sourceArchiveSHA256':archive_sha,
            'scope':'ROM-derived eight-stage data with separately declared controlled-original loader verification.',
            'stages':stages,'tileSets':list(sets.values()),'extraMapSets':{'base-boss':boss_map},
            'chrPresets':[presets[i]for i in sorted(presets)],'paletteColors':colors,
            'chrAnimationDispatch':{'source':source(0xc848,16,[14,15]),
                                    'targets':[word(0x1c848+i*2)for i in range(8)],
                                    'interpretation':'inline CHR animation dispatch data; never translated as instructions'},
            'limitations':['Map entries include aliases and base perspective phases, not a count of playable rooms.',
                           'Vertical map data has eight 32-pixel rows while nametable scrolling wraps at 240 pixels.',
                           'CHR presets are defaults/transition assignments; per-frame original CHR animation may replace a page.',
                           'Dynamic terrain and enemy events must use the separate update-code tables.']}
