"""Natural original RAM sample-IRQ prefix, including actual VDP batches and IO."""
import argparse,ctypes as C,gzip,json,os,subprocess
from pathlib import Path
from extract_audio_assets import AudioSession
from reference import ROOT,CPUHead,buttons_at,sha
CORE=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
FIELDS=['kind','pc','width','address','value','cycle','refresh','line','vertical','counter','reg0','reg1','reload','hpending','vpending','batch','phase','flags','control_word','saved_color','ports0','ports1','held0','held1','held2','held3','held4','held5','held6','held7','pin','mask']
class CPUControl(C.Structure):
 _fields_=CPUHead._fields_+[('prev_pc',C.c_uint),('prev_dr',C.c_uint*8),('prev_ar',C.c_uint*8),('sp',C.c_uint*5)]+[(n,C.c_uint)for n in ['ir','t1','supervisor','x','n','z','v','c','mask','pin']]
class Event(C.Structure):_fields_=[(n,C.c_uint32)for n in FIELDS]
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True);tmp=out/'tmp';tmp.mkdir();source=ROOT/'scripts/yuyuhakusho/battle-prefix-observer.c';binary=tmp/'prefix.dylib';subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(source),'-o',str(binary)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
 b=C.CDLL(str(binary));b.battle_prefix_attach.argtypes=[C.c_void_p]*14;b.battle_prefix_attach.restype=C.c_int;b.battle_prefix_count.restype=C.c_uint;b.battle_prefix_complete.restype=C.c_uint;b.battle_prefix_size.restype=C.c_uint;b.battle_prefix_events.restype=C.POINTER(Event);assert b.battle_prefix_size()==C.sizeof(Event)
 controls_path=ROOT/'games/yuyuhakusho/engine/testdata/audio-timing-baseline-original.json.gz';controls=json.loads(gzip.decompress(controls_path.read_bytes()));inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());selected=set(range(2068,2075))|set(range(2393,2401));rows=[]
 with AudioSession(ROOT,out/'reference',core=CORE)as ref:
  pointers=[]
  for n,t in [('reg',C.c_ubyte),('mcycles_vdp',C.c_uint),('v_counter',C.c_ushort),('h_counter',C.c_ubyte),('hint_pending',C.c_ubyte),('vint_pending',C.c_ubyte),('input',C.c_ubyte)]:pointers.append(t.in_dll(ref.lib,n))
  r,line,vc,h,hp,vp,inp=map(C.addressof,pointers);get=ref.api('m68k_get_reg',C.c_uint,C.c_uint)
  assert b.battle_prefix_attach(ref.ram_pointer,r,C.addressof(ref.cpu)+CPUHead.cycles.offset,C.addressof(ref.cpu)+CPUHead.refresh_cycles.offset,line,vc,h,hp,vp,C.addressof(ref.cpu)+CPUHead.cycle_end.offset,inp,C.cast(get,C.c_void_p),C.addressof(ref.cpu)+CPUControl.pin.offset,C.addressof(ref.cpu)+CPUControl.mask.offset)
  native_control=CPUControl.in_dll(ref.lib,'m68k');assert native_control.cycles==ref.cpu.cycles and native_control.pc==get(16) and native_control.mask==get(17)&0x700
  hook=ref.api('set_cpu_hook',None,C.c_void_p)
  for frame in range(1,2401):
   if frame in selected:b.battle_prefix_begin();hook(C.cast(b.battle_prefix_hook,C.c_void_p))
   try:result,pcm=ref.step_audio(frame,buttons_at(inputs,frame))
   finally:hook(None)
   expected=controls[frame-1];assert result['reference']==expected['reference']and len(pcm)//4==expected['frames']and sha(pcm)==expected['pcm_sha256'],('observer changed original frame',frame)
   if frame in selected:
    assert b.battle_prefix_complete(),('prefix not complete',frame,b.battle_prefix_count());events=b.battle_prefix_events();rows.append({'frame':frame,'events':[[getattr(events[i],n)for n in FIELDS]for i in range(b.battle_prefix_count())]})
  checkpoint=ref.save()
  device=ref.api('retro_set_controller_port_device',None,C.c_uint,C.c_uint);reset=ref.api('io_reset',None);io_write=ref.api('io_68k_write',None,C.c_uint,C.c_uint)
  def cpu_byte(address,value):C.c_ubyte.from_address(ref.ram_pointer+((address&65535)^1)).value=value
  mapping=[4,5,6,7,0,8,1,3,11,9,10,2]
  for topology in [(1,1),(4,1),(1,4),(4,4)]:
   for ready in [False,True]:
    controls=[];captured=None
    for observed in [False,True]:
     ref.restore(checkpoint,restore_refresh_cycles=True)
     for port,count in enumerate(topology):device(port,513 if count==1 else 1793)
     reset()
     for port,count in enumerate(topology):
      value=0x40 if count==1 else 0x60;io_write(1+port,value);io_write(4+port,value)
     cpu_byte(0xf500,(ref.ram_cpu()[0xf500]&127)|(128 if ready else 0));cpu_byte(0xf516,0);cpu_byte(0xf517,0)
     held=[[0x441,0x212,0x124,0x888],[0x822,0x241,0x418,0x184]];buttons=[]
     for port,count in enumerate(topology):
      for i in range(count):buttons.append(sum(1<<mapping[j]for j in range(12)if held[port][i]&(1<<j)))
     buttons+=([0]*(8-len(buttons)))
     if observed:b.battle_prefix_begin();hook(C.cast(b.battle_prefix_hook,C.c_void_p))
     try:result,pcm=ref.step_audio(2401,buttons)
     finally:hook(None)
     controls.append((result['reference'],sha(pcm)))
     if observed:
      assert b.battle_prefix_complete(),('private prefix incomplete',topology,ready);events=b.battle_prefix_events();captured={'frame':2401,'private':{'topology':topology,'ready':ready},'events':[[getattr(events[i],n)for n in FIELDS]for i in range(b.battle_prefix_count())]}
    assert controls[0]==controls[1],('private observer changed frame',topology,ready);rows.append(captured)
  identity=ref.identity
 raw=(json.dumps({'version':1,'fields':FIELDS,'cases':rows},separators=(',',':'))+'\n').encode();fixture=out/'native-battle-prefix.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(rows),'events':sum(len(r['events'])for r in rows),'all_2400_video_ram_and_pcm_controls_equal':True,'private_observer_on_off_equal':True,'original_control_sha256':sha(controls_path.read_bytes()),'identity':identity,'fixture_sha256':sha(fixture.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),source]},'scope':'Actual natural FFF000 phase0 through219ac entry with original batch boundaries/VC reads/controller IO. Does not independently infer world entry clocks or entire match.'};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
