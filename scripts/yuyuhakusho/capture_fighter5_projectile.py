#!/usr/bin/env python3
"""Pinned original68000 source5 child, hit-stop slave and creator-clock evidence."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data

ENTRIES = {0x1414e: 'linear', 0x143e4: 'steered', 0x146ba: 'follower', 0x142b4: 'linear_stop', 0x1458a: 'steered_stop'}
AN_RETURNS = [0x1edaa, 0x21970, 0x21982, 0x21994, 0x219a6, 0x219aa]
CREATORS = {0x1b704: 'creator47', 0x1b7fa: 'creator48', 0x1b8f4: 'creator49', 0x1b9bc: 'creator50'}
RTS = [0x14328, 0x1435e, 0x14368, 0x14398, 0x143a2, 0x143d2, 0x143dc,
       0x145fe, 0x14634, 0x1463e, 0x1466e, 0x14678, 0x146a8, 0x146b2]


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def draw(ram, off):
    return {'ObjectID': off, 'GraphicsSet': n(ram, off+0x1c, 4), 'TileBank': n(ram, off+0x20, 4),
            'Parameter': n(ram, off+0x3c), 'ScreenX': n(ram, off+8), 'ScreenY': n(ram, off+12),
            'OffsetX': n(ram, off+0x28, 2, True), 'OffsetY': n(ram, off+0x2a, 2, True),
            'Attributes': n(ram, off+0x24), 'Depth': n(ram, off+0x3e), 'Layer': ram[off+0x26],
            'SkipTiles': ram[off+0x27], 'Cache': [n(ram, off+0x2c+i*4, 4) for i in range(4)]}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--baseline', nargs=3, action='append', required=True, metavar=('INPUTS', 'REFERENCE', 'MANIFEST'))
    p.add_argument('--checkpoint', type=int, default=2400)
    p.add_argument('--natural-only', action='store_true')
    p.add_argument('--prefix', default='fighter5-projectile')
    args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT) and args.prefix.replace('-', '').isalnum()
    out.mkdir(parents=True, exist_ok=True)
    rom_path = ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'; rom = rom_path.read_bytes()
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [*ENTRIES, *CREATORS, *RTS, 0x19c96, 0x19cc2, 0x15afc, 0x1580, 0x3556, 0x3574, 0x1eb40, *AN_RETURNS,
              0x37c56, 0x37cac, 0x3982, 0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    assert len(points) <= 64
    pcs = (C.c_uint*len(points))(*points)
    natural, isolated, streams, errors, baselines, library = [], [], [], [], [], {}
    pending = None; private = False; requested = ''; label = ''; scenario = ''; sequence = 0
    binding = None

    def snapshot(ram, parent=0x8400):
        obj = ram[parent:parent+256]; pointer = n(obj, 0x90, 4)&0xffffff
        memory, index = (rom, pointer) if pointer < len(rom) else (ram, pointer&65535)
        return {'object_id': parent, 'family': ram[parent:parent+1024].hex(),
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'buffers': ram[0xa200:0xa600].hex(), 'offsets': [n(ram, 0xa004+i*2) for i in range(4)],
                'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'parameters': {key: n(memory, index+delta, 4, True) for key, delta in [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]},
                'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': n(ram, 0xa03e),
                                'arena_left': n(ram, 0xa010, 2, True), 'arena_right': n(ram, 0xa012, 2, True),
                                'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True), 'shake': n(ram, 0xf566, 2, True)}}

    for source_index, names in enumerate(args.baseline):
        ip, rp, mp = [Path(name).resolve() for name in names]
        assert all(path.is_relative_to(ROOT) for path in (ip, rp, mp))
        events, expected, manifest = json.loads(ip.read_text()), json.loads(rp.read_bytes()), json.loads(mp.read_text())
        assert manifest['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
        assert manifest['input']['sha256'] == sha(ip.read_bytes()) and manifest['output']['sha256'] == sha(rp.read_bytes())
        scenario = str(ip.relative_to(ROOT))
        with ReferenceSession(ROOT, ROOT/'.local/md-yuyuhakusho'/('fighter5-projectile-observed-'+args.prefix+'-'+str(source_index)), core=core) as ref:
            def remember_frame(ram, address):
                key, data, origin = selected_data(rom, ram, ram[address:address+256]); identity = json.dumps(key, sort_keys=True)
                assert identity not in library or library[identity]['data'] == data
                library[identity] = {'key': key, 'data': data, 'source': origin}
                return key, data

            @cbtype
            def observe(kind, width, pc, value):
                nonlocal pending, sequence
                try:
                    raw_pc = pc; pc &= 0xffffff
                    if pc == 0xfff000:
                        if pending is not None: pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), '_ram': ref.ram_cpu()})
                        return
                    if pc in [0xfff09a, 0xfff0a8, 0xfff13a]:
                        if pending is not None and pending['_irq']:
                            irq = pending['_irq'].pop(); ram = ref.ram_cpu()
                            irq.update(exit=hex(pc), changes=[{'offset': i, 'before': a, 'after': b} for i, (a,b) in enumerate(zip(irq.pop('_ram'), ram)) if a != b])
                            pending['interrupts'].append(irq)
                        return
                    if pc in [0x37c56, 0x37cac]:
                        if pending is not None:
                            a = ref.get_register(9)&65535; pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                        return
                    if pc == 0x3982:
                        if pending is not None: pending['rng_calls'] += 1
                        return
                    address, sp = ref.get_register(13)&65535, ref.get_register(15); parent = address&0xfc00; ram = ref.ram_cpu()
                    role = ENTRIES.get(pc)
                    if private and pending is None:
                        role = CREATORS.get(pc, role)
                        if requested == 'clock' and pc == 0x19c96: role = 'clock'
                        if requested == 'record' and pc == 0x1eb40: role = 'record'
                    if role and ram[parent+0x58] == 5 and (private or ref.frame <= 3600):
                        assert pending is None, ('nested', role, pending['kind'])
                        sequence += 1
                        pending = {'kind': role, 'name': label, 'scenario': scenario, 'sequence': sequence, 'entry': hex(pc), 'frame': ref.frame,
                                   'object_id': address, 'source_register': hex(ref.get_register(13)), 'entry_sp': sp,
                                   'before': snapshot(ram, parent), 'draw': False, 'cleared_bytes': 0, 'external_queue_clears': [],
                                   'rng_calls': 0, 'interrupts': [], '_irq': [], '_ram': ram}
                        if role == 'record': remember_frame(ram, address)
                    elif pending is not None:
                        if pc == 0x1eb40: remember_frame(ram, address)
                        if pc == 0x3556:
                            pending['cleared_bytes'] = (n(ram, address+4) or 64)//16*16
                            assert 0 < pending['cleared_bytes'] <= 256
                        finish = ((pc == 0x1580 or pc in RTS) and pending['kind'] in ENTRIES.values() or
                                  pc == 0x3574 and pending['cleared_bytes'] or
                                  pc == 0x15afc and pending['kind'].startswith('creator') or
                                  pc == 0x19cc2 and pending['kind'] == 'clock' or
                                  pc in AN_RETURNS and pending['kind'] == 'record')
                        if finish:
                            row = pending; pending = None; assert sp == row['entry_sp'] and not row.pop('_irq')
                            row.update(after=snapshot(ram, parent), draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame,
                                       draw_request=draw(ram, address) if pc == 0x1580 else None)
                            original_ram = row.pop('_ram')
                            row['ram_changes'] = [{'offset': i, 'before': a, 'after': b} for i,(a,b) in enumerate(zip(original_ram, ram)) if a != b]
                            allowed = (set(range(parent,parent+1024)) | set(range(0x9400,0x9c00)) | set(range(0xa200,0xa600)) |
                                       set(range(0xa004,0xa00c)) | {0xa030,0xa036,0xa037} | set(range(0xf80a,0xf80e)) |
                                       {(row['entry_sp']-i)&65535 for i in range(0x180)})
                            row['external_changes'] = [r for r in row['ram_changes'] if r['offset'] not in allowed]
                            row['unexplained_changes'] = []
                            for delta in row['external_changes']:
                                current, steps = delta['before'], 0
                                for irq in row['interrupts']:
                                    for d in irq['changes']:
                                        if d['offset'] == delta['offset']:
                                            assert d['before'] == current; current, steps = d['after'], steps+1
                                if not steps or current != delta['after']: row['unexplained_changes'].append(delta)
                            assert not row['unexplained_changes'] and row['rng_calls'] == 0
                            assert row['before']['random_seed'] == row['after']['random_seed']
                            if private:
                                assert not row['external_changes'] and not row['external_queue_clears']; isolated.append(row); ref.cpu.cycles = 8000000
                            else: natural.append(row)
                except Exception as exc:
                    errors.append(repr(exc))
                    if private: ref.cpu.cycles = 8000000

            assert bridge.yyh_set_observer(observe, pcs, len(points))
            hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
            hook(observer); frames = []
            for frame in range(1, len(expected)+1):
                frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
                if frame == args.checkpoint: checkpoint, base_ram = ref.save(), ref.ram_cpu()
            hook(None); assert frames == expected and pending is None and not errors, (errors[:3], pending is not None)
            identity = ref.identity
            baselines.append({'input': str(ip.relative_to(ROOT)), 'input_sha256': sha(ip.read_bytes()), 'reference': str(rp.relative_to(ROOT)),
                              'reference_raw_sha256': sha(rp.read_bytes()), 'reference_manifest': str(mp.relative_to(ROOT)),
                              'reference_manifest_sha256': sha(mp.read_bytes()), 'video_steps': len(expected), 'all_ram_video_match': True})
            assert base_ram[0x8458] == 5
            if source_index != 0 or args.natural_only: continue
            binding = {'source_tag': 5, 'checkpoint_frame': args.checkpoint, 'source_actor_sha256': sha(base_ram[0x8400:0x8500]),
                       'animation_set': n(base_ram,0x8450,4), 'shape_set': n(base_ram,0x848c,4), 'movement_source': n(base_ram,0x8490,4)}
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, size=1): write_bytes(address, (value&((1<<(size*8))-1)).to_bytes(size,'big'))

            def setup(seed=0, behavior=0x1414e, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                write_bytes(0xff8400, base_ram[0x8400:0x8500]); write_bytes(0xff8500, bytes((i*73+seed*29+17)&255 for i in range(768)))
                write_bytes(0xff9400, bytes((i*37+seed*23+13)&255 for i in range(2048)))
                write_bytes(0xffa200, bytes((i*19+seed*7+5)&255 for i in range(1024)))
                address = 0x8600 if behavior == 0x146ba else 0x8500
                v = {'stage': 2, 'size':256, 'index':0, 'duration':8, 'ticks':0, 'flags':0, 'completed':0, 'pause':0, 'slow':0,
                     'facing':0, 'x':100<<16|0xabcd, 'y':260<<16|0x5678, 'vx':0, 'vy':0, 'ground':320<<16|0x1234,
                     'aux48':0x03abcdef, 'gravity':0x00123456, 'action':0, 'contact':0, 'pose':0x17 if behavior == 0x1414e else 0x18,
                     'owner':0x8400, 'parent_held':0, 'parent_pose':0xa3, 'stop':0, 'shake':0, 'pool_busy':False, 'table':0x1479c}
                v.update(changes)
                fields = {0:(behavior,4),4:(v['size'],2),6:(v['stage'],2),0x10:(v['vx'],4),0x14:(v['vy'],4),0x1c:(0xd8f1a,4),0x20:(0x240000,4),
                          0x50:(v['table'] if behavior == 0x146ba else 0xd92ca,4),0x8c:(0xd9a3a,4),0x59:(v['index'],1),0x5a:(v['duration'],1),
                          0x5b:(v['ticks'],1),0x5c:(v['flags'],1),0x60:(v['completed'],1),0x5d:(v['action'],1),0x5f:(v['pose'],1),0x61:(v['facing'],1),
                          0x40:(v['x'],4),0x44:(v['y'],4),0x48:(v['aux48'],4),0x4c:(v['gravity'],4),0x9c:(v['ground'],4),
                          0x89:(v['contact'],1),0xfe:(v['owner'],2),0x69:(v['stop'],1),0x6a:(v['shake'],1)}
                for off,(value,count) in fields.items(): write(0xff0000+address+off,value,count)
                write(0xff845f,v['parent_pose']); write(0xff8471,v['parent_held']); write(0xff8469,0xa7); write(0xff8475,0xe6)
                write(0xff8440,0x007babcd,4); write(0xff8444,0x00fe4321,4); write(0xff849c,0x01401234,4)
                for i in range(4): write(0xffa004+i*2,[0,240,256][(i+seed)%3],2)
                for i in range(16): write(0xff9404+i*128,128 if v['pool_busy'] or i%3 == 0 else 0,2)
                write(0xffa036,(seed%16)*128,2); write(0xffa030,0x97); write(0xffff18,0x12345678,4)
                write_bytes(0xfff80a,bytes([0,0x80,0,0x81])); write(0xffa03e,[0,0x2000,0xffff][seed%3],2)
                for off,value in [(0xa014,31),(0xa016,23),(0xf566,-3)]: write(0xff0000+off,value,2)
                write(0xffa010,0,2); write(0xffa012,320,2)
                write(0xfff550,v['pause']); write(0xfff55a,v['slow'])
                return address

            def run(role, name, address):
                nonlocal label, requested
                label, requested = name, role
                for reg in range(8): setreg(reg,0)
                setreg(17,0x2700); setreg(15,0xffffff00); write(0xffff00,0x3d8,4); setreg(13,0xffff0000|address)
                entries = {v:k for k,v in ENTRIES.items()} | {v:k for k,v in CREATORS.items()} | {'clock':0x19c96,'record':0x1eb40}
                setreg(16,entries[role]); ref.cpu.cycles,ref.cpu.refresh_cycles = 0,896
                count = len(isolated); hook(observer); execute(8000000); hook(None)
                assert len(isolated) == count+1 and pending is None and not errors,(name,errors[:3],pending and pending['kind'])
                return isolated[-1]

            # Dirty branches are collected before production code exists.
            for behavior in [0x1414e,0x143e4]:
                role = ENTRIES[behavior]
                for seed,(stage,paused,index,duration,ticks,completed,facing) in enumerate(itertools.product([0,2],range(3),range(3),[0,8,255],[0,1,255],[0,1,2],[0,1,2,255])):
                    a=setup(seed,behavior,stage=stage,pause=128 if paused==1 else 0,slow=255 if paused==2 else 0,index=index,duration=duration,ticks=ticks,completed=completed,facing=facing)
                    run(role,f'clock-{role}-{seed}',a)
                for seed,(contact,action,gravity,facing,busy) in enumerate(itertools.product([0,4,8,16,24,28],[0,1],[0,1,2,3,4,255],[0,1,2,255],[False,True])):
                    a=setup(seed,behavior,contact=contact,action=action,gravity=(gravity<<24)|0x123456,facing=facing,pool_busy=busy,y=300<<16|0x5678)
                    run(role,f'contact-{role}-{seed}',a)
                for seed,(x,y,vy) in enumerate(itertools.product([-33,-32,-31,0,352,353,32767,-32768],[-1,0,287,288,319,320,321],[0,-1,0x7fffffff,-0x80000000])):
                    a=setup(seed,behavior,x=(x<<16)|0xabcd,y=(y<<16)|0x5678,vy=vy); run(role,f'geometry-{role}-{seed}',a)
                for seed,size in enumerate([0,31,271,*range(16,257,16)]):
                    a=setup(seed,behavior,size=size,contact=24,y=300<<16|0x5678); run(role,f'clear-{role}-{size}',a)
                stoprole='linear_stop' if behavior==0x1414e else 'steered_stop'
                for seed,(stop,shake,offset) in enumerate(itertools.product([0,1,2,255],[0,1,2,255],[-32768,-1,0,32767])):
                    a=setup(seed,behavior,stop=stop,shake=shake); write(0xff0000+a+0x54,offset,2); run(stoprole,f'stop-{role}-{seed}',a)
            for seed,(held,vy,facing) in enumerate(itertools.product([0,1,2,3,255],[-0x60001,-0x60000,-0x5ffff,0,0x5ffff,0x60000,0x60001],[0,1])):
                a=setup(seed,0x143e4,parent_held=held,vy=vy,facing=facing,pose=0x19); run('steered',f'steer-{seed}',a)
            for seed,(stage,paused,index,ticks,parent_pose) in enumerate(itertools.product([0,2],range(3),range(9),[0,1,2,255],[0xa3,0xa2,0xff])):
                if stage == 2 and index == 8 and ticks == 1: continue
                a=setup(seed,0x146ba,stage=stage,pause=128 if paused==1 else 0,slow=255 if paused==2 else 0,index=index,ticks=ticks,parent_pose=parent_pose); run('follower',f'follow-{seed}',a)
            boundary_count=len(isolated)
            print(json.dumps({'boundaries_collected':boundary_count,'natural_so_far':len(natural)}),flush=True)
            # Each creator/clock/record/child stream retains one initial RAM state.
            for sequence_index,(action,facing,paused) in enumerate(itertools.product([47,48,49,50],[0,1],[False,True])):
                setup(sequence_index); write_bytes(0xff8500,bytes(768))
                write(0xff8400,0x12668,4); write(0xff8404,256,2); write(0xff8406,2,2)
                for off,value in [(0x5d,action),(0x5f,{47:0xa0,48:0xac,49:0xa3,50:0xa0}[action]),(0x59,0),(0x5b,255),(0x60,0),(0x61,facing)]: write(0xff8400+off,value)
                write(0xff8410,0x10000 if action==48 else 0,4); write(0xff8414,0x10000 if action==48 else 0x48000,4); write(0xff8448,0,4); write(0xff844c,0x03123456 if action==48 else 0x00123456,4)
                run('record','initial-record',0x8400); initial=snapshot(ref.ram_cpu()); calls=[]
                for step in range(512):
                    external={}
                    if paused and step==1: write(0xfff550,128); external['pause_flags']=128
                    if paused and step==2: write(0xfff550,0); external['pause_flags']=0
                    rows=[]; ram=ref.ram_cpu()
                    if ram[0x845d] in [47,48,49,50] and not ram[0xf550]&128:
                        rows.append(run('clock',f'{sequence_index}/{step}-clock',0x8400))
                        rows.append(run('creator'+str(ram[0x845d]),f'{sequence_index}/{step}-creator',0x8400))
                        rows.append(run('record',f'{sequence_index}/{step}-record',0x8400))
                    for address in [0x8500,0x8600,0x8700]:
                        ram=ref.ram_cpu(); behavior=n(ram,address,4)
                        if n(ram,address+4) and behavior in ENTRIES: rows.append(run(ENTRIES[behavior],f'{sequence_index}/{step}-child',address))
                    assert rows, ('empty-stream-step',action,facing,paused,step,ram[0x845d],ram[0x845f],ram[0x8459],ram[0x845b],[n(ram,a+4) for a in [0x8500,0x8600,0x8700]])
                    rows[0]['external_input']=external; calls.extend(rows)
                    ram=ref.ram_cpu()
                    if ram[0x845d] not in [47,48,49,50] and not any(n(ram,a+4) for a in [0x8500,0x8600,0x8700]): break
                else: raise AssertionError(('creator/child flow did not terminate',action))
                streams.append({'name':f'{action}-{facing}-{paused}','initial':initial,'calls':calls})
                print(json.dumps({'stream':streams[-1]['name'],'calls':len(calls)}),flush=True)
            private=False

    collections=[(args.prefix+'-oracle',natural,'Actual source5 callbacks from Root fixed real-menu baselines')]
    if not args.natural_only: collections += [(args.prefix+'-boundaries',isolated[:boundary_count],'Dirty original callbacks derived from a proven source5 checkpoint'),(args.prefix+'-sequences',streams,'Single-initial creator/clock/record/child through final clear')]
    for name,rows,scope in collections:
        raw=(json.dumps({'library':list(library.values()),'calls':rows},separators=(',',':'))+'\n').encode(); packed=gzip.compress(raw,mtime=0)
        (out/(name+'.json.gz')).write_bytes(packed)
        calls=[r for s in rows for r in s['calls']] if name.endswith('sequences') else rows
        report={'pass':bool(rows) and not errors,'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),
                'filter_sha256':sha(bridge_path.read_bytes()),'records':len(rows),'calls':len(calls),'kind_counts':dict(Counter(r['kind'] for r in calls)),
                'fixture_sha256':sha(raw),'gzip_sha256':sha(packed),'frames_match_original_core':True,'baselines':baselines,'binding':binding,'library_frames':len(library),
                'unexplained_change_calls':sum(bool(r['unexplained_changes']) for r in calls),'external_change_calls':sum(bool(r['external_changes']) for r in calls),
                'rng_calls':sum(r['rng_calls'] for r in calls),'clears':sum(bool(r['cleared_bytes']) for r in calls),'draws':sum(r['draw'] for r in calls),
                'interrupt_calls':sum(len(r['interrupts']) for r in calls),'incomplete_calls':0,'errors':errors,
                'limits':['No private context is called natural; zero natural callbacks is pass:false','Full64KB deltas retained and outside writes individually attributed','Hit-stop slave natural reachability only established when actually observed','Typed animation data only; originalCPU/ROM is research-only']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n')
        print(json.dumps({'name':name,'records':len(rows),'calls':len(calls),'kinds':report['kind_counts'],'pass':report['pass']}))
    catalog_path=ROOT/'games/yuyuhakusho/data/animation-catalog.json.gz'; existing=json.loads(gzip.decompress(catalog_path.read_bytes()))['frames']
    missing=[r for r in library.values() if not any(v['key']==r['key'] and v['data']==r['data'] for v in existing)]
    small={'source_sha256':sha(rom),'frames':list(library.values()),'follower':{'address':'0x1479c','bytes':rom[0x1479c:0x147ae].hex(),'values':[[rom[0x1479c+i*2],rom[0x1479d+i*2]] for i in range(9)]}}
    (out/(args.prefix+'-data.json')).write_text(json.dumps(small,indent=2)+'\n')
    print(json.dumps({'missing_animation_frames':len(missing),'all_frames':len(library)}))


if __name__=='__main__': main()
