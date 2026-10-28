#!/usr/bin/env python3
"""Recover Japanese stage-one terrain from ROM bytes and compare real captures.

Verification boundaries established before extraction: wrong region/banks,
confusing tile and screen pointer windows, RLE count/copy overruns, treating the
lookahead alias as another screen, wrong two-bit collision packing, discarding
partial streaming state, or claiming an unobserved solid collision code.
"""
import gzip
import argparse
import hashlib
import json
from pathlib import Path
import zipfile

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'docs/research/nes-contra/evidence'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def rows(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


def decode_screen(bank, pointer):
    at=pointer-0x8000;start=at;values=[];commands=[]
    assert 0<=at<len(bank)
    while len(values)<56:
        offset=at;code=bank[at];at+=1
        if code<0x80:
            values.append(code);commands.append({'cpu':hex(0x8000+offset),'op':'literal','value':code})
        elif code<0xf0:
            count=code&0x7f
            assert count>0 and len(values)+count<=56,'invalid stage-one RLE count'
            value=bank[at];at+=1;values.extend([value]*count)
            commands.append({'cpu':hex(0x8000+offset),'op':'repeat','count':count,'value':value})
        else:
            row=(code&15)*8
            assert len(values)+8<=56
            for index in range(8):
                assert row+index<len(values),'row-copy reads uninitialized output'
                values.append(values[row+index])
            commands.append({'cpu':hex(0x8000+offset),'op':'copy-row','row':code&15})
    assert len(values)==56
    return values,{'cpu_start':hex(pointer),'cpu_end_exclusive':hex(0x8000+at),
                   'prg_offset':4*8192+start,'rom_offset':16+4*8192+start,
                   'bytes':at-start,'sha256':sha(bank[start:at]),'commands':commands}


def classify(tile, limits):
    if tile==0:return 0
    if tile<limits[0]:return 1
    if tile<limits[1]:return 0
    if tile<limits[2]:return 2
    return 3


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--primary',default='terrain-walk-jump-01')
    parser.add_argument('--repeat',default='terrain-walk-jump-02')
    args=parser.parse_args()
    import re
    assert all(re.fullmatch(r'[a-z0-9][a-z0-9-]{0,64}',name) for name in [args.primary,args.repeat])
    before=sha(ROM.read_bytes())
    with zipfile.ZipFile(ROM) as archive:
        assert archive.testzip() is None
        raw=archive.read('Contra (J).nes')
    assert sha(raw)==ROM_SHA
    prg=raw[16:16+131072]
    maps=prg[4*8192:6*8192];graphics=prg[6*8192:8*8192]
    header=maps[0x287b:0x289b]
    assert header.hex()=='000001800180c1860b06fbff0508050802030405000122070b00000000000000'
    screen_table=int.from_bytes(header[2:4],'little')
    tile_table=int.from_bytes(header[4:6],'little')
    palette_table=int.from_bytes(header[6:8],'little')
    limits=list(header[9:12]);first=int.from_bytes(maps[screen_table-0x8000:screen_table-0x8000+2],'little')
    count=(first-screen_table)//2
    assert count==14 and (first-screen_table)%2==0
    pointers=[int.from_bytes(maps[screen_table-0x8000+2*i:screen_table-0x8000+2*i+2],'little') for i in range(count)]
    assert len(set(pointers))==13 and pointers[13]==pointers[0]
    tile_count=(palette_table-tile_table)//16
    assert tile_count==108
    supertiles=[]
    for i in range(tile_count):
        offset=tile_table-0x8000+i*16;data=list(graphics[offset:offset+16])
        supertiles.append({'id':i,'tiles':data,'attribute':graphics[palette_table-0x8000+i],
                           'collision_2bit':[classify(data[y*4+x],limits) for y in (0,2) for x in (0,2)]})
    screens=[]
    for number,pointer in enumerate(pointers):
        indices,source=decode_screen(maps,pointer)
        assert max(indices)<tile_count
        expanded=[0]*896;packed=[0]*64;cells=[0]*224;attributes=[]
        for row in range(7):
            for col in range(8):
                tile=supertiles[indices[row*8+col]];attributes.append(tile['attribute'])
                for y in range(4):expanded[(row*4+y)*32+col*4:(row*4+y)*32+col*4+4]=tile['tiles'][y*4:y*4+4]
                for y in range(2):
                    for x in range(2):
                        code=tile['collision_2bit'][y*2+x];cy=row*2+y;cx=col*2+x
                        packed[cy*4+cx//4]|=code<<(6-2*(cx%4));cells[cy*16+cx]=[0,1,2,128][code]
        screens.append({'index':number,'alias_of':0 if number==13 else None,'source':source,
                        'supertiles':indices,'tiles':expanded,'attributes':attributes,
                        'collision_packed_hex':bytes(packed).hex(),'collision_codes':cells})
    entry=ROOT/'.local/contra/runs/coop-play-entry'
    entry_source=json.loads((entry/'source.json').read_text());assert entry_source['rom_sha256']==ROM_SHA
    ram_all=(entry/'ram.bin').read_bytes();ram=ram_all[199*2048:200*2048]
    ppu=(entry/'frame-00200-ppu.bin').read_bytes()
    assert bytes(screens[0]['supertiles'])==ram[0x600:0x638]
    assert bytes(screens[1]['supertiles'])==ram[0x640:0x678]
    assert bytes(screens[0]['tiles'])==ppu[0x2000:0x2380]
    assert bytes(screens[0]['attributes'])==ppu[0x23c0:0x23f8]
    assert bytes.fromhex(screens[0]['collision_packed_hex'])==ram[0x680:0x6c0]
    # Screen 1 is genuinely streaming. Verify its already written left 16
    # columns, while retaining remaining original bytes without fabricating them.
    assert all(bytes(screens[1]['tiles'][r*32:r*32+16])==ppu[0x2400+r*32:0x2400+r*32+16] for r in range(28))
    fixture={'rom_sha256':ROM_SHA,'source_is_original_execution':True,
             'entry':{'source':entry_source,'scenario':json.loads((entry/'scenario.json').read_text()),
                      'frame':200,'ram_sha256':sha(ram),'ppu_sha256':sha(ppu),
                      'level_header_hex':ram[0x40:0x60].hex(),
                      'supertiles_hex':ram[0x600:0x680].hex(),'collision_hex':ram[0x680:0x700].hex(),
                      'nametables_hex':ppu[0x2000:0x2800].hex()},'queries':[],'collision_maps':[],'streaming':[]}
    primary=ROOT/'.local/contra-static/oracle/runs'/args.primary
    repeat=ROOT/'.local/contra-static/oracle/runs'/args.repeat
    for name in ['ram.bin','collision.jsonl','collision-maps.jsonl','streaming.jsonl']:
        assert (primary/name).read_bytes()==(repeat/name).read_bytes(),f'non-repeatable oracle: {name}'
    fixture['source']=json.loads((primary/'source.json').read_text())
    fixture['scenario']=json.loads((primary/'scenario.json').read_text())
    fixture['queries']=rows(primary/'collision.jsonl')
    fixture['collision_maps']=rows(primary/'collision-maps.jsonl')
    fixture['streaming']=rows(primary/'streaming.jsonl')
    for stream in fixture['streaming']:
        assert stream['vertical']==0 and bytes(screens[stream['screen']]['supertiles']).hex()==stream['indices']
    table={m['id']:bytes.fromhex(m['bytes']) for m in fixture['collision_maps']}
    codes=set()
    for q in fixture['queries']:
        vertical=q['y']+q['scrollY'];v=vertical&255
        if vertical>255 or v>=240:v=(v+16)&255
        horizontal=q['x']+q['scrollX'];page=(q['ppuControl']&1)^(horizontal>>8)
        index=((v>>2)&60)|((horizontal&255)>>6)|(page*64);column=(horizontal>>4)&3
        value=table[q['map']][index]
        code=0 if q['y']>=224 else [0,1,2,128][(value>>(6-column*2))&3]
        assert (index,column,value,code,code&1,code&128)==tuple(q[k] for k in ['sourceIndex','column','sourceByte','code','carry','negative'])
        codes.add(code)
    assert fixture['queries'] and fixture['streaming']
    assert sha(ROM.read_bytes())==before
    OUT.mkdir(parents=True,exist_ok=True)
    result={'schema':1,'rom_sha256':ROM_SHA,'source_archive_sha256':before,
            'scope':'Japanese first stage only; further stages are not synthesized.',
            'header':{'prg_offset':0xa87b,'rom_offset':16+0xa87b,'cpu':'0xa87b','bank_8k':5,'bytes':list(header)},
            'map_bank_8k':[4,5],'tile_bank_8k':[6,7],'screen_table':hex(screen_table),
            'tile_table':hex(tile_table),'palette_table':hex(palette_table),'collision_limits':limits,
            'distinct_screens':13,'lookahead_alias_index':13,'screen_pixels':[256,224],
            'supertile_pixels':[32,32],'collision_cell_pixels':[16,16],
            'header_stop_scroll_screen':header[24],'header_alternate_graphics_screen':header[8],
            'supertiles':supertiles,'screens':screens,
            'limitations':['Screens 0 and 1 indices, screen 0 full graphics/collision, and screen 2 streaming were observed dynamically.',
                           'Other first-stage screens are ROM-derived and still need gameplay coverage.',
                           'Destructible and script-driven terrain changes are not recovered by this initial map export.',
                           'Other seven stages require separate header/bank/encoding verification.']}
    raw=json.dumps(result,ensure_ascii=False,indent=2).encode()+b'\n'
    (OUT/'static-level1-terrain.json').write_bytes(raw)
    fixture_bytes=json.dumps(fixture,separators=(',',':')).encode()+b'\n'
    packed=gzip.compress(fixture_bytes,mtime=0)
    (OUT/'static-terrain-oracle.json.gz').write_bytes(packed)
    verification={'passed':True,'rom_source_unchanged':True,'screen_pointer_entries':len(screens),'distinct_screens':13,
                  'supertiles':len(supertiles),'oracle_queries':len(fixture['queries']),'oracle_maps':len(table),
                  'oracle_codes':sorted(codes),'observed_stream_decodes':len(fixture['streaming']),
                  'repeat_byte_identical':['ram.bin','collision.jsonl','collision-maps.jsonl','streaming.jsonl'],
                  'first_screen':{'supertile_ids':56,'pattern_tiles':896,'attributes':56,'collision_bytes':64,'all_match':True},
                  'second_screen':{'supertile_ids_match':56,'left_tile_columns_match':16,'remaining_columns':'partial original streaming; preserved as observed'},
                  'terrain_sha256':sha(raw),'oracle_sha256':sha(packed),'oracle_uncompressed_sha256':sha(fixture_bytes)}
    (OUT/'static-terrain-verification.json').write_text(json.dumps(verification,indent=2)+'\n')
    print(json.dumps(verification))


if __name__=='__main__':main()
