"""Native complete sample suffix / RTE / immediately following reload IRQ."""
import argparse,ctypes as C,gzip,json,os,subprocess
from pathlib import Path
from reference import ROOT,ReferenceSession,CPUHead,buttons_at,sha
from research_audio_loops import ORIGINAL,OBSERVED
from capture_battle_sample_prefix import CPUControl,FIELDS,Event

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True);tmp=out/'tmp';tmp.mkdir();source=ROOT/'scripts/yuyuhakusho/battle-return-observer.c';binary=tmp/'return.dylib';subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(source),'-o',str(binary)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
 b=C.CDLL(str(binary));b.battle_return_io_attach.argtypes=[C.c_void_p];b.battle_return_io_attach.restype=C.c_int;b.battle_return_io_detach.restype=C.c_int;b.battle_prefix_attach.argtypes=[C.c_void_p]*14;b.battle_prefix_attach.restype=C.c_int;b.battle_prefix_events.restype=C.POINTER(Event);b.battle_prefix_count.restype=C.c_uint;b.battle_prefix_complete.restype=C.c_uint;b.battle_return_start.argtypes=[C.c_uint,C.c_uint];b.battle_return_cpu.argtypes=[C.c_uint];b.battle_return_end.argtypes=[C.c_uint];b.battle_return_pending.argtypes=[C.c_uint];b.battle_return_ack_attach.argtypes=[C.c_void_p];b.battle_return_ack_attach.restype=C.c_int;b.battle_return_ack_pointer.restype=C.c_void_p;b.battle_return_ack_detach.restype=C.c_int
 inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());golds=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text());profiles=[]
 for mask in [0,3,4,7]:
  for pin in [0,4]:
   for counter in [0,1,9]:
    for offset in list(range(70))+[895,896,2350,2400,2412,2450,2700,3300]:profiles.append({'mask':mask,'pin':pin,'counter':counter,'offset':offset})
 records=[];identities={};off_on=0
 for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  with ReferenceSession(ROOT,out/name,core=core)as ref:
   for f in range(1,2401):assert ref.step(f,buttons_at(inputs,f))['reference']==golds[f-1]
   checkpoint=ref.save();identities[name]=ref.identity;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);run=ref.api('m68k_run',None,C.c_uint);setirq=ref.api('m68k_set_irq',None,C.c_uint);updateirq=ref.api('m68k_update_irq',None,C.c_uint);ack=ref.api('vdp_68k_irq_ack',C.c_int,C.c_int);native=CPUControl.in_dll(ref.lib,'m68k')
   props={n:t.in_dll(ref.lib,n)for n,t in [('reg',C.c_uint8*32),('mcycles_vdp',C.c_uint),('v_counter',C.c_ushort),('h_counter',C.c_ubyte),('hint_pending',C.c_ubyte),('vint_pending',C.c_ubyte),('input',C.c_ubyte)]}
   ptr=lambda n:C.addressof(props[n]);assert b.battle_prefix_attach(ref.ram_pointer,ptr('reg'),C.addressof(ref.cpu)+CPUHead.cycles.offset,C.addressof(ref.cpu)+CPUHead.refresh_cycles.offset,ptr('mcycles_vdp'),ptr('v_counter'),ptr('h_counter'),ptr('hint_pending'),ptr('vint_pending'),C.addressof(ref.cpu)+CPUHead.cycle_end.offset,ptr('input'),C.cast(ref.lib.m68k_get_reg,C.c_void_p),C.addressof(ref.cpu)+CPUControl.pin.offset,C.addressof(ref.cpu)+CPUControl.mask.offset)
   def write(address,value,size=1):
    for i,x in enumerate((value&((1<<(8*size))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=x
   def events():
    array=b.battle_prefix_events();return [[getattr(array[i],n)for n in FIELDS]for i in range(b.battle_prefix_count())]
   for index,profile in enumerate(profiles):
    outcomes=[];observed_events=None
    modes=[False,True]if index%32==0 else[True]
    for observed in modes:
     ref.restore(checkpoint,restore_refresh_cycles=True);setirq(0);setreg(17,0x2700);setreg(15,0xfffe00);setreg(16,0xfff096)
     values=[(0x12340000+i*0x12345+index)&0xffffffff for i in range(14)]
     for i,value in enumerate(values):write(0xfe00+i*4,value,4)
     write(0xfe38,0x2000|(profile['mask']<<8)|0x15,2);write(0xfe3a,0x3d8,4);write(0xf516,2,2)
     props['reg'][0]=0x14;props['reg'][1]=0x64;props['reg'][10]=184;props['v_counter'].value=9;props['mcycles_vdp'].value=47*3420;props['h_counter'].value=profile['counter'];props['hint_pending'].value=16 if profile['pin']else 0;props['vint_pending'].value=0
     origin=47*3420+profile['offset'];ref.cpu.cycles,ref.cpu.refresh_cycles=origin,origin+896;setirq(profile['pin']);b.battle_return_begin()
     if observed:
      assert b.battle_return_io_attach(C.addressof(ref.cpu));b.battle_return_start(0x3d8,0xfffe3e)
     pending_world=False
     try:
      for _ in range(64):
       while ref.cpu.cycles>=props['mcycles_vdp'].value+3420:
        props['mcycles_vdp'].value+=3420;props['v_counter'].value+=1
        if props['h_counter'].value==0:props['h_counter'].value=props['reg'][10];props['hint_pending'].value=16;updateirq(4)
        else:props['h_counter'].value-=1
       pc=ref.get_register(16)&0xffffff
       if pc==0xfff000 and int.from_bytes(ref.ram_cpu()[0xf516:0xf518],'big')>=4:
        pending_world=True
        if observed:b.battle_return_pending(pc)
        break
       if pc==0x3d8 and native.pin<=native.mask:
        if observed:b.battle_return_end(pc)
        break
       before=ref.cpu.cycles
       if native.pin>native.mask:run(before+1);continue
       if observed:b.battle_return_cpu(pc)
       assert pc in [0xfff096,0xfff09a,0xfff000,0xfff004,0xfff00a,0xfff09c,0xfff0a4,0xfff0a8],hex(pc)
       run(before+1)
      else:raise AssertionError(('suffix cap',profile))
      assert ref.get_register(15)==(0xfffe38 if pending_world else 0xfffe3e) and [ref.get_register(i)for i in range(14)]==values,('restore',profile)
      outcomes.append([ref.cpu.cycles,ref.cpu.refresh_cycles,ref.get_register(17),ref.ram_cpu()[0xf516:0xf518].hex(),props['mcycles_vdp'].value,props['v_counter'].value,props['h_counter'].value,props['reg'][10],props['hint_pending'].value,native.pin,native.mask,pending_world])
      if observed:assert b.battle_prefix_complete();observed_events=events()
     finally:
      if observed:assert b.battle_return_io_detach()
    if len(outcomes)==2:assert outcomes[0]==outcomes[1],('observer changed suffix',profile);off_on+=1
    record={'input':profile,'events':observed_events,'after':outcomes[-1]}
    if name=='original':records.append(record)
    else:assert record==records[index],('cross-core return',index)
   print(json.dumps({'core':name,'profiles':len(profiles),'passive_checks_total':off_on}),flush=True)
 raw=(json.dumps({'version':1,'fields':FIELDS,'cases':records},separators=(',',':'))+'\n').encode();fixture=out/'native-battle-return.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(profiles),'cross_core_equal':True,'observer_off_on_cases':off_on,'registers_and_original_return_stack_verified':True,'background_feedback':'Declared zero, direct suffix CPU execution plus explicit active-line external VDP advances; no PCM or DMA progress','cores':identities,'fixture_sha256':sha(fixture.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),source,ROOT/'scripts/yuyuhakusho/battle-prefix-observer.c']}};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
