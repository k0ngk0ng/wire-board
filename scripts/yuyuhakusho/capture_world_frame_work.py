"""Actual world video/pause/logic/RNG/feedback work quanta, two fixed cores."""
import argparse,ctypes as C,gzip,json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
from research_audio_loops import ORIGINAL,OBSERVED
from capture_world_clock import FIELDS,snapshot
ENTRIES={'video':0x3a76,'logic':0x3a86,'toggle':0x3a9c,'pause':0x3ad6,'shake':0x39ac,'random':0x3982}
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
 source=ROOT/'games/yuyuhakusho/engine/testdata/world-clock-boundaries.json.gz';original=json.loads(gzip.decompress(source.read_bytes()));rows=[{'kind':r['kind'],'name':r['name'],'before':r['before']['state'],'after':r['after']['state']}for r in original]
 for seed in [0,1,0xffff,0xffff0000,0xffffffff,0x2a6d365a,0x5eedbeef,0x80000000]:
  for phase in range(4):rows.append({'kind':'random','name':f'random-{seed:x}-{phase}','before':original[0]['before']['state'],'seed':seed})
 inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());gold=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text());records=[];native=[];identities={}
 for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  with ReferenceSession(ROOT,out/name,core=core)as ref:
   for frame in range(1,2401):assert ref.step(frame,buttons_at(inputs,frame))['reference']==gold[frame-1]
   checkpoint=ref.save();identities[name]=ref.identity;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);run=ref.api('m68k_run',None,C.c_uint)
   def write(address,value,size=1):
    for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=b
   for index,row in enumerate(rows):
    ref.restore(checkpoint,restore_refresh_cycles=True)
    for field,(off,size)in FIELDS.items():write(off,row['before'][field],size)
    for i,value in enumerate(row['before']['pressed']):write(0xf798+i,value)
    if row['kind']=='random':write(0xff18,row['seed'],4)
    for reg in range(15):setreg(reg,0)
    setreg(17,0x2700);setreg(15,0xffff00);write(0xff00,0x3d8,4);setreg(16,ENTRIES[row['kind']]);origin,refresh=[(0,0),(1,896),(895,896),(1008,1008)][index%4];ref.cpu.cycles,ref.cpu.refresh_cycles=origin,refresh;before=ref.ram_cpu();steps=[];portable=[]
    for _ in range(2000):
     pc=ref.get_register(16)&0xffffff
     if pc==0x3d8:break
     assert 0x3982<=pc<=0x3afc,(index,hex(pc))
     clock,deadline=ref.cpu.cycles,ref.cpu.refresh_cycles;run(clock+1);steps.append([pc,clock,deadline,ref.cpu.cycles,ref.cpu.refresh_cycles]);portable.append([ref.cpu.cycles,ref.cpu.refresh_cycles])
    else:raise AssertionError(('world frame helper step cap',index))
    ram=ref.ram_cpu();after=snapshot(ram)['state']
    if row['kind']!='random':assert after==row['after'],('original frozen helper output changed',name,index)
    allowed=set(range(0xf550,0xf590))|{0xf807}|set(range(0xfed0,0xff04))|set(range(0xff18,0xff1c));assert not[i for i,(x,y)in enumerate(zip(before,ram))if x!=y and i not in allowed],('unexpected helper write',index)
    record={'Name':row['name'],'Kind':row['kind'],'Before':row['before'],'After':after,'Seed':row.get('seed',0),'NextSeed':int.from_bytes(ram[0xff18:0xff1c],'big')if row['kind']=='random'else 0,'RandomResult':ref.get_register(0)if row['kind']=='random'else 0,'Clock':{'cycle':origin,'refresh':refresh},'Steps':[{'cycle':c,'refresh':f}for c,f in portable]}
    if name=='original':records.append(record);native.append(steps)
    else:assert record==records[index]and steps==native[index],('cross-core world frame work differs',index)
   print(json.dumps({'core':name,'cases':len(records),'steps':sum(len(r['Steps'])for r in records)}),flush=True)
 raw=(json.dumps({'Version':1,'Cases':records},separators=(',',':'))+'\n').encode();fixture=out/'native-frame-work.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));srcsteps=out/'source-steps.json.gz';srcsteps.write_bytes(gzip.compress((json.dumps(native,separators=(',',':'))+'\n').encode(),mtime=0));report={'pass':True,'cases':len(records),'steps':sum(map(len,native)),'all_2400_video_ram_controls_equal_per_core':True,'cross_core_steps_and_outcomes_equal':True,'frozen_helper_outcomes_unchanged':True,'identity':identities,'fixture_sha256':sha(fixture.read_bytes()),'source_steps_sha256':sha(srcsteps.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [source,Path(__file__)]},'scope':'Actual original helper execution under mask7 with declared data and entry clock. Every work boundary and whole result observed; full-world entry/IRQ schedule belongs to joint caller.'};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
