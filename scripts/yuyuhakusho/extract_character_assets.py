#!/usr/bin/env python3
"""Research-only extraction of original palette bindings and indexed tile assets."""
import argparse
import ctypes as C
import gzip
import json
import os
from pathlib import Path
import subprocess
from reference import ROOT, ReferenceSession, ROM_SHA256, buttons_at, sha
from extract_animation_catalog import initializer_bindings, be

TRACE_SOURCE = r'''#include <stdint.h>
typedef uint32_t(*reg_t)(unsigned);
typedef struct {uint32_t frame,pc,address,width,value,a[7],d[4],recent[16];} record_t;
static record_t records[131072];static unsigned count,dropped,last_pc,frame,head,recent[32];static reg_t reg;
void configure(reg_t r){reg=r;}void set_frame(unsigned f){frame=f;}unsigned get_count(void){return count;}unsigned get_dropped(void){return dropped;}
const record_t*get_records(void){return records;}
void observe(int kind,int width,unsigned address,unsigned value){
 if(kind==1){last_pc=address;recent[head++&31]=address;return;}
 if(kind!=4)return;address&=0xffffff;
 if(address>=0xffe180||address+(unsigned)width<=0xffe080)return;
 if(count==131072){dropped++;return;}record_t*r=&records[count++];r->frame=frame;r->pc=last_pc;r->address=address;r->width=width;r->value=value;
 for(unsigned i=0;i<7;i++)r->a[i]=reg(8+i);for(unsigned i=0;i<4;i++)r->d[i]=reg(i);for(unsigned i=0;i<16;i++)r->recent[i]=recent[(head-i-1)&31];
}
'''
class Record(C.Structure):
    _fields_=[(n,C.c_uint32)for n in ['frame','pc','address','width','value']]+[('a',C.c_uint32*7),('d',C.c_uint32*4),('recent',C.c_uint32*16)]

def trace_palettes(out):
    out.mkdir(parents=True,exist_ok=True)
    (out/'tmp').mkdir(exist_ok=True);(out/'cache').mkdir(exist_ok=True)
    source=out/'observer.c';source.write_text(TRACE_SOURCE);dylib=out/'observer.dylib'
    subprocess.run(['clang','-dynamiclib','-O2',str(source),'-o',str(dylib)],check=True,capture_output=True,env=dict(os.environ,TMPDIR=str(out/'tmp'),CLANG_MODULE_CACHE_PATH=str(out/'cache')))
    bridge=C.CDLL(str(dylib));bridge.configure.argtypes=[C.c_void_p];bridge.get_count.restype=C.c_uint;bridge.get_dropped.restype=C.c_uint;bridge.get_records.restype=C.POINTER(Record);bridge.set_frame.argtypes=[C.c_uint]
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    with ReferenceSession(ROOT,out,core=core)as ref:
        bridge.configure(C.cast(ref.get_register,C.c_void_p));hook=ref.api('set_cpu_hook',None,C.c_void_p);hook(C.cast(bridge.observe,C.c_void_p));frames=[]
        for f in range(1,3601):bridge.set_frame(f);frames.append(ref.step(f,buttons_at(events,f))['reference'])
        hook(None);assert frames==expected and bridge.get_dropped()==0
        raw=bridge.get_records();rows=[]
        for i in range(bridge.get_count()):
            r=raw[i];rows.append({'frame':r.frame,'pc':hex(r.pc),'address':hex(r.address),'width':r.width,'value':hex(r.value),'a':[hex(v)for v in r.a],'d':[hex(v)for v in r.d],'recent':[hex(v)for v in r.recent]})
        report={'identity':ref.identity,'script_sha256':sha(Path(__file__).read_bytes()),'frames_match_original_core':frames==expected,'dropped':bridge.get_dropped(),'records':len(rows),'pass':bool(rows),'writes':rows}
    (out/'palette-trace.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:v for k,v in report.items()if k!='writes'},indent=2))
    from collections import Counter
    print(Counter(r['pc']for r in rows))

def extract_assets(out):
    out.mkdir(parents=True,exist_ok=True)
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();assert sha(rom)==ROM_SHA256
    sprite_path=ROOT/'games/yuyuhakusho/data/sprite-oracle.json.gz'
    original=json.loads(gzip.decompress(sprite_path.read_bytes()))
    bank=[];bank_ids={};tile_sources=[];frames=[];source_tiles={}
    for source in original['library']:
        raw=bytes.fromhex(source['payload']);cursor=0;assignments=[];slots={}
        for _ in range(4):
            descriptor=be(raw,cursor,4);cursor+=4
            for index in range(descriptor&255):
                address=source['tiles_base']+(((descriptor>>16)&0x7fff)+index)*32
                tile=rom[address:address+32];assert len(tile)==32
                source_tiles[address]=tile.hex()
                if tile not in bank_ids:
                    bank_ids[tile]=len(bank);bank.append(tile);tile_sources.append({'address':address,'sha256':sha(tile)})
                slot=((descriptor>>8)&255)+index;slots[slot]=bank_ids[tile]
                assignments.append({'slot':slot,'tile':bank_ids[tile],'source_address':address})
            if descriptor&0x80000000:break
        pieces=[];missing=set()
        while cursor<len(raw):
            value=be(raw,cursor,4);cursor+=4;size=(value>>8)&15;flags=((value&0xf000)>>1)
            width,height=(size>>2)+1,(size&3)+1;first=value&255
            missing.update(i for i in range(first,first+width*height)if i not in slots)
            pieces.append({'center_x':((value>>24)&127)-64,'center_y':((value>>16)&127)-64,
                           'width_tiles':width,'height_tiles':height,'first_slot':first,
                           'flip_x':bool(flags&0x800),'flip_y':bool(flags&0x1000),
                           'palette_slot':((flags>>13)&3)if flags&0x6000 else -1})
            if value&0x80000000:break
        assert cursor==len(raw)
        frames.append({'character_id':source['character_id'],'parameter':source['parameter'],
                       'source_sha256':source['source_sha256'],'assignments':assignments,'pieces':pieces,'required_prior_slots':sorted(missing)})
    palettes=[]
    for cid in range(11):
        address=be(rom,0xfcb8+cid*4,4)
        palettes.append({'character_id':cid,'source_address':address,
                         'variants':[[be(rom,address+variant*32+12+i*2,2)for i in range(10)]for variant in range(4)]})
    shared={'source_address':0x283cc0,'variants':[[be(rom,0x283cc0+variant*32+12+i*2,2)for i in range(10)]for variant in range(8)]}
    assets={'schema':1,'rom_sha256':ROM_SHA256,'sprite_fixture_sha256':sha(sprite_path.read_bytes()),
            'tile_format':'md-4bpp-row-major-high-nibble-first','tile_bytes':b''.join(bank).hex(),
            'tile_sources':tile_sources,'palettes':palettes,'shared_palettes':shared,'frames':frames,
            'palette_prefix_rule':'Indices0..5 come from the caller world palette; selectors replace only6..15',
            'cache_rule':'Apply slot assignments in order; unassigned referenced slots retain prior actor contents'}
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint);bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=(C.c_uint*1)(0x3d8);sample=None
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    bindings=[];pixels=[]
    characters=initializer_bindings(rom)
    # Restore the pinned CPU prefix (through prefetch/run flags) after private
    # direct calls before resuming whole reference frames. RAM edits survive;
    # the dummy return marker must never become the next frame's program PC.
    (out/'tmp').mkdir(exist_ok=True);(out/'cache').mkdir(exist_ok=True)
    probe=out/'cpu_prefix.c';probe.write_text('#include <stddef.h>\n#include <limits.h>\n#include "m68k.h"\nsize_t prefix_size(void){return offsetof(m68ki_cpu_core,aerr_trap);}\n')
    probe_lib=out/'cpu_prefix.dylib'
    core_source=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core'
    subprocess.run(['clang','-dynamiclib','-O2','-I'+str(core_source),'-I'+str(core_source/'m68k'),str(probe),'-o',str(probe_lib)],check=True,capture_output=True,env=dict(os.environ,TMPDIR=str(out/'tmp'),CLANG_MODULE_CACHE_PATH=str(out/'cache')))
    cpu_probe=C.CDLL(str(probe_lib));cpu_probe.prefix_size.restype=C.c_size_t;prefix_size=cpu_probe.prefix_size()
    with ReferenceSession(ROOT,out,core=core)as ref:
        baseline=[]
        for f in range(1,3601):
            baseline.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:
                checkpoint=ref.save();base_ram=ref.ram_cpu();base_cpu=C.string_at(C.addressof(ref.cpu),prefix_size)
                base_registers=[ref.get_register(i)for i in range(21)]
        assert baseline==expected
        setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint);hook=ref.api('set_cpu_hook',None,C.c_void_p);observer=C.cast(bridge.yyh_cpu_hook,C.c_void_p)
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal sample
            assert sample is None
            sample={'ram':ref.ram_cpu(),'registers':[ref.get_register(i)for i in range(18)]};ref.cpu.cycles=4000000
        assert bridge.yyh_set_observer(observe,points,1)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        def write_bytes(address,data):
            for i,b in enumerate(data):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        def invoke(pc):
            nonlocal sample
            setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(16,pc)
            ref.cpu.cycles=0;ref.cpu.refresh_cycles=896;sample=None;hook(observer);execute(1000000);hook(None)
            assert sample is not None
            return sample
        def resume_cpu():
            write_bytes(0xffff80,base_ram[0xff80:])
            C.memmove(C.addressof(ref.cpu),base_cpu,prefix_size)
            assert [ref.get_register(i)for i in range(21)]==base_registers
        # A register-only private function followed by restoration must replay
        # the original complete RAM/video sequence, not merely retain a PC.
        ref.restore(checkpoint,restore_refresh_cycles=True);setreg(0,0);setreg(1,1);setreg(2,10);invoke(0xfc88);resume_cpu()
        assert ref.ram_cpu()==base_ram
        resume_control=[ref.step(f,buttons_at(events,f))['reference']for f in range(2401,2411)]
        assert resume_control==expected[2400:2410]
        for cid in range(11):
            for selector in range(12):
                for slot in range(4):
                    ref.restore(checkpoint,restore_refresh_cycles=True)
                    for reg in range(15):setreg(reg,0)
                    initial=bytes((i*73+cid*31+selector*17+slot*11)&255 for i in range(128));write_bytes(0xffe080,initial)
                    setreg(0,selector);setreg(1,slot);setreg(2,cid+1)
                    selected=invoke(0xfc88);source=selected['registers'][9]&0xffffff
                    assert selected['ram'][0xe080:0xe100]==initial
                    after=invoke(0xfc6a)['ram']
                    allowed=set(range(0xe080+slot*32+12,0xe080+slot*32+32))|set(range(0xff80,0xffe4))
                    unexpected=[i for i,(a,b)in enumerate(zip(selected['ram'],after))if a!=b and i not in allowed];assert not unexpected,unexpected
                    bindings.append({'character_id':cid,'selector':selector,'slot':slot,'source_pointer':source,
                                     'before':initial.hex(),'after':after[0xe080:0xe100].hex(),'return':'0x3d8'})
                # A declared private state then advances actual original frames.
                # The palette call changes only the chosen slot's final10 colors.
                ref.restore(checkpoint,restore_refresh_cycles=True)
                setreg(13,0xff8400);write(0xff8458,cid);invoke(0x126c8)
                for address,value,size in [(0xff8440,(be(base_ram,0xa014,2,True)+96)<<16,4),(0xff9040,(be(base_ram,0xa014,2,True)+260)<<16,4),
                                          (0xff8444,0x1400000,4),(0xff849c,0x1400000,4),(0xff8469,0,1),
                                          (0xff8461,selector&1,1),
                                          (0xff8424,(be(base_ram,0x8424,2)&~0x6000)|0x2000,2)]:write(address,value,size)
                setreg(0,selector);setreg(1,1);setreg(2,cid+1);selected=invoke(0xfc88);source=selected['registers'][9]&0xffffff;invoke(0xfc6a)
                # Resolve the already confirmed palette source without a fade.
                before_palette=ref.ram_cpu()[0xe080:0xe100];write_bytes(0xffe000,before_palette);write_bytes(0xffe100,before_palette)
                write(0xfff501,0);write(0xfff502,0);write(0xfff506,0)
                resume_cpu()
                ref.step(2401,[0]*8)
                ram=ref.ram_cpu();native=bytes((C.c_uint8*65536).in_dll(ref.lib,'vram'));vram=bytearray(65536);vram[0::2],vram[1::2]=native[1::2],native[0::2]
                cram=list((C.c_uint16*64).in_dll(ref.lib,'cram'));regs=list((C.c_uint8*32).in_dll(ref.lib,'reg'))
                actor_bytes=ram[0x8400:0x8500];base=be(actor_bytes,0x24,2)&0x7ff
                assert actor_bytes[0x58]==cid
                assert (be(actor_bytes,0x24,2)>>13)&3==1
                for i in range(10):
                    word=be(rom,source+i*2,2);compact=((word&0xe00)>>3)|((word&0xe0)>>2)|((word&0xe)>>1)
                    assert cram[16+6+i]==compact,(cid,selector,i,cram[16+6+i],compact)
                ref.step(2402,[0]*8)
                pixels.append({'character_id':cid,'selector':selector,'slot':1,'source_pointer':source,
                               'state_frame':2401,'video_frame':2402,'actor':actor_bytes.hex(),
                               'vram_tiles':bytes(vram[base*32:base*32+8192]).hex(),'cram_compact':cram,
                               'world_palette':ram[0xe000:0xe080].hex(),'vdp_registers':regs,'width':ref.width,'height':ref.height,
                               'pixel_format':ref.pixel_format,'video':ref.image.hex()})
        identity=ref.identity
    raw=(json.dumps(assets,separators=(',',':'))+'\n').encode();zipped=gzip.compress(raw,mtime=0);(out/'character-assets.json.gz').write_bytes(zipped)
    oracle={'schema':1,'asset_sha256':sha(raw),'bindings':bindings,'pixels':pixels,
            'source_tiles':[{'address':address,'bytes':data}for address,data in sorted(source_tiles.items())]}
    oraw=(json.dumps(oracle,separators=(',',':'))+'\n').encode();ozip=gzip.compress(oraw,mtime=0);(out/'character-assets-oracle.json.gz').write_bytes(ozip)
    report={'scope':'Palette selector/slot binding CPU calls and declared-private original frame pixels; indexedbank and graphic metadata',
            'identity':identity,'producer_sha256':sha(Path(__file__).read_bytes()),'asset_sha256':sha(raw),'asset_gzip_sha256':sha(zipped),
            'oracle_sha256':sha(oraw),'oracle_gzip_sha256':sha(ozip),'source_sprite_fixture_sha256':sha(sprite_path.read_bytes()),
            'source_characters':11,'palette_selectors':12,'binding_calls':len(bindings),'pixel_cases':len(pixels),
            'cpu_prefix_bytes':prefix_size,'resume_control_frames_match_original':len(resume_control),
            'frames':len(frames),'tiles':len(bank),'prior_tile_dependent_frames':sum(bool(f['required_prior_slots'])for f in frames),
            'frames_match_original_core':baseline==expected,'pass':True,
            'limits':['Selector values0..11 are exact CPU bindings, not a claim that every menu path has been exercised',
                      'Indices0..5 stay world/slot dependent','Seven graphic frames retain unresolved prior-slot dependencies',
                      'Private pixel scenarios use idle original-frame states; whole matches and all poses are not pixel verified']}
    (out/'character-assets-manifest.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))

def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);p.add_argument('--trace',action='store_true');args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT)
    if args.trace:trace_palettes(out)
    else:extract_assets(out)
if __name__=='__main__':main()
