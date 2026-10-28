#!/usr/bin/env python3
"""Capture complete original jump/landing constructors and pre-tail air actions."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
from capture_ground_actions import state as ground_state
from capture_effect_pool import pool_state,slot_index

ENTRY={0x18580:'jump',0x18564:'landing',0x19dc6:'air'}
JUMP_EXITS={0x185c2,0x185dc,0x185fa,0x1860a}
BASE={0x59,0x5a,0x5b,0x60,0x64,0x6d,0x77,0x6e,0x5d,0x5f,0x6c}


def integer(data,offset,size,signed=False):return int.from_bytes(data[offset:offset+size],'big',signed=signed)


def body_state(obj):
    return ground_state(obj)|{'x':integer(obj,0x40,4,True),'y':integer(obj,0x44,4,True),
           'velocity_x':integer(obj,0x10,4,True),'gravity':integer(obj,0x4c,4,True),'ground':integer(obj,0x9c,4,True),
           'extra_jump':obj[0x87],'attributes':integer(obj,0x24,2),'layer':obj[0x26],'depth':integer(obj,0x3e,2)}


def allowed(kind):
    if kind=='jump':return BASE|set(range(0x10,0x18))|set(range(0x4c,0x50))
    if kind=='landing':return BASE
    return BASE|set(range(0x40,0x48))|set(range(0x14,0x18))|{0x87}


def main():
    ap=argparse.ArgumentParser();ap.add_argument('--out',type=Path,required=True);args=ap.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    watch=list(ENTRY)+list(JUMP_EXITS)+[0x19c44,0x15afc,0x1fae6,0x1faee]
    pcs=(C.c_uint*len(watch))(*watch)
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    baseline=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural=[];cases=[];pending={};errors=[];isolated=None;sample=None;returns=[];landing_observed=False
    def parameters(obj,ram):
        p=integer(obj,0x90,4)&0xffffff
        memory,offset=(rom,p) if p<len(rom) else (ram,p&0xffff)
        return {'vertical_velocity':integer(memory,offset+4,4,True),'gravity':integer(memory,offset+8,4,True),'horizontal_velocity':integer(memory,offset+12,4,True)}
    def row(kind,ram,address):
        off=address&0xffff;obj=ram[off:off+256]
        return {'kind':kind,'actor':address,'before':body_state(obj),'actor_before':obj.hex(),
                'pool_before':pool_state(ram),'parameters':parameters(obj,ram),'returned_slots':[],'landed':False}
    def finish(record,ram,address):
        off=address&0xffff;obj=ram[off:off+256];before=bytes.fromhex(record['actor_before'])
        unknown=[i for i,(a,b)in enumerate(zip(before,obj))if a!=b and i not in allowed(record['kind'])]
        if unknown:raise ValueError(f"Unknown {record['kind']} actor writes {unknown}")
        record.update(after=body_state(obj),actor_after=obj.hex(),pool_after=pool_state(ram))
        if record['returned_slots']:
            if len(record['returned_slots'])!=2:raise ValueError('Incorrect dust allocation count')
        else:record['returned_slots']=[-1,-1]
        return record
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal sample,returns,landing_observed
            try:
                address=ref.get_register(13)&0xffffff;ram=ref.ram_cpu()
                if isolated:
                    if isolated=='air' and pc==0x18564:landing_observed=True
                    if pc in (0x1fae6,0x1faee):returns.append(slot_index(ref.get_register(8)))
                    done=(isolated=='jump' and pc in JUMP_EXITS)or(isolated=='landing' and pc==0x19c44)or(isolated=='air' and pc==0x15afc)
                    if done:
                        if sample is not None:raise ValueError('Multiple isolated exits')
                        sample=(ram,pc);ref.cpu.cycles=200000
                    return
                if pc in ENTRY:
                    if pc==0x18564 and ('air',address) in pending:pending[('air',address)]['landed']=True
                    key=(ENTRY[pc],address)
                    if key in pending:raise ValueError('Overlapping original entry')
                    record=row(ENTRY[pc],ram,address);record.update(scope='natural',frame=ref.frame,entry=hex(pc))
                    pending[key]=record
                elif pc in (0x1fae6,0x1faee):
                    for (what,actor),record in pending.items():
                        if actor==address:record['returned_slots'].append(slot_index(ref.get_register(8)))
                else:
                    what='jump' if pc in JUMP_EXITS else 'landing' if pc==0x19c44 else 'air'
                    key=(what,address)
                    if key in pending:
                        record=pending.pop(key);record.update(exit=hex(pc),exit_frame=ref.frame)
                        natural.append(finish(record,ram,address))
            except Exception as exc:errors.append(str(exc))
        assert bridge.yyh_set_observer(observe,pcs,len(watch))
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        frames=[]
        for f in range(1,3601):
            frames.append(ref.step(f,buttons_at(events,f))['reference'])
            if f==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        assert frames==baseline and not pending and not errors,(len(pending),errors)
        setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        def call(kind,setup,pool_kind):
            nonlocal isolated,sample,returns,landing_observed
            isolated=kind;ref.restore(checkpoint,restore_refresh_cycles=True)
            setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400)
            for reg in range(8):setreg(reg,0)
            for i in range(256):write(0xff8400+i,(i*37+19)&255)
            for i in range(0x800):write(0xff9400+i,(i*73+17)&255)
            for i in range(16):write(0xff9404+i*0x80,0 if pool_kind=='empty' or pool_kind=='partial' and i==0 else 0x80,2)
            write(0xffa036,0,2)
            write(0xff8490,0xff7000,4)
            for off,val in [(4,-0x58000),(8,0x3600),(12,0x1c000)]:write(0xff7000+off,val,4)
            setup()
            before=ref.ram_cpu();record=row(kind,before,0xff8400)
            sample=None;returns=[];landing_observed=False
            entry={'jump':0x18580,'landing':0x18564,'air':0x19dc6}[kind]
            setreg(16,entry);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert sample is not None and not errors,(kind,errors)
            after,pc=sample;record.update(scope='isolated-original-cpu',entry=hex(entry),exit=hex(pc),returned_slots=list(returns),pool_kind=pool_kind,landed=landing_observed)
            changed=[i for i,(a,b)in enumerate(zip(before,after))if a!=b]
            unknown=[i for i in changed if not 0x8400<=i<0x8500 and not 0x9400<=i<0x9c00 and i not in (0xa036,0xa037) and not 0xffa0<=i<0xffe4]
            assert not unknown,('Unexpected isolated global writes',unknown)
            return finish(record,after,0xff8400)
        for pool_kind in ['empty','full','partial']:
            for direction in [0,1,5,9,255]:
                for facing in [0,1,128]:
                    for parameter_values in [(-0x58000,0x3600,0x1c000),(-0x7fffffff,0x7fffffff,-0x80000000)]:
                        def setup():
                            write(0xff8414,(direction<<24)|0x123456,4);write(0xff8461,facing)
                            write(0xff8410,0x1234abcd,4);write(0xff8440,0x7fff1234,4);write(0xff8444,0x140abcd,4)
                            for offset,value in zip([4,8,12],parameter_values):write(0xff7000+offset,value,4)
                        record=call('jump',setup,pool_kind);record.update(direction=direction,facing=facing);cases.append(record)
            for source_x,source_y in [(0x7fffabcd,0x01401234),(-0x80000000,-1)]:
                def setup():write(0xff8440,source_x,4);write(0xff8444,source_y,4)
                cases.append(call('landing',setup,pool_kind))
            for values in [(0,0,-0x10000,0x1000,0),(0,0xffff,0,0,1),(0,0xffff,-1,0,0),
                           (0,0x10000,0,0,0x10001),(0x7fffffff,0x7fffffff,0x7fffffff,1,0),
                           (0,-0x80000000,0,0,0x7fff0000),(0,0x7fff0000,0,0,-0x80000000)]:
                def setup():
                    for off,val in zip([0x40,0x44,0x14,0x4c,0x9c],values):write(0xff8400+off,val,4)
                    write(0xff8410,1,4);write(0xff8487,3);write(0xff845d,2);write(0xff845f,6)
                cases.append(call('air',setup,pool_kind))
        identity=ref.identity
    for name,records,scope in [('jump-oracle',natural,'Natural original calls; nested landing records overlap their air parent'),('jump-boundaries',cases,'Isolated original CPU calls with declared inputs')]:
        raw=(json.dumps(records,separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        report={'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'counts':dict(Counter(r['kind']for r in records)),
                'complete_calls':len(records),'fixture_sha256':sha(raw),'gzip_sha256':sha(compressed),'frames_match_original_core':frames==baseline,
                'errors':errors,'incomplete_calls':0,'pass':True,'limit':'Shared action tail 0x15afc and later is not executed by this recovered stage'}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
