#!/usr/bin/env python3
"""Passive fixed two-human dragon lifecycle against the single original control."""
import json
from collections import Counter
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha

inputs=ROOT/'docs/research/md-yuyuhakusho/source4-special-discovery-duo-inputs.json'
baseline=ROOT/'games/yuyuhakusho/engine/testdata/fighter4-attacks-duo-reference-frames.json'
out=ROOT/'.local/md-yuyuhakusho/source4-special-discovery/duo-lifecycle';out.mkdir(parents=True,exist_ok=True)
events=json.loads(inputs.read_text());expected=json.loads(baseline.read_text());assert len(expected)==5502
core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
rows=[];frames=[];counts=Counter();boost=[];enhanced=[];negative=[];normal=[];header=None;last=None;header_before=[];header_after=[]
def w(a,o):return int.from_bytes(a[o:o+2],'big',signed=True)
with ReferenceSession(ROOT,out,core=core) as ref:
 for f in range(1,5503):
  pre=ref.ram_cpu()
  if 1800<=f<=5500:header_before.append({'frame':f,'behavior':hex(int.from_bytes(pre[0x8000:0x8004],'big')),'stage':int.from_bytes(pre[0x8006:0x8008],'big'),'manager':hex(int.from_bytes(pre[0x8040:0x8044],'big')),'manager_stage':int.from_bytes(pre[0x8046:0x8048],'big')})
  frames.append(ref.step(f,buttons_at(events,f))['reference']);ram=ref.ram_cpu();a=ram[0x9000:0x9100];c=ram[0x9100:0x9200]
  if 1800<=f<=5500:header_after.append({'frame':f,'behavior':hex(int.from_bytes(ram[0x8000:0x8004],'big')),'stage':int.from_bytes(ram[0x8006:0x8008],'big'),'manager':hex(int.from_bytes(ram[0x8040:0x8044],'big')),'manager_stage':int.from_bytes(ram[0x8046:0x8048],'big')})
  if f==2500:header={'main_address':'0x8000','main_behavior':hex(int.from_bytes(ram[0x8000:0x8004],'big')),'main_stage':int.from_bytes(ram[0x8006:0x8008],'big'),'characters':list(ram[0xa100:0xa104]),'controllers':list(ram[0xa104:0xa108]),'channels':[ram[0x8476],ram[0x9076]],'tags':[ram[0x8458],ram[0x9058]]}
  if f<2200:continue
  source=int.from_bytes(a[0x90:0x94],'big');size=int.from_bytes(c[4:6],'big');code=int.from_bytes(c[:4],'big');energy=w(a,0xb8)
  counts[hex(a[0x5d])]+=1
  if source==0x15946:boost.append(f)
  if size and code==0x13fc8:enhanced.append(f)
  if energy<0:negative.append(f)
  if boost and source==0x12578 and size==0 and a[0x5d]==0:normal.append(f)
  key=(source,size,code,energy,a[0x5d],a[0x5f])
  if key!=last or f%100==0:rows.append({'frame':f,'hp':w(a,0x78),'energy':energy,'energy_fraction':int.from_bytes(a[0xba:0xbc],'big'),'action':hex(a[0x5d]),'pose':hex(a[0x5f]),'source':hex(source),'child_size':size,'child_code':hex(code)});last=key
 identity=ref.identity;ref.screenshot(out/'source4-special-discovery-duo-complete.png')
assert frames==expected
def summarize_headers(rows):
 counts=Counter();ranges=[]
 for row in rows:
  key=(row['behavior'],row['stage'],row['manager'],row['manager_stage']);counts[str(key)]+=1
  if ranges and ranges[-1]['key']==list(key):ranges[-1]['end']=row['frame']
  else:ranges.append({'start':row['frame'],'end':row['frame'],'key':list(key)})
 ready=[r['frame'] for r in rows if (r['behavior'],r['stage']) in [('0x9ad0',20),('0xa01e',32)] and r['manager']=='0xf2a4']
 return {'counts':dict(counts),'ranges':ranges,'first_ready':ready[0] if ready else None,'ready_frames':len(ready),'not_ready_frames':len(rows)-len(ready)}
report={'scope':'Fixed physical two-human source4 full swallow/enhanced/recovery lifecycle; passive frames equal unique original control','identity':identity,'input':{'path':str(inputs.relative_to(ROOT)),'sha256':sha(inputs.read_bytes())},'control':{'path':str(baseline.relative_to(ROOT)),'sha256':sha(baseline.read_bytes())},'frames':5502,'frames_match_original_core':True,'scene':header,'header_before_video_step':summarize_headers(header_before),'header_after_video_step':summarize_headers(header_after),'boost_source_first':boost[0],'boost_source_last':boost[-1],'source_restored_after':boost[-1]+1,'enhanced_active_first':enhanced[0],'enhanced_active_last':enhanced[-1],'child_cleared_after':enhanced[-1]+1,'first_negative_energy':negative[0],'first_normal_stand_after_boost':normal[0],'actions':dict(counts),'state_changes':rows,'capture_sha256':sha(Path(__file__).read_bytes()),'manual_state_writes':False,'restore_or_rng_injection':False,'independent_go_world_acceptance':False}
(out/'source4-special-discovery-duo-lifecycle.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps({k:v for k,v in report.items() if k!='state_changes'},indent=2),flush=True)
