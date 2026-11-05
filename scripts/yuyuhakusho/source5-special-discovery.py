#!/usr/bin/env python3
"""Record physical source5 inputs chosen from visible state; fixed cold replay required."""
import ctypes as C
import json
from pathlib import Path
from reference import ROOT, ReferenceSession, buttons_at, sha

OUT=ROOT/'.local/md-yuyuhakusho/source5-discovery'
OUT.mkdir(parents=True,exist_ok=True)
menu=[e for e in json.loads((ROOT/'docs/research/md-yuyuhakusho/source5-next-inputs.json').read_text()) if e['start']<1800]
# Every constructor must meet its actual resource gate, not just be entered.
TASKS=[('r1',0x19240,16),('r1down',0x19284,16),('r3',0x19216,8),('r4a',0x192c8,4),('r4b',0x192f6,4),('r4c',0x19324,4),('air4',0x19364,4),('r6a',0x193a4,50),('r6b',0x193bc,50),('r6c',0x193d4,50)]
FOLLOW={0x1926a,0x192ae,0x19408,0x19426,0x19444,0x1414e,0x143e4,0x146ba}
points=sorted({p for _,p,_ in TASKS}|FOLLOW)
bridge=C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'))
CB=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
bridge.yyh_set_observer.argtypes=[CB,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int
pc_array=(C.c_uint*len(points))(*points)
def word(a,i):return int.from_bytes(a[i:i+2],'big',signed=True)
def expand(parts):return [list(keys) for keys,n in parts for _ in range(n)]
index=0;success=False;queue=[];retry=1800;jumped=0;events=menu.copy();last=None;segment=None;hits=[];states=[];completed=[];errors=[]
core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
with ReferenceSession(ROOT,OUT,core=core) as ref:
 @CB
 def observe(k,width,pc,value):
  global success
  try:
   address=ref.get_register(13)&65535
   if not 0x8400<=address<0x8800:return
   ram=ref.ram_cpu();a=ram[0x8400:0x8500]
   if address==0x8400:
    hit={'frame':ref.frame,'pc':hex(pc),'hp':word(a,0x78),'energy':word(a,0xb8),'facing':a[0x61]};hits.append(hit)
    if index<len(TASKS) and pc==TASKS[index][1] and word(a,0xb8)>=TASKS[index][2] and (not TASKS[index][0].startswith('r6') or word(a,0x78)<=111):success=True
   elif pc in FOLLOW and (not hits or hits[-1].get('pc')!=hex(pc) or hits[-1].get('object')!=hex(address)):hits.append({'frame':ref.frame,'pc':hex(pc),'object':hex(address)})
  except Exception as exc:errors.append(repr(exc))
 assert bridge.yyh_set_observer(observe,pc_array,len(points))
 hook=ref.api('set_cpu_hook',None,C.c_void_p);hook(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
 for frame in range(1,8003):
  ram=ref.ram_cpu();a=ram[0x8400:0x8500];keys=[]
  if frame<1800:buttons=buttons_at(menu,frame)
  else:
   if success:
    completed.append({'task':TASKS[index][0],'frame':frame-1});print('success',completed[-1],flush=True);index+=1;success=False;retry=frame+35;jumped=0
   if queue:keys=queue.pop(0)
   elif index<len(TASKS) and frame>=retry and a[0x58]==5 and int.from_bytes(a[4:6],'big')==256:
    name,pc,cost=TASKS[index];forward=6 if a[0x61]!=0 else 7;ground=bool(a[0x6c]&8);air=bool(a[0x6c]&16);child=int.from_bytes(ram[0x8504:0x8506],'big')
    if word(a,0xb8)<cost:keys=[0]
    elif name.startswith('r6') and word(a,0x78)>111:keys=[]
    elif name=='air4' and not air:
     if ground and frame-jumped>80:queue=expand([([4],2),([],8)]);jumped=frame
    elif (air if name=='air4' else ground) and (not(name.startswith('r4') or name.startswith('r6') or name=='air4') or child==0):
     if name.startswith('r1'):queue=expand([([],3),([0,1]+([5]if name=='r1down'else[]),4),([],3)])
     elif name=='r3':queue=expand([([5],2),([],2),([5],2),([],1),([0],4),([],3)])
     elif name.startswith('r4') or name=='air4':
      b=1 if name.endswith('a') or name=='air4' else 0 if name.endswith('b') else 8
      queue=expand([([5],2),([5,forward],2),([forward],2),([],1),([b],4),([],3)])
     else:
      b=1 if name.endswith('a') else 0 if name.endswith('b') else 8
      queue=expand([([5],2),([],2),([5],2),([forward],2),([],1),([b],4),([],3)])
     retry=frame+85
    if queue:keys=queue.pop(0)
   buttons=[0]*8
   for b in keys:buttons[0]|=1<<b
   if keys!=last:
    if segment is not None:segment['end']=frame-1;events.append(segment)
    segment={'start':frame,'end':frame,'buttons':keys.copy()}if keys else None;last=keys.copy()
  ref.step(frame,buttons);ram=ref.ram_cpu();a=ram[0x8400:0x8500]
  if frame>=1800 and (frame%100==0 or success):states.append({'frame':frame,'task_index':index,'hp':word(a,0x78),'energy':word(a,0xb8),'action':hex(a[0x5d]),'pose':hex(a[0x5f]),'stages':hex(a[0x6c]),'x':word(a,0x40),'y':word(a,0x44),'facing':a[0x61],'child_size':int.from_bytes(ram[0x8504:0x8506],'big')})
  if frame>1800 and word(a,0x78)<=0:break
  if index==len(TASKS) and frame>completed[-1]['frame']+400:break
 hook(None);identity=ref.identity;ref.screenshot(OUT/'final.png')
if segment is not None:segment['end']=frame;events.append(segment)
events.sort(key=lambda e:e['start']);path=OUT/'inputs.json';path.write_text(json.dumps(events,indent=2)+'\n')
r={'scope':'Adaptive real physical inputs only; no RAM/register/request/health/energy/seed edits. Fixed cold replay pending.','identity':identity,'frames':frame,'inputs_sha256':sha(path.read_bytes()),'completed':completed,'hits':hits,'states':states,'errors':errors,'cold_fixed_replay_verified':False}
(OUT/'report.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({'frames':frame,'completed':completed,'last':states[-1]if states else None,'errors':errors},indent=2),flush=True)
