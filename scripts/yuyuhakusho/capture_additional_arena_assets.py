#!/usr/bin/env python3
"""Passive, full-control-matched raster references for one additional arena.

The caller supplies the unique unobserved original-core baseline and its legal
menu inputs. This producer never generates another original-core baseline.
"""
import argparse
import ctypes as C
import gzip
import json
import os
from pathlib import Path
import subprocess

from extract_additional_arena_assets import ARENAS, resource_bindings
from extract_arena_assets import Line, swapped
from reference import ROOT, ROM_SHA256, ReferenceSession, buttons_at, sha

ORIGINAL_CORE_SHA = '1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
HOOK_CORE_SHA = 'e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
SELECTED = [2700, 2720, 2740, 2760, 2780, 2800, 2820, 2850, 2880, 2910,
            2940, 2970, 3000, 3050, 3100, 3150, 3250, 3350, 3450, 3550]

OBSERVER = r'''
#include <stdint.h>
#include <string.h>
typedef struct {uint8_t seen;int32_t first_cycle;uint8_t regs[32],vsram[4],hscroll[4];uint16_t cram[64];} line_t;
static line_t rows[224];static uint8_t first_vram[65536],row_vram[224][65536],has_vram[224];
static const uint8_t *vram,*vsram,*regs,*ram;static const uint16_t *cram,*vc;static const int32_t *cycles;static const uint32_t *base;
typedef void(*render_t)(int);static render_t original_bg;static render_t*background;
static int enabled,captured,changed,pending_palette=-1;static uint32_t rebindings;
typedef uint32_t(*reg_t)(unsigned);static reg_t reader;static uint32_t frame,load_count,load_dropped,load_frames[16],load_lists[16];
static uint8_t loaded_palette[32];static uint32_t palette_frame;static int arena_loading;
static void record(unsigned y){
 line_t*l=&rows[y];l->first_cycle=*cycles-*base;memcpy(l->regs,regs,32);memcpy(l->vsram,vsram,4);memcpy(l->cram,cram,128);
 unsigned masks[4]={0,7,248,255};unsigned h=((regs[13]&63)<<10)+((y&masks[regs[11]&3])<<2);memcpy(l->hscroll,vram+(h&65535),4);
}
void observe_bg(int line){
 if(enabled&&line>=0&&line<224){
  if(!captured){memcpy(first_vram,vram,65536);captured=1;}
  else if(memcmp(first_vram,vram,0x4000)||memcmp(first_vram+0x8000,vram+0x8000,0x7000))changed=1;
  record(line);rows[line].seen=1;has_vram[line]=1;memcpy(row_vram[line],vram,65536);
 }
 original_bg(line);
}
void configure(const void*v,const void*vs,const void*r,const void*c,const void*counter,const void*cpu_cycles,const void*vdp_cycles,void*bg_pointer,void*registers_reader,const void*game_ram){vram=v;vsram=vs;regs=r;cram=c;vc=counter;cycles=cpu_cycles;base=vdp_cycles;background=bg_pointer;original_bg=*background;*background=observe_bg;reader=(reg_t)registers_reader;ram=game_ram;}
void set_frame(uint32_t f){frame=f;}uint32_t get_load_count(void){return load_count;}uint32_t get_load_dropped(void){return load_dropped;}uint32_t get_load_frame(uint32_t i){return load_frames[i];}uint32_t get_load_list(uint32_t i){return load_lists[i];}
void finish(void){if(*background==observe_bg)*background=original_bg;}
void begin_frame(int active){enabled=active;captured=changed=0;pending_palette=-1;memset(rows,0,sizeof(rows));memset(first_vram,0,sizeof(first_vram));memset(has_vram,0,sizeof(has_vram));if(active)for(unsigned y=0;y<224;y++)record(y);}
const line_t* get_lines(void){return rows;}const uint8_t* get_first_vram(void){return first_vram;}
const uint8_t* get_row_vram(uint32_t y){return y<224&&has_vram[y]?row_vram[y]:0;}
const uint8_t* get_loaded_palette(void){return loaded_palette;}uint32_t get_palette_frame(void){return palette_frame;}
int get_captured(void){return captured;}int get_changed(void){return changed;}uint32_t get_rebindings(void){return rebindings;}
void observe(int type,int width,unsigned address,unsigned value){
 (void)width;(void)value;
 if(type==4){unsigned a=address&0xffffff;if(enabled&&*vc<224&&a>=0xc00000&&a<0xc00004&&*cycles-*base<=860)pending_palette=*vc;return;}
 if(type!=1)return;
 if(address==0x800){uint32_t a=reader(8)&0xffffff;if(a>=0x21720&&a<0x218d8){arena_loading=1;if(load_count<16){load_frames[load_count]=frame;load_lists[load_count++]=a;}else load_dropped++;}}
 if(address==0x83c&&arena_loading&&!ram[0xf54e^1]){memcpy(loaded_palette,ram+0xe080,32);palette_frame=frame;arena_loading=0;}
 if(pending_palette>=0){memcpy(rows[pending_palette].cram,cram,128);pending_palette=-1;}
 if(*background!=observe_bg){original_bg=*background;*background=observe_bg;rebindings++;}
 if(!enabled||*vc>=224)return;
 if(*cycles-*base<=860&&!(regs[1]&64)){record(*vc);rows[*vc].seen=0;}
}
'''


def require(value, message):
    if not value:
        raise ValueError(message)


def changed_ranges(before, after, start, end):
    result, opened = [], None
    for position in range(start, end + 1):
        different = position < end and before[position] != after[position]
        if different and opened is None:
            opened = position
        elif not different and opened is not None:
            result.append({'address': opened, 'bytes': after[opened:position].hex()})
            opened = None
    return result


def compile_observer(out):
    temporary, cache = out / 'tmp', out / 'cache'
    temporary.mkdir(exist_ok=True)
    cache.mkdir(exist_ok=True)
    source, binary = out / 'observer.c', out / 'observer.dylib'
    source.write_text(OBSERVER)
    subprocess.run(['clang', '-dynamiclib', '-O2', str(source), '-o', str(binary)], check=True,
                   capture_output=True, env=dict(os.environ, TMPDIR=str(temporary), CLANG_MODULE_CACHE_PATH=str(cache)))
    library = C.CDLL(str(binary))
    library.configure.argtypes = [C.c_void_p] * 10
    library.begin_frame.argtypes = [C.c_int]
    library.get_lines.restype = C.POINTER(Line)
    library.get_first_vram.restype = C.POINTER(C.c_uint8)
    library.get_loaded_palette.restype = C.POINTER(C.c_uint8)
    library.get_row_vram.restype = C.POINTER(C.c_uint8)
    library.get_row_vram.argtypes = [C.c_uint32]
    library.set_frame.argtypes = [C.c_uint32]
    for name in ['get_load_count', 'get_load_dropped', 'get_load_frame', 'get_load_list', 'get_rebindings', 'get_palette_frame']:
        getattr(library, name).restype = C.c_uint32
    library.get_load_frame.argtypes = [C.c_uint32]
    library.get_load_list.argtypes = [C.c_uint32]
    return library, binary


def collect_active(bridge, sample, ref, frame):
    sample['video_next'], sample['video_next_frame'] = ref.image.hex(), frame
    first = swapped(bytes(bridge.get_first_vram()[:65536])) if bridge.get_captured() else bytes.fromhex(sample['vram'])
    sample['active_vram'] = first.hex()
    sample['active_background_vram_changed'] = bool(bridge.get_changed())
    rows, changes, previous, other_changes = bridge.get_lines(), [], first, []
    sample['active_lines'] = []
    for y in range(224):
        row = rows[y]
        sample['active_lines'].append({'line': y, 'draw': bool(row.seen), 'first_cycle': row.first_cycle,
                                       'registers': list(row.regs), 'cram': list(row.cram),
                                       'vsram': swapped(bytes(row.vsram)).hex(), 'hscroll': swapped(bytes(row.hscroll)).hex()})
        pointer = bridge.get_row_vram(y)
        if pointer and row.seen:
            active = swapped(bytes(pointer[:65536]))
            if active != previous:
                relevant = changed_ranges(previous, active, 0, 0x4000) + changed_ranges(previous, active, 0x8000, 0xf000)
                if relevant:
                    changes.append({'line': y, 'ranges': relevant})
                unrelated = changed_ranges(previous, active, 0x4000, 0x8000) + changed_ranges(previous, active, 0xf000, 0x10000)
                if unrelated:
                    other_changes.append({'line': y, 'ranges': unrelated})
            previous = active
    sample['active_vram_changes'] = changes
    sample['active_other_vram_changes'] = other_changes
    require(bool(changes) == sample['active_background_vram_changed'], 'observer change flag disagrees with exact row changes')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--arena', required=True, type=lambda value: int(value, 16))
    parser.add_argument('--inputs', required=True, type=Path)
    parser.add_argument('--frames', required=True, type=Path)
    parser.add_argument('--control-manifest', required=True, type=Path)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--selected', default=','.join(map(str, SELECTED)))
    args = parser.parse_args()
    require(args.arena in ARENAS, 'unsupported additional arena selection')
    out = args.out.resolve()
    require(out.is_relative_to(ROOT), 'output must remain inside this worktree')
    out.mkdir(parents=True, exist_ok=True)
    inputs_raw, frames_raw = args.inputs.read_bytes(), args.frames.read_bytes()
    inputs, expected, control = json.loads(inputs_raw), json.loads(frames_raw), json.loads(args.control_manifest.read_bytes())
    require(control['identity']['rom_sha256'] == ROM_SHA256 and control['identity']['core_sha256'] == ORIGINAL_CORE_SHA
            and control.get('observer_attached') is False, 'control must be the pinned unobserved original core')
    require(control['input']['sha256'] == sha(inputs_raw) and control['output']['sha256'] == sha(frames_raw)
            and control['frames'] == len(expected), 'control inputs/output identities differ')
    selected = sorted({int(value) for value in args.selected.split(',')})
    require(selected and selected[0] > 0 and selected[-1] < len(expected), 'selected frames lack next-frame control')
    require(all(row['frame'] == index + 1 for index, row in enumerate(expected)), 'control frame sequence is not complete')
    core = ROOT / '.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    require(sha(core.read_bytes()) == HOOK_CORE_SHA, 'hook core differs from pinned identity')
    private = out / f'arena-{args.arena:x}-observer'
    private.mkdir(exist_ok=True)
    bridge, bridge_path = compile_observer(private)
    samples, observed, mismatches = [], [], []
    with ReferenceSession(ROOT, private, core=core, teamplayer=control['identity'].get('teamplayer', False)) as ref:
        vram, vsram = (C.c_uint8 * 65536).in_dll(ref.lib, 'vram'), (C.c_uint8 * 128).in_dll(ref.lib, 'vsram')
        cram, registers = (C.c_uint16 * 64).in_dll(ref.lib, 'cram'), (C.c_uint8 * 32).in_dll(ref.lib, 'reg')
        counter = C.c_uint16.in_dll(ref.lib, 'v_counter')
        vdp_cycles = C.c_uint32.in_dll(ref.lib, 'mcycles_vdp')
        bg_pointer = C.c_void_p.in_dll(ref.lib, 'render_bg')
        bridge.configure(C.addressof(vram), C.addressof(vsram), C.addressof(registers), C.addressof(cram),
                         C.addressof(counter), C.addressof(ref.cpu) + type(ref.cpu).cycles.offset,
                         C.addressof(vdp_cycles), C.addressof(bg_pointer), C.cast(ref.get_register, C.c_void_p), ref.ram_pointer)
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(C.cast(bridge.observe, C.c_void_p))
        for frame in range(1, len(expected) + 1):
            bridge.set_frame(frame)
            bridge.begin_frame(int(frame - 1 in selected))
            actual = ref.step(frame, buttons_at(inputs, frame))['reference']
            observed.append(actual)
            if actual != expected[frame - 1]:
                mismatches.append({'frame': frame, 'actual': actual, 'expected': expected[frame - 1]})
            if samples and samples[-1]['frame'] == frame - 1:
                collect_active(bridge, samples[-1], ref, frame)
            if frame in selected:
                ram = ref.ram_cpu()
                arena = int.from_bytes(ram[0xa060:0xa062], 'big')
                require(arena == args.arena and int.from_bytes(ram[0x8180:0x8184], 'big') == ARENAS[arena][0],
                        f'selected frame{frame} does not contain confirmed arena{args.arena:x}')
                samples.append({'frame': frame, 'arena_offset': arena, 'scene_object': ram[0x8180:0x81c0].hex(),
                                'ram': ram.hex(), 'vram': swapped(bytes(vram)).hex(), 'vsram': swapped(bytes(vsram)).hex(),
                                'cram': list(cram), 'registers': list(registers), 'width': ref.width,
                                'height': ref.height, 'pixel_format': ref.pixel_format, 'video': ref.image.hex()})
        hook(None)
        bridge.finish()
        identity = ref.identity
    require(not mismatches and observed == expected, f'passive observer changed original control at {len(mismatches)} frames')
    require(len(samples) == len(selected) and bridge.get_load_dropped() == 0, 'incomplete original observation')
    loads = [{'frame': bridge.get_load_frame(index), 'resource_list': bridge.get_load_list(index)}
             for index in range(bridge.get_load_count())]
    (out / f'arena-{args.arena:x}-load-observation.json').write_text(json.dumps({
        'arena_offset': args.arena, 'frames_match_original_core': len(observed),
        'control_mismatches': len(mismatches), 'observed_loads': loads}, indent=2) + '\n')
    known_lists = {0x21720, *(entry[1] for entry in ARENAS.values())}
    require(loads and all(row['resource_list'] in known_lists for row in loads)
            and loads[-1]['resource_list'] == ARENAS[args.arena][1] and loads[-1]['frame'] < selected[0],
            f'final original resource-list identity differs from selected arena: {loads}')
    rom = (ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    require(sha(rom) == ROM_SHA256, 'ROM changed during capture')
    binding, _, _ = resource_bindings(rom, args.arena)
    palette = binding['resources'][-1]
    palette_words = [int.from_bytes(rom[palette['source'] + index * 2:palette['source'] + index * 2 + 2], 'big')
                     for index in range(16)]
    loaded_palette = swapped(bytes(bridge.get_loaded_palette()[:32]))
    require(bridge.get_palette_frame() >= loads[-1]['frame'] and loaded_palette == rom[palette['source']:palette['source'] + 32],
            'initial scene palette was not observed at original resource completion')
    palette_observation = {'frame': bridge.get_palette_frame(), 'pc': 0x83c, 'ram_address': 0xe080,
                           'bytes': loaded_palette.hex(), 'sha256': sha(loaded_palette)}
    data = {'schema': 1, 'identity': identity, 'frames': samples, 'bindings': [binding],
            'observed_loads': loads, 'palette_words': palette_words, 'initial_scene_palette_observation': palette_observation,
            'selected_resource_load': loads[-1], 'earlier_menu_resource_loads': loads[:-1],
            'original_control': {'path': str(args.control_manifest.resolve().relative_to(ROOT)),
                                 'sha256': sha(args.control_manifest.read_bytes()), 'frames': len(expected),
                                 'input_sha256': sha(inputs_raw), 'output_sha256': sha(frames_raw)}}
    raw = (json.dumps(data, separators=(',', ':')) + '\n').encode()
    compressed = gzip.compress(raw, mtime=0)
    name = f'arena-assets-{args.arena:x}-reference'
    (out / f'{name}.json.gz').write_bytes(compressed)
    report = {'pass': True, 'scope': f'Passive arena{args.arena:x} original VDP/video references; selected samples only',
              'identity': identity, 'original_control': data['original_control'], 'producer_sha256': sha(Path(__file__).read_bytes()),
              'observer_source_sha256': sha(OBSERVER.encode()), 'observer_sha256': sha(bridge_path.read_bytes()),
              'observer_route': 'Actual background-render entry wrapper calls original exactly once; CPU observation tracks palette/blank/redraw bindings',
              'render_function_rebindings': bridge.get_rebindings(), 'records': len(samples), 'observed_loads': loads,
              'selected_resource_load': loads[-1], 'earlier_menu_resource_loads': loads[:-1],
              'initial_scene_palette_observation': palette_observation,
              'frames_match_original_core': len(observed), 'control_mismatches': len(mismatches),
              'active_line_observations': len(samples) * 224,
              'active_background_vram_changed_frames': [sample['frame'] for sample in samples if sample['active_background_vram_changed']],
              'active_vram_change_rows': sum(len(sample['active_vram_changes']) for sample in samples),
              'fixture_sha256': sha(raw), 'gzip_sha256': sha(compressed), 'gzip_bytes': len(compressed),
              'register_states': sorted({tuple(sample['registers']) for sample in samples}),
              'limits': ['Original references are development-only, not runtime assets',
                         'Selected fixed-input samples do not prove unobserved dynamic material states']}
    (out / f'{name}-manifest.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2), flush=True)


if __name__ == '__main__':
    main()
