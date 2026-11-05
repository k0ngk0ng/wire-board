#!/usr/bin/env python3
"""Record only real physical buttons chosen from visible original state; cold replay follows."""
import ctypes as C,json,sys
from collections import Counter
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
out=ROOT/'.local/md-yuyuhakusho/source4-special-discovery/active';out.mkdir(parents=True,exist_ok=True)
menu=[e for e in json.loads((ROOT/'docs/research/md-yuyuhakusho/source4-next-inputs.json').read_text()) if e['start']<1800]
tasks=[('r1',0x1904e,8),('r2',0x190a8,32),('r3b',0x190d2,16),('r3a',0x190f0,24),('r4b',0x1911a,24),('r4a',0x1913c,24),('airb',0x191d4,16),('aira',0x191ec,24),('r6',0x19190,200)]
points=[r for _,r,_ in tasks]+[0x19092,0x191ba,0x13bb6,0x13e9a,0x13fc8];bridge=C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'));typ=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint);bridge.yyh_set_observer.argtypes=[typ,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int;pcs=(C.c_uint*len(points))(*points)
index=0;success=False;queue=[];retry_at=1800;last_keys=None;segment=None;events=menu.copy();hits=[];states=[];completed=[];jump_at=0;last_status=None
core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
def w(a,o):return int.from_bytes(a[o:o+2],'big',signed=True)
def seq(parts):return [buttons for buttons,n in parts for _ in range(n)]
with ReferenceSession(ROOT,out,core=core) as ref:
 @typ
 def observe(k,width,pc,value):
  global success
  address=ref.get_register(13)&65535
  if not 0x8400<=address<0x8800:return
  ram=ref.ram_cpu();a=ram[0x8400:0x8500]
  if address==0x8400:
   row={'frame':ref.frame,'pc':hex(pc),'hp':w(a,0x78),'energy':w(a,0xb8),'facing':a[0x61],'request':a[0x6b]};hits.append(row)
   if index<len(tasks) and pc==tasks[index][1] and w(a,0xb8)>=tasks[index][2] and (index!=8 or w(a,0x78)<=111 and int.from_bytes(ram[0x8504:0x8506],'big')==0):success=True
  elif not hits or hits[-1].get('pc')!=hex(pc):hits.append({'frame':ref.frame,'pc':hex(pc),'child':hex(address)})
 assert bridge.yyh_set_observer(observe,pcs,len(points));hook=ref.api('set_cpu_hook',None,C.c_void_p);hook(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
 for frame in range(1,6001):
  ram=ref.ram_cpu();a=ram[0x8400:0x8500];child=ram[0x8500:0x8600]
  keys=[]
  if frame<1800:buttons=buttons_at(menu,frame)
  else:
   if success:
    completed.append({'task':tasks[index][0],'frame':frame-1});index+=1;success=False;retry_at=frame+20;jump_at=0
   if queue:keys=queue.pop(0)
   elif index<len(tasks) and frame>=retry_at:
    name,pc,cost=tasks[index];forward=6 if a[0x61]!=0 else 7
    ground=bool(a[0x6c]&8);air=bool(a[0x6c]&16)
    if w(a,0xb8)<cost:keys=[0]
    elif name=='r6' and w(a,0x78)>111:keys=[]
    elif name.startswith('air') and not air:
     if ground and frame-jump_at>80:queue=seq([([4],2),([],8)]);jump_at=frame
    elif (air if name.startswith('air') else ground):
     if name=='r1':queue=seq([([forward],2),([],2)]*4+[([],3)])
     elif name=='r2':queue=seq([([],2),([0,1],4),([],3)])
     elif name.startswith('r3') or name.startswith('air'):
      b=0 if name.endswith('b') else 1;queue=seq([([5],2),([5,forward],2),([forward],2),([],1),([b],4),([],3)])
     elif name.startswith('r4'):
      b=0 if name.endswith('b') else 1;queue=seq([([5],2),([],2),([5],2),([],1),([b],4),([],3)])
     else:queue=seq([([5],2),([forward],2),([5],2),([forward],2),([],1),([0],4),([],3)])
     retry_at=frame+60
    if queue:keys=queue.pop(0)
   elif index>=len(tasks) and a[0x5d]==0x2a:
    dx=w(a,0x40)-w(child,0x40);dy=w(a,0x44)-32-w(child,0x44)
    if abs(dx)>12:keys.append(7 if dx>0 else 6)
    if abs(dy)>8:keys.append(5 if dy>0 else 4)
   buttons=[0]*8
   for b in keys:buttons[0]|=1<<b
   if keys!=last_keys:
    if segment is not None:segment['end']=frame-1;events.append(segment)
    segment={'start':frame,'end':frame,'buttons':keys.copy()} if keys else None;last_keys=keys.copy()
  ref.step(frame,buttons);ram=ref.ram_cpu();a=ram[0x8400:0x8500]
  status=(index,a[0x5d],w(a,0x78),w(a,0xb8))
  if frame>=1800 and (status!=last_status or frame%100==0):states.append({'frame':frame,'task_index':index,'hp':w(a,0x78),'energy':w(a,0xb8),'action':hex(a[0x5d]),'pose':hex(a[0x5f]),'stages':hex(a[0x6c]),'facing':a[0x61],'x':w(a,0x40),'y':w(a,0x44),'child_size':int.from_bytes(ram[0x8504:0x8506],'big'),'movement_source':hex(int.from_bytes(a[0x90:0x94],'big'))});last_status=status
  if frame>1800 and w(a,0x78)<=0:break
  if any(h['pc']=='0x13fc8' for h in hits):break
 hook(None);identity=ref.identity;ref.screenshot(out/'source4-special-discovery-active-final.png')
if segment is not None:segment['end']=frame;events.append(segment)
events.sort(key=lambda e:e['start']);p=out/'source4-special-discovery-active-inputs.json';p.write_text(json.dumps(events,indent=2)+'\n')
r={'scope':'Adaptive original-state physical buttons only, recorded for mandatory fixed cold replay; no state injection','identity':identity,'input_sha256':sha(p.read_bytes()),'frames':frame,'completed':completed,'hits':hits,'states':states,'cold_fixed_replay_verified':False};(out/'source4-special-discovery-active-report.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({'frames':frame,'completed':completed,'last':states[-1],'hits':hits[-15:]},indent=2))
