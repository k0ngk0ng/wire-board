#!/usr/bin/env python3
"""Bounded independent world sprite assets and original presentation evidence."""
import argparse
import ctypes as C
import gzip
import json
import os
import subprocess
from pathlib import Path
from reference import ROOT, ReferenceSession, ROM_SHA256, buttons_at, sha
from extract_animation_catalog import be
from extract_sprites import snapshot as sprite_snapshot

EXTRA_SETS = [(0xd8f1a, 0x240000, 0xd92ca, False), (0xd9e4a, 0x246f80, 0xda16a, False), (0xda16a, 0x24b620, 0xda1da, False),
              (0xda1da, 0x24b9e0, 0xdaa4a, False), (0xdaa4a, 0x25d260, 0xe0000, False), (0x27f618, 0, 0x300000, True)]
SELECTED = [1800, 1855, 1955, 2005, 2055, 2105, 2155, 2170, 2178, 2179, 2180, 2181, 2182, 2184, 2190, 2200, 2255, 2355, 2400, 2455, 2505, 2555, 2605, 2655, 2705, 2805, 2905, 2955, 3055, 3155]
OBSERVER = r'''
#include <stdint.h>
typedef uint32_t(*reg_t)(unsigned);typedef void(*callback_t)(int,int,unsigned,unsigned);
static reg_t reg;static callback_t cb;static const uint8_t*ram;static uint32_t ret,sp;static int active,pending;
void configure(reg_t r,const uint8_t*m,callback_t c){reg=r;ram=m;cb=c;}
void set_active(int a){active=a;pending=0;}
static uint32_t byte(uint32_t a){return ram[(a&65535)^1];}
static uint32_t longword(uint32_t a){return byte(a)<<24|byte(a+1)<<16|byte(a+2)<<8|byte(a+3);}
void observe(int type,int width,unsigned pc,unsigned value){
 (void)width;(void)value;if(type!=1||!active)return;
 if(pending&&pc==ret&&reg(15)==sp+4){pending=0;cb(3,0,pc,0);}
 if(pc==0x129c)cb(1,0,pc,0);
 if(pc==0x1580){sp=reg(15);ret=longword(sp);pending=1;cb(2,0,pc,0);}
}
'''


def swapped(raw):
    result = bytearray(len(raw)); result[0::2], result[1::2] = raw[1::2], raw[0::2]
    return bytes(result)


def extra_frames(rom):
    frames = []
    bounds = []
    for graphics, bank, limit, preloaded in EXTRA_SETS:
        first = be(rom, graphics, 2)
        assert first > 0 and first % 2 == 0
        offsets = [be(rom, graphics+i*2, 2, True) for i in range(first//2)]
        valid = [(i*2, v) for i, v in enumerate(offsets) if v != 0]
        assert valid and min(v for _, v in valid) == first
        assert all(v >= first for _, v in valid)
        assert [v for _, v in valid] == sorted(set(v for _, v in valid))
        for n, (parameter, offset) in enumerate(valid):
            start = graphics+offset
            end_limit = graphics+valid[n+1][1] if n+1 < len(valid) else limit
            cursor = start
            for _ in range(4):
                word = be(rom, cursor, 4); cursor += 4
                if word&0x80000000: break
            for _ in range(80):
                word = be(rom, cursor, 4); cursor += 4
                if word&0x80000000: break
            else: raise ValueError('Unterminated bounded world sprite frame')
            assert cursor <= end_limit
            frames.append({'graphics_set': graphics, 'parameter': parameter, 'tiles_base': bank,
                           'preloaded_only': preloaded, 'source_address': start, 'payload': rom[start:cursor].hex(),
                           'source_sha256': sha(rom[start:cursor])})
        bounds.append({'graphics_set': graphics, 'table_bytes': first, 'nonzero_keys': len(valid),
                       'excluded_zero_parameters': [i*2 for i, value in enumerate(offsets) if value == 0], 'upper_bound': limit})
    return frames, bounds


def main():
    parser = argparse.ArgumentParser(); parser.add_argument('--out', required=True, type=Path); args = parser.parse_args()
    out = args.out.resolve(); assert out.is_relative_to(ROOT); out.mkdir(parents=True, exist_ok=True)
    (out/'tmp').mkdir(exist_ok=True); (out/'cache').mkdir(exist_ok=True)
    rom = (ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes(); assert sha(rom) == ROM_SHA256
    old = json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/data/sprite-oracle.json.gz').read_bytes()))
    character_assets = json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/data/character-assets.json.gz').read_bytes()))
    additional, bounds = extra_frames(rom)
    originals = [{**f, 'preloaded_only': False} for f in old['library']]+additional
    bank_bytes = bytes.fromhex(character_assets['tile_bytes'])
    tiles = [bank_bytes[i:i+32] for i in range(0, len(bank_bytes), 32)]
    tile_ids = {value: i for i, value in enumerate(tiles)}
    def intern(raw):
        assert len(raw) == 32
        if raw not in tile_ids: tile_ids[raw] = len(tiles); tiles.append(raw)
        return tile_ids[raw]
    semantic_frames = []
    for f in originals:
        raw = bytes.fromhex(f['payload']); cursor = 0; transfers = []
        for _ in range(4):
            descriptor = be(raw, cursor, 4); cursor += 4; stamp = descriptor&0x7fffffff
            assigned = []
            if not f['preloaded_only']:
                for index in range(stamp&255):
                    address = f['tiles_base']+((stamp>>16)+index)*32
                    assigned.append(intern(rom[address:address+32]))
            transfers.append({'stamp': stamp, 'tiles': assigned})
            if descriptor&0x80000000: break
        pieces = []
        while cursor < len(raw):
            word = be(raw, cursor, 4); cursor += 4
            pieces.append({'center_x': ((word>>24)&127)-64, 'center_y': ((word>>16)&127)-64,
                           'width_tiles': ((word>>10)&3)+1, 'height_tiles': ((word>>8)&3)+1,
                           'first_slot': word&255, 'xor_attributes': (word&0xf000)>>1})
            if word&0x80000000: break
        assert cursor == len(raw)
        semantic_frames.append({'key': {'graphics_set': f['graphics_set'], 'parameter': f['parameter']},
                                'tile_bank': f['tiles_base'], 'preloaded_only': f['preloaded_only'],
                                'transfers': transfers, 'pieces': pieces})
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    source = out/'observer.c'; source.write_text(OBSERVER); bridge_path = out/'observer.dylib'
    subprocess.run(['clang','-dynamiclib','-O2',str(source),'-o',str(bridge_path)], check=True, capture_output=True,
                   env=dict(os.environ,TMPDIR=str(out/'tmp'),CLANG_MODULE_CACHE_PATH=str(out/'cache')))
    bridge = C.CDLL(str(bridge_path)); cbtype = C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
    bridge.configure.argtypes = [C.c_void_p,C.c_void_p,cbtype]; bridge.set_active.argtypes = [C.c_int]
    filter_lib = C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'))
    filter_lib.yyh_set_observer.argtypes = [cbtype,C.POINTER(C.c_uint),C.c_size_t]; filter_lib.yyh_set_observer.restype = C.c_int
    input_paths = [ROOT/'docs/research/md-yuyuhakusho/inputs.json', ROOT/'games/yuyuhakusho/engine/testdata/chu-special-motion-inputs.json']
    natural, cases, errors, baselines = [], [], [], []
    static_slots = None
    for input_index, input_path in enumerate(input_paths):
        events = json.loads(input_path.read_text())
        selected = sorted(SELECTED+[1600, 1620, 1640, 1660, 1680, 1700, 1740, 1780]+([4295, 4300, 4320, 4350, 4400, 4500, 4700] if input_index == 0 else []))
        frame_limit = 5500 if input_index == 0 else 3600
        if input_index == 0:
            expected = json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/engine/testdata/match-lifecycle-reference-frames.json.gz').read_bytes()))
            assert len(expected) == frame_limit and expected[:3600] == json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
        else:
            with ReferenceSession(ROOT,out/'unobserved-special') as reference:
                expected = [reference.step(f,buttons_at(events,f))['reference']for f in range(1,3601)]
        with ReferenceSession(ROOT,out/('natural-'+str(input_index)),core=core) as ref:
            vram = (C.c_uint8*65536).in_dll(ref.lib,'vram'); cram = (C.c_uint16*64).in_dll(ref.lib,'cram'); regs = (C.c_uint8*32).in_dll(ref.lib,'reg')
            by_frame = {}; pending = None
            @cbtype
            def observe(kind,width,pc,value):
                nonlocal pending
                try:
                    ram = ref.ram_cpu()
                    if kind == 1:
                        if ref.frame-1 in by_frame:
                            by_frame[ref.frame-1].update(uploaded_vram=swapped(bytes(vram)).hex(),palette=list(cram),registers=list(regs))
                        if ref.frame in selected:
                            assert ref.frame not in by_frame and be(ram,0xf578,2) == be(ram,0xf57a,2)+1
                            row = {'scenario':input_path.name,'frame':ref.frame,'before_vram':swapped(bytes(vram)).hex(),
                                   'initial_count':be(ram,0xf578,2),'initial_flags':ram[0xf54f],
                                   'reserved_sprites':ram[0xe200:0xe200+be(ram,0xf578,2)*8].hex(),'draws':[]}
                            by_frame[ref.frame] = row
                    elif ref.frame in by_frame:
                        row = by_frame[ref.frame]
                        if kind == 2:
                            assert pending is None
                            address = ref.get_register(13)&0xffff
                            pending = {'owner':address,'actor':ram[address:address+256].hex(),'count':be(ram,0xf578,2),
                                       'flags':ram[0xf54f],'cursor':be(ram,0xf57c,2)}
                        elif kind == 3:
                            assert pending is not None
                            address = pending['owner']; pending.update(after_actor=ram[address:address+256].hex(),after_count=be(ram,0xf578,2),
                                                                      after_flags=ram[0xf54f],after_cursor=be(ram,0xf57c,2))
                            row['draws'].append(pending); pending = None
                except Exception as exc: errors.append(f'frame={ref.frame} event={kind} pc={pc:x} pending={pending is not None}: {exc!r}')
            bridge.configure(C.cast(ref.get_register,C.c_void_p),ref.ram_pointer,observe)
            hook = ref.api('set_cpu_hook',None,C.c_void_p); hook(C.cast(bridge.observe,C.c_void_p)); seen = []
            for f in range(1,frame_limit+1):
                bridge.set_active(int(f in selected or f-1 in selected))
                seen.append(ref.step(f,buttons_at(events,f))['reference'])
                if f-1 in by_frame:
                    row = by_frame[f-1]; row.update(video_next=ref.image.hex(),video_frame=f)
                if f in by_frame:
                    row = by_frame[f]; row.update(palette_after_frame=list(cram),registers_after_frame=list(regs),width=ref.width,height=ref.height,pixel_format=ref.pixel_format)
                if f == 2400:
                    checkpoint = ref.save(); base_vram = swapped(bytes(vram))
                    if static_slots is None:
                        static_slots = [{'slot':slot,'tile':intern(base_vram[slot*32:(slot+1)*32])}for slot in range(0xc0,0x200)]
            hook(None); assert seen == expected and not pending and not errors, (errors,pending)
            assert len(by_frame) == len(selected)
            for row in by_frame.values():
                assert row.get('uploaded_vram')and row.get('video_next')and row['video_frame'] == row['frame']+1
                natural.append(row)
            baselines.append({'inputs':str(input_path.relative_to(ROOT)),'sha256':sha(input_path.read_bytes()),'frames':frame_limit,'match':seen==expected})
            if input_index == 0: identity = ref.identity; continue
            # Explicit isolated submissions test all extra graph keys and edge
            # states through the ROM's full renderer return, not a re-encoder.
            sample = None
            @cbtype
            def finish(kind,width,pc,value):
                nonlocal sample
                assert sample is None
                sample = ref.ram_cpu(); ref.cpu.cycles = 4000000
            points = (C.c_uint*1)(0x3d8); assert filter_lib.yyh_set_observer(finish,points,1)
            observer = C.cast(filter_lib.yyh_cpu_hook,C.c_void_p)
            setreg = ref.api('m68k_set_reg',None,C.c_uint,C.c_uint); execute = ref.api('m68k_run',None,C.c_uint)
            def write(address,value,size=1):
                for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):
                    C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&0xffff)^1)).value=b
            for index, frame in enumerate(additional):
                for variant in range(22 if frame['parameter'] == 2 else 8):
                    ref.restore(checkpoint,restore_refresh_cycles=True); setreg(17,0x2700); setreg(15,0xffff00); write(0xffff00,0x3d8,4); setreg(13,0xffff8400)
                    for reg in range(8): setreg(reg,0)
                    for off,value,size in [(0x1c,frame['graphics_set'],4),(0x20,frame['tiles_base'],4),(0x3c,frame['parameter'],2),
                                           (8,0x180,2),(12,0x140,2),(0x28,-7 if variant&1 else 9,2),(0x2a,3 if variant&2 else -5,2),
                                           (0x24,0x8200|((variant&3)<<11),2),(0x26,0x40,1),(0x27,int(frame['preloaded_only']or variant==17),1),
                                           (0x3e,0x100 if variant<4 else 0xd8 if variant<8 else [0,0xffff,0x80,0x7fff][(variant-8)%4],2)]:write(0xff8400+off,value,size)
                    for i in range(4):write(0xff842c+i*4,0x11223344 if variant&1 else 0,4)
                    if variant==21:
                        payload=bytes.fromhex(frame['payload'])
                        for i in range(4):
                            value=be(payload,i*4,4);write(0xff842c+i*4,value&0x7fffffff,4)
                            if value&0x80000000:
                                for j in range(i+1,4):write(0xff842c+j*4,0,4)
                                break
                    if variant in [13,14]:write(0xff8408,(0x40 if variant==13 else 0x200)-(9 if not variant&1 else -7),2)
                    if variant in [15,16]:write(0xff840c,(0x40 if variant==15 else 0x1a0)-(3 if variant&2 else -5),2)
                    write(0xfff54f,0 if variant==12 else 0x80);write(0xfff518,0xd400,2);write(0xfff578,{18:79,19:80,20:255}.get(variant,0),2);write(0xfff57c,0xe600,2)
                    for i in range(0,256,4):write(0xfff302+i+2,0xe600,2)
                    before=ref.ram_cpu();sample=None;setreg(16,0x1580);ref.cpu.cycles=0;ref.cpu.refresh_cycles=896;hook(observer);execute(1000000);hook(None)
                    assert sample is not None
                    allowed=set(range(0x8400,0x8500))|set(range(0xd400,0xf000))|set(range(0xf300,0xf400))|{0xf518,0xf519,0xf54f,0xf578,0xf579,0xf57c,0xf57d}|set(range(0xfec0,0xff04))
                    assert not [i for i,(a,b)in enumerate(zip(before,sample))if a!=b and i not in allowed]
                    cases.append({'graphics_set':frame['graphics_set'],'parameter':frame['parameter'],'variant':variant,'before':sprite_snapshot(before),'after':sprite_snapshot(sample),'return':'0x3d8'})
    catalog = {'schema':1,'tile_format':'md-4bpp-row-major-high-nibble-first','tile_bytes':b''.join(tiles).hex(),'frames':semantic_frames,
               'default_static_slots':static_slots,'characters':[{**c,'graphics_set':next(f['graphics_set']for f in old['library']if f['character_id']==c['character_id'])}for c in character_assets['palettes']],
               'shared_palettes':character_assets['shared_palettes'],'rom_sha256':ROM_SHA256}
    payload = {'schema':1,'rom_sha256':ROM_SHA256,'library':additional,'calls':cases,'natural':natural,'catalog_source_frames':originals}
    for name,value in [('world-sprites',catalog),('world-sprites-oracle',payload)]:
        raw=(json.dumps(value,separators=(',',':'))+'\n').encode();zipped=gzip.compress(raw,mtime=0);(out/(name+'.json.gz')).write_bytes(zipped)
    report={'scope':'Independent world sprite assets and full original renderer/natural frame evidence','identity':identity,'script_sha256':sha(Path(__file__).read_bytes()),
            'catalog_sha256':sha(gzip.decompress((out/'world-sprites.json.gz').read_bytes())),'catalog_gzip_sha256':sha((out/'world-sprites.json.gz').read_bytes()),
            'oracle_sha256':sha(gzip.decompress((out/'world-sprites-oracle.json.gz').read_bytes())),'oracle_gzip_sha256':sha((out/'world-sprites-oracle.json.gz').read_bytes()),
            'source_character_assets_sha256':sha((ROOT/'games/yuyuhakusho/data/character-assets.json.gz').read_bytes()),'extra_graphics_tables':bounds,
            'extra_frames':len(additional),'all_frames':len(semantic_frames),'tiles':len(tiles),'original_cpu_calls':len(cases),'natural_frames':len(natural),
            'baselines':baselines,'frames_match_original_core':all(b['match']for b in baselines),'errors':errors,'pass':True,
            'limits':['Graphics table reachability beyond referenced sequences is not proved','Default preloaded slots are declared scene0 initial data, not universal dynamic UI','Only normal high-priority natural sprite pixels are compared; background merging and scanline limits are separate']}
    (out/'world-sprites-manifest.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
