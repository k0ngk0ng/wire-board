"""Original native interactive chip/status oracle with a non-mutating FIR projection.

No gameplay frames run. Each declared cold audio-only case resets the original
chips and blip using native APIs. FM raw output is projected into a separate
native blip copy only for comparison with the Go in-flight representation.
"""
from pathlib import Path
import argparse,base64,ctypes as C,gzip,json
from extract_audio_assets import AudioSession
from inspect_dac_dsp_state import Config,Snd,Blip,YM,symbols
from fmpsg_capture_ym import normal,tone,reg,sha,ORIGINAL,PINNED
from fmpsg_psg_capture import PSG,typed,blip_state
from reference import ROOT
F=896040
def yw(cycle,writes):return [dict(Cycle=cycle,Kind=1,Port=p,Value=v)for p,v in writes]
def pw(cycle,values):return [dict(Cycle=cycle,Kind=2,Port=0,Value=v)for v in values]
def rd(cycle,port=0):return dict(Cycle=cycle,Kind=3,Port=port,Value=0)
def boundary():return dict(Boundary=True)

def cases():
 out=[]
 setup=tone(0,7,6,2)+tone(5,4,7,4)
 ops=yw(0,setup)
 ops+=[rd(t)for t in [0,1,41,42,1343,1344,1345]]
 ops+=yw(1400,reg(0,0x2a,42))+[rd(t,p)for t,p in [(1400,0),(1400,1),(2771,2),(2771,3),(2771,0),(2772,0),(2773,0)]]
 ops+=[boundary(),rd(F+5),boundary()]
 out.append(dict(Name='busy-grid-boundaries',Operations=ops))
 ops=yw(0,setup)+pw(0,[0x81,0,0x90,0xe7,0xf0])
 for t in [1,239,240,241,479,480,481,1007,1008,1009,8000]:
  ops+=yw(t,[[0,0xa4],[2,0xa5],[1,0x2f],[0,0xa0],[3,t&255]])+pw(t,[0x90|(t%16)])+[rd(t,0),rd(t,3)]
 ops+=[boundary(),boundary()]
 out.append(dict(Name='ports-psg-same-cycle',Operations=ops))
 ops=yw(0,setup)+pw(0,[0x81,0,0x90,0xc1,0,0xe7,0xf0])
 ops+=yw(F-20,reg(0,0x2a,0)+reg(0,0x2b,128))+[rd(F+195)]+pw(F+195,[0x94,0xf4])+[boundary()]
 ops+=[rd(F+195),rd(F+1008)]+yw(2*F+1008,reg(0,0x22,15))+pw(2*F+1008,[0x91,0xf0])+[boundary(),rd(2*F+1008),rd(2*F+2400),boundary(),boundary()]
 out.append(dict(Name='explicit-frame-overrun',Operations=ops))
 programs=[]
 for i,start in enumerate([0,100,3000,3100,7000,7100,11000,11100]):
  programs.append(dict(Start=start,PollStep=[70,84,126][i%3],AddressCost=[28,42][i%2],DataCost=[42,56][i%2],Bank=i%2,Register=[0x22,0x27,0xb4,0x2a][i%4],Value=[15,0,192,31][i%4]))
 out.append(dict(Name='adaptive-service',Programs=programs,Operations=[boundary()]))
 ops=yw(0,setup+tone(2,3,7,6)+reg(0,0x24,255)+reg(0,0x25,1)+reg(0,0x26,255)+reg(0,0x27,0x8f))
 for frame in range(4):
  for j in range(18):
   t=frame*F+500+j*49100
   ops+=[rd(t),rd(t,1)]+pw(t,[0xe0|(j%8),0xf0|(j%16)])
  ops+=[boundary()]
 out.append(dict(Name='status-timers-mixed',Operations=ops))
 return out

def capture(core,out):
 lookup=symbols(core);rows=[];projection_checks=0
 with AudioSession(ROOT,out,core=core)as ref:
  image=C.cast(ref.lib.psg_reset,C.c_void_p).value-lookup['_psg_reset'][0]
  nativeym=YM.from_address(image+lookup['_ym2612'][0]);nativepsg=PSG.from_address(image+lookup['_psg'][0]);snd=Snd.in_dll(ref.lib,'snd');cfg=Config.in_dll(ref.lib,'config')
  assert (cfg.hq_fm,cfg.hq_psg,cfg.fm_preamp,cfg.psg_preamp,cfg.filter,cfg.ym2612)==(1,1,100,150,0,0)
  scalar=lambda name:C.c_int.from_address(image+lookup['_'+name][0]).value
  fmraw=(C.c_int32*(1080*2*24)).from_address(image+lookup['_fm_buffer'][0])
  writer=C.CFUNCTYPE(None,C.c_uint,C.c_uint,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_write').value)
  reader=C.CFUNCTYPE(C.c_uint,C.c_uint,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_read').value)
  pwrite=ref.api('psg_write',None,C.c_uint,C.c_uint);update=ref.api('audio_update',C.c_int,C.c_void_p);nativeend=C.c_uint.in_dll(ref.lib,'mcycles_vdp')
  new=ref.api('blip_new',C.c_void_p,C.c_int);delete=ref.api('blip_delete',None,C.c_void_p);add=ref.api('blip_add_delta',None,C.c_void_p,C.c_uint,C.c_int,C.c_int)
  for row in cases():
   ref.api('YM2612Init',None)();ref.api('YM2612Config',None,C.c_int)(0);ref.api('psg_init',None,C.c_int)(1);ref.api('sound_reset',None)();ref.api('audio_reset',None)()
   base=cursor=psgcursor=count=0;ops=[];ram=sha(ref.ram_cpu());video=sha(ref.image)
   def snapshot():
    nonlocal projection_checks
    pointer=snd.blips[0];blip=Blip.from_address(pointer);before=(blip_state(pointer),C.string_at(C.addressof(nativeym),C.sizeof(YM)),C.string_at(C.addressof(nativepsg),C.sizeof(PSG)))
    projected=new(blip.size);assert projected
    try:
     other=Blip.from_address(projected);other.factor=blip.factor;other.offset=blip.offset
     for side in range(2):other.integrator[side]=blip.integrator[side];C.memmove(other.buffer[side],blip.buffer[side],(blip.size+18)*4)
     fmstart=scalar('fm_cycles_start');fmcount=scalar('fm_cycles_count');assert fmcount>=fmstart and (fmcount-fmstart)%1008==0
     generated=(fmcount-fmstart)//1008
     previous=[C.c_int.from_address(image+lookup['_fm_last.'+str(i)][0]).value for i in range(2)]
     for index in range(generated):
      time=fmstart+index*1008
      if time>=F:break
      current=[fmraw[index*2],fmraw[index*2+1]];add(projected,time,current[0]-previous[0],current[1]-previous[1]);previous=current
     samplecount=(F*blip.factor+blip.offset)>>52;assert samplecount in [735,736]
     fir=[list((C.c_int32*(samplecount+18)).from_address(other.buffer[side]))for side in range(2)]
     busy=scalar('fm_cycles_busy')
     result=dict(Time=base,Cursor=cursor,InputCount=count,FMGenerated=generated,YM=normal(nativeym),PSG=typed(nativepsg,base,psgcursor),LastFMOutput=previous,Integrator=list(blip.integrator),BusyUntil=base+busy if busy else 0,FIR=fir)
    finally:delete(projected)
    assert before==(blip_state(pointer),C.string_at(C.addressof(nativeym),C.sizeof(YM)),C.string_at(C.addressof(nativepsg),C.sizeof(PSG))),'FIR projection changed original'
    assert sha(ref.ram_cpu())==ram and sha(ref.image)==video,'sound-only boundary changed guest state'
    projection_checks+=1;return result
   def committed(state):
    assert count==0
    return dict(Time=base,YM=state['YM'],PSG=state['PSG'],LastFMOutput=state['LastFMOutput'],Integrator=state['Integrator'],Tail=[x[:18]for x in state['FIR']],InputAhead=cursor-base,BusyUntil=state['BusyUntil'],Pending=None)
   initial=committed(snapshot())
   def event(e):
    nonlocal cursor,psgcursor,count
    assert e['Cycle']>=cursor and e['Cycle']<=base+F+1008,(row['Name'],e,cursor,base)
    local=e['Cycle']-base;value=0
    if e['Kind']==1:writer(local,e['Port'],e['Value'])
    elif e['Kind']==2:pwrite(local,e['Value']);psgcursor=e['Cycle']
    else:value=reader(local,e['Port'])
    cursor=e['Cycle'];count+=1
    ops.append(dict(Boundary=False,Event=e,Read=value,Snapshot=snapshot(),Committed=None,PCM=''))
    return value
   for request in row.get('Programs',[]):
    cycle=request['Start']
    while event(rd(cycle))&128:cycle+=request['PollStep']
    cycle+=request['AddressCost'];event(yw(cycle,[[request['Bank']*2,request['Register']]])[0])
    cycle+=request['DataCost'];event(yw(cycle,[[request['Bank']*2+1,request['Value']]])[0])
   for item in row['Operations']:
    if not item.get('Boundary'):event(item);continue
    nativeend.value=F;buffer=(C.c_int16*(4410*2))();length=update(buffer);pcm=C.string_at(buffer,length*4)
    base+=F;cursor=max(cursor,base);psgcursor=max(psgcursor,base);count=0
    state=snapshot();ops.append(dict(Boundary=True,Event=dict(Cycle=0,Kind=0,Port=0,Value=0),Read=0,Snapshot=state,Committed=committed(state),PCM=base64.b64encode(pcm).decode()))
   rows.append(dict(Name=row['Name'],Config=dict(Profile=1,MasterOrigin=0),Initial=initial,Programs=row.get('Programs',[]),Operations=ops))
 return rows,projection_checks

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 original,checks=capture(ORIGINAL,ROOT/'.local/md-yuyuhakusho/fmpsg-interactive-capture/original');observed,other=capture(PINNED,ROOT/'.local/md-yuyuhakusho/fmpsg-interactive-capture/observed');assert original==observed and checks==other
 raw=(json.dumps(dict(Cases=original),separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0);(out/'native.json.gz').write_bytes(compressed)
 report=dict(pass_=True,scope='Private cold original chip resets, immediate native port/status feedback and non-mutating in-flight FIR projection; no CPU or real driver phase-cost claim',cases=len(original),events=sum(not o['Boundary']for r in original for o in r['Operations']),frames=sum(o['Boundary']for r in original for o in r['Operations']),stereo_frames=sum(len(base64.b64decode(o['PCM']))//4 for r in original for o in r['Operations']if o['Boundary']),original_sha256=sha(ORIGINAL.read_bytes()),observed_sha256=sha(PINNED.read_bytes()),projection_nochange_checks_per_core=checks,producer_sha256=sha(Path(__file__).read_bytes()),json_sha256=sha(raw),gzip_sha256=sha(compressed),gzip_bytes=len(compressed))
 report['pass']=report.pop('pass_');(out/'native-capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
if __name__=='__main__':main()
