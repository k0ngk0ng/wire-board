#!/usr/bin/env python3
"""Original complete15540 returns, including real Chu AI and recording resumption."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from capture_chu_ai import snapshot as ai_snapshot, FIGHTERS


def number(raw, off, size):
    return int.from_bytes(raw[off:off+size], 'big')


def snapshot(ram, address, rom, stream_base=None):
    off = address&0xffff
    obj = ram[off:off+256]
    pointer = number(obj, 0xca, 4)
    if stream_base is None:
        stream_base = pointer&~1023 if pointer else 0xff6000
    if stream_base < len(rom):
        stream = rom[stream_base:stream_base+1024]
    else:
        assert 0xff0000 <= stream_base <= 0xfffc00, hex(stream_base)
        stream = ram[stream_base&0xffff:(stream_base&0xffff)+1024]
    channel = obj[0x76]
    frame = {key: (ram[0xf780+channel+i*8] | ram[0xf7a8+channel+i*8]<<8) if channel < 8 else 0
             for i, key in enumerate(['current', 'previous', 'changed', 'pressed', 'released'])}
    return {'ai': ai_snapshot(ram, address), 'envelope': {'actor': obj.hex(), 'stream_base': stream_base,
            'stream': stream.hex(), 'input': frame, 'disabled': ram[0xa032], 'stopped': ram[0xa09a], 'markers': list(ram[0xfffc:])}}


def route(row):
    envelope = row['before']['envelope']
    obj = bytes.fromhex(envelope['actor'])
    if envelope['disabled']&1:
        return 'disabled'
    if 0 < obj[0xc9] < 128:
        return 'playback-stopped' if envelope['stopped'] or number(obj, 0xca, 4) == 0 else 'playback'
    if obj[0x76] == 255:
        return 'ai-record' if obj[0xc9] >= 128 else 'ai'
    return 'human-record' if obj[0xc9] >= 128 else 'human'


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
    points = [0x15540, 0x1269a, 0x3d8, 0x1cc8a, 0x15594, 0x3982]
    pcs = (C.c_uint*len(points))(*points)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    natural, cases, streams, errors = [], [], [], []
    pending = None
    private, label = False, ''
    with ReferenceSession(ROOT, out, core=core) as ref:
        unused_writer = C.cast(ref.lib.m68k_unused_8_w, C.c_void_p).value
        assert ref.cpu.memory_map[3] == unused_writer
        @cbtype
        def observe(kind, width, pc, value):
            nonlocal pending
            try:
                if pc == 0x15540:
                    assert pending is None, 'Nested full input entry'
                    address = ref.get_register(13)&0xffffff
                    ram = ref.ram_cpu()
                    assert ram[(address&0xffff)+0x58] == 9
                    pending = {'object_id': address&0xffff, 'source_register': hex(ref.get_register(13)), 'frame': ref.frame,
                               'entry_sp': ref.get_register(15), 'before': snapshot(ram, address, rom), 'ai_calls': 0, 'record_calls': 0, 'rng_calls': 0}
                elif pending is not None and pc in (0x1cc8a, 0x15594, 0x3982):
                    pending[{0x1cc8a: 'ai_calls', 0x15594: 'record_calls', 0x3982: 'rng_calls'}[pc]] += 1
                elif pending is not None and pc == (0x3d8 if private else 0x1269a):
                    assert ref.get_register(15) == pending['entry_sp']+4, 'Not a complete15540 return'
                    pending['after'] = snapshot(ref.ram_cpu(), pending['object_id'], rom, pending['before']['envelope']['stream_base'])
                    pending.update(exit=hex(pc), route=route(pending), exit_frame=ref.frame)
                    assert pending['ai_calls'] in (0, 1) and pending['record_calls'] in (0, 1)
                    if not pending['ai_calls']:
                        assert pending['rng_calls'] == 0
                    if private:
                        pending['name'] = label
                        cases.append(pending)
                        ref.cpu.cycles = 4000000
                    else:
                        natural.append(pending)
                    pending = None
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
        assert frames == expected and not pending and not errors, (len(natural), errors)
        private = True
        setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint)
        execute = ref.api('m68k_run', None, C.c_uint)

        def write(address, value, size=1):
            for i, b in enumerate((value&((1 << (size*8))-1)).to_bytes(size, 'big')):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = b

        def write_bytes(address, raw):
            for i, b in enumerate(raw):
                C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value = b

        profiles = [(0, 0, 0, 0, 0, 0), (0x80, 0x80, 0x80, 0, 0, 0),
                    (0x10, 0x10, 0x10, 1, 2, 4), (0x8f, 0x70, 0x50, 255, 128, 64)]

        def setup(serial, source, mode=0, channel=255, retain=0, stopped=0, disabled=0, profile=0):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15):
                setreg(reg, 0)
            write_bytes(0xff8400, bytes((i*73+serial*29+17)&255 for i in range(4096)))
            for i, owner in enumerate(FIGHTERS):
                for off, value, size in [(4, 256, 2), (0x58, 9, 1), (0x6e, 0 if owner == source else 1, 1),
                                         (0x78, 223, 2), (0x40, 100 if owner == source else 160+i*24, 2), (0x44, 320, 2),
                                         (0x9c, 320, 2), (0x5c, 0, 1), (0x64, 0, 1)]:
                    write(0xff0000+owner+off, value, size)
                write(0xff0000+owner+0x144, 0, 2)
            for off, value, size in [(0xc9, mode, 1), (0x76, channel, 1), (0x75, retain, 1), (0xca, 0xff6000+([0, 4, 1020][serial%3]), 4),
                                     (0x61, [0, 1, 3][serial%3], 1), (0x5f, [0, 0xa6][serial%2], 1), (0xb8, [32, 100, 223][serial%3], 2),
                                     (0xce, [2, 6, 12, 16][serial%4], 1), (0xd0, (FIGHTERS.index(source)*2)&7, 2),
                                     (0xd2, [0, 10][serial%2], 2), (0xd4, [0xffff, 0, 8][serial%3], 2), (0xd6, FIGHTERS[(FIGHTERS.index(source)+1)%4], 2)]:
                write(0xff0000+source+off, value, size)
            write_bytes(0xff6000, bytes((i*31+serial*13+9)&255 for i in range(1024)))
            write_bytes(0xfffffc, bytes([1, 0x22, 0x44, 0x7f]))
            write(0xffa032, disabled)
            write(0xffa09a, stopped)
            write(0xffa010, 32, 2)
            write(0xffa012, 600, 2)
            write(0xffff12, [0, 2, 4, 6][serial%4], 2)
            write(0xffff18, [0, 1, 0x12345678, 0xffffffff][serial%4], 4)
            if channel < 8:
                held, pressed, changed, extheld, extpress, extchange = profiles[profile]
                for off, value in [(0xf780, held), (0xf798, pressed), (0xf790, changed), (0xf7a8, extheld), (0xf7c0, extpress), (0xf7b8, extchange)]:
                    write(0xff0000+off+channel, value)

        def invoke(source, name):
            nonlocal label
            label = name
            setreg(17, 0x2700)
            setreg(15, 0xffffe0)
            write(0xffffe0, 0x3d8, 4)
            setreg(13, 0xffff0000+source)
            setreg(16, 0x15540)
            ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896
            before = ref.ram_cpu()
            count = len(cases)
            hook(observer)
            execute(2000000)
            hook(None)
            assert len(cases) == count+1 and not pending and not errors, (name, errors)
            after = ref.ram_cpu()
            allowed = set(range(source, source+256)) | set(range(0x6000, 0x6400)) | set(range(0xff18, 0xff1c)) | set(range(0xff80, 0xffe4)) | set(range(0xfffc, 0x10000)) | {0xa09a}
            unexpected = [i for i, (a, b) in enumerate(zip(before, after)) if a != b and i not in allowed]
            assert not unexpected, (name, unexpected)
            return cases[-1]

        for serial, (source, mode, channel, retain, stopped, disabled, profile) in enumerate(itertools.product(FIGHTERS, [-128, -1, 0, 1, 127], list(range(8))+[255], [0, 1], [0, 1], [0, 1], range(4))):
            setup(serial, source, mode, channel, retain, stopped, disabled, profile)
            if mode > 0 and serial%7 == 0:
                write(0xff0000+source+0xca, 0, 4)
            invoke(source, f'input-{source:x}/{mode}/{channel}/{retain}/{stopped}/{disabled}/{profile}')
        boundary_count = len(cases)
        for index, (source, mode, channel) in enumerate(itertools.product(FIGHTERS, [0, -1, 1], [0, 255])):
            setup(index, source, mode, channel, retain=2)
            write(0xff0000+source+0xca, 0xff6000, 4)
            write_bytes(0xff6000, bytes([0x11, 0x12, 0x13, 0x14])*256)
            calls = []
            for step in range(12):
                current = [0, 8, 16, 0, 0x80, 0, 64, 0, 0, 16, 0, 8][step]
                if channel < 8:
                    for off in [0xf780, 0xf798, 0xf790]:
                        write(0xff0000+off+channel, current)
                enemy = FIGHTERS[(FIGHTERS.index(source)+1)%4]
                write(0xff0000+enemy+0x40, 100+(step%5)*24, 2)
                call = invoke(source, f'stream-{source:x}/{mode}/{channel}/{step}')
                call['external_input'] = call['before']['envelope']['input']
                call['external_enemy'] = {'object_id': enemy, 'x': 100+(step%5)*24}
                calls.append(call)
            streams.append({'initial': calls[0]['before'], 'calls': calls})
        identity = ref.identity
    for name, records, scope in [('chu-input-oracle', natural, 'Complete natural15540 including real AI returns'),
                                  ('chu-input-boundaries', cases[:boundary_count], 'Declared isolated complete human/AI/playback/recording calls'),
                                  ('chu-input-sequences', streams, 'Single-initial-state complete input streams with declared external controls/world inputs')]:
        raw = (json.dumps(records, separators=(',', ':'))+'\n').encode()
        zipped = gzip.compress(raw, mtime=0)
        (out/(name+'.json.gz')).write_bytes(zipped)
        rows = [call for stream in records for call in stream['calls']] if name.endswith('-sequences') else records
        report = {'scope': scope, 'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'records': len(records), 'calls': len(rows),
                  'routes': dict(Counter(r['route'] for r in rows)), 'ai_calls': sum(r['ai_calls'] for r in rows), 'record_continuations': sum(r['record_calls'] for r in rows),
                  'rng_call_counts': dict(Counter(str(r['rng_calls']) for r in rows)), 'canonical_source_registers': all(r['source_register'] == hex(0xffff0000+r['object_id']) for r in rows),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
                  'rom_slot_zero_byte_writer_is_noop': True, 'errors': errors, 'incomplete_calls': 0, 'pass': bool(records) and not errors,
                  'limits': ['Complete input function only; not full fighter/world/matches', 'AI is the recovered source9 strategy; other character strategies remain separate']}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
