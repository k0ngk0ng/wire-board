"""Read original sound-driver semantics for the gameplay alias at $00ff.

Development evidence only: executes unchanged bank-0 instructions in the
existing py65 harness, never included in product runtime or media playback.
Failure cases checked before any semantic implementation: incomplete writer
inventory, externally read device state, drift from retained music+shot trace,
loss of carried scratch on reset/pause, or disagreement with FCEUX driver RAM.
No ROM patch, PCM recording, installation, or product source changes.
"""
import collections
import csv
import hashlib
import json
from pathlib import Path
from verify_pass_floor_original import ROOT, Memory, MPU
from py65.disassembler import Disassembler

OUT = ROOT / '.local/basketball/audio/work-byte'
ROM_PATH = ROOT / '.local/nes-basket/basketball.nes'
EXPECTED_ROM = '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
WRITERS = {0x80b0, 0x8275, 0x832a, 0x8364, 0x846d, 0x8746, 0x880d, 0x888e, 0x88f2, 0x8968}


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class DriverMemory(Memory):
    def __init__(self, prg):
        super().__init__(prg, bytes(2048))
        self[0x8000:0xc000] = prg[:16384]
        self.current_pc = 0
        self.writes = []
        self.device_reads = set()
        self.ram_reads = set()

    def __getitem__(self, address):
        if isinstance(address, int):
            if 0x2000 <= address < 0x8000:
                self.device_reads.add(address)
            if address < 0x800:
                self.ram_reads.add(address)
        return super().__getitem__(address)

    def __setitem__(self, address, value):
        if isinstance(address, int) and address == 0xff:
            self.writes.append({'pc': f'{self.current_pc:04x}', 'channel': list.__getitem__(self, 0x704), 'value': value})
        if isinstance(address, int) and 0x4000 <= address <= 0x4017:
            return list.__setitem__(self, address, value)
        return super().__setitem__(address, value)


def call(memory, entry, argument=0):
    cpu = MPU(memory=memory, pc=entry)
    cpu.a, cpu.sp = argument, 255
    cpu.stPushWord(0x6fff)
    before = list.__getitem__(memory, 0xff)
    start = len(memory.writes)
    for steps in range(30000):
        if cpu.pc == 0x7000:
            break
        memory.current_pc = cpu.pc
        cpu.step()
    else:
        raise RuntimeError(f'driver did not return: {cpu.pc:04x}')
    return {'entry': f'{entry:04x}', 'argument': argument, 'before': before,
            'after': list.__getitem__(memory, 0xff), 'writes': memory.writes[start:],
            'flags': list.__getitem__(memory, 0x7e5), 'tempo': list.__getitem__(memory, 0x705),
            'tempo_phase': list.__getitem__(memory, 0x706), 'instruction_count': steps}


def compare_fceux(prg, directory, limit=600):
    """Reuse original recorded states, no new FCEUX capture or APU emulation."""
    calls = list(csv.DictReader((directory / 'calls.csv').open()))
    raw = (directory / 'driver-ram.bin').read_bytes()
    width = 0x7e7 - 0x700
    assert len(raw) % width == 0
    # The recording logs registerexec frame IDs. Each frameadvance then saves
    # its completed driver state; startup RAM before init has the NES power-on
    # pattern, so align the first full post-init state within ten saved frames.
    memory = DriverMemory(prg)
    states = [bytes(memory[0x700:0x7e7])]
    groups = collections.defaultdict(list)
    for event in calls:
        groups[int(event['frame'])].append(event)
    for frame in sorted(groups):
        for event in groups[frame]:
            call(memory, 0x8000 if event['kind'] == 'init' else 0x8003, int(event['value']))
        states.append(bytes(memory[0x700:0x7e7]))
        if len(states) >= limit:
            break
    observed = [raw[offset:offset+width] for offset in range(0, min(len(raw), width*limit), width)]
    candidate = states[1:]
    matches = [index for index, value in enumerate(observed[:10]) if value == candidate[0]]
    assert len(matches) == 1, {'directory': str(directory), 'initial_state_matches': matches}
    first = matches[0]
    count = min(len(candidate), len(observed)-first)
    for index in range(count):
        if observed[first+index] != candidate[index]:
            differences = [{'address': f'{0x700+j:04x}', 'fceux': a, 'isolated': b}
                           for j, (a,b) in enumerate(zip(observed[first+index], candidate[index])) if a != b]
            raise AssertionError({'directory': str(directory), 'step': index, 'differences': differences})
    return {'directory': str(directory.relative_to(ROOT)), 'driver_sha256': sha(directory/'driver-ram.bin'),
            'calls_sha256': sha(directory/'calls.csv'), 'matched_states': count, 'pre_init_states': first,
            'scope': '$0700..07e6 matches existing FCEUX capture; original capture does not include $00ff'}


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    rom = ROM_PATH.read_bytes()
    assert hashlib.sha256(rom).hexdigest() == EXPECTED_ROM
    prg = rom[16:16+131072]
    reference_path = ROOT / '.local/basketball/rea/special-turn-alias/music-shot-work.json'
    reference = json.loads(reference_path.read_text())
    cases, observed, counts, memories = [], [], collections.Counter(), []
    for music in range(59, 67):
        memory = DriverMemory(prg)
        rows = [dict(kind='request', **call(memory, 0x8000, 0)), dict(kind='request', **call(memory, 0x8000, music))]
        for tick in range(90):
            rows.append(dict(kind='tick_before_shot', tick=tick+1, **call(memory, 0x8003)))
        rows.append(dict(kind='request', **call(memory, 0x8000, 30)))
        for tick in range(180):
            row = dict(kind='tick_after_shot', tick=tick+1, **call(memory, 0x8003))
            rows.append(row)
            observed.append({'music': music, 'frameAfterShot': tick+1, 'soundWork': row['after']})
        for row in memory.writes:
            counts[row['pc']] += 1
            assert int(row['pc'], 16) in WRITERS, row
        # Sentinel return-PC reads are not device data reads: execution stops
        # before fetching the sentinel instruction at $7000.
        assert not memory.device_reads, memory.device_reads
        cases.append({'music': music, 'rows': rows})
        memories.append(memory)
    assert observed == reference
    (OUT/'music-shot-writers.json').write_text(json.dumps(cases, indent=2)+'\n')
    control_memory = DriverMemory(prg)
    list.__setitem__(control_memory, 0xff, 0x5a)
    controls = [dict(kind='reset_with_carried_work', **call(control_memory, 0x8000, 0))]
    assert controls[-1]['after'] == 0x5a and controls[-1]['writes'] == []
    controls.append(dict(kind='music61', **call(control_memory, 0x8000, 61)))
    for _ in range(90):
        call(control_memory, 0x8003)
    for ident, name in [(30,'shot30'), (253,'pause'), (255,'resume'), (0,'reset')]:
        controls.append(dict(kind=name, **call(control_memory, 0x8000, ident)))
        for tick in range(5):
            controls.append(dict(kind=name+'_tick', tick=tick+1, **call(control_memory, 0x8003)))
    (OUT/'control-writers.json').write_text(json.dumps(controls, indent=2)+'\n')
    fceux = [compare_fceux(prg, ROOT/'.local/basketball/audio/export'/directory) for directory in [
        'additional-sound-030-run-1', 'court-music-061-20000-run-1']]
    memory = DriverMemory(prg)
    disassembler = Disassembler(MPU(memory=memory))
    static_writers = []
    pc = 0x8000
    while pc < 0x897b:
        if pc == 0x836e:  # 32 low bytes and 32 high bytes for commands C0..DF
            pc = 0x83ae
        length, assembly = disassembler.instruction_at(pc)
        if assembly.split(' ')[0] in {'STA','STX','STY','INC','DEC','ASL','LSR','ROL','ROR'} and assembly.endswith(' $ff'):
            static_writers.append({'pc': f'{pc:04x}', 'instruction': assembly,
                                   'bytes': bytes(memory[pc:pc+length]).hex()})
        pc += length
    assert {int(item['pc'],16) for item in static_writers} == WRITERS
    report = {'scope': 'isolated unchanged original driver; not natural whole-game reachability or product implementation',
        'rom_sha256': EXPECTED_ROM, 'reference_trace': str(reference_path.relative_to(ROOT)),
        'reference_sha256': sha(reference_path), 'matched_music_shot_steps': len(observed),
        'static_writers': static_writers, 'dynamic_writer_counts': dict(counts),
        'fceux_state_comparisons': fceux, 'device_state_reads': [],
        'work_trace': {'path': str((OUT/'music-shot-writers.json').relative_to(ROOT)), 'sha256': sha(OUT/'music-shot-writers.json')},
        'control_trace': {'path': str((OUT/'control-writers.json').relative_to(ROOT)), 'sha256': sha(OUT/'control-writers.json')},
        'note': '$00ff is carried scratch, including reset/pause paths with no writer. A future semantic model must retain the last value.'}
    (OUT/'probe-summary.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
