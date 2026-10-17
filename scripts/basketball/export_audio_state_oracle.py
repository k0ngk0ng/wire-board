"""Create independent silent-driver test oracles before Go implementation.

Failure scenarios: lost carried work at reset; effect advancement while paused;
wrong channel ordering, tempo carry, multiply-zero behavior, note envelopes,
priority rejection, repeated requests, nested percussion, source loops/returns,
finite end, restoration, and unknown requests. The development-only reference
executes unchanged original instructions; no CPU/APU enters the Go package.
"""
import base64
import gzip
import hashlib
import json
from probe_audio_work_byte import ROOT, ROM_PATH, EXPECTED_ROM, DriverMemory, call


def main():
    rom = ROM_PATH.read_bytes()
    assert hashlib.sha256(rom).hexdigest() == EXPECTED_ROM
    prg = rom[16:16+131072]
    cases = []

    def scenario(name, schedule, work=0x5a):
        memory = DriverMemory(prg)
        list.__setitem__(memory, 0xff, work)
        rows = []
        for kind, value in schedule:
            repetitions = value if kind == 'ticks' else 1
            for _ in range(repetitions):
                call(memory, 0x8003 if kind == 'ticks' else 0x8000, 0 if kind == 'ticks' else value)
                rows.append({'kind': 'tick' if kind == 'ticks' else 'request', 'id': 0 if kind == 'ticks' else value,
                    'work': list.__getitem__(memory, 0xff),
                    'driver': base64.b64encode(bytes(memory[0x700:0x7e7])).decode()})
                memory.writes.clear()
        assert not memory.device_reads
        cases.append({'name': name, 'initial_work': work, 'steps': rows})

    for music in range(59, 67):
        scenario(f'music{music}-shot30', [('request',0),('request',music),('ticks',90),('request',30),('ticks',180)])
    scenario('controls-and-priority', [('request',0),('ticks',3),('request',61),('ticks',90),('request',30),('ticks',5),
        ('request',253),('ticks',12),('request',255),('ticks',30),('request',30),('request',3),('request',31),('ticks',60),
        ('request',253),('request',66),('ticks',90),('request',0),('ticks',5),('request',61),('request',254),('ticks',12),
        ('request',252),('ticks',4),('request',255),('ticks',90),('request',251),('ticks',12)])
    for ident in range(1,77):
        scenario(f'sound{ident}-alone', [('request',0),('request',ident),('ticks',600)])
    # Cover the finite ending and the intro/loop transition without rerecording.
    scenario('ending75-complete',[('request',0),('request',75),('ticks',9000)])
    scenario('ending76-intro-loop',[('request',0),('request',76),('ticks',20000)])
    out = ROOT/'games/basketball/audiostate/testdata'
    out.mkdir(parents=True, exist_ok=True)
    data = {'schema':1,'rom_sha256':EXPECTED_ROM,'scope':'isolated original driver, complete source-control state after each request/tick; not PCM or world scheduling', 'cases':cases}
    packed = gzip.compress((json.dumps(data,separators=(',',':'))+'\n').encode(),mtime=0)
    (out/'original-driver.json.gz').write_bytes(packed)
    metadata = {'rom_sha256':EXPECTED_ROM,'oracle_sha256':hashlib.sha256(packed).hexdigest(),
        'generator_sha256':hashlib.sha256(__import__('pathlib').Path(__file__).read_bytes()).hexdigest(),
        'cases':len(cases),'steps':sum(len(case['steps']) for case in cases),'size':len(packed),
        'no_apu_state_reads':True,'runtime_cpu_emulation':False}
    (out/'original-driver-source.json').write_text(json.dumps(metadata,indent=2)+'\n')
    print(json.dumps(metadata))


if __name__ == '__main__':
    main()
