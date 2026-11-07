#!/usr/bin/env python3
"""Freeze complete native 2C9E calls before independent normalization work.

Failure scenarios and scope are frozen in verification/controller-normalization-
work/PLAN.md. No core changes, debugger, new tools or production ROM execution.
"""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path

from reference import ROOT, ReferenceSession, buttons_at, sha
from research_audio_loops import ORIGINAL, OBSERVED
from capture_controller_scan_work import NativeInput


FIELDS = ['current', 'previous', 'changed', 'pressed', 'released']
VERIFY = ROOT/'games/yuyuhakusho/verification/controller-normalization-work'


def packet_ports(packet):
    ports = []
    for port in range(2):
        base = port*40
        team = int.from_bytes(packet[base:base+2], 'big') == 16
        pads = []
        for index in range(4):
            at = base+(index+1)*8 if team else base
            if not team and index:
                pads.append({'device': 0, 'held': 0})
                continue
            kind = int.from_bytes(packet[at:at+2], 'big')
            assert kind in [4, 8], (port, index, kind)
            held = (packet[at+2]^15) | ((packet[at+3]^15)<<4)
            if kind == 8:
                held |= (packet[at+4]^15)<<8
            assert not held&~4095
            pads.append({'device': 3 if kind == 4 else 6, 'held': held})
        ports.append({'team_player': team, 'pads': pads})
    return ports


def state(seed):
    devices = [[0, 3, 6][(seed+i)%3] for i in range(8)]
    return {'devices': devices, 'controllers': [
        {name: (seed*1129+i*349+j*197)&65535 for j, name in enumerate(FIELDS)}
        for i in range(8)]}


def snapshot(ram):
    frames, devices = [], []
    for i in range(8):
        frames.append({name: ram[0xf780+i+j*8] | ram[0xf7a8+i+j*8]<<8
                       for j, name in enumerate(FIELDS)})
        flags = ram[0xf7d0+i]
        assert flags in [0, 128, 192], flags
        devices.append(0 if not flags&128 else 6 if flags&64 else 3)
    return {'controllers': frames, 'devices': devices}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    assert out.is_relative_to(VERIFY) and not out.exists()
    out.mkdir(parents=True)
    inputs = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    sources, profiles = {}, []
    for filename in ['native-controller-scan.json.gz', 'cold-io-reset-controller-scan.json.gz']:
        source = ROOT/'games/yuyuhakusho/verification/battle-sample-irq'/filename
        sources[str(source.relative_to(ROOT))] = sha(source.read_bytes())
        data = json.loads(gzip.decompress(source.read_bytes()))
        assert data['version'] == 1 and len(data['cases']) == 288
        for i, row in enumerate(data['cases']):
            packet = bytes.fromhex(row['after'])
            profiles.append({'name': f'{filename}/{i}', 'packet': packet,
                             'ports': packet_ports(packet)})
    records, source_steps, identities = [], [], {}
    for core_name, core in [('original', ORIGINAL), ('observed', OBSERVED)]:
        with ReferenceSession(ROOT, out/core_name, core=core) as ref:
            for frame in range(1, 2401):
                assert ref.step(frame, buttons_at(inputs, frame))['reference'] == expected[frame-1]
            checkpoint = ref.save()
            identities[core_name] = ref.identity
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
            run = ref.api('m68k_run', None, C.c_uint)
            setirq = ref.api('m68k_set_irq', None, C.c_uint)

            def write(address, value, size=1):
                for i, b in enumerate((value&((1<<(size*8))-1)).to_bytes(size, 'big')):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = b

            def write_packet(packet):
                assert len(packet) == 80
                for i, b in enumerate(packet):
                    write(0xf700+i, b)

            def entry(pc):
                setirq(0)
                setreg(17, 0x2700)
                setreg(15, 0xffff00)
                write(0xff00, 0x3d8, 4)
                setreg(16, pc)

            # Additional MD3 records come from the actual original scanner and
            # explicit physical three-button devices, not fabricated type words.
            md3_profiles = []
            for kinds in [(3, 3), (3, 6), (6, 3)]:
                for pattern in range(3):
                    ref.restore(checkpoint, restore_refresh_cycles=True)
                    device = ref.api('retro_set_controller_port_device', None, C.c_uint, C.c_uint)
                    for port, kind in enumerate(kinds):
                        device(port, 257 if kind == 3 else 513)
                    ref.api('io_reset', None)()
                    native = NativeInput.in_dll(ref.lib, 'input')
                    io_write = ref.api('io_68k_write', None, C.c_uint, C.c_uint)
                    for port, kind in enumerate(kinds):
                        assert native.dev[port*4] == (0 if kind == 3 else 1)
                        native.pad[port*4] = [0, 0x75, 0xf8][(pattern+port)%3]
                        io_write(1+port, 0x40)
                        io_write(4+port, 0x40)
                    for reg in range(15):
                        setreg(reg, 0)
                    entry(0x2514)
                    ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
                    for _ in range(20000):
                        pc = ref.get_register(16)&0xffffff
                        if pc == 0x3d8:
                            break
                        assert 0x2514 <= pc <= 0x2c60, hex(pc)
                        run(ref.cpu.cycles+1)
                    else:
                        raise AssertionError('MD3 scan step cap')
                    packet = ref.ram_cpu()[0xf700:0xf750]
                    ports = packet_ports(packet)
                    assert [p['pads'][0]['device'] for p in ports] == list(kinds)
                    md3_profiles.append({'name': f'physical-MD{kinds[0]}-MD{kinds[1]}/{pattern}',
                                         'packet': packet, 'ports': ports})
            if core_name == 'original':
                md3_frozen = md3_profiles
            else:
                assert md3_profiles == md3_frozen

            cases = []
            for index, profile in enumerate(profiles):
                cases.append({'profile': profile, 'count': index%9, 'seed': index,
                              'swap': [0, 1, 85, 170, 255][index%5], 'carry': (index//9)%16})
            for index, profile in enumerate(md3_profiles):
                for count in range(9):
                    cases.append({'profile': profile, 'count': count, 'seed': 700+index*9+count,
                                  'swap': [0, 1, 255][index%3], 'carry': (index+count)%16})
            # All sixteen carries, including MD3 before MD6, have whole-call
            # evidence independent of the naturally zero battle caller.
            for carry in range(16):
                cases.append({'profile': md3_profiles[3], 'count': 8, 'seed': 900+carry,
                              'swap': carry*17, 'carry': carry})
            # Change completed-scan records after old type / each base nibble /
            # extension reads. The type latch and physical location remain old.
            for pc in [0x2cb4, 0x2dce, 0x2dd2, 0x2dd6]:
                for destination in [md3_profiles[0], profiles[72], profiles[144], profiles[216]]:
                    cases.append({'profile': profiles[0], 'count': 8, 'seed': 1000+len(cases),
                                  'swap': 255, 'carry': 11, 'switch_pc': pc,
                                  'destination': destination})

            for index, case in enumerate(cases):
                profile, initial = case['profile'], state(case['seed'])
                origin, refresh = [(0, 0), (1, 896), (895, 896), (1008, 1008)][index%4]

                def setup():
                    ref.restore(checkpoint, restore_refresh_cycles=True)
                    for reg in range(15):
                        setreg(reg, (case['seed']*0x10001+reg*0x10307)&0xffffffff)
                    # Only these four bits affect the original sixteen-bit
                    # normalized storage domain; retain them explicitly.
                    setreg(1, 0x91ab0055 | case['carry']<<8)
                    entry(0x2c9e)
                    write_packet(profile['packet'])
                    write(0xf552, case['count'], 2)
                    write(0xf554, 0xa5a5, 2)
                    for channel, frame in enumerate(initial['controllers']):
                        for j, name in enumerate(FIELDS):
                            write(0xf780+channel+j*8, frame[name])
                            write(0xf7a8+channel+j*8, frame[name]>>8)
                        old = initial['devices'][channel]
                        write(0xf7d0+channel, 0 if old == 0 else 128 if old == 3 else 192)
                        write(0xff50+channel, (case['swap']>>channel)&1)
                    ref.cpu.cycles, ref.cpu.refresh_cycles = origin, refresh

                setup()
                before = ref.ram_cpu()
                assert snapshot(before) == initial
                steps, changes, switched = [], [], False
                for _ in range(2000):
                    pc = ref.get_register(16)&0xffffff
                    if pc == 0x3d8:
                        break
                    assert 0x2c9e <= pc <= 0x2e32, (index, hex(pc))
                    cycle, deadline = ref.cpu.cycles, ref.cpu.refresh_cycles
                    run(cycle+1)
                    steps.append([pc, cycle, deadline, ref.cpu.cycles, ref.cpu.refresh_cycles])
                    if not switched and pc == case.get('switch_pc'):
                        switched = True
                        write_packet(case['destination']['packet'])
                        changes.append({'after_step': len(steps), 'ports': case['destination']['ports']})
                else:
                    raise AssertionError(('normalization step cap', index))
                assert switched == ('switch_pc' in case)
                after = ref.ram_cpu()
                unknown = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and not
                           (0xf700 <= i < 0xf750 or 0xf780 <= i < 0xf7d8 or
                            0xf554 <= i < 0xf556 or 0xfec0 <= i < 0xff04)]
                assert not unknown, (index, unknown)
                stepped = (sha(after), ref.cpu.cycles, ref.cpu.refresh_cycles,
                           [ref.get_register(i) for i in range(21)])
                # Full native call, or two uninterrupted native segments around
                # the externally completed scan. No instruction observer.
                setup()
                for change in changes:
                    run(steps[change['after_step']-1][3])
                    write_packet(case['destination']['packet'])
                run(steps[-1][3])
                whole = (sha(ref.ram_cpu()), ref.cpu.cycles, ref.cpu.refresh_cycles,
                         [ref.get_register(i) for i in range(21)])
                assert whole == stepped, ('whole call differs from stepping', index)
                row = {'name': profile['name'], 'input': {
                    'ports': profile['ports'], 'channel_count': case['count'],
                    'swap_ac_mask': case['swap'], 'reserved_carry': case['carry']},
                    'before': initial, 'entry': [origin, refresh], 'after': snapshot(after),
                    'cursor': int.from_bytes(after[0xf554:0xf556], 'big'),
                    'steps': [[s[3], s[4]] for s in steps], 'changes': changes}
                if core_name == 'original':
                    records.append(row)
                    source_steps.append(steps)
                else:
                    assert row == records[index] and steps == source_steps[index], ('cross core', index)
                if index%150 == 0:
                    print(json.dumps({'core': core_name, 'case': index, 'cases': len(cases)}), flush=True)

    raw = (json.dumps({'version': 1, 'cases': records}, separators=(',', ':'))+'\n').encode()
    fixture = out/'native-controller-normalization.json.gz'
    fixture.write_bytes(gzip.compress(raw, mtime=0))
    (out/'source-steps.json.gz').write_bytes(gzip.compress(
        json.dumps(source_steps, separators=(',', ':')).encode(), mtime=0))
    legacy = ROOT/'games/yuyuhakusho/engine/testdata/input-oracle.json'
    natural = json.loads(legacy.read_bytes())
    battle = [r for r in natural if r['frame'] >= 2068 and r['six_buttons']]
    assert len(battle) == 3066 and all(not r['entry_d1']&0x0f00 for r in battle)
    report = {'pass': True, 'cases': len(records), 'steps': sum(map(len, source_steps)),
              'complete_original_2c9e_calls': True, 'whole_calls_equal_stepping': True,
              'cross_core_equal': True, 'baseline_2400_video_ram_controls_equal': True,
              'no_unexpected_RAM_writes': True, 'cores': identities,
              'fixture_sha256': sha(fixture.read_bytes()), 'packet_sources': sources,
              'source_sha256': sha(Path(__file__).read_bytes()),
              'legacy_input_sha256': sha(legacy.read_bytes()),
              'battle_natural_six_button_calls': len(battle), 'battle_carry_all_zero': True,
              'scope': 'Complete normalization including all sixteen trusted carry nibbles; physical MD3/MD6 direct and four-MD6 TeamPlayer records; transient completed-scan replacement at declared operation boundaries',
              'limits': ['No IRQ scheduling/duration claim: completed scans replace packets between native normalization operations',
                         'Inactive F7D0 bit6 without bit7 and lower reserved device-flag bits are outside the typed normalized device history domain']}
    (out/'capture.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report), flush=True)


if __name__ == '__main__':
    main()
