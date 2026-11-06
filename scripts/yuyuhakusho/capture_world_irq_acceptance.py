"""Observe normal IRQ acceptance at an execution-batch boundary, two cores."""
import argparse,ctypes as C,gzip,json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
from research_audio_loops import ORIGINAL,OBSERVED

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
 inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text());records=[];arena_records=[];identities={}
 for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  with ReferenceSession(ROOT,out/name,core=core)as ref:
   for frame in range(1,2401):assert ref.step(frame,buttons_at(inputs,frame))['reference']==expected[frame-1]
   checkpoint=ref.save();identities[name]=ref.identity;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);setirq=ref.api('m68k_set_irq',None,C.c_uint);run=ref.api('m68k_run',None,C.c_uint);rom=ref.rom_path.read_bytes();assert rom[0x38c0c:0x38c0e]==bytes.fromhex('4e71')
   for mask in range(8):
    for level in [0,4,6]:
     for origin in range(70):
      ref.restore(checkpoint,restore_refresh_cycles=True);setirq(0);setreg(17,0x2000|mask<<8);setreg(15,0xfffe00);setreg(16,0x38c0c)
      ref.cpu.cycles,ref.cpu.refresh_cycles=origin,896;setirq(level);run(origin+1);pc=ref.get_register(16)&0xffffff;status=ref.get_register(17);accepted=pc!=0x38c0e
      if accepted:
       assert level>mask and pc==int.from_bytes(rom[(24+level)*4:(25+level)*4],'big')&0xffffff,(mask,level,origin,hex(pc))
       raw=ref.ram_cpu();sp=ref.get_register(15)&65535;assert int.from_bytes(raw[sp:sp+2],'big')==0x2000|mask<<8;assert int.from_bytes(raw[sp+2:sp+6],'big')==0x38c0c
      else:assert level<=mask and ref.get_register(15)==0xfffe00
      record={'mask':mask,'level':level,'origin':origin,'accepted':accepted,'entry_cycle':ref.cpu.cycles if accepted else origin,'mask_after':(status>>8)&7,'refresh':ref.cpu.refresh_cycles,'native_after_cycle':ref.cpu.cycles,'native_after_pc':pc}
      index=(mask*3+[0,4,6].index(level))*70+origin
      if name=='original':records.append(record)
      else:assert record==records[index],('cross core',record,records[index])
   arena_index=0
   def put(address,value,size=2):
    for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=b
   for stage in [0,2]:
    for pan in [0,1]:
     for used in [0,16]:
      for origin in [158000,159000,160000,160700]:
       ref.restore(checkpoint,restore_refresh_cycles=True);setirq(0)
       for reg in range(15):setreg(reg,0)
       for address,size in [(0x8180,128),(0xa00e,14),(0xa800,128),(0xe500,64),(0xe600,1536),(0x6000,896)]:
        for i in range(size):put(address+i,0,1)
       for address,value,size in [(0x8186,stage,2),(0xa00e,used,2),(0xa014,864,2),(0xa016,16,2),(0xf550,0,1),(0xf55a,0,1),(0xf566,0,2),(0xf572,pan,2)]:put(address,value,size)
       for i in range(4):put(0xa800+i*4,900<<16,4);put(0xa840+i*4,320<<16,4)
       setreg(17,0x2000);setreg(15,0xfffe00);setreg(13,0xff8180);setreg(16,0x20918);ref.cpu.cycles,ref.cpu.refresh_cycles=origin,origin+896
       deadline=47*3420;run(deadline);before_irq=ref.cpu.cycles;interrupted=ref.get_register(16)&0xffffff;assert before_irq>=deadline and interrupted!=0x3d8
       setirq(4);run(before_irq+1);entry=ref.cpu.cycles;status=ref.get_register(17);assert status&0x700==0x400
       record={'stage':stage,'pan':pan,'used':used,'origin':origin,'refresh':origin+896,'deadline':deadline,'before_irq':before_irq,'entry_cycle':entry,'refresh_after':ref.cpu.refresh_cycles,'native_interrupted_pc':interrupted,'native_handler_pc':ref.get_register(16)&0xffffff}
       if name=='original':arena_records.append(record)
       else:assert record==arena_records[arena_index],('arena cross core',record,arena_records[arena_index])
       arena_index+=1
   print(json.dumps({'core':name,'profiles':len(records),'arena_deadline_profiles':arena_index}),flush=True)
 raw=(json.dumps({'version':1,'cases':records,'arena_cases':arena_records},separators=(',',':'))+'\n').encode();fixture=out/'native-irq-acceptance.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'profiles':len(records),'accepted':sum(r['accepted']for r in records),'arena_deadline_profiles':len(arena_records),'cross_core_equal':True,'stack_frame_verified':True,'cores':identities,'fixture_sha256':sha(fixture.read_bytes()),'capture_sha256':sha(Path(__file__).read_bytes()),'scope':'Immediate normal level 4/6 pin state at m68k_run entry, plus actual default-arena execution crossing an externally supplied first-HINT deadline. No deferred register-write IRQ, mask-changing instruction, independently inferred whole-world signal schedule or original CPU in production.'};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
