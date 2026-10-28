#!/usr/bin/env python3
"""Declared private-state original CPU cases for the human input latch.

The hold-window cases are isolated calls, not additional natural-game coverage.
"""
import argparse
import ctypes as C
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def main():
    p=argparse.ArgumentParser()
    p.add_argument('--out',required=True,type=Path)
    args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT)
    out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    pcs=(C.c_uint*2)(0x15628,0x156b4)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    result=[];samples=[]
    def state(ram):
        obj=ram[0x8400:0x8500]
        return {'held':obj[0x71]|obj[0xc0]<<8,'pressed':obj[0x72]|obj[0xc1]<<8,'changed':obj[0x73]|obj[0xc2]<<8,'raw_held':obj[0x74],'retain_ticks':obj[0x75]}
    with ReferenceSession(ROOT,out,core=core) as ref:
        for frame in range(1,2401):ref.step(frame,buttons_at(events,frame))
        checkpoint=ref.save();setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        @cbtype
        def observe(kind,width,pc,value):
            samples.append((pc,ref.ram_cpu()))
            ref.cpu.cycles=200000
        assert bridge.yyh_set_observer(observe,pcs,2)
        for ticks,mask,high in itertools.product([0,1,255],range(8),[0x0,0xf0]):
            ref.restore(checkpoint,restore_refresh_cycles=True)
            setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400);setreg(0,0)
            for off,val in [(0x71,0x50),(0x72,0x20),(0x73,0x70),(0x74,0x99),(0x75,ticks),(0x76,0),(0xc0,0xab),(0xc1,0xcd),(0xc2,0xef)]:write(0xff8400+off,val)
            current=(high|1)<<8 | 0x89 | (0x10 if mask&1 else 0)
            pressed=(high|2)<<8 | 0x04 | (0x20 if mask&2 else 0)
            changed=(high|4)<<8 | 0x81 | (0x40 if mask&4 else 0)
            controls={'current':current,'previous':0x3412,'pressed':pressed,'changed':changed,'released':0x7856}
            for i,name in enumerate(['current','previous','changed','pressed','released']):
                write(0xfff780+i*8,controls[name]&255);write(0xfff7a8+i*8,controls[name]>>8)
            before=ref.ram_cpu();samples.clear();setreg(16,0x155e8);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert len(samples)==1,'Unexpected original routine exit count'
            pc,after=samples[0]
            allowed={0x71,0x72,0x73,0x74,0x75,0xc0,0xc1,0xc2}
            unexpected=[i for i,(a,b) in enumerate(zip(before[0x8400:0x8500],after[0x8400:0x8500]))if a!=b and i not in allowed]
            assert not unexpected,unexpected
            result.append({'retain_case':ticks,'attack_presence_mask':mask,'extension_high':high,'entry':'0x155e8','exit':hex(pc),'before':state(before),'controls':controls,'after':state(after)})
        identity=ref.identity
    fixture=out/'fighter-input-boundaries.json';fixture.write_text(json.dumps(result,separators=(',',':'))+'\n')
    report={'scope':'Isolated original human-latch calls; not natural gameplay','identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'fixture_sha256':sha(fixture.read_bytes()),'complete_calls':len(result),'retention_values':[0,1,255],'independent_attack_presence_combinations':8,'extension_high_patterns':[0,240],'unknown_actor_writes':0,'pass':True}
    (out/'report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
