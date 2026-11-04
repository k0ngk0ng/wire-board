#!/usr/bin/env python3
"""Complete original 0x126c8 initialization and bounded source specifications."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, ROM_SHA256, buttons_at, sha
from extract_animation_catalog import initializer_bindings, be
from capture_ground_idle import actor

WRITES = set(range(0x1c,0x26)) | {0x27} | set(range(0x28,0x3e)) | set(range(0x50,0x54)) | {0x59,0x5a,0x5b,0x5c,0x5d,0x5f,0x6c,0x6e} | set(range(0x78,0x7c)) | set(range(0x7e,0x82)) | set(range(0x8c,0x94)) | {0xb8,0xb9,0xbc,0xbd,0xc4,0xc5}


def state(obj):
    return {'body':actor(obj),'character_id':obj[0x58],
            'graphics_set':be(obj,0x1c,4),'tiles_base':be(obj,0x20,4),
            'animation_set':be(obj,0x50,4),'shape_set':be(obj,0x8c,4),'movement_source':be(obj,0x90,4),
            'skip_graphics':obj[0x27],'graphic_offset_x':be(obj,0x28,2,True),'graphic_offset_y':be(obj,0x2a,2,True),
            'graphic_cache':[be(obj,0x2c+i*4,4)for i in range(4)],'graphic_parameter':be(obj,0x3c,2),
            'displayed_health':be(obj,0x7a,2,True),'energy':be(obj,0xb8,4,True),'displayed_energy':be(obj,0xbc,2,True),
            'attack_scale':be(obj,0xc4,2)}


def specifications(rom):
    specs=[];sources=[]
    for binding in initializer_bindings(rom):
        cid=binding['character_id'];entry=binding['initializer'];p=binding['movement_parameters']
        for delta,field in [(0x36,0x7e),(0x3c,0x80),(0x42,0xc4)]:
            assert rom[entry+delta:entry+delta+2]==bytes.fromhex('3b7c')
            assert be(rom,entry+delta+4,2)==field
        stun=be(rom,entry+0x38,2,True);assert stun==be(rom,entry+0x3e,2,True)
        specs.append({'character_id':cid,'graphics_set':binding['graphics_set'],'tiles_base':binding['tiles_base'],
                      'animation_set':binding['animation_set'],'shape_set':binding['shape_set'],'movement_source':p,
                      'base_stun':stun,'attack_scale':be(rom,entry+0x44,2),
                      'movement':{'walk_speed':be(rom,p,4,True),'vertical_velocity':be(rom,p+4,4,True),
                                  'gravity':be(rom,p+8,4,True),'horizontal_velocity':be(rom,p+12,4,True),
                                  'burst_speed':be(rom,p+24,4,True),'energy_drain':be(rom,p+28,2,True)}})
        sources.append({'character_id':cid,'initializer':entry,'initializer_sha256':sha(rom[entry:entry+0x4a]),
                        'movement_address':p,'movement_bytes':rom[p:p+30].hex(),'movement_sha256':sha(rom[p:p+30]),
                        'unassigned_parameter_span':{'offset':16,'length':8,'bytes':rom[p+16:p+24].hex()}})
    return {'schema':1,'rom_sha256':ROM_SHA256,'specs':specs,'sources':sources}


def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();assert sha(rom)==ROM_SHA256
    data=specifications(rom)
    spec_raw=(json.dumps(data,indent=2)+'\n').encode();(out/'fighter-specs.json').write_bytes(spec_raw)
    exits={0x12748+i*0x4a+0x48 for i in range(11)}
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint);bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=sorted({0x126c8}|exits);pcs=(C.c_uint*len(points))(*points)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural=[];isolated=[];pending={};errors=[];private=False;label=''
    with ReferenceSession(ROOT,out,core=core)as ref:
        @cbtype
        def observe(kind,width,pc,value):
            try:
                address=ref.get_register(13)&0xffffff;sp=ref.get_register(15);key=(address,sp);ram=ref.ram_cpu();offset=address&0xffff
                if pc==0x126c8:
                    assert not pending,'Overlapping initialization'
                    obj=ram[offset:offset+256];assert obj[0x58]<11
                    pending[key]={'frame':ref.frame,'object_id':offset,'before':state(obj),'actor_before':obj.hex(),
                                  'attribute_mask':be(ram,0xa03e,2)}
                elif key in pending:
                    row=pending.pop(key);obj=ram[offset:offset+256]
                    assert pc==0x12748+row['before']['character_id']*0x4a+0x48
                    old=bytes.fromhex(row['actor_before']);unexpected=[i for i,(a,b)in enumerate(zip(old,obj))if a!=b and i not in WRITES];assert not unexpected,unexpected
                    row.update(after=state(obj),actor_after=obj.hex(),exit=hex(pc),exit_frame=ref.frame)
                    if private:row['name']=label;isolated.append(row);ref.cpu.cycles=2000000
                    else:natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private:ref.cpu.cycles=2000000
        assert bridge.yyh_set_observer(observe,pcs,len(points))
        hook=ref.api('set_cpu_hook',None,C.c_void_p);observer=C.cast(bridge.yyh_cpu_hook,C.c_void_p);hook(observer);frames=[]
        for f in range(1,3601):
            frames.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:checkpoint=ref.save()
        hook(None);assert frames==expected and not pending and not errors,(errors,list(pending))
        private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        def write_bytes(address,raw):
            for i,b in enumerate(raw):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        for cid,seed,mask,attributes in itertools.product(range(11),range(16),[0,1,0x8000,0x7fff,0xffff,0xa5a5],[0,0xffff,0x1800,0xe3ff]):
            label=f'character-{cid}/dirty-{seed}/mask-{mask:x}/attributes-{attributes:x}'
            ref.restore(checkpoint,restore_refresh_cycles=True);setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400)
            for reg in range(8):setreg(reg,0xa5a50000|(reg*0x111))
            write_bytes(0xff8400,bytes((i*73+seed*47+cid*13+17)&255 for i in range(256)))
            write(0xff8458,cid);write(0xff8424,attributes,2);write(0xffa03e,mask,2)
            write(0xff8460,[0,1,127,255][seed%4]);write(0xff846d,[0,1,0x20,255][seed%4]);write(0xff8477,0xa5)
            write(0xff84ba,[0,1,0x7fff,0x8000,0xffff][seed%5],2);write(0xff846f,[0,1,127,255][seed%4])
            before=ref.ram_cpu();count=len(isolated);setreg(16,0x126c8);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            hook(observer);execute(1000000);hook(None)
            assert len(isolated)==count+1 and not pending and not errors,(label,errors,list(pending))
            after=ref.ram_cpu();allowed=set(range(0x8400,0x8500))|set(range(0xffc0,0xffe4));unexpected=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed];assert not unexpected,(label,unexpected)
        identity=ref.identity
    for name,rows,scope in [('fighter-initialization-oracle',natural,'Natural complete original initializer calls'),('fighter-initialization-boundaries',isolated,'Declared dirty-input original CPU calls')]:
        raw=(json.dumps(rows,separators=(',',':'))+'\n').encode();zipped=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(zipped)
        report={'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'spec_sha256':sha(spec_raw),
                'records':len(rows),'character_counts':dict(Counter(str(r['before']['character_id'])for r in rows)),
                'fixture_sha256':sha(raw),'gzip_sha256':sha(zipped),'frames_match_original_core':frames==expected,
                'incomplete_calls':0,'errors':errors,'pass':bool(rows)and not errors,
                'limits':['Initializer only, not spawn/menu/ready/world update','Untouched dirty fields and energy low word deliberately remain','Only movement offsets0/4/8/c/18/1c are named;10/14 are retained as unassigned source bytes']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
