#!/usr/bin/env python3
"""Original match overlay/shadow rules before sprite emission or after clear."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

ENTRIES = {0xfe78: 'pause-label', 0xfeb2: 'demo', 0xff74: 'training', 0x10088: 'round', 0x10116: 'bracket-round', 0x10172: 'bracket-opponent',
           0x101f2: 'intro', 0x10330: 'intro-part', 0x10436: 'draw', 0x104ac: 'win',
           0x10522: 'character', 0x10616: 'team', 0x106e8: 'partner', 0x14882: 'shadow'}
STAGES = {'pause-label': [0, 2], 'demo': range(0, 16, 2), 'training': [0, 2], 'round': [0, 2], 'bracket-round': [0, 2], 'bracket-opponent': [0, 2],
          'intro': range(0, 14, 2), 'intro-part': range(0, 14, 2), 'draw': range(0, 8, 2),
          'win': range(0, 8, 2), 'character': range(0, 12, 2), 'team': range(0, 12, 2), 'partner': range(0, 12, 2), 'shadow': [0, 2]}
FIGHTERS = [0x8400, 0x8800, 0x8c00, 0x9000]


def n(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram):
    return {'objects': ram[0x8100:0x8400].hex(), 'shadow_bytes': ram[0x9c00:0x9c80].hex(), 'owners': {str(a): ram[a:a+256].hex() for a in FIGHTERS},
            'registration': ram[0xa880:0xa900].hex(), 'used': n(ram, 0xa034, 2), 'queue': list(ram[0xf80a:0xf80e]),
            'pause': ram[0xf550], 'slow': ram[0xf55a], 'attributes': n(ram, 0xa03e, 2),
            'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True), 'shake': n(ram, 0xf566, 2, True),
            'winner': n(ram, 0xa074, 2, True), 'characters': list(ram[0xa100:0xa104]),
            'team_titles': [n(ram, 0xa084, 2), n(ram, 0xa086, 2)], 'mode': ram[0xa096], 'round': n(ram, 0xa06a, 2), 'blink': ram[0xf585],
            'held': list(ram[0xf780:0xf784]), 'extended': list(ram[0xf7c0:0xf7c4])}


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', type=Path, required=True); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    expected = json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/engine/testdata/match-lifecycle-reference-frames.json.gz').read_bytes()))
    assert len(expected) == 5500
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = sorted(set(ENTRIES)|{0x1580, 0x10200, 0x1053c, 0x10630, 0x10702, 0x14920, 0x3556, 0x37c56, 0x37cac,
                                0xfeaa,0xfeba,0xff94,0xffdc,0xfff6,0x100fc})
    pcs = (C.c_uint*len(points))(*points)
    natural, isolated, sequences, errors = [], [], [], []
    pending = None; private = False; label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                address = ref.get_register(13)&0xffff; sp = ref.get_register(15)
                if pc in ENTRIES:
                    assert pending is None, 'Nested match overlay callback'
                    pending = {'kind': ENTRIES[pc], 'object_id': address, 'frame': ref.frame, 'entry_sp': sp,
                               'before': snapshot(ref.ram_cpu()), 'cleared_bytes': 0, 'external_queue_clears': []}
                elif pc in [0x37c56, 0x37cac]:
                    if pending is not None:
                        a = ref.get_register(9)&0xffff
                        pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                elif pending is not None and pc == 0x3556 and address == pending['object_id']:
                    size = n(ref.ram_cpu(), address+4, 2)
                    cleared = ((size or 64)>>4)*16
                    assert 0 < cleared <= 64, ('Unexpected overlay clear', size)
                    pending['cleared_bytes'] = cleared
                elif pending is not None and address == pending['object_id'] and sp == pending['entry_sp'] and pc in [0x1580, 0x10200, 0x1053c, 0x10630, 0x10702, 0x14920,0xfeaa,0xfeba,0xff94,0xffdc,0xfff6,0x100fc]:
                    row = pending; pending = None
                    row.update(after=snapshot(ref.ram_cpu()), draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame)
                    if row['kind'] != 'training': assert row['before']['owners'] == row['after']['owners']
                    mode=row['before']['mode'];blink=row['before']['blink'];round_index=row['before']['round']
                    lookup=row['kind']=='round' and mode<128 and not(mode==1 and blink&0x60)
                    row['reference_only']=lookup and round_index>=11
                    assert row['before']['registration'] == row['after']['registration']
                    if private:
                        row['name'] = label; isolated.append(row); ref.cpu.cycles = 4000000
                    else: natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook,C.c_void_p)
        hook(observer); frames = []
        for f in range(1, 5501):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400: checkpoint = ref.save()
        hook(None)
        assert frames == expected and pending is None and not errors, (len(natural), errors)
        private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)
        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size,'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b
        def write_bytes(address, raw):
            for i,b in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b
        def setup(seed, kind, stage, pause=0, slow=0, timer=0, depth=0, data=0, counter=0, parameter=0, used=8, cursor=0, winner=4, character=10, team=1, mode=1, round_index=0, blink=0, held=0, extended=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15): setreg(reg, 0)
            write_bytes(0xff8100, bytes((i*73+seed*19+17)&255 for i in range(768)))
            write_bytes(0xff9c00, bytes((i*43+seed*29+71)&255 for i in range(128)))
            owner = 0x9c00 if kind == 'shadow' else 0x8140 if kind=='demo' else 0x81c0 if kind=='training' else 0x8200 if kind=='pause-label' else 0x8240 if kind in ['round','bracket-round'] else 0x8280 if kind=='bracket-opponent' else 0x8340 if kind in ['intro-part','character','team'] else 0x8380 if kind == 'partner' else 0x8300
            behavior = next(a for a,k in ENTRIES.items()if k == kind)
            for off,value,size in [(0,behavior,4),(4,64 if kind != 'shadow' else 128,2),(6,stage,2),(0x10,counter,2),(0x18,timer,2),(0x1a,data,2),(0x3c,parameter,2),(0x3e,depth,2),(0x50,cursor,2)]:
                write(0xff0000+owner+off,value,size)
            if kind == 'intro':
                write(0xff8340,0x10330 if seed%2 else 0,4); write(0xff8380,0x10330 if seed%3 else 0,4)
            for i,a in enumerate(FIGHTERS):
                write_bytes(0xff0000+a,bytes((j*31+seed*23+i*47)&255 for j in range(256)))
                write(0xff0000+a+0x3e,[0x100,0xd8,0x8000,0xffff][i],2)
                write(0xff0000+a+0x40,[0x7fffabcd,-0x80000000,-1,0x12345678][i],4)
                write(0xff0000+a+0x9c,[0x130abcd,0x1405678,-0x80000000,0x7fffffff][i],4)
                if kind=='training':
                    write(0xff0000+a,[0x12668,0,0x1313e,0x12668][(seed+i)%4],4)
                    write(0xff0000+a+0x78,[-1024,-1,0,223][(seed//4+i)%4],2)
            for i in range(64): write(0xffa880+i*2,FIGHTERS[i%4],2)
            write(0xffa034,used,2);write(0xfff550,pause);write(0xfff55a,slow)
            write(0xffa03e,[0,0x2000,0x6000,0xffff][seed%4],2)
            write(0xffa014,[0,32767,32768,65535][seed%4],2);write(0xffa016,[0,32768,65535,32767][seed%4],2)
            write(0xfff566,[0,1,32768,65535][seed%4],2);write(0xffa074,winner,2)
            write_bytes(0xffa100,bytes([character]*4));write(0xffa084,team,2);write(0xffa086,9-team,2)
            write(0xffa096,mode);write(0xffa06a,round_index,2);write(0xfff585,blink)
            write_bytes(0xfff780,bytes([held,0,0,0]));write_bytes(0xfff7c0,bytes([extended,0,0,0]))
            write_bytes(0xfff80a,bytes([0x80,0x81,0x82,0x83]if seed%2 else[0,0x80,0,0x81]))
            return owner
        def invoke(kind, owner, name):
            nonlocal label
            label = name;setreg(17,0x2700);setreg(15,0xffff00);write(0xffff00,0x3d8,4);setreg(13,0xffff0000|owner)
            setreg(16,next(a for a,k in ENTRIES.items()if k == kind));ref.cpu.cycles=0;ref.cpu.refresh_cycles=896
            before=ref.ram_cpu();count=len(isolated);hook(observer);execute(200000);hook(None)
            assert len(isolated)==count+1 and pending is None and not errors,(kind,name,errors)
            after=ref.ram_cpu();allowed=set(range(0x8100,0x8400))|set(range(0x9c00,0x9c80))|{0xa034,0xa035,0xa096,0xf55a}|set(range(0xf80a,0xf80e))|set(range(0xfed0,0xff04))
            if kind=='training':
                for fighter in FIGHTERS:allowed.update([fighter+0x78,fighter+0x79])
            bad=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed]
            assert not bad,(kind,name,bad)
            return isolated[-1]
        seed=0
        for kind,stages in STAGES.items():
            timers=[0,2,6,12,14]if kind in ['bracket-round','bracket-opponent']else[0,1,0x7fff,0x8000,0xffff]
            for stage,pause,slow,timer,depth,data in itertools.product(stages,[0,128],[0,1],timers,[0,32,224,256,32767,32768,65535],[0,0x100,0x109]):
                owner=setup(seed,kind,stage,pause,slow,timer,depth,data,counter=[0,23,31,32,65535][seed%5],parameter=0 if seed%2 else 2,
                            used=[0,2,8,128,65535][seed%5],cursor=[0,2,6,126,32766][seed%5])
                invoke(kind,owner,f'{kind}/{stage}/{pause}/{slow}/{timer}/{depth}/{data}');seed+=1
        for kind in ['character','team']:
            for winner,character,team in itertools.product([1,2,3,4],range(1,12),range(1,9)):
                owner=setup(seed,kind,4,winner=winner,character=character,team=team)
                invoke(kind,owner,f'lookup-{winner}-{character}-{team}');seed+=1
        for kind in ['intro','intro-part']:
            for size in [0,16,32,48,64]:
                owner=setup(seed,kind,12);write(0xff0000+owner+4,size,2)
                invoke(kind,owner,f'clear-{size}');seed+=1
        for mode,round_index,blink in itertools.product([0,1,2,3,127,128,255],range(12),[0,0x20,0x40,0x60,255]):
            owner=setup(seed,'round',0,mode=mode,round_index=round_index,blink=blink)
            invoke('round',owner,f'round-{mode}-{round_index}-{blink}');seed+=1
        for kind in ['bracket-round','bracket-opponent']:
            for timer in range(0,16,2):
                owner=setup(seed,kind,0,timer=timer);invoke(kind,owner,f'bracket-{timer}');seed+=1
        for stage,mode in itertools.product(range(0,16,2),[0,1,2,3,127,128,255]):
            owner=setup(seed,'demo',stage,mode=mode,data=[0,0x100,0xffff][seed%3],timer=[0,1,64,96,65535][seed%5])
            invoke('demo',owner,f'demo-mode-{stage}-{mode}');seed+=1
        for pause,held,extended in itertools.product([0,1,128,255],range(16),range(8)):
            owner=setup(seed,'training',0,pause=pause,held=held,extended=extended)
            invoke('training',owner,f'training-{pause}-{held}-{extended}');seed+=1
        for data,counter in itertools.product([0,1,9,11,0x109,0x10b],[0,31,32,65535]):
            owner=setup(seed,'character',10,data=data,counter=counter,parameter=2)
            invoke('character',owner,f'character-delayed-{data}-{counter}');seed+=1
        for pause,base_slot,extra_slot,held,extended in itertools.product([0,128],range(4),range(4),[0,1,2,4,8,15],[0,1,2,3,4,7]):
            owner=setup(seed,'training',0,pause=pause)
            write_bytes(0xfff780,bytes(held if i==base_slot else 0 for i in range(4)))
            write_bytes(0xfff7c0,bytes(extended if i==extra_slot else 0 for i in range(4)))
            invoke('training',owner,f'training-channels-{pause}-{base_slot}-{extra_slot}-{held}-{extended}');seed+=1
        boundaries=len(isolated)
        for kind in ENTRIES.values():
            owner=setup(0,kind,0,winner=4,character=11,team=1)
            calls=[]
            for step in range(400 if kind == 'intro' else 64 if kind == 'shadow' else 240):
                if kind=='demo'and step==3:write(0xffa096,2)
                if kind=='round':write(0xfff585,(step*17)&255)
                if kind=='training':
                    external={'pause':128 if step<32 or step>=48 else 0,'held':[15,0,0,0], 'extended':[1 if step<12 or step>=48 else 2,0,0,0]}
                    write(0xfff550,external['pause']);write_bytes(0xfff780,bytes(external['held']));write_bytes(0xfff7c0,bytes(external['extended']))
                # Intro child objects are external callbacks, so drive them by
                # their own actual CPU calls rather than injecting final flags.
                call=invoke(kind,owner,f'stream-{kind}/{step}');calls.append(call)
                if kind=='demo'and step==3:call['external_mode']=2
                if kind=='round':call['external_blink']=(step*17)&255
                if kind=='training':call['external_training']=external
                if kind == 'shadow': call['external_registration_used'] = 8
                if kind=='intro':
                    for child in [owner+64,owner+128]:
                        if n(ref.ram_cpu(),child,4)==0x10330:
                            calls.append(invoke('intro-part',child,f'stream-intro/child-{child:x}/{step}'))
                if call['cleared_bytes'] or kind not in ['intro','intro-part','shadow','demo'] and step>=140:break
                if kind=='shadow':
                    # Registration is the preceding actor round's external input.
                    write(0xffa034,8,2)
            sequences.append({'kind':kind,'initial':calls[0]['before'],'calls':calls})
        identity=ref.identity
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    tables={'character_parameters':[n(rom,0x10c3e+i*2,2)for i in range(11)],'character_sounds':[n(rom,0x10c56+i*2,2)for i in range(11)],
            'team_parameters':[n(rom,0x106c8+i*2,2)for i in range(8)],'team_sounds':[n(rom,0x106d8+i*2,2)for i in range(8)],
            'round_parameters':[n(rom,0x100fe+i*2,2)for i in range(12)],
            'bracket_round_x':[n(rom,0x10152+i*2,2)for i in range(8)],'bracket_round_parameters':[n(rom,0x10162+i*2,2)for i in range(8)],
            'bracket_opponent_attributes':[n(rom,0x101ae+i*2,2)for i in range(8)],'bracket_opponent_x':[n(rom,0x101be+i*2,2)for i in range(8)],
            'bracket_opponent_parameters':[n(rom,0x101ce+i*2,2)for i in range(8)]}
    for name,rows in [('match-overlays-oracle',natural),('match-overlays-boundaries',isolated[:boundaries]),('match-overlays-sequences',sequences)]:
        payload={'tables':tables,'calls':rows};raw=(json.dumps(payload,separators=(',',':'))+'\n').encode();zipped=gzip.compress(raw,mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        calls=[c for s in rows for c in s['calls']]if name.endswith('-sequences')else rows
        report={'scope':'Complete overlay/shadow rule callbacks before sprite emission or after internal clear','identity':identity,'records':len(rows),'calls':len(calls),
                'kinds':dict(Counter(r['kind']for r in calls)),'draws':sum(r['draw']for r in calls),'clear_calls':sum(bool(r['cleared_bytes'])for r in calls),
                'external_queue_calls':sum(bool(r['external_queue_clears'])for r in calls),'reference_only_calls':sum(r['reference_only']for r in calls),'frames_match_original_core':frames==expected,
                'capture_sha256':sha(Path(__file__).read_bytes()),'fixture_sha256':sha(raw),'gzip_sha256':sha(zipped),'errors':errors,'incomplete_calls':0,'pass':bool(rows)and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
