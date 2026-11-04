#!/usr/bin/env python3
"""Passive readers of character graphic parameters in the pinned MD program."""
import argparse
import ctypes as C
import json
import os
from pathlib import Path
import subprocess
from reference import ROOT, ReferenceSession, buttons_at, sha

SOURCE = r'''
#include <stdint.h>
typedef uint32_t (*reg_t)(unsigned);
typedef struct { uint32_t pc,hits,fields,owners,first,last,address,sr,sp,a[7],recent[24]; } record_t;
static record_t records[4096]; static uint32_t count,dropped,frame,last_pc,recent[64],head;
static reg_t reg;
void configure(reg_t reader){reg=reader;}
void set_frame(uint32_t f){frame=f;}
uint32_t get_count(void){return count;}
uint32_t get_dropped(void){return dropped;}
const record_t *get_records(void){return records;}
void observe(int type,int width,unsigned address,unsigned value){
 (void)value;
 if(type==1){last_pc=address;recent[head++&63]=address;return;}
 if(type!=2 || frame<1800 || frame>3600)return;
 address&=0xffffff;
 if(address<0xff8400 || address>=0xff9400)return;
 unsigned owner=(address-0xff8400)>>8,offset=address&255,fields=0;
 if(offset<0x20 && offset+(unsigned)width>0x1c)fields|=1;
 if(offset<0x24 && offset+(unsigned)width>0x20)fields|=2;
 if(offset<0x3e && offset+(unsigned)width>0x3c)fields|=4;
 if(!fields)return;
 unsigned i=0;for(;i<count;i++)if(records[i].pc==last_pc)break;
 if(i==count){
  if(count==4096){dropped++;return;}
  count++;record_t*r=&records[i];r->pc=last_pc;r->first=frame;r->address=address;
  r->sr=reg(17);r->sp=reg(15);
  for(unsigned n=0;n<7;n++)r->a[n]=reg(8+n);
  for(unsigned n=0;n<24;n++)r->recent[n]=recent[(head-n-1)&63];
 }
 record_t*r=&records[i];r->hits++;r->fields|=fields;r->owners|=1u<<owner;r->last=frame;
}
'''


class Record(C.Structure):
    _fields_ = [(n, C.c_uint32) for n in ['pc', 'hits', 'fields', 'owners', 'first', 'last', 'address', 'sr', 'sp']] + [('a', C.c_uint32*7), ('recent', C.c_uint32*24)]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--out', required=True, type=Path)
    args = parser.parse_args()
    out = args.out.resolve()
    assert out.is_relative_to(ROOT)
    out.mkdir(parents=True, exist_ok=True)
    (out/'tmp').mkdir(exist_ok=True)
    (out/'cache').mkdir(exist_ok=True)
    source = out/'observer.c'
    source.write_text(SOURCE)
    dylib = out/'observer.dylib'
    subprocess.run(['clang', '-dynamiclib', '-O2', str(source), '-o', str(dylib)], check=True, capture_output=True,
                   env=dict(os.environ, TMPDIR=str(out/'tmp'), CLANG_MODULE_CACHE_PATH=str(out/'cache')))
    bridge = C.CDLL(str(dylib))
    bridge.configure.argtypes = [C.c_void_p]
    bridge.set_frame.argtypes = [C.c_uint32]
    bridge.get_count.restype = C.c_uint32
    bridge.get_dropped.restype = C.c_uint32
    bridge.get_records.restype = C.POINTER(Record)
    events = json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text())
    expected = json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    core = ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    with ReferenceSession(ROOT, out, core=core) as ref:
        bridge.configure(C.cast(ref.get_register, C.c_void_p))
        hook = ref.api('set_cpu_hook', None, C.c_void_p)
        hook(C.cast(bridge.observe, C.c_void_p))
        frames = []
        for frame in range(1, 3601):
            bridge.set_frame(frame)
            frames.append(ref.step(frame, buttons_at(events, frame))['reference'])
        hook(None)
        count = bridge.get_count()
        records = bridge.get_records()
        rows = []
        for i in range(count):
            r = records[i]
            rows.append({'pc': hex(r.pc), 'reads': r.hits, 'fields': r.fields, 'owner_mask': hex(r.owners),
                         'first_frame': r.first, 'last_frame': r.last, 'first_address': hex(r.address),
                         'sr': hex(r.sr), 'sp': hex(r.sp), 'address_registers': [hex(v) for v in r.a],
                         'recent_pcs': [hex(v) for v in r.recent]})
        report = {'scope': 'Passive M68K graphics-field readers in battle frames1800..3600', 'identity': ref.identity,
                  'script_sha256': sha(Path(__file__).read_bytes()), 'observer_source_sha256': sha(SOURCE.encode()),
                  'observer_sha256': sha(dylib.read_bytes()), 'frames_match_original_core': frames == expected,
                  'unique_pcs': count, 'dropped': bridge.get_dropped(), 'observations': sorted(rows, key=lambda r: int(r['pc'], 16)),
                  'pass': frames == expected and count > 0 and bridge.get_dropped() == 0,
                  'limits': ['Field reads are not yet independently decoded sprites', 'Only one fixed natural battle sample',
                             'Address-register/recent-PC context is not a recovered call stack']}
    (out/'report.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps({k: v for k, v in report.items() if k != 'observations'}, indent=2))
    for row in report['observations']:
        print(row['pc'], row['reads'], row['fields'], row['owner_mask'])
    assert report['pass']


if __name__ == '__main__':
    main()
