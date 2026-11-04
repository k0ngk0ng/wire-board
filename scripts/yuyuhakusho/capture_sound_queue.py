#!/usr/bin/env python3
"""Complete original sound-priority consumption; no audio synthesis emulation."""
import argparse
import ctypes as C
import gzip
import itertools
import json
from collections import Counter
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

ANIMATION_RETURNS = {0x1edaa, 0x21970, 0x21982, 0x21994, 0x219a6, 0x219aa}


def snapshot(ram):
    return {'priority': ram[0xf800], 'selected': ram[0xf809], 'queue': list(ram[0xf80a:0xf80e]),
            'bytes': ram[0xf800:0xf900].hex()}


def main():
    p = argparse.ArgumentParser(); p.add_argument('--out', required=True, type=Path); args = p.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    filter_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(filter_path)); cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]; bridge.yyh_set_observer.restype = C.c_int
    points = [0x37c44, 0x37c8e, 0x37cb2, 0x1eb40]+sorted(ANIMATION_RETURNS); pcs = (C.c_uint*len(points))(*points)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    table = list(rom[0x3a50e:0x3a50e+255]); assert len(table) == 255
    library = {'address': '0x3a50e', 'index_rule': 'nonzero byte code minus one, index0..254',
               'rom_sha256': sha(rom), 'bytes_sha256': sha(bytes(table)), 'priorities': table}
    (out/'sound-queue-priorities.json').write_text(json.dumps(library, indent=2)+'\n')
    natural, isolated, streams, errors = [], [], [], []
    pending = None; private = False; label = ''; active_animation = {}
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                sp = ref.get_register(15)
                if pc == 0x1eb40:
                    if not private:
                        key = (ref.get_register(13), sp)
                        assert key not in active_animation
                        active_animation[key] = {'frame': ref.frame, 'actor': hex(key[0]), 'stack': sp, 'entry_sr': ref.get_register(17)}
                    return
                if pc in ANIMATION_RETURNS:
                    if not private: active_animation.pop((ref.get_register(13), sp), None)
                    return
                if pc == 0x37c44:
                    assert pending is None, 'Nested sound-priority consumer'
                    ram = ref.ram_cpu(); ptr = sp&0xffff
                    row = {'frame': ref.frame, 'before': snapshot(ram), 'entry_sr': ref.get_register(17), 'stack': sp,
                           'return_address': hex(int.from_bytes(ram[ptr:ptr+4], 'big')),
                           'interrupted_animation_calls': list(active_animation.values())}
                    pending = (row, ram)
                elif pc in (0x37c8e, 0x37cb2):
                    assert pending is not None, 'Sound consumer exit without entry'
                    row, before = pending; pending = None; after = ref.ram_cpu()
                    assert row['stack'] == sp
                    allowed = {0xf800, 0xf809, 0xf80a, 0xf80b, 0xf80c, 0xf80d}
                    unknown = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
                    assert not unknown, ('Unexpected consumer writes', unknown)
                    row.update(after=snapshot(after), exit=hex(pc), exit_frame=ref.frame)
                    if private:
                        row['name'] = label; isolated.append(row); ref.cpu.cycles = 200000
                    else: natural.append(row)
            except Exception as exc:
                errors.append(str(exc))
                if private: ref.cpu.cycles = 200000
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p); observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer); frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400: checkpoint = ref.save()
        hook(None)
        assert frames == expected and pending is None and not active_animation and not errors and natural, (len(natural), errors, active_animation)
        private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def setup(priority, selected, queue, seed=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for i in range(256): write(0xfff800+i, (i*73+seed*29+17)&255)
            write(0xfff800, priority); write(0xfff809, selected)
            for i, code in enumerate(queue): write(0xfff80a+i, code)

        def invoke(name, seed=0):
            nonlocal label
            label = name; setreg(17, 0x2700); setreg(15, 0xffffe0); write(0xffffe0, 0x3d8, 4)
            for reg in range(8): setreg(reg, (seed+reg)*0x1020304&0xffffffff)
            count = len(isolated); setreg(16, 0x37c44); ref.cpu.cycles = 0; ref.cpu.refresh_cycles = 896
            hook(observer); execute(100000); hook(None)
            assert len(isolated) == count+1 and pending is None and not errors, (name, errors)
            return isolated[-1]

        for priority in range(256):
            setup(priority, (priority*29)&255, [0]*4, priority); invoke(f'empty/priority-{priority}', priority)
        for code, priority in enumerate(table, 1):
            values = {0, 1, 127} if priority == 255 else {0, 127, priority&127, max(0, (priority&127)-1), min(127, (priority&127)+1)}
            current_values = sorted(values | {value|128 for value in values})
            for current, slot in itertools.product(current_values, range(4)):
                seed = code*31+current*7+slot; queue = [0]*4; queue[slot] = code
                setup(current, seed&255, queue, seed); invoke(f'code-{code}/current-{current}/slot-{slot}', seed)
        by_priority = {}
        for code, priority in enumerate(table, 1): by_priority.setdefault(priority, []).append(code)
        representatives = [0]+[by_priority[p][0] for p in sorted(by_priority)]
        for i, queue in enumerate(itertools.product(representatives, repeat=4)):
            current = [0, 127, 128, 255][i%4]; setup(current, i&255, queue, i); invoke(f'classes-{i}/current-{current}', i)
        for priority, codes in by_priority.items():
            if len(codes) < 2: continue
            for slots, order, current in itertools.product(itertools.combinations(range(4), 2), [0, 1], [0, priority&127, priority|128, 127, 255]):
                queue = [0]*4; queue[slots[0]], queue[slots[1]] = (codes[0], codes[-1]) if order == 0 else (codes[-1], codes[0])
                seed = priority+slots[0]*13+slots[1]*7+current
                setup(current, 0xa5, queue, seed); invoke(f'ties-{priority}/slots-{slots}/order-{order}/current-{current}', seed)
        independent_count = len(isolated)
        for i in range(16):
            codes = by_priority[255]
            queue = [codes[(i+j*37)%len(codes)] if (i+j)%3 else representatives[(i+j)%len(representatives)] for j in range(4)]
            setup([0, 80, 129, 255][i%4], 0xa5, queue, i); calls = []
            for step in range(8): calls.append(invoke(f'stream-{i}/step-{step}', i+step))
            streams.append({'name': f'stream-{i}', 'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity
    for name, records, scope in [('sound-queue-oracle', natural, 'Natural complete sound-priority consumer'),
                                  ('sound-queue-boundaries', isolated[:independent_count], 'Declared originalCPU complete consumer boundary calls'),
                                  ('sound-queue-sequences', streams, 'Single-initial-state originalCPU consumer streams')]:
        calls = [c for s in records for c in s['calls']] if name.endswith('-sequences') else records
        payload = {'priorities': table, 'calls': records}; raw = (json.dumps(payload, separators=(',', ':'))+'\n').encode(); compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        report = {'scope': scope, 'identity': identity, 'records': len(records), 'calls': len(calls), 'capture_sha256': sha(Path(__file__).read_bytes()),
                  'filter_sha256': sha(filter_path.read_bytes()), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed),
                  'priority_data_sha256': sha(bytes(table)), 'frames_match_original_core': frames == expected,
                  'nonempty_calls': sum(any(c['before']['queue']) for c in calls), 'special_branch_calls': sum(c['exit'] == '0x37cb2' for c in calls),
                  'interrupted_animation_calls': sum(bool(c['interrupted_animation_calls']) for c in calls),
                  'exit_counts': dict(Counter(c['exit'] for c in calls)), 'incomplete_calls': 0, 'errors': errors, 'pass': bool(records) and not errors,
                  'limits': ['Full priority consumer only, not its scheduler, playback or sound synthesis',
                             'All255 nonzero byte codes are indexable inputs, not255 established playable sounds']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__': main()
