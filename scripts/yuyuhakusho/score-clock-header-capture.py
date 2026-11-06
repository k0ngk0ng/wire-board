"""Pinned native original caller+callee+return header boundary oracle.

The C recorder only reads native PC/cycle/refresh pointers around each original
instruction. No runtime instruction program or original PC enters production.
"""
import argparse
import ctypes as C
import gzip
import json
import os
from pathlib import Path
import subprocess

from extract_audio_assets import AudioSession, write_cpu
from reference import ROOT, CPUHead, sha
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint

SOURCE = ROOT / 'scripts/yuyuhakusho/score-clock-header-observer.c'
WAIT = [0, 1, 254, 255, 7, 128, 0, 255, 1, 254]
FIELDS = ['pc', 'before', 'deadline', 'after', 'next_deadline']
class Step(C.Structure):
    _fields_ = [(name, C.c_uint32) for name in FIELDS]

def saved(path, value):
    raw = (json.dumps(value, separators=(',', ':')) + '\n').encode()
    path.write_bytes(gzip.compress(raw, mtime=0) if path.suffix == '.gz' else raw)

def build(out):
    tmp = out / 'tmp'
    tmp.mkdir(parents=True, exist_ok=True)
    library = out / 'header-observer.dylib'
    subprocess.run(['clang', '-dynamiclib', '-O2', '-fno-modules', str(SOURCE), '-o', str(library)],
                   cwd=ROOT, env=dict(os.environ, TMPDIR=str(tmp), CLANG_MODULE_CACHE_PATH=str(tmp / 'clang-cache')), check=True)
    recorder = C.CDLL(str(library))
    recorder.score_clock_header_attach.argtypes = [C.c_void_p] * 3
    recorder.score_clock_header_attach.restype = C.c_int
    recorder.score_clock_header_detach.restype = C.c_int
    recorder.score_clock_header_count.restype = C.c_uint
    recorder.score_clock_header_overflow.restype = C.c_uint
    recorder.score_clock_header_size.restype = C.c_uint
    recorder.score_clock_header_steps.restype = C.POINTER(Step)
    assert recorder.score_clock_header_size() == C.sizeof(Step)
    return recorder, library

def profiles():
    masks = [0] + [1 << slot for slot in range(10)] + [0x155, 0x2aa, 0x3ff]
    tempos = [{'enabled': False, 'reload': False, 'active_music_mask': 0},
              {'enabled': True, 'reload': False, 'active_music_mask': 0}]
    tempos += [{'enabled': True, 'reload': True, 'active_music_mask': mask} for mask in masks]
    variants = [('control', tempos[0]), ('fade', tempos[0])]
    variants += [('tempo', work) for work in tempos] + [('combined', work) for work in tempos]
    phases = [('entry', 0, 0), ('future', 1, 8192), ('near-next', 895, 896), ('aligned', 1008, 1008), ('near-entry', 2048, 2050)]
    rows = []
    for kind, work in variants:
        for phase, origin, refresh in phases:
            ident = f"{kind}-e{int(work['enabled'])}-r{int(work['reload'])}-m{work['active_music_mask']:03x}-{phase}"
            rows.append({'id': ident, 'kind': kind, 'tempo': work, 'origin': origin, 'refresh': refresh})
    assert len(rows) == 170
    return rows

def setup(ref, checkpoint, case):
    ref.restore(checkpoint, restore_refresh_cycles=True)
    ref.cpu.cycles = case['origin']
    ref.cpu.refresh_cycles = case['refresh']
    start = {'control': 0x375ba, 'tempo': 0x375be, 'fade': 0x375c2, 'combined': 0x375ba}[case['kind']]
    setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
    for index, value in [(17, 0x2700), (15, 0xffff00), (16, start)]:
        setreg(index, value)
    for address in [0xfff828, 0xfff82b, 0xfff804]:
        write_cpu(ref, address, 0)
    work = case['tempo']
    write_cpu(ref, 0xfff802, 7 if work['enabled'] else 0)
    write_cpu(ref, 0xfff801, (1 if work['reload'] else 2) if work['enabled'] else 0)
    for slot in range(10):
        base = 0xfff840 + slot * 0x30
        write_cpu(ref, base, 0x80 if work['active_music_mask'] & 1 << slot else 0x40)
        write_cpu(ref, base + 0xe, WAIT[slot])
    return ref.save()

def execute(ref, case, recorder=None):
    stops = {'control': [0x375be], 'tempo': [0x375c2], 'fade': [0x375c6], 'combined': [0x375be, 0x375c2, 0x375c6]}[case['kind']]
    allowed = {0x375ba, 0x375be, 0x375c2, 0x38c2e, 0x38c34, 0x38c38, 0x38c3c,
               0x38c60, 0x38c66, 0x38c6a, 0x38c6e, 0x38d06,
               0x389a8, 0x389ac, 0x389ae, 0x389b2, 0x389b4, 0x389ba, 0x389be,
               0x389c0, 0x389c2, 0x389c4, 0x389c6, 0x389ca, 0x389cc, 0x389d0,
               0x3893a, 0x3893c, 0x38940, 0x3894c}
    run = ref.api('m68k_run', None, C.c_uint)
    steps, boundaries = [], []
    for _ in range(128):
        pc = ref.get_register(16) & 0xffffff
        if pc == stops[len(boundaries)]:
            boundaries.append({'cycle': ref.cpu.cycles, 'refresh': ref.cpu.refresh_cycles})
            if len(boundaries) == len(stops):
                return steps, boundaries
        assert pc in allowed, ('Escaped supported header profile', case['id'], hex(pc))
        before, deadline = ref.cpu.cycles, ref.cpu.refresh_cycles
        if recorder:
            recorder.score_clock_header_before()
        run(before + 1)
        if recorder:
            recorder.score_clock_header_after()
        steps.append([pc, before, deadline, ref.cpu.cycles, ref.cpu.refresh_cycles])
    raise AssertionError(('Original header exceeded fixed cap', case['id']))

def recorded(recorder):
    assert not recorder.score_clock_header_overflow()
    pointer = recorder.score_clock_header_steps()
    return [[int(getattr(pointer[index], field)) for field in FIELDS]
            for index in range(recorder.score_clock_header_count())]

def effects(ref, case):
    ram = ref.ram_cpu()
    work = case['tempo']
    has_tempo = case['kind'] in ['tempo', 'combined']
    reload = has_tempo and work['reload']
    waits = [ram[0xf84e + slot * 0x30] for slot in range(10)]
    expected = [(WAIT[slot] + int(reload and bool(work['active_music_mask'] & 1 << slot))) & 255 for slot in range(10)]
    assert waits == expected, ('Native note-wait path differs from declared work', case['id'], waits, expected)
    remaining = (7 if work['reload'] else (1 if has_tempo else 2)) if work['enabled'] else 0
    assert ram[0xf801] == remaining and ram[0xf802] == (7 if work['enabled'] else 0), case['id']
    assert not ram[0xf828] and not ram[0xf82b] and not ram[0xf804], case['id']
    return {'tempo_remaining': ram[0xf801], 'tempo_period': ram[0xf802], 'note_wait': waits}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--fixture', type=Path)
    parser.add_argument('--smoke', action='store_true')
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    recorder, library = build(out)
    cases = profiles()
    if args.smoke:
        cases = [cases[index] for index in [0, 5, 10, 15, 20, 85, 90, 95, 100, 105, 169]]
    controls, reports, identities = {}, [], {}
    for mode, core in [('original', ORIGINAL), ('hook', OBSERVED)]:
        with AudioSession(ROOT, out / ('core-' + mode), core=core) as ref:
            checkpoint = quiet_checkpoint(ref)
            identities[mode] = ref.identity
            for case in cases:
                initial = setup(ref, checkpoint, case)
                control_steps, control_boundaries = execute(ref, case)
                control_state, control_ram = ref.save(), ref.ram_cpu()
                control_effects = effects(ref, case)
                rebuilt = setup(ref, checkpoint, case)
                assert initial['data'] == rebuilt['data'] and initial['refresh_cycles'] == rebuilt['refresh_cycles'], ('Private header start differs', case['id'])
                address = C.addressof(ref.cpu)
                assert recorder.score_clock_header_attach(address + CPUHead.cycles.offset, address + CPUHead.refresh_cycles.offset, address + CPUHead.pc.offset)
                try:
                    steps, boundaries = execute(ref, case, recorder)
                    passive_steps = recorded(recorder)
                finally:
                    assert recorder.score_clock_header_detach() == 1
                after = ref.save()
                after_effects = effects(ref, case)
                assert control_steps == steps == passive_steps and control_boundaries == boundaries and control_state['data'] == after['data'] and control_state['refresh_cycles'] == after['refresh_cycles'] and control_ram == ref.ram_cpu() and control_effects == after_effects, ('Recorder changed native header', mode, case['id'])
                result = {'steps': steps, 'boundaries': boundaries, 'effects': after_effects}
                if mode == 'original':
                    controls[case['id']] = result
                else:
                    assert controls[case['id']] == result, ('Pinned header cores disagree', case['id'])
                    reports.append({'input': case, **result, 'recorder_on_off_native_state_equal': True, 'both_native_builds_equal': True})
            print(json.dumps({'mode': mode, 'cases': len(cases), 'pass': True}), flush=True)
    scope = 'Private original PC-relative caller + control-idle/tempo/fade-idle callee + one return; no non-idle duck/fade, queue, dispatch or whole tick'
    full = out / 'native-header-full.json.gz'
    saved(full, {'schema': 1, 'identity': identities, 'scope': scope, 'cases': reports})
    inputs = [SOURCE, Path(__file__), ROOT / 'scripts/yuyuhakusho/reference.py', ROOT / 'scripts/yuyuhakusho/extract_audio_assets.py', ROOT / 'scripts/yuyuhakusho/research_audio_loops.py']
    summary = {'pass': True, 'cases': len(reports), 'routine_stops': sum(len(row['boundaries']) for row in reports),
               'native_builds': 2, 'recorder_on_off_profile_executions': len(reports) * 4, 'identity': identities, 'scope': scope,
               'source_sha256': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in inputs},
               'artifact': {'path': str(full.relative_to(ROOT)), 'sha256': sha(full.read_bytes())},
               'recorder_binary_sha256': sha(library.read_bytes()), 'recorder_is_read_only_native_pointer_sampler': True,
               'no_native_code_or_ROM_modified': True, 'natural_driver_or_PCM_proof': False}
    if args.fixture:
        fixture = args.fixture.resolve()
        assert fixture.is_relative_to(ROOT / 'games/yuyuhakusho/audio/scoreclock/testdata') and fixture.name.startswith('header-')
        saved(fixture, {'schema': 1, 'scope': scope, 'cases': [{'input': row['input'], 'boundaries': row['boundaries']} for row in reports]})
        manifest = {'source': summary['artifact'], 'fixture': {'path': str(fixture.relative_to(ROOT)), 'sha256': sha(fixture.read_bytes())},
                    'cases': len(reports), 'routine_stops': summary['routine_stops'], 'producer_sha256': sha(Path(__file__).read_bytes()),
                    'source_addresses_or_opcodes_in_fixture': False}
        saved(fixture.with_name('header-native-manifest.json'), manifest)
        summary['fixture'] = manifest['fixture']
    saved(out / 'capture.json', summary)
    print(json.dumps({'pass': True, 'cases': len(reports), 'routine_stops': summary['routine_stops']}), flush=True)

if __name__ == '__main__':
    main()
