#!/usr/bin/env python3
"""Original battle-manager0/2 rules plus observed presentation requests."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

OWNERS = [0x8400, 0x8800, 0x8c00, 0x9000]
OBJECTS = [0x8100, 0x8140, 0x8180, 0x81c0, 0x8200, 0x8240, 0x8280, 0x9c00, 0x9c80, 0x9d00]


def n(raw, off, size=2, signed=False): return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram):
    return {'world': ram[0x8040:0xa000].hex(), 'globals': ram[0xa000:0xa200].hex(), 'colors': ram[0xe080:0xe100].hex(),
            'system': ram[0xf500:0xf5b2].hex(), 'registers': ram[0xf6d0:0xf6f6].hex(), 'queue': list(ram[0xf80a:0xf80e]),
            'options': list(ram[0xff14:0xff17]), 'reset_words': [n(ram, a) for a in [0xe800, 0xea00, 0xe500, 0xe528]]}


def extract_library(rom):
    w = lambda a: n(rom, a)
    palettes = []
    for cid in range(11):
        base = n(rom, 0xfcb8+cid*4, 4)
        palettes.append([[w(base+color*32+12+i*2) for i in range(10)] for color in range(4)])
    names = []
    for cid in range(11):
        base = n(rom, 0x109ca+cid*4, 4)
        assert w(base+6) == 0xffff
        names.append([w(base+i*2)for i in range(3)])
    return {'initial_colors': [w(0x27ff68+i*2)for i in range(64)], 'arena_resources': [n(rom, 0x21700+i*4, 4)for i in range(8)],
            'arena_behaviors': [n(rom, 0xf94c+i*4, 4)for i in range(8)], 'palettes': palettes,
            'shared_palettes': [[w(0x283cc0+c*32+12+i*2)for i in range(10)]for c in range(8)],
            'hud_offsets': [[w(0xfd06+mode*8+i*2)for i in range(4)]for mode in range(1, 5)],
            'round_opponents': [n(rom, 0x10050+i*2, 2, True)for i in range(28)], 'names': names,
            'controller_labels': [[w(0xfd5c+c*6), w(0xfd5e+c*6)]for c in range(5)],
            'health_template': [w(0xfd86+i*2)for i in range(9)], 'energy_template': [w(0xfdaa+i*2)for i in range(9)]}


def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib';bridge=C.CDLL(str(bridge_path));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
    points=[0xf2a4,0xf2b2,0xf316,0xf3f4,0x3436,0x800,0xfe68,0xfdbe,0x20dc]
    pcs=(C.c_uint*len(points))(*points);rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();library=extract_library(rom)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural=[];isolated=[];streams=[];pending=None;errors=[];private=False;label=''
    with ReferenceSession(ROOT,out,core=core)as ref:
        @cbtype
        def observe(kind,width,pc,value):
            nonlocal pending
            try:
                ram=ref.ram_cpu();sp=ref.get_register(15)
                if pc==0xf2a4:
                    stage=n(ram,0x8046)
                    if stage not in [0,2]:return
                    assert pending is None
                    pending={'stage':stage,'frame':ref.frame,'before':snapshot(ram),'entry_sp':sp,'events':[]}
                elif pending is not None and pc in [0xf2b2,0xf316,0xf3f4]and sp==pending['entry_sp']:
                    pending.update(after=snapshot(ram),exit=hex(pc));(isolated if private else natural).append(pending)
                    if private:pending['name']=label;ref.cpu.cycles=8000000
                    pending=None
                elif pending is not None:
                    if pc==0x3436:pending['events'].append({'kind':'reset'})
                    elif pc==0x800:pending['events'].append({'kind':'resource','source':ref.get_register(8)&0xffffff})
                    elif pc==0xfe68:
                        source=ref.get_register(9)&0xffffff;words=[]
                        while n(rom,source)!=0xffff:
                            words.append(n(rom,source));source+=2;assert len(words)<=16
                        pending['events'].append({'kind':'words','offset':ref.get_register(0)&65535,'words':words})
                    elif pc==0xfdbe:
                        owner=ref.get_register(8)&0xffff;cid=ram[owner+0x58]
                        pending['events'].append({'kind':'name','object_id':owner,'offset':n(ram,owner+0x8a),'glyphs':library['names'][cid]})
                    elif pc==0x20dc:
                        owner=ref.get_register(8)&0xffff
                        pending['events'].append({'kind':'portrait','object_id':owner,'attributes':n(ram,0x8064),'x':n(ram,0x8048),'y':n(ram,0x804c),'parameter':n(ram,0x807c)})
            except Exception as e:
                errors.append(str(e))
                if private:ref.cpu.cycles=8000000
        assert bridge.yyh_set_observer(observe,pcs,len(points));hook=ref.api('set_cpu_hook',None,C.c_void_p);observer=C.cast(bridge.yyh_cpu_hook,C.c_void_p);hook(observer);frames=[]
        for frame in range(1,3601):
            frames.append(ref.step(frame,buttons_at(events,frame))['reference'])
            if frame==2400:checkpoint=ref.save()
        hook(None);assert frames==expected and pending is None and not errors,(errors,pending)
        private=True;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);execute=ref.api('m68k_run',None,C.c_uint)
        def write(address,value,size=1):
            for i,b in enumerate((value&((1<<(8*size))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=b
        def write_bytes(address,data):
            for i,b in enumerate(data):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=b
        def setup(seed,stage=2,arena=0,layout=2,mask=9,controllers=(1,0,0,0),chars=(10,10,10,10),colors=(0,1,2,3),rule=2,pause=0,mode=0,busy=0):
            ref.restore(checkpoint,restore_refresh_cycles=True)
            for reg in range(15):setreg(reg,0)
            write_bytes(0xff8040,bytes((i*73+seed*11+17)&255 for i in range(0x1fc0)))
            write(0xff8040,0xf2a4,4);write(0xff8044,64,2);write(0xff8046,stage,2)
            write(0xffa060,arena,2);write(0xffa062,rule,2);write(0xffa070,layout,2);write(0xffa072,0,2);write(0xffa07e,0,2)
            write(0xfff550,pause);write(0xffa096,mode);write(0xffa097,0);write(0xfff54e,busy)
            for i in range(3):write(0xffff14+i,0)
            write_bytes(0xffa100,bytes((chars[i]if mask&(1<<i)else 0)for i in range(4)))
            write_bytes(0xffa104,bytes(controllers));write_bytes(0xffa108,bytes(colors));write_bytes(0xffa10c,bytes([1,2,3,4]))
            write_bytes(0xfff80a,bytes([0x80,0x81,0x82,0x83]if seed%2 else [0,0x80,0,0x81]))
        def invoke(name):
            nonlocal label
            label=name;setreg(17,0x2700);setreg(15,0xffffff00);write(0xffff00,0x3d8,4);setreg(13,0xffff8040);setreg(16,0xf2a4);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            count=len(isolated);hook(observer);execute(6000000);hook(None)
            assert len(isolated)==count+1 and pending is None and not errors,(name,len(isolated),errors,pending is not None)
            return isolated[-1]
        for seed,(stage,arena,pause,mode,busy)in enumerate(itertools.product([0,2],range(0,16,2),[0,127,128],[0,1,2],[0,3])):
            setup(seed,stage,arena,pause=pause,mode=mode,busy=busy);invoke(f'entry-{stage}-{arena}-{pause}-{mode}-{busy}')
        for seed,(mask,layout,controls)in enumerate(itertools.product(range(16),range(1,5),[(0,0,0,0),(1,2,3,4),(4,3,0,1)])):
            setup(seed,mask=mask,layout=layout,arena=(seed%8)*2,controllers=controls,rule=[0,2,4][seed%3]);invoke(f'config-{mask}-{layout}-{controls}')
        for seed,(cid,slot,color)in enumerate(itertools.product(range(1,12),range(4),[0,1,3,4,11,127,128,131,132,255])):
            setup(seed,mask=1<<slot,chars=(cid,)*4,colors=(color,)*4,layout=1+seed%4,arena=(seed%8)*2,controllers=(seed%5,)*4);invoke(f'character-{cid}-{slot}-{color}')
        for seed,(round_index,pair)in enumerate(itertools.product(range(14),[0,2])):
            setup(seed,rule=4,mask=15);write(0xffa07e,round_index,2);write(0xffa072,pair,2);invoke(f'round-{round_index}-{pair}')
        for seed,options in enumerate(itertools.product([0,1,255],repeat=4)):
            setup(seed,mask=15,mode=seed%2,rule=seed%5)
            for i,v in enumerate(options[:3]):write(0xffff14+i,v)
            write(0xffa097,options[3]);invoke(f'options-{options}')
        for seed,(teams,mask,rule)in enumerate(itertools.product([(1,1,4,4),(1,1,1,1),(4,4,1,1)],[3,9,15],[2,4])):
            setup(seed,mask=mask,rule=rule);write_bytes(0xffa10c,bytes(teams));invoke(f'teams-{teams}-{mask}-{rule}')
        boundary_count=len(isolated)
        for scenario in range(4):
            setup(scenario,stage=0,mask=[0,3,9,15][scenario],layout=1+scenario,rule=[0,2,4,2][scenario],arena=scenario*2)
            calls=[]
            for step in range(5):
                if step==3:write(0xfff54e,0)
                call=invoke(f'stream-{scenario}/{step}');call['resource_complete']=step==3;calls.append(call)
                if n(ref.ram_cpu(),0x8046)==4:break
            assert n(ref.ram_cpu(),0x8046)==4
            streams.append({'initial':calls[0]['before'],'calls':calls})
        identity=ref.identity
    (out/'match-start-library.json').write_text(json.dumps(library,indent=2)+'\n')
    for name,calls in [('match-start-oracle',natural),('match-start-boundaries',isolated[:boundary_count]),('match-start-sequences',streams)]:
        raw=(json.dumps(calls,separators=(',',':'))+'\n').encode();zipped=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(zipped)
        rows=[c for s in calls for c in s['calls']]if name.endswith('-sequences')else calls
        report={'scope':'Full0/2 rules with explicit presentation-request boundary; hardware graphics buffers excluded','identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'library_sha256':sha((out/'match-start-library.json').read_bytes()),'records':len(calls),'calls':len(rows),'stage_counts':dict(Counter(str(c['stage'])for c in rows)),'events':dict(Counter(e['kind']for c in rows for e in c['events'])),'frames_match_original_core':frames==expected,'fixture_sha256':sha(raw),'gzip_sha256':sha(zipped),'incomplete_calls':0,'errors':errors,'pass':bool(calls)and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
