#!/usr/bin/env python3
"""Original ground-action constructors, including their tail-called reset."""
import argparse
from collections import Counter
import ctypes as C
import json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha

ENTRIES={0x19c1c:'reset',0x18440:'stand',0x18454:'rise',0x1846a:'crouch',0x18480:'guard',0x184ac:'turn',0x184f0:'crouch_turn',0x18534:'walk',0x1854a:'prepare_jump'}
BASE={0x59,0x5a,0x5b,0x60,0x64,0x6d,0x77,0x6e}
def allowed(kind):
    result=set(BASE)
    if kind!='reset':result|={0x5d,0x5f,0x6c}
    if kind in ('turn','crouch_turn'):result|={0x61,0x6b}|set(range(0xa8,0xb8))
    if kind=='prepare_jump':result|={0x14}
    return result
def state(obj):
    return {'clock':{'frame_index':obj[0x59],'duration':obj[0x5a],'ticks':obj[0x5b],'flags':obj[0x5c],'completed':obj[0x60]},
            'action_kind':obj[0x5d],'pose':obj[0x5f],'facing':obj[0x61],'auxiliary_64':obj[0x64],
            'request':obj[0x6b],'enabled_stages':obj[0x6c],'action_flags':obj[0x6d],
            'team':obj[0x6e],'base_team':obj[0x6f],'auxiliary_77':obj[0x77],
            'vertical_velocity':int.from_bytes(obj[0x14:0x18],'big',signed=True),'commands':list(obj[0xa8:0xb8])}

def main():
    p=argparse.ArgumentParser();p.add_argument('--out',type=Path,required=True);args=p.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint);bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=list(ENTRIES)+[0x19c44];pcs=(C.c_uint*len(points))(*points)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rows=[];pending={};errors=[];isolated=False;isolated_samples=[]
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            try:
                address=ref.get_register(13)&0xffffff;ram=ref.ram_cpu();off=address&0xffff;obj=ram[off:off+256]
                if isolated:
                    if pc==0x19c44:isolated_samples.append(obj);ref.cpu.cycles=200000
                    return
                if pc in ENTRIES:
                    row={'frame':ref.frame,'object':address,'kind':ENTRIES[pc],'entry':hex(pc),'direction':ref.get_register(0)&255,'held':obj[0x71],'before':state(obj)}
                    pending.setdefault(address,[]).append((row,obj))
                elif pc==0x19c44:
                    for row,before in pending.pop(address,[]):
                        unknown=[i for i,(a,b) in enumerate(zip(before,obj))if a!=b and i not in allowed(row['kind'])]
                        if unknown:raise ValueError(f"Unknown {row['kind']} writes: {unknown}")
                        row['after']=state(obj);row['exit_frame']=ref.frame;rows.append(row)
            except Exception as e:errors.append(str(e))
        assert bridge.yyh_set_observer(observe,pcs,len(pcs))
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        frames=[]
        for f in range(1,3601):
            frames.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        natural={'scope':'Natural constructors and tail-called resets','identity':ref.identity,'counts':dict(Counter(r['kind']for r in rows)),'complete_calls':len(rows),'frames_match_original_core':frames==expected,'errors':errors,'incomplete_calls':sum(len(v)for v in pending.values()),'pass':bool(rows)and frames==expected and not errors and not pending}
        assert natural['pass'],natural
        (out/'ground-actions-oracle.json').write_text(json.dumps(rows,separators=(',',':'))+'\n')
        isolated=True;cases=[];setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(8*size))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        for entry,name in ENTRIES.items():
            for facing in [0,1,128,255]:
                for direction in [0,2,5,9,255]:
                    ref.restore(checkpoint,restore_refresh_cycles=True);setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400);setreg(0,direction)
                    # Declare every byte as input, with distinctive old values;
                    # this routine only touches the fields listed in its audit.
                    for i in range(256):write(0xff8400+i,(i*37+19)&255)
                    write(0xff8461,facing);write(0xff8471,direction);write(0xff846f,0xe7)
                    before=ref.ram_cpu()[0x8400:0x8500];isolated_samples.clear();setreg(16,entry);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
                    ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
                    assert len(isolated_samples)==1,'Incorrect isolated return boundary'
                    after=isolated_samples[0]
                    unknown=[i for i,(a,b) in enumerate(zip(before,after))if a!=b and i not in allowed(name)];assert not unknown,unknown
                    cases.append({'kind':name,'entry':hex(entry),'direction':direction,'held':direction,'before':state(before),'after':state(after)})
        (out/'ground-actions-boundaries.json').write_text(json.dumps(cases,separators=(',',':'))+'\n')
        boundary={'scope':'Isolated original CPU constructors; not natural gameplay','identity':ref.identity,'counts':dict(Counter(r['kind']for r in cases)),'complete_calls':len(cases),'pass':True}
    for name,report in [('ground-actions-oracle',natural),('ground-actions-boundaries',boundary)]:
        report['fixture_sha256']=sha((out/(name+'.json')).read_bytes());report['capture_sha256']=sha(Path(__file__).read_bytes());report['filter_sha256']=sha(bridge_path.read_bytes())
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
