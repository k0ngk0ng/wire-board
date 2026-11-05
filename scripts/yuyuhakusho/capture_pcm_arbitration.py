"""Private real PCM arbitration boundaries; no driver/ROM instruction edits."""
from pathlib import Path
import argparse,ctypes as C,gzip,itertools,json
from extract_audio_assets import AudioSession,write_cpu
from research_audio_loops import quiet_checkpoint
from reference import ROOT,sha

OBSERVED=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
BRIDGE=ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'

def delta(a,b):
 result=[];i=0
 while i<len(a):
  if a[i]==b[i]:i+=1;continue
  start=i
  while i<len(a)and a[i]!=b[i]:i+=1
  result.append([start,a[start:i].hex(),b[start:i].hex()])
 return result

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();assert sha(rom)=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
 bridge=C.CDLL(str(BRIDGE));cbtype=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
 bridge.yyh_set_observer.argtypes=[cbtype,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
 points_list=[0x37da4,0x37e9e,0x37dd8,0x37f1e,0x37fce,0x37f48,0x37ff8,0x3d8];points=(C.c_uint*len(points_list))(*points_list)
 calls=[];errors=[];current=None;finished=None
 with AudioSession(ROOT,out/'private-core',core=OBSERVED)as ref:
  checkpoint=quiet_checkpoint(ref);z=(C.c_ubyte*0x2000).in_dll(ref.lib,'zram')
  setreg=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint);run=ref.api('m68k_run',None,C.c_uint);hook=ref.api('set_cpu_hook',None,C.c_void_p)
  @cbtype
  def observe(kind,width,rawpc,value):
   nonlocal finished
   try:
    pc=rawpc&0xffffff
    if pc in(0x37da4,0x37e9e):
     ram=ref.ram_cpu();regs=[ref.get_register(i)&255 for i in range(6)]
     assert current['view']is None,'multiple arbitration reads'
     current['view']={'ModeFlags':regs[1],'SlotFlags':regs[2:4],'SlotBusy':regs[4:6]if pc==0x37e9e else [z[0x1f86],z[0x1fa6]],'ChoiceBits':ram[0xf82c],'DuckMode':ram[0xf82b]}
     current['read_pc']=hex(pc)
    elif pc in(0x37dd8,0x37f1e,0x37fce):
     assert current['destination']==0,'multiple PCM accepted destinations'
     current['destination']={0x37dd8:1,0x37f1e:2,0x37fce:3}[pc]
    elif pc in(0x37f48,0x37ff8):current['start_dual']=True
    elif pc==0x3d8:finished=ref.ram_cpu();current['exit_sp']=ref.get_register(15);ref.cpu.cycles=4000000
   except Exception as error:errors.append(repr(error));ref.cpu.cycles=4000000
  assert bridge.yyh_set_observer(observe,points,len(points_list))
  def call(name,code,mode,flags,busy,choice,duck,restore=True):
   nonlocal current,finished
   if restore:ref.restore(checkpoint,restore_refresh_cycles=True)
   address=0x380a8+((code-6)if code!=0xf0 else 122)*8
   for i in range(15):setreg(i,(len(calls)*257+i*19)&0xffffffff)
   setreg(8,address);setreg(17,0x2700);setreg(15,0xffff00);write_cpu(ref,0xffff00,0x3d8,4);setreg(16,0x37d66)
   z[0x1ffc]=mode;z[0x1f87],z[0x1fa7]=flags;z[0x1f86],z[0x1fa6]=busy;z[0x1ffd]=z[0x1ffe]=0
   write_cpu(ref,0xfff82c,choice);write_cpu(ref,0xfff82b,duck)
   ref.cpu.cycles=0;ref.cpu.refresh_cycles=896;before=ref.ram_cpu();finished=None
   current={'name':name,'code':code,'descriptor':rom[address:address+8].hex(),'command':{'Flags':rom[address+5],'RoutingOrPan':rom[address+6]},'view':None,'destination':0,'start_dual':False}
   hook(C.cast(bridge.yyh_cpu_hook,C.c_void_p));run(400000);hook(None)
   assert finished is not None and not errors,(name,errors)
   assert current['exit_sp']==0xffff04,('return stack',name,current['exit_sp'])
   assert current['view']is not None,('missing original arbitration read',name)
   after=finished;changes=delta(before,after)
   changed={address+i for address,old,new in changes for i in range(len(bytes.fromhex(old)))}
   allowed={0xf82b,0xf82c}|set(range(0xfee0,0xff00));unknown=sorted(changed-allowed);assert not unknown,(name,'unexpected68kRAM',unknown)
   current['expected']={'Destination':current.pop('destination'),'ChoiceBits':after[0xf82c],'StartDual':current.pop('start_dual'),'DuckMode':after[0xf82b]}
   current.update(ram_bytes=65536,ram_before_sha256=sha(before),ram_after_sha256=sha(after),delta=changes,unknown_writes=unknown)
   calls.append(current)
  # Complete real descriptor set in four modes; the additionalF0 descriptor is separate.
  codes=list(range(6,128))+[0xf0]
  for code,mode in itertools.product(codes,[0,1,0x80,0xc1]):call(f'all-{code:02x}-{mode:02x}',code,mode,[0x41,0xc1],[0x40,0x80],0xaa,0)
  all_count=len(calls)
  for index,(code,mode,flags,busy,choice,duck)in enumerate(itertools.product([6,0x1f,0x0c,0x73,0x79,0x7a,0x7f,0xf0],[0,1,0x80,0x81,0xc0,0xc1,0x20,0x21],[[0,0],[0x41,0x83],[0xc1,0x41],[0xc1,0xc1]],[[0,0],[0x40,0],[0,0x80],[255,255]],[0,1,0xaa,255],[0,2,255])):
   call(f'boundary-{index}',code,mode,flags,busy,choice,duck)
  boundary_count=len(calls)-all_count
  # Explicit continuous branches retain current choice andduck writes. Playback
  # flags are external acknowledgements and supplied anew as temporary views.
  for stream in range(8):
   mode=1;choice=stream*32;duck=0
   for step in range(32):
    code=[0x0c,0x0d,0x1f,0x3f,0x79,0x7a,0x73,0x7f][(step+stream)%8]
    call(f'stream-{stream}-{step}',code,mode,[0xc1 if step%3 else 0,0x41 if step%5 else 0],[0x40 if step%2 else 0,0x80 if step%3 else 0],choice,duck,restore=step==0)
    choice,duck=calls[-1]['expected']['ChoiceBits'],calls[-1]['expected']['DuckMode'];mode=[0,1,0xc1,0x20][step%4]
  identity=ref.identity
 payload={'schema':1,'all_descriptor_calls':all_count,'boundary_calls':boundary_count,'continuous_calls':len(calls)-all_count-boundary_count,'calls':calls}
 raw=(json.dumps(payload,separators=(',',':'))+'\n').encode();path=out/'pcm-arbitration-private.json.gz';path.write_bytes(gzip.compress(raw,mtime=0))
 report={'pass':True,'identity':identity,'scope':'Private real37d66 descriptor branches, no synthesized playback or natural reachability claim','calls':len(calls),'all_descriptor_calls':all_count,'boundary_calls':boundary_count,'continuous_calls':payload['continuous_calls'],'unknown_writes':0,'fixture_path':str(path.relative_to(ROOT)),'fixture_json_sha256':sha(raw),'fixture_gzip_sha256':sha(path.read_bytes()),'producer_sha256':sha(Path(__file__).read_bytes()),'raw_decision_read_pcs':['0x37da4','0x37e9e'],'accepted_destination_pcs':['0x37dd8','0x37f1e','0x37fce'],'start_dual_pcs':['0x37f48','0x37ff8'],'limits':['Busy/flags are declared private raw inputs; acceptance intent does not assertZ80 has acknowledged or played the sample','Whole64KiB RAM delta audited; autonomousZ80 memory/hardware are not reconstructed by the proposed helper','ChoiceBits andDuckMode retained across explicit continuous branches; other playback inputs remain caller-owned views']}
 (out/'pcm-arbitration-private-manifest.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
