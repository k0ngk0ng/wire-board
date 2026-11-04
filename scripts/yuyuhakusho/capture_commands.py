#!/usr/bin/env python3
"""Original generic command recognizers and Chu's ordered eight-slot preparation."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha

BINDINGS=[(6,'direction',0x1eaa4),(7,'direction',0x1eaa9),(0,'buttons',0x1eaf2),(1,'buttons',0x1eaeb),(2,'direction',0x1eab7),(3,'buttons',0x1eb00),(4,'direction',0x1ead9),(5,'direction',0x1eacf)]
ENTRIES={0x1e8d0:'direction',0x1e972:'buttons',0x17d80:'chu'}
EXITS={0x1e92a:'direction',0x1e9ac:'buttons',0x17e10:'chu'}
def state(obj):
    return {'raw_held':obj[0x74],'facing':obj[0x61],'request':obj[0x6b],'timeouts':list(obj[0xa8:0xb0]),'indices':list(obj[0xb0:0xb8])}

def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=list(ENTRIES)+list(EXITS);pcs=(C.c_uint*len(points))(*points)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    library={}
    def program(address,ram):
        address&=0xffffff
        source,offset=(rom,address) if address<len(rom) else (ram,address&0xffff)
        for length in range(1,257):
            if source[offset+length]==255:
                return {'window':source[offset],'steps':list(source[offset+1:offset+length+1])}
        raise ValueError('No terminator in declared command budget')
    for slot,kind,address in BINDINGS:library[str(address)]=program(address,b'')
    rows=[];pending={};errors=[];private=False;sample=None
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal sample
            try:
                address=ref.get_register(13)&0xffffff;offset=address&0xffff;sp=ref.get_register(15);ram=ref.ram_cpu();obj=ram[offset:offset+256]
                if private:
                    if pc in EXITS and sp==0xffffe0:sample=(ram,pc);ref.cpu.cycles=200000
                    return
                label=ENTRIES.get(pc) or EXITS[pc];key=(label,address,sp)
                if pc in ENTRIES:
                    if key in pending:raise ValueError('Overlapping command call')
                    record={'kind':label,'frame':ref.frame,'actor':address,'return_stack':sp,'before':state(obj),'actor_before':obj.hex()}
                    if label!='chu':
                        slot=ref.get_register(9);ptr=ref.get_register(10)&0xffffff
                        if slot>=8:raise ValueError('Command slot outside known eight-slot array')
                        data=program(ptr,ram)
                        if str(ptr) in library and library[str(ptr)]!=data:raise ValueError('Immutable command data changed')
                        library[str(ptr)]=data;record.update(slot=slot,program=ptr)
                    pending[key]=record
                else:
                    record=pending.pop(key)
                    allowed={0x6b}|(set(range(0xa8,0xb8)) if label=='chu' else {0xa8+record['slot'],0xb0+record['slot']})
                    unknown=[i for i,(a,b)in enumerate(zip(bytes.fromhex(record['actor_before']),obj))if a!=b and i not in allowed];assert not unknown,unknown
                    record.update(after=state(obj),actor_after=obj.hex(),exit_frame=ref.frame);rows.append(record)
            except Exception as exc:errors.append(str(exc))
        assert bridge.yyh_set_observer(observe,pcs,len(points))
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));frames=[]
        for frame in range(1,3601):
            frames.append(ref.step(frame,buttons_at(events,frame))['reference'])
            if frame==2400:checkpoint=ref.save()
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        assert frames==expected and not pending and not errors,(errors[:8],len(pending))
        private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
        def prepare():
            ref.restore(checkpoint,restore_refresh_cycles=True)
        def invoke(label,slot=0,address=0):
            nonlocal sample
            setreg(17,0x2700);setreg(15,0xffffe0);write(0xffffe0,0x3d8,4);setreg(13,0xff8400);setreg(9,slot);setreg(10,address)
            before=ref.ram_cpu()[0x8400:0x8500];sample=None;entry={'direction':0x1e8d0,'buttons':0x1e972,'chu':0x17d80}[label]
            setreg(16,entry);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.yyh_cpu_hook,C.c_void_p));execute(100000);ref.api('set_cpu_hook',None,C.c_void_p)(None)
            assert sample is not None and not errors,(label,errors[:4])
            ram,pc=sample;after=ram[0x8400:0x8500]
            allowed={0x6b}|(set(range(0xa8,0xb8)) if label=='chu' else {0xa8+slot,0xb0+slot})
            unknown=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed];assert not unknown,unknown
            return {'kind':label,'slot':slot,'program':address,'before':state(before),'after':state(after),'actor_before':before.hex(),'actor_after':after.hex(),'exit':hex(pc)}
        boundary=[]
        for slot,label,ptr in BINDINGS:
            data=library[str(ptr)]
            for index,timeout,facing,raw,request in itertools.product(range(len(data['steps'])),[0,data['window']-1,255],[0,1,128],[0,2,4,8,16,64,80,128,255],[0,slot+1,99]):
                prepare()
                for i in range(8):write(0xff84a8+i,17+i);write(0xff84b0+i,0)
                write(0xff84a8+slot,timeout);write(0xff84b0+slot,index);write(0xff8461,facing);write(0xff8474,raw);write(0xff846b,request)
                boundary.append(invoke(label,slot,ptr))
        # Additional zero-window and byte-index wrap cases use explicit private
        # data, never replace the production Chu sequences.
        for label,window,timeout,index,matching in itertools.product(['direction','buttons'],[0,255],[0,254,255],[0,254,255],[False,True]):
            prepare();data={'window':window,'steps':[8 if label=='direction' else 16]*255+[255]}
            write(0xff7000,window)
            for i,b in enumerate(data['steps']):write(0xff7001+i,b)
            for i in range(8):write(0xff84a8+i,0);write(0xff84b0+i,0)
            write(0xff84a8,timeout);write(0xff84b0,index);write(0xff8461,0);write(0xff8474,(8 if label=='direction' else 16) if matching else 0);write(0xff846b,1)
            record=invoke(label,0,0xff7000);record['data']=data;boundary.append(record)
        # Multiple completed slots in one preparation call establish the real
        # 6,7,0,1,2,3,4,5 priority, rather than assuming numeric slot order.
        for mask,facing,raw in itertools.product([0,1,2,4,8,16,32,64,128,63,192,255],[0,1],[0,80]):
            prepare()
            for slot,label,ptr in BINDINGS:
                write(0xff84a8+slot,library[str(ptr)]['window']-1)
                write(0xff84b0+slot,len(library[str(ptr)]['steps'])-1 if mask&(1<<slot) else 0)
            write(0xff8461,facing);write(0xff8474,raw);write(0xff846b,8)
            record=invoke('chu');record['completed_slot_mask']=mask;boundary.append(record)
        sequences=[]
        streams=[[8,0,8,0,4,0,4,0,2,0,2,0,2,10,8,0,2,8,9,0],
                 [16,0,16,0,16,0,64,0,64,0,64,0,0,80,0],
                 [4,0,4,0,8,0,8,0,2,0,2,0,2,6,4,0,2,4,5,0],
                 [0]*20+[8]*30+[0]*20+[255]*20]
        for facing,stream in itertools.product([0,1,128],streams):
            prepare()
            for i in range(8):write(0xff84a8+i,0);write(0xff84b0+i,0)
            write(0xff8461,facing);write(0xff846b,0)
            initial=state(ref.ram_cpu()[0x8400:0x8500]);records=[]
            for raw in stream:
                write(0xff8474,raw);record=invoke('chu');record['raw_input']=raw;records.append(record)
            sequences.append({'scope':'One initial command state; only RawHeld changes between calls','initial':initial,'calls':records})
        identity=ref.identity
    for name,calls,scope in [('commands-oracle',rows,'Natural complete command helpers and overlapping Chu preparation'),('commands-boundaries',boundary,'Isolated original CPU command matrix'),('commands-sequences',sequences,'Single initial state original Chu command streams')]:
        payload={'library':library,'bindings':[{'slot':slot,'kind':kind,'program':ptr}for slot,kind,ptr in BINDINGS],'calls':calls}
        raw=(json.dumps(payload,separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(compressed)
        report={'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'records':len(calls),'library_entries':len(library),'fixture_sha256':sha(raw),'gzip_sha256':sha(compressed),'frames_match_original_core':frames==expected,'errors':errors,'incomplete_calls':0,'pass':bool(calls)and not errors}
        if name!='commands-sequences':report['kind_counts']=dict(Counter(r['kind']for r in calls));report['nonzero_requests_after']=sum(r['after']['request']!=0 for r in calls)
        else:report['step_counts']=[len(r['calls'])for r in calls]
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
