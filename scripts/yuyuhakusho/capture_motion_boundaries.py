#!/usr/bin/env python3
"""Execute the original arithmetic routines with declared isolated inputs.

This changes a private emulator state. It is not a natural gameplay trace.
Every case restores the same reference checkpoint before setting its inputs.
"""
import argparse
import ctypes as C
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def main():
    ap=argparse.ArgumentParser()
    ap.add_argument('--out', required=True, type=Path)
    args=ap.parse_args()
    out=args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge=C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t]
    bridge.yyh_set_observer.restype=C.c_int
    exits=[0x39aa,0x1574e,0x19dc2,0x19df2,0x19df6]
    pcs=(C.c_uint*len(exits))(*exits)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    rows={'rng':[],'projection':[],'walk':[],'air':[]}
    samples=[]
    with ReferenceSession(ROOT,out,core=core) as ref:
        for f in range(1,2401):ref.step(f,buttons_at(events,f))
        checkpoint=ref.save()
        setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
        execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size):
            raw=(value & ((1<<(8*size))-1)).to_bytes(size,'big')
            for i,b in enumerate(raw):C.c_ubyte.from_address(ref.ram_pointer+((address+i)&0xffff ^ 1)).value=b
        def read(ram,address,size,signed=False):
            return int.from_bytes(ram[address&0xffff:(address&0xffff)+size],'big',signed=signed)
        @cbtype
        def observe(kind,width,pc,value):
            samples.append((pc,ref.ram_cpu(),ref.get_register(0)))
            # The callback is before the watched instruction. Capture first;
            # the loop executes that instruction once then reaches its budget.
            ref.cpu.cycles=200000
        assert bridge.yyh_set_observer(observe,pcs,len(pcs))
        def call(entry,setup):
            ref.restore(checkpoint,restore_refresh_cycles=True)
            setreg(17,0x2700)  # no asynchronous interrupt during this isolated call
            setreg(15,0xffffe0)
            write(0xffffe0,0x3d8,4)
            setreg(13,0xff8400)
            for n in range(8):setreg(n,0)
            setup()
            before=ref.ram_cpu()
            setreg(16,entry)
            ref.cpu.cycles=0
            ref.cpu.refresh_cycles=896
            samples.clear()
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
            execute(100000)
            ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert len(samples)==1, f'Original routine did not reach exactly one declared exit: {samples[:1]}'
            pc,after,d0=samples[0]
            return before,after,pc,d0

        for seed in [0,1,0xffff,0x10000,0x7fffffff,0x80000000,0xffffffff,0x2a6d365a,0x12345678]:
            before,after,pc,d0=call(0x3982,lambda:write(0xffff18,seed,4))
            rows['rng'].append({'seed':seed,'next_seed':read(after,0xffff18,4),'returned':d0,'entry':'0x3982','exit':hex(pc)})

        def pstate(ram):
            return {'world_x':read(ram,0xff8440,4,True),'world_y':read(ram,0xff8444,4,True),
                    'screen_x':read(ram,0xff8408,2,True),'screen_y':read(ram,0xff840c,2,True),
                    'offset_x':read(ram,0xff8454,2,True),'offset_y':read(ram,0xff8456,2,True),
                    'camera_x':read(ram,0xffa014,2,True),'camera_y':read(ram,0xffa016,2,True),
                    'shake':read(ram,0xfff566,2,True)}
        for values in [(0,0,0,0,0,0,0), (0x7fff0000,0x7fff0000,-1,-1,0,0,0),
                       (-0x80000000,-0x80000000,1,1,0,0,0),(0,0,0,0,-32768,32767,-32768),
                       (383<<16,416<<16,0,0,0,0,0),(-64<<16,32<<16,0,0,0,0,0),
                       (100<<16,100<<16,100,100,10,-10,20)]:
            def setup_projection():
                for address,v,size in zip([0xff8440,0xff8444,0xffa014,0xffa016,0xff8454,0xff8456,0xfff566],values,[4,4,2,2,2,2,2]):write(address,v,size)
            before,after,pc,d0=call(0x156f0,setup_projection)
            rows['projection'].append({'before':pstate(before),'after':pstate(after),'entry':'0x156f0','exit':hex(pc)})

        def mstate(ram):
            return {name:read(ram,0xff8400+offset,4,True) for name,offset in
                    [('x',0x40),('y',0x44),('vx',0x10),('vy',0x14),('gravity',0x4c),('ground',0x9c)]}|{'facing':read(ram,0xff8461,1)}
        def setup_motion(values,speed=0):
            for off,val in zip([0x40,0x44,0x10,0x14,0x4c,0x9c],values[:6]):write(0xff8400+off,val,4)
            write(0xff8461,values[6],1)
            # Replace only this declared input table pointer in the private case.
            write(0xff8490,0xff7000,4)
            write(0xff7000,speed,4)
        for x,speed,facing in [(0,0x18000,0),(0,0x18000,1),(0,-0x18000,0),(0,-0x18000,128),(0x7fffffff,1,0),(-0x80000000,1,1)]:
            before,after,pc,d0=call(0x19da2,lambda:setup_motion((x,123,0,0,0,0x55aa55aa,facing),speed))
            rows['walk'].append({'before':mstate(before),'after':mstate(after),'speed':speed,'entry':'0x19da2','exit':hex(pc)})
        for values in [(0,0,0,-0x10000,0x1000,0,0),(0,0xffff,0,0,0,1,0),
                       (0,0x0000ffff,0,-1,0,0,0),(0,0x10000,0,0,0,0x10001,0),
                       (0x7fffffff,0x7fffffff,1,0x7fffffff,1,0,0),
                       (-0x80000000,-0x80000000,-1,-1,1,-0x80000000,0),
                       (0,0x7fff0000,0,0,0,-0x80000000,0),(0,-0x80000000,0,0,0,0x7fff0000,0)]:
            before,after,pc,d0=call(0x19dc6,lambda:setup_motion(values))
            rows['air'].append({'before':mstate(before),'after':mstate(after),'landing':pc==0x19df6,'entry':'0x19dc6','exit':hex(pc)})
        identity=ref.identity
    fixture=out/'motion-boundaries.json'
    fixture.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    report={'scope':'Controlled isolated original CPU calls; not natural gameplay','identity':identity,
            'capture_sha256':sha(Path(__file__).read_bytes()),'counts':{k:len(v) for k,v in rows.items()},
            'fixture_sha256':sha(fixture.read_bytes()),'pass':True,
            'limitations':['Every case restores checkpoint and declares changed inputs','Only declared routine exit is observed','Not a whole-game determinism test']}
    (out/'report.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))


if __name__=='__main__':main()
