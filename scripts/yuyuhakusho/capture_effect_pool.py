#!/usr/bin/env python3
"""Original MD effect-pool evidence: passive pairs and explicit isolated cases."""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

POOL=0x9400
SIZE=0x800
CURSOR=0xa036
ACTOR=0x8400


def number(data,offset,size,signed=False):
    return int.from_bytes(data[offset:offset+size],'big',signed=signed)


def pool_state(ram):
    return {'cursor':number(ram,CURSOR,2),'bytes':ram[POOL:POOL+SIZE].hex()}


def source_state(ram,offset=ACTOR):
    return {'x':number(ram,offset+0x40,4,True),'y':number(ram,offset+0x44,4,True),
            'attributes':number(ram,offset+0x24,2),'layer':ram[offset+0x26],
            'depth':number(ram,offset+0x3e,2)}


def slot_index(address):
    if not address:return -1
    offset=(address&0xffffff)-0xff9400
    if not 0<=offset<SIZE or offset%0x80:raise ValueError(f'Unexpected effect pointer {address:x}')
    return offset//0x80


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args()
    out=args.out.resolve();assert out.is_relative_to(ROOT)
    out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge=C.CDLL(str(bridge_path))
    cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t]
    bridge.yyh_set_observer.restype=C.c_int
    pcs=(C.c_uint*4)(0x1fce8,0x1fd80,0x1fae6,0x1faee)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    baseline=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural=[];isolated=[];errors=[];pending=None;mode='natural';sample=None;returns=[]
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal pending,sample,returns
            try:
                ram=ref.ram_cpu()
                if mode=='natural':
                    if pc==0x1fce8:
                        if pending is not None:raise ValueError('Nested dust-pair entry')
                        address=ref.get_register(13)&0xffff
                        pending=({'scope':'natural','kind':'pair','frame':ref.frame,
                                  'actor':address,'source':source_state(ram,address),'before':pool_state(ram)},
                                 ram[address:address+256])
                        returns=[]
                    elif pc in (0x1fae6,0x1faee) and pending is not None:
                        returns.append(slot_index(ref.get_register(8)))
                    elif pc==0x1fd80:
                        if pending is None:raise ValueError('Pair exit without entry')
                        row,actor_before=pending;pending=None
                        if ram[row['actor']:row['actor']+256]!=actor_before:raise ValueError('Pair modified source actor')
                        if len(returns)!=2:raise ValueError(f'Pair allocation count {len(returns)}')
                        row.update(after=pool_state(ram),returned_slots=list(returns),exit_frame=ref.frame)
                        natural.append(row)
                else:
                    if pc in (0x1fae6,0x1faee):returns.append(slot_index(ref.get_register(8)))
                    if (mode=='allocate' and pc in (0x1fae6,0x1faee)) or (mode=='pair' and pc==0x1fd80):
                        if sample is not None:raise ValueError('Isolated routine reached multiple exits')
                        sample=(ram,ref.get_register(8),pc)
                        # Observe before the final RTS, then stop the CPU after it.
                        ref.cpu.cycles=200000
            except Exception as exc:errors.append(str(exc))
        assert bridge.yyh_set_observer(observe,pcs,len(pcs))
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
        frames=[]
        for frame in range(1,3601):
            frames.append(ref.step(frame,buttons_at(events,frame))['reference'])
            if frame==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        assert frames==baseline,'Passive observer changed original frames'
        assert pending is None and not errors and natural,(pending,errors,len(natural))
        setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
        execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size):
            data=(value&((1<<(8*size))-1)).to_bytes(size,'big')
            for i,b in enumerate(data):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        def write_bytes(address,data):
            for i,b in enumerate(data):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        source_cases=[
            {'x':0x00641234,'y':0x01405678,'attributes':0xffff,'layer':2,'depth':0x1234},
            {'x':0x7fffabcd,'y':-0x80000000,'attributes':0x4855,'layer':0,'depth':0xffff},
            {'x':-0x80000000,'y':-1,'attributes':0,'layer':255,'depth':0},
        ]
        occupancy=[('empty',set()),('full',set(range(16))),('only_current_free',set(range(1,16))),
                   ('only_next_free',set(range(16))-{1}),('only_last_scanned_free',set(range(16))-{15}),
                   ('alternating',set(range(0,16,2)))]
        for mode_value in ('allocate','pair'):
            mode=mode_value
            for cursor in (0,0x780,0xff80):
                for name,busy in occupancy:
                    for source_index,source in enumerate(source_cases):
                        current=(cursor&0x7ff)//0x80
                        if name=='only_current_free':busy=set(range(16))-{current}
                        elif name=='only_next_free':busy=set(range(16))-{(current+1)%16}
                        elif name=='only_last_scanned_free':busy=set(range(16))-{(current+15)%16}
                        ref.restore(checkpoint,restore_refresh_cycles=True)
                        setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4)
                        setreg(13,0xff8400)
                        for reg in range(8):setreg(reg,0)
                        raw=bytearray(((i*73+source_index*29+17)&255)for i in range(SIZE))
                        for slot in range(16):raw[slot*0x80+4:slot*0x80+6]=([0x80,0x100,0xffff,1][(slot+source_index)%4] if slot in busy else 0).to_bytes(2,'big')
                        write_bytes(0xff9400,raw);write(0xffa036,cursor,2)
                        for key,off,size in [('x',0x40,4),('y',0x44,4),('attributes',0x24,2),('layer',0x26,1),('depth',0x3e,2)]:write(0xff8400+off,source[key],size)
                        before=ref.ram_cpu();sample=None;returns=[]
                        setreg(16,0x1fa96 if mode=='allocate' else 0x1fce8)
                        ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
                        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
                        execute(100000)
                        ref.api('set_cpu_hook',None,C.c_void_p)(None)
                        assert sample is not None and not errors,(name,errors)
                        after,a0,pc=sample
                        assert len(returns)==(1 if mode=='allocate' else 2)
                        assert before[ACTOR:ACTOR+256]==after[ACTOR:ACTOR+256]
                        changed=[i for i,(a,b) in enumerate(zip(before,after)) if a!=b]
                        unexpected=[i for i in changed if not POOL<=i<POOL+SIZE and i not in (CURSOR,CURSOR+1) and not 0xffc0<=i<0xffe4]
                        assert not unexpected,('Isolated unexpected RAM writes',unexpected)
                        isolated.append({'scope':'isolated-original-cpu','kind':mode,'name':f'{name}/cursor-{cursor:x}/source-{source_index}',
                                         'source':source_state(before),'before':pool_state(before),'after':pool_state(after),
                                         'returned_slots':list(returns),'exit':hex(pc)})
        identity=ref.identity
    records=natural+isolated
    payload=(json.dumps(records,separators=(',',':'))+'\n').encode()
    compressed=gzip.compress(payload,mtime=0)
    (out/'effects-oracle.json.gz').write_bytes(compressed)
    report={'scope':'Effect allocation and paired dust initialization only','identity':identity,
            'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),
            'fixture_sha256':sha(payload),'gzip_sha256':sha(compressed),'frames':3600,
            'frames_match_original_core':frames==baseline,'natural_pair_calls':len(natural),
            'isolated_original_calls':len(isolated),'complete_calls':len(records),'errors':errors,
            'incomplete_calls':0,'pass':True,'limits':['Not effect animation or rendering','Isolated cases deliberately replace private emulator inputs','Semantic fields are checked against every byte of each original pool result']}
    (out/'report.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))


if __name__=='__main__':main()
