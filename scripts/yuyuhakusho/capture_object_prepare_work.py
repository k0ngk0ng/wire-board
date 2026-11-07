"""Freeze complete original object drawing preparation on two pinned cores."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import json
from pathlib import Path

from reference import ROOT, ReferenceSession, buttons_at, sha
from research_audio_loops import ORIGINAL, OBSERVED, BRIDGE


def word(raw, address):
    return int.from_bytes(raw[address:address + 2], 'big')


def semantic(ref):
    def n(address, size=2):
        return sum(C.c_ubyte.from_address(ref.ram_pointer + ((address + i) ^ 1)).value << (8 * (size - 1 - i)) for i in range(size))
    return {'ReservedCount': n(0xf57a) + 1, 'Count': n(0xf578),
            'WriteIndex': ((n(0xf57c) - 0xe200) & 65535) // 8,
            'FrameStartIndex': ((n(0xf57e) - 0xe200) & 65535) // 8,
            'Flags': n(0xf54f, 1)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT / '.local') and not out.exists()
    out.mkdir(parents=True)
    input_path = ROOT / 'docs/research/md-yuyuhakusho/inputs.json'
    expected_path = ROOT / 'docs/research/md-yuyuhakusho/frame-hashes.json'
    inputs, gold = json.loads(input_path.read_text()), json.loads(expected_path.read_text())
    bridge = C.CDLL(str(BRIDGE))
    callback_type = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [callback_type, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = (C.c_uint * 2)(0x125a, 0x129c)
    records, identities, natural = [], {}, []
    for core_name, core in [('original', ORIGINAL), ('observed', OBSERVED)]:
        with ReferenceSession(ROOT, out / core_name, core=core) as ref:
            pending, errors = [], []
            @callback_type
            def observe(kind, width, pc, value):
                try:
                    if pc == 0x125a:
                        assert not pending
                        pending.append({'frame': ref.frame, 'before': semantic(ref)})
                    else:
                        row = pending.pop()
                        row['after'] = semantic(ref)
                        reserved = row['before']['ReservedCount']
                        assert 1 <= reserved <= 80
                        assert row['after'] == dict(row['before'], Count=reserved, WriteIndex=reserved,
                                                    FrameStartIndex=reserved, Flags=128)
                        natural.append(row)
                except Exception as exc:
                    errors.append(repr(exc))
            if core_name == 'observed':
                assert bridge.yyh_set_observer(observe, points, len(points))
                hook = ref.api('set_cpu_hook', None, C.c_void_p)
                hook(C.cast(bridge.yyh_cpu_hook, C.c_void_p))
            for frame in range(1, 2401):
                assert ref.step(frame, buttons_at(inputs, frame))['reference'] == gold[frame - 1]
            if core_name == 'observed':
                hook(None)
                assert not pending and not errors and natural, (pending, errors)
            checkpoint = ref.save()
            identities[core_name] = ref.identity
            setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
            run = ref.api('m68k_run', None, C.c_uint)
            def write(address, value, size=2):
                for i, byte in enumerate((value & ((1 << (size * 8)) - 1)).to_bytes(size, 'big')):
                    C.c_ubyte.from_address(ref.ram_pointer + (((address + i) & 65535) ^ 1)).value = byte
            for count in range(1, 81):
                for phase, (origin, refresh) in enumerate([(0, 0), (1, 896), (895, 896), (1008, 1008)]):
                    index = (count - 1) * 4 + phase
                    ref.restore(checkpoint, restore_refresh_cycles=True)
                    for address in [*range(0xe200, 0xe480), *range(0xf300, 0xf400)]:
                        write(address, (address * 73 + index * 17 + 41) & 255, 1)
                    write(0xf57a, count - 1)
                    write(0xf578, [0, 255, 80, 65535][phase])
                    write(0xf57c, 0xe200 + ((count + phase) % 81) * 8)
                    write(0xf57e, 0xe200 + ((count * 3 + phase) % 81) * 8)
                    write(0xf54f, [0, 1, 128, 255][phase], 1)
                    for reg in range(15):
                        setreg(reg, 0)
                    setreg(17, 0x2700)
                    setreg(15, 0xffff00)
                    write(0xff00, 0x3d8, 4)
                    setreg(16, 0x125a)
                    ref.cpu.cycles, ref.cpu.refresh_cycles = origin, refresh
                    before_ram, before, steps = ref.ram_cpu(), semantic(ref), []
                    for _ in range(600):
                        pc = ref.get_register(16) & 0xffffff
                        if pc == 0x3d8:
                            break
                        assert 0x125a <= pc <= 0x129c, hex(pc)
                        live = semantic(ref)
                        run(ref.cpu.cycles + 1)
                        steps.append({'Cycle': ref.cpu.cycles, 'Refresh': ref.cpu.refresh_cycles, 'Before': live})
                    else:
                        raise AssertionError(('object prepare step cap', index))
                    assert len(steps) == 204 + 4 * count
                    after_ram, after = ref.ram_cpu(), semantic(ref)
                    assert after == dict(before, Count=count, WriteIndex=count, FrameStartIndex=count, Flags=128)
                    for layer in range(64):
                        assert int.from_bytes(after_ram[0xf300 + layer * 4:0xf304 + layer * 4], 'big') == 0xf306 + 4 * layer
                    allowed = set(range(0xf300, 0xf400)) | {0xf54f, 0xf578, 0xf579, 0xf57c, 0xf57d, 0xf57e, 0xf57f}
                    for sprite in range(count):
                        address = 0xe203 + sprite * 8
                        allowed.add(address)
                        assert after_ram[address] == sprite + 1
                    assert not [i for i, (left, right) in enumerate(zip(before_ram, after_ram)) if left != right and i not in allowed]
                    row = {'Name': f'reserved-{count}-phase-{phase}', 'Before': before, 'After': after,
                           'Clock': {'Cycle': origin, 'Refresh': refresh}, 'Steps': steps}
                    if core_name == 'original':
                        records.append(row)
                    else:
                        assert row == records[index], ('cross-core object prepare differs', index)
            print(json.dumps({'core': core_name, 'cases': len(records), 'work_steps': sum(len(row['Steps']) for row in records)}), flush=True)
    fixture = out / 'native-object-prepare-work.json.gz'
    fixture.write_bytes(gzip.compress((json.dumps({'Version': 1, 'Cases': records}, separators=(',', ':')) + '\n').encode(), mtime=0))
    natural_path = out / 'natural-calls.json.gz'
    natural_path.write_bytes(gzip.compress((json.dumps(natural, separators=(',', ':')) + '\n').encode(), mtime=0))
    source_paths = [input_path, expected_path, Path(__file__), ROOT / 'docs/research/md-yuyuhakusho/static/input-detail.txt',
                    ROOT / 'docs/research/md-yuyuhakusho/static/sprite-submit.txt', ROOT / 'docs/research/md-yuyuhakusho/static/world-sprites-raster-source.txt']
    report = {'pass': True, 'cases': len(records), 'steps': sum(len(row['Steps']) for row in records),
              'cross_core_work_and_outcomes_equal': True, 'all_2400_video_ram_controls_equal_per_core': True,
              'all_64_layer_heads_and_retained_link_bytes_equal': True, 'no_unowned_ram_writes': True,
              'natural_calls': len(natural), 'natural_reserved_counts': dict(Counter(row['before']['ReservedCount'] for row in natural)),
              'identity': identities, 'fixture_sha256': sha(fixture.read_bytes()), 'natural_sha256': sha(natural_path.read_bytes()),
              'sources': {str(path.relative_to(ROOT)): sha(path.read_bytes()) for path in source_paths},
              'scope': 'Complete original 125A under mask7, all 1..80 retained counts and four refresh phases. Natural counts are entry observations on the fixed 2400-frame scenario. H40 capacity80 is from the existing raster/source evidence; private counts are not claimed naturally reachable. Caller/IRQ/audio integration is separate.'}
    (out / 'capture.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report), flush=True)


if __name__ == '__main__':
    main()
