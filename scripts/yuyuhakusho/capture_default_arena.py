#!/usr/bin/env python3
"""Complete default arena/camera, palette actors and fade rule evidence."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

ENTRIES = {0x20918: 'arena', 0x21556: 'camera', 0x150ca: 'sun', 0x1525e: 'moon', 0x15330: 'spark', 0x2300: 'progress', 0x233c: 'colors'}
DRAW_EXITS = {0x150d8: 'sun', 0x152c0: 'moon', 0x1542a: 'spark'}
IDS = {'arena': 0x8180, 'camera': 0x8180, 'sun': 0x9c80, 'moon': 0x9d00, 'spark': 0x9d80, 'progress': 0x8180, 'colors': 0x8180}


def num(raw, off, n, signed=False): return int.from_bytes(raw[off:off+n], 'big', signed=signed)


def snapshot(ram):
    return {'arena': ram[0x8180:0x8200].hex(), 'sky': ram[0x9c80:0x9e00].hex(),
            'camera': ram[0xa00e:0xa01c].hex(), 'samples': ram[0xa800:0xa880].hex(),
            'prefix': ram[0xe600:0xe800].hex(), 'scroll_a': ram[0xe800:0xea00].hex(), 'scroll_b': ram[0xea00:0xec00].hex(),
            'pan_a': ram[0x6000:0x61c0].hex(), 'pan_b': ram[0x6200:0x63c0].hex(),
            'source_a': num(ram, 0xf598, 4, True), 'source_b': num(ram, 0xf59c, 4, True),
            'vertical': ram[0xe500:0xe540].hex(), 'scroll_mode': ram[0xf6e7],
            'palette_registers': ram[0xf500:0xf508].hex(), 'palettes': ram[0xe000:0xe180].hex(),
            'seed': num(ram, 0xff18, 4), 'pause': ram[0xf550], 'slow': ram[0xf55a],
            'shake': num(ram, 0xf566, 2, True), 'pan': num(ram, 0xf572, 2, True)}


def extract_library(rom):
    sequence = []
    for i in range(64):
        parameter, ticks = rom[0x15432+i*2:0x15434+i*2]
        sequence.append({'parameter': parameter*2, 'ticks': ticks})
        if ticks == 0: break
    assert len(sequence) == 17 and sequence[-1]['ticks'] == 0
    return {'wave_x': [num(rom, 0x34000+i*2, 1, True) for i in range(1024)],
            'wave_y': [num(rom, 0x34200+i*2, 1, True) for i in range(1024)],
            'tails': {str(x): [num(rom, 0x24000+(x-0x2e0)*256+0x50+i*2, 2, True) for i in range(56)] for x in range(744, 985)},
            'color_targets': [[num(rom, base+i*2, 2) for i in range(16)] for base in [0x28acf2, 0x28ad12, 0x28ad32, 0x28ad72]],
            'fade_adjust': [[num(rom, 0x23b6+row*32+delta, 2, True) for delta in range(-14, 15, 2)] for row in range(8)],
            'spark_sequence': sequence}


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', required=True, type=Path); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'; bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = set(ENTRIES)|set(DRAW_EXITS)|{0x3556, 0x3574, 0x3d8, 0x3982}
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes(); library = extract_library(rom)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, streams, errors = [], [], [], []
    pending = {}; private = False; active_kind = ''; label = ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        def configure():
            pcs = (C.c_uint*len(points))(*sorted(points)); assert bridge.yyh_set_observer(observe, pcs, len(points))
        def finish(key, draw=False, deleted=False):
            row = pending.pop(key); row.update(after=snapshot(ref.ram_cpu()), draw=draw, deleted=deleted, exit_frame=ref.frame)
            old_sky, new_sky = bytes.fromhex(row['before']['sky']), bytes.fromhex(row['after']['sky'])
            row['spawned'] = row['kind'] == 'moon' and num(old_sky, 260, 2) == 0 and num(new_sky, 260, 2) == 128
            if private:
                row['name'] = label; isolated.append(row); ref.cpu.cycles = 4000000
            else: natural.append(row)
        @cbtype
        def observe(kind, width, pc, value):
            try:
                sp = ref.get_register(15); address = ref.get_register(13)&0xffff
                if pc == 0x3982:
                    for row in pending.values(): row['rng_calls'] += 1
                    return
                # Full helper returns are observed after the actual RTS, even
                # when the function tail-jumps to the shared copy routine.
                for key, row in list(pending.items()):
                    if row['kind'] not in ['sun', 'moon', 'spark'] and pc == row['return_pc'] and sp == row['stack']+4:
                        finish(key)
                if pc in ENTRIES:
                    name = ENTRIES[pc]
                    if private and name != active_kind: return
                    if name in ['sun', 'moon', 'spark'] and address != IDS[name]: return
                    key = (name, sp); assert key not in pending, ('Nested arena function', key)
                    ram = ref.ram_cpu(); ret = num(ram, sp&0xffff, 4)&0xffffff
                    pending[key] = {'kind': name, 'frame': ref.frame, 'object_id': address, 'stack': sp, 'return_pc': ret,
                                    'before': snapshot(ram), 'rng_calls': 0, 'cleared_bytes': 0}
                    if name not in ['sun', 'moon', 'spark'] and ret not in points:
                        points.add(ret); configure()
                elif pc in DRAW_EXITS:
                    key = (DRAW_EXITS[pc], sp)
                    if key in pending: finish(key, draw=True)
                elif pc == 0x3556:
                    key = ('spark', sp)
                    if key in pending:
                        size = num(ref.ram_cpu(), address+4, 2); clear = ((size or 64)>>4)*16
                        assert 0 < clear <= 128; pending[key]['cleared_bytes'] = clear
                elif pc == 0x3574 and ('spark', sp) in pending:
                    finish(('spark', sp), deleted=True)
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 4000000
        configure(); hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for f in range(1, 3601):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400: checkpoint = ref.save()
        hook(None); assert frames == expected and not pending and not errors, (errors, list(pending))
        private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)
        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b
        def data(address, raw):
            for i, b in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b
        def setup(seed=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15): setreg(reg, 0)
            for base, size in [(0x8180, 128), (0x9c80, 384), (0xe500, 64), (0xe600, 1536), (0x6000, 1024)]:
                data(0xff0000+base, bytes((i*73+seed*29+17)&255 for i in range(size)))
            write(0xfff550, 0); write(0xfff55a, 0); write(0xfff566, seed%5, 2); write(0xfff572, 0, 2)
            write(0xffa00e, 0, 2); write(0xffa014, 864, 2); write(0xffa016, 16, 2)
            write(0xffa010, 880, 2); write(0xffa012, 1168, 2); write(0xffa018, 864, 2); write(0xffa01a, 16, 2)
            for i in range(16): write(0xffa800+i*4, ((100+i*3)<<16)|0x1234, 4); write(0xffa840+i*4, ((320-i*2)<<16)|0xabcd, 4)
            for base, behavior in [(0x9c80, 0x150ca), (0x9d00, 0x1525e), (0x9d80, 0x15330)]:
                write(0xff0000+base, behavior, 4); write(0xff0000+base+4, 128, 2); write(0xff0000+base+6, 2, 2)
                write(0xff0000+base+0x48, 0x2000000|seed%65536, 4); write(0xff0000+base+0x50, 0x15432, 4)
                write(0xff0000+base+0x59, seed%16); write(0xff0000+base+0x5b, seed%3)
            for i in range(192): write(0xffe000+i*2, (seed*37+i*131)&65535, 2)
            write(0xfff500, 1); write(0xfff501, 0); write(0xfff502, seed%16, 2); write(0xfff504, 0x100, 2); write(0xfff506, 0xfff, 2)
            write(0xffff18, [0, 1, 256, 0xffffffff][seed%4], 4)
        def invoke(name, case):
            nonlocal active_kind, label
            active_kind, label = name, case; setreg(17, 0x2700); setreg(15, 0xffff00); write(0xffff00, 0x3d8, 4)
            setreg(13, 0xffff0000|IDS[name]); setreg(16, next(pc for pc, k in ENTRIES.items() if k == name))
            ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896; count = len(isolated); before = ref.ram_cpu()
            hook(observer); execute(2000000); hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (name, case, errors, list(pending))
            after = ref.ram_cpu()
            allowed = set(range(0x8180, 0x8200))|set(range(0x9c80, 0x9e00))|set(range(0xe000, 0xec00))|set(range(0x6000, 0x6400))|set(range(0xa00e, 0xa01c))|set(range(0xf500, 0xf508))|set(range(0xf598, 0xf5a0))|{0xf6e7}|set(range(0xff18, 0xff1c))|set(range(0xfed0, 0xff04))
            unknown = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unknown, (name, case, unknown)
            return isolated[-1]
        for seed, (used, x, y, samples) in enumerate(itertools.product([0, 4, 8, 64], [-32768, 0, 744, 864, 984, 32767], [-32768, 0, 16, 32, 32767], [0, 1, 2])):
            setup(seed); write(0xffa00e, used, 2); write(0xffa014, x, 2); write(0xffa016, y, 2)
            for i in range(16):
                write(0xffa800+i*4, ([0x7fff1234, 0x8000abcd, (100+i)<<16][samples]), 4)
                write(0xffa840+i*4, ([0xffffabcd, 0x80005678, (320-i)<<16][samples]), 4)
            invoke('camera', f'camera-{used}-{x}-{y}-{samples}')
        for seed, (stage, pause, slow, pan, x, y, used) in enumerate(itertools.product([0, 2], [0, 128], [0, 1], [0, 1, -32768], [744, 864, 984], [0, 16, 32], [0, 64])):
            setup(seed); write(0xff8186, stage, 2); write(0xfff550, pause); write(0xfff55a, slow); write(0xfff572, pan, 2)
            write(0xffa014, x, 2); write(0xffa016, y, 2); write(0xffa00e, used, 2)
            invoke('arena', f'arena-{stage}-{pause}-{slow}-{pan}-{x}-{y}-{used}')
        for name in ['sun', 'moon']:
            for seed, (stage, pause, slow, pan, phase, status) in enumerate(itertools.product([0, 2], [0, 128], [0, 1], [0, 1], [0, 0x1000000, 0x1800000, 0x2000000, 0x2800000, 0x3800000, 0xffffffff], [0, 1, 128])):
                setup(seed); base = IDS[name]; write(0xff0000+base+6, stage, 2); write(0xff0000+base+0x48, phase, 4)
                write(0xfff550, pause); write(0xfff55a, slow); write(0xfff572, pan, 2); write(0xfff501, status)
                write(0xff9d84, 0 if seed%2 else 128, 2)
                call = invoke(name, f'{name}-{stage}-{pause}-{slow}-{pan}-{phase}-{status}')
                if name == 'moon' and stage != 0 and pause < 128 and slow == 0 and phase >> 16 > 1023:
                    call['reference_only'] = 'Invalid persistent moon phase reads outside decoded wave table'
        for seed, (stage, pause, slow, index, ticks, size) in enumerate(itertools.product([0, 2], [0, 128], [0, 1], [0, 15, 255], [0, 1, 2, 255], [0, 16, 32, 48, 64, 80, 96, 112, 128])):
            setup(seed); write(0xff9d86, stage, 2); write(0xff9dd9, index); write(0xff9ddb, ticks); write(0xff9d84, size, 2)
            write(0xfff550, pause); write(0xfff55a, slow)
            invoke('spark', f'spark-{stage}-{pause}-{slow}-{index}-{ticks}-{size}')
        for phase,busy,rng,pan in itertools.product([383,384,512,640,641],[0,128],[0,1,256,0xffffffff],[0,1]):
            setup(0);write(0xff9d48,phase<<16,4);write(0xff9d84,busy,2);write(0xffff18,rng,4);write(0xfff572,pan,2)
            invoke('moon',f'moon-spawn-{phase}-{busy}-{rng}-{pan}')
        for seed, (status, mode, step, target) in enumerate(itertools.product([0, 1, 2, 128, 255], [0, 1, 2, 14, 255, 0xfff, 65535], [0, 1, 256, 65535], [0, 2, 255, 0xfff, 65535])):
            setup(seed); write(0xfff501, status); write(0xfff502, mode, 2); write(0xfff504, step, 2); write(0xfff506, target, 2)
            invoke('progress', f'progress-{status}-{mode}-{step}-{target}'); invoke('colors', f'colors-{status}-{mode}-{step}-{target}')
        # MOVE.B reads the high byte at F502/F506. Exercise every interpolation
        # row independently with dirty reserved/odd color bits, not only the
        # normal fade's restricted RGB inputs or modes below 0x100.
        for seed,(status,mode_byte,target_byte,fraction) in enumerate(itertools.product([0,1,2,3,128,129],list(range(16))+[127,255],[0,15,255],[0,255])):
            setup(seed);write(0xfff501,status);write(0xfff502,(mode_byte<<8)|fraction,2);write(0xfff506,(target_byte<<8)|(255-fraction),2)
            invoke('colors',f'color-byte-{status}-{mode_byte}-{target_byte}-{fraction}')
        boundaries = len(isolated)
        for name, count, seed in [('arena', 160, 0), ('sun', 260, 1), ('moon', 260, 2), ('spark', 80, 3), ('progress', 40, 4)]:
            setup(seed)
            if name == 'arena': write(0xff8186, 0, 2)
            if name in ['sun', 'moon', 'spark']: write(0xff0000+IDS[name]+6, 0, 2)
            if name == 'progress': write(0xfff501, 128); write(0xfff502, 0, 2); write(0xfff504, 256, 2); write(0xfff506, 0xfff, 2)
            calls = []
            for i in range(count):
                external = None
                if name == 'arena':
                    external = {'samples_x': [((900+(i%16))<<16)|0x1234]*4, 'samples_y': [320<<16|0x5678]*4}
                    for j, value in enumerate(external['samples_x']): write(0xffa800+j*4, value, 4)
                    for j, value in enumerate(external['samples_y']): write(0xffa840+j*4, value, 4)
                    write(0xffa00e, 16, 2)
                call = invoke(name, f'stream-{name}/{i}'); call['external_input'] = external; calls.append(call)
                if name == 'spark' and call['deleted']: break
                if name == 'progress': calls.append(invoke('colors', f'stream-colors/{i}'))
            streams.append({'kind': name, 'initial': calls[0]['before'], 'calls': calls})
        setup(0);write(0xff8186,0,2);write(0xff9c86,0,2);write(0xff9d48,512<<16,4);write(0xff9d84,0,2);write(0xffff18,256,4)
        calls=[]
        for i in range(180):
            external={'samples_x':[(900+i%16)<<16]*4,'samples_y':[320<<16]*4}
            for j,value in enumerate(external['samples_x']):write(0xffa800+j*4,value,4)
            for j,value in enumerate(external['samples_y']):write(0xffa840+j*4,value,4)
            write(0xffa00e,16,2)
            for kind in ['arena','sun','moon','spark','progress','colors']:
                if kind=='spark' and num(ref.ram_cpu(),0x9d84,2)==0:continue
                call=invoke(kind,f'combined/{i}/{kind}')
                if kind=='arena':call['external_input']=external
                calls.append(call)
        streams.append({'kind':'combined','initial':calls[0]['before'],'calls':calls})
        identity = ref.identity
    runtime = {'schema': 1, 'rom_sha256': identity['rom_sha256'], 'library': library}
    (out/'arena-runtime-rules.json').write_text(json.dumps(runtime, separators=(',', ':'))+'\n')
    for name, records in [('default-arena-oracle', natural), ('default-arena-boundaries', isolated[:boundaries]), ('default-arena-sequences', streams)]:
        payload = {'library': library, 'calls': records}; raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode(); zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        calls = [c for s in records for c in s['calls']] if name.endswith('-sequences') else records
        report = {'scope': 'Default arena/camera/sky rules and complete palette step/color functions; sprite1580 excluded', 'identity': identity,
                  'records': len(records), 'calls': len(calls), 'kind_counts': dict(Counter(c['kind']for c in calls)), 'draws': sum(c['draw']for c in calls), 'deleted': sum(c['deleted']for c in calls),
                  'reference_only': sum(bool(c.get('reference_only'))for c in calls), 'spawned': sum(c['spawned']for c in calls),
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped),
                  'frames_match_original_core': frames == expected, 'incomplete_calls': 0, 'errors': errors, 'pass': bool(records)and not errors}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__': main()
