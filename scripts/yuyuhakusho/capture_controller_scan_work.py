"""Original complete two-port MD6/TeamPlayer scanner, passive IO and cross-core."""
import argparse,ctypes as C,gzip,json,os,subprocess
from pathlib import Path
from reference import ROOT,ReferenceSession,CPUHead,buttons_at,sha
from research_audio_loops import ORIGINAL,OBSERVED

class NativeInput(C.Structure):
 _fields_=[('system',C.c_uint8*2),('dev',C.c_uint8*8),('pad',C.c_uint16*8),('analog',C.c_int16*16),('x',C.c_int),('y',C.c_int)]
class Event(C.Structure):
 _fields_=[(n,C.c_uint32)for n in ['pc','at','after','address','value','write']]

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True);tmp=out/'tmp';tmp.mkdir()
 binary=tmp/'controller-io.dylib';source=ROOT/'scripts/yuyuhakusho/controller-scan-io.c';subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(source),'-o',str(binary)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
 observer=C.CDLL(str(binary));observer.controller_io_attach.argtypes=[C.c_void_p,C.c_void_p];observer.controller_io_attach.restype=C.c_int;observer.controller_io_detach.restype=C.c_int;observer.controller_io_pc.argtypes=[C.c_uint];observer.controller_io_events.restype=C.POINTER(Event);observer.controller_io_count.restype=C.c_uint;observer.controller_io_overflow.restype=C.c_uint
 assert C.sizeof(NativeInput)==68 and NativeInput.pad.offset==10
 directions=[0,1,2,4,8,5,6,9,10];actions=[0,0x10,0x20,0x40,0x80,0x800,0x700,0xff0];profiles=[]
 for ports in [(1,1),(4,1),(1,4),(4,4)]:
  for d,dirbits in enumerate(directions):
   for j,action in enumerate(actions):
    held=[]
    for port,count in enumerate(ports):held.append([directions[(d+i+port*3)%9]|actions[(j+i+port)%8]if i<count else 0 for i in range(4)])
    profiles.append({'ports':list(ports),'held':held})
 records=[];source_steps=[];identities={};inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());expected=json.loads((ROOT/'docs/research/md-yuyuhakusho/frame-hashes.json').read_text())
 for name,core in [('original',ORIGINAL),('observed',OBSERVED)]:
  with ReferenceSession(ROOT,out/name,core=core)as ref:
   for f in range(1,2401):assert ref.step(f,buttons_at(inputs,f))['reference']==expected[f-1]
   checkpoint=ref.save();identities[name]=ref.identity;setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);run=ref.api('m68k_run',None,C.c_uint);setirq=ref.api('m68k_set_irq',None,C.c_uint);device=ref.api('retro_set_controller_port_device',None,C.c_uint,C.c_uint);reset=ref.api('io_reset',None);io_write=ref.api('io_68k_write',None,C.c_uint,C.c_uint);native=NativeInput.in_dll(ref.lib,'input')
   def write(address,value,size=1):
    for i,b in enumerate((value&((1<<(size*8))-1)).to_bytes(size,'big')):C.c_ubyte.from_address(ref.ram_pointer+(((address+i)&65535)^1)).value=b
   for index,profile in enumerate(profiles):
    executions=[];io=[]
    for observed in [False,True]:
     ref.restore(checkpoint,restore_refresh_cycles=True);setirq(0)
     for port,count in enumerate(profile['ports']):device(port,513 if count==1 else 1793)
     reset();assert list(native.system)==[1 if n==1 else 13 for n in profile['ports']]
     # Previous complete scans leave TH high. Recreate that settled port
     # baseline rather than the distinct cold-IO-reset first-scan profile.
     for port,count in enumerate(profile['ports']):
      value=0x40 if count==1 else 0x60
      io_write(1+port,value);io_write(4+port,value)
     for port,count in enumerate(profile['ports']):
      for i in range(4):
       if i<count:assert native.dev[port*4+i]==1,(index,port,i,list(native.dev))
       native.pad[port*4+i]=profile['held'][port][i]
     for i in range(80):write(0xf700+i,0xa5)
     for reg in range(15):setreg(reg,0)
     setreg(17,0x2700);setreg(15,0xffff00);write(0xff00,0x3d8,4);setreg(16,0x2514)
     origin,refresh=[(0,0),(1,896),(895,896),(1008,1008)][index%4];ref.cpu.cycles,ref.cpu.refresh_cycles=origin,refresh;before=ref.ram_cpu();steps=[];portable=[]
     if observed:assert observer.controller_io_attach(C.addressof(ref.cpu),C.addressof(ref.cpu)+CPUHead.cycles.offset)
     try:
      for _ in range(20000):
       pc=ref.get_register(16)&0xffffff
       if pc==0x3d8:break
       assert 0x2514<=pc<=0x2c60,hex(pc)
       cycle,deadline=ref.cpu.cycles,ref.cpu.refresh_cycles
       if observed:observer.controller_io_pc(pc)
       run(cycle+1);steps.append([pc,cycle,deadline,ref.cpu.cycles,ref.cpu.refresh_cycles]);portable.append([ref.cpu.cycles,ref.cpu.refresh_cycles])
      else:raise AssertionError(('step cap',index))
     finally:
      if observed:assert observer.controller_io_detach()
     after=ref.ram_cpu();unknown=[i for i,(a,b)in enumerate(zip(before,after))if a!=b and not(0xf700<=i<0xf750 or 0xfec0<=i<0xff04)];assert not unknown,(index,unknown)
     executions.append((after[0xf700:0xf750].hex(),steps,portable,sha(after)))
     if observed:
      assert not observer.controller_io_overflow();events=observer.controller_io_events()
      for i in range(observer.controller_io_count()):
       e=events[i];assert e.at==e.after,('unexpected IO wait',index,e.at,e.after)
       offset=e.address&15;assert offset in [3,5,9,11],hex(e.address)
       io.append({'port':0 if offset in[3,9]else 1,'control':offset>=9,'write':bool(e.write),'value':e.value,'at':e.at})
    assert executions[0]==executions[1],('observer changed execution',index)
    record={'input':profile,'entry':[origin,refresh],'after':executions[0][0],'steps':executions[0][2],'io':io}
    if name=='original':records.append(record);source_steps.append(executions[0][1])
    else:assert record==records[index]and executions[0][1]==source_steps[index],('cross core',index)
   print(json.dumps({'core':name,'cases':len(profiles),'steps':sum(map(len,source_steps))}),flush=True)
 raw=(json.dumps({'version':1,'cases':records},separators=(',',':'))+'\n').encode();fixture=out/'native-controller-scan.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));(out/'source-steps.json.gz').write_bytes(gzip.compress(json.dumps(source_steps,separators=(',',':')).encode(),mtime=0));report={'pass':True,'cases':len(profiles),'steps':sum(map(len,source_steps)),'cross_core_equal':True,'observer_on_off_equal':True,'no_unexpected_RAM_writes':True,'input_scope':'settled virtual MD6 devices, two ports; direct or four-MD6 TeamPlayer; physically valid D-pad directions','cores':identities,'fixture_sha256':sha(fixture.read_bytes()),'source_sha256':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),source]},'scope':'Complete original 2514 through real return. No CPU/ROM or trace playback in production.'};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
