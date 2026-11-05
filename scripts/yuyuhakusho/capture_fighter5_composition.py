#!/usr/bin/env python3
"""Complete source5 actor, nested action/input, and bounded private streams."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import hashlib
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_action import snapshot as action_snapshot, ACTION_POSES
from capture_chu_actor import snapshot as actor_snapshot, view as actor_view
from capture_chu_input import snapshot as input_snapshot, route as input_route
from capture_animation_records import selected_data

FIGHTERS = [0x8400, 0x8800, 0x8c00, 0x9000]
POSES = {k: v for k, v in ACTION_POSES.items() if k <= 13}
POSES.update({0x0f:0x0b,0x11:0x0c,0x2d:0xa8,0x2e:0xa4,0x2f:0xa0,0x30:0xac,0x31:0xa3,0x32:0xa0})


def n(raw, offset, size=2, signed=False):
    return int.from_bytes(raw[offset:offset + size], 'big', signed=signed)


class Sink:
    def __init__(self, path, kind):
        self.path, self.kind = path, kind
        self.file = path.open('w')
        self.count, self.external, self.threat = 0, 0, 0
        self.actions, self.routes, self.rng, self.sources = Counter(), Counter(), Counter(), Counter()

    def add(self, row):
        self.file.write(json.dumps(row, separators=(',', ':')) + '\n')
        self.count += 1
        self.external += bool(row['external_queue_clears'])
        self.threat += row.get('threat_branches', 0)
        self.rng[row['rng_calls']] += 1
        self.sources[row['source_register']] += 1
        if self.kind == 'action':
            self.actions[row['before']['tail']['body']['action_kind']] += 1
        elif self.kind == 'actor':
            self.actions[row['before']['action']['tail']['body']['action_kind']] += 1
        else:
            self.routes[row['route']] += 1

    def close(self):
        self.file.close()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--inputs', type=Path, default=ROOT/'docs/research/md-yuyuhakusho/source5-next-inputs.json')
    parser.add_argument('--baseline', type=Path, default=ROOT/'games/yuyuhakusho/engine/testdata/fighter5-attacks-reference-frames.json')
    parser.add_argument('--baseline-manifest', type=Path, default=ROOT/'games/yuyuhakusho/engine/testdata/fighter5-attacks-reference-manifest.json')
    parser.add_argument('--natural-label', choices=['natural','special'], default='natural')
    parser.add_argument('--natural-only', action='store_true')
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    input_path,reference_path,reference_manifest_path=args.inputs.resolve(),args.baseline.resolve(),args.baseline_manifest.resolve()
    assert all(p.is_relative_to(ROOT)for p in [input_path,reference_path,reference_manifest_path])
    reference_manifest=json.loads(reference_manifest_path.read_text())
    compressed=reference_path.read_bytes()
    reference_raw=gzip.decompress(compressed)if reference_path.suffix=='.gz'else compressed
    if 'unobserved_identity'in reference_manifest:
        assert reference_manifest['pass'] and reference_manifest['frames_match_original_core']
        assert sha(input_path.read_bytes())==reference_manifest['inputs_sha256']
        assert sha(reference_raw)==reference_manifest['fixture_sha256'] and sha(compressed)==reference_manifest['gzip_sha256']
        assert reference_manifest['unobserved_identity']['core_sha256']=='1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
    else:
        assert sha(input_path.read_bytes())==reference_manifest['input']['sha256']
        assert sha(reference_raw)==reference_manifest['output']['sha256'] and reference_manifest['observer_attached']is False
    expected,events=json.loads(reference_raw),json.loads(input_path.read_text())
    entry_limit=min(len(expected),3600)
    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT / '.local/md-yuyuhakusho/tools/hook_filter.dylib'
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    bridge = C.CDLL(str(bridge_path))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = [0x12682,0x126ba,0x15540,0x1269a,0x15a18,0x15b78,0x15b82,0x3d8,0x3982,
              0x1cc8a,0x15594,0x14922,0x156f0,0x208d8,0x15772,0x37c56,0x37cac]
    pcs = (C.c_uint * len(points))(*points)
    sinks = {(group, kind): Sink(out / (group + '-' + kind + '.jsonl'), kind)
             for group in [args.natural_label, 'boundaries'] for kind in ['action', 'input', 'actor']}
    pending, library, errors, completed = {}, {}, [], {}
    library_groups = {args.natural_label: {}, 'boundaries': {}}
    private, collect, target, label = False, True, '', ''
    stream_rows = {'action': [], 'input': [], 'actor': []}

    def snapshot(kind,ram,address):
        if kind=='action':
            value=action_snapshot(ram,address,rom);value['global_a038']=n(ram,0xa038)
            return value
        if kind=='input':return input_snapshot(ram,address,rom,stream_base=0xff6000)
        value=actor_snapshot(ram,address,rom);value['action']['global_a038']=n(ram,0xa038)
        return value

    with ReferenceSession(ROOT, out / 'observed', core=core) as ref:
        def finish(kind, ram, address):
            row = pending.pop(kind)
            row.update(after=snapshot(kind, ram, address), exit_frame=ref.frame)
            if kind == 'action':
                assert row['rng_calls'] == 1 and 'animation_key' in row
            completed[kind] = row
            if not private or collect:
                sinks[('boundaries' if private else args.natural_label, kind)].add(row)
            if private and kind == target:
                ref.cpu.cycles = 8000000

        @callback_type
        def observe(hook_kind, width, pc, value):
            try:
                address, sp = ref.get_register(13) & 0xffffff, ref.get_register(15)
                if pc in [0x37c56, 0x37cac]:
                    a = ref.get_register(9) & 65535
                    for row in pending.values():
                        row['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a], 'sr': ref.get_register(17), 'sp': sp})
                    return
                if pc == 0x3982:
                    for row in pending.values():
                        row['rng_calls'] += 1
                    return
                if pc in [0x1cc8a, 0x15594]:
                    for row in pending.values():
                        row['ai_calls' if pc == 0x1cc8a else 'record_calls'] += 1
                if 'actor' in pending and pc in [0x14922,0x15540,0x15a18,0x156f0,0x208d8,0x15772]:
                    stages = pending['actor']['stages']
                    stages[hex(pc)] = stages.get(hex(pc), 0) + 1
                ram = ref.ram_cpu()
                if pc in [0x12682,0x15540,0x15a18] and ram[(address & 65535)+0x58] == 5 and (private or ref.frame<=entry_limit):
                    kind = {0x12682:'actor',0x15540:'input',0x15a18:'action'}[pc]
                    assert kind not in pending, ('nested same API', kind)
                    row = {'object_id': address & 65535, 'source_register': hex(ref.get_register(13)), 'entry_sp': sp,
                           'frame': ref.frame, 'scenario': 'private' if private else 'real-menu-source5', 'name': label,
                           'before': snapshot(kind, ram, address), 'external_queue_clears': [], 'rng_calls': 0,
                           'ai_calls': 0, 'record_calls': 0, 'threat_branches': 0}
                    assert ref.get_register(13) == 0xffff0000 | (address & 65535)
                    if kind == 'actor':
                        row['view'], row['stages'] = actor_view(ram), {}
                    elif kind == 'input':
                        row['route'] = input_route(row)
                    pending[kind] = row
                elif pc == 0x15b78 and 'action' in pending:
                    key, data, source = selected_data(rom, ram, ram[address & 65535:(address & 65535)+256])
                    identity = json.dumps(key, sort_keys=True)
                    assert identity not in library or library[identity]['data'] == data
                    library[identity] = {'key': key, 'data': data, 'source': source}
                    library_groups['boundaries' if private else args.natural_label][identity] = library[identity]
                    pending['action']['animation_key'] = key
                elif pc == 0x15b82 and 'action' in pending:
                    assert sp == pending['action']['entry_sp']
                    finish('action', ram, address)
                elif pc in [0x1269a,0x3d8] and 'input' in pending:
                    assert sp == pending['input']['entry_sp'] + 4
                    finish('input', ram, address)
                elif pc == 0x126ba and 'actor' in pending:
                    assert sp == pending['actor']['entry_sp']
                    finish('actor', ram, address)
            except Exception as error:
                errors.append(repr(error))
                if private:
                    ref.cpu.cycles = 8000000

        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for frame in range(1, len(expected)+1):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint, base_ram = ref.save(), ref.ram_cpu()
        hook(None)
        assert frames == expected and not errors and not pending, (errors[:5], list(pending))
        assert base_ram[0x8458] == 5
        binding = {'source_tag': 5, 'source_object': '0x8400', 'checkpoint_video_label': 2400,
                   'animation_set': n(base_ram,0x8450,4), 'shape_set': n(base_ram,0x848c,4),
                   'parameter_pointer': n(base_ram,0x8490,4), 'source_actor_sha256': sha(base_ram[0x8400:0x8500])}
        movement_keys=[('walk_speed',0,4),('vertical_velocity',4,4),('gravity',8,4),('horizontal_velocity',12,4),('burst_speed',24,4),('energy_drain',28,2)]
        movement_rules={'pass':True,'scope':'Independent source5 default immutable movement parameter data, read from the actual +90 pointer, not Go bundle',
                        'rom_sha256':sha(rom),'rules':[{'source':source,'movement':{key:n(rom,source+offset,size,True)for key,offset,size in movement_keys},
                            'source_bytes':rom[source:source+30].hex(),'source_sha256':sha(rom[source:source+30])}for source in [binding['parameter_pointer']]]}
        (out/'fighter5-composition-movement-rules.json').write_text(json.dumps(movement_rules,indent=2)+'\n')
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write_bytes(address, data):
            for i, byte in enumerate(data):
                C.c_ubyte.from_address(ref.ram_pointer + (((address+i)&65535)^1)).value = byte

        def write(address, value, size=1):
            write_bytes(address, (value & ((1 << (size*8))-1)).to_bytes(size, 'big'))

        routes = {'human':(0,0,0,0), 'ai':(0,255,0,0), 'human-record':(-1,0,0,0),
                  'ai-record':(-1,255,0,0), 'playback':(1,0,0,0), 'playback-stopped':(1,0,0,1), 'disabled':(0,255,1,0)}

        def setup(seed, owner=0x8400, action=0, route='human', **changes):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            for j, actor_id in enumerate(FIGHTERS):
                write_bytes(0xff0000+actor_id, bytes((i*73+seed*11+j*29+17)&255 for i in range(1024)))
                for offset, value, size in [(0,0x12668,4),(4,0x100,2),(6,2,2),(0x58,9,1),(0x6e,1,1),(0x78,100,2),
                                           (0x40,(100+j*45)<<16|0x1234,4),(0x44,0x1405678,4),(0x9c,0x1405678,4),
                                           (0x5c,0,1),(0x64,0,1),(0xb8,100<<16|0x4321,4)]:
                    write(0xff0000+actor_id+offset,value,size)
                for child in range(3):
                    for offset,value,size in [(4,0,2),(0x40,0,4),(0x44,0,4),(0x64,0,1),(0x65,0,1),(0x9c,0x1405678,4)]:
                        write(0xff0000+actor_id+256*(child+1)+offset,value,size)
            base = 0xff0000+owner
            write_bytes(base+0x50,base_ram[0x8450:0x8454])
            write_bytes(base+0x8c,base_ram[0x848c:0x8494])
            mode, channel, disabled, stopped = routes[route]
            pose,index=changes.pop('pose',POSES[action]),changes.pop('index',0)
            child_sizes=changes.pop('child_sizes',[0,0,0]);pool_free=changes.pop('pool_free',None);bank_cursor=changes.pop('bank_cursor',0);child_values={key:changes.pop(key,None)for key in ['cx','cy','child_direction','child_held','child_pressed','child_contact','child_bank']}
            obj = bytearray(ref.ram_cpu()[owner:owner+256]);obj[0x5f],obj[0x59]=pose,index
            _, metadata, _ = selected_data(rom,ref.ram_cpu(),obj)
            fields = {0x90:(n(base_ram,0x8490,4),4),0x58:(5,1),0x59:(index,1),0x5a:(metadata['duration'],1),0x5b:(0,1),0x5c:(metadata['flags'],1),0x5d:(action,1),0x5f:(pose,1),0x60:(0,1),0x61:(seed%3,1),
                      0x69:(0,1),0x6a:(seed%4,1),0x6b:(0,1),0x6c:(metadata['enabled_stages'],1),0x6d:(0,1),0x6e:(0,1),0x6f:(0,1),0x70:((owner-0x8400)//256,1),
                      0x71:(0,1),0x72:(0,1),0x73:(0,1),0x74:(0,1),0x75:(0,1),0x76:(channel,1),0xc0:(0,1),0xc1:(0,1),0xc2:(0,1),0xc7:(0,1),0xc9:(mode,1),0xca:(0xff6000,4),
                      0x78:(100,2),0x7a:(100,2),0x7c:(0,2),0x7e:(-1,2),0x80:(100,2),0x84:(129,2),0x86:(0,1),0x87:(0,1),0x88:(0,1),0x89:(0,1),0xc8:(0,1),
                      0x40:(100<<16|0x1234,4),0x44:(0x1405678,4),0x9c:(0x1405678,4),0x10:(0x48001,4),0x14:(0x38001,4),0x4c:(0x3600,4),0x48:(0xff123456,4),
                      8:(0x111,2),12:(0x222,2),0x54:(-3,2),0x56:(5,2),0x8a:(0,2),0xb8:(223<<16|0xabcd,4),0xbc:(223,2),0x98:(0,4),0xc4:(0,2),
                      0xce:(2,1),0xd0:(0,2),0xd2:(100,2),0xd4:(0,2),0xd6:(FIGHTERS[(FIGHTERS.index(owner)+1)%4],2)}
            aliases = {'ticks':0x5b,'completed':0x60,'facing':0x61,'stop':0x69,'request':0x6b,'enabled':0x6c,'held':0x71,'pressed':0x72,'changed':0x73,'raw':0x74,'health':0x78,'contact':0x89,'x':0x40,'y':0x44,'vx':0x10,'vy':0x14,'aux':0x48,'energy':0xb8,'retain':0x75}
            aliases.update({'gravity':0x4c,'power':0x7c,'ground':0x9c,'extra_jump':0x87,'flags':0x5c,'extended_held':0xc0,'extended_pressed':0xc1,'extended_changed':0xc2,'movement_source':0x90})
            if changes.get('ticks')=='end':changes['ticks']=(metadata['duration']-1)&255
            for key,value in changes.items():
                offset=aliases[key];fields[offset]=(value,fields[offset][1])
            for offset,(value,size) in fields.items():
                write(base+offset,value,size)
            write_bytes(base+0xa8,bytes(16))
            write_bytes(0xff6000,bytes([8,16,16,8,0,64,64,0])*128)
            for child,size in enumerate(child_sizes):write(base+256*(child+1)+4,size,2)
            for key,offset,size in [('cx',0x40,4),('cy',0x44,4),('child_direction',0x48,4),('child_held',0x71,1),('child_pressed',0x72,1),('child_contact',0x89,1),('child_bank',0xfe,2)]:
                if child_values[key]is not None:write(base+256+offset,child_values[key],size)
            write_bytes(0xffa200,bytes((i*19+seed*31+91)&255 for i in range(0x700)))
            for i in range(4):write(0xffa004+i*2,0,2)
            write(0xffa00c,0,2);write(0xffa00e,0,2);write(0xffa034,0,2)
            pool=bytearray((i*37+seed*23+13)&255 for i in range(2048))
            for i in range(16):pool[i*128+4:i*128+6]=(0 if pool_free is not None and i in pool_free else 128 if pool_free is not None or seed%3==2 else 0).to_bytes(2,'big')
            write_bytes(0xff9400,pool);write(0xffa036,0xff80 if seed%2 else 0,2)
            write(0xffa038,bank_cursor,2);write_bytes(0xff7000,bytes((i*23+seed*11+19)&255 for i in range(2048)));write(0xffa03a,0,2);write(0xffa042,0xa0);write(0xffa030,0x97 if seed%2 else 0)
            write(0xffa010,32,2);write(0xffa012,600,2);write(0xffa014,0,2);write(0xffa016,0,2)
            write(0xffa032,disabled);write(0xffa09a,stopped);write_bytes(0xfffffc,bytes([1,0x22,0x44,0x7f]))
            write(0xfff550,0);write(0xfff55a,0);write(0xfff566,0,2);write(0xfff568,0,2);write(0xfff56a,0,2);write(0xfff585,seed%2)
            write(0xffff12,6,2);write(0xffff18,[1,0x12345678,0xffffffff][seed%3],4)
            write_bytes(0xfff80a,bytes([0x80,0x81,0x82,0x83]if seed%2 else[0,0x80,0,0x81]))
            write_bytes(0xffd400,bytes((i*13+seed*3+7)&255 for i in range(3072)));write(0xfff518,0xd400,2)
            write_bytes(0xffe602,bytes((i*17+seed*5+9)&255 for i in range(1024)))
            for c in range(8):
                for i in range(5):write(0xfff780+c+i*8,0);write(0xfff7a8+c+i*8,0)

        def invoke(kind, owner, name):
            nonlocal target,label,completed
            target,label,completed=kind,name,{}
            setreg(17,0x2700);setreg(15,0xffffff00);write(0xffff00,0x3d8,4)
            setreg(13,0xffff0000+owner);setreg(16,{'actor':0x12682,'action':0x15a18,'input':0x15540}[kind])
            ref.cpu.cycles,ref.cpu.refresh_cycles=0,896
            before=ref.ram_cpu();hook(observer);execute(6000000);hook(None)
            assert kind in completed and not pending and not errors,(name,errors[:5],list(pending))
            after=ref.ram_cpu()
            allowed=set(range(owner,owner+1024))|set(range(0x6000,0x6400))|set(range(0xfec0,0xff04))|set(range(0xff18,0xff1c))|set(range(0xfffc,0x10000))|{0xa09a}
            if kind!='input':
                allowed|=set(range(0x9400,0x9c00))|set(range(0xa004,0xa00e))|set(range(0xa200,0xa800))|{0xa030,0xa036,0xa037,0xa038,0xa039,0xa042}|set(range(0xf568,0xf56c))|set(range(0xf80a,0xf80e))
                for other in FIGHTERS:
                    if other!=owner:allowed|={other+0xb8,other+0xb9}
            if kind=='actor':allowed|=set(range(0xa200,0xa900))|{0xa00e,0xa00f,0xa034,0xa035}|set(range(0xd400,0xe000))|set(range(0xe602,0xea02))|{0xf518,0xf519}
            unknown=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and i not in allowed]
            assert not unknown,(name,[hex(i)for i in unknown])
            completed[kind]['private_ram_audit']={'bytes_compared':65536,'changed_bytes':sum(a!=b for a,b in zip(before,after)),'unknown_writes':0,'before_sha256':sha(before),'after_sha256':sha(after),'history_7000_77ff_unchanged':before[0x7000:0x7800]==after[0x7000:0x7800]}
            assert completed[kind]['private_ram_audit']['history_7000_77ff_unchanged'],name
            return completed[kind]

        if not args.natural_only:
            seed=0
            for owner in FIGHTERS:
                for action in POSES:
                    setup(seed,owner,action,stop=0)
                    invoke('action',owner,f'common-{owner:x}-{action:x}')
                    seed+=1
            # Each profile is a declared private initial state. Complete action,
            # nested input and actor calls still run the original CPU to return.
            profiles=[(0x11,{'index':index,'ticks':'end','completed':complete,'facing':facing})for index,complete,facing in[(0,0,0),(6,0,1),(0,1,2)]]
            profiles += [(0x2d,{'completed':complete,'contact':contact})for complete,contact in[(0,0),(1,4)]]
            profiles += [(0x2e,{'pose':pose,'completed':complete,'facing':facing,'vx':vx})for pose,complete,facing,vx in[
                (0xa4,0,0,0x48001),(0xa4,1,1,-0x48001),(0xa6,0,2,0x28001),(0xa6,1,0,0)]]
            profiles += [(action,{'pose':pose,'index':index,'ticks':'end','gravity':mode<<24|0x123456,'child_sizes':children,'pool_free':free})for action,pose,index,mode,children,free in[
                (0x2f,0xa0,0,0,[0,0,0],None),(0x2f,0xa1,0,1,[0x100,0,0],[]),(0x2f,0xa2,1,2,[0,0,0],[1]),
                (0x30,0xac,0,3,[0,0,0],None),(0x30,0xac,1,3,[0x100,0,0],[]),
                (0x32,0xa0,0,0,[0,0,0],None),(0x32,0xa1,1,1,[0x100,0,0],[]),(0x32,0xa2,1,2,[0,0,0],[1])]]
            profiles += [(0x31,{'aux':counter<<16|0xabcd,'gravity':mode<<24|0x123456,'completed':complete,'child_sizes':children})for counter,mode,complete,children in[
                (0,0,0,[0,0,0]),(30,1,0,[0,0,0]),(31,2,0,[0,0,0]),(32,0,0,[0x100,0,0]),(1,1,1,[0,0,0])]]
            profiles += [(action,{'stop':stop,'completed':complete})for action,stop,complete in[(0,1,0),(0x31,2,0),(0x11,0,1),(0x30,0,1)]]
            for action,options in profiles:
                setup(seed,0x8400,action,**options);invoke('action',0x8400,f'phase-{action:x}-{seed}');seed+=1
            constructors=[
                (0,{'held':8,'pressed':64,'changed':64,'enabled':11},3),
                (0,{'request':7,'enabled':11},0x0f),(0,{'request':8,'enabled':11},0x11),
                (0,{'request':1,'enabled':8,'energy':16<<16|0xabcd},0x2e),
                (0,{'request':1,'enabled':8,'energy':15<<16|0xabcd},None),
                (0,{'request':1,'held':2,'enabled':8,'energy':16<<16|0xabcd},0x2e),
                (0,{'request':3,'changed':16,'enabled':8,'energy':8<<16|0xabcd},0x2d),
            ]
            constructors += [(0,{'request':4,'changed':button,'enabled':8,'energy':4<<16|0xabcd,'child_sizes':children},expected_action)for button,children,expected_action in[
                (64,[0,0,0],0x2f),(16,[0,0,0],0x2f),(32,[0,0,0],0x2f),(64,[0x100,0,0],None)]]
            constructors += [(0,{'request':6,'changed':button,'enabled':8,'energy':50<<16|0xabcd,'health':health,'child_sizes':children,'aux':0x1234abcd},expected_action)for button,health,children,expected_action in[
                (64,111,[0,0,0],0x31),(16,111,[0,0,0],0x31),(32,111,[0,0,0],0x31),(64,112,[0,0,0],None),(64,111,[0x100,0,0],None)]]
            constructors += [(2,{'request':4,'changed':button,'enabled':16,'energy':4<<16|0xabcd,'y':y,'vy':-1,'gravity':0},expected_action)for button,y,expected_action in[
                (64,0x1301234,0x30),(16,0x1301234,0x30),(32,0x1301234,0x30),(64,0xa01234,None)]]
            for index,(action,options,expected_action)in enumerate(constructors):
                setup(seed,0x8400,action,**options);row=invoke('action',0x8400,f'full-tail-constructor-{index}')
                if expected_action is not None:assert row['after']['tail']['body']['action_kind']==expected_action,(index,row['after']['tail']['body']['action_kind'])
                if index==0:assert row['after']['tail']['throw_offset']==32,'source5 ground throw absent'
                seed+=1
            for owner in FIGHTERS:
                for route in routes:
                    setup(seed,owner,0,route=route);invoke('actor',owner,f'route-{owner:x}-{route}');seed+=1
                for pause,slow in[(128,0),(0,1),(255,255)]:
                    setup(seed,owner,0);write(0xfff550,pause);write(0xfff55a,slow)
                    invoke('actor',owner,f'gate-{owner:x}-{pause}-{slow}');seed+=1
            for action,options in[(0x2f,{'index':0,'ticks':'end','gravity':0x1123456}),
                (0x30,{'index':0,'ticks':'end','gravity':0x3123456,'y':0x1301234}),
                (0x31,{'aux':31<<16|0xabcd,'gravity':0x2123456})]:
                setup(seed,0x8400,action,**options);invoke('actor',0x8400,f'full-actor-special-{action:x}');seed+=1
            collect=False
            streams=[(0,0,48,0),(0x0f,0x0b,160,0),(0x11,0x0c,160,0),(0x2d,0xa8,192,0),
                (0x2e,0xa4,192,0),(0x2e,0xa6,192,0),
                (0x2f,0xa0,192,0),(0x2f,0xa1,192,1),(0x2f,0xa2,192,2),
                (0x30,0xac,192,3),(0x31,0xa3,384,0),(0x31,0xa3,384,1),(0x31,0xa3,384,2),
                (0x32,0xa0,192,0),(0x32,0xa1,192,1),(0x32,0xa2,192,2)]
            motions=[0x0f,0x11,0x2d,0x2e,0x2f,0x30,0x31,0x32]
            for action,pose,limit,variant in streams:
                setup(seed,0x8400,action,pose=pose,ticks=255,vx=0x28000,vy=-0x10000 if action==0x30 else 0,
                      gravity=variant<<24|0x123456 if action in[0x2f,0x30,0x31,0x32]else 0x3600,
                      y=0x1301234 if action==0x30 else 0x1405678,aux=0xabcd if action==0x31 else 0xff123456)
                calls=[];previous=0;created=set()
                for step in range(limit):
                    held=0;changed=held^previous;external={'held':held,'pressed':held&changed,'changed':changed,'raw_held':held}
                    for field,offset in[('held',0x71),('pressed',0x72),('changed',0x73),('raw_held',0x74)]:write(0xff8400+offset,external[field])
                    row=invoke('action',0x8400,f'action-stream-{action:x}-pose{pose:x}-variant{variant}-{step}');row['external_input']=external;calls.append(row);previous=held
                    old=bytes.fromhex(row['before']['children']);new=bytes.fromhex(row['after']['children']);created.update(n(new,i*256,4)for i in range(3)if n(old,i*256+4)==0 and n(new,i*256+4)!=0)
                    current=row['after']['tail']['body']['action_kind']
                    if action!=0 and current not in motions:break
                if action!=0:assert current not in motions,('source5 full action did not exit',action,current)
                assert n(bytes.fromhex(row['after']['tail']['actor_bytes']),0x90,4)==binding['parameter_pointer'],'source5 unexpectedly changed movement source'
                stream_rows['action'].append({'initial':calls[0]['before'],'calls':calls,'created_children':[hex(v)for v in sorted(created)]});seed+=1
            for route in ['human-record','ai-record','playback']:
                setup(seed,0x8400,0,route=route);calls=[];previous=0
                for step in range(16):
                    held=8 if step<4 else 16 if step<8 else 0;changed=held^previous
                    values=[held,previous,changed,held&changed,previous&changed]
                    for i,value in enumerate(values):write(0xfff780+i*8,value)
                    row=invoke('input',0x8400,f'input-stream-{route}-{step}')
                    row['external_input']=row['before']['envelope']['input'];calls.append(row);previous=held
                stream_rows['input'].append({'initial':calls[0]['before'],'calls':calls});seed+=1
            for route in ['human','ai-record','playback','human-record']:
                setup(seed,0x8400,0,route=route);calls=[];previous=0
                for step in range(24):
                    held=8 if step<4 else 16 if 8<=step<12 else 0;changed=held^previous
                    for i,value in enumerate([held,previous,changed,held&changed,previous&changed]):write(0xfff780+i*8,value)
                    pause=128 if route=='human'and 5<=step<8 else 0;slow=1 if route=='human'and 15<=step<18 else 0
                    write(0xfff550,pause);write(0xfff55a,slow)
                    row=invoke('actor',0x8400,f'actor-stream-{route}-{step}');row['external_view']={'controllers':row['view']['controllers'],'pause':pause,'slow':slow};calls.append(row);previous=held
                stream_rows['actor'].append({'initial':calls[0]['before'],'view':calls[0]['view'],'calls':calls});seed+=1
        identity=ref.identity
        assert identity['core_sha256']=='e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
        assert identity['rom_sha256']=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
    for sink in sinks.values():sink.close()
    def emit(name,kind,records,rows,stats,group):
        selected_library = library_groups[group]
        raw_hash=hashlib.sha256();path=out/(name+'.json.gz')
        with path.open('wb')as file,gzip.GzipFile(filename='',mode='wb',fileobj=file,mtime=0)as z:
            def write(raw):raw_hash.update(raw);z.write(raw)
            write(b'{"library":'+json.dumps(list(selected_library.values()),separators=(',',':')).encode()+b',"calls":[')
            for index,row in enumerate(rows):
                if index:write(b',')
                write(row if isinstance(row,bytes)else json.dumps(row,separators=(',',':')).encode())
            write(b']}\n')
        report={'pass':True,'scope':kind+' source5 complete original API returns; actor stops before sprites','identity':identity,
                'capture_sha256':sha(Path(__file__).read_bytes()),'filter_sha256':sha(bridge_path.read_bytes()),'records':records,
                'calls':stats.get('calls',records),'frames_match_original_core':frames==expected,'external_queue_calls':stats.get('external',0),
                'fixture_sha256':raw_hash.hexdigest(),'gzip_sha256':sha(path.read_bytes()),'binding':binding,'library_frames':len(selected_library),
                'baseline_path':str(reference_path.relative_to(ROOT)),'baseline_sha256':sha(reference_path.read_bytes()),
                'baseline_manifest_sha256':sha(reference_manifest_path.read_bytes()),'input_path':str(input_path.relative_to(ROOT)),
                'input_sha256':sha(input_path.read_bytes()),'baseline_raw_sha256':sha(reference_raw),'accepted_entry_video_limit':entry_limit,'canonical_source_registers':True,'private_unknown_ram_writes':0,'incomplete_calls':0,
                'errors':errors,'statistics':stats,'limits':['Natural source5 actor is human; source5 AI is private evidence only.',
                'Actor excludes sprite, child, collision-manager and world callbacks.','Independent audio consumer exclusions require observed PC/address markers.']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report,indent=2)+'\n')
        print(json.dumps({'name':name,'records':records,'stats':stats,'pass':True}))
    for(group,kind),sink in sinks.items():
        if sink.count==0:continue
        with sink.path.open('rb')as file:
            emit('fighter5-composition-'+kind+'-'+group,kind,sink.count,(line.rstrip(b'\n')for line in file),
                 {'calls':sink.count,'external':sink.external,'threat_branches':sink.threat,'actions':dict(sink.actions),'routes':dict(sink.routes),'rng_counts':dict(sink.rng),'sources':dict(sink.sources)}, group)
    for kind,streams in stream_rows.items():
        if not streams:continue
        emit('fighter5-composition-'+kind+'-sequences',kind,len(streams),streams,{'calls':sum(len(s['calls'])for s in streams)}, 'boundaries')


if __name__=='__main__':main()
