#!/usr/bin/env python3
"""Natural, isolated and continuous original lane-motion phases."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
from capture_jump import body_state
from capture_effect_pool import pool_state,slot_index

ENTRIES={0x19e08:10,0x19ecc:11}
ACTOR_WRITES=set(range(0x14,0x18))|{0x26,0x3e,0x3f}|set(range(0x40,0x48))|set(range(0x9c,0xa0))|{0x59,0x5a,0x5b,0x60,0x64,0x6d,0x77,0x6e,0x5d,0x5f,0x6c}
def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    events=json.loads((ROOT/'scripts/yuyuhakusho/lane-inputs.json').read_text())
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    watch=[*ENTRIES,0x15afc,0x18564,0x1fae6,0x1faee,0x19c44];pcs=(C.c_uint*len(watch))(*watch)
    with ReferenceSession(ROOT,out/'unobserved') as ref:
        expected=[ref.step(f,buttons_at(events,f))['reference']for f in range(1,3601)];reference_identity=ref.identity
    natural=[];isolated=[];sequences=[];pending={};errors=[];private=False;private_exit=0x15afc;sample=None;slots=[];landed=False
    def snapshot(ram,address):
        offset=address&0xffff;obj=ram[offset:offset+256]
        return {'body':body_state(obj),'actor_bytes':obj.hex(),'pool':pool_state(ram),'occupied':ram[0xa042]}
    def finish(record,ram,address):
        after=snapshot(ram,address);before=bytes.fromhex(record['before']['actor_bytes']);raw=bytes.fromhex(after['actor_bytes'])
        unknown=[i for i,(a,b)in enumerate(zip(before,raw))if a!=b and i not in ACTOR_WRITES];assert not unknown,unknown
        record['after']=after
        if not record['slots']:record['slots']=[-1,-1]
        assert len(record['slots'])==2
        return record
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal sample,landed
            try:
                address=ref.get_register(13)&0xffffff;ram=ref.ram_cpu()
                if private:
                    if pc==0x18564:landed=True
                    if pc in (0x1fae6,0x1faee):slots.append(slot_index(ref.get_register(8)))
                    if pc==private_exit:sample=ram;ref.cpu.cycles=200000
                    return
                if pc in ENTRIES:
                    if address in pending:raise ValueError('Overlapping lane-motion phase')
                    pending[address]={'frame':ref.frame,'kind':ENTRIES[pc],'before':snapshot(ram,address),'slots':[],'landed':False}
                elif address in pending:
                    row=pending[address]
                    if pc==0x18564:row['landed']=True
                    elif pc in (0x1fae6,0x1faee):row['slots'].append(slot_index(ref.get_register(8)))
                    elif pc==0x15afc:row['exit_frame']=ref.frame;natural.append(finish(pending.pop(address),ram,address))
            except Exception as error:errors.append(str(error));ref.cpu.cycles=200000 if private else ref.cpu.cycles
        assert bridge.yyh_set_observer(observe,pcs,len(watch))
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        observed=[]
        for f in range(1,3601):
            observed.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        assert observed==expected and not pending and not errors,(errors,len(pending))
        setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        def prepare(pool_kind):
            ref.restore(checkpoint,restore_refresh_cycles=True)
            for i in range(0x800):write(0xff9400+i,(i*37+13)&255)
            for i in range(16):write(0xff9404+i*0x80,0 if pool_kind=='empty' or pool_kind=='single' and i==0 else 0x80,2)
            write(0xffa036,0,2)
        def call(entry,exit_pc=0x15afc):
            nonlocal private,private_exit,sample,slots,landed
            private=True;private_exit=exit_pc;sample=None;slots=[];landed=False
            setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400)
            row={'before':snapshot(ref.ram_cpu(),0xff8400),'kind':ENTRIES.get(entry,0),'slots':[],'landed':False}
            setreg(16,entry);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert sample is not None and not errors,(entry,errors)
            row.update(after=snapshot(sample,0xff8400),slots=list(slots)or[-1,-1],landed=landed)
            return finish(row,sample,0xff8400) if entry in ENTRIES else row
        for kind,pose,pool_kind,depth in itertools.product([10,11],[0x0f,0x10,0],['empty','full','single'],[0xff,0x7fff,0xffff]):
            for y,vy in [(0x1300000,-0x20000),(0x140ffff,0),(0x7fffffff,0x7fffffff)]:
                prepare(pool_kind);write(0xff845d,kind);write(0xff845f,pose);write(0xff843e,depth,2);write(0xff8426,1)
                write(0xff8440,0x7fffffff,4);write(0xff8410,1,4);write(0xff8444,y,4);write(0xff8414,vy,4);write(0xff849c,0x140abcd,4);write(0xff8487,7);write(0xffa042,0x80)
                row=call(0x19e08 if kind==10 else 0x19ecc);row['case']={'pool':pool_kind,'depth':depth,'pose':pose};isolated.append(row)
        for ground,occupied,pool_kind in itertools.product([0x1400000,0x1300000],[0,3],['empty','full','single']):
            prepare(pool_kind);write(0xff849c,ground,4);write(0xff8444,ground,4);write(0xffa042,occupied);write(0xff8426,0x40 if ground==0x1400000 else 0xc0);write(0xff843e,0x100 if ground==0x1400000 else 0xd8,2)
            initial=call(0x186be,0x19c44);steps=[]
            for index in range(256):
                state=snapshot(ref.ram_cpu(),0xff8400);kind=state['body']['action_kind'];assert kind in (10,11),kind
                row=call(0x19e08 if kind==10 else 0x19ecc);steps.append(row)
                if row['landed']:break
            assert steps[-1]['landed'],'Trajectory did not land'
            sequences.append({'scope':'Isolated original sequence, no frame reset after constructor','initial':initial,'steps':steps,'pool_kind':pool_kind})
        identity=ref.identity
    for name,rows,scope in [('lane-motion-oracle',natural,'Natural lane-motion phases before shared tail'),('lane-motion-boundaries',isolated,'Isolated original CPU arithmetic and pool cases'),('lane-motion-sequences',sequences,'Single initial state, isolated original lane trajectories')]:
        raw=(json.dumps(rows,separators=(',',':'))+'\n').encode();z=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(z)
        report={'scope':scope,'identity':identity,'reference_identity':reference_identity,'inputs_sha256':sha((ROOT/'scripts/yuyuhakusho/lane-inputs.json').read_bytes()),'capture_sha256':sha(Path(__file__).read_bytes()),'records':len(rows),'fixture_sha256':sha(raw),'gzip_sha256':sha(z),'frames_match_original_core':observed==expected,'incomplete_calls':0,'errors':errors,'pass':bool(rows)and not errors}
        if name!='lane-motion-sequences':report['landings']=sum(r['landed']for r in rows);report['kind_counts']=dict(Counter(str(r['kind'])for r in rows))
        else:report['step_counts']=[len(r['steps'])for r in rows]
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
