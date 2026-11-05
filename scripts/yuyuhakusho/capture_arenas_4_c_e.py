#!/usr/bin/env python3
"""Original-CPU arena 4/c/e evidence; no rule implementation or original control acquisition."""
import argparse
from collections import Counter
import ctypes as C
import gzip
import itertools
import json
import os
from pathlib import Path
import subprocess
import sys

sys.dont_write_bytecode = True
from reference import ROOT, ReferenceSession, buttons_at, sha

ENTRIES = {4: 0x20cb2, 12: 0x212f8, 14: 0x21412}
CORE = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
CORE_SHA = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
BRIDGE_SOURCE = r'''
#include <stdint.h>
#include <stddef.h>
typedef void(*callback_t)(int,int,unsigned,unsigned);
typedef struct {uint32_t pc,address,width,value,irq,kind;} event_t;
static callback_t cb; static unsigned pcs[128],last_pc; static size_t points,count;
static unsigned active,overflow,irq_depth; static event_t events[32768];
static const unsigned char *rom,*ram;static size_t rom_size;
void yyh_memory(const unsigned char *r,size_t n,const unsigned char *w){rom=r;rom_size=n;ram=w;}
int yyh_set_observer(callback_t f,const unsigned *p,size_t n){if(n>128)return 0;cb=f;points=n;for(size_t i=0;i<n;i++)pcs[i]=p[i];return 1;}
void yyh_begin(void){count=overflow=irq_depth=0;active=1;}
void yyh_end(void){active=0;}
size_t yyh_count(void){return count;}
unsigned yyh_overflow(void){return overflow;}
event_t *yyh_events(void){return events;}
static void put(unsigned p,unsigned a,unsigned w,unsigned v,unsigned k){if(count==32768){overflow=1;return;}events[count++]=(event_t){p,a,w,v,irq_depth,k};}
void yyh_cpu_hook(int type,int width,unsigned address,unsigned value){
 unsigned p=address&0xffffff;
 if(type==1){
  last_pc=address;
  if(active){
   if(p==0xfff000||p==0x2e5e){++irq_depth;put(address,0,0,0,2);}
   unsigned op=0;
   if(p+1<rom_size)op=((unsigned)rom[p]<<8)|rom[p+1];
   else if(p>=0xe00000)op=((unsigned)ram[(p&65535)^1]<<8)|ram[((p+1)&65535)^1];
   if(op==0x4e73){put(address,0,0,0,3);if(irq_depth)--irq_depth;}
   if((p>=0x21960&&p<=0x219e8)||p==0x37c56||p==0x37cac)put(address,0,0,0,4);
  }
  if(cb)for(size_t i=0;i<points;i++)if(p==pcs[i]){cb(type,width,address,value);break;}
 }else if(type==4&&active&&p>=0xe00000)put(last_pc,address,(unsigned)width,value,1);
}
'''


def inside(path):
    path = path.resolve()
    if not path.is_relative_to(ROOT):
        raise ValueError('all generated paths must remain inside this worktree')
    return path


def num(raw, off, n, signed=False):
    return int.from_bytes(raw[off:off+n], 'big', signed=signed)


def snapshot(ram):
    return {'arena': ram[0x8180:0x8280].hex(), 'camera': ram[0xa00e:0xa01c].hex(),
            'samples': ram[0xa800:0xa880].hex(), 'prefix': ram[0xe600:0xe800].hex(),
            'scroll_a': ram[0xe800:0xea00].hex(), 'scroll_b': ram[0xea00:0xec00].hex(),
            'pan_a': ram[0x6000:0x61c0].hex(), 'pan_b': ram[0x6200:0x63c0].hex(),
            'source_a': num(ram, 0xf598, 4, True), 'source_b': num(ram, 0xf59c, 4, True),
            'vertical': ram[0xe500:0xe540].hex(), 'scroll_mode': ram[0xf6e7],
            'video_flags': ram[0xf6df], 'frame_counter': ram[0xf589],
            'palette_registers': ram[0xf500:0xf508].hex(), 'palettes': ram[0xe000:0xe180].hex(),
            'pause': ram[0xf550], 'slow': ram[0xf55a],
            'shake': num(ram, 0xf566, 2, True), 'pan': num(ram, 0xf572, 2, True)}


def arena4_library(rom):
    pointers = [num(rom, 0x20e00+i*4, 4) for i in range(4)]
    assert pointers == [0x20e10, 0x20e16, 0x20e1c, 0x20e22]
    rows = {str(i): [num(rom, 0x2a000+i*256+0x30+j*2, 2, True) for j in range(112)] for i in range(64)}
    colors = [[num(rom, p+i*2, 2) for i in range(3)] for p in pointers]
    return {'schema': 1, 'arena4_rows': rows, 'arena4_colors': colors,
            'source': {'rom_sha256': sha(rom), 'rows_address': '2a000+phase*256+30',
                       'rows': 64, 'words_per_row': 112, 'color_pointer_table': '20e00',
                       'color_pointers': [hex(p) for p in pointers]}}


def owned_bytes(arena, kind):
    out = set()
    def add(start, n): out.update(range(start, start+n))
    if kind == 'palette':
        add(0x81a0, 4); add(0xe092, 6)
        return out
    add(0xa00e, 14); add(0xe500, 2); add(0xe528, 2); add(0xf598, 8); add(0xf6e7, 1)
    add(0x6000, 448); add(0x6200, 448); add(0x8186, 2)
    if arena == 4:
        add(0x8188, 2); add(0x81a0, 4); add(0xe092, 6)
        add(0xe8a0, 320); add(0xea10, 320); add(0xeb50, 160)
    elif arena == 12:
        add(0x8188, 2); add(0x8190, 8); add(0x819c, 4); add(0xf6df, 1)
        add(0xe8a0, 320); add(0xea20, 448)
    elif arena == 14:
        add(0x8188, 4); add(0x818c, 2); add(0x8190, 4); add(0x8194, 2); add(0x8198, 2); add(0xf6df, 1)
        add(0xe8a0, 336); add(0xea20, 448)
    return out


def audit(arena, kind, before, after, trace, stack):
    """Every RAM write is replayed, including stack and explicit IRQ effects."""
    image = bytearray(before); allowed = owned_bytes(arena, kind); external = []; stack_writes = 0
    for pc, address, width, value, irq, event in trace:
        if event != 1: continue
        if width not in [1, 2, 4]: raise ValueError(('unknown RAM write width', width))
        address &= 0xffff
        offsets = [(address+i)&0xffff for i in range(width)]
        raw = (value & ((1 << (8*width))-1)).to_bytes(width, 'big')
        previous = bytes(image[o] for o in offsets)
        local_pc = pc & 0xffffff
        component_pc = ((0x20cb2 <= local_pc <= 0x20dfe and arena == 4) or
                        (0x212f8 <= local_pc <= 0x21410 and arena == 12) or
                        (0x21412 <= local_pc <= 0x21554 and arena == 14) or
                        0x21556 <= local_pc <= 0x215f8 or 0x21690 <= local_pc <= 0x216ee)
        is_stack = all(((stack & 0xffff)-160 <= o < (stack & 0xffff)+4) for o in offsets)
        if irq:
            external.append({'pc': pc, 'address': address, 'before': previous.hex(), 'after': raw.hex()})
        elif is_stack:
            stack_writes += 1
        elif component_pc and all(o in allowed for o in offsets):
            pass
        else:
            raise ValueError(('unexplained RAM write', arena, kind, hex(pc), hex(address), width, hex(value)))
        for offset, byte in zip(offsets, raw): image[offset] = byte
    if sum(e[5] == 2 for e in trace) != sum(e[5] == 3 for e in trace):
        raise ValueError(('unbalanced observed IRQ entry/RTE', [(hex(e[0]), e[5]) for e in trace if e[5] in [2, 3]]))
    if bytes(image) != after:
        unknown = [(hex(i), a, b) for i, (a, b) in enumerate(zip(image, after)) if a != b]
        raise ValueError(('complete 64KiB write reconstruction mismatch', unknown[:24]))
    return {'ram_before': before.hex(), 'ram_after_sha256': sha(after), 'trace': trace,
            'ram_bytes': 65536, 'unknown_writes': 0, 'stack_writes': stack_writes,
            'external_writes': external}


class Event(C.Structure):
    _fields_ = [(k, C.c_uint32) for k in ['pc', 'address', 'width', 'value', 'irq', 'kind']]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--arena', type=lambda s: int(s, 16), choices=ENTRIES, required=True)
    p.add_argument('--inputs', type=Path, required=True)
    p.add_argument('--control', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--library-only', action='store_true')
    args = p.parse_args(); out = inside(args.out); out.mkdir(parents=True, exist_ok=True)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    lib = arena4_library(rom)
    (out/'arena-scene-4-tables.json').write_text(json.dumps(lib, separators=(',', ':'))+'\n')
    if args.library_only: return
    events = json.loads(args.inputs.read_text()); expected = json.loads(args.control.read_text())
    if isinstance(events, dict): events = events['events']
    if isinstance(expected, dict): expected = expected['frames']
    if len(expected) != 3602: raise ValueError('use the unique unified original 3602-frame control')
    source = out/'observer.c'; source.write_text(BRIDGE_SOURCE)
    for sub in ['tmp', 'cache']: (out/sub).mkdir(exist_ok=True)
    bridge_path = out/'observer.dylib'
    subprocess.run(['clang', '-dynamiclib', '-O2', str(source), '-o', str(bridge_path)], check=True,
                   capture_output=True, env=dict(os.environ, TMPDIR=str(out/'tmp'), CLANG_MODULE_CACHE_PATH=str(out/'cache')))
    bridge = C.CDLL(str(bridge_path)); cbtype = C.CFUNCTYPE(None, C.c_int, C.c_int, C.c_uint, C.c_uint)
    bridge.yyh_set_observer.argtypes = [cbtype, C.POINTER(C.c_uint), C.c_size_t]
    bridge.yyh_set_observer.restype = C.c_int
    bridge.yyh_memory.argtypes = [C.c_void_p, C.c_size_t, C.c_void_p]
    bridge.yyh_count.restype = C.c_size_t; bridge.yyh_events.restype = C.POINTER(Event)
    points = {ENTRIES[args.arena], 0x20dca, 0x3d8}; pending = {}; errors = []
    private = False; label = ''; active_kind = 'arena'; rows = []; streams = []
    with ReferenceSession(ROOT, out/'reference', core=CORE) as ref:
        if ref.identity['core_sha256'] != CORE_SHA: raise ValueError('hook core identity differs')
        bridge.yyh_memory(C.cast(ref.rom_buffer, C.c_void_p), len(rom), ref.ram_pointer)
        def configure():
            pcs = (C.c_uint*len(points))(*sorted(points)); assert bridge.yyh_set_observer(observe, pcs, len(points))
        def trace_slice(start):
            if bridge.yyh_overflow(): raise ValueError('write trace overflow')
            trace = bridge.yyh_events()
            return [[getattr(trace[i], k) for k, _ in Event._fields_] for i in range(start, bridge.yyh_count())]
        @cbtype
        def observe(hook_kind, width, pc, value):
            try:
                local_pc = pc & 0xffffff; sp = ref.get_register(15)
                for key, row in list(pending.items()):
                    if local_pc == row['return_pc'] and sp == row['stack']+4:
                        after = ref.ram_cpu(); before = row.pop('_before'); start = row.pop('_trace_start')
                        row.update(after=snapshot(after), exit_frame=ref.frame,
                                   audit=audit(args.arena, row['kind'], before, after, trace_slice(start), row['stack']))
                        rows.append(row); pending.pop(key)
                        if not pending: bridge.yyh_end()
                        if private and row['kind'] == active_kind: ref.cpu.cycles = 4000000
                kind = 'arena' if local_pc == ENTRIES[args.arena] else 'palette' if local_pc == 0x20dca else None
                if kind is None or (private and kind != active_kind): return
                if (ref.get_register(13)&0xffff) != 0x8180: return
                if not pending: bridge.yyh_begin()
                ram = ref.ram_cpu(); ret = num(ram, sp&0xffff, 4)&0xffffff; key = (kind, sp)
                if key in pending: raise ValueError('nested same-key callback')
                pending[key] = {'kind': kind, 'arena_id': args.arena, 'frame': ref.frame, 'name': label,
                                'stack': sp, 'return_pc': ret, 'before': snapshot(ram),
                                '_before': ram, '_trace_start': bridge.yyh_count()}
                if ret not in points: points.add(ret); configure()
            except Exception as error:
                errors.append(repr(error)); bridge.yyh_end()
                if private: ref.cpu.cycles = 4000000
        configure(); hook = ref.api('set_cpu_hook', None, C.c_void_p)
        observer = C.cast(bridge.yyh_cpu_hook, C.c_void_p); hook(observer); observed = []
        for frame in range(1, 3603):
            got = ref.step(frame, buttons_at(events, frame))['reference']; observed.append(got)
            if got != expected[frame-1]: raise ValueError(('observer differs from original control', args.arena, frame))
            if frame == 2400: checkpoint = ref.save()
        hook(None)
        if pending or errors: raise ValueError((list(pending), errors[:12]))
        natural_count = len(rows)
        if not any(row['kind'] == 'arena' for row in rows): raise ValueError('legal inputs did not reach arena callback')
        if any(row['before']['arena'][:8] != ENTRIES[args.arena].to_bytes(4, 'big').hex() for row in rows):
            raise ValueError('unexpected arena identity')
        private = True; setreg = ref.api('m68k_set_reg', None, C.c_uint, C.c_uint); execute = ref.api('m68k_run', None, C.c_uint)
        def data(address, raw):
            for i, byte in enumerate(raw): C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value = byte
        def write(address, value, size=1): data(address, (value&((1 << (size*8))-1)).to_bytes(size, 'big'))
        def setup(seed, stage=2):
            ref.restore(checkpoint, restore_refresh_cycles=True)
            for reg in range(15): setreg(reg, 0)
            for base, size in [(0x8180, 256), (0xe000, 384), (0xe500, 64), (0xe600, 1536), (0x6000, 1024)]:
                data(base, bytes((i*73+seed*29+17)&255 for i in range(size)))
            for address, value, size in [(0x8180, ENTRIES[args.arena], 4), (0x8184, 128, 2), (0x8186, stage, 2),
                                         (0xf550, 0, 1), (0xf55a, 0, 1), (0xf566, 0, 2), (0xf572, 0, 2),
                                         (0xa00e, 0, 2), (0xa014, 864, 2), (0xa016, 16, 2),
                                         (0xa010, 880, 2), (0xa012, 1168, 2), (0xa018, 864, 2), (0xa01a, 16, 2),
                                         (0xf501, 0, 1), (0xf589, seed&255, 1), (0xf6df, 0xa5, 1), (0xf6e7, 0x5a, 1)]: write(address, value, size)
            for i in range(16):
                write(0xa800+i*4, ((900+i*3)<<16)|0x1234, 4); write(0xa840+i*4, ((320-i*2)<<16)|0xabcd, 4)
        def invoke(kind, name, external=None):
            nonlocal active_kind, label
            active_kind, label = kind, name; setreg(17, 0x2700); setreg(15, 0xffff00); write(0xff00, 0x3d8, 4)
            setreg(13, 0xffff8180); setreg(16, ENTRIES[args.arena] if kind == 'arena' else 0x20dca)
            ref.cpu.cycles, ref.cpu.refresh_cycles = 0, 896; count = len(rows)
            hook(observer); execute(2000000); hook(None)
            if len(rows) != count+1 or pending or errors: raise ValueError((kind, name, len(rows)-count, list(pending), errors[:8]))
            if external is not None: rows[-1]['external_input'] = external
            return rows[-1]
        # Dirty state, gating, feedback extrema, every row selection and word arithmetic.
        cases = itertools.product([0, 2, 1, 65535], [0, 1, 127, 128, 255], [0, 1], [0, 1, -32768], [744, 864, 984], [0, 16, 32])
        for seed, (stage, pause, slow, pan, x, y) in enumerate(cases):
            setup(seed, stage)
            for address, value, size in [(0xf550, pause, 1), (0xf55a, slow, 1), (0xf572, pan, 2), (0xa014, x, 2), (0xa016, y, 2)]: write(address, value, size)
            invoke('arena', f'gate-{stage}-{pause}-{slow}-{pan}-{x}-{y}')
        for seed, x in enumerate(range(744, 985)):
            setup(seed); write(0xa014, x, 2); write(0xa00e, 64 if seed%2 else 0, 2)
            invoke('arena', f'camera-table-{x}')
        for seed, (y, shake) in enumerate(itertools.product([-32768, -32767, -1, 32766, 32767], [-32768, -1, 1, 32767])):
            setup(seed); write(0xa016, y, 2); write(0xf566, shake, 2)
            invoke('arena', f'word-vertical-{y}-{shake}')
        for seed, (dy, pan) in enumerate(itertools.product([-256, -255, -1, 0, 31, 32], [1, -32768, 32767])):
            setup(seed); write(0xf566, 16-dy, 2); write(0xf572, pan, 2)
            invoke('arena', f'feedback-{dy}-{pan}')
        if args.arena in [12, 14]:
            for seed, (value, phase, frame, pan) in enumerate(itertools.product([0x00007fff, 0x00008000, 0x7fffffff, 0x80000000, 0xffff8000, 0xffffffff], [0, 32767, 32768, 65535], [0, 1, 128, 255], [0, 1])):
                setup(seed); write(0xf589, frame); write(0xf572, pan, 2)
                if args.arena == 12:
                    write(0x8190, value, 4); write(0x8194, value, 4); write(0x819c, phase, 2)
                else: write(0x8188, value, 4)
                invoke('arena', f'long-carry-{value}-{phase}-{frame}-{pan}')
        if args.arena == 4:
            for seed, (status, pan, phase, timer) in enumerate(itertools.product([0, 1, 2, 128, 255], [0, 1, -32768], [0, 4, 8, 12, 65535, 32767], [0, 1, 5, 32767, 32768, 65535])):
                setup(seed); write(0xf501, status); write(0xf572, pan, 2); write(0x81a0, phase, 2); write(0x81a2, timer, 2)
                invoke('palette', f'palette-{status}-{pan}-{phase}-{timer}')
        boundary_count = len(rows)-natural_count
        # One original initial state per stream; subsequent callbacks carry state.
        for stream_seed, kind in enumerate(['arena']+(['palette'] if args.arena == 4 else [])):
            setup(stream_seed, 0); calls = []; initial = snapshot(ref.ram_cpu())
            for step in range(180):
                external = {'pan': [0, 1, -32768, 0][step//45], 'pause': 128 if 50 <= step < 54 else 0,
                            'slow': 1 if 100 <= step < 104 else 0, 'frame_counter': step&255, 'status': 1 if 80 <= step < 84 else 0,
                            'samples_x': [((900+step%37)<<16)|0x1234]*4, 'samples_y': [(320<<16)|0x5678]*4, 'used': 16}
                for address, value, size in [(0xf572, external['pan'], 2), (0xf550, external['pause'], 1), (0xf55a, external['slow'], 1),
                                             (0xf589, external['frame_counter'], 1), (0xf501, external['status'], 1), (0xa00e, external['used'], 2)]: write(address, value, size)
                for i, value in enumerate(external['samples_x']): write(0xa800+i*4, value, 4)
                for i, value in enumerate(external['samples_y']): write(0xa840+i*4, value, 4)
                calls.append(invoke(kind, f'stream-{kind}/{step}', external))
            streams.append({'kind': kind, 'initial': initial, 'calls': calls})
        identity = ref.identity
    natural, boundaries = rows[:natural_count], rows[natural_count:natural_count+boundary_count]
    for suffix, calls in [('natural', natural), ('boundaries', boundaries)]:
        raw = b''.join((json.dumps(row, separators=(',', ':'))+'\n').encode() for row in calls)
        zipped = gzip.compress(raw, mtime=0); name = f'arenas-4-c-e-{args.arena:x}-{suffix}'
        (out/(name+'.jsonl.gz')).write_bytes(zipped)
        report = {'pass': True, 'scope': 'complete original arena/palette returns with exact 65536-byte RAM-write reconstruction',
                  'identity': identity, 'arena_id': args.arena, 'calls': len(calls), 'kind_counts': dict(Counter(c['kind'] for c in calls)),
                  'frames': 3602, 'frames_match_original_control': observed == expected, 'incomplete_calls': 0, 'unknown_writes': 0,
                  'input_sha256': sha(args.inputs.read_bytes()), 'control_sha256': sha(args.control.read_bytes()),
                  'capture_sha256': sha(Path(__file__).read_bytes()), 'bridge_source_sha256': sha(BRIDGE_SOURCE.encode()),
                  'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped),
                  'irq_calls': sum(any(e[5] == 2 for e in c['audit']['trace']) for c in calls),
                  'rte_counts': dict(Counter(hex(e[0]) for c in calls for e in c['audit']['trace'] if e[5] == 3)),
                  'sound_clear_counts': dict(Counter(hex(e[0]) for c in calls for e in c['audit']['trace'] if e[5] == 1 and e[0]&0xffffff in [0x37c56, 0x37cac])),
                  'errors': []}
        (out/(name+'-manifest.json')).write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))
    raw = (json.dumps({'streams': streams}, separators=(',', ':'))+'\n').encode()
    zipped = gzip.compress(raw, mtime=0); name = f'arenas-4-c-e-{args.arena:x}-sequences'
    (out/(name+'.json.gz')).write_bytes(zipped)
    (out/(name+'-manifest.json')).write_text(json.dumps({'pass': True, 'streams': len(streams), 'calls': sum(len(s['calls']) for s in streams),
        'identity': identity, 'capture_sha256': sha(Path(__file__).read_bytes()), 'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped),
        'single_initial_state': True, 'unknown_writes': 0, 'incomplete_calls': 0}, indent=2)+'\n')


if __name__ == '__main__':
    main()
