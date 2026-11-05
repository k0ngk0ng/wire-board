#!/usr/bin/env python3
"""Physical-input source4 discovery only; no register/RAM/ROM state edits."""
import argparse
from collections import Counter
import ctypes as C
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

ROOTS = {0x1904e:'r1',0x190a8:'r2',0x190d2:'r3-b',0x190f0:'r3-a',0x1911a:'r4-b',0x1913c:'r4-a',
         0x19190:'r6',0x191d4:'air3-b',0x191ec:'air3-a',0x19092:'rush-finish',0x191ba:'dragon-followup',
         0x13bb6:'dragon-primary',0x13e9a:'dragon-tail',0x13fc8:'dragon-enhanced'}

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--inputs',type=Path,required=True);ap.add_argument('--out',type=Path,required=True);ap.add_argument('--frames',type=int,default=3602);args=ap.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT) and args.inputs.resolve().is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    events=json.loads(args.inputs.read_text());core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge=C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'));typ=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[typ,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int;points=(C.c_uint*len(ROOTS))(*ROOTS)
    hits=[];samples=[];counts=Counter();errors=[]
    def state(ram):
        a=ram[0x8400:0x8500]
        def w(o):return int.from_bytes(a[o:o+2],'big',signed=True)
        return {'hp':w(0x78),'energy':w(0xb8),'action':a[0x5d],'pose':a[0x5f],'facing':a[0x61],'request':a[0x6b],'stages':a[0x6c],
                'x':w(0x40),'y':w(0x44),'child_size':int.from_bytes(ram[0x8504:0x8506],'big'),'child_code':ram[0x8500:0x8504].hex(),'movement_source':hex(int.from_bytes(a[0x90:0x94],'big'))}
    with ReferenceSession(ROOT,out,core=core) as ref:
        @typ
        def observe(k,width,pc,value):
            try:
                address=ref.get_register(13)&65535
                if 0x8400<=address<0x8800 or pc in [0x13e9a]:
                    ram=ref.ram_cpu();hits.append({'frame':ref.frame,'pc':hex(pc),'kind':ROOTS[pc],'object':hex(address),'parent':hex(int.from_bytes(ram[address+0xc:address+0xe],'big')),'state':state(ram)})
            except Exception as exc:errors.append(repr(exc))
        assert bridge.yyh_set_observer(observe,points,len(ROOTS));hook=ref.api('set_cpu_hook',None,C.c_void_p);hook(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        previous=None
        for frame in range(1,args.frames+1):
            ref.step(frame,buttons_at(events,frame));ram=ref.ram_cpu()
            if ram[0x8458]==4 and int.from_bytes(ram[0x8404:0x8406],'big')==256:
                s=state(ram);counts[hex(s['action'])]+=1
                key=(s['action'],s['hp'],s['energy'])
                if key!=previous or frame%100==0:samples.append({'frame':frame,**s});previous=key
        hook(None);identity=ref.identity
        ref.screenshot(out/'source4-special-discovery-final.png')
    r={'scope':'Cold physical source4 special discovery, passive hook only; no state/seed/register writes','identity':identity,'input':{'path':str(args.inputs.resolve().relative_to(ROOT)),'sha256':sha(args.inputs.read_bytes())},'capture_sha256':sha(Path(__file__).read_bytes()),'frames':args.frames,'action_counts':dict(counts),'hits':hits,'state_changes':samples,'errors':errors,'unobserved_control_generated':False}
    (out/'source4-special-discovery-report.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({'actions':dict(counts),'hits':[(h['frame'],h['kind']) for h in hits if h['object']=='0x8400'],'child_hits':dict(Counter(h['kind'] for h in hits if h['object']!='0x8400')),'last':samples[-1] if samples else None,'errors':errors},indent=2),flush=True)
    assert not errors

if __name__=='__main__':main()
