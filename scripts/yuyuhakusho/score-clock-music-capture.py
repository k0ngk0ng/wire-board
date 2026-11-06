"""Original DAC commit, live arbitration and held-bus metadata publication."""
import argparse
import ctypes as C
import gzip
import importlib.util
import json
import os
from pathlib import Path
import subprocess

from extract_audio_assets import AudioSession, write_cpu
from export_audio_score import Registry
from reference import ROOT
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint
from inspect_dac_dsp_state import symbols

spec = importlib.util.spec_from_file_location('music_shared', ROOT/'scripts/yuyuhakusho/score-clock-capture.py')
shared = importlib.util.module_from_spec(spec)
spec.loader.exec_module(shared)
VOICE = 0xfff840


def cases(rom):
    formal = json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/data/audio-score.json.gz').read_bytes()))
    wanted = {d['id']: d for d in formal['dac_descriptors']}
    registry, notes = Registry(rom), {}
    for note in range(0x81, 0x9a):
        try:
            identity = registry.descriptor(note)
        except (ValueError, IndexError):
            continue
        if identity in wanted:
            assert registry.descriptors[identity] == wanted[identity]
            notes.setdefault(identity, note)
    assert set(notes) == set(wanted) and len(notes) == 6
    result = []
    phases = [(0, 0), (1, 896), (1008, 1008), (2048, 2050)]
    for identity, note in sorted(notes.items()):
        d = wanted[identity]
        descriptor = {'ID': identity, 'PayloadID': d['sample_id'], 'NativeRate': d['native_rate'],
                      'Flags': d['flags'], 'RoutingOrPan': 0, 'Protocol': 2}
        for marker in [0, 1, 128, 255]:
            for flags in [0, 33, 64, 128, 192, 255]:
                for pan in [192, 181]:
                    i = len(result)
                    origin, refresh = phases[i % 4]
                    result.append({'id': f'{i}-music-m{marker}-f{flags}-p{pan}', 'descriptor': descriptor,
                                   'marker': marker, 'flags': flags, 'pan': pan,
                                   'origin': origin, 'refresh': refresh, 'native_note': note})
    return result, {identity: registry.descriptor_sources[note] for identity, note in notes.items()}


def build(out):
    tmp = out/'tmp'
    tmp.mkdir()
    source = ROOT/'scripts/yuyuhakusho/score-clock-music-observer.c'
    library = out/'music-observer.dylib'
    subprocess.run(['clang', '-dynamiclib', '-O2', '-fno-modules', str(source), '-o', str(library)],
                   cwd=ROOT, env=dict(os.environ, TMPDIR=str(tmp), CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')), check=True)
    b = C.CDLL(str(library))
    b.score_clock_attach.argtypes = [C.c_void_p]*9
    b.score_clock_attach.restype = C.c_int
    b.score_clock_pc.argtypes = [C.c_uint]
    b.score_clock_count.restype = C.c_size_t
    b.score_clock_events.restype = C.POINTER(shared.Event)
    for name in ['score_clock_overflow', 'score_clock_detach', 'score_clock_music_enable', 'score_clock_music_disable']:
        getattr(b, name).restype = C.c_int
    return b


def setup(ref, checkpoint, case, lookup):
    ref.restore(checkpoint, restore_refresh_cycles=True)
    image = C.cast(ref.lib.audio_update, C.c_void_p).value-lookup['_audio_update'][0]
    C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value = 0
    C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value = image+lookup['_fm_buffer'][0]
    ref.cpu.cycles = case['origin']
    ref.api('gen_zbusreq_w', None, C.c_uint, C.c_uint)(1, case['origin'])
    ref.cpu.cycles, ref.cpu.refresh_cycles = case['origin'], case['refresh']
    write_cpu(ref, VOICE+0x10, case['native_note'])
    write_cpu(ref, VOICE+0x27, case['pan'])
    zram = (C.c_ubyte*8192).in_dll(ref.lib, 'zram')
    for address in [0x1fe6, 0x1fe7, 0x1fe8, 0x1fe9]:
        zram[address] = 17
    for address, value in [(0x1ffd, case['marker']), (0x1ffc, case['flags']), (0x1f2a, 9),
                           (0x1ff8, 64), (0x1ff9, 64), (0x1ffe, 17), (0x1ffb, 5)]:
        zram[address] = value
    setter = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
    for index in range(15):
        setter(index, 0)
    for index, value in [(17, 0x2700), (15, 0xffff00), (12, 0xfffd00), (13, VOICE), (16, 0x37694)]:
        setter(index, value)
    write_cpu(ref, 0xffff00, 0x3d8, 4)
    return ref.save()


def execute(ref, case, b=None):
    run = ref.api('m68k_run', None, C.c_uint)
    steps = []
    for _ in range(128):
        pc = ref.get_register(16) & 0xffffff
        if pc == 0x3d8:
            return steps
        assert 0x37694 <= pc <= 0x37722, ('escaped music publication', case['id'], hex(pc))
        before, refresh = ref.cpu.cycles, ref.cpu.refresh_cycles
        if b:
            b.score_clock_pc(pc)
        run(before+1)
        steps.append([pc, before, refresh, ref.cpu.cycles, ref.cpu.refresh_cycles])
    raise AssertionError(('music publication step cap', case['id']))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT/'.local') and not out.exists()
    out.mkdir(parents=True)
    rom = ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    inputs, provenance = cases(rom.read_bytes())
    assert len(inputs) == 288
    b = build(out)
    controls, rows, identities = {}, [], {}
    for name, core in [('original', ORIGINAL), ('observed', OBSERVED)]:
        with AudioSession(ROOT, out/('core-'+name), core=core) as ref:
            checkpoint, lookup = quiet_checkpoint(ref), symbols(core)
            identities[name] = ref.identity
            for i, case in enumerate(inputs):
                initial = setup(ref, checkpoint, case, lookup)
                plain_steps = execute(ref, case)
                plain, plain_ram = ref.save(), ref.ram_cpu()
                again = setup(ref, checkpoint, case, lookup)
                assert initial['data'] == again['data']
                shared.attach(b, ref)
                assert b.score_clock_music_enable() == 1
                if name == 'observed':
                    ref.api('set_cpu_hook', None, C.c_void_p)(C.cast(b.score_clock_hook, C.c_void_p))
                try:
                    steps, events = execute(ref, case, b), shared.events(b)
                finally:
                    if name == 'observed':
                        ref.api('set_cpu_hook', None, C.c_void_p)(None)
                    assert b.score_clock_music_disable() == 1 and b.score_clock_detach() == 1
                saved = ref.save()
                assert (plain_steps == steps and plain['data'] == saved['data']
                        and plain['refresh_cycles'] == saved['refresh_cycles'] and plain_ram == ref.ram_cpu()), ('observer changed music publication', case['id'])
                zram = (C.c_ubyte*8192).in_dll(ref.lib, 'zram')
                result = {'steps': steps, 'events': [e for e in events if e['kind'] != 7],
                          'end': {'Cycle': ref.cpu.cycles, 'Refresh': ref.cpu.refresh_cycles},
                          'fields': {str(a): int(zram[a]) for a in [0x1ffd, 0x1ffc, 0x1ff8, 0x1ff9, 0x1ffe, 0x1ffb, 0x1f2a]}}
                if name == 'original':
                    controls[i] = result
                else:
                    assert controls[i] == result, ('pinned cores disagree', case['id'])
                    public = {k: v for k, v in case.items() if k != 'native_note'}
                    rows.append({'input': public, **result, 'native_observer_state_equal': True})
        print(json.dumps({'core': name, 'cases': len(inputs), 'pass': True}), flush=True)
    sources = [Path(__file__), ROOT/'scripts/yuyuhakusho/score-clock-music-observer.c',
               ROOT/'scripts/yuyuhakusho/score-clock-bridge.c',
               ROOT/'games/yuyuhakusho/data/audio-score.json.gz',
               ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']
    payload = {'version': 1, 'scope': 'Six formal music descriptors; DAC commit through arbitration/publication/return; initially held bus, busy9, no extra IRQ',
               'identities': identities, 'cases': rows, 'descriptor_provenance': provenance,
               'sources': {str(p.relative_to(ROOT)): shared.sha(p.read_bytes()) for p in sources}}
    target = out/'native-music-full.json.gz'
    target.write_bytes(gzip.compress((json.dumps(payload, sort_keys=True, separators=(',', ':'))+'\n').encode(), mtime=0))
    print(json.dumps({'pass': True, 'cases': len(rows), 'executions': len(rows)*4,
                      'fixture_sha256': shared.sha(target.read_bytes())}), flush=True)


if __name__ == '__main__':
    main()
