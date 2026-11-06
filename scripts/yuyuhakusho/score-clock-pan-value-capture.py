"""Supplement original pan oracle: stored field differs from YM call value."""
import argparse
import ctypes as C
import gzip
import importlib.util
import json
from pathlib import Path

from extract_audio_assets import AudioSession
from reference import ROOT
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint
from inspect_dac_dsp_state import symbols

spec = importlib.util.spec_from_file_location('pan_original', ROOT/'scripts/yuyuhakusho/score-clock-pan-capture.py')
pan = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pan)


def setup(ref, checkpoint, case, lookup):
    pan.setup(ref, checkpoint, case, lookup)
    ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)(1, case['write_pan'])
    return ref.save()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT/'.local') and not out.exists()
    out.mkdir(parents=True)
    b, _ = pan.build(out)
    inputs = []
    phases = [(0, 0), (1, 896), (1008, 1008), (2048, 2050)]
    for channel in [0, 1, 2, 4, 5, 6]:
        for marker in [0, 1, 128, 255]:
            for field, argument in [(181, 117), (117, 181)]:
                i = len(inputs)
                origin, refresh = phases[i % 4]
                inputs.append({'id': f'{i}-c{channel}-m{marker}-field{field}-value{argument}',
                               'entry': 'voice', 'claimed': False, 'channel': channel, 'marker': marker,
                               'pan': field, 'write_pan': argument, 'origin': origin, 'refresh': refresh})
    assert len(inputs) == 48
    controls, rows, identities = {}, [], {}
    for name, core in [('original', ORIGINAL), ('observed', OBSERVED)]:
        with AudioSession(ROOT, out/('core-'+name), core=core) as ref:
            checkpoint, lookup = quiet_checkpoint(ref), symbols(core)
            identities[name] = ref.identity
            for i, case in enumerate(inputs):
                initial = setup(ref, checkpoint, case, lookup)
                plain_steps = pan.execute(ref, case)
                plain, plain_ram = ref.save(), ref.ram_cpu()
                again = setup(ref, checkpoint, case, lookup)
                assert initial['data'] == again['data']
                pan.shared.attach(b, ref)
                assert b.score_clock_pan_enable() == 1
                if name == 'observed':
                    ref.api('set_cpu_hook', None, C.c_void_p)(C.cast(b.score_clock_hook, C.c_void_p))
                try:
                    steps, events = pan.execute(ref, case, b), pan.shared.events(b)
                finally:
                    if name == 'observed':
                        ref.api('set_cpu_hook', None, C.c_void_p)(None)
                    assert b.score_clock_pan_disable() == 1 and b.score_clock_detach() == 1
                saved = ref.save()
                assert (plain_steps == steps and plain['data'] == saved['data']
                        and plain['refresh_cycles'] == saved['refresh_cycles'] and plain_ram == ref.ram_cpu())
                zram = (C.c_ubyte*8192).in_dll(ref.lib, 'zram')
                result = {'steps': steps, 'events': [e for e in events if e['kind'] != 7],
                          'end': {'Cycle': ref.cpu.cycles, 'Refresh': ref.cpu.refresh_cycles},
                          'restore_pan': int(zram[0x1ff8]), 'marker': int(zram[0x1ffd])}
                if name == 'original':
                    controls[i] = result
                else:
                    assert controls[i] == result
                    rows.append({'input': case, **result, 'native_observer_state_equal': True})
        print(json.dumps({'core': name, 'cases': len(inputs), 'pass': True}), flush=True)
    sources = [Path(__file__), ROOT/'scripts/yuyuhakusho/score-clock-pan-capture.py',
               ROOT/'scripts/yuyuhakusho/score-clock-pan-observer.c', ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',
               ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']
    payload = {'version': 1, 'scope': 'Legal FM voice rows; distinct stored pan and call value, initial held bus/mailbox0/no extra IRQ',
               'identities': identities, 'cases': rows,
               'sources': {str(p.relative_to(ROOT)): pan.shared.sha(p.read_bytes()) for p in sources}}
    target = out/'native-pan-value-full.json.gz'
    target.write_bytes(gzip.compress((json.dumps(payload, sort_keys=True, separators=(',', ':'))+'\n').encode(), mtime=0))
    print(json.dumps({'pass': True, 'cases': len(rows), 'executions': len(rows)*4,
                      'fixture_sha256': pan.shared.sha(target.read_bytes())}), flush=True)


if __name__ == '__main__':
    main()
