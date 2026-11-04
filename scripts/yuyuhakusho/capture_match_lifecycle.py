#!/usr/bin/env python3
"""Complete covered battle-manager stages from original CPU execution."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

STAGES = set(range(4, 28, 2))
RETURNS = {0xf2b2, 0xf430, 0xf436, 0xf468, 0xf48e, 0xf49c, 0xf4c0, 0xf4d8,
           0xf534, 0xf546, 0xf57e, 0xf6fc, 0x3554, 0x21970, 0x21982, 0x21994, 0x219a6, 0x219aa}
FIGHTERS = [0x8400, 0x8800, 0x8c00, 0x9000]


def n(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def snapshot(ram):
    return {'world': ram[0x8040:0xa000].hex(), 'globals': ram[0xa000:0xa200].hex(),
            'palette_registers': ram[0xf500:0xf514].hex(), 'palette_current': ram[0xe000:0xe080].hex(),
            'palette_secondary': ram[0xe100:0xe180].hex(), 'pause_flags': ram[0xf550], 'pause_mask': ram[0xf551],
            'pressed': list(ram[0xf798:0xf79c]), 'queue': list(ram[0xf80a:0xf80e]),
            'stats': ram[0xff60:0x10000].hex(), 'option_ff14': ram[0xff14], 'option_ff16': ram[0xff16]}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    args = p.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    with ReferenceSession(ROOT, out/'unobserved') as ref:
        expected = [ref.step(f, buttons_at(events, f))['reference'] for f in range(1, 5501)]
    assert expected[:3600] == json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    (out/'reference-frames.json.gz').write_bytes(gzip.compress((json.dumps(expected, separators=(',', ':'))+'\n').encode(), mtime=0))
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = sorted(RETURNS|{0xf2a4, 0x101de, 0x103b8, 0x353a, 0x37c56, 0x37cac})
    pcs = (C.c_uint*len(points))(*points)
    natural, isolated, sequences, errors = [], [], [], []
    pending = None
    private, label = False, ''
    with ReferenceSession(ROOT, out/'observed', core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                sp = ref.get_register(15)
                if pc == 0xf2a4:
                    assert pending is None, 'Nested manager'
                    ram = ref.ram_cpu()
                    stage = n(ram, 0x8046, 2)
                    if stage not in STAGES and ram[0xf550] < 128 and ram[0xa096] != 2:
                        return
                    assert ref.get_register(13)&0xffff == 0x8040
                    pending = {'frame': ref.frame, 'stage': stage, 'before': snapshot(ram), 'entry_sp': sp,
                               'events': [], 'external_queue_clears': []}
                elif pc in (0x37c56, 0x37cac):
                    if pending is not None:
                        a = ref.get_register(9)&0xffff
                        pending['external_queue_clears'].append({'pc': hex(pc), 'address': hex(a), 'before': ref.ram_cpu()[a], 'sp': sp, 'sr': ref.get_register(17)})
                elif pc in (0x101de, 0x103b8, 0x353a):
                    if pending is not None:
                        pending['events'].append({0x101de: 'intro', 0x103b8: 'results', 0x353a: 'clear-world'}[pc])
                elif pc in RETURNS and pending is not None and sp == pending['entry_sp']:
                    pending.update(after=snapshot(ref.ram_cpu()), exit=hex(pc), exit_frame=ref.frame)
                    if private:
                        pending['name'] = label
                        isolated.append(pending)
                        ref.cpu.cycles = 4000000
                    else:
                        natural.append(pending)
                    pending = None
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for f in range(1, 5501):
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if f == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and pending is None and not errors, (len(natural), errors)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        def write_bytes(address, raw):
            for i, b in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = b

        life_cases = [([223, 223, 223, 223], [1, 2, 3, 4]), ([223, 223, 223, 223], [1, 1, 4, 4]),
                      ([-1, -1024, -1024, 223], [1, 1, 4, 4]), ([-1024, -1024, -1024, 223], [1, 2, 3, 4]),
                      ([-1024]*4, [1, 2, 3, 4])]

        def setup(seed, stage=14, pause=0, mode_flag=0, count=0, life=0, rule=0, grouping=2):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            write_bytes(0xff8040, bytes((i*73+seed*17+19)&255 for i in range(0x1fc0)))
            write(0xff8040, 0xf2a4, 4)
            write(0xff8044, 64, 2)
            write(0xff8046, stage, 2)
            for index, (owner, health, team) in enumerate(zip(FIGHTERS, *life_cases[life])):
                write(0xff0000+owner, 0x12668, 4)
                write(0xff0000+owner+4, 256, 2)
                write(0xff0000+owner+0x78, health, 2)
                write(0xff0000+owner+0x6f, team)
            write(0xffa032, 0x91)
            write(0xffa03a, count, 2)
            write(0xffa03c, count, 2)
            write(0xffa062, rule, 2)
            write(0xffa06a, 0, 2)
            write(0xffa074, 0, 2)
            write(0xffa07a, seed%2)
            write(0xffa07c, 15)
            write(0xffa088, grouping, 2)
            write(0xffa096, mode_flag)
            write(0xffa097, 0)
            write_bytes(0xffa100, bytes([1, 2, 3, 4, 1, 2, 3, 4]))
            write(0xffff14, 0)
            write(0xffff16, 0)
            write(0xfff550, pause)
            write(0xfff551, seed%16)
            write_bytes(0xfff798, bytes(4))
            write(0xffa030, 0x97 if seed%2 else 0)
            write_bytes(0xfff80a, bytes([0x80, 0x81, 0x82, 0x83] if seed%2 else [0, 0x80, 0, 0x81]))
            write_bytes(0xffe000, bytes((i*17+seed)&255 for i in range(128)))
            write_bytes(0xffe100, bytes((i*31+seed)&255 for i in range(128)))
            write(0xfff500, seed%256)
            write(0xfff501, 0x80 if seed%2 else 0)
            write(0xfff502, 0x2345, 2)
            write(0xfff504, 0x4567, 2)
            write(0xfff506, 0x6789, 2)
            write(0xfff512, 0x2468, 2)
            write_bytes(0xffff60, bytes([seed%256])*160)

        def invoke(name):
            nonlocal label
            label = name
            setreg(17, 0x2700)
            # Original game stack is below FF00. The usual isolated FFE0
            # trampoline would alias the actual loss-statistics table here.
            setreg(15, 0xffffff00)
            write(0xffff00, 0x3d8, 4)
            setreg(13, 0xffff8040)
            setreg(16, 0xf2a4)
            ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
            before = ref.ram_cpu()
            # Deliberately corrupt team codes can make the original taunt
            # selector read outside the decoded character/configuration data.
            # Retain these CPU results as diagnostics, never game semantics.
            reference_only = n(before, 0x8046, 2) == 20 and any(
                n(before, owner, 4) == 0x12668 and n(before, owner+0x78, 2, True) != -1024 and before[owner+0x6f] > 4
                for owner in FIGHTERS)
            count = len(isolated)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(isolated) == count+1 and pending is None and not errors, (name, errors)
            if reference_only:
                isolated[-1]['reference_only_reason'] = 'Out-of-game base-team code leads to an undecoded taunt configuration/ROM-table index; independent rules reject atomically'
            after = ref.ram_cpu()
            allowed = set(range(0x8040, 0xa200)) | set(range(0xe000, 0xe180)) | set(range(0xf500, 0xf514)) | {0xf550, 0xf551} | set(range(0xf80a, 0xf80e)) | set(range(0xff60, 0x10000)) | set(range(0xfec0, 0xff04))
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return isolated[-1]

        for seed, (stage, pause, flag, count, life) in enumerate(itertools.product(sorted(STAGES), [0, 1, 127, 128], [0, 1, 2], [0, 63, 255], range(len(life_cases)))):
            setup(seed, stage, pause, flag, count, life)
            invoke(f'stage-{stage}/pause-{pause}/flag-{flag}/count-{count}/life-{life}')
        for seed, (stage, slot, behavior, health, team) in enumerate(itertools.product([14, 20], range(4), [0, 0x12668, 0x1313e], [223, 0, -1, -1023, -1024, -1025], [0, 1, 4, 128, 255])):
            setup(seed, stage, life=4)
            owner = FIGHTERS[slot]
            write(0xff0000+owner, behavior, 4)
            write(0xff0000+owner+0x78, health, 2)
            write(0xff0000+owner+0x6f, team)
            invoke(f'life-edge-{stage}-{slot}-{behavior}-{health}-{team}')
        for seed, (stage, pause_mask, input_mask, flag) in enumerate(itertools.product([10, 14, 22], range(16), range(16), [0, 1])):
            setup(seed, stage, mode_flag=flag, count=223 if stage == 22 else 0)
            write(0xfff551, pause_mask)
            write(0xffa07c, (seed//7)%16)
            write_bytes(0xfff798, bytes(128 if input_mask&(1 << i) else 0 for i in range(4)))
            invoke(f'start-{stage}-{pause_mask}-{input_mask}-{flag}')
        voice = [480, 320, 352, 352, 352, 384, 320, 320, 336, 352, 384]
        for seed, (char, winner, grouping, delta) in enumerate(itertools.product(range(1, 12), range(1, 5), [2, 4], [-1, 0, 1])):
            setup(seed, 22, count=voice[char-1]-1+delta, grouping=grouping)
            write_bytes(0xffa100, bytes([char]*4))
            write(0xffa074, winner, 2)
            invoke(f'voice-{char}-{winner}-{grouping}-{delta}')
        for seed, (winner, grouping, controller_set, char_set, score) in enumerate(itertools.product(range(1, 5), [2, 4], [[1, 2, 3, 4], [1, 0, 2, 0], [4, 3, 2, 1]], [[1, 2, 3, 4], [11, 10, 9, 8]], [0, 199, 200, 255])):
            setup(seed, 20, life=4, rule=2, grouping=grouping)
            for index, owner in enumerate(FIGHTERS):
                team = index+1 if grouping == 2 else 1 if index < 2 else 4
                write(0xff0000+owner+0x6f, team)
                write(0xff0000+owner+0x78, 223 if team == winner else -1024, 2)
            write_bytes(0xffa100, bytes(char_set+controller_set))
            write_bytes(0xffff60, bytes([score])*160)
            invoke(f'stats-{winner}-{grouping}-{controller_set}-{char_set}-{score}')
        for seed, (stage, count) in enumerate(itertools.product([10, 18, 22], [63, 64, 223, 224, 255, 256, 511, 512, 32767, 32768, 65535])):
            setup(seed, stage, count=count, life=3)
            invoke(f'counter-edge-{stage}-{count}')
        for seed, (option, extra, round_index, rule) in enumerate(itertools.product([0, 1, 255], [0, 1, 255], [-32768, -1, 0, 9, 10, 32767], [0, 2])):
            setup(seed, 22, count=319, rule=rule)
            write_bytes(0xffa100, bytes([2]*4))
            write(0xffa074, 1, 2)
            write(0xffff14, option)
            write(0xffa097, extra)
            write(0xffa06a, round_index, 2)
            write_bytes(0xfff80a, bytes(4))
            invoke(f'voice-gate-{option}-{extra}-{round_index}-{rule}')
        for seed, (controllers, rule, option, pause) in enumerate(itertools.product([[0]*4, [1, 2, 3, 4], [1, 0, 2, 0], [7, 8, 0, 0], [33, 255, 1, 0]], [0, 2, 4], [0, 1, 255], [0, 1, 127])):
            setup(seed, 8, pause=pause, rule=rule)
            write_bytes(0xffa104, bytes(controllers))
            write(0xffff16, option)
            invoke(f'pause-config-{controllers}-{rule}-{option}-{pause}')
        for seed, (winner, grouping, score) in enumerate(itertools.product(range(1, 5), [2, 4], [0, 199, 200, 255])):
            setup(seed, 20, life=4, rule=2, grouping=grouping)
            for index, owner in enumerate(FIGHTERS):
                team = index+1 if grouping == 2 else 1 if index < 2 else 4
                write(0xff0000+owner+0x6f, team)
                write(0xff0000+owner+0x78, 223 if team == winner else -1024, 2)
            write_bytes(0xffa104, bytes([1, 1, 2, 2]))
            write_bytes(0xffff60, bytes([score])*160)
            invoke(f'duplicate-controller-{winner}-{grouping}-{score}')
        boundary_count = len(isolated)
        for scenario in range(4):
            setup(scenario, 8, life=0 if scenario != 2 else 1, rule=2 if scenario == 3 else 0, grouping=4 if scenario == 2 else 2)
            calls = []
            for step in range(1000):
                external = {'health': {}, 'palette_flags': None}
                if step in (300, 310):
                    for index in ([0, 1, 2, 3] if scenario == 1 else [0, 1] if scenario == 2 else [0, 1, 2]):
                        value = -1 if step == 300 else -1024
                        write(0xff0000+FIGHTERS[index]+0x78, value, 2)
                        external['health'][str(index)] = value
                ram = ref.ram_cpu()
                if n(ram, 0x8046, 2) == 26:
                    write(0xfff501, 0)
                    external['palette_flags'] = 0
                call = invoke(f'stream-{scenario}/{step}')
                call['external_input'] = external
                calls.append(call)
                if 'clear-world' in call['events']:
                    break
            assert 'clear-world' in calls[-1]['events'], scenario
            sequences.append({'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity
    for name, calls, scope in [('match-lifecycle-oracle', natural, 'Natural covered complete manager returns including a real KO/result/clear'),
                               ('match-lifecycle-boundaries', isolated[:boundary_count], 'Declared isolated covered manager returns'),
                               ('match-lifecycle-sequences', sequences, 'Single-initial-state lifecycle streams with declared health/fade updates')]:
        rows = [c for s in calls for c in s['calls']] if name.endswith('-sequences') else calls
        raw = (json.dumps(calls, separators=(',', ':'))+'\n').encode()
        compressed = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(compressed)
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'records': len(calls), 'calls': len(rows),
                  'stage_counts': dict(Counter(str(c['stage'])for c in rows)), 'events': dict(Counter(e for c in rows for e in c['events'])),
                  'frames_match_original_core': frames == expected, 'baseline_frames': len(frames), 'incomplete_calls': 0, 'errors': errors,
                  'external_queue_calls': sum(bool(c['external_queue_clears'])for c in rows), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed),
                  'reference_only_calls': sum(bool(c.get('reference_only_reason'))for c in rows),
                  'pass': bool(calls)and not errors,
                  'limits': ['Stages0/2 scene/actor/resource initialization remain outside this covered phase API', 'Not a full world or all-menu/all-mode implementation']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
