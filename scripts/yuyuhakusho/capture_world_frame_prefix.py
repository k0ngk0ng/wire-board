"""Actual logical frame prefix and input normalization with live VINT interleaving."""
import argparse,ctypes as C,gzip,json,os,subprocess
from pathlib import Path
from extract_audio_assets import AudioSession
from reference import ROOT,CPUHead,buttons_at,sha
from capture_battle_world_entry import CORE,CPUControl
from capture_battle_world_dma import FIELDS,Event
from capture_pcm_transfer_gate import command
from capture_world_frame import snapshot

def ports(raw):
 word=lambda off:int.from_bytes(raw[off:off+2],'big')
 result=[]
 for index in range(2):
  base=0xf700+index*40;team=word(base)==12;pads=[]
  for pad in range(4):
   if not team and pad>0:pads.append({'Device':0,'Held':0});continue
   offset=base+(pad+1)*8 if team else base;kind=word(offset);assert kind in[4,8],(index,pad,kind)
   held=((raw[offset+2]^15)&15)|(((raw[offset+3]^15)&15)<<4)
   if kind==8:held|=((raw[offset+4]^15)&15)<<8
   pads.append({'Device':6 if kind==8 else 3,'Held':held})
  result.append({'team_player':team,'pads':pads})
 return result
def graphics(raw):
 word=lambda off:int.from_bytes(raw[off:off+2],'big')
 return {'Pending':None,'UploadPending':bool(raw[0xf54e]&128),'CachedDisplay':bool(word(0xf6d2)&64),'SceneIRQReady':bool(raw[0xf50f]),'PreviousVideoMode':word(0xf514),'SavedColors':[word(0xf140),word(0xf142)],'SpriteMarker':word(0xe202),'Layout':{'Planes':[(word(0xf6d4)&0x38)<<10,(word(0xf6d8)&7)<<13],'Window':(word(0xf6d6)&0x3e)<<10,'Sprites':(word(0xf6da)&0x7e)<<9,'HScroll':(word(0xf6ea)&0x3f)<<10,'PlaneSize':word(0xf6f0)&255,'WindowHorizontal':word(0xf6f2)&255,'WindowVertical':word(0xf6f4)&255,'CounterBase':word(0xf6e4)&255}}
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True);tmp=out/'tmp';tmp.mkdir();source=ROOT/'scripts/yuyuhakusho/world-frame-prefix-observer.c';binary=tmp/'world-prefix.dylib'
 subprocess.run(['clang','-dynamiclib','-O2','-fno-modules',str(source),'-o',str(binary)],cwd=ROOT,env=dict(os.environ,TMPDIR=str(tmp),CLANG_MODULE_CACHE_PATH=str(tmp/'clang-cache')),check=True)
 b=C.CDLL(str(binary));b.battle_prefix_attach.argtypes=[C.c_void_p]*14;b.battle_prefix_attach.restype=C.c_int;b.battle_prefix_count.restype=C.c_uint;b.battle_prefix_complete.restype=C.c_uint;b.battle_world_attach.argtypes=[C.c_void_p]*3;b.battle_world_attach.restype=C.c_int;b.battle_world_dma_attach.argtypes=[C.c_void_p]*4;b.battle_world_dma_attach.restype=C.c_int;b.battle_world_dma_events.restype=C.POINTER(Event);b.world_frame_prefix_snapshot.argtypes=[C.c_uint,C.c_uint];b.world_frame_prefix_snapshot.restype=C.c_void_p;b.world_frame_prefix_carry.restype=C.c_uint;b.world_frame_prefix_attach_read.argtypes=[C.c_void_p];b.world_frame_prefix_attach_read.restype=C.c_int;b.world_frame_prefix_detach_read.restype=C.c_int
 for name in ['world_frame_prefix_cases','world_frame_prefix_active','world_frame_prefix_first','world_frame_prefix_last','world_frame_prefix_frame','world_frame_prefix_end_frame']:getattr(b,name).restype=C.c_uint
 controls_path=ROOT/'games/yuyuhakusho/engine/testdata/audio-timing-baseline-original.json.gz';gold=json.loads(gzip.decompress(controls_path.read_bytes()));inputs=json.loads((ROOT/'docs/research/md-yuyuhakusho/inputs.json').read_text());selected={2069,2071,2072,2073,2074,2393,2394,2395,2397,2398,2400};rows=[]
 # A selected frame may contain both a carried call and a later new call.
 # Retain all calls, then drain two real control frames beyond selection.
 with AudioSession(ROOT,out/'unobserved-controls',core=command.ORIGINAL)as control_ref:
  extended=[]
  for frame in range(1,2403):
   result,pcm=control_ref.step_audio(frame,buttons_at(inputs,frame));extended.append({'reference':result['reference'],'frames':len(pcm)//4,'pcm_sha256':sha(pcm)})
 assert all(x['reference']==y['reference']and x['frames']==y['frames']and x['pcm_sha256']==y['pcm_sha256']for x,y in zip(extended[:2400],gold))
 gold=extended
 with AudioSession(ROOT,out/'reference',core=CORE)as ref:
  values=[t.in_dll(ref.lib,n)for n,t in [('reg',C.c_ubyte),('mcycles_vdp',C.c_uint),('v_counter',C.c_ushort),('h_counter',C.c_ubyte),('hint_pending',C.c_ubyte),('vint_pending',C.c_ubyte),('input',C.c_ubyte)]];reg,line,vc,hc,hp,vp,inp=map(C.addressof,values);get=ref.api('m68k_get_reg',C.c_uint,C.c_uint)
  assert b.battle_prefix_attach(ref.ram_pointer,reg,C.addressof(ref.cpu)+CPUHead.cycles.offset,C.addressof(ref.cpu)+CPUHead.refresh_cycles.offset,line,vc,hc,hp,vp,C.addressof(ref.cpu)+CPUHead.cycle_end.offset,inp,C.cast(get,C.c_void_p),C.addressof(ref.cpu)+CPUControl.pin.offset,C.addressof(ref.cpu)+CPUControl.mask.offset)
  z=command.Z80.in_dll(ref.lib,'Z80');zr=(C.c_uint8*8192).in_dll(ref.lib,'zram');bus=C.c_uint8.in_dll(ref.lib,'zstate');assert b.battle_world_attach(C.addressof(zr),C.addressof(bus),C.addressof(z)+command.Z80.cycles.offset)
  dma=[t.in_dll(ref.lib,n)for n,t in [('dma_length',C.c_uint),('dma_endCycles',C.c_uint),('dma_type',C.c_ubyte),('status',C.c_ushort)]];assert b.battle_world_dma_attach(*map(C.addressof,dma));hook=ref.api('set_cpu_hook',None,C.c_void_p)
  read_slot=C.c_void_p.in_dll(ref.lib,'z80_readmem');assert b.world_frame_prefix_attach_read(C.addressof(read_slot))
  b.world_frame_prefix_begin()
  for frame in sorted(selected):b.world_frame_prefix_select(frame)
  for frame in range(1,2403):
   b.battle_world_dma_frame(frame)
   if frame in selected or b.world_frame_prefix_active():hook(C.cast(b.world_frame_prefix_hook,C.c_void_p))
   try:result,pcm=ref.step_audio(frame,buttons_at(inputs,frame))
   finally:hook(None)
   expected=gold[frame-1];assert result['reference']==expected['reference']and len(pcm)//4==expected['frames']and sha(pcm)==expected['pcm_sha256'],('observer changed original frame',frame)
  assert not b.world_frame_prefix_active() and b.battle_prefix_count()<8192,('incomplete final prefix',b.world_frame_prefix_active(),b.world_frame_prefix_cases(),b.battle_prefix_count())
  events=b.battle_world_dma_events()
  for index in range(b.world_frame_prefix_cases()):
   before=C.string_at(b.world_frame_prefix_snapshot(index,0),65536);after=C.string_at(b.world_frame_prefix_snapshot(index,1),65536)
   rows.append({'frame':b.world_frame_prefix_frame(index),'end_frame':b.world_frame_prefix_end_frame(index),'before':snapshot(before),'after':snapshot(after),'graphics_before':graphics(before),'graphics_after':graphics(after),'scene':{'SceneMode':int.from_bytes(before[0xf580:0xf582],'big'),'SceneRequest':int.from_bytes(before[0xf582:0xf584],'big'),'VideoMode':int.from_bytes(before[0xf512:0xf514],'big'),'InterruptMode':int.from_bytes(before[0xf510:0xf512],'big'),'ScrollMode':before[0xf6e7],'BackdropColor':before[0xf6df],'SoundQueue':list(before[0xf800:0xf804])},'normalization':{'ports':ports(before),'channel_count':int.from_bytes(before[0xf552:0xf554],'big'),'swap_ac_mask':sum((1<<i)for i,v in enumerate(before[0xff50:0xff58])if v),'reserved_carry':b.world_frame_prefix_carry(index)},'devices':[6 if v&64 else 3 if v&128 else 0 for v in before[0xf7d0:0xf7d8]],'events':[[getattr(events[i],n)for n in FIELDS]for i in range(b.world_frame_prefix_first(index),b.world_frame_prefix_last(index))]})
  assert b.world_frame_prefix_detach_read()
  identity=ref.identity
 assert len(rows)>=11,len(rows)
 raw=(json.dumps({'version':1,'fields':FIELDS,'cases':rows},separators=(',',':'))+'\n').encode();fixture=out/'native-frame-prefix.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(rows),'events':sum(len(r['events'])for r in rows),'cross_frame':sum(r['frame']!=r['end_frame']for r in rows),'all_2400_video_ram_pcm_controls_equal':True,'identity':identity,'fixture_sha256':sha(fixture.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),source,ROOT/'scripts/yuyuhakusho/battle-world-dma-observer.c']},'scope':'Actual3a2e through3a5a includingcomplete normalization, clock/RNG/feedback and nestedIRQ. Logicalobjects andtail remainoutside.'};(out/'capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
