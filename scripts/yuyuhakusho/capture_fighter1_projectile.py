#!/usr/bin/env python3
"""Source1 linear/wave/legacy-beam callbacks and real creator-clock contexts."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_animation_records import selected_data

CHILD = {0x12ee8: 'linear', 0x1300c: 'wave', 0x12dd4: 'beam'}
CREATOR = {0x1a8f0: 'creator_linear', 0x1a9be: 'creator_wave', 0x1aac2: 'creator_beam'}


def n(raw, off, size=2, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', type=Path, required=True); p.add_argument('--natural-only', action='store_true')
    p.add_argument('--inputs', type=Path, default=ROOT/'docs/research/md-yuyuhakusho/source1-next-inputs.json')
    p.add_argument('--reference', type=Path, default=ROOT/'games/yuyuhakusho/engine/testdata/fighter1-attacks-reference-frames.json')
    p.add_argument('--reference-manifest', type=Path, default=ROOT/'games/yuyuhakusho/engine/testdata/fighter1-attacks-reference-manifest.json')
    p.add_argument('--prefix', default='fighter1-projectile'); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT) and args.prefix.replace('-', '').isalnum(); out.mkdir(parents=True, exist_ok=True)
    input_path, reference_path, manifest_path = args.inputs.resolve(), args.reference.resolve(), args.reference_manifest.resolve()
    reference_raw = reference_path.read_bytes(); expected = json.loads(reference_raw); events = json.loads(input_path.read_text()); original = json.loads(manifest_path.read_text())
    assert original['output']['sha256'] == sha(reference_raw) and original['input']['sha256'] == sha(input_path.read_bytes())
    assert original['identity']['core_sha256'] == '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    arena_path = ROOT/'games/yuyuhakusho/data/arena-runtime-rules.json'; wave = json.loads(arena_path.read_text())['library']['wave_x']
    assert wave == [n(rom, 0x34000+i*2, 1, True) for i in range(1024)]
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [*CHILD, *CREATOR, 0x19c96, 0x19cc2, 0x15afc, 0x1580, 0x3556, 0x3574, 0x1eb40, 0x3982, 0x37c56, 0x37cac,
              0xfff000, 0xfff09a, 0xfff0a8, 0xfff13a, 0xfffff000, 0xfffff09a, 0xfffff0a8, 0xfffff13a]
    pcs = (C.c_uint*len(points))(*points); natural, isolated, streams, errors = [], [], [], []
    library = {}; pending = None; private = False; requested = ''; label = ''
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'

    def snapshot(ram, parent=0x8400):
        obj = ram[parent:parent+256]; ptr = n(obj, 0x90, 4)&0xffffff; memory, index = (rom, ptr) if ptr < len(rom) else (ram, ptr&65535)
        return {'object_id': parent, 'family': ram[parent:parent+1024].hex(), 'pool': {'cursor': n(ram, 0xa036), 'bytes': ram[0x9400:0x9c00].hex()},
                'buffers': ram[0xa200:0xa600].hex(), 'offsets': [n(ram, 0xa004+i*2) for i in range(4)], 'pending_sound': ram[0xa030],
                'sound_queue': list(ram[0xf80a:0xf80e]), 'random_seed': n(ram, 0xff18, 4),
                'parameters': {key: n(memory, index+delta, 4, True) for key, delta in [('vertical_velocity', 4), ('gravity', 8), ('horizontal_velocity', 12)]},
                'environment': {'pause_flags': ram[0xf550], 'slow_flag': ram[0xf55a], 'palette_attributes': n(ram, 0xa03e),
                                'arena_left': n(ram, 0xa010, 2, True), 'arena_right': n(ram, 0xa012, 2, True),
                                'camera_x': n(ram, 0xa014, 2, True), 'camera_y': n(ram, 0xa016, 2, True), 'shake': n(ram, 0xf566, 2, True)}}

    with ReferenceSession(ROOT, out/'observed', core=core) as ref:
        def remember(ram, address):
            off = address&65535; key, data, origin = selected_data(rom, ram, ram[off:off+256]); identity = json.dumps(key, sort_keys=True)
            assert identity not in library or library[identity]['data'] == data
            library[identity] = {'key': key, 'data': data, 'source': origin}; return key, data

        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                raw_pc = pc; pc &= 0xffffff
                if pc == 0xfff000:
                    if pending is not None: pending['_irq'].append({'entry': hex(pc), 'entry_register': hex(raw_pc), '_ram': ref.ram_cpu()})
                    return
                if pc in [0xfff09a, 0xfff0a8, 0xfff13a]:
                    if pending is not None and pending['_irq']:
                        irq = pending['_irq'].pop(); ram = ref.ram_cpu(); irq.update(exit=hex(pc), changes=[{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(irq.pop('_ram'), ram)) if a != b]); pending['interrupts'].append(irq)
                    return
                if pc == 0x3982:
                    if pending is not None: pending['rng_calls'] += 1
                    return
                if pc in [0x37c56, 0x37cac]:
                    if pending is not None:
                        a = ref.get_register(9)&65535; pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a]})
                    return
                address, sp = ref.get_register(13)&0xffffff, ref.get_register(15); parent = (address&65535)&0xfc00; ram = ref.ram_cpu()
                role = CHILD.get(pc)
                if private and pc in CREATOR: role = CREATOR[pc]
                if private and requested == 'clock' and pc == 0x19c96: role = 'clock'
                if role and ram[parent+0x58] == 1 and (private or ref.frame <= 3600):
                    assert pending is None
                    pending = {'kind': role, 'frame': ref.frame, 'object_id': address&65535, 'source_register': hex(ref.get_register(13)), 'entry_sp': sp,
                               'before': snapshot(ram, parent), 'draw': False, 'cleared_bytes': 0, 'rng_calls': 0, 'external_queue_clears': [], 'interrupts': [], '_irq': [], '_ram': ram}
                elif pending is not None:
                    if pc == 0x1eb40: remember(ram, address)
                    elif pc == 0x3556:
                        pending['cleared_bytes'] = (n(ram, (address&65535)+4) or 64)//16*16; assert 0 < pending['cleared_bytes'] <= 256
                    finish = (pc == 0x1580 and pending['kind'] in CHILD.values() or pc == 0x3574 and pending['cleared_bytes'] or
                              pc == 0x15afc and pending['kind'].startswith('creator_') or pc == 0x19cc2 and pending['kind'] == 'clock')
                    if finish:
                        row = pending; pending = None; assert sp == row['entry_sp'] and not row.pop('_irq')
                        row.update(after=snapshot(ram, parent), draw=pc == 0x1580, exit=hex(pc), exit_frame=ref.frame)
                        allowed = (set(range(parent, parent+1024)) | set(range(0x9400, 0x9c00)) | set(range(0xa200, 0xa600)) | set(range(0xa004, 0xa00c)) |
                                   {0xa030, 0xa036, 0xa037} | set(range(0xf80a, 0xf80e)) | {(row['entry_sp']-i)&65535 for i in range(0x180)})
                        row['external_changes'] = [{'offset': i, 'before': a, 'after': b} for i, (a, b) in enumerate(zip(row.pop('_ram'), ram)) if a != b and i not in allowed]
                        row['unexplained_changes'] = []
                        for change in row['external_changes']:
                            current, steps = change['before'], 0
                            for irq in row['interrupts']:
                                for delta in irq['changes']:
                                    if delta['offset'] != change['offset']: continue
                                    assert delta['before'] == current; current, steps = delta['after'], steps+1
                            if steps == 0 or current != change['after']: row['unexplained_changes'].append(change)
                        assert not row['unexplained_changes'] and row['rng_calls'] == 0 and row['before']['random_seed'] == row['after']['random_seed']
                        if private:
                            assert not row['external_changes'] and not row['external_queue_clears']; row['name'] = label; isolated.append(row); ref.cpu.cycles = 8000000
                        else: natural.append(row)
            except Exception as exc:
                errors.append(repr(exc))
                if private: ref.cpu.cycles = 8000000

        assert bridge.yyh_set_observer(observe, pcs, len(points)); hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for frame in range(1, len(expected)+1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400: checkpoint, base_ram = ref.save(), ref.ram_cpu()
        hook(None); assert frames == expected and pending is None and not errors, errors[:3]
        assert base_ram[0x8458] == 1
        identity = ref.identity
        baselines = [{'input': str(input_path.relative_to(ROOT)), 'input_sha256': sha(input_path.read_bytes()), 'reference': str(reference_path.relative_to(ROOT)),
                      'reference_raw_sha256': sha(reference_raw), 'reference_manifest': str(manifest_path.relative_to(ROOT)), 'reference_manifest_sha256': sha(manifest_path.read_bytes()),
                      'video_steps': len(expected), 'all_ram_video_match': True}]
        binding = {'source_tag': 1, 'source_actor_sha256': sha(base_ram[0x8400:0x8500]), 'animation_set': n(base_ram, 0x8450, 4), 'shape_set': n(base_ram, 0x848c, 4), 'movement_source': n(base_ram, 0x8490, 4)}
        if not args.natural_only:
            private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

            def write_bytes(address, raw):
                for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = byte

            def write(address, value, size=1): write_bytes(address, (value&((1 << (size*8))-1)).to_bytes(size, 'big'))

            def setup(role='linear', seed=0, **changes):
                ref.restore(checkpoint, restore_refresh_cycles=True); write_bytes(0xff8400, base_ram[0x8400:0x8500])
                write_bytes(0xff8500, bytes((i*73+seed*29+17)&255 for i in range(768))); write_bytes(0xff9400, bytes((i*37+seed*23+13)&255 for i in range(2048)))
                write_bytes(0xffa200, bytes((i*19+seed*7+5)&255 for i in range(1024)))
                v = {'stage': 2, 'size': 256, 'index': 0, 'duration': 255, 'ticks': 0, 'flags': 0, 'completed': 0, 'facing': 0,
                     'x': 100<<16|0x5678, 'y': 260<<16|0x5678, 'vx': 100<<16|0xabcd if role == 'wave' else 0x60000,
                     'vy': -0x28000 if role == 'wave' else -1, 'ground': 320<<16|0x1234, 'phase': 0x2c0, 'remaining': 24,
                     'contact': 0, 'pause': 0, 'slow': 0, 'left': 0, 'right': 600, 'pool': 'empty'}; v.update(changes)
                pose = {'linear':10,'wave':11,'beam':9}[role]
                fields = {0:({'linear':0x12ee8,'wave':0x1300c,'beam':0x12dd4}[role],4),4:(v['size'],2),6:(v['stage'],2),0x1c:(0xd8f1a,4),0x20:(0x240000,4),
                          0x50:(0xd92ca,4),0x8c:(0xd9a3a,4),0x5f:(pose,1),0x59:(v['index'],1),0x5a:(v['duration'],1),0x5b:(v['ticks'],1),0x5c:(v['flags'],1),
                          0x60:(v['completed'],1),0x61:(v['facing'],1),0x40:(v['x'],4),0x44:(v['y'],4),0x10:(v['vx'],4),0x14:(v['vy'],4),0x9c:(v['ground'],4),
                          0x48:(v['phase']<<16|0xabcd,4),0x4c:(v['remaining']<<16|0x1234,4),0x89:(v['contact'],1),0x54:(-3,2),0x56:(4,2),0x69:(0,1)}
                for off,(value,count) in fields.items(): write(0xff8500+off,value,count)
                for i in range(16): write(0xff9404+i*128,128 if v['pool']=='full' else 0,2)
                for i in range(4): write(0xffa004+i*2,[0,240,256][(seed+i)%3],2)
                write(0xffa036,0,2);write(0xffa030,0x97);write(0xffff18,0x12345678,4);write_bytes(0xfff80a,bytes([0,0x80,0,0x81]));write(0xffa03e,[0,0x2000,0xffff][seed%3],2)
                for off,value in [(0xa010,v['left']),(0xa012,v['right']),(0xa014,31),(0xa016,23),(0xf566,-3)]:write(0xff0000+off,value,2)
                write(0xfff550,v['pause']);write(0xfff55a,v['slow'])

            def run(role,name,address=0x8500):
                nonlocal label,requested
                label,requested=name,role
                for reg in range(8):setreg(reg,0)
                setreg(17,0x2700);setreg(15,0xffffff00);write(0xffff00,0x3d8,4);setreg(13,0xffff0000|address)
                entry={'linear':0x12ee8,'wave':0x1300c,'beam':0x12dd4,'creator_linear':0x1a8f0,'creator_wave':0x1a9be,'creator_beam':0x1aac2,'clock':0x19c96}[role]
                setreg(16,entry);ref.cpu.cycles,ref.cpu.refresh_cycles=0,896;count=len(isolated)
                hook(observer);execute(8000000);hook(None);assert len(isolated)==count+1 and pending is None and not errors,(name,errors[:3]);return isolated[-1]

            def record(name):
                before=snapshot(ref.ram_cpu());key,data=remember(ref.ram_cpu(),0xff8400);write(0xff845a,data['duration']);write(0xff845c,data['flags'])
                row={'kind':'record','name':name,'object_id':0x8400,'before':before,'after':snapshot(ref.ram_cpu()),'record_key':key,'draw':False,'cleared_bytes':0,
                     'rng_calls':0,'external_queue_clears':[],'interrupts':[],'external_changes':[],'unexplained_changes':[]};isolated.append(row);return row

            def parent_setup(role,seed,facing=0,pool='empty',pose=None):
                setup(role,seed,pool=pool);write_bytes(0xff8500,bytes(768));write(0xff8400,0x12668,4);write(0xff8404,256,2);write(0xff8406,2,2)
                for off,value in [(0x5d,{'linear':0x1b,'wave':0x1c,'beam':0x1a}[role]),(0x5f,pose if pose is not None else {'linear':0xa2,'wave':0xa0,'beam':0xa3}[role]),(0x59,0),(0x5b,255),(0x60,0),(0x61,facing)]:write(0xff8400+off,value)
                write(0xff8410,0x300000 if role=='beam' else 0x40000,4);write(0xff8440,(100 if facing==0 else 500)<<16|0xabcd,4)
                write(0xff8444,320<<16|0x5678,4);write(0xff849c,320<<16|0x1234,4);write(0xff847c,0,2)

            for role,pose in [('linear',10),('wave',11)]:
                start=0xd92ca+n(rom,0xd92ca+pose*2,2,True)
                for index in range(64):
                    for facing in [0,1]:setup(role,index+facing,index=index,duration=255,facing=facing);run(role,f'animation-{pose}-{index}-{facing}')
                    if rom[start+index*8+2]&128:break
                else:raise AssertionError('unterminated AN')
            for seed,(role,stage,index,duration,ticks,completed,paused,facing) in enumerate(itertools.product(['linear','wave'],[0,2],[0,1],[0,8,255],[0,1,255],[0,1,2],range(3),[0,1,3])):
                setup(role,seed,stage=stage,index=index,duration=duration,ticks=ticks,flags=128 if role=='linear' and index else 0,completed=completed,pause=128 if paused==1 else 0,slow=255 if paused==2 else 0,facing=facing)
                run(role,f'phase-{role}-{stage}-{index}-{duration}-{ticks}-{completed}-{paused}-{facing}')
            geometry=[(-32768,-32768),(-1,260),(31,319),(100,319),(100,320),(100,321),(632,260),(633,260),(32767,32767),(700,321)]
            for seed,((x,y),vy,contact,facing) in enumerate(itertools.product(geometry,[-1,0,1],[0,1,4,8,16,24],[0,1,3])):
                setup('linear',seed,x=x<<16|0xabcd,y=y<<16|0x5678,vx=0,vy=vy,contact=contact,facing=facing);run('linear',f'linear-geometry-{seed}')
            for seed,(phase,remaining,contact,facing) in enumerate(itertools.product([0,1,63,64,511,704,960,1023,65535],[0,1,2,24,65535],[0,8,16],[0,1,2,255])):
                setup('wave',seed,phase=phase,remaining=remaining,contact=contact,facing=facing,left=600,right=-1);run('wave',f'wave-boundary-{phase}-{remaining}-{contact}-{facing}')
            for phase in range(1024):setup('wave',phase,phase=(phase-64)&65535,remaining=2,facing=phase%4);run('wave',f'wave-table-{phase}')
            for seed,(origin,oldx,vy,facing) in enumerate(itertools.product([-0x80000000,-1,0,0x7fffffff],[-0x80000000,0x7fffffff],[-0x80000000,-1,0,0x7fffffff],[0,1,2,255])):
                setup('wave',seed,vx=origin,x=oldx,vy=vy,facing=facing);run('wave',f'wave-origin-{seed}')
            for seed,(role,size) in enumerate(itertools.product(['linear','wave'],[0,31,271,*range(16,257,16)])):
                setup(role,seed,size=size,contact=8);run(role,f'clear-{role}-{size}')
            boundary_count=len(isolated)
            configs=[]
            for role,pose in [('beam',0xa3),('linear',0xa2),('wave',0xa0),('wave',0xa1)]:
                for facing,pool in itertools.product([0,1],['empty','full']):configs.append((role,pose,facing,pool))
            for stream,(role,pose,facing,pool) in enumerate(configs):
                parent_setup(role,stream,facing,pool,pose);record('initial-record');initial=snapshot(ref.ram_cpu());calls=[]
                for step in range(1024):
                    rows=[]
                    if ref.ram_cpu()[0x845d]=={'beam':0x1a,'linear':0x1b,'wave':0x1c}[role]:
                        rows.append(run('clock',f'{stream}/{step}-clock',0x8400));rows.append(run('creator_'+role,f'{stream}/{step}-parent',0x8400));rows.append(record(f'{stream}/{step}-record'))
                    for offset in [0x100,0x200,0x300]:
                        ram=ref.ram_cpu();address=0x8400+offset
                        if n(ram,address+4)==0:continue
                        behavior=n(ram,address,4);childrole=CHILD[behavior];rows.append(run(childrole,f'{stream}/{step}-{childrole}',address))
                    assert rows;calls.extend(rows);ram=ref.ram_cpu()
                    if ram[0x845d]!={'beam':0x1a,'linear':0x1b,'wave':0x1c}[role]and all(n(ram,0x8504+i*256)==0 for i in range(3)):break
                else:raise AssertionError(('source1 flow did not finish',role,pose,facing,pool))
                streams.append({'name':f'{role}-{pose}-{facing}-{pool}','initial':initial,'calls':calls})

    collections=[(args.prefix+'-oracle',natural,'Actual source1 callbacks under the declared real-menu input flow')]
    if not args.natural_only:collections += [(args.prefix+'-boundaries',isolated[:boundary_count],'Complete original linear/wave callbacks, timing, wave table and clear bounds'),(args.prefix+'-sequences',streams,'Real parent clock/creator plus all reserved child lifecycles from one initial state')]
    for name,rows,scope in collections:
        raw=(json.dumps({'library':list(library.values()),'calls':rows},separators=(',',':'))+'\n').encode();packed=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(packed)
        calls=[r for stream in rows for r in stream['calls']]if name.endswith('sequences')else rows
        report={'pass':bool(rows)and not errors,'baseline_verified':frames==expected,'scope':scope,'identity':identity,'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),
                'records':len(rows),'calls':len(calls),'kind_counts':dict(Counter(r['kind']for r in calls)),'library_frames':len(library),'fixture_sha256':sha(raw),'gzip_sha256':sha(packed),'baselines':baselines,'binding':binding,
                'frames_match_original_core':frames==expected,'rng_calls':sum(r['rng_calls']for r in calls),'external_queue_calls':sum(bool(r['external_queue_clears'])for r in calls),
                'external_change_calls':sum(bool(r['external_changes'])for r in calls),'unexplained_change_calls':sum(bool(r['unexplained_changes'])for r in calls),'interrupt_calls':sum(len(r['interrupts'])for r in calls),
                'clears':sum(r['cleared_bytes']!=0 for r in calls),'errors':errors,'incomplete_calls':0,'wave_table':{'source':'0x34000 signed-byte stride2','existing_arena_sha256':sha(arena_path.read_bytes()),'existing_wave_x_equal':True},
                'limits':['Zero natural callbacks remain pass:false, not renamed private evidence','Original clock/phase/callback stages and readonly Duration/Flags refresh only; no full tail/world/collision/activeFX simulation','No expected-state or RNG injection within a stream']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'name':name,'records':len(rows),'calls':len(calls),'kinds':report['kind_counts'],'pass':report['pass']}))
    missing=[row for row in library.values()if row['key']['animation_set']==0xd92ca and row['key']['shape_set']==0xd9a3a and row['key']['pose']in[10,11]]
    if not args.natural_only:assert len(missing)==8
    missing.sort(key=lambda r:(r['key']['pose'],r['key']['index']))
    data=(json.dumps({'animations':[{'key':r['key'],'data':r['data']}for r in missing]},separators=(',',':'))+'\n').encode()
    (out/'fighter1-projectile-rules.json').write_bytes(data)
    (out/'fighter1-projectile-rules-manifest.json').write_text(json.dumps({'pass':len(missing)==8,'scope':'Only missing public pose10/11 typed animation/shape data; no new wave table or schema','animations':len(missing),'poses':[10,11],
        'rules_bytes':len(data),'rules_sha256':sha(data),'capture_sha256':sha(Path(__file__).read_bytes()),'existing_wave_x_equal':True,'existing_arena_sha256':sha(arena_path.read_bytes()),'rom_sha256':identity['rom_sha256']},indent=2)+'\n')


if __name__=='__main__':main()
