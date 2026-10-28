#!/usr/bin/env python3
"""Original hit-stop / ordinary action prelude after command preparation."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
from capture_ground_idle import actor

ALLOWED={0x69,0x6a,0x86,0xc8,0x7e,0x7f,0x59,0x5b,0x60,0x54,0x55}|set(range(0x40,0x44))|set(range(0x98,0x9c))
def state(ram,address):
    off=address&0xffff;obj=ram[off:off+256]
    return {'body':actor(obj),'hit_stop_ticks':obj[0x69],'shake':obj[0x6a],'screen_offset_x':int.from_bytes(obj[0x54:0x56],'big',signed=True),
            'status_delay':obj[0x86],'residual':int.from_bytes(obj[0x98:0x9c],'big',signed=True),'occupied':ram[0xa042]}
def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint);bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    pcs=(C.c_uint*3)(0x15a20,0x15a58,0x15b40)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows=[];cases=[];pending={};errors=[];private=False
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            try:
                address=ref.get_register(13)&0xffffff;off=address&0xffff;ram=ref.ram_cpu()
                if pc==0x15a20:
                    if address in pending:raise ValueError('Overlapping action prelude')
                    pending[address]={'frame':ref.frame,'before':state(ram,address),'actor_before':ram[off:off+256].hex()}
                elif address in pending:
                    row=pending.pop(address);old=bytes.fromhex(row['actor_before']);obj=ram[off:off+256]
                    unknown=[i for i,(a,b)in enumerate(zip(old,obj))if a!=b and i not in ALLOWED];assert not unknown,unknown
                    row.update(after=state(ram,address),actor_after=obj.hex(),dispatch=pc==0x15a58,exit_frame=ref.frame)
                    (cases if private else rows).append(row)
                    if private:ref.cpu.cycles=200000
            except Exception as e:errors.append(str(e));ref.cpu.cycles=200000 if private else ref.cpu.cycles
        assert bridge.yyh_set_observer(observe,pcs,3)
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));frames=[]
        for f in range(1,3601):
            frames.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None);assert frames==expected and not pending and not errors,(errors,len(pending))
        private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        variants=list(itertools.product([0,1,2,255],[0,1,2,255],[0,1,255],[0,1,255]))
        for i,(stop,shake,status,delay) in enumerate(variants):
            ref.restore(checkpoint,restore_refresh_cycles=True);setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400);setreg(0,0)
            write(0xff8469,stop);write(0xff846a,shake);write(0xff8486,status);write(0xff84c8,delay)
            write(0xff8478,[-1,0,1][i%3],2);write(0xff849c,([0x130,0x140,0x150][(i//3)%3]<<16)|0xabcd,4);write(0xffa042,0x80)
            write(0xff847e,-9,2);write(0xff8480,123,2);write(0xff8454,[0x7fff,0x8000,0xffff][i%3],2)
            write(0xff8498,[0,1,0x4800,0x9000,-1][i%5],4);write(0xff8440,0x7fffffff,4);write(0xff8461,i%3)
            write(0xff8459,255);write(0xff845a,[0,1,255][i%3]);write(0xff845b,[0,255,127][i%3]);write(0xff845c,128 if i%2 else 0);write(0xff8460,7)
            n=len(cases);setreg(16,0x15a20);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert len(cases)==n+1 and not pending and not errors,(i,errors[:3])
        identity=ref.identity
    for name,data,scope in [('action-prelude-oracle',rows,'Natural complete ordinary/hit-stop prelude'),('action-prelude-boundaries',cases,'Explicit isolated original CPU prelude cases')]:
        raw=(json.dumps(data,separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(compressed)
        report={'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'records':len(data),'dispatch':sum(r['dispatch']for r in data),'hit_stop':sum(not r['dispatch']for r in data),'fixture_sha256':sha(raw),'gzip_sha256':sha(compressed),'frames_match_original_core':frames==expected,'incomplete_calls':0,'errors':errors,'pass':bool(data) and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
