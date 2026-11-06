"""Bounded research-only native PSG raw store and fixed leaf oracle.

Each pinned original core executes the unmodified original instructions.
Source PCs and architectural steps remain in verification provenance only.
The declared private fixture resets the PSG after restoring a quiet world.
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

SOURCE = ROOT / 'scripts/yuyuhakusho/score-clock-psg-observer.c'
FIELDS = ['pc', 'at', 'after', 'refresh', 'address', 'value']
class Event(C.Structure):
    _fields_ = [(name, C.c_uint32) for name in FIELDS]

def saved(path, value):
    raw = (json.dumps(value, separators=(',', ':')) + '\n').encode()
    path.write_bytes(gzip.compress(raw, mtime=0) if path.suffix == '.gz' else raw)

def build(out):
    tmp = out / 'tmp'
    tmp.mkdir(parents=True, exist_ok=True)
    lib = out / 'score-clock-psg-observer.dylib'
    subprocess.run(['clang', '-dynamiclib', '-O2', '-fno-modules', str(SOURCE), '-o', str(lib)],
                   cwd=ROOT, env=dict(os.environ, TMPDIR=str(tmp), CLANG_MODULE_CACHE_PATH=str(tmp / 'clang-cache')), check=True)
    observer = C.CDLL(str(lib))
    observer.score_clock_psg_attach.argtypes = [C.c_void_p] * 3
    observer.score_clock_psg_attach.restype = C.c_int
    observer.score_clock_psg_detach.restype = C.c_int
    observer.score_clock_psg_pc.argtypes = [C.c_uint]
    observer.score_clock_psg_count.restype = C.c_uint
    observer.score_clock_psg_overflow.restype = C.c_uint
    observer.score_clock_psg_size.restype = C.c_uint
    observer.score_clock_psg_events.restype = C.POINTER(Event)
    assert observer.score_clock_psg_size() == C.sizeof(Event)
    return observer, lib

def native_psg(ref):
    buffer = C.create_string_buffer(1024)
    count = ref.api('psg_context_save', C.c_int, C.c_void_p)(buffer)
    assert count == 31 * C.sizeof(C.c_int), count
    words = list((C.c_int * 31).from_buffer_copy(buffer.raw[:count]))
    regs = words[3:11]
    return {'latch': words[1], 'tone_divider': regs[0:6:2], 'level': regs[1:8:2]}, buffer.raw[:count].hex()

def cases():
    variants = []
    for kind in ['store', 'single']:
        for source, values in [('volume', [0x90, 0xb3, 0xd5, 0xf7]), ('keyoff', [0x9f, 0xbf, 0xdf, 0xff])]:
            for value in values:
                variants.append({'kind': kind, 'source': source, 'value': value, 'data': 0})
    for channel in range(3):
        for divider in [1, 1023]:
            variants.append({'kind': 'pitch-pair', 'source': 'pitch-pair', 'value': 0x80 | channel << 5 | divider & 15, 'data': divider >> 4})
    variants.append({'kind': 'mute-all', 'source': 'mute-all', 'value': 0, 'data': 0})
    result = []
    for variant in variants:
        before_return = {'store': 112, 'single': 112, 'pitch-pair': 224, 'mute-all': 420}[variant['kind']]
        phases = [('future', 0, 4096), ('entry', 0, 0), ('odd-next-store', 1, 113),
                  ('aligned-entry', 1008, 1008), ('near-entry', 2048, 2050), ('return', 1, 1 + before_return)]
        for phase, origin, deadline in phases:
            result.append({**variant, 'id': f"{variant['kind']}-{variant['source']}-v{variant['value']:02x}-d{variant['data']:02x}-{phase}",
                           'origin': origin, 'refresh': deadline})
    assert len(result) == 138
    return result

def entry(case):
    return {'volume': 0x3a140, 'keyoff': 0x3a186, 'pitch-pair': 0x3a0c6, 'mute-all': 0x3a18e}[case['source']]

def setup(ref, checkpoint, case):
    ref.restore(checkpoint, restore_refresh_cycles=True)
    ref.api('psg_reset', None)()
    origin = case['origin']
    ref.cpu.cycles = origin
    ref.cpu.refresh_cycles = case['refresh']
    setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
    data = case['data'] if case['kind'] == 'pitch-pair' else case['value']
    for index, value in [(17, 0x2700), (15, 0xffff00), (0, case['value']), (6, data), (16, entry(case))]:
        setreg(index, value)
    write_cpu(ref, 0xffff00, 0x3d8, 4)
    return ref.save(), native_psg(ref)[1]

def execute(ref, case, observer=None):
    run = ref.api('m68k_run', None, C.c_uint)
    finish = entry(case) + 6 if case['kind'] == 'store' else 0x3d8
    steps = []
    allowed = {0x3a0c6, 0x3a0cc, 0x3a0d2, 0x3a140, 0x3a146, 0x3a186, 0x3a18c,
               0x3a18e, 0x3a194, 0x3a198, 0x3a19c, 0x3a1a0, 0x3a1a4}
    for _ in range(8):
        pc = ref.get_register(16) & 0xffffff
        if pc == finish:
            break
        assert pc in allowed, ('Escaped PSG leaf', case['id'], hex(pc))
        if observer:
            observer.score_clock_psg_pc(pc)
        before, deadline = ref.cpu.cycles, ref.cpu.refresh_cycles
        run(before + 1)
        steps.append([pc, before, deadline, ref.cpu.cycles, ref.cpu.refresh_cycles])
    else:
        raise AssertionError(('Native PSG leaf exceeded fixed cap', case['id']))
    return steps

def trace(observer):
    assert not observer.score_clock_psg_overflow()
    rows = observer.score_clock_psg_events()
    result = [{name: int(getattr(rows[index], name)) for name in FIELDS}
              for index in range(observer.score_clock_psg_count())]
    assert all(row['at'] == row['after'] and row['address'] == 0xc00011 for row in result), result
    return result

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--fixture', type=Path)
    parser.add_argument('--smoke', action='store_true')
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    observer, library = build(out)
    selected = cases()
    if args.smoke:
        selected = [selected[index] for index in [0, 1, 48, 49, 96, 97, 132, 133]]
    controls, reports, identities = {}, [], {}
    for mode, core in [('original', ORIGINAL), ('hook', OBSERVED)]:
        with AudioSession(ROOT, out / ('core-' + mode), core=core) as ref:
            checkpoint = quiet_checkpoint(ref)
            identities[mode] = ref.identity
            for case in selected:
                initial, initial_psg = setup(ref, checkpoint, case)
                control_steps = execute(ref, case)
                control_state, control_ram = ref.save(), ref.ram_cpu()
                control_psg, control_context = native_psg(ref)
                rebuilt, rebuilt_psg = setup(ref, checkpoint, case)
                assert initial['data'] == rebuilt['data'] and initial['refresh_cycles'] == rebuilt['refresh_cycles'] and initial_psg == rebuilt_psg, ('Private initial state differs', case['id'])
                address = C.addressof(ref.cpu)
                assert observer.score_clock_psg_attach(address, address + CPUHead.cycles.offset, address + CPUHead.refresh_cycles.offset)
                try:
                    steps = execute(ref, case, observer)
                    writes = trace(observer)
                finally:
                    assert observer.score_clock_psg_detach() == 1
                after, after_ram = ref.save(), ref.ram_cpu()
                after_psg, after_context = native_psg(ref)
                assert control_steps == steps and control_state['data'] == after['data'] and control_state['refresh_cycles'] == after['refresh_cycles'] and control_ram == after_ram and control_context == after_context, ('Recorder changed native state', mode, case['id'])
                result = {'steps': steps, 'events': writes, 'end': {'cycle': ref.cpu.cycles, 'refresh': ref.cpu.refresh_cycles}, 'psg': after_psg, 'psg_context_hex': after_context}
                expected_writes = {'store': 1, 'single': 1, 'pitch-pair': 2, 'mute-all': 4}[case['kind']]
                assert len(writes) == expected_writes, (case['id'], writes)
                if mode == 'original':
                    controls[case['id']] = result
                else:
                    assert controls[case['id']] == result, ('Pinned native cores disagree', case['id'])
                    reports.append({'input': case, **result, 'native_state_equal_without_recorder': True, 'both_native_builds_equal': True})
            print(json.dumps({'mode': mode, 'cases': len(selected), 'done': True}), flush=True)
    scope = 'Private reset-PSG raw absolute store and fixed single/pitch-pair/mute-all leaves; no caller preparation/call or whole-driver timing claim'
    full = out / 'native-services-full.json.gz'
    saved(full, {'schema': 1, 'scope': scope, 'identity': identities, 'cases': reports})
    inputs = [SOURCE, Path(__file__), ROOT / 'scripts/yuyuhakusho/reference.py', ROOT / 'scripts/yuyuhakusho/extract_audio_assets.py', ROOT / 'scripts/yuyuhakusho/research_audio_loops.py']
    summary = {'pass': True, 'cases': len(reports), 'writes': sum(len(row['events']) for row in reports), 'native_builds': 2,
               'recorder_on_off_invocations': len(reports) * 4, 'identity': identities, 'scope': scope,
               'source_sha256': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in inputs},
               'artifact': {'path': str(full.relative_to(ROOT)), 'sha256': sha(full.read_bytes())},
               'observer_binary_sha256': sha(library.read_bytes()), 'native_reset_psg_fixture': True,
               'native_pcm_or_natural_driver_proof': False}
    if args.fixture:
        fixture = args.fixture.resolve()
        assert fixture.is_relative_to(ROOT / 'games/yuyuhakusho/audio/scoreclock/testdata')
        assert fixture.name.startswith('psg-')
        portable = {'schema': 1, 'scope': scope, 'cases': [
            {'input': {name: row['input'][name] for name in ['id', 'kind', 'value', 'data', 'origin', 'refresh']},
             'events': [{'at': event['at'], 'value': event['value']} for event in row['events']],
             'end': row['end'], 'psg': row['psg']} for row in reports]}
        saved(fixture, portable)
        manifest = {'source': summary['artifact'], 'fixture': {'path': str(fixture.relative_to(ROOT)), 'sha256': sha(fixture.read_bytes())},
                    'cases': len(reports), 'writes': summary['writes'], 'producer_sha256': sha(Path(__file__).read_bytes()),
                    'source_addresses_or_opcodes_in_fixture': False}
        saved(fixture.with_name('psg-native-manifest.json'), manifest)
        summary['fixture'] = manifest['fixture']
    saved(out / 'capture.json', summary)
    print(json.dumps({'pass': True, 'cases': len(reports), 'writes': summary['writes']}), flush=True)

if __name__ == '__main__':
    main()
