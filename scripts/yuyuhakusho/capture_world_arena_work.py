"""Research-only full default arena/camera operation costs on two pinned cores."""
import argparse
import ctypes as C
import gzip
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha
from research_audio_loops import ORIGINAL, OBSERVED

REGIONS = {'arena': (0x8180,128), 'sky':(0x9c80,384), 'camera':(0xa00e,14), 'samples':(0xa800,128), 'prefix':(0xe600,512), 'scroll_a':(0xe800,512), 'scroll_b':(0xea00,512), 'pan_a':(0x6000,448), 'pan_b':(0x6200,448), 'vertical':(0xe500,64), 'palette_registers':(0xf500,8), 'palettes':(0xe000,384)}

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT/'.local') and not out.exists();out.mkdir(parents=True)
 source=ROOT/'games/yuyuhakusho/engine/testdata/default-arena-boundaries.json.gz';payload=json.loads(gzip.decompress(source.read_bytes()));rows=[r for r in payload['calls']if r['kind']in ['arena','camera']];assert len(rows)==792,len(rows)
 events=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text());records=[];identities={};native_steps=[]
 for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  with ReferenceSession(ROOT,out/name,core=core)as ref:
   for frame in range(1,2401):assert ref.step(frame,buttons_at(events,frame))['reference']==expected[frame-1]
   checkpoint=ref.save();identities[name]=ref.identity;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);run=ref.api('m68k_run',None,C.c_uint)
   def data(address,raw):
    for i,b in enumerate(raw):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=b
   def word(address,value,size=2):data(address,(value&((1<<(size*8))-1)).to_bytes(size,'big'))
   for index,row in enumerate(rows):
    ref.restore(checkpoint,restore_refresh_cycles=True);before=row['before']
    for key,(address,size)in REGIONS.items():
     raw=bytes.fromhex(before[key]);assert len(raw)==size;data(address,raw)
    for address,key,size in [(0xf598,'source_a',4),(0xf59c,'source_b',4),(0xf6e7,'scroll_mode',1),(0xff18,'seed',4),(0xf550,'pause',1),(0xf55a,'slow',1),(0xf566,'shake',2),(0xf572,'pan',2)]:word(address,before[key],size)
    for reg in range(15):setreg(reg,0)
    setreg(17,0x2700);setreg(15,0xffff00);word(0xff00,0x3d8,4);setreg(13,0xff8180);setreg(16,0x20918 if row['kind']=='arena'else 0x21556)
    origin,refresh=[(0,0),(1,896),(895,896),(1008,1008)][index%4];ref.cpu.cycles,ref.cpu.refresh_cycles=origin,refresh
    steps=[];portable=[]
    for _ in range(10000):
     pc=ref.get_register(16)&0xffffff
     if pc==0x3d8:break
     assert 0x20918<=pc<=0x20ade or 0x21556<=pc<=0x215f8 or 0x21690<=pc<=0x216ee,hex(pc)
     cycle,deadline=ref.cpu.cycles,ref.cpu.refresh_cycles;run(cycle+1)
     steps.append([pc,cycle,deadline,ref.cpu.cycles,ref.cpu.refresh_cycles]);portable.append([ref.cpu.cycles,ref.cpu.refresh_cycles])
    else:raise AssertionError(('step cap',index))
    actual=ref.ram_cpu();after={k:actual[a:a+n].hex()for k,(a,n)in REGIONS.items()}
    for k in REGIONS:assert after[k]==row['after'][k],(name,index,k)
    for a,k,n in [(0xf598,'source_a',4),(0xf59c,'source_b',4),(0xf6e7,'scroll_mode',1),(0xff18,'seed',4),(0xf550,'pause',1),(0xf55a,'slow',1),(0xf566,'shake',2),(0xf572,'pan',2)]:assert int.from_bytes(actual[a:a+n],'big')==row['after'][k]&((1<<(8*n))-1),(index,k)
    record={'name':row['name'],'kind':row['kind'],'before':before,'after':row['after'],'entry':[origin,refresh],'steps':portable}
    if name=='original':records.append(record);native_steps.append(steps)
    else:assert record==records[index] and steps==native_steps[index],('cross-core',index)
   print(json.dumps({'core':name,'cases':len(rows),'steps':sum(len(r['steps'])for r in records)}),flush=True)
 raw=(json.dumps({'version':1,'library':payload['library'],'cases':records},separators=(',',':'))+'\n').encode();(out/'native-arena-work.json.gz').write_bytes(gzip.compress(raw,mtime=0))
 (out/'source-steps.json.gz').write_bytes(gzip.compress(json.dumps(native_steps,separators=(',',':')).encode(),mtime=0))
 report={'pass':True,'cases':len(records),'steps':sum(map(len,native_steps)),'cores':identities,'original_outcomes_unchanged':True,'cross_core_steps_equal':True,'fixture_sha256':sha((out/'native-arena-work.json.gz').read_bytes()),'source_steps_sha256':sha((out/'source-steps.json.gz').read_bytes()),'inputs':{str(source.relative_to(ROOT)):sha(source.read_bytes()),str(Path(__file__).relative_to(ROOT)):sha(Path(__file__).read_bytes())},'scope':'Given typed callback entry state and clock. IRQ masked for direct complete callback costs. Does not prove whole-world entry clock, VDP/IRQ acceptance or sound.'};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
