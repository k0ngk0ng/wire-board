"""Whole original219ac interrupt and219ea catchup through the real driver."""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
from importlib.util import spec_from_file_location, module_from_spec

from extract_audio_assets import AudioSession, write_cpu
from inspect_dac_dsp_state import symbols
from reference import ROOT, CPUHead
from research_audio_loops import ORIGINAL, OBSERVED, quiet_checkpoint


def module(name, filename):
    spec = spec_from_file_location(name, ROOT/'scripts/yuyuhakusho'/filename)
    result = module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


dispatch = module('control_dispatch', 'score-clock-dispatch-init-capture.py')
running, note = dispatch.running, dispatch.note
FIELDS = {0x38808: ('priority-clear', 0xf800), 0x38a58: ('priority-save', 0xf800),
          0x38a6e: ('priority-restore', 0xf800), 0x38a72: ('header-selected', 0xf809),
          0x38a42: ('clear-selected', 0xf809),
          0x375b2: ('pause-read', 0xf807), 0x37b90: ('pause-read', 0xf807),
          0x37b9c: ('pause-read', 0xf807), 0x37ba2: ('pause-write', 0xf807), 0x37c08: ('pause-write', 0xf807),
          0x38916: ('fade-remaining', 0xf806), 0x38928: ('fade-remaining', 0xf806),
          0x38948: ('fade-remaining', 0xf806), 0x38956: ('fade-remaining', 0xf806),
          0x3891c: ('fade-steps', 0xf804), 0x3892e: ('fade-steps', 0xf804), 0x3894e: ('fade-steps', 0xf804)}


SCHEDULER_FIELDS = {0x219ac:('delay-read',0xf560),0x219b4:('delay-write',0xf560),0x219ba:('delayed-read',0xf562),0x219c8:('delayed-write',0xf562),0x219cc:('busy-read',0xf564),0x219d2:('backlog-add',0xf563),0x219d8:('busy-write',0xf564),0x219e4:('busy-write',0xf564),0x219ea:('backlog-read',0xf563),0x219f0:('busy-write',0xf564),0x219fc:('busy-write',0xf564),0x21a00:('backlog-decrement',0xf563)}
REQUEST_FIELDS = {}
for slot, (test, compare, store) in enumerate([(0x21960,0x21966,0x2196c),(0x21972,0x21978,0x2197e),(0x21984,0x2198a,0x21990),(0x21996,0x2199c,0x219a2)]):
    REQUEST_FIELDS.update({test:('request-test-read',0xf80a+slot),compare:('request-compare-read',0xf80a+slot),store:('request-store',0xf80a+slot)})
READ_FIELDS = {'priority-save','pause-read','delay-read','delayed-read','busy-read','backlog-read','request-test-read','request-compare-read','choice-read'}
def scheduler_state(ref):
    ram = ref.ram_cpu()
    return dict(Delay=int.from_bytes(ram[0xf560:0xf562],'big'),DelayedCode=ram[0xf562],Backlog=ram[0xf563],Busy=ram[0xf564])


def execute(ref, row, observer=None, port=None):
    run = ref.api('m68k_run', None, C.c_uint)
    steps, fields, scheduler_fields, request_fields, choice_fields, entries = [], [], [], [], [], []
    for _ in range(60000):
        pc = ref.get_register(16) & 0xffffff
        if pc == 0x3d8:
            return steps, fields, scheduler_fields, request_fields, choice_fields, entries
        assert 0x21960 <= pc <= 0x21a06 or 0x375a6 <= pc <= 0x3a1a4, ('escaped sound interrupt', row['id'], hex(pc))
        before, refresh = ref.cpu.cycles, ref.cpu.refresh_cycles
        if pc == 0x375a6: entries.append(dict(Cycle=ref.cpu.cycles, Refresh=ref.cpu.refresh_cycles))
        event = FIELDS.get(pc)
        category = 'driver'
        if pc in SCHEDULER_FIELDS:
            event = SCHEDULER_FIELDS[pc]; category = 'scheduler'
        elif pc in REQUEST_FIELDS:
            event = REQUEST_FIELDS[pc]; category = 'request'
        elif pc in [0x37ef8,0x37f1e,0x37fce]:
            event = ('choice-read' if pc == 0x37ef8 else 'choice-write', 0xf82c); category = 'choice'
        at = before+(14 if before >= refresh else 0)
        value = None
        if event:
            width = 2 if event[1] == 0xf560 else 1
            value = int.from_bytes(ref.ram_cpu()[event[1]:event[1]+width], 'big')
        if observer:
            observer.score_clock_pc(pc)
        if port:
            port.score_clock_psg_pc(pc)
        run(before+1)
        steps.append([pc, before, refresh, ref.cpu.cycles, ref.cpu.refresh_cycles])
        if event:
            if event[0] not in READ_FIELDS:
                width = 2 if event[1] == 0xf560 else 1
                value = int.from_bytes(ref.ram_cpu()[event[1]:event[1]+width], 'big')
            # The existing driver diagnostics omit the normal zero pause test.
            if pc != 0x375b2 or value:
                target = fields if category == 'driver' else scheduler_fields if category == 'scheduler' else request_fields if category == 'request' else choice_fields
                item = dict(kind=event[0], at=at, value=value)
                if category == 'request': item['slot'] = event[1]-0xf80a
                target.append(item)
    raise AssertionError(('control tick step cap', row['id']))


def cache_levels(ref):
    raw = ref.ram_cpu()[0xfbe0:0xfc00]
    return [[raw[(ch//3)*16+physical*4+ch % 3] & 127 for physical in [0, 2, 1, 3]] for ch in range(6)]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT/'.local') and not out.exists()
    out.mkdir(parents=True)
    candidates = ROOT/'.local/md-yuyuhakusho/sound-irq-case-export/cases.json.gz'
    inputs = note.load(candidates)['cases']
    rompath = ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rompath.read_bytes()
    catalog = note.load(ROOT/'games/yuyuhakusho/data/audio-score.json.gz')
    contexts = note.contexts(rom, catalog)
    work = {r['id']: r for r in note.load(ROOT/'games/yuyuhakusho/data/audio-score-instrument-work.json.gz')['profiles']}
    envpath = ROOT/'.local/md-yuyuhakusho/envelope-work-export-v2/provenance.json.gz'
    env = {(r['key']['family'], r['key']['envelope_id'], r['key']['section'], r['key']['tick_index']): r for r in note.load(envpath)['records']}
    patterns = [p['id'] for p in catalog['pan_patterns']]
    registry, descriptors = note.Registry(rom), {}
    wanted = {d['id'] for d in catalog['dac_descriptors']}
    for code in range(0x81, 0x9a):
        try:
            identity = registry.descriptor(code)
        except (ValueError, IndexError):
            continue
        if identity in wanted:
            descriptors.setdefault(identity, code)
    for i, row in enumerate(inputs):
        row['id'] = f'{i}-{row["id"]}'
        row['origin'], row['refresh'] = [(0, 0), (1, 896), (895, 896), (1008, 1008)][i % 4]
    source = ROOT/'scripts/yuyuhakusho/sound-irq-observer.c'
    tmp = out/'tmp'
    tmp.mkdir()
    library = out/'sound-irq-observer.dylib'
    note.subprocess.run(['clang', '-dynamiclib', '-O2', '-fno-modules', str(source), '-o', str(library)], cwd=ROOT,
                        env=dict(note.os.environ, TMPDIR=str(tmp), CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')), check=True)
    observer = C.CDLL(str(library))
    observer.score_clock_attach.argtypes = [C.c_void_p]*9
    observer.score_clock_attach.restype = C.c_int
    observer.score_clock_pc.argtypes = [C.c_uint]
    observer.score_clock_count.restype = C.c_size_t
    observer.score_clock_events.restype = C.POINTER(note.music.shared.Event)
    for name in ['score_clock_overflow', 'score_clock_detach', 'score_clock_irq_enable', 'score_clock_irq_disable']:
        getattr(observer, name).restype = C.c_int
    port, _ = note.psg.build(out)
    controls, rows, identities = {}, [], {}
    print(json.dumps({'profiles': len(inputs)}), flush=True)
    for name, core in [('original', ORIGINAL), ('observed', OBSERVED)]:
        with AudioSession(ROOT, out/('core-'+name), core=core) as ref:
            checkpoint, lookup = quiet_checkpoint(ref), symbols(core)
            identities[name] = ref.identity
            image = C.cast(ref.lib.audio_update, C.c_void_p).value-lookup['_audio_update'][0]
            writer = C.CFUNCTYPE(None, C.c_uint, C.c_uint, C.c_uint)(C.c_void_p.in_dll(ref.lib, 'fm_write').value)
            for i, row in enumerate(inputs):
                def setup():
                    running.setup(ref, checkpoint, row, lookup, contexts, work, env, patterns, descriptors)
                    root = row['root']
                    for address, value in [(0xf800, root['priority']), (0xf804, root['fade']['steps']),
                                           (0xf806, root['fade']['remaining']), (0xf807, row['pause']),
                                           (0xf809, root['selected']), (0xf82c, root['choice_bits'])]:
                        write_cpu(ref, 0xff0000+address, value)
                    for slot, value in enumerate(row['queue']):
                        write_cpu(ref, 0xfff80a+slot, value)
                    saved = root.get('pause_restore', {}).get('Levels', row['levels'])
                    for index in range(32):
                        write_cpu(ref, 0xfffbe0+index, 0)
                    for channel in range(6):
                        bank, ch = channel//3, channel % 3
                        for physical, logical in enumerate([0, 2, 1, 3]):
                            reg = 0x40+physical*4+ch
                            writer(0, bank*2, reg)
                            writer(0, bank*2+1, row['levels'][channel][logical])
                            write_cpu(ref, 0xfffbe0+bank*16+physical*4+ch, saved[channel][logical])
                    for offset, value, width in [(0, row['scheduler']['Delay'], 2), (2, row['scheduler']['DelayedCode'], 1), (3, row['scheduler']['Backlog'], 1), (4, row['scheduler']['Busy'], 1)]:
                        write_cpu(ref, 0xfff560+offset, value, width)
                    ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)(16, 0x219ac if row['kind'] == 'interrupt' else 0x219ea)
                    C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value = 0
                    return ref.save()
                initial = setup()
                plain_steps, plain_fields, plain_scheduler, plain_request, plain_choice, plain_entries = execute(ref, row)
                plain, ram, plain_psg = ref.save(), ref.ram_cpu(), note.psg.native_psg(ref)[1]
                assert setup()['data'] == initial['data']
                note.music.shared.attach(observer, ref)
                assert observer.score_clock_irq_enable() == 1
                address = C.addressof(ref.cpu)
                assert port.score_clock_psg_attach(address, address+CPUHead.cycles.offset, address+CPUHead.refresh_cycles.offset)
                try:
                    steps, fields, scheduler_fields, request_fields, choice_fields, entries = execute(ref, row, observer, port)
                    events, writes = note.music.shared.events(observer), note.psg.trace(port)
                finally:
                    assert port.score_clock_psg_detach() == 1 and observer.score_clock_irq_disable() == 1 and observer.score_clock_detach() == 1
                saved = ref.save()
                assert steps == plain_steps and fields == plain_fields and scheduler_fields == plain_scheduler and request_fields == plain_request and choice_fields == plain_choice and entries == plain_entries and saved['data'] == plain['data'] and saved['refresh_cycles'] == plain['refresh_cycles'] and ram == ref.ram_cpu() and plain_psg == note.psg.native_psg(ref)[1]
                result = dict(steps=steps, fields=fields, scheduler_fields=scheduler_fields, request_fields=request_fields, choice_fields=choice_fields, entries=entries, scheduler_after=scheduler_state(ref), events=[e for e in events if e['kind'] != 7 and not (e['kind'] in [17,19,20] and 0x37d66 <= e['pc'] <= 0x380a6)], psg=writes,
                              end=dict(Cycle=ref.cpu.cycles, Refresh=ref.cpu.refresh_cycles), audio_ram=ref.ram_cpu()[0xf800:0xfba0].hex(), pause_cache=cache_levels(ref))
                if name == 'original':
                    controls[i] = result
                else:
                    assert controls[i] == result, ('control core mismatch', row['id'])
                    rows.append(dict(input=row, **result, native_observer_state_equal=True))
        print(json.dumps({'core': name, 'profiles': len(inputs), 'pass': True}), flush=True)
    paths = [Path(__file__), source, candidates, ROOT/'scripts/yuyuhakusho/sound_irq_cases.py',
             ROOT/'scripts/yuyuhakusho/score-clock-running-tick-capture.py',
             ROOT/'docs/research/md-yuyuhakusho/static/audio-control.txt',
             ROOT/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt']
    payload = dict(version=1, scope='Whole sound interrupt/catchup through real driver; given entry clock, no extra IRQ acceptance claim.',
                   priorities=list(rom[0x3a50e:0x3a50e+255]), identities=identities, cases=rows,
                   sources={str(p.relative_to(ROOT)): note.psg.sha(p.read_bytes()) for p in paths})
    target = out/'native-sound-irq-full.json.gz'
    target.write_bytes(gzip.compress((json.dumps(payload, sort_keys=True, separators=(',', ':'))+'\n').encode(), mtime=0))
    print(json.dumps({'pass': True, 'profiles': len(rows), 'executions': len(rows)*4, 'sha256': note.psg.sha(target.read_bytes())}), flush=True)


if __name__ == '__main__':
    main()
