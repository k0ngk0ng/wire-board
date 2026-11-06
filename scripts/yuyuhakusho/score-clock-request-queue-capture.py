"""Original live queue consumer fields and complete ordinary-request driver ticks."""
import argparse
import ctypes as C
import gzip
import importlib.util
import json
from pathlib import Path
from extract_audio_assets import AudioSession,write_cpu
from inspect_dac_dsp_state import symbols
from reference import ROOT,CPUHead
from research_audio_loops import ORIGINAL,OBSERVED,quiet_checkpoint

def module(name,file):
 s=importlib.util.spec_from_file_location(name,ROOT/'scripts/yuyuhakusho'/file);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
ordinary=module('request_ordinary','score-clock-ordinary-capture.py')
command=module('request_command','pcm-command-capture.py')

def load(p):
 raw=p.read_bytes();return json.loads(gzip.decompress(raw)if p.suffix=='.gz'else raw)
def shape(before,table):
 candidate=before['priority'];special=False;result=[]
 for slot in range(3,-1,-1):
  code=before['queue'][slot]
  if special:result.append(('lower',slot,code==0,bool(code and table[code-1]==255)));continue
  if code==0:result.append(('normal',slot,'zero'));continue
  priority=table[code-1]
  if priority==255:result.append(('normal',slot,'special'));special=True;continue
  accept=(priority&127)>=(candidate&127);result.append(('normal',slot,'accept'if accept else'reject'))
  if accept:candidate=priority
 result.append(('exit',special,candidate>=128));return tuple(result)
def cases(table):
 old=load(ROOT/'games/yuyuhakusho/engine/testdata/sound-queue-boundaries.json.gz');assert old['priorities']==table
 seen={}
 for row in old['calls']:
  before={k:row['before'][k]for k in['priority','selected','queue']};seen.setdefault(shape(before,table),before)
 rows=[dict(kind='queue',before=b)for b in seen.values()]
 rows.insert(0,dict(kind='queue',before=dict(priority=0,selected=255,queue=[0,0,0,0])))
 for code in[6,130,162,1,255]:
  for slot in range(4):
   q=[0]*4;q[slot]=code;rows.append(dict(kind='queue',before=dict(priority=80,selected=255,queue=q)))
 data=load(ROOT/'games/yuyuhakusho/data/audio-pcm-payloads.json.gz')
 for r in data['ordinary']:
  rows.append(dict(kind='ordinary',code=r['code'],before=dict(priority=0,selected=255,queue=[r['code'],0,0,0]),descriptor=r['descriptor']))
 for i,row in enumerate(rows):row['id']=f'{i}-{row["kind"]}';row['origin'],row['refresh']=[(0,0),(1,896),(895,896),(1008,1008)][i%4]
 return rows

def setup(ref,checkpoint,row,lookup):
 ref.restore(checkpoint,restore_refresh_cycles=True);ref.api('psg_reset',None)();image=C.cast(ref.lib.audio_update,C.c_void_p).value-lookup['_audio_update'][0];C.c_int.from_address(image+lookup['_fm_cycles_busy'][0]).value=0;C.c_void_p.from_address(image+lookup['_fm_ptr'][0]).value=image+lookup['_fm_buffer'][0];command.initial(ref);ref.cpu.cycles=0;ref.api('gen_zbusreq_w',None,C.c_uint,C.c_uint)(1,0)
 for i in range(928):write_cpu(ref,0xfff800+i,0)
 before=row['before'];write_cpu(ref,0xfff800,before['priority']);write_cpu(ref,0xfff809,before['selected'])
 for i,code in enumerate(before['queue']):write_cpu(ref,0xfff80a+i,code)
 z=(C.c_ubyte*8192).in_dll(ref.lib,'zram');z[0x1ffa]=192;z[0x1ffb]=1;z[0x1ffe]=z[0x1ffd]=z[0x1f2a]=0
 setter=ref.api('m68k_set_reg',None,C.c_uint,C.c_uint)
 for i in range(15):setter(i,0)
 for i,value in[(17,0x2700),(15,0xffff00),(16,0x37c44 if row['kind']=='queue'else 0x375ae)]:setter(i,value)
 write_cpu(ref,0xffff00,0x3d8,4);ref.cpu.cycles,ref.cpu.refresh_cycles=row['origin'],row['refresh'];return ref.save()

def execute(ref,row,observer=None):
 run=ref.api('m68k_run',None,C.c_uint);steps=[];fields=[]
 # Field events are native observations at actual original memory instructions.
 for _ in range(2000):
  pc=ref.get_register(16)&0xffffff
  if pc==0x3d8:return steps,fields
  assert 0x375ae<=pc<=0x37656 or 0x37c44<=pc<=0x380a6 or 0x3893a<=pc<=0x389d0 or 0x38c2e<=pc<=0x38d06,('escaped request path',row['id'],hex(pc))
  before,refresh=ref.cpu.cycles,ref.cpu.refresh_cycles;at=before+(14 if before>=refresh else 0);address=None;kind=None
  if pc==0x37c4c:address=0xf800;kind='priority-read'
  elif pc in[0x37c52,0x37c96]:address=((ref.get_register(9)&65535)-1)&65535;kind='queue-read'
  elif pc in[0x37c56,0x37cac]:address=ref.get_register(9)&65535;kind='queue-clear'
  elif pc in[0x37c7e,0x37c90,0x37cba]:address=0xf809;kind='selected-write'
  elif pc==0x37c8a:address=0xf800;kind='priority-write'
  elif pc in[0x37ef8,0x37f1e,0x37fce]:address=0xf82c;kind='choice-read'if pc==0x37ef8 else'choice-write'
  value=C.c_ubyte.from_address(ref.ram_pointer+(address^1)).value if address is not None else None
  if observer:observer.score_clock_pc(pc)
  run(before+1);steps.append([pc,before,refresh,ref.cpu.cycles,ref.cpu.refresh_cycles])
  if address is not None:
   if kind.endswith('write')or kind=='queue-clear':value=C.c_ubyte.from_address(ref.ram_pointer+(address^1)).value
   fields.append(dict(kind=kind,at=at,address=address,value=value))
 raise AssertionError(('request path step cap',row['id']))

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True);rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();table=list(rom[0x3a50e:0x3a50e+255]);inputs=cases(table);observer=ordinary.build(out);controls={};rows=[];identities={};print(json.dumps({'profiles':len(inputs),'queue':sum(r['kind']=='queue'for r in inputs),'ordinary':sum(r['kind']=='ordinary'for r in inputs)}),flush=True)
 for name,core in[('original',ORIGINAL),('observed',OBSERVED)]:
  with AudioSession(ROOT,out/('core-'+name),core=core)as ref:
   checkpoint,lookup=quiet_checkpoint(ref),symbols(core);identities[name]=ref.identity
   for i,row in enumerate(inputs):
    initial=setup(ref,checkpoint,row,lookup);plain_steps,plain_fields=execute(ref,row);plain,ram=ref.save(),ref.ram_cpu();again=setup(ref,checkpoint,row,lookup);assert initial['data']==again['data'];ordinary.shared.attach(observer,ref);assert observer.score_clock_ordinary_enable()==1
    try:steps,fields=execute(ref,row,observer);events=ordinary.shared.events(observer)
    finally:assert observer.score_clock_ordinary_disable()==1 and observer.score_clock_detach()==1
    saved=ref.save();assert steps==plain_steps and fields==plain_fields and saved['data']==plain['data']and saved['refresh_cycles']==plain['refresh_cycles']and ram==ref.ram_cpu();r=ref.ram_cpu();z=(C.c_ubyte*8192).in_dll(ref.lib,'zram');result=dict(steps=steps,fields=fields,events=[e for e in events if e['kind']!=7],end=dict(Cycle=ref.cpu.cycles,Refresh=ref.cpu.refresh_cycles),after=dict(priority=r[0xf800],selected=r[0xf809],queue=list(r[0xf80a:0xf80e]),choice=r[0xf82c]),pcm_fields={str(k):int(z[k])for k in[0x1ffc,0x1ffd,0x1ffe,0x1ffb,0x1ffa,0x1f86,0x1f87,0x1fa6,0x1fa7]})
    if name=='original':controls[i]=result
    else:assert controls[i]==result,('request cores mismatch',row['id']);rows.append(dict(input=row,**result,native_observer_state_equal=True))
  print(json.dumps({'core':name,'profiles':len(inputs),'pass':True}),flush=True)
 paths=[Path(__file__),rompath,ROOT/'scripts/yuyuhakusho/score-clock-ordinary-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-ordinary-observer.c',ROOT/'scripts/yuyuhakusho/pcm-command-capture.py',ROOT/'games/yuyuhakusho/engine/testdata/sound-queue-boundaries.json.gz'];payload=dict(version=1,scope='Actual queue consumer fields and complete ordinary-request driver ticks; canonical empty auxiliary held0, no extraIRQ or score voices.',priorities=table,identities=identities,cases=rows,sources={str(p.relative_to(ROOT)):ordinary.shared.sha(p.read_bytes())for p in paths});target=out/'native-request-queue-full.json.gz';target.write_bytes(gzip.compress((json.dumps(payload,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0));print(json.dumps({'pass':True,'profiles':len(rows),'executions':len(rows)*4,'sha256':ordinary.shared.sha(target.read_bytes())}),flush=True)
if __name__=='__main__':main()
