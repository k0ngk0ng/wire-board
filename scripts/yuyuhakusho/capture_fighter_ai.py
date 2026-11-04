#!/usr/bin/env python3
"""Complete all-character AI returns; streamed original CPU fixtures."""
import argparse
from collections import Counter, defaultdict
import ctypes as C
import gzip
import hashlib
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_ai import FIGHTERS, BEHAVIORS, ALLOWED, snapshot, next_random, decision_seeds, number

CHARACTERS = [0x1d32a, 0x1d4a4, 0x1d628, 0x1d790, 0x1d904, 0x1dab4, 0x1dc50, 0x1de48, 0x1e010, 0x1e1e4, 0x1e3cc]
LEAVES = [0x1d75c, 0x1d776, 0x1e562, 0x1e580, 0x1e592, 0x1e5be, 0x1e5d0, 0x1e5fc, 0x1e60e, 0x1e620,
          0x1e64c, 0x1e65e, 0x1e670, 0x1e682, 0x1e694, 0x1e6b2, 0x1e6ee, 0x1e70c, 0x1e72a,
          0x1e748, 0x1e766, 0x1e784, 0x1e7a2, 0x1e7c0, 0x1e7de, 0x1e7fc, 0x1e81a, 0x1e836]
DISTANCES = [[28,40,56,64,72,96,128,192], [32,48,56,60,64,88,96,128,192],
             [34,48,72,88,96,128,160,192], [32,48,56,72,88,128,192], [30,48,56,72,88,128],
             [32,72,88,128,192], [32,48,56,96,128,160], [32,48,64,96,128,192,256],
             [32,48,72,88,128], [32,48,64,96,128,192], [40,56,112,128]]


class Rows:
    def __init__(self, path):
        self.path = path; self.file = path.open('w'); self.count = 0; self.characters = Counter(); self.paths = Counter(); self.rng = Counter(); self.odd = []
    def add(self, row):
        self.file.write(json.dumps(row, separators=(',', ':'))+'\n'); self.count += 1
        self.characters[str(row['character_id'])] += 1; self.paths.update(row['paths']); self.rng[str(row['rng_calls'])] += 1
        self.odd.extend(row['odd_ccr'])
    def close(self): self.file.close()


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', required=True, type=Path); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = sorted(set([0x1cc8a,0x156ee,0x3d8,0x3982,0x1ceca,0x1cef2,0x1d55e,0x1d562]+BEHAVIORS+CHARACTERS+LEAVES))
    assert len(points) <= 64, len(points)
    pcs = (C.c_uint*len(points))(*points)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text()); expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated = Rows(out/'natural.jsonl'), Rows(out/'boundaries.jsonl')
    sequences = []; pending = None; private = False; stream_mode = False; last_row = None; label = ''; errors = []
    with ReferenceSession(ROOT,out,core=core) as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal pending,last_row
            try:
                if pc == 0x1cc8a:
                    assert pending is None, 'Nested AI entry'
                    address = ref.get_register(13)&0xffffff; ram = ref.ram_cpu(); cid = ram[(address&65535)+0x58]
                    assert cid < 11
                    pending = {'character_id':cid,'object_id':address&65535,'source_register':hex(ref.get_register(13)),
                               'frame':ref.frame,'entry_sp':ref.get_register(15),'before':snapshot(ram,address),'paths':[],'rng_calls':0,'odd_ccr':[]}
                elif pending is not None and pc == 0x3982:
                    pending['rng_calls'] += 1
                elif pending is not None and pc in [0x156ee,0x3d8]:
                    if pc != (0x3d8 if private else 0x156ee): return
                    assert ref.get_register(15) == pending['entry_sp']+4
                    ram = ref.ram_cpu(); pending.update(after=snapshot(ram,pending['object_id']),exit=hex(pc),exit_frame=ref.frame)
                    before,after = bytes.fromhex(pending['before']['actor']),bytes.fromhex(pending['after']['actor'])
                    assert not [i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in ALLOWED]
                    last_row = pending; pending = None
                    if private:
                        last_row['name'] = label
                        if not stream_mode: isolated.add(last_row)
                        ref.cpu.cycles = 4000000
                    else: natural.add(last_row)
                elif pending is not None:
                    pending['paths'].append(hex(pc))
                    if pc in [0x1d55e,0x1d562]:
                        pending['odd_ccr'].append({'pc':hex(pc),'d1':ref.get_register(1),'sr':ref.get_register(17)})
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe,pcs,len(points))
        hook = ref.api('set_cpu_hook',None,C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook,C.c_void_p); hook(observer); frames=[]
        for f in range(1,3601):
            frames.append(ref.step(f,buttons_at(events,f))['reference'])
            if f == 2400: checkpoint = ref.save()
        hook(None); assert frames == expected and pending is None and not errors
        private=True; setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=b
        def write_bytes(address,raw):
            for i,b in enumerate(raw):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=b
        def setup(serial,cid,source=0x9000,behavior=2,difficulty=6,cooldown=10,elapsed=9,facing=0,energy=100,dx=100,dy=0,air=0,seed=1,attack=0,projectile=0,power=0,eligible=15):
            ref.restore(checkpoint,restore_refresh_cycles=True)
            for reg in range(15):setreg(reg,0)
            write_bytes(0xff8400,bytes((i*73+serial*29+17)&255 for i in range(4096)))
            for i,owner in enumerate(FIGHTERS):
                own = owner == source
                for off,value,size in [(4,256 if own or eligible&(1<<i) else 0,2),(0x58,cid if own else 9,1),(0x6e,0 if own else 1,1),(0x78,223,2),
                                       (0x40,100 if own else 100+dx,2),(0x44,320+air if own else 320+air+dy,2),(0x9c,320,2),(0x5c,0x10 if attack else 0,1),(0x64,attack,1)]:write(0xff0000+owner+off,value,size)
                for off,value,size in [(0x40,100+projectile,2),(0x44,300 if projectile else 0,2),(0x9c,320,2),(0x64,attack,1),(0x65,power,1)]:write(0xff0000+owner+0x100+off,value,size)
            target=FIGHTERS[(FIGHTERS.index(source)+1)%4]
            for off,value,size in [(0x61,facing,1),(0x5f,[0,0xa4,0xa6][serial%3],1),(0xb8,energy,2),(0xce,behavior,1),
                                   (0xd0,(FIGHTERS.index(target)*2-2)&7,2),(0xd2,cooldown,2),(0xd4,elapsed,2),(0xd6,target,2)]:write(0xff0000+source+off,value,size)
            write(0xffff12,difficulty,2);write(0xffff18,seed,4);write(0xffa010,32,2);write(0xffa012,600,2)
            return source,target
        def invoke(name,source):
            nonlocal label,last_row
            label=name;last_row=None;setreg(17,0x2700);setreg(15,0xffff00);write(0xffff00,0x3d8,4);setreg(13,0xffff0000+source);setreg(16,0x1cc8a)
            ref.cpu.cycles=0;ref.cpu.refresh_cycles=896;before=ref.ram_cpu();hook(observer);execute(2000000);hook(None)
            assert last_row is not None and pending is None and not errors,(name,errors)
            after=ref.ram_cpu();allowed=set(range(source,source+256))|set(range(0xff18,0xff1c))|set(range(0xfed0,0xff04))
            assert not [i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed],name
            return last_row
        serial=0;seeds=decision_seeds()
        for cid,behavior,difficulty,facing,cooldown,energy in itertools.product(range(11),range(17),[0,2,4,6],[0,1,3],[0,1,2,32768],[32,100]):
            source=FIGHTERS[(serial//7)%4];setup(serial,cid,source,behavior,difficulty,cooldown,[65535,0,1,7,8,32767][serial%6],facing,energy,seed=[0,1,0x12345678,0xffffffff][serial%4])
            invoke(f'common-{cid}-{behavior}-{difficulty}-{facing}-{cooldown}-{energy}',source);serial+=1
        for cid in range(11):
            distances=sorted(set([-32768,-192,-1,0,32767]+[x+d for x in DISTANCES[cid] for d in [-1,0,1]]))
            for dx,seed in itertools.product(distances,seeds):
                source=FIGHTERS[serial%4];setup(serial,cid,source,dx=dx,seed=seed,facing=[0,1,3][serial%3])
                invoke(f'ground-{cid}-{dx}/seed-{seed}',source);serial+=1
            for air,dy,dx,seed in itertools.product([0,-200],[-1,-32,-56,-64,-72,-80,-88,-96,-128,-160,-32768,32,48,64,96,160],[16,32,48,64,80,128],seeds[:4]):
                source=FIGHTERS[serial%4];setup(serial,cid,source,dx=dx,dy=dy,air=air,seed=seed,facing=[0,1,3][serial%3])
                invoke(f'vertical-{cid}-{air}-{dy}-{dx}/seed-{seed}',source);serial+=1
        reaction=[]
        for seed in range(1,100000):
            s,r1=next_random(seed);_,r2=next_random(s)
            if r1<58399 and r2&15==0:reaction.append(seed)
            if len(reaction)==16:break
        for cid,attack,projectile,power,distance,seed in itertools.product(range(11),[4,8,12],[0,1,80,81],[0,47,48,255],[0,128,-32768],reaction[:2]):
            source=FIGHTERS[serial%4];setup(serial,cid,source,attack=attack,projectile=projectile,power=power,dx=distance,seed=seed)
            invoke(f'threat-{cid}-{attack}-{projectile}-{power}-{distance}/seed-{seed}',source);serial+=1
        for cid,cursor,eligible,variant in itertools.product(range(11),[0,2,4,6],range(8),['ordinary','same-team','negative-health']):
            source=0x9000;setup(serial,cid,source,eligible=eligible,seed=serial*7919+1)
            write(0xff9000+0xd0,cursor,2)
            if variant!='ordinary':
                for owner in FIGHTERS[:3]:write(0xff0000+owner+(0x6e if variant=='same-team' else 0x78),0 if variant=='same-team' else -1,1 if variant=='same-team' else 2)
            invoke(f'filter-{cid}-{cursor}-{eligible}-{variant}',source);serial+=1
        stream_mode=True
        for cid,facing in itertools.product(range(11),[0,1,3]):
            source,target=setup(serial,cid,behavior=16,cooldown=0,elapsed=65535,facing=facing,seed=serial*391+1)
            calls=[]
            for step in range(24):
                x=80+(step%9)*24;write(0xff0000+target+0x40,x,2)
                row=invoke(f'stream-{cid}-{facing}/{step}',source);row['external_target_x']=x;row['external_target_id']=target;calls.append(row)
            sequences.append({'character_id':cid,'initial':calls[0]['before'],'calls':calls});serial+=1
        identity=ref.identity
    natural.close();isolated.close()
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    tables={name:[number(rom,off+i*2,2)for i in range(4)]for name,off in [('period_scales',0x1cc82),('decision_thresholds',0x1cd60),('reaction_thresholds',0x1cd68),('lane_masks',0x1d294)]}
    for name,source,scope in [('fighter-ai-oracle',natural,'Natural complete source9 AI returns'),('fighter-ai-boundaries',isolated,'All11 declared-private complete AI returns'),('fighter-ai-sequences',sequences,'All11 single-initial-state AI streams')]:
        path=out/(name+'.json.gz');digest=hashlib.sha256();records=source.count if isinstance(source,Rows)else len(source)
        with path.open('wb')as f:
            with gzip.GzipFile(fileobj=f,mode='wb',mtime=0)as z:
                def emit(b):digest.update(b);z.write(b)
                emit((json.dumps({'tables':tables,'character_entries':CHARACTERS},separators=(',',':'))[:-1]+',"calls":[').encode())
                if isinstance(source,Rows):
                    with source.path.open('rb')as rows:
                        for i,line in enumerate(rows):
                            if i:emit(b',')
                            emit(line.rstrip(b'\n'))
                else:
                    for i,row in enumerate(source):
                        if i:emit(b',')
                        emit(json.dumps(row,separators=(',',':')).encode())
                emit(b']}\n')
        report={'scope':scope,'identity':identity,'records':records,'calls':records if isinstance(source,Rows)else sum(len(s['calls'])for s in source),
                'capture_sha256':sha(Path(__file__).read_bytes()),'capture_dependencies':[{'path':'scripts/yuyuhakusho/capture_chu_ai.py','sha256':sha((ROOT/'scripts/yuyuhakusho/capture_chu_ai.py').read_bytes())}],
                'frames_match_original_core':frames==expected,'fixture_sha256':digest.hexdigest(),'gzip_sha256':sha(path.read_bytes()),'errors':errors,'incomplete_calls':0,'pass':bool(records)and not errors,
                'character_counts':dict(source.characters)if isinstance(source,Rows)else dict(Counter(str(s['character_id'])for s in source)),
                'paths':dict(source.paths)if isinstance(source,Rows)else{},'rng_calls':dict(source.rng)if isinstance(source,Rows)else{},
                'odd_source1_ccr':{'observations':len(source.odd),'all_d1_high_zero':all(e['d1']>>16==0 for e in source.odd),'all_nonnegative_no_overflow':all(e['sr']&10==0 for e in source.odd)}if isinstance(source,Rows)else{},
                'limits':['Natural source9 only; allothercharacters explicitly isolated','AI returns are not attacks/matches/platformtakeover','Canonical sign-extended source A5 follows actual original scheduler']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:v for k,v in report.items()if k!='paths'},indent=2))


if __name__=='__main__':main()
