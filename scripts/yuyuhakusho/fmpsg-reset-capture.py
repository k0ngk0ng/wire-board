"""Original ordered fm_reset inputs; no sorting in the golden call stream.

The CPU counterexample is replayed from its frozen original audio calls, with
PCM/recordedDSP checked against that CPU run. Other cases directly drive active
native chips. In-flight FIR is a non-mutating projection using native blip APIs.
"""
from pathlib import Path
import argparse,base64,ctypes as C,gzip,json
from extract_audio_assets import AudioSession
from research_audio_loops import quiet_checkpoint
from inspect_dac_dsp_state import Config,Snd,Blip,YM,symbols,snapshot as native_dsp
from fmpsg_capture_ym import normal,tone,reg,sha,ORIGINAL,PINNED
from fmpsg_psg_capture import PSG,typed,blip_state
from reference import ROOT
F=896040
CPU=ROOT/'.local/md-yuyuhakusho/pcm-command-reset-chip-tick/single-reset-crossing-chip-tick.json.gz'
CPU_SHA='fafee45b94e24b01779d94a45d11af25ac905ccab2737beeb75bbd82368463b9'
def write(cycle,ports):return [dict(Kind='write',Cycle=cycle,Port=p,Value=v)for p,v in ports]
def psg(cycle,data):return [dict(Kind='psg',Cycle=cycle,Port=0,Value=v)for v in data]
def reset(cycle):return dict(Kind='reset',Cycle=cycle)
def after(request,complete):return dict(Kind='after-dac',Cycle=request,CompletedDAC=complete)
def read(cycle,port=0):return dict(Kind='read',Cycle=cycle,Port=port)
def frame():return dict(Kind='frame',Cycle=0)
def private_cases():
 setup=sum((tone(c,c%8,c%8,c,8+(c%8))for c in range(6)),[])
 dac=reg(0,0x2a,115)+reg(0,0x2b,128)+reg(1,0xb6,192)
 return [
  dict(Name='warm-fm-psg-reset',Operations=write(0,setup+reg(0,0x22,15)+reg(0,0x24,255)+reg(0,0x25,1)+reg(0,0x27,0x8f))+psg(0,[0x81,0,0x90,0xe7,0xf0])+[read(12345)]+write(20000,[[2,0xff]])+[reset(20000),read(20000)]+psg(20001,[0x95])+[read(20100),frame(),frame()]),
  dict(Name='private-cross-chip-tick',Operations=write(0,dac)+write(1100,reg(0,0x2a,123))+[after(1007,1100),reset(3300),frame(),frame()]),
  dict(Name='private-bound194-and104',Operations=write(0,dac)+write(1202,reg(0,0x2a,123))+[after(1008,1202),read(1202)]+write(3000,reg(0,0x2b,128)+reg(0,0x2a,144))+write(3600,reg(0,0x2a,127))+[after(3496,3600),frame(),frame()]),
  dict(Name='frame-ahead-reset-carry',Operations=write(0,dac)+psg(0,[0x81,0,0x90,0xe7,0xf0])+psg(F+193,[0x95])+write(F+194,reg(0,0x2a,123))+[after(F,F+194),frame(),read(F+194),reset(F+194)]+write(2*F+500,reg(0,0x2b,128)+reg(0,0x2a,115))+psg(2*F+1007,[0x91])+write(2*F+1008,reg(0,0x2a,123))+[after(2*F+814,2*F+1008),reset(2*F+1008),frame(),read(2*F+1008),frame()]),
  dict(Name='same-cycle-repeated-reset',Operations=write(0,reg(0,0x22,15))+[reset(0),reset(0),reset(0),read(0),frame()]),
  dict(Name='warm-address-latch-survives',Operations=write(0,dac)+write(3000,[[2,0xb6]])+[reset(3000)]+write(3000,[[1,64]])+[read(4000),frame(),frame()]),
 ]

def cpu_case(body):
 rows=[];cursor=0
 for f in range(len(body['frames'])):
  raw=[e for e in body['events']if e['frame']==f and e['kind']in [1,2,9,10]]
  for e in raw:
   cycle=f*F+e['cycles_before']
   if e['kind']==10:
    op=after(cycle,cursor)if cycle<cursor else reset(cycle)
    for previous in rows[-2:]:previous['Observe']=True
    op['Observe']=True;rows.append(op)
   else:
    bank=e['address']>>8;address=e['address']&255
    rows.extend(write(cycle,reg(bank,address,e['data'])))
    for op in rows[-2:]:op['Observe']=False
   cursor=max(cursor,cycle)
  rows.append(frame());cursor=max(cursor,(f+1)*F)
 return dict(Name='cpu-single-reset-crossing-chip-tick',Operations=rows,CPU=body)

class Oracle:
 def __init__(self,ref,lookup,origin):
  self.ref=ref;self.lookup=lookup;self.origin=origin;self.base=self.cursor=self.psgcursor=self.count=self.checks=0
  self.image=C.cast(ref.lib.psg_reset,C.c_void_p).value-lookup['_psg_reset'][0]
  self.ym=YM.from_address(self.image+lookup['_ym2612'][0]);self.psg=PSG.from_address(self.image+lookup['_psg'][0]);self.snd=Snd.in_dll(ref.lib,'snd')
  self.fmraw=(C.c_int32*(1080*2*24)).from_address(self.image+lookup['_fm_buffer'][0])
  self.writer=C.CFUNCTYPE(None,C.c_uint,C.c_uint,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_write').value)
  self.reset=C.CFUNCTYPE(None,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_reset').value)
  self.reader=C.CFUNCTYPE(C.c_uint,C.c_uint,C.c_uint)(C.c_void_p.in_dll(ref.lib,'fm_read').value)
  self.pwrite=ref.api('psg_write',None,C.c_uint,C.c_uint);self.update=ref.api('audio_update',C.c_int,C.c_void_p);self.nativeend=C.c_uint.in_dll(ref.lib,'mcycles_vdp')
  self.new=ref.api('blip_new',C.c_void_p,C.c_int);self.delete=ref.api('blip_delete',None,C.c_void_p);self.add=ref.api('blip_add_delta',None,C.c_void_p,C.c_uint,C.c_int,C.c_int)
  self.ram=sha(ref.ram_cpu());self.video=sha(ref.image)
 def scalar(self,name):return C.c_int.from_address(self.image+self.lookup['_'+name][0]).value
 def snapshot(self):
  pointer=self.snd.blips[0];b=Blip.from_address(pointer);before=(blip_state(pointer),C.string_at(C.addressof(self.ym),C.sizeof(YM)),C.string_at(C.addressof(self.psg),C.sizeof(PSG)))
  clone=self.new(b.size);assert clone
  try:
   v=Blip.from_address(clone);v.factor=b.factor;v.offset=b.offset
   for side in range(2):v.integrator[side]=b.integrator[side];C.memmove(v.buffer[side],b.buffer[side],(b.size+18)*4)
   start=self.scalar('fm_cycles_start');count=self.scalar('fm_cycles_count');assert count>=start and(count-start)%1008==0
   generated=(count-start)//1008
   last=[C.c_int.from_address(self.image+self.lookup['_fm_last.'+str(i)][0]).value for i in range(2)]
   for index in range(generated):
    time=start+index*1008
    if time>=F:break
    current=[self.fmraw[index*2],self.fmraw[index*2+1]];self.add(clone,time,current[0]-last[0],current[1]-last[1]);last=current
   samples=(F*b.factor+b.offset)>>52;assert samples in [735,736]
   fir=[list((C.c_int32*(samples+18)).from_address(v.buffer[side]))for side in range(2)]
   busy=self.scalar('fm_cycles_busy')
   result=dict(Time=self.base,Cursor=self.cursor,InputCount=self.count,FMGenerated=generated,YM=normal(self.ym),PSG=typed(self.psg,self.base,self.psgcursor),LastFMOutput=last,Integrator=list(b.integrator),BusyUntil=self.base+busy if busy else 0,FIR=fir)
  finally:self.delete(clone)
  assert before==(blip_state(pointer),C.string_at(C.addressof(self.ym),C.sizeof(YM)),C.string_at(C.addressof(self.psg),C.sizeof(PSG)))
  assert sha(self.ref.ram_cpu())==self.ram and sha(self.ref.image)==self.video
  self.checks+=1;return result
 def committed(self,state):
  assert self.count==0
  return dict(Time=self.base,YM=state['YM'],PSG=state['PSG'],LastFMOutput=state['LastFMOutput'],Integrator=state['Integrator'],Tail=[x[:18]for x in state['FIR']],InputAhead=self.cursor-self.base,BusyUntil=state['BusyUntil'],Pending=None)
 def apply(self,op,hypothesis=False):
  kind=op['Kind'];cycle=op.get('Cycle',0);value=0;pcm='';committed=None
  if kind=='frame':
   self.nativeend.value=F;buf=(C.c_int16*8820)();n=self.update(buf);pcm=base64.b64encode(C.string_at(buf,n*4)).decode()
   self.base+=F;self.cursor=max(self.cursor,self.base);self.psgcursor=max(self.psgcursor,self.base);self.count=0;self.ref.frame=(self.origin+self.base)//F
  else:
   assert self.base<=cycle<=self.base+F+1008
   if kind=='after-dac'and not hypothesis:assert op['CompletedDAC']==self.cursor and 0<=self.cursor-cycle<=194
   else:assert cycle>=self.cursor
   local=cycle-self.base
   if kind=='write':self.writer(local,op['Port'],op['Value'])
   elif kind=='psg':self.pwrite(local,op['Value']);self.psgcursor=cycle
   elif kind=='read':value=self.reader(local,op.get('Port',0))
   else:self.reset(local)
   self.cursor=max(self.cursor,cycle);self.count+=1
  state=self.snapshot()if op.get('Observe',True)or kind=='frame'else None
  if kind=='frame':committed=self.committed(state)
  return dict(Kind=kind,Cycle=cycle,CompletedDAC=op.get('CompletedDAC',0),Port=op.get('Port',0),Value=op.get('Value',0),Read=value,Snapshot=state,Committed=committed,PCM=pcm)

def run_case(ref,lookup,case,origin,sorted_hypothesis=False):
 oracle=Oracle(ref,lookup,origin);initial=oracle.committed(oracle.snapshot());ops=[];first=None;frames=0
 plans=case['Operations']
 if sorted_hypothesis:
  plans=[];batch=[]
  for op in case['Operations']:
   if op['Kind']=='frame':plans+=sorted(batch,key=lambda e:e['Cycle'])+[op];batch=[]
   else:batch.append(op)
  assert not batch
 for op in plans:
  row=oracle.apply(op,sorted_hypothesis);ops.append(row)
  if op['Kind']=='frame':
   if 'CPU'in case:
    expected=case['CPU']['frames'][frames];actual=base64.b64decode(row['PCM']);want=bytes.fromhex(expected['pcm_hex'])
    if actual!=want and first is None:first=dict(frame=frames,first_byte=next(i for i,(a,b)in enumerate(zip(actual,want))if a!=b))
    if not sorted_hypothesis:
     assert actual==want,('ordered replay differs from frozen CPU PCM',frames)
     assert native_dsp(ref,lookup)['initial_state']==expected['native_dsp'],('ordered replay recorded DSP state differs',frames)
   frames+=1
 return dict(Name=case['Name'],Config=dict(Profile=1,MasterOrigin=origin),Initial=initial,Operations=ops),dict(projection_checks=oracle.checks,frames=frames,first_difference=first)

def capture(core,out):
 lookup=symbols(core);rows=[];stats=[];sort_result=None
 with AudioSession(ROOT,out,core=core)as ref:
  cfg=Config.in_dll(ref.lib,'config');assert(cfg.hq_fm,cfg.hq_psg,cfg.fm_preamp,cfg.psg_preamp,cfg.filter,cfg.ym2612)==(1,1,100,150,0,0)
  checkpoint=quiet_checkpoint(ref);assert sha(CPU.read_bytes())==CPU_SHA;body=json.loads(gzip.decompress(CPU.read_bytes()));origin=body['initial_dsp']['initial_state']['master_origin']
  ref.restore(checkpoint,restore_refresh_cycles=True);assert native_dsp(ref,lookup)['initial_state']==body['initial_dsp']['initial_state']
  row,stat=run_case(ref,lookup,cpu_case(body),origin);rows.append(row);stats.append(stat)
  # Fresh quiescent native initial state, never restoring an in-flight reset.
  # Existing quiet_checkpoint is complete at the native frame boundary.
  ref.restore(checkpoint,restore_refresh_cycles=True)
  # The previous case ended at a flush with fm_ptr at buffer and busy0.
  assert C.c_int.from_address(C.cast(ref.lib.psg_reset,C.c_void_p).value-lookup['_psg_reset'][0]+lookup['_fm_cycles_busy'][0]).value==0
  sorted_row,sort_result=run_case(ref,lookup,cpu_case(body),origin,True)
  assert sort_result['first_difference']==dict(frame=8,first_byte=56),sort_result
  for case in private_cases():
   ref.api('YM2612Init',None)();ref.api('YM2612Config',None,C.c_int)(0);ref.api('psg_init',None,C.c_int)(1);ref.api('sound_reset',None)();ref.api('audio_reset',None)();ref.frame=0
   row,stat=run_case(ref,lookup,case,0);rows.append(row);stats.append(stat)
 return rows,stats,sort_result

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 original,stats,sortred=capture(ORIGINAL,ROOT/'.local/md-yuyuhakusho/fmpsg-reset-capture/original');observed,other,sortother=capture(PINNED,ROOT/'.local/md-yuyuhakusho/fmpsg-reset-capture/observed');assert original==observed and stats==other and sortred==sortother
 raw=(json.dumps(dict(Cases=original),separators=(',',':'))+'\n').encode();compressed=gzip.compress(raw,mtime=0);(out/'native.json.gz').write_bytes(compressed)
 report=dict(pass_=True,scope='Ordered actual fm_reset callbacks with full warm chip semantics; private194 profile plus frozen CPU DAC reset counterexample; no global event schedule or hardware-maximum claim',cases=len(original),events=sum(o['Kind']!='frame'for r in original for o in r['Operations']),frames=sum(o['Kind']=='frame'for r in original for o in r['Operations']),stereo_frames=sum(len(base64.b64decode(o['PCM']))//4 for r in original for o in r['Operations']if o['Kind']=='frame'),cpu_source=dict(path=str(CPU.relative_to(ROOT)),sha256=CPU_SHA,frames=12,stereo_frames=8832,all_pcm_and_recorded_dsp_match=True),original_sha256=sha(ORIGINAL.read_bytes()),observed_sha256=sha(PINNED.read_bytes()),projection_nochange_checks_per_core=sum(s['projection_checks']for s in stats),private_retreat_bound_master=194,raw_cycles_modified=False,producer_sha256=sha(Path(__file__).read_bytes()),json_sha256=sha(raw),gzip_sha256=sha(compressed),gzip_bytes=len(compressed))
 report['pass']=report.pop('pass_');(out/'native-capture.json').write_text(json.dumps(report,indent=2)+'\n');(out/'native-sort-counterexample.json').write_text(json.dumps(dict(pass_=False,sort_by_raw_cycle=True,first_difference=sortred['first_difference'],frames=12,stereo_frames=8832,cpu_source_sha256=CPU_SHA,scope='Deliberate wrong hypothesis; old20no-difference correction kept'),indent=2)+'\n');print(json.dumps(report))
if __name__=='__main__':main()
