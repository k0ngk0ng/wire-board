"""Native FM/PSG producer-order calls on independent raw input frontiers."""
import argparse
import base64
import ctypes as C
import gzip
import json
from pathlib import Path
from importlib.util import spec_from_file_location, module_from_spec

from reference import ROOT
from extract_audio_assets import AudioSession
from inspect_dac_dsp_state import Config, symbols
from fmpsg_capture_ym import ORIGINAL, PINNED, reg, tone, sha

spec = spec_from_file_location('lane_reset', ROOT/'scripts/yuyuhakusho/fmpsg-reset-capture.py')
reset = module_from_spec(spec)
spec.loader.exec_module(reset)
F = reset.F


def cases():
    setup = sum((tone(ch, ch % 8, ch % 8, ch, 8+ch % 8) for ch in range(6)), [])
    start = reset.write(0, setup)+reset.psg(0, [0x81, 0, 0x90, 0xe7, 0xf0])
    dac = reset.write(0, reg(0, 0x2a, 115)+reg(0, 0x2b, 128)+reg(1, 0xb6, 192))
    return [
        dict(Name='driver-lazy-ym-after-psg', Operations=start+reset.write(104903, reg(0, 0x42, 28))
             +reset.psg(118105, [0xf6])+reset.write(105495, [[0, 0x2b]])+reset.write(105555, [[1, 0]])
             +[reset.read(123980)]+reset.psg(123981, [0x93])+reset.write(124000, reg(0, 0x42, 49))+[reset.frame(), reset.frame()]),
        dict(Name='psg-after-later-ym', Operations=start+reset.write(50000, reg(0, 0x28, 0))
             +reset.psg(1000, [0x83, 2, 0x94])+reset.write(60000, reg(0, 0x28, 0xf0))
             +reset.psg(20000, [0x98])+[reset.frame(), reset.frame()]),
        dict(Name='same-time-two-domains', Operations=start+reset.psg(4000, [0x91])+reset.write(4000, reg(0, 0x40, 30))
             +[reset.read(4000), reset.reset(4000)]+reset.psg(4000, [0x92])+[reset.frame()]),
        dict(Name='ym-reset-behind-psg', Operations=start+[reset.read(10000)]+reset.psg(90000, [0x97])
             +[reset.reset(30000), reset.read(30000)]+reset.psg(90001, [0x99])+[reset.frame(), reset.frame()]),
        dict(Name='dac-reset194-behind-psg', Operations=dac+reset.write(1202, reg(0, 0x2a, 123))
             +reset.psg(6000, [0x81, 0, 0x90])+[reset.after(1008, 1202), reset.read(1202)]
             +reset.write(4000, reg(0, 0x2b, 128)+reg(0, 0x2a, 144))+[reset.frame(), reset.frame()]),
        dict(Name='psg-ahead-frame-carry', Operations=start+reset.psg(F+193, [0x95])
             +reset.write(F-100, reg(0, 0x40, 63))+[reset.frame(), reset.read(F)]
             +reset.write(F+100, reg(0, 0x40, 18))+reset.psg(F+300, [0x93])+[reset.frame(), reset.frame()]),
        dict(Name='late-shared-address-latch', Operations=start+reset.psg(14000, [0xb2])
             +reset.write(1000, [[2, 0xb6]])+reset.write(1001, [[1, 64]])
             +reset.write(1200, reg(0, 0x22, 15))+[reset.read(1300)]+reset.psg(14001, [0xb4])+[reset.frame()]),
        dict(Name='repeated-reset-with-psg-ahead', Operations=start+reset.psg(80000, [0xf3])
             +[reset.reset(10000), reset.reset(10000), reset.read(10000)]
             +reset.write(20000, reg(0, 0x2a, 200))+[reset.reset(20000)]+reset.psg(80001, [0xf6])+[reset.frame(), reset.frame()]),
    ]


class LaneOracle(reset.Oracle):
    def __init__(self, ref, lookup):
        super().__init__(ref, lookup, 0)
        self.ymcursor = 0

    def snapshot(self):
        state = super().snapshot()
        lag = self.cursor-self.ymcursor
        assert lag >= 0
        if lag:
            assert self.cursor == self.psgcursor
            state['YMLag'] = lag
        return state

    def apply_lane(self, op, observe):
        kind, cycle = op['Kind'], op.get('Cycle', 0)
        value, pcm, committed = 0, '', None
        if kind == 'frame':
            self.nativeend.value = F
            buf = (C.c_int16*8820)()
            count = self.update(buf)
            pcm = base64.b64encode(C.string_at(buf, count*4)).decode()
            self.base += F
            self.ymcursor = max(self.ymcursor, self.base)
            self.psgcursor = max(self.psgcursor, self.base)
            self.cursor = max(self.ymcursor, self.psgcursor)
            self.count = 0
            self.ref.frame = self.base//F
        else:
            assert self.base <= cycle <= self.base+F+1008
            if kind == 'psg':
                assert cycle >= self.psgcursor
                self.pwrite(cycle-self.base, op['Value'])
                self.psgcursor = cycle
            else:
                if kind == 'after-dac':
                    assert op['CompletedDAC'] == self.ymcursor and 0 <= self.ymcursor-cycle <= 194
                else:
                    assert cycle >= self.ymcursor
                local = cycle-self.base
                if kind == 'write':
                    self.writer(local, op['Port'], op['Value'])
                elif kind == 'read':
                    value = self.reader(local, op.get('Port', 0))
                else:
                    self.reset(local)
                self.ymcursor = max(self.ymcursor, cycle)
            self.cursor = max(self.ymcursor, self.psgcursor)
            self.count += 1
        state = self.snapshot() if observe or kind == 'frame' else None
        if kind == 'frame' and not state.get('YMLag', 0):
            committed = self.committed(state)
        return dict(Kind=kind, Cycle=cycle, CompletedDAC=op.get('CompletedDAC', 0), Port=op.get('Port', 0),
                    Value=op.get('Value', 0), Read=value, Snapshot=state, Committed=committed, PCM=pcm)


def fresh(ref):
    ref.api('YM2612Init', None)()
    ref.api('YM2612Config', None, C.c_int)(0)
    ref.api('psg_init', None, C.c_int)(1)
    ref.api('sound_reset', None)()
    ref.api('audio_reset', None)()
    ref.frame = 0


def capture(core, out):
    rows, projections = [], 0
    lookup = symbols(core)
    with AudioSession(ROOT, out, core=core) as ref:
        cfg = Config.in_dll(ref.lib, 'config')
        assert (cfg.hq_fm, cfg.hq_psg, cfg.fm_preamp, cfg.psg_preamp, cfg.filter, cfg.ym2612) == (1, 1, 100, 150, 0, 0)
        for case in cases():
            fresh(ref)
            observed = LaneOracle(ref, lookup)
            initial = observed.committed(observed.snapshot())
            operations = [observed.apply_lane(op, True) for op in case['Operations']]
            ending, ram, psg = ref.save(), ref.ram_cpu(), C.string_at(C.addressof(observed.psg), C.sizeof(observed.psg))
            projections += observed.checks
            fresh(ref)
            plain = LaneOracle(ref, lookup)
            no_projection = [plain.apply_lane(op, False) for op in case['Operations']]
            after = ref.save()
            assert ending['data'] == after['data'] and ending['refresh_cycles'] == after['refresh_cycles'] and ram == ref.ram_cpu()
            assert psg == C.string_at(C.addressof(plain.psg), C.sizeof(plain.psg))
            assert [(op['Read'], op['PCM']) for op in operations] == [(op['Read'], op['PCM']) for op in no_projection]
            rows.append(dict(Name=case['Name'], Config=dict(Profile=1, MasterOrigin=0), Initial=initial, Operations=operations))
    return rows, projections


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT/'.local') and not out.exists()
    out.mkdir(parents=True)
    original, count = capture(ORIGINAL, out/'original')
    observed, other = capture(PINNED, out/'observed')
    assert original == observed and count == other
    raw = (json.dumps(dict(Cases=original), separators=(',', ':'))+'\n').encode()
    packed = gzip.compress(raw, mtime=0)
    (out/'native.json.gz').write_bytes(packed)
    report = dict(pass_=True, cases=len(original), executions=len(original)*4, projection_checks_per_core=count,
                  operations=sum(len(r['Operations']) for r in original), frames=sum(o['Kind'] == 'frame' for r in original for o in r['Operations']),
                  stereo_frames=sum(len(base64.b64decode(o['PCM']))//4 for r in original for o in r['Operations'] if o['Kind'] == 'frame'),
                  producer_sha256=sha(Path(__file__).read_bytes()), raw_sha256=sha(raw), gzip_sha256=sha(packed),
                  original_core_sha256=sha(ORIGINAL.read_bytes()), observed_core_sha256=sha(PINNED.read_bytes()),
                  scope='Original callback order, per-chip monotonic clocks, no global sort; full YM/PSG/FIR/PCM, projection-on/off and cross-core equality.')
    report['pass'] = report.pop('pass_')
    (out/'native-capture.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report), flush=True)


if __name__ == '__main__':
    main()
