#!/usr/bin/env python3
"""Passive M68K shape-buffer read inventory in the actual battle window."""
import argparse
import ctypes as C
import json
import os
from pathlib import Path
import subprocess
from reference import ROOT,ReferenceSession,buttons_at,sha

SOURCE=r'''
#include <stdint.h>
typedef uint32_t(*reg_t)(unsigned);
typedef struct {uint32_t pc,hits,groups,first,last,min,max,sr,sp,a[7],recent[16],stack[16];} record_t;
static record_t records[4096];static uint32_t count,dropped,last_pc,frame,recent[32],head;
static reg_t reg;static const uint8_t *ram;
void configure(reg_t reader,const uint8_t *memory){reg=reader;ram=memory;}
void set_frame(uint32_t f){frame=f;}
uint32_t get_count(void){return count;}uint32_t get_dropped(void){return dropped;}
const record_t* get_records(void){return records;}
static uint32_t word(uint32_t address){return ((uint32_t)ram[(address&65535)^1]<<8)|ram[((address+1)&65535)^1];}
void observe(int type,int width,unsigned address,unsigned value){
 (void)value;
 if(type==1){last_pc=address;recent[head++&31]=address;return;}
 if(type!=2||frame<1800||frame>3600)return;
 address&=0xffffff;
 if(address>=0xffa600||address+(unsigned)width<=0xffa200)return;
 uint32_t i=0;for(;i<count;i++)if(records[i].pc==last_pc)break;
 if(i==count){
  if(count==4096){dropped++;return;}
  count++;record_t*r=&records[i];r->pc=last_pc;r->first=frame;r->min=address;r->max=address;
  r->sr=reg(17);r->sp=reg(15);for(unsigned n=0;n<7;n++)r->a[n]=reg(8+n);
  for(unsigned n=0;n<16;n++)r->recent[n]=recent[(head-n-1)&31];
  uint32_t sp=r->sp&0xffffff;if(sp>=0xff0000&&sp+64<=0x1000000)for(unsigned n=0;n<16;n++)r->stack[n]=(word(sp+n*4)<<16)|word(sp+n*4+2);
 }
 record_t*r=&records[i];r->hits++;r->last=frame;if(address<r->min)r->min=address;if(address>r->max)r->max=address;
 for(unsigned g=0;g<4;g++)if(address<0xffa300+g*256&&address+(unsigned)width>0xffa200+g*256)r->groups|=1u<<g;
}
'''

class Record(C.Structure):
    _fields_=[(n,C.c_uint32)for n in ['pc','hits','groups','first','last','min','max','sr','sp']]+[('a',C.c_uint32*7),('recent',C.c_uint32*16),('stack',C.c_uint32*16)]

def main():
    p=argparse.ArgumentParser();p.add_argument('--out',required=True,type=Path);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    for d in ['tmp','cache']:(out/d).mkdir(exist_ok=True)
    source=out/'observer.c';source.write_text(SOURCE);dylib=out/'observer.dylib'
    subprocess.run(['clang','-dynamiclib','-O2',str(source),'-o',str(dylib)],check=True,capture_output=True,env=dict(os.environ,TMPDIR=str(out/'tmp'),CLANG_MODULE_CACHE_PATH=str(out/'cache')))
    bridge=C.CDLL(str(dylib));bridge.configure.argtypes=[C.c_void_p,C.c_void_p];bridge.set_frame.argtypes=[C.c_uint32];bridge.get_count.restype=C.c_uint32;bridge.get_dropped.restype=C.c_uint32;bridge.get_records.restype=C.POINTER(Record)
    events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
    core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
    with ReferenceSession(ROOT,out,core=core) as ref:
        bridge.configure(C.cast(ref.get_register,C.c_void_p),ref.ram_pointer)
        ref.api('set_cpu_hook',None,C.c_void_p)(C.cast(bridge.observe,C.c_void_p));frames=[]
        for f in range(1,3601):bridge.set_frame(f);frames.append(ref.step(f,buttons_at(events,f))['reference'])
        ref.api('set_cpu_hook',None,C.c_void_p)(None)
        count=bridge.get_count();records=bridge.get_records();rows=[]
        for i in range(count):
            r=records[i]
            rows.append({'pc':hex(r.pc),'reads':r.hits,'groups':r.groups,'first_frame':r.first,'last_frame':r.last,'address_min':hex(r.min),'address_max':hex(r.max),'sr':hex(r.sr),'sp':hex(r.sp),'address_registers':[hex(v)for v in r.a],'recent_pcs':[hex(v)for v in r.recent],'stack_candidates':[hex(v)for v in r.stack],'inside_known_producer':0x1eb40<=r.pc<=0x1edaa})
        report={'scope':'M68K data reads of shape buffers, frames 1800..3600, fixed input','identity':ref.identity,'trace_source_sha256':sha(SOURCE.encode()),'script_sha256':sha(Path(__file__).read_bytes()),'observer_sha256':sha(dylib.read_bytes()),'frames_match_original_core':frames==expected,'unique_pcs':count,'dropped':bridge.get_dropped(),'observations':sorted(rows,key=lambda r:int(r['pc'],16)),'pass':frames==expected and count>0 and bridge.get_dropped()==0,'limitations':['Only selected core M68K read hooks are covered','Stack words are candidates, not recovered call-stack frames','One battle sample does not cover all characters, modes or consumers']}
    (out/'report.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:v for k,v in report.items()if k!='observations'},indent=2))
    for r in report['observations']:print(r['pc'],r['reads'],r['groups'],r['inside_known_producer'])
    assert report['pass']

if __name__=='__main__':main()
