#!/usr/bin/env python3
"""Complete original ground-idle action including jump/recovery constructors."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
from capture_jump import body_state
from capture_effect_pool import pool_state,slot_index

def actor(obj):
    read=lambda off:int.from_bytes(obj[off:off+2],'big',signed=True)
    return body_state(obj)|{'health':read(0x78),'auxiliary_7e':read(0x7e),'auxiliary_80':read(0x80),'transition_delay':obj[0xc8]}
ALLOWED={0x59,0x5a,0x5b,0x60,0x64,0x6d,0x77,0x6e,0x5d,0x5f,0x6c,0x7e,0x7f,0xc8}|set(range(0x10,0x18))|set(range(0x44,0x50))
def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint);bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    pcs=(C.c_uint*4)(0x19cee,0x15afc,0x1fae6,0x1faee)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural=[];cases=[];pending={};errors=[];private=False
    def snapshot(ram,address):
        off=address&0xffff;obj=ram[off:off+256];pointer=int.from_bytes(obj[0x90:0x94],'big')&0xffffff
        source,index=(rom,pointer) if pointer<len(rom) else (ram,pointer&0xffff)
        return {'body':actor(obj),'actor_bytes':obj.hex(),'held':obj[0x71],'pool':pool_state(ram),'parameters':{name:int.from_bytes(source[index+delta:index+delta+4],'big',signed=True)for name,delta in [('vertical_velocity',4),('gravity',8),('horizontal_velocity',12)]}}
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            try:
                address=ref.get_register(13)&0xffffff;ram=ref.ram_cpu()
                if pc==0x19cee:
                    if address in pending:raise ValueError('Overlapping idle action')
                    pending[address]={'frame':ref.frame,'before':snapshot(ram,address),'slots':[]}
                elif address in pending:
                    row=pending[address]
                    if pc in (0x1fae6,0x1faee):row['slots'].append(slot_index(ref.get_register(8)))
                    else:
                        row['after']=snapshot(ram,address);row['exit_frame']=ref.frame
                        before=bytes.fromhex(row['before']['actor_bytes']);after=bytes.fromhex(row['after']['actor_bytes'])
                        unknown=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in ALLOWED];assert not unknown,unknown
                        if not row['slots']:row['slots']=[-1,-1]
                        assert len(row['slots'])==2
                        (cases if private else natural).append(pending.pop(address))
                        if private:ref.cpu.cycles=200000
            except Exception as exc:errors.append(str(exc));ref.cpu.cycles=200000 if private else ref.cpu.cycles
        assert bridge.yyh_set_observer(observe,pcs,4)
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));frames=[]
        for f in range(1,3601):
            frames.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        assert frames==expected and not errors and not pending,(errors,len(pending))
        private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        variants=itertools.product([0,1,2,3,4,13,14,41,42],[0,1,2,255],[0,2],[-1,0,1],[-1,1])
        for number,(pose,completed,held,health,status) in enumerate(variants):
            ref.restore(checkpoint,restore_refresh_cycles=True);setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400);setreg(0,0)
            write(0xff845d,0);write(0xff845f,pose);write(0xff8460,completed);write(0xff8471,held);write(0xff8478,health,2);write(0xff847e,status,2);write(0xff8480,0x2468,2)
            write(0xff8444,0x12345678,4);write(0xff849c,0x1401234,4);write(0xff8414,(([1,5,9][number%3])<<24)|0x5678,4)
            for i in range(16):write(0xff9404+i*0x80,0 if number%3==0 or number%3==1 and i==0 else 0x80,2)
            write(0xffa036,0,2);write(0xff8490,0xff7000,4)
            for offset,value in [(4,-0x58000),(8,0x3600),(12,0x1c000)]:write(0xff7000+offset,value,4)
            n=len(cases);setreg(16,0x19cee);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert len(cases)==n+1 and not errors and not pending,(pose,completed,errors[:3])
        identity=ref.identity
    for name,rows,scope in [('ground-idle-oracle',natural,'Natural complete ground-idle actions before shared tail'),('ground-idle-boundaries',cases,'Explicit isolated original CPU ground-idle matrix')]:
        raw=(json.dumps(rows,separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(compressed)
        report={'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'records':len(rows),'fixture_sha256':sha(raw),'gzip_sha256':sha(compressed),'frames_match_original_core':frames==expected,'incomplete_calls':0,'errors':errors,'pass':bool(rows)and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
