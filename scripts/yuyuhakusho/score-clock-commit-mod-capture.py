"""Bounded original note cleanup, DAC commit prefix and modulation timings."""
import argparse
import ctypes as C
import gzip
import hashlib
import json
from pathlib import Path

from extract_audio_assets import AudioSession, write_cpu
from reference import ROOT
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint

VOICE = 0xfff840
DEFINITION = 0xfffd00
RETURN = 0x3d8


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def cases():
    rows = []
    for family in ['fm', 'psg']:
        for tied in [False, True]:
            for enabled in [False, True]:
                rows.append({'kind': 'commit', 'family': family, 'tied': tied,
                             'modulation_enabled': enabled, 'start': 0x3789e, 'stop': RETURN})
    for sample in [False, True]:
        rows.append({'kind': 'commit', 'family': 'dac', 'dac_sample': sample,
                     'start': 0x37694, 'stop': 0x376ac if sample else RETURN})
    rows.append({'kind': 'modulation', 'enabled': False, 'delay': 0, 'rate': 0, 'steps': 0,
                 'start': 0x3790e, 'stop': RETURN})
    for delay in [0, 1, 255]:
        for rate in [0, 1, 2, 255]:
            for steps in [0, 1, 255]:
                rows.append({'kind': 'modulation', 'enabled': True, 'delay': delay,
                             'rate': rate, 'steps': steps, 'start': 0x3790e, 'stop': RETURN})
    return [{**row, 'id': f"{i}-{row['kind']}-phase{phase}", 'origin': origin, 'refresh': refresh}
            for i, row in enumerate(rows)
            for phase, (origin, refresh) in enumerate([(0, 0), (1, 896), (895, 896), (1008, 1008)])]


def setup(ref, checkpoint, case):
    ref.restore(checkpoint, restore_refresh_cycles=True)
    for offset in range(48):
        write_cpu(ref, VOICE+offset, 0)
    write_cpu(ref, DEFINITION, 0xff03fdff, 4)
    write_cpu(ref, VOICE, 0x90 if case.get('tied') else 0x80)
    write_cpu(ref, VOICE+1, 0x80 if case.get('family') == 'psg' else 0)
    write_cpu(ref, VOICE+0xa, 0x80 if case.get('enabled', case.get('modulation_enabled', False)) else 0)
    write_cpu(ref, VOICE+0xc, 29)
    write_cpu(ref, VOICE+0xf, 17)
    write_cpu(ref, VOICE+0x10, 0x81 if case.get('dac_sample') else 0x80)
    write_cpu(ref, VOICE+0x12, 19)
    write_cpu(ref, VOICE+0x13, 7)
    write_cpu(ref, VOICE+0x14, DEFINITION, 4)
    write_cpu(ref, VOICE+0x18, case.get('delay', 9))
    write_cpu(ref, VOICE+0x19, case.get('rate', 9))
    write_cpu(ref, VOICE+0x1a, 0x80)
    write_cpu(ref, VOICE+0x1b, case.get('steps', 9))
    write_cpu(ref, VOICE+0x1c, 0x8001, 2)
    write_cpu(ref, VOICE+0x26, 31)
    write_cpu(ref, VOICE+3, 2)
    setter = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
    for index in range(15):
        setter(index, 0)
    for index, value in [(17, 0x2700), (15, 0xffff00), (12, 0xfffe00), (13, VOICE), (16, case['start'])]:
        setter(index, value)
    write_cpu(ref, 0xffff00, RETURN, 4)
    ref.cpu.cycles, ref.cpu.refresh_cycles = case['origin'], case['refresh']
    return ref.save()


def execute(ref, case, record):
    run = ref.api('m68k_run', None, C.c_uint)
    steps = []
    for _ in range(64):
        pc = ref.get_register(16) & 0xffffff
        if pc == case['stop']:
            return steps, {'Cycle': ref.cpu.cycles, 'Refresh': ref.cpu.refresh_cycles}
        assert (0x3789e <= pc <= 0x378e4 or 0x3790e <= pc <= 0x3795c
                or 0x37694 <= pc <= 0x376a8 or pc == 0x37722), ('escaped commit/modulation phase', case['id'], hex(pc))
        before, refresh = ref.cpu.cycles, ref.cpu.refresh_cycles
        run(before+1)
        if record:
            steps.append([pc, before, refresh, ref.cpu.cycles, ref.cpu.refresh_cycles])
    raise AssertionError(('commit/modulation step cap', case['id']))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT/'.local') and not out.exists()
    out.mkdir(parents=True)
    inputs = cases()
    assert len(inputs) == 188
    controls, rows, identities = {}, [], {}
    for name, core in [('original', ORIGINAL), ('observed', OBSERVED)]:
        with AudioSession(ROOT, out/('core-'+name), core=core) as ref:
            checkpoint = quiet_checkpoint(ref)
            identities[name] = ref.identity
            for i, case in enumerate(inputs):
                initial = setup(ref, checkpoint, case)
                _, plain_end = execute(ref, case, False)
                plain, plain_ram = ref.save(), ref.ram_cpu()
                plain_regs = [ref.get_register(index) for index in range(18)]
                again = setup(ref, checkpoint, case)
                assert initial['data'] == again['data']
                steps, end = execute(ref, case, True)
                saved, ram = ref.save(), ref.ram_cpu()
                registers = [ref.get_register(index) for index in range(18)]
                assert (plain_end == end and plain['data'] == saved['data']
                        and plain['refresh_cycles'] == saved['refresh_cycles']
                        and plain_ram == ram and plain_regs == registers), ('trace changed native', case['id'])
                result = {'steps': steps, 'end': end, 'registers': registers,
                          'voice_hex': ram[VOICE & 0xffff:(VOICE & 0xffff)+48].hex(), 'ram_sha256': sha(ram)}
                if name == 'original':
                    controls[i] = result
                else:
                    assert controls[i] == result, ('pinned cores disagree', case['id'])
                    public = {k: v for k, v in case.items() if k not in ['start', 'stop']}
                    rows.append({'input': public, **result, 'native_passive_state_equal': True})
        print(json.dumps({'core': name, 'cases': len(inputs), 'pass': True}), flush=True)
    source = ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt'
    payload = {'version': 1, 'scope': 'Private cleanup/modulation complete routines and DAC commit prefix before publication; no IRQ/whole-driver price',
               'identities': identities, 'cases': rows,
               'sources': {str(p.relative_to(ROOT)): sha(p.read_bytes()) for p in [Path(__file__), source]}}
    target = out/'native-commit-mod-full.json.gz'
    target.write_bytes(gzip.compress((json.dumps(payload, sort_keys=True, separators=(',', ':'))+'\n').encode(), mtime=0))
    print(json.dumps({'pass': True, 'cases': len(rows), 'executions': len(rows)*4,
                      'fixture_sha256': sha(target.read_bytes())}), flush=True)


if __name__ == '__main__':
    main()
