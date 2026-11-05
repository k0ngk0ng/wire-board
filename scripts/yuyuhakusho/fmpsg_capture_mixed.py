"""Declared native FM+PSG+DAC port streams and complete PCM/frame state."""
from pathlib import Path
import argparse,base64,ctypes as C,gzip,json,random
from extract_audio_assets import AudioSession
from research_audio_loops import quiet_checkpoint
from inspect_dac_dsp_state import symbols,snapshot
from fmpsg_capture_ym import normal,tone,reg,sha,ORIGINAL,PINNED
from fmpsg_psg_capture import PSG,typed
from reference import ROOT
F=896040

def ym(cycle,writes):return [dict(Cycle=cycle,Kind=1,Port=port,Value=value)for port,value in writes]
def psg(cycle,values):return [dict(Cycle=cycle,Kind=2,Port=0,Value=value)for value in values]
def read(cycle,port=0):return dict(Cycle=cycle,Kind=3,Port=port,Value=0)
def cases():
 rows=[]
 def add(name,frames):
  # Same-cycle input order stays explicit. Frame-ahead inputs never move back.
  prior=0
  for frame,events in enumerate(frames):
   events.sort(key=lambda e:e['Cycle'])
   for e in events:
    e['Cycle']+=frame*F;assert e['Cycle']>=prior,(name,frame,e,prior);prior=e['Cycle']
  rows.append((name,frames))
 setup=sum((tone(c,c%8,c%8,c)for c in range(6)),[])
 frames=[]
 for i in range(24):
  events=ym(0,setup if i==0 else reg(i%2,0xb0+i%3,i%8|((i%8)<<3)))
  for cycle,c in [(1000,0),(1008,3),(19333,1),(342000,4),(F-1,2)]:
   bank,ch=divmod(c,3);events+=ym(cycle,reg(bank,0xa4+ch,0x20|(i%8))+reg(bank,0xa0+ch,(i*13+61)&255)+reg(bank,0xb4+ch,[0,64,128,192][i%4]))
  events+=[read(19334),read(19334,1),read(F)];frames.append(events)
 add('six-voice-algorithms',frames)
 frames=[]
 for i in range(36):
  events=ym(0,setup if i==0 else [])
  events+=ym(100+i,reg(0,0x22,8|(i%8))+reg(0,0x90,8|(i%8))+reg(0,0x50,31 if i%3 else 1)+reg(0,0xb4,0xc0|((i%4)<<4)|(i%8)))
  if i==5:events+=ym(3000,reg(0,0xac,0x34)+reg(0,0xa8,0x78)+reg(0,0xad,0x2f)+reg(0,0xa9,0xba)+reg(0,0xae,0x21)+reg(0,0xaa,0x22)+reg(0,0x27,0x40))
  if i==12:events+=ym(4000,reg(0,0x24,255)+reg(0,0x25,2)+reg(0,0x26,255)+reg(0,0x27,0x8f))
  if i==25:events+=ym(7777,reg(0,0x27,0x3f))
  events+=[read(10000),read(10224),read(100000)];frames.append(events)
 add('lfo-ssg-special-csm',frames)
 add('fm-lookahead-state',[ym(0,setup)+ym(F+195,reg(0,0x28,0)),ym(195,reg(0,0x28,0xf0))+ym(F+1008,reg(0,0x22,15)),ym(1008,reg(0,0x27,0x40)),[],ym(F,reg(0,0x27,0)),[]])
 add('psg-lookahead-tail',[psg(0,[0x81,0,0x90,0xc1,0,0xdf,0xe7,0xf0])+psg(F+1008,[0x99,0xf8]),psg(1008,[0x90,0xf0])+psg(F+195,[0x91]),psg(195,[0x95,0xe3]),[],psg(F,[0x9f,0xff]),[]])
 rng=random.Random(0x4d449);frames=[]
 for i in range(120):
  events=ym(0,setup if i==0 else [])+psg(0,[0x80|(i&15),i%64,0x90|(i%16),0xa1,0,0xb3,0xe4|(i%4),0xf2])
  for j in range(16):
   cycle=1200+j*47000+rng.randrange(2000)
   if j%3==0:
    c=rng.randrange(6);bank,ch=divmod(c,3);events+=ym(cycle,reg(bank,0xa4+ch,rng.randrange(64))+reg(bank,0xa0+ch,rng.randrange(256)))
   elif j%3==1:events+=psg(cycle,[rng.randrange(256)])
   else:events+=ym(cycle,reg(0,0x2a,rng.randrange(256))+reg(0,0x2b,128 if i%3 else 0))
  events+=[read(800001),read(800002,2)];frames.append(events)
 add('long-mixed-dac-psg',frames)
 return rows

def capture(core,out):
 lookup=symbols(core);rows=[]
 with AudioSession(ROOT,out,core=core)as ref:
  checkpoint=quiet_checkpoint(ref)
  image=C.cast(ref.lib.psg_reset,C.c_void_p).value-lookup['_psg_reset'][0];rawpsg=PSG.from_address(image+lookup['_psg'][0])
  rawym=__import__('inspect_dac_dsp_state').YM.from_address(image+lookup['_ym2612'][0])
  writer=C.CFUNCTYPE(None,C.c_uint,C.c_uint,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_write').value)
  reader=C.CFUNCTYPE(C.c_uint,C.c_uint,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_read').value)
  pwrite=ref.api('psg_write',None,C.c_uint,C.c_uint);end_time=C.c_uint.in_dll(ref.lib,'mcycles_vdp');update=ref.api('audio_update',C.c_int,C.c_void_p)
  def state(time,global_cursor,psg_cursor):
   ref.frame=checkpoint['frame']+time//F;s=snapshot(ref,lookup)['initial_state'];busy=C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value
   assert s['resampler']['available_samples']==0
   return dict(Time=time,YM=normal(rawym),PSG=typed(rawpsg,time,max(psg_cursor,time)),LastFMOutput=s['previous_chip_output'],Integrator=s['resampler']['integrator'],Tail=s['resampler']['tail'],InputAhead=max(0,global_cursor-time),BusyUntil=time+busy if busy else 0,Pending=None)
  for name,frames in cases():
   ref.restore(checkpoint,restore_refresh_cycles=True);initial=state(0,0,0);parts=[];states=[];global_cursor=psg_cursor=0;ram=sha(ref.ram_cpu());video=sha(ref.image)
   for frame,events in enumerate(frames):
    reads=[]
    for event in events:
     cycle=event['Cycle']-frame*F
     if event['Kind']==1:writer(cycle,event['Port'],event['Value'])
     elif event['Kind']==2:pwrite(cycle,event['Value']);psg_cursor=event['Cycle']
     else:reads.append(dict(Cycle=event['Cycle'],Port=event['Port'],Value=reader(cycle,event['Port'])))
     global_cursor=event['Cycle']
    end_time.value=F;buffer=(C.c_int16*(4410*2))();count=update(buffer);assert 0<count<=4410;pcm=C.string_at(buffer,count*4);parts.append(pcm)
    assert sha(ref.ram_cpu())==ram and sha(ref.image)==video,'private sound boundary changed non-audio guest state'
    states.append(dict(Events=events,Output=base64.b64encode(pcm).decode(),Reads=reads,State=state((frame+1)*F,global_cursor,psg_cursor)))
   rows.append(dict(Name=name,Config=dict(Profile=1,MasterOrigin=checkpoint['frame']*F),Initial=initial,Frames=states,PCM_SHA256=sha(b''.join(parts))))
 return rows

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 original=capture(ORIGINAL,ROOT/'.local/md-yuyuhakusho/fmpsg-mixed-native/original');observed=capture(PINNED,ROOT/'.local/md-yuyuhakusho/fmpsg-mixed-native/observed');assert original==observed,'original/hooked full mixed PCM/state differs'
 raw=(json.dumps(dict(Cases=original),separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0);(out/'native-frames.json.gz').write_bytes(compressed)
 report=dict(pass_=True,scope='Declared private FM/PSG/DAC/status inputs and complete original native mixed audio_update boundary; no original CPU/game event timing claim',cases=len(original),frames=sum(len(c['Frames'])for c in original),stereo_frames=sum(len(base64.b64decode(f['Output']))//4 for c in original for f in c['Frames']),original_sha256=sha(ORIGINAL.read_bytes()),observed_sha256=sha(PINNED.read_bytes()),producer_sha256=sha(Path(__file__).read_bytes()),json_bytes=len(raw),json_sha256=sha(raw),gzip_bytes=len(compressed),gzip_sha256=sha(compressed))
 report['pass']=report.pop('pass_');(out/'native-capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
if __name__=='__main__':main()
