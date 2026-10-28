#!/usr/bin/env python3
"""Observe the complete human branch of the original fighter input latch."""
import argparse
from collections import Counter
import ctypes as C
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def main():
    p=argparse.ArgumentParser()
    p.add_argument('--out',required=True,type=Path)
    args=p.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT)
    out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t]
    bridge.yyh_set_observer.restype=C.c_int
    pcs=(C.c_uint*3)(0x155e8,0x15628,0x156b4)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows,pending,errors=[],{},[]
    branch_counts=Counter()
    def state(obj):
        return {'held':obj[0x71]|obj[0xc0]<<8,'pressed':obj[0x72]|obj[0xc1]<<8,
                'changed':obj[0x73]|obj[0xc2]<<8,'raw_held':obj[0x74],'retain_ticks':obj[0x75]}
    def control(ram,slot):
        return {name:ram[0xf780+slot+i*8]|ram[0xf7a8+slot+i*8]<<8 for i,name in enumerate(['current','previous','changed','pressed','released'])}
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            try:
                address=ref.get_register(13)&0xffffff
                ram=ref.ram_cpu();off=address&0xffff;obj=ram[off:off+256]
                if pc==0x155e8:
                    channel=obj[0x76]
                    if channel==255:return
                    if not channel<8:raise ValueError('Unexpected human channel')
                    if address in pending:raise ValueError('Overlapping fighter input call')
                    pending[address]=({'frame':ref.frame,'object':address,'channel':channel,
                                       'before':state(obj),'controls':control(ram,channel)},obj)
                elif address in pending:
                    row,before=pending.pop(address)
                    allowed={0x71,0x72,0x73,0x74,0x75,0xc0,0xc1,0xc2}
                    unknown=[i for i,(a,b) in enumerate(zip(before,obj)) if a!=b and i not in allowed]
                    if unknown:raise ValueError(f'Unknown input side effects {unknown}')
                    row['after']=state(obj);row['exit']=hex(pc);row['exit_frame']=ref.frame
                    rows.append(row)
                    branch_counts['retaining' if row['before']['retain_ticks'] else 'direct']+=1
            except Exception as e:errors.append(str(e))
        assert bridge.yyh_set_observer(observe,pcs,3)
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        frames=[ref.step(f,buttons_at(events,f))['reference'] for f in range(1,3601)]
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        report={'scope':'Complete human input latch; AI and recording excluded','identity':ref.identity,
                'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),
                'frames':3600,'frames_match_original_core':frames==expected,'complete_calls':len(rows),
                'branch_counts':dict(branch_counts),'incomplete_calls':[r[0] for r in pending.values()],
                'errors':errors,'pass':bool(rows) and not pending and not errors and frames==expected}
    fixture=out/'fighter-input-oracle.json'
    fixture.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    report['fixture_sha256']=sha(fixture.read_bytes())
    (out/'report.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))
    assert report['pass']


if __name__=='__main__':main()
