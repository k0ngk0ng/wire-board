#!/usr/bin/env python3
"""Bounded MD animation/shape catalog with independent original CPU checks.

Semantic records only; no emulator, ROM program, images or audio are embedded.
The original-CPU checks are explicitly isolated calls, not complete matches.
"""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha, ROM_SHA256
from capture_animation_records import selected_data, snapshot, RETURNS


def be(data, offset, size, signed=False):
    assert 0 <= offset and offset + size <= len(data)
    return int.from_bytes(data[offset:offset+size], 'big', signed=signed)


def initializer_bindings(rom):
    result=[]
    for character in range(11):
        entry=0x12732+character*2
        target=entry+be(rom,entry,2,True)
        assert target==0x12748+character*0x4a
        values={}
        for name, offset, field in [('graphics_set',0,0x1c),('tiles_base',8,0x20),('animation_set',16,0x50),('shape_set',24,0x8c),('movement_parameters',32,0x90)]:
            assert rom[target+offset:target+offset+2]==bytes.fromhex('2b7c')
            assert be(rom,target+offset+6,2)==field
            values[name]=be(rom,target+offset+2,4)
        assert rom[target+0x48:target+0x4a]==bytes.fromhex('4e75')
        result.append({'character_id':character,'name':None,'initializer':target,**values})
    assert 0x12732+11*2==result[0]['initializer']
    return result


def extract_records(rom, characters):
    records=[];coverage=[]
    for ch in characters:
        anim,shape=ch['animation_set'],ch['shape_set']
        first=be(rom,anim,2,True)
        assert first==0x180, 'Unknown animation table layout'
        table=[be(rom,anim+i*2,2,True)for i in range(first//2)]
        assert table[-1]==0 and all(x>0 for x in table[:-1]), 'Unexpected zero/table entries'
        offsets=table[:-1]
        assert offsets==sorted(set(offsets)) and offsets[0]==first
        assert all(x%8==0 and anim+x<shape for x in offsets)
        shape_table_bytes=be(rom,shape+2,2,True)
        assert shape_table_bytes>0 and shape_table_bytes%4==0
        directory_offsets=[be(rom,shape+i+2,2,True)for i in range(0,shape_table_bytes,4)]
        assert min(directory_offsets)==shape_table_bytes
        assert all(x>=shape_table_bytes for x in directory_offsets)
        payload_starts=sorted(set(shape+x for x in directory_offsets))
        shape_limit=characters[ch['character_id']+1]['graphics_set'] if ch['character_id']<10 else len(rom)
        assert shape<shape_limit
        intervals=[];count=0
        for pose,start_offset in enumerate(offsets):
            end=anim+offsets[pose+1] if pose+1<len(offsets) else shape
            start=anim+start_offset
            assert (end-start)%8==0 and start<end
            obj=bytearray(256);obj[0x50:0x54]=anim.to_bytes(4,'big');obj[0x8c:0x90]=shape.to_bytes(4,'big');obj[0x5f]=pose
            terminal=None
            for index in range(min((end-start)//8,256)):
                obj[0x59]=index
                address=start+index*8
                record=rom[address:address+8]
                assert record[1]<len(directory_offsets), 'Frame refers outside verified shape directory'
                key,data,source=selected_data(rom,b'',obj)
                assert int(source['animation_address'],16)==address
                payload=int(source['shape_address'],16)
                consumed=len(bytes.fromhex(source['shape_bytes']))
                next_payload=next((p for p in payload_starts if p>payload),shape_limit)
                assert shape+shape_table_bytes<=payload<=next_payload and payload+consumed<=next_payload
                records.append({'key':key,'data':data,'source':source,'character_id':ch['character_id']})
                count+=1
                if record[2]&0x80:
                    terminal=index
                    break
            assert terminal is not None, f'No bounded terminal marker: character {ch["character_id"]} pose {pose}'
            tail=rom[start+(terminal+1)*8:end]
            assert not any(tail), 'Nonzero data after first terminal marker needs separate analysis'
            intervals.append({'pose':pose,'start':start,'end_exclusive':start+(terminal+1)*8,'frames':terminal+1,'terminal_flags':rom[start+terminal*8+2],
                              'padding_bytes':len(tail),'source_sha256':sha(rom[start:start+(terminal+1)*8])})
        coverage.append({'character_id':ch['character_id'],'animation_set':anim,'shape_set':shape,
                         'table_start':anim,'table_bytes':first,'nonzero_pose_entries':len(offsets),
                         'excluded_zero_entry':{'pose':191,'offset':0,'reason':'Points into the table, not a confirmed animation entry'},
                         'shape_directory_entries':len(directory_offsets),'shape_payload_limit':shape_limit,
                         'shape_limit_is_only_rom_end':ch['character_id']==10,'frames':count,'poses':intervals,
                         'runtime_reachability':'Not all table entries are proven reachable by gameplay'})
    assert len({tuple(r['key'].values())for r in records})==len(records)
    return records,coverage


def main():
    ap=argparse.ArgumentParser()
    ap.add_argument('--out',required=True,type=Path)
    ap.add_argument('--verify-limit',type=int,default=0,help='Exploratory frame cap; 0 verifies every catalog record')
    args=ap.parse_args();out=args.out.resolve()
    assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();assert sha(rom)==ROM_SHA256
    characters=initializer_bindings(rom);records,coverage=extract_records(rom,characters)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    watch=[0x3d8]+sorted(RETURNS);pcs=(C.c_uint*len(watch))(*watch)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    sample=None;stop_init=False;errors=[];initializations=[];initialized_actors={};cases=[]
    with ReferenceSession(ROOT,out,core=core) as ref:
        for frame in range(1,2401):ref.step(frame,buttons_at(events,frame))
        checkpoint=ref.save();setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(8*size))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal sample
            if (stop_init and pc==0x3d8)or(not stop_init and pc in RETURNS):
                if sample is not None:errors.append('More than one declared return')
                sample=(ref.ram_cpu(),pc);ref.cpu.cycles=200000
        assert bridge.yyh_set_observer(observe,pcs,len(watch))
        def prepare():
            ref.restore(checkpoint,restore_refresh_cycles=True);setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400)
            for i in range(8):setreg(i,0)
        def run(entry):
            nonlocal sample
            sample=None;setreg(16,entry);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert sample is not None and not errors,(hex(entry),errors)
            return sample
        stop_init=True
        for ch in characters:
            prepare();write(0xff8458,ch['character_id'])
            ram,pc=run(0x126c8);obj=ram[0x8400:0x8500]
            observed={name:be(obj,offset,4)for name,offset in [('animation_set',0x50),('shape_set',0x8c),('movement_parameters',0x90),('graphics_set',0x1c),('tiles_base',0x20)]}
            assert all(observed[k]==ch[k]for k in observed),(ch,observed)
            initialized_actors[ch['character_id']]=obj
            initializations.append({'character_id':ch['character_id'],'entry':'0x126c8','return':'0x3d8','observed':observed,'actor_sha256':sha(obj)})
        stop_init=False
        selected=records if args.verify_limit==0 else records[:args.verify_limit]
        for index,record in enumerate(selected):
            for variant in range(2):
                prepare();key=record['key']
                for offset,value in enumerate(initialized_actors[record['character_id']]):write(0xff8400+offset,value)
                write(0xff8450,key['animation_set'],4);write(0xff848c,key['shape_set'],4);write(0xff845f,key['pose']);write(0xff8459,key['index'])
                write(0xff845b,variant);write(0xff8461,0 if variant==0 else 3);write(0xff846d,4)
                write(0xff8440,0x7fff1234 if variant else 0x7f4321,4);write(0xff8444,-0x80000000 if variant else 0x1400000,4)
                write(0xff8424,0xa5cd,2);write(0xffa030,0)
                for i in range(4):write(0xffa004+i*2,[0,240,256][(index+variant+i)%3],2);write(0xfff80a+i,0 if variant==0 else 0x81+i)
                before=snapshot(ref.ram_cpu(),0xff8400)
                after_ram,pc=run(0x1eb40);after=snapshot(after_ram,0xff8400)
                cases.append({'key':key,'character_id':record['character_id'],'variant':variant,'before':before,'after':after,'exit':hex(pc)})
        identity=ref.identity
    data={'schema':1,'rom_sha256':ROM_SHA256,'characters':characters,'frames':records}
    raw=(json.dumps(data,separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0);(out/'animation-catalog.json.gz').write_bytes(compressed)
    oracle={'schema':1,'catalog_sha256':sha(raw),'scope':'Isolated original CPU selection/submission checks, not full matches','calls':cases}
    oracle_raw=(json.dumps(oracle,separators=(',',':'))+'\n').encode();oracle_zip=gzip.compress(oracle_raw,mtime=0);(out/'animation-catalog-oracle.json.gz').write_bytes(oracle_zip)
    report={'schema':1,'scope':'Bounded semantic animation/shape data; not all-character gameplay or graphics','identity':identity,'extractor_sha256':sha(Path(__file__).read_bytes()),
            'characters':len(characters),'poses_per_character':191,'frames':len(records),'frames_by_character':{str(c['character_id']):c['frames']for c in coverage},
            'initializer_checks':initializations,'coverage':coverage,'original_cpu_checked_frames':len(selected),'original_cpu_calls':len(cases),'all_catalog_frames_checked':len(selected)==len(records),
            'catalog_sha256':sha(raw),'catalog_gzip_sha256':sha(compressed),'oracle_sha256':sha(oracle_raw),'oracle_gzip_sha256':sha(oracle_zip),
            'limitations':['All table entries are not proven reachable as player actions','Character display names are not inferred','Pose 191 zero entries and trailing zero padding are excluded','Last shape section has only a ROM-end upper bound; referenced payloads terminate explicitly','No sprites, music or complete rules are provided','Go comparison is a separate required step'],
            'extraction_pass':True,'complete_cpu_coverage':len(selected)==len(records),'accepted_for_release':False}
    (out/'animation-catalog-manifest.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:report[k]for k in ['characters','poses_per_character','frames','frames_by_character','original_cpu_calls','all_catalog_frames_checked']},indent=2))


if __name__=='__main__':main()
