#!/usr/bin/env python3
"""Capture the complete original lane-switch constructor and its effect pool."""
import argparse
from collections import Counter
import ctypes as C
import itertools
import json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
from capture_ground_actions import state as action_state, BASE

def body(obj):
    read=lambda off,n,signed=False:int.from_bytes(obj[off:off+n],'big',signed=signed)
    return action_state(obj)|{'x':read(0x40,4,True),'y':read(0x44,4,True),'velocity_x':read(0x10,4,True),
                              'gravity':read(0x4c,4,True),'ground':read(0x9c,4,True),'extra_jump':obj[0x87],
                              'attributes':read(0x24,2),'layer':obj[0x26],'depth':read(0x3e,2)}

def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);p.add_argument('--inputs',type=Path);args=p.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    pcs=(C.c_uint*4)(0x186be,0x19c44,0x1fcf2,0x1fd3e)
    input_path=args.inputs or ROOT/'docs/research/md-yuyuhakusho/inputs.json'
    events=json.loads(input_path.read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    if args.inputs:
        with ReferenceSession(ROOT,out/'unobserved') as unobserved:
            expected=[unobserved.step(f,buttons_at(events,f))['reference'] for f in range(1,3601)]
        (out/'unobserved-frames.json').write_text(json.dumps(expected,separators=(',',':'))+'\n')
    rows=[];pending={};errors=[];isolated=False;case=None
    allowed=BASE|{0x5d,0x5f,0x6c,0x26}|set(range(0x10,0x18))|set(range(0x9c,0xa0))|{0x3e,0x3f}
    def snapshot(ram,address):
        off=address&0xffff;obj=ram[off:off+256]
        return {'body':body(obj),'pool':{'cursor':int.from_bytes(ram[0xa036:0xa038],'big'),'bytes':ram[0x9400:0x9c00].hex()},'occupied_lanes':ram[0xa042]},obj
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            try:
                address=ref.get_register(13)&0xffffff;ram=ref.ram_cpu()
                if pc==0x186be:
                    if address in pending:raise ValueError('Overlapping lane constructor')
                    before,obj=snapshot(ram,address)
                    pending[address]=({'scope':'isolated-original-cpu' if isolated else 'natural','case':case,'frame':ref.frame,'object':address,'before':before,'slots':[-1,-1]},obj)
                elif address in pending:
                    record,old=pending[address]
                    if pc in (0x1fcf2,0x1fd3e):
                        slot=((ref.get_register(8)&0xffffff)-0xff9400)//0x80
                        if not 0<=slot<16:raise ValueError('Invalid original allocated slot')
                        record['slots'][0 if pc==0x1fcf2 else 1]=slot
                    else:
                        after,obj=snapshot(ram,address)
                        unknown=[i for i,(a,b) in enumerate(zip(old,obj))if a!=b and i not in allowed]
                        if unknown:raise ValueError(f'Unknown lane actor writes {unknown}')
                        record['after']=after;record['exit_frame']=ref.frame;rows.append(record);del pending[address]
                        if isolated:ref.cpu.cycles=200000
            except Exception as e:errors.append(str(e));ref.cpu.cycles=200000 if isolated else ref.cpu.cycles
        assert bridge.yyh_set_observer(observe,pcs,4)
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        frames=[]
        for f in range(1,3601):
            frames.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        assert frames==expected and not pending and not errors,(len(pending),errors[:3])
        natural_count=len(rows);isolated=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(8*size))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        for ground,occupancy,pool_kind,cursor,layer in itertools.product([0x1400042,0x130ff01],[0,1,2,3,255],['empty','full','single'],[0,0x780],[0,240]):
            ref.restore(checkpoint,restore_refresh_cycles=True);setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400);setreg(0,0)
            write(0xff849c,ground,4);write(0xff8426,layer);write(0xff843e,0x1234,2);write(0xffa042,occupancy);write(0xffa036,cursor,2)
            for i in range(0x800):write(0xff9400+i,(i*13+27)&255)
            for slot in range(16):write(0xff9404+slot*0x80,0 if pool_kind=='empty' or pool_kind=='single' and slot==0 else 0x80,2)
            case={'ground':ground,'occupied_lanes':occupancy,'pool':pool_kind,'cursor':cursor,'layer':layer}
            n=len(rows);setreg(16,0x186be);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert len(rows)==n+1 and not pending and not errors,(case,errors[:3])
        report={'scope':'Complete lane-switch initialization before subsequent motion','identity':ref.identity,'capture_sha256':sha(Path(__file__).read_bytes()),'inputs_sha256':sha(input_path.read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),'frames_match_original_core':frames==expected,'natural_calls':natural_count,'isolated_calls':len(rows)-natural_count,'complete_calls':len(rows),'incomplete_calls':0,'errors':errors,'branch_counts':dict(Counter(str(r['after']['body']['action_kind'])for r in rows)),'pass':bool(rows) and not errors}
    fixture=out/'lane-switch-oracle.json';fixture.write_text(json.dumps(rows,separators=(',',':'))+'\n');report['fixture_sha256']=sha(fixture.read_bytes());(out/'report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
