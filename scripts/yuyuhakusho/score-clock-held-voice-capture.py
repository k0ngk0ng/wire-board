"""Capture complete original held-note FM/PSG voice calls, not isolated leafs."""
import argparse
import ctypes as C
import gzip
import importlib.util
import itertools
import json
from pathlib import Path

from extract_audio_assets import AudioSession, write_cpu
from reference import ROOT, CPUHead
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint
from inspect_dac_dsp_state import symbols


def module(name, file):
    spec = importlib.util.spec_from_file_location(name, ROOT/'scripts/yuyuhakusho'/file)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


pan = module('held_pan', 'score-clock-pan-capture.py')
psg = module('held_psg', 'score-clock-psg-capture.py')
VOICE = 0xfff840
MOD = 0xfffc00
MODES = ['disabled', 'advance-on-note', 'restart-on-note-once', 'restart-on-note-loop', 'restart-every-driver-tick']


def cases(provenance):
    rows = []

    def add(label, **values):
        i = len(rows)
        origin, refresh = [(0, 0), (1, 896), (895, 896), (1008, 1008)][i % 4]
        case = dict(id=f'{i}-{label}', channel=0, note_wait=2, gate_remaining=0, gate_ticks=0,
                    note_off=False, claimed=False, tie=False, base_pitch=8805, detune=-5, scale=1,
                    base_level=15, mod_enabled=False, mod_delay=0, mod_rate=1, mod_steps=2,
                    mod_delta=5, mod_offset=-77, modulation=dict(delay=2, rate=3, delta=5, steps=4),
                    pan_definition=dict(mode='disabled'), pan_state=dict(step=0, elapsed=0),
                    pitch_key=None, volume_key=None, origin=origin, refresh=refresh)
        case.update(values)
        rows.append(case)

    # Each formal logical resource step now traverses its entire held caller.
    for record in provenance:
        pitch = record['key']['family'] == 'pitch'
        add('formal-'+record['key']['family'], channel=0 if pitch else 128,
            pitch_key=record['key'] if pitch else None, volume_key=None if pitch else record['key'],
            mod_enabled=bool(len(rows) % 2))
    channels = [0, 1, 2, 4, 5, 128, 160, 192, 224]
    for channel in channels:
        add('disabled', channel=channel)
        for tie, claimed in itertools.product([False, True], repeat=2):
            add('gate-expiry', channel=channel, gate_remaining=1, tie=tie, claimed=claimed)
        for remaining in [2, 255]:
            add('gate-wait-wrap', channel=channel, gate_remaining=remaining, note_wait=0)
    for channel in [0, 128]:
        for delay, rate, steps, offset in [(2, 1, 2, -77), (0, 0, 2, -77), (0, 2, 2, -77),
                                           (0, 1, 0, -77), (0, 1, 2, 32767)]:
            add('modulation', channel=channel, mod_enabled=True, mod_delay=delay,
                mod_rate=rate, mod_steps=steps, mod_offset=offset)
        for note, claimed in [(True, False), (False, True), (True, True)]:
            add('pitch-guard', channel=channel, mod_enabled=True, note_off=note, claimed=claimed)
    automation = json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/verification/score-clock-frequency-pan/native-frequency-pan-full.json.gz').read_bytes()))
    for row in automation['cases']:
        case = row['input']
        if case['kind'] == 'automation' and not case['on_note']:
            add('pan', pan_definition=case['definition'], pan_state=case['state'],
                note_off=case['note_off'], claimed=case['claimed'])
    sample = next(r for r in provenance if r['key']['family'] == 'volume' and r['tick']['writes_level'])
    for channel, changes in itertools.product([128, 224], [dict(note_off=True), dict(claimed=True),
            dict(tie=True, gate_ticks=1), dict(tie=True), dict(tie=True, gate_ticks=1, gate_remaining=2)]):
        add('volume-guard', channel=channel, volume_key=sample['key'], mod_enabled=True, **changes)
    return rows


def setup(ref, checkpoint, case, lookup, resources):
    ref.restore(checkpoint, restore_refresh_cycles=True)
    ref.api('psg_reset', None)()
    image = C.cast(ref.lib.audio_update, C.c_void_p).value-lookup['_audio_update'][0]
    C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value = 0
    C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value = image+lookup['_fm_buffer'][0]
    ref.cpu.cycles = case['origin']
    ref.api('gen_zbusreq_w', None, C.c_uint, C.c_uint)(1, case['origin'])
    ref.cpu.cycles, ref.cpu.refresh_cycles = case['origin'], case['refresh']
    z = (C.c_ubyte*8192).in_dll(ref.lib, 'zram')
    z[0x1f2a] = z[0x1ffd] = z[0x1ffe] = 0
    for offset in range(48):
        write_cpu(ref, VOICE+offset, 0)
    flags = 128 | (2 if case['note_off'] else 0) | (4 if case['claimed'] else 0) | (16 if case['tie'] else 0)
    fields = {0: flags, 1: case['channel'], 3: case['scale'] & 255, 9: case['base_level'],
              0xe: case['note_wait'], 0x12: case['gate_remaining'], 0x13: case['gate_ticks'],
              0x18: case['mod_delay'], 0x19: case['mod_rate'], 0x1a: case['mod_delta'] & 255,
              0x1b: case['mod_steps'], 0x1e: case['detune'] & 255, 0x27: 181}
    for family, control, cursor in [('pitch', 0xa, 0x26), ('volume', 0xb, 0xc)]:
        key = case[family+'_key']
        if key is not None:
            record = resources[(family, key['envelope_id'], key['section'], key['tick_index'])]
            fields[control], fields[cursor] = key['envelope_id'], record['source_cursor']
    fields[0xa] = fields.get(0xa, 0) | (128 if case['mod_enabled'] else 0)
    definition, state = case['pan_definition'], case['pan_state']
    fields.update({0x1f: MODES.index(definition['mode']), 0x20: 1, 0x21: state['step'],
                   0x22: definition.get('step_limit', 0), 0x23: definition.get('period', 0), 0x24: state['elapsed']})
    for offset, value in fields.items():
        write_cpu(ref, VOICE+offset, value)
    for offset, value in [(0x10, case['base_pitch']), (0x1c, case['mod_offset'] & 65535)]:
        write_cpu(ref, VOICE+offset, value, 2)
    write_cpu(ref, VOICE+0x14, MOD, 4)
    for i, field in enumerate(['delay', 'rate', 'delta', 'steps']):
        write_cpu(ref, MOD+i, case['modulation'][field] & 255)
    write_cpu(ref, 0xfff80f, 0)
    setter = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
    for i in range(15):
        setter(i, 0)
    for i, value in [(17, 0x2700), (15, 0xffff00), (13, VOICE), (16, 0x377ec if case['channel'] < 128 else 0x39ffe)]:
        setter(i, value)
    write_cpu(ref, 0xffff00, 0x3d8, 4)
    return ref.save()


def execute(ref, case, fm=None, port=None):
    run, steps = ref.api('m68k_run', None, C.c_uint), []
    for _ in range(768):
        pc = ref.get_register(16) & 0xffffff
        if pc == 0x3d8:
            return steps
        assert (0x377ec <= pc <= 0x37b8e or 0x38ae0 <= pc <= 0x38c2c or 0x39ffe <= pc <= 0x3a18c), ('escaped held voice', case['id'], hex(pc))
        if fm:
            fm.score_clock_pc(pc)
        if port:
            port.score_clock_psg_pc(pc)
        before, refresh = ref.cpu.cycles, ref.cpu.refresh_cycles
        run(before+1)
        steps.append([pc, before, refresh, ref.cpu.cycles, ref.cpu.refresh_cycles])
    raise AssertionError(('held voice step cap', case['id']))


def projection(ref):
    ram = ref.ram_cpu()
    byte = lambda n: ram[(VOICE+n) & 65535]
    signed = lambda value: value-256 if value >= 128 else value
    offset = byte(0x1c)*256+byte(0x1d)
    return dict(note_wait=byte(0xe), gate_remaining=byte(0x12), note_off=bool(byte(0) & 2),
                scale=signed(byte(3)), mod_delay=byte(0x18), mod_rate=byte(0x19),
                mod_steps=byte(0x1b), mod_delta=signed(byte(0x1a)), mod_offset=offset-65536 if offset >= 32768 else offset,
                pan_state=dict(step=byte(0x21), elapsed=byte(0x24)), stored_pan=byte(0x27))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT/'.local') and not out.exists()
    out.mkdir(parents=True)
    provenance_path = ROOT/'.local/md-yuyuhakusho/envelope-work-export-v2/provenance.json.gz'
    provenance = json.loads(gzip.decompress(provenance_path.read_bytes()))['records']
    resources = {(r['key']['family'], r['key']['envelope_id'], r['key']['section'], r['key']['tick_index']): r for r in provenance}
    inputs = cases(provenance)
    fm, _ = pan.build(out)
    port, _ = psg.build(out)
    controls, rows, identities = {}, [], {}
    print(json.dumps({'cases': len(inputs)}), flush=True)
    for name, core in [('original', ORIGINAL), ('observed', OBSERVED)]:
        with AudioSession(ROOT, out/('core-'+name), core=core) as ref:
            checkpoint, lookup = quiet_checkpoint(ref), symbols(core)
            identities[name] = ref.identity
            for i, case in enumerate(inputs):
                initial = setup(ref, checkpoint, case, lookup, resources)
                plain_steps = execute(ref, case)
                plain, plain_ram, plain_psg = ref.save(), ref.ram_cpu(), psg.native_psg(ref)[1]
                again = setup(ref, checkpoint, case, lookup, resources)
                assert initial['data'] == again['data']
                pan.shared.attach(fm, ref)
                address = C.addressof(ref.cpu)
                assert port.score_clock_psg_attach(address, address+CPUHead.cycles.offset, address+CPUHead.refresh_cycles.offset)
                try:
                    steps = execute(ref, case, fm, port)
                    events, writes = pan.shared.events(fm), psg.trace(port)
                finally:
                    assert port.score_clock_psg_detach() == 1 and fm.score_clock_detach() == 1
                saved = ref.save()
                assert (steps == plain_steps and saved['data'] == plain['data'] and saved['refresh_cycles'] == plain['refresh_cycles'] and plain_ram == ref.ram_cpu() and plain_psg == psg.native_psg(ref)[1]), ('passive observer changed state', case['id'])
                result = dict(steps=steps, events=[e for e in events if e['kind'] != 7], psg=writes,
                              end=dict(Cycle=ref.cpu.cycles, Refresh=ref.cpu.refresh_cycles), state=projection(ref))
                if name == 'original':
                    controls[i] = result
                else:
                    assert controls[i] == result, ('pinned cores disagree', case['id'])
                    rows.append(dict(input=case, **result, native_observer_state_equal=True))
        print(json.dumps({'core': name, 'cases': len(inputs), 'pass': True}), flush=True)
    sources = [Path(__file__), provenance_path, ROOT/'scripts/yuyuhakusho/score-clock-pan-capture.py',
               ROOT/'scripts/yuyuhakusho/score-clock-psg-capture.py', ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',
               ROOT/'scripts/yuyuhakusho/score-clock-psg-observer.c', ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt',
               ROOT/'games/yuyuhakusho/verification/score-clock-frequency-pan/native-frequency-pan-full.json.gz']
    result = dict(version=1, scope='Complete held-note FM/PSG voice body; all580 logical envelope steps plus caller branches. No parser, FM6, extraIRQ or whole world tick.',
                  identities=identities, cases=rows, sources={str(p.relative_to(ROOT)): psg.sha(p.read_bytes()) for p in sources})
    target = out/'native-held-voice-full.json.gz'
    target.write_bytes(gzip.compress((json.dumps(result, sort_keys=True, separators=(',', ':'))+'\n').encode(), mtime=0))
    print(json.dumps({'pass': True, 'cases': len(rows), 'executions': len(rows)*4, 'sha256': psg.sha(target.read_bytes())}), flush=True)


if __name__ == '__main__':
    main()
