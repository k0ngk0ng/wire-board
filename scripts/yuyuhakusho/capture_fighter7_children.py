#!/usr/bin/env python3
"""Pinned68000 source7 child callbacks; all private flows derive from one proven menu checkpoint."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
import sys

sys.dont_write_bytecode = True
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data

ENTRIES = {0x136e6: 'first', 0x1391e: 'second', 0x15094: 'pan', 0x149ac: 'effect'}
CREATORS = {0x1be64: 'creator59', 0x1bfce: 'creator60', 0x1c032: 'creator61'}
AN_RETURNS = {0x1edaa, 0x21970, 0x21982, 0x21994, 0x219a6, 0x219aa}
RETURNS = {0x150c2, 0x14a64, 0x14d6a}
PATHS = {0x137f4: 'first_hit_stop', 0x13a08: 'second_hit_stop', 0x13b1a: 'heal', 0x20666: 'healing_fx'}
PARENTS = [0x8400, 0x8800, 0x8c00, 0x9000]


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def draw(ram, off):
    return {'ObjectID': off, 'GraphicsSet': n(ram, off+0x1c, 4), 'TileBank': n(ram, off+0x20, 4),
            'Parameter': n(ram, off+0x3c), 'ScreenX': n(ram, off+8), 'ScreenY': n(ram, off+12),
            'OffsetX': n(ram, off+0x28, 2, True), 'OffsetY': n(ram, off+0x2a, 2, True),
            'Attributes': n(ram, off+0x24), 'Depth': n(ram, off+0x3e), 'Layer': ram[off+0x26],
            'SkipTiles': ram[off+0x27], 'Cache': [n(ram, off+0x2c+i*4, 4) for i in range(4)]}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--baseline', nargs=3, action='append', required=True, metavar=('INPUTS', 'REFERENCE', 'MANIFEST'))
    p.add_argument('--checkpoint', type=int, default=2400)
    p.add_argument('--natural-only', action='store_true')
    p.add_argument('--prefix', default='fighter7-children')
    args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT) and args.prefix.replace('-', '').isalnum()
    out.mkdir(parents=True, exist_ok=True)
    rom_path = ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'; rom = rom_path.read_bytes()
    assert sha(rom) == 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = sorted(set(ENTRIES) | set(CREATORS) | AN_RETURNS | RETURNS | set(PATHS) |
                    {0x19c96, 0x19cc2, 0x15afc, 0x1580, 0x3556, 0x3574, 0x1eb40, 0x37c56, 0x37cac, 0x3982,
                     0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a})
    assert len(points) <= 64
    pcs = (C.c_uint*len(points))(*points)
    natural, isolated, streams, errors, baselines, library = [], [], [], [], [], {}
    pending = None; private = False; requested = ''; label = ''; scenario = ''; sequence = 0; binding = None
    tracked_fx = set()

    def snapshot(ram):
        return {'object_id': 0x8400, 'family': ram[0x8400:0x8800].hex(),
                'parents': [{'object_id': address, 'bytes': ram[address:address+256].hex()} for address in PARENTS],
                'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'buffers': ram[0xa200:0xa600].hex(), 'offsets': [n(ram, 0xa004+i*2) for i in range(4)],
                'pending_sound': ram[0xa030], 'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'pan_amplitude': n(ram, 0xf570), 'tile_cursor': ram[0xa031],
                'parameters': {'vertical_velocity': n(rom, n(ram, 0x8490, 4)+4, 4, True),
                               'gravity': n(rom, n(ram, 0x8490, 4)+8, 4, True),
                               'horizontal_velocity': n(rom, n(ram, 0x8490, 4)+12, 4, True)},
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
        with ReferenceSession(ROOT, ROOT/'.local/md-yuyuhakusho'/('fighter7-children-observed-'+args.prefix+'-'+str(source_index)), core=core) as ref:
            def remember_frame(ram, address):
                key, data, origin = selected_data(rom, ram, ram[address:address+256]); identity = json.dumps(key, sort_keys=True)
                assert identity not in library or library[identity]['data'] == data
                library[identity] = {'key': key, 'data': data, 'source': origin}

            @cbtype
            def observe(kind, width, pc, value):
                nonlocal pending, sequence
                try:
                    if kind != 1: return
                    raw_pc = pc; pc &= 0xffffff
                    if pc == 0xfff000:
                        if pending is not None:
                            pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), 'entry_sp': ref.get_register(15), '_ram': ref.ram_cpu()})
                        return
                    if pc in [0xfff09a, 0xfff0a8, 0xfff13a]:
                        if pending is not None and pending['_irq']:
                            irq = pending['_irq'].pop(); ram = ref.ram_cpu()
                            irq.update(exit=hex(pc), exit_register=hex(raw_pc), exit_sp=ref.get_register(15),
                                       changes=[{'offset': i, 'before': a, 'after': b} for i, (a,b) in enumerate(zip(irq.pop('_ram'), ram)) if a != b])
                            pending['interrupts'].append(irq)
                        return
                    if pc in [0x37c56, 0x37cac]:
                        if pending is not None:
                            a = ref.get_register(9)&65535
                            pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                        return
                    if pc == 0x3982:
                        if pending is not None: pending['rng_calls'] += 1
                        return
                    address, sp = ref.get_register(13)&65535, ref.get_register(15); ram = ref.ram_cpu()
                    if pending is not None and pc in PATHS:
                        pending['paths'].append(PATHS[pc])
                    role = ENTRIES.get(pc)
                    if role == 'effect' and not private and address not in tracked_fx: role = None
                    if private and pending is None:
                        role = CREATORS.get(pc, role)
                        if requested == 'clock' and pc == 0x19c96: role = 'clock'
                        if requested == 'record' and pc == 0x1eb40: role = 'record'
                    if role and ram[0x8458] == 7 and (private or ref.frame <= len(expected)-2):
                        assert pending is None, ('nested', role, pending and pending['kind'])
                        sequence += 1
                        pending = {'kind': role, 'name': label, 'scenario': scenario, 'sequence': sequence, 'entry': hex(pc), 'frame': ref.frame,
                                   'object_id': address, 'source_register': hex(ref.get_register(13)), 'entry_sp': sp, 'paths': [],
                                   'before': snapshot(ram), 'draw': False, 'cleared_bytes': 0, 'external_queue_clears': [],
                                   'rng_calls': 0, 'interrupts': [], '_irq': [], '_ram': ram}
                        if role == 'record': remember_frame(ram, address)
                    elif pending is not None:
                        if pc == 0x1eb40: remember_frame(ram, address)
                        if pc == 0x3556:
                            pending['cleared_bytes'] = (n(ram, address+4) or 64)//16*16
                            assert 0 < pending['cleared_bytes'] <= (128 if pending['kind'] == 'effect' else 256)
                        finish = ((pc == 0x1580 or pc in RETURNS) and pending['kind'] in ENTRIES.values() or
                                  pc == 0x3574 and pending['cleared_bytes'] or
                                  pc == 0x15afc and pending['kind'].startswith('creator') or
                                  pc == 0x19cc2 and pending['kind'] == 'clock' or
                                  pc in AN_RETURNS and pending['kind'] == 'record')
                        if finish:
                            row = pending; pending = None; assert sp == row['entry_sp'] and not row.pop('_irq')
                            row.update(after=snapshot(ram), draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame,
                                       draw_request=draw(ram, address) if pc == 0x1580 else None)
                            original_ram = row.pop('_ram')
                            row['ram_changes'] = [{'offset': i, 'before': a, 'after': b} for i,(a,b) in enumerate(zip(original_ram, ram)) if a != b]
                            allowed = (set(range(0x8400,0x8800)) | set(range(0x9400,0x9c00)) | set(range(0xa200,0xa600)) |
                                       set(range(0xa004,0xa00c)) | {0xa030,0xa031,0xa036,0xa037,0xf570,0xf571} | set(range(0xf80a,0xf80e)) |
                                       {(row['entry_sp']-i)&65535 for i in range(0x300)})
                            if row['kind'] == 'second': allowed |= {a+off for a in PARENTS for off in [0x78,0x79]}
                            if row['kind'] == 'creator61': allowed |= {a+0x69 for a in PARENTS}
                            row['external_changes'] = [r for r in row['ram_changes'] if r['offset'] not in allowed]
                            row['unexplained_changes'] = []
                            for delta in row['external_changes']:
                                current, steps = delta['before'], 0
                                for irq in row['interrupts']:
                                    for d in irq['changes']:
                                        if d['offset'] == delta['offset']:
                                            assert d['before'] == current; current, steps = d['after'], steps+1
                                for clear in row['external_queue_clears']:
                                    if int(clear['address'],16) == delta['offset']:
                                        assert clear['before'] == current; current, steps = 0, steps+1
                                if not steps or current != delta['after']: row['unexplained_changes'].append(delta)
                            assert not row['unexplained_changes'] and row['rng_calls'] == 0, row['unexplained_changes'][:8]
                            assert row['before']['random_seed'] == row['after']['random_seed']
                            if row['kind'] == 'second':
                                for a in range(0x9400,0x9c00,128):
                                    if not n(original_ram,a+4) and n(ram,a+4) and ram[a+0x58] == 0x24: tracked_fx.add(a)
                            if row['kind'] == 'effect' and row['cleared_bytes']: tracked_fx.discard(address)
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
                              'reference_manifest_sha256': sha(mp.read_bytes()), 'video_steps': len(expected),
                              'entry_video_limit': len(expected)-2, 'drain_video_steps': 2, 'all_ram_video_match': True})
            assert base_ram[0x8458] == 7 and n(base_ram,0x8400,4) == 0x12668 and n(base_ram,0x8404) == 256
            if source_index != 0 or args.natural_only: continue
            binding = {'source_tag': 7, 'checkpoint_frame': args.checkpoint, 'source_actor_sha256': sha(base_ram[0x8400:0x8500]),
                       'animation_set': n(base_ram,0x8450,4), 'shape_set': n(base_ram,0x848c,4), 'movement_source': n(base_ram,0x8490,4)}
            private = True
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, size=1): write_bytes(address, (value&((1<<(size*8))-1)).to_bytes(size,'big'))

            def setup(seed=0, behavior=0x136e6, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True)
                write_bytes(0xff8400, base_ram[0x8400:0x8500]); write_bytes(0xff8500, bytes((i*73+seed*29+17)&255 for i in range(768)))
                write_bytes(0xff9400, bytes((i*37+seed*23+13)&255 for i in range(2048)))
                write_bytes(0xffa200, bytes((i*19+seed*7+5)&255 for i in range(1024)))
                address = {0x136e6:0x8500,0x1391e:0x8600,0x15094:0x8700}[behavior]
                v = {'stage':2,'size':256,'index':0,'duration':8,'ticks':0,'flags':0,'completed':0,'pause':0,'slow':0,
                     'facing':0,'x':100<<16|0xabcd,'y':260<<16|0x5678,'vx':90<<16|0x1234,'vy':250<<16|0x4321,
                     'ground':320<<16|0x1234,'aux48':0x01abcdef,'residual':0,'action':0,'contact':0,'stop':0,'shake':0,
                     'pose':0x3e if behavior==0x136e6 else 0x3c,'held':0,'pressed':0,'parent_pose':0xa7,'pool_mode':0,'pan':0x4321}
                v.update(changes)
                fields = {0:(behavior,4),4:(v['size'],2),6:(v['stage'],2),0x10:(v['vx'],4),0x14:(v['vy'],4),0x1c:(0xd8f1a,4),0x20:(0x240000,4),
                          0x50:(0xd92ca,4),0x8c:(0xd9a3a,4),0x59:(v['index'],1),0x5a:(v['duration'],1),0x5b:(v['ticks'],1),
                          0x5c:(v['flags'],1),0x60:(v['completed'],1),0x5d:(v['action'],1),0x5f:(v['pose'],1),0x61:(v['facing'],1),
                          0x40:(v['x'],4),0x44:(v['y'],4),0x48:(v['aux48'],4),0x98:(v['residual'],4),0x9c:(v['ground'],4),
                          0x89:(v['contact'],1),0x69:(v['stop'],1),0x6a:(v['shake'],1),0x71:(v['held'],1),0x72:(v['pressed'],1)}
                for off,(value,count) in fields.items(): write(address+off,value,count)
                write(0x845f,v['parent_pose']); write(0x8469,0xa7); write(0x8475,0xe6)
                for i,a in enumerate(PARENTS):
                    write(a+4,256,2);write(a+0x78,[0,207,223,-1][i],2);write(a+0x40,(100+i*10)<<16|0x3456,4);write(a+0x44,292<<16|0xabcd,4)
                for i in range(4): write(0xa004+i*2,[0,240,256][(i+seed)%3],2)
                for i in range(16):
                    occupied = [i%3==0,True,False,i!=(seed%16),i!=((seed+1)%16)][v['pool_mode']]
                    write(0x9404+i*128,128 if occupied else 0,2)
                write(0xa036,(seed%16)*128,2);write(0xa030,0x97);write(0xa031,0x35);write(0xff18,0x12345678,4)
                write_bytes(0xf80a,bytes([0,0x80,0,0x81]));write(0xa03e,[0,0x2000,0xffff][seed%3],2)
                for off,value in [(0xa014,31),(0xa016,23),(0xf566,-3)]:write(off,value,2)
                write(0xa010,0,2);write(0xa012,320,2);write(0xf570,v['pan'],2);write(0xf550,v['pause']);write(0xf55a,v['slow'])
                return address

            def run(role, name, address):
                nonlocal label, requested
                label, requested = name, role
                for reg in range(8):setreg(reg,0)
                setreg(17,0x2700);setreg(15,0xffffff00);write(0xff00,0x3d8,4);setreg(13,0xffff0000|address)
                entries = {v:k for k,v in ENTRIES.items()} | {v:k for k,v in CREATORS.items()} | {'clock':0x19c96,'record':0x1eb40}
                setreg(16,entries[role]);ref.cpu.cycles,ref.cpu.refresh_cycles = 0,896
                count=len(isolated);hook(observer);execute(8000000);hook(None)
                assert len(isolated)==count+1 and pending is None and not errors,(name,errors[:3],pending and pending['kind'])
                return isolated[-1]

            for behavior in [0x136e6,0x1391e]:
                role=ENTRIES[behavior]
                for seed,(stage,paused,index,duration,ticks,completed,facing) in enumerate(itertools.product([0,2],range(3),[0,1,5],[0,8,255],[0,1,255],[0,1,2],[0,1,2,255])):
                    a=setup(seed,behavior,stage=stage,pause=128 if paused==1 else 0,slow=255 if paused==2 else 0,index=index,duration=duration,ticks=ticks,completed=completed,facing=facing)
                    run(role,f'clock-{role}-{seed}',a)
                for seed,(action,parent_pose,held,pressed,completed) in enumerate(itertools.product(range(3 if role=='first' else 4),[0xa7,0xa6],[0,1,2,3,4,8,12,15,16,17,31],[0,64,255],[0,1,2])):
                    a=setup(seed,behavior,action=action,parent_pose=parent_pose,held=held,pressed=pressed,completed=completed)
                    run(role,f'action-{role}-{seed}',a)
                for seed,(x,y,facing,residual) in enumerate(itertools.product([-32768,-33,-32,-31,0,352,353,32767],[-1,0,319,320,321],[0,1,2],[-1,0,1,0x47ff,0x4800,0x4801,0x7fffffff])):
                    a=setup(seed,behavior,x=x<<16|0xabcd,y=y<<16|0x5678,facing=facing,residual=residual)
                    run(role,f'geometry-{role}-{seed}',a)
                for seed,(stop,shake,offset) in enumerate(itertools.product([1,2,255],[0,1,2,255],[-32768,-1,0,32767])):
                    a=setup(seed,behavior,stop=stop,shake=shake);write(a+0x54,offset,2);run(role,f'stop-{role}-{seed}',a)
                for seed,size in enumerate([0,31,271,*range(16,257,16)]):
                    a=setup(seed,behavior,size=size,action=1,completed=1);run(role,f'clear-{role}-{size}',a)
                for seed,(index,duration,ticks,flags,completed) in enumerate(itertools.product([0,5],[0,8,255],[0,1,7,8,254,255],[0,1,127,128,255],[0,1,2])):
                    a=setup(seed,behavior,index=index,duration=duration,ticks=ticks,flags=flags,completed=completed)
                    run(role,f'flags-{role}-{seed}',a)
                for seed,(left,right,x,y,ground) in enumerate(itertools.product([-32768,32767],[-32768,32767],[-32768,-33,-32,0,352,353,32767],[-1,0,32767],[-32768,-1,0,32767])):
                    a=setup(seed,behavior,x=x<<16|0xabcd,y=y<<16|0x5678,ground=ground<<16|0x1234)
                    write(0xa010,left,2);write(0xa012,right,2);run(role,f'wordwrap-{role}-{seed}',a)
            longs=[-0x80000000,-0x7fffffff,-1,0,1,0x7ffffffe,0x7fffffff]
            for seed,(position,target,face) in enumerate(itertools.product(longs,longs,[0,1,2,255])):
                a=setup(seed,0x1391e,action=2,x=position,y=-position-1,vx=target,vy=-target-1,aux48=face<<24|0xabcdef)
                run('second',f'long-smoothing-{seed}',a)
            for seed,(size,health,dx,dy) in enumerate(itertools.product([0,256],[-32768,-1,0,207,208,222,223,224,32751,32752,32767],[-32768,-25,-24,-23,0,23,24,25,32767],[-32768,-33,-32,-31,0,31,32,33,32767])):
                a=setup(seed,0x1391e,action=2,held=16,pool_mode=seed%5)
                for i,parent in enumerate(PARENTS):
                    write(parent+4,size if i%2==0 else 256,2);write(parent+0x78,health,2)
                    write(parent+0x40,(100+dx)<<16|0x3456,4);write(parent+0x44,(292+dy)<<16|0xabcd,4)
                run('second',f'heal-{seed}',a)
            for seed,(stage,paused,high,low,pan) in enumerate(itertools.product([0,2],range(3),[-32768,-1,0,1,15,16,17,4096,32767],[0,0x1234,65535],[0,1,65535])):
                a=setup(seed,0x15094,stage=stage,pause=128 if paused==1 else 0,slow=255 if paused==2 else 0,vy=high<<16|low,pan=pan)
                run('pan',f'pan-{seed}',a)
            for seed,size in enumerate([0,31,271,*range(16,257,16)]):
                a=setup(seed,0x15094,size=size,vy=16<<16|0x1234);run('pan',f'clear-pan-{size}',a)
            boundary_count=len(isolated)
            print(json.dumps({'boundaries_collected':boundary_count,'natural_so_far':len(natural)}),flush=True)
            for flow,(action,facing,paused) in enumerate(itertools.product([59,61],[0,1],[False,True])):
                setup(flow);write_bytes(0x8500,bytes(768));write_bytes(0x9400,bytes(2048));write(0xa036,0,2)
                write(0x845d,action);write(0x845f,0xa6 if action==59 else 0xaa);write(0x8459,0);write(0x845b,255);write(0x8460,0)
                write(0x8461,facing);write(0x8410,0x0018abcd,4);write(0x84b8,0x1000000,4);write(0x8471,0);write(0x8472,0)
                run('record','initial-record',0x8400);initial=snapshot(ref.ram_cpu());calls=[]
                for step in range(640):
                    external={};rows=[]
                    changes={100:{'pause_flags':128},101:{'pause_flags':0}} if paused else {}
                    changes.update({80:{'held':1,'pressed':64},81:{'pressed':0},120:{'held':0,'pressed':64},121:{'pressed':0},160:{'held':2,'pressed':64},161:{'pressed':0},200:{'held':16},201:{'held':0},240:{'pressed':32}})
                    for key,value in changes.get(step,{}).items():
                        write({'pause_flags':0xf550,'held':0x8471,'pressed':0x8472}[key],value);external[key]=value
                    ram=ref.ram_cpu()
                    if ram[0x845d] in [59,60,61] and not ram[0xf550]&128:
                        rows.append(run('clock',f'{flow}/{step}-clock',0x8400))
                        rows.append(run('creator'+str(ref.ram_cpu()[0x845d]),f'{flow}/{step}-creator',0x8400))
                        rows.append(run('record',f'{flow}/{step}-record',0x8400))
                    for address in [0x8500,0x8600,0x8700,*range(0x9400,0x9c00,128)]:
                        ram=ref.ram_cpu();behavior=n(ram,address,4)
                        if n(ram,address+4) and behavior in ENTRIES:rows.append(run(ENTRIES[behavior],f'{flow}/{step}-object',address))
                    assert rows,('empty-stream-step',action,step)
                    rows[0]['external_input']=external;calls.extend(rows);ram=ref.ram_cpu()
                    if ram[0x845d] not in [59,60,61] and not any(n(ram,a+4) for a in [0x8500,0x8600,0x8700,*range(0x9400,0x9c00,128)]):break
                else:raise AssertionError(('creator/child flow did not terminate',action))
                streams.append({'name':f'{action}-{facing}-{paused}','initial':initial,'calls':calls})
                print(json.dumps({'stream':streams[-1]['name'],'calls':len(calls)}),flush=True)
            private=False

    collections=[(args.prefix+'-oracle',natural,'Actual source7 child callbacks from Root fixed real-menu baselines')]
    if not args.natural_only:collections += [(args.prefix+'-boundaries',isolated[:boundary_count],'Dirty original callbacks derived from the proven source7 checkpoint'),(args.prefix+'-sequences',streams,'Single initial creator/clock/record/children and derivedFX through final clear')]
    for name,rows,scope in collections:
        raw=(json.dumps({'library':list(library.values()),'calls':rows},separators=(',',':'))+'\n').encode();packed=gzip.compress(raw,mtime=0)
        (out/(name+'.json.gz')).write_bytes(packed)
        calls=[r for s in rows for r in s['calls']] if name.endswith('sequences') else rows
        report={'pass':bool(rows) and not errors,'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),
                'filter_sha256':sha(bridge_path.read_bytes()),'records':len(rows),'calls':len(calls),'kind_counts':dict(Counter(r['kind'] for r in calls)),
                'fixture_sha256':sha(raw),'gzip_sha256':sha(packed),'frames_match_original_core':True,'baselines':baselines,'binding':binding,'library_frames':len(library),
                'unexplained_change_calls':sum(bool(r['unexplained_changes']) for r in calls),'external_change_calls':sum(bool(r['external_changes']) for r in calls),
                'rng_calls':sum(r['rng_calls'] for r in calls),'clears':sum(bool(r['cleared_bytes']) for r in calls),'draws':sum(r['draw'] for r in calls),
                'interrupt_calls':sum(len(r['interrupts']) for r in calls),'paths':dict(Counter(p for r in calls for p in r['paths'])),'incomplete_calls':0,'errors':errors,
                'limits':['Private cases are never called natural; zero natural callbacks is pass:false','Full64KB deltas retained; four raw parents andF570 captured separately from theP1 family',
                          'Each draw is captured at its own1580 boundary','Typed data only; originalCPU andROM remain research-only']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n')
        print(json.dumps({'name':name,'records':len(rows),'calls':len(calls),'kinds':report['kind_counts'],'pass':report['pass']}))
    (out/(args.prefix+'-data.json')).write_text(json.dumps({'source_sha256':sha(rom),'frames':list(library.values())},indent=2)+'\n')


if __name__=='__main__':main()
