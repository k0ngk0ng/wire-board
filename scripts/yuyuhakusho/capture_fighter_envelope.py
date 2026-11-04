#!/usr/bin/env python3
"""Original input dispatch/resumption, projection, HUD and registration evidence."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

ENTRIES = {0x15540: 'input', 0x15594: 'record', 0x156f0: 'project', 0x15772: 'hud', 0x14922: 'register'}
RETURNS = {0x15590, 0x155e6, 0x15628, 0x156b4, 0x156ce, 0x156ee, 0x1574e, 0x158ae, 0x1492e}


def n(raw, off, size, signed=False):
    return int.from_bytes(raw[off:off+size], 'big', signed=signed)


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    args = p.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    bridge_path = ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'
    bridge = C.CDLL(str(bridge_path))
    cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    points = sorted(set(ENTRIES)|RETURNS|{0x156e8})
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, isolated, streams, errors = [], [], [], []
    pending = {}
    private, label, requested = False, '', None
    with ReferenceSession(ROOT, out, core=core) as ref:
        # The recorded channel alias can address ROM bytes0..6. Confirm the
        # actual selected mapping uses the pinned core's no-op byte writer.
        unused_writer = C.cast(ref.lib.m68k_unused_8_w, C.c_void_p).value
        rom_writes_ignored = ref.cpu.memory_map[3] == unused_writer
        assert rom_writes_ignored
        def snapshot(kind, address, stream_base=None):
            ram = ref.ram_cpu()
            off = address&0xffff
            obj = ram[off:off+256]
            assert len(obj) == 256
            row = {'actor': obj.hex()}
            if kind in ('input', 'record'):
                pointer = n(obj, 0xca, 4)
                if stream_base is None:
                    stream_base = pointer&~1023 if pointer else 0xff6000
                if stream_base < len(rom):
                    source = rom[stream_base:stream_base+1024]
                else:
                    assert 0xff0000 <= stream_base <= 0xfffc00, hex(stream_base)
                    source = ram[stream_base&0xffff:(stream_base&0xffff)+1024]
                channel = obj[0x76]
                frame = {key: (ram[0xf780+channel+i*8] | ram[0xf7a8+channel+i*8]<<8) if channel < 8 else 0
                         for i, key in enumerate(['current', 'previous', 'changed', 'pressed', 'released'])}
                row.update(stream_base=stream_base, stream=source.hex(), input=frame, disabled=ram[0xa032], stopped=ram[0xa09a], markers=list(ram[0xfffc:]))
            elif kind == 'project':
                row.update(camera_x=n(ram, 0xa014, 2, True), camera_y=n(ram, 0xa016, 2, True), shake=n(ram, 0xf566, 2, True))
            elif kind == 'hud':
                row.update(cells=ram[0xe602:0xea02].hex(), queue=ram[0xd400:0xe000].hex(), cursor=n(ram, 0xf518, 2), blink=ram[0xf585])
            else:
                row.update(owners=ram[0xa880:0xa900].hex(), used=n(ram, 0xa034, 2))
            return row

        def finish(key, route='complete', exit_pc=0):
            row = pending.pop(key)
            row['after'] = snapshot(row['kind'], row['object_id'], row['before'].get('stream_base'))
            row.update(route=route, exit=hex(exit_pc), exit_frame=ref.frame)
            if private:
                row['name'] = label
                isolated.append(row)
                ref.cpu.cycles = 4000000
            else:
                natural.append(row)

        @cbtype
        def observe(kind, width, pc, value):
            try:
                address, sp = ref.get_register(13)&0xffffff, ref.get_register(15)
                if pc in ENTRIES and (not private or ENTRIES[pc] == requested):
                    scope = ENTRIES[pc]
                    key = (scope, address, sp)
                    assert key not in pending, ('nested envelope', key)
                    pending[key] = {'kind': scope, 'object_id': address&0xffff, 'frame': ref.frame,
                                    'entry_sp': sp, 'before': snapshot(scope, address)}
                if pc == 0x156e8:
                    for key, row in list(pending.items()):
                        if row['kind'] == 'input' and key[1] == address:
                            mode = bytes.fromhex(row['before']['actor'])[0xc9]
                            assert sp == row['entry_sp']-(4 if mode >= 128 else 0)
                            finish(key, 'ai-record' if mode >= 128 else 'ai', pc)
                elif pc in RETURNS:
                    for key, row in list(pending.items()):
                        if key[1] == address and key[2] == sp:
                            valid = (row['kind'] in ('input', 'record') and pc in RETURNS&{0x15590, 0x155e6, 0x15628, 0x156b4, 0x156ce, 0x156ee}) or \
                                    (row['kind'] == 'project' and pc == 0x1574e) or (row['kind'] == 'hud' and pc == 0x158ae) or (row['kind'] == 'register' and pc == 0x1492e)
                            if valid:
                                finish(key, exit_pc=pc)
            except Exception as exc:
                errors.append(str(exc))
                if private:
                    ref.cpu.cycles = 4000000
        assert bridge.yyh_set_observer(observe, pcs, len(points))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p)
        hook(observer)
        frames = []
        for frame in range(1, 3601):
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
            if frame == 2400:
                checkpoint = ref.save()
        hook(None)
        assert frames == expected and not pending and not errors, (len(natural), errors[:8], list(pending)[:4])
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, byte in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def write_bytes(address, raw):
            for i, byte in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte

        def setup(seed=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            write_bytes(0xff8400, bytes((i*73+seed*19+17)&255 for i in range(256)))
            write_bytes(0xff6000, bytes((i*31+seed*13+9)&255 for i in range(1024)))
            write_bytes(0xffe602, bytes((i*19+seed*23+91)&255 for i in range(1024)))
            write_bytes(0xffd400, bytes((i*37+seed*29+13)&255 for i in range(3072)))
            write_bytes(0xffa880, bytes((i*47+seed*17+11)&255 for i in range(128)))
            write_bytes(0xfffffc, bytes([1, 0x22, 0x44, 0x7f]))

        def invoke(scope, name):
            nonlocal label, requested
            label, requested = name, scope
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xffff8400)
            setreg(16, next(pc for pc, kind in ENTRIES.items() if kind == scope))
            ref.cpu.cycles = 0
            ref.cpu.refresh_cycles = 896
            before = ref.ram_cpu()
            count = len(isolated)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(isolated) == count+1 and not pending and not errors, (scope, name, errors[:3], list(pending))
            after = ref.ram_cpu()
            allowed = set(range(0x8400, 0x8500)) | set(range(0xff80, 0xffe4))
            if scope in ('input', 'record'):
                allowed |= set(range(0x6000, 0x6400)) | {0xa09a} | set(range(0xfffc, 0x10000))
            elif scope == 'hud':
                allowed |= set(range(0xd400, 0xe000)) | set(range(0xe602, 0xea02)) | {0xf518, 0xf519}
            elif scope == 'register':
                allowed |= set(range(0xa880, 0xa900)) | {0xa034, 0xa035}
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return isolated[-1]

        profiles = [(0, 0, 0), (0x80, 0x80, 0x80), (0x10, 0x40, 0x20), (0x8f, 0x70, 0x50), (0xff, 0xff, 0xff), (0x12, 0x14, 0x16)]
        for seed, (mode, channel, retain, stopped, disabled, profile) in enumerate(itertools.product([-128, -1, 0, 1, 127], list(range(8))+[255], [0, 1], [0, 1], [0, 1], range(len(profiles)))):
            setup(seed)
            write(0xff84c9, mode)
            write(0xff8476, channel)
            write(0xff8475, retain)
            write(0xff84ca, 0 if seed%7 == 0 and mode > 0 else 0xff6000+(seed%4)*4, 4)
            write(0xffa032, disabled)
            write(0xffa09a, stopped)
            if channel < 8:
                current, pressed, changed = profiles[profile]
                for off, value in [(0xf780, current), (0xf798, pressed), (0xf790, changed), (0xf7a8, seed&255), (0xf7c0, (seed*3)&255), (0xf7b8, (seed*7)&255)]:
                    write(0xff0000+off+channel, value)
            invoke('input', f'input-{mode}-{channel}-{retain}-{stopped}-{disabled}-{profile}')
        for seed, (channel, stopped, current) in enumerate(itertools.product(list(range(8))+[255], [0, 1, 255], [0, 0x80, 255])):
            setup(seed)
            write(0xff84c9, -1)
            write(0xff8476, channel)
            write(0xff84ca, 0xff6000+(seed%4)*4, 4)
            write(0xffa09a, stopped)
            if channel < 8:
                write(0xfff780+channel, current)
            invoke('record', f'record-{channel}-{stopped}-{current}')
        actual = [-1024, -1, 0, 1, 31, 32, 111, 112, 199, 200, 222, 223, 224, 32767]
        display = [0, 1, 31, 32, 111, 112, 199, 200, 222, 223, 224, 255]
        for seed, (health, shown, blink, target) in enumerate(itertools.product(actual, display, [0, 1], [0, 4, 12, 255])):
            setup(seed)
            for off, value, size in [(0x78, health, 2), (0x7a, shown, 2), (0xb8, actual[(seed//4)%len(actual)], 2),
                                     (0xbc, display[(seed//3)%len(display)], 2), (0x8a, [0, 0x40, 0x100, 0x200][seed%4], 2), (0x70, target, 1)]:
                write(0xff8400+off, value, size)
            write(0xfff585, blink)
            write(0xfff518, [0xd400, 0xd410, 0xdfe0][seed%3], 2)
            invoke('hud', f'hud-{health}-{shown}-{blink}-{target}')
        for seed, (used, owner) in enumerate(itertools.product([0, 2, 30, 126], FIGHTER_IDS := [0x8400, 0x8800, 0x8c00, 0x9000])):
            setup(seed)
            write(0xffa034, used, 2)
            # Registration reads only A5's low word, not its pointed actor.
            label, requested = f'register-{used}-{owner:x}', 'register'
            setreg(17, 0x2700); setreg(15, 0xffffe0); write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xffff0000+owner); setreg(16, 0x14922); ref.cpu.cycles = 0; ref.cpu.refresh_cycles = 896
            count = len(isolated); hook(observer); execute(2000000); hook(None)
            assert len(isolated) == count+1 and not pending and not errors
        boundary_count = len(isolated)
        for mode in [0, -1, 1]:
            setup(mode&255)
            write(0xff84c9, mode); write(0xff8476, 0); write(0xff8475, 2)
            write(0xff84ca, 0xff6000, 4); write(0xffa032, 0); write(0xffa09a, 0)
            write_bytes(0xff6000, bytes([0x11, 0x12, 0x13, 0x14])*64)
            rows = []
            for step, current in enumerate([0, 8, 16, 0, 0x80, 0, 64, 0]):
                write(0xfff780, current); write(0xfff798, current); write(0xfff790, current)
                row = invoke('input', f'input-stream-{mode}/{step}')
                row['external_input'] = row['before']['input']
                rows.append(row)
            streams.append({'kind': 'input', 'initial': rows[0]['before'], 'calls': rows})
        for health, energy in [(0, 223), (-1024, 0), (223, 200)]:
            setup(health&255)
            for off, value in [(0x78, health), (0x7a, 40), (0xb8, energy), (0xbc, 190), (0x8a, 0)]:
                write(0xff8400+off, value, 2)
            write(0xff8470, 0); write(0xfff585, 1); write(0xfff518, 0xd400, 2)
            rows = [invoke('hud', f'hud-stream-{health}-{energy}/{step}') for step in range(16)]
            streams.append({'kind': 'hud', 'initial': rows[0]['before'], 'calls': rows})
        identity = ref.identity
    for name, records, scope in [('fighter-envelope-oracle', natural, 'Natural complete envelopes; AI input prefix stops explicitly before1cc8a'),
                                  ('fighter-envelope-boundaries', isolated[:boundary_count], 'Declared isolated complete functions or explicitAI prefix'),
                                  ('fighter-envelope-sequences', streams, 'Single-initial-state input/HUD streams')]:
        raw = (json.dumps(records, separators=(',', ':'))+'\n').encode(); zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        rows = [r for stream in records for r in stream['calls']] if name.endswith('sequences') else records
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'records': len(records), 'calls': len(rows),
                  'kind_counts': dict(Counter(r['kind'] for r in rows)), 'routes': dict(Counter(r['route'] for r in rows)),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
                  'incomplete_calls': 0, 'errors': errors, 'pass': bool(records) and not errors,
                  'rom_slot_zero_byte_writer_is_noop': rom_writes_ignored,
                  'limits': ['AI output itself is not injected or predicted; prefix explicitly returns at156e8', 'Streams are isolated function calls, not fullworld steps', 'HUD current display values are the observed nonnegative domain plus declared positive edges']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
