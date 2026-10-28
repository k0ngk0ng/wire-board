#!/usr/bin/env python3
"""Original animation/frame-shape submission including the sound-queue tail."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
import os
from pathlib import Path
import subprocess
from reference import ROOT,ReferenceSession,buttons_at,sha

RETURNS={0x1edaa,0x21970,0x21982,0x21994,0x219a6,0x219aa}
ACTOR_WRITES={0x5a,0x5c,0x6c,0x24,0x64,0x65,0x66,0x67,0x68}|set(range(0x28,0x2c))|{0x3c,0x3d}|set(range(0x94,0x98))

def number(data,offset,size,signed=False):return int.from_bytes(data[offset:offset+size],'big',signed=signed)

def actor_state(obj,address):
    return {'object_id':address&0xffff,'animation_set':number(obj,0x50,4),'shape_set':number(obj,0x8c,4),
            'pose':obj[0x5f],'clock':{'frame_index':obj[0x59],'duration':obj[0x5a],'ticks':obj[0x5b],'flags':obj[0x5c],'completed':obj[0x60]},
            'facing':obj[0x61],'action_flags':obj[0x6d],'enabled_stages':obj[0x6c],
            'attributes':number(obj,0x24,2),'graphic_offset_x':number(obj,0x28,2,True),'graphic_offset_y':number(obj,0x2a,2,True),
            'world_x':number(obj,0x40,4,True),'world_y':number(obj,0x44,4,True),'graphic_parameter':number(obj,0x3c,2),
            'auxiliary_94':number(obj,0x94,4),'auxiliary_64':obj[0x64],'value_65':obj[0x65],'value_66':obj[0x66],'value_67':obj[0x67],'value_68':obj[0x68]}

def snapshot(ram,address):
    off=address&0xffff;obj=ram[off:off+256]
    return {'actor':actor_state(obj,address),'actor_bytes':obj.hex(),'buffers':ram[0xa200:0xa600].hex(),
            'offsets':[number(ram,0xa004+i*2,2)for i in range(4)],'pending_sound':ram[0xa030],'sound_queue':list(ram[0xf80a:0xf80e])}

def selected_data(rom,ram,obj):
    def read(address,n):
        address&=0xffffff
        if address+n<=len(rom):return rom[address:address+n]
        if 0xff0000<=address and address+n<=0x1000000:return ram[address&0xffff:(address&0xffff)+n]
        raise ValueError(f'Unsupported table address {address:x}')
    anim=number(obj,0x50,4);shape=number(obj,0x8c,4);pose=obj[0x5f];index=obj[0x59]
    record_address=(anim+number(read(anim+pose*2,2),0,2,True)+index*8)&0xffffff
    record=read(record_address,8);directory=read(shape+record[1]*4,4)
    payload_address=(shape+number(directory,2,2,True))&0xffffff;p=payload_address
    metadata=[0,0,0,0];groups=[[],[],[],[]]
    if directory[0]&0x40:metadata=list(read(p,4));p+=4
    for group,flag in enumerate([0x40,0x20,0x10,0x08]):
        if not directory[0]&flag:continue
        for _ in range(64):
            row=read(p,4);p+=4
            groups[group].append({'width':row[0],'offset_x':row[1]-0x80,'height':row[2],'offset_y':row[3]-0x80})
            if group==3 or row[0]&0x80:break
        else:raise ValueError('Shape list has no terminal marker within declared budget')
    key={'animation_set':anim,'shape_set':shape,'pose':pose,'index':index}
    data={'duration':record[0],'flags':record[2],'enabled_stages':record[3],
          'sound':rom[0x218d8+record[4]],'offset_x':record[6]-0x80,'offset_y':record[7]-0xa0,
          'graphic_parameter':directory[1]*2,'shape_flags':directory[0],'metadata':metadata,'groups':groups}
    source={'animation_address':hex(record_address),'animation_bytes':record.hex(),'directory_bytes':directory.hex(),'shape_address':hex(payload_address),'shape_bytes':read(payload_address,p-payload_address).hex(),'sound_index':record[4]}
    return key,data,source

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--out',type=Path,required=True);args=ap.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    observer_source=out/'observer.c'
    observer_source.write_text('''#include <stddef.h>
typedef void(*callback_t)(int,int,unsigned,unsigned);
static callback_t cb; static unsigned pcs[64],last_pc,recent[64],position;static size_t count;
int yyh_set_observer(callback_t callback,const unsigned *addresses,size_t n){if(n>64)return 0;cb=callback;count=n;for(size_t i=0;i<n;i++)pcs[i]=addresses[i];return 1;}
unsigned yyh_last_pc(void){return last_pc;}
unsigned yyh_recent_pc(unsigned distance){return recent[(position-distance-1)&63];}
void yyh_cpu_hook(int type,int width,unsigned address,unsigned value){
 if(type==1){last_pc=address;recent[position++&63]=address;}
 if(!cb)return;
 if(type==4){unsigned a=address&0xffffff;if(a<0xfff80e&&a+(unsigned)width>0xfff80a)cb(type,width,address,value);return;}
 if(type!=1)return;for(size_t i=0;i<count;i++)if(address==pcs[i]){cb(type,width,address,value);return;}
}
''')
    bridge_path=out/'observer.dylib';(out/'tmp').mkdir(exist_ok=True);(out/'cache').mkdir(exist_ok=True)
    subprocess.run(['clang','-dynamiclib','-O2',str(observer_source),'-o',str(bridge_path)],check=True,capture_output=True,env=dict(os.environ,TMPDIR=str(out/'tmp'),CLANG_MODULE_CACHE_PATH=str(out/'cache')))
    bridge=C.CDLL(str(bridge_path));bridge.yyh_last_pc.restype=C.c_uint;bridge.yyh_recent_pc.argtypes=[C.c_uint];bridge.yyh_recent_pc.restype=C.c_uint;cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=[0x1eb40]+sorted(RETURNS);pcs=(C.c_uint*len(points))(*points)
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());baseline=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural=[];frames_library={};pending={};errors=[];isolated=False;sample=None
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal sample
            try:
                if kind==4:
                    if not isolated:
                        for row,_ in pending.values():row['sound_writes'].append({'pc':hex(bridge.yyh_last_pc()),'address':hex(pc&0xffffff),'width':width,'value':value,'frame':ref.frame,'sr':ref.get_register(17),'stack':ref.get_register(15),'recent_pc':[hex(bridge.yyh_recent_pc(i))for i in range(16)]})
                    return
                address=ref.get_register(13)&0xffffff;sp=ref.get_register(15);ram=ref.ram_cpu()
                if isolated:
                    if pc in RETURNS:
                        if sample is not None:raise ValueError('Multiple isolated animation exits')
                        sample=(ram,pc);ref.cpu.cycles=200000
                    return
                call_key=(address,sp)
                if pc==0x1eb40:
                    if call_key in pending:raise ValueError('Overlapping original animation call')
                    before=snapshot(ram,address);key,data,source=selected_data(rom,ram,bytes.fromhex(before['actor_bytes']))
                    library_key=json.dumps(key,sort_keys=True)
                    if library_key in frames_library and frames_library[library_key]['data']!=data:raise ValueError('Same animation/shape identity changed immutable data')
                    frames_library[library_key]={'key':key,'data':data,'source':source}
                    pending[call_key]=({'frame':ref.frame,'actor':address,'return_stack':sp,'entry_sr':ref.get_register(17),'before':before,'key':key,'sound_writes':[]},ram)
                elif call_key in pending:
                    row,before_ram=pending.pop(call_key);after=snapshot(ram,address)
                    old=bytes.fromhex(row['before']['actor_bytes']);new=bytes.fromhex(after['actor_bytes'])
                    unknown=[i for i,(a,b)in enumerate(zip(old,new))if a!=b and i not in ACTOR_WRITES]
                    if unknown:raise ValueError(f'Unexpected actor writes {unknown}')
                    offset=address&0xffff
                    permitted=set(range(offset,offset+256))|set(range(0xa200,0xa600))|set(range(0xa004,0xa00c))|{0xa030}|set(range(0xf80a,0xf80e))
                    external=[{'offset':i,'before':a,'after':b}for i,(a,b)in enumerate(zip(before_ram,ram))if a!=b and i not in permitted]
                    row.update(after=after,exit=hex(pc),exit_frame=ref.frame,external_changes=external)
                    natural.append(row)
            except Exception as e:errors.append(str(e))
        assert bridge.yyh_set_observer(observe,pcs,len(points))
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        frames=[]
        for f in range(1,3601):
            frames.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        assert frames==baseline and not pending and not errors,(len(pending),errors[:8])
        natural_report={'scope':'Complete natural animation/shape submission with sound tail','identity':ref.identity,
                        'complete_calls':len(natural),'library_frames':len(frames_library),'incomplete_calls':len(pending),'errors':errors,
                        'frames_match_original_core':frames==baseline,'external_change_calls':sum(bool(r['external_changes'])for r in natural),
                        'exit_counts':dict(Counter(r['exit']for r in natural)),
                        'external_sound_consumer_calls':sum(any(w['pc']=='0x37c56'for w in r['sound_writes'])for r in natural),
                        'limitations':['Natural snapshots retain external sound-consumer interleaving; capture equivalence does not mean an atomic animation function predicts those unrelated writes.'],
                        'pass':bool(natural)}
        natural_payload={'library':list(frames_library.values()),'calls':natural}
        (out/'animation-records-oracle.json.gz').write_bytes(gzip.compress((json.dumps(natural_payload,separators=(',',':'))+'\n').encode(),mtime=0))
        # Declared private-state cases: immutable synthetic data tables are read
        # by the original CPU, with checkpoint restored before each invocation.
        isolated=True;cases=[];setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        def case(name,flags=0x73,counts=(0,0,0,0),length=1,ticks=0,facing=0,sound_index=0,pending_sound=0,queue=(0,0,0,0),world=(0x7fff1234,-0x80000000),record_flags=0x13):
            nonlocal sample
            ref.restore(checkpoint,restore_refresh_cycles=True);setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400)
            for i in range(8):setreg(i,0)
            for i in range(256):write(0xff8400+i,(i*37+19)&255)
            write(0xff8450,0xff6000,4);write(0xff848c,0xff6200,4);write(0xff845f,0);write(0xff8459,0);write(0xff845b,ticks);write(0xff8461,facing)
            write(0xff8440,world[0],4);write(0xff8444,world[1],4);write(0xff6000,0x100,2)
            record=[7,0,record_flags,0x0b,sound_index,0xa5,0,255]
            for i,b in enumerate(record):write(0xff6100+i,b)
            write(0xff6200,flags);write(0xff6201,255);write(0xff6202,0x100,2)
            p=0xff6300
            if flags&0x40:
                for b in [0x91,0x72,0x53,0xa5]:write(p,b);p+=1
            for group,flag in enumerate([0x40,0x20,0x10,0x08]):
                if not flags&flag:continue
                n=1 if group==3 else length
                for j in range(n):
                    for b in [(0x80 if j==n-1 else 0)|(j+3),0 if j%2==0 else 255,0xe1,255 if j%2==0 else 0]:write(p,b);p+=1
            for i in range(0x400):write(0xffa200+i,(i*17+11)&255)
            for i,n in enumerate(counts):write(0xffa004+i*2,n,2)
            write(0xffa030,pending_sound)
            for i,q in enumerate(queue):write(0xfff80a+i,q)
            before_ram=ref.ram_cpu();before=snapshot(before_ram,0xff8400);key,data,source=selected_data(rom,before_ram,bytes.fromhex(before['actor_bytes']))
            sample=None;setreg(16,0x1eb40);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert sample is not None and not errors,(name,errors[:3])
            after_ram,pc=sample;after=snapshot(after_ram,0xff8400)
            changed=[i for i,(a,b)in enumerate(zip(before_ram,after_ram))if a!=b]
            permitted=set(range(0x8400,0x8500))|set(range(0xa200,0xa600))|set(range(0xa004,0xa00c))|{0xa030}|set(range(0xf80a,0xf80e))
            assert all(i in permitted for i in changed),(name,[hex(i)for i in changed if i not in permitted])
            unknown=[i for i,(a,b)in enumerate(zip(bytes.fromhex(before['actor_bytes']),bytes.fromhex(after['actor_bytes'])))if a!=b and i not in ACTOR_WRITES];assert not unknown,(name,unknown)
            cases.append({'scope':'isolated-original-cpu','name':name,'before':before,'after':after,'key':key,'data':data,'source':source,'exit':hex(pc)})
        sound_index=next(i for i in range(256)if rom[0x218d8+i]>=128)
        sound=rom[0x218d8+sound_index]
        for groupflags in [0,8,16,32,64,120]:
            for facing in [0,1,2,3,128,255]:case(f'groups-{groupflags}-facing-{facing}',flags=groupflags,facing=facing)
        for count in [0,0xe0,0xf0,0xff,0x100,0xfff0]:
            for length in [1,2,18]:case(f'capacity-{count}-{length}',flags=0x78,counts=(count,)*4,length=length)
        for ticks in [0,1,255]:
            for recordflags in [0,0x10,0xff]:case(f'ticks-{ticks}-flags-{recordflags}',flags=0x78,ticks=ticks,record_flags=recordflags,sound_index=sound_index)
        for position in range(5):
            queue=[0x80+(sound%64)+i for i in range(4)]
            queue=[(q+7)&255 if q==sound else q for q in queue]
            if position<4:queue[position]=0 if position%2==0 else sound
            case(f'sound-position-{position}',flags=0,pending_sound=sound,queue=queue,ticks=1)
        case('zero-sound-preserves-existing-request',flags=0,sound_index=0,pending_sound=0x12,queue=(0x81,0x82,0x83,0x84),record_flags=0)
        boundary_report={'scope':'Isolated original CPU animation/shape and sound-queue cases','identity':ref.identity,'complete_calls':len(cases),'errors':errors,'pass':True}
        (out/'animation-records-boundaries.json.gz').write_bytes(gzip.compress((json.dumps({'calls':cases},separators=(',',':'))+'\n').encode(),mtime=0))
    for name,report in [('animation-records-oracle',natural_report),('animation-records-boundaries',boundary_report)]:
        p=out/(name+'.json.gz');report.update(fixture_sha256=sha(gzip.decompress(p.read_bytes())),gzip_sha256=sha(p.read_bytes()),capture_sha256=sha(Path(__file__).read_bytes()),filter_sha256=sha(bridge_path.read_bytes()))
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
