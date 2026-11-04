#!/usr/bin/env python3
"""Original arena video/VDP source references; no production emulation."""
import argparse
import ctypes as C
import gzip
import json
import os
from pathlib import Path
import subprocess
from reference import ROOT, ReferenceSession, buttons_at, sha

OBSERVER = r'''
#include <stdint.h>
#include <string.h>
typedef struct {uint8_t seen;int32_t first_cycle;uint8_t regs[32],vsram[4],hscroll[4];uint16_t cram[64];} line_t;
static line_t rows[224];static uint8_t first_vram[65536];
static const uint8_t *vram,*vsram,*regs;static const uint16_t *cram,*vc;static const int32_t *cycles;static const uint32_t *base;
typedef void(*render_t)(int);static render_t original_bg;static render_t*background;
static int enabled,captured,changed,pending_palette=-1;static uint32_t rebindings;
typedef uint32_t(*reg_t)(unsigned);static reg_t reader;static uint32_t frame,load_count,load_dropped,load_frames[16],load_lists[16];
static void record(unsigned y){
 line_t*l=&rows[y];l->first_cycle=*cycles-*base;memcpy(l->regs,regs,32);memcpy(l->vsram,vsram,4);memcpy(l->cram,cram,128);
 unsigned masks[4]={0,7,248,255};unsigned h=((regs[13]&63)<<10)+((y&masks[regs[11]&3])<<2);memcpy(l->hscroll,vram+(h&65535),4);
}
void observe_bg(int line){
 if(enabled&&line>=0&&line<224){
  if(!captured){memcpy(first_vram,vram,65536);captured=1;}
  else if(memcmp(first_vram,vram,0x4000)||memcmp(first_vram+0x8000,vram+0x8000,0x7000))changed=1;
  record(line);rows[line].seen=1;
 }
 original_bg(line);
}
void configure(const void*v,const void*vs,const void*r,const void*c,const void*counter,const void*cpu_cycles,const void*vdp_cycles,void*bg_pointer,void*registers_reader){vram=v;vsram=vs;regs=r;cram=c;vc=counter;cycles=cpu_cycles;base=vdp_cycles;background=bg_pointer;original_bg=*background;*background=observe_bg;reader=(reg_t)registers_reader;}
void set_frame(uint32_t f){frame=f;}uint32_t get_load_count(void){return load_count;}uint32_t get_load_dropped(void){return load_dropped;}uint32_t get_load_frame(uint32_t i){return load_frames[i];}uint32_t get_load_list(uint32_t i){return load_lists[i];}
void finish(void){if(*background==observe_bg)*background=original_bg;}
void begin_frame(int active){enabled=active;captured=changed=0;pending_palette=-1;memset(rows,0,sizeof(rows));memset(first_vram,0,sizeof(first_vram));if(active)for(unsigned y=0;y<224;y++)record(y);}
const line_t* get_lines(void){return rows;}const uint8_t* get_first_vram(void){return first_vram;}
int get_captured(void){return captured;}int get_changed(void){return changed;}uint32_t get_rebindings(void){return rebindings;}
void observe(int type,int width,unsigned address,unsigned value){
 (void)width;(void)value;
 if(type==4){unsigned a=address&0xffffff;if(enabled&&*vc<224&&a>=0xc00000&&a<0xc00004&&*cycles-*base<=860)pending_palette=*vc;return;}
 if(type!=1)return;
 if(address==0x800){uint32_t a=reader(8)&0xffffff;if(a>=0x21720&&a<0x218d8){if(load_count<16){load_frames[load_count]=frame;load_lists[load_count++]=a;}else load_dropped++;}}
 if(pending_palette>=0){memcpy(rows[pending_palette].cram,cram,128);pending_palette=-1;}
 if(*background!=observe_bg){original_bg=*background;*background=observe_bg;rebindings++;}
 if(!enabled||*vc>=224)return;
 if(*cycles-*base<=860&&!(regs[1]&64)){record(*vc);rows[*vc].seen=0;}
}
'''


class Line(C.Structure):
    _fields_ = [('seen', C.c_uint8), ('first_cycle', C.c_int32), ('regs', C.c_uint8*32),
                ('vsram', C.c_uint8*4), ('hscroll', C.c_uint8*4), ('cram', C.c_uint16*64)]


def swapped(raw):
    result = bytearray(len(raw))
    result[0::2], result[1::2] = raw[1::2], raw[0::2]
    return bytes(result)


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--out', required=True, type=Path)
    args = p.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    (out/'tmp').mkdir(exist_ok=True)
    (out/'cache').mkdir(exist_ok=True)
    source = out/'observer.c'
    source.write_text(OBSERVER)
    bridge_path = out/'observer.dylib'
    subprocess.run(['clang', '-dynamiclib', '-O2', str(source), '-o', str(bridge_path)], check=True,
                   capture_output=True, env=dict(os.environ, TMPDIR=str(out/'tmp'), CLANG_MODULE_CACHE_PATH=str(out/'cache')))
    bridge = C.CDLL(str(bridge_path))
    bridge.configure.argtypes = [C.c_void_p]*9
    bridge.begin_frame.argtypes = [C.c_int]
    bridge.get_lines.restype = C.POINTER(Line)
    bridge.get_first_vram.restype = C.POINTER(C.c_uint8)
    bridge.get_captured.restype = C.c_int
    bridge.get_changed.restype = C.c_int
    bridge.get_rebindings.restype = C.c_uint32
    bridge.set_frame.argtypes = [C.c_uint32]
    for name in ['get_load_count', 'get_load_dropped', 'get_load_frame', 'get_load_list']:
        getattr(bridge, name).restype = C.c_uint32
    bridge.get_load_frame.argtypes = [C.c_uint32]
    bridge.get_load_list.argtypes = [C.c_uint32]
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    frames, samples = [], []
    selected = [1800, 1855, 1955, 2005, 2055, 2105, 2155, 2255, 2355, 2455, 2505, 2555, 2605, 2655, 2705, 2805, 2905, 2955, 3055, 3155]
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    with ReferenceSession(ROOT, out, core=core) as ref:
        vram = (C.c_uint8*65536).in_dll(ref.lib, 'vram')
        vsram = (C.c_uint8*128).in_dll(ref.lib, 'vsram')
        cram = (C.c_uint16*64).in_dll(ref.lib, 'cram')
        registers = (C.c_uint8*32).in_dll(ref.lib, 'reg')
        counter = C.c_uint16.in_dll(ref.lib, 'v_counter')
        vdp_cycles = C.c_uint32.in_dll(ref.lib, 'mcycles_vdp')
        bg_pointer = C.c_void_p.in_dll(ref.lib, 'render_bg')
        bridge.configure(C.addressof(vram), C.addressof(vsram), C.addressof(registers), C.addressof(cram),
                         C.addressof(counter), C.addressof(ref.cpu)+type(ref.cpu).cycles.offset, C.addressof(vdp_cycles), C.addressof(bg_pointer), C.cast(ref.get_register, C.c_void_p))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(C.cast(bridge.observe, C.c_void_p))
        for f in range(1, 3601):
            bridge.set_frame(f)
            bridge.begin_frame(int(f-1 in selected))
            frames.append(ref.step(f, buttons_at(events, f))['reference'])
            if samples and samples[-1]['frame'] == f-1:
                samples[-1]['video_next'] = ref.image.hex()
                samples[-1]['video_next_frame'] = f
                lines = bridge.get_lines()
                samples[-1]['active_vram'] = swapped(bytes(bridge.get_first_vram()[:65536])).hex() if bridge.get_captured() else samples[-1]['vram']
                samples[-1]['active_background_vram_changed'] = bool(bridge.get_changed())
                samples[-1]['active_lines'] = [{'line': y, 'draw': bool(lines[y].seen), 'first_cycle': lines[y].first_cycle,
                                               'registers': list(lines[y].regs), 'cram': list(lines[y].cram),
                                               'vsram': swapped(bytes(lines[y].vsram)).hex(),
                                               'hscroll': swapped(bytes(lines[y].hscroll)).hex()} for y in range(224)]
            if f in selected:
                ram = ref.ram_cpu()
                samples.append({'frame': f, 'arena_offset': int.from_bytes(ram[0xa060:0xa062], 'big'),
                                'scene_object': ram[0x8180:0x81c0].hex(), 'ram': ram.hex(),
                                'vram': swapped(bytes(vram)).hex(), 'vsram': swapped(bytes(vsram)).hex(),
                                'cram': list(cram), 'registers': list(registers), 'width': ref.width,
                                'height': ref.height, 'pixel_format': ref.pixel_format, 'video': ref.image.hex()})
        identity = ref.identity
        hook(None)
        bridge.finish()
        rebindings = bridge.get_rebindings()
        loads = [{'frame': bridge.get_load_frame(i), 'resource_list': bridge.get_load_list(i)}for i in range(bridge.get_load_count())]
        assert bridge.get_load_dropped() == 0
    assert frames == expected and len(samples) == len(selected)
    assert all(s['video_next_frame'] == s['frame']+1 for s in samples)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    word = lambda offset: int.from_bytes(rom[offset:offset+2], 'big')
    long = lambda offset: int.from_bytes(rom[offset:offset+4], 'big')
    bindings = []
    for i in range(8):
        start = long(0x21700+i*4)
        pointer, resources = start, []
        while word(pointer) != 65535:
            kind, source, destination = word(pointer), long(pointer+2), word(pointer+6)
            assert kind in [0, 6]
            entry = {'kind': kind, 'source': source, 'destination': destination}
            if kind == 0:
                entry['length'] = word(pointer+8)
            else:
                entry['encoded_bytes'] = word(source)
                entry['encoded_sha256'] = sha(rom[source:source+2+word(source)])
            resources.append(entry)
            pointer += 10 if kind == 0 else 8
        bindings.append({'arena_offset': i*2, 'scene_behavior': long(0xf94c+i*4), 'resource_list': start,
                         'list_sha256': sha(rom[start:pointer+2]), 'resources': resources})
    palette = bindings[0]['resources'][-1]
    assert palette['kind'] == 0 and palette['destination'] == 0xe080 and palette['length'] == 32
    palette_words = [word(palette['source']+i*2)for i in range(16)]
    assert loads and {l['resource_list']for l in loads} == {bindings[0]['resource_list']}
    data = {'schema': 1, 'identity': identity, 'frames': samples, 'bindings': bindings, 'observed_loads': loads, 'palette_words': palette_words}
    raw = (json.dumps(data, separators=(',', ':'))+'\n').encode()
    zipped = gzip.compress(raw, mtime=0)
    (out/'arena-assets-reference.json.gz').write_bytes(zipped)
    report = {'scope': 'Untouched original default-arena natural VDP/video references',
              'identity': identity, 'producer_sha256': sha(Path(__file__).read_bytes()), 'records': len(samples),
              'observer_source_sha256': sha(OBSERVER.encode()), 'observer_sha256': sha(bridge_path.read_bytes()),
              'active_line_observations': sum(len(s['active_lines'])for s in samples),
              'observer_route': 'Wrapper records actual background-render entry, invokes original exactly once; CPU hook tracks rebindings/blank redraws',
              'render_function_rebindings': rebindings,
              'active_background_vram_changes': sum(s['active_background_vram_changed']for s in samples),
              'known_resource_bindings': len(bindings), 'observed_loads': loads,
              'arena_offsets': sorted({s['arena_offset'] for s in samples}),
              'register_states': [list(x) for x in sorted({tuple(s['registers'])for s in samples})],
              'fixture_sha256': sha(raw), 'gzip_sha256': sha(zipped), 'frames_match_original_core': frames == expected,
              'pass': True, 'limits': ['Raw references alone are not independent presentation or ROM bindings',
                                    'Selected fixed-input default-arena frames only']}
    (out/'arena-assets-reference-manifest.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
