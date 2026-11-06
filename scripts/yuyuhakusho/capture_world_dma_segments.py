"""Two pinned cores: actual forced-blank H40 DMA segment effects, not a formula oracle."""
import argparse,ctypes as C,gzip,json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
from research_audio_loops import ORIGINAL,OBSERVED
from capture_pcm_transfer_gate import command
class Viewport(C.Structure):_fields_=[(n,C.c_int)for n in ['x','y','w','h','ow','oh','changed']]
class Bitmap(C.Structure):_fields_=[('data',C.c_void_p),('width',C.c_int),('height',C.c_int),('pitch',C.c_int),('viewport',Viewport)]
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
 inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());baseline=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text());rows=[];identities={}
 for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  with ReferenceSession(ROOT,out/name,core=core)as ref:
   for frame in range(1,2401):assert ref.step(frame,buttons_at(inputs,frame))['reference']==baseline[frame-1]
   bitmap=Bitmap.in_dll(ref.lib,'bitmap');assert bitmap.viewport.w==320 and bitmap.viewport.h==224;reg=(C.c_uint8*32).in_dll(ref.lib,'reg');z=command.Z80.in_dll(ref.lib,'Z80');state=C.c_uint8.in_dll(ref.lib,'zstate');variables={n:t.in_dll(ref.lib,n)for n,t in [('dma_length',C.c_uint32),('dma_endCycles',C.c_uint32),('dma_type',C.c_uint8),('status',C.c_uint16),('mcycles_vdp',C.c_uint32)]};update=ref.api('vdp_dma_update',None,C.c_uint);reset=ref.api('vdp_reset',None);control=ref.api('vdp_68k_ctrl_w',None,C.c_uint);index=0
   for target,mode,code in [('VRAM',1,1),('CRAM',0,3),('VSRAM',0,5)]:
    for blank in [False,True]:
     for line in ([0,1,36]if blank else[38,100,261]):
      for offset in [0,1,14,100,1709,3300,3419,3420,3421,3433]:
       at=line*3420+offset;boundary=37*3420 if blank else(line+1)*3420;rate=102 if mode else 198;numerator=(boundary-at)*rate;available=numerator//3420 if numerator>=0 else-((-numerator)//3420)
       for length in sorted({1,32,224,560,65536,max(1,available-1),max(1,available),available+1}):
        for waiting in [False,True]:
         reset();bitmap.viewport.w=bitmap.viewport.ow=320;bitmap.viewport.h=bitmap.viewport.oh=224;control(0x8124);control(0x8c81);control(0xc000 if target=='CRAM'else 0x4000);control(0x10 if target=='VSRAM'else 0);reg[1]=0x34;reg[12]=1;reg[15]=2;reg[19]=length&255;reg[20]=(length>>8)&255;reg[21]=reg[22]=0;reg[23]=0x7f
         for key,value in {'dma_length':length,'dma_endCycles':0,'dma_type':mode,'status':8 if blank else 0,'mcycles_vdp':line*3420}.items():variables[key].value=value
         state.value=5 if waiting else 1;ref.cpu.cycles=at+17;z.cycles=max(0,at-500)
         inp={'Target':target,'Remaining':length,'At':at,'Line':line,'VBlank':blank,'AuxWaiting':waiting,'MainBefore':ref.cpu.cycles,'AuxBefore':z.cycles}
         update(at)
         row={'Input':inp,'End':variables['dma_endCycles'].value,'Remaining':variables['dma_length'].value,'MainAfter':ref.cpu.cycles,'AuxAfter':z.cycles,'AuxWaitingAfter':bool(state.value&4),'NativeMode':variables['dma_type'].value}
         if name=='original':rows.append(row)
         else:assert row==rows[index],('cross-core DMA differs',index,row,rows[index])
         index+=1
   identities[name]=ref.identity;print(json.dumps({'core':name,'cases':index}),flush=True)
 raw=(json.dumps({'version':1,'cases':rows},separators=(',',':'))+'\n').encode();fixture=out/'native-dma-segments.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(rows),'cross_core_equal':True,'all_2400_video_ram_controls_equal_per_core':True,'identity':identities,'fixture_sha256':sha(fixture.read_bytes()),'source_sha256':sha(Path(__file__).read_bytes()),'declared_viewport':[320,224],'viewport_after_reset_explicitly_restored':True,'scope':'Actual exported vdp_dma_update called with declared forced-blank H40 RAM source/VRAM,CRAM,VSRAM destination profiles. Original helper executes all transfers; no main CPU program timing or whole World claim.'};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
