#!/usr/bin/env python3
"""Capture FE68's complete shared-cell writes; development oracle only."""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    out = parser.parse_args().out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT / '.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = (C.c_uint * 2)(0xfe68, 0xfe76)
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    inputs = json.loads((ROOT / 'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT / 'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    library_path = ROOT / 'games/yuyuhakusho/engine/testdata/match-start-library.json'
    library = json.loads(library_path.read_text())
    natural, isolated, errors = [], [], []
    pending, private, label = None, False, ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                if pc == 0xfe68:
                    assert pending is None
                    source = ref.get_register(9) & 0xffffff
                    words, address = [], source
                    while int.from_bytes(rom[address:address + 2], 'big') != 0xffff:
                        words.append(int.from_bytes(rom[address:address + 2], 'big'))
                        address += 2
                        assert len(words) <= 16
                    pending = {'frame': ref.frame, 'name': label, 'source': source,
                               'offset': ref.get_register(0) & 65535, 'words': words,
                               'entry_sp': ref.get_register(15),
                               'before': ref.ram_cpu()[0xe600:0xec00].hex()}
                else:
                    assert pending is not None and ref.get_register(15) == pending['entry_sp']
                    pending['after'] = ref.ram_cpu()[0xe600:0xec00].hex()
                    (isolated if private else natural).append(pending)
                    pending = None
                    if private:
                        ref.cpu.cycles = 8000000
            except Exception as error:
                errors.append(repr(error))
                if private:
                    ref.cpu.cycles = 8000000

        assert bridge.yyh_set_observer(observe, points, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(inputs, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and pending is None and not errors, (errors, pending)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, raw):
            for i, byte in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer + (((address + i) & 65535) ^ 1)).value = byte

        def invoke(name, offset, source):
            nonlocal label
            label = name
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            write(0xe600, bytes((i * 73 + len(isolated) * 11 + 17) & 255 for i in range(1536)))
            setreg(0, offset)
            setreg(9, source)
            setreg(17, 0x2700)
            setreg(15, 0xffffff00)
            write(0xff00, (0x3d8).to_bytes(4, 'big'))
            setreg(16, 0xfe68)
            ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
            count = len(isolated)
            hook(observer)
            execute(6000000)
            hook(None)
            assert len(isolated) == count + 1 and pending is None and not errors, (name, errors)

        for layout, offsets in enumerate(library['hud_offsets'], 1):
            for slot, offset in enumerate(offsets):
                for controller in range(5):
                    invoke(f'layout-{layout}-slot-{slot}-controller-{controller}', offset + 0x102, 0xfd5c + controller * 6)
                invoke(f'layout-{layout}-slot-{slot}-health', offset, 0xfd86)
                invoke(f'layout-{layout}-slot-{slot}-energy', offset + 0x80, 0xfdaa)
        for source, size in [(0xfd86, 9), (0xfdaa, 9), (0xfd5c, 2)]:
            for offset in [0, 2, 1536 - size * 2]:
                invoke(f'shared-cell-boundary-{source:x}-{offset}', offset, source)
        identity = ref.identity
    rows = natural + isolated
    for row in rows:
        before, after = bytes.fromhex(row['before']), bytes.fromhex(row['after'])
        candidate = bytearray(before)
        start = row['offset']
        candidate[start:start + len(row['words']) * 2] = b''.join(word.to_bytes(2, 'big') for word in row['words'])
        assert bytes(candidate) == after, row['name']
    raw = (json.dumps({'natural': natural, 'isolated': isolated}, separators=(',', ':')) + '\n').encode()
    zipped = gzip.compress(raw, mtime=0)
    (out / 'match-start-cells.json.gz').write_bytes(zipped)
    report = {'pass': True, 'scope': 'FE68 complete E600..EC00 before/after; explicit words and byte offsets',
              'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()),
              'library_sha256': sha(library_path.read_bytes()), 'bridge_sha256': sha(bridge_path.read_bytes()),
              'natural_calls': len(natural), 'isolated_calls': len(isolated),
              'frames_match_original_core': frames == expected, 'fixture_sha256': sha(raw),
              'gzip_sha256': sha(zipped), 'incomplete_calls': int(pending is not None), 'errors': errors}
    (out / 'match-start-cells-manifest.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
