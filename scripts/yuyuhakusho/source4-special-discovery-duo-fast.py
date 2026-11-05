#!/usr/bin/env python3
"""Two actual human controller ports: recorded damage, then source4 dragon control."""
import ctypes as C,json
from pathlib import Path
from reference import ROOT,ReferenceSession,buttons_at,sha
out=ROOT/'.local/md-yuyuhakusho/source4-special-discovery/duo-fast';out.mkdir(parents=True,exist_ok=True)
menu=json.loads((ROOT/'docs/research/md-yuyuhakusho/source4-special-discovery-duo-menu-inputs.json').read_text());menu=[dict(e,port=e.get('port',0)) for e in menu]
points=[0x19190,0x191ba,0x13bb6,0x13e9a,0x13fc8];bridge=C.CDLL(str(ROOT/'.local/md-yuyuhakusho/tools/hook_filter.dylib'));typ=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint);bridge.yyh_set_observer.argtypes=[typ,C.POINTER(C.c_uint),C.c_size_t];bridge.yyh_set_observer.restype=C.c_int;pcs=(C.c_uint*len(points))(*points)
def w(a,o):return int.from_bytes(a[o:o+2],'big',signed=True)
def seq(parts):return [b for b,n in parts for _ in range(n)]
events=menu.copy();segments=[None,None];lastkeys=[None,None];queue=[];cast=False;lane_started=False;lane_time=0;steer_start=0;hits=[];samples=[];last=None;retry=0
core=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
with ReferenceSession(ROOT,out,core=core) as ref:
 @typ
 def observe(k,width,pc,value):
  global cast
  address=ref.get_register(13)&65535;ram=ref.ram_cpu();a=ram[0x9000:0x9100]
  if address in range(0x9000,0x9400) or pc==0x13e9a:
   if not hits or hits[-1]['pc']!=hex(pc) or hits[-1]['object']!=hex(address):hits.append({'frame':ref.frame,'pc':hex(pc),'object':hex(address),'hp':w(a,0x78),'energy':w(a,0xb8),'owner48':hex(int.from_bytes(ram[address+0x48:address+0x4a],'big'))})
   if pc==0x19190 and address==0x9000 and w(a,0x78)<=111 and w(a,0xb8)>=200:cast=True
 assert bridge.yyh_set_observer(observe,pcs,len(points));hook=ref.api('set_cpu_hook',None,C.c_void_p);hook(C.cast(bridge.yyh_cpu_hook,C.c_void_p))
 for frame in range(1,6001):
  ram=ref.ram_cpu();p1=ram[0x8400:0x8500];p2=ram[0x9000:0x9100];child=ram[0x9100:0x9200];keys=[[],[]]
  if frame<2350:buttons=buttons_at(menu,frame)
  else:
   if w(p2,0x78)>111:
    dx=w(p2,0x40)-w(p1,0x40)
    if abs(dx)>22:keys[0]=[7 if dx>0 else 6]
    elif frame%18<3:keys[0]=[0]
   else:
    if not lane_started and p1[0x6c]&1:keys[0]=[8,4];lane_started=True;lane_time=frame
    if not cast and frame>lane_time+1 and w(p1,0x9c)!=w(p2,0x9c):
     if queue:keys[1]=queue.pop(0)
     elif p2[0x6c]&8 and frame>=retry:
      forward=6 if p2[0x61]!=0 else 7
      queue=seq([([5],2),([forward],2),([5],2),([forward],2),([],1),([0],4),([],3)]);keys[1]=queue.pop(0);retry=frame+40
    elif cast and p2[0x5d]==0x2a:
     if not steer_start:steer_start=frame
     tick=frame-steer_start
     if tick<16:keys[1]=[4]
     elif tick<32:keys[1]=[6]
     else:
      dx=w(p2,0x40)-w(child,0x40);dy=w(p2,0x44)-32-w(child,0x44)
      if abs(dx)>10:keys[1].append(7 if dx>0 else 6)
      if abs(dy)>6:keys[1].append(5 if dy>0 else 4)
   buttons=[0]*8
   for port in range(2):
    for b in keys[port]:buttons[port]|=1<<b
    if keys[port]!=lastkeys[port]:
     if segments[port] is not None:segments[port]['end']=frame-1;events.append(segments[port])
     segments[port]={'start':frame,'end':frame,'buttons':keys[port].copy(),'port':port} if keys[port] else None;lastkeys[port]=keys[port].copy()
  ref.step(frame,buttons);ram=ref.ram_cpu();p2=ram[0x9000:0x9100];p1=ram[0x8400:0x8500];child=ram[0x9100:0x9200]
  status=(w(p2,0x78),p2[0x5d],int.from_bytes(p2[0x90:0x94],'big'),int.from_bytes(child[4:6],'big'))
  if frame>=2350 and (status!=last or frame%50==0):samples.append({'frame':frame,'hp':w(p2,0x78),'energy':w(p2,0xb8),'action':hex(p2[0x5d]),'pose':hex(p2[0x5f]),'facing':p2[0x61],'stages':hex(p2[0x6c]),'x':w(p2,0x40),'y':w(p2,0x44),'ground':w(p2,0x9c),'p1ground':w(p1,0x9c),'child':{'code':child[:4].hex(),'size':int.from_bytes(child[4:6],'big'),'x':w(child,0x40),'y':w(child,0x44),'angle':w(child,0x48)},'movement_source':hex(int.from_bytes(p2[0x90:0x94],'big'))});last=status
  if any(h['pc']=='0x13fc8' for h in hits):break
 hook(None);identity=ref.identity;ref.screenshot(out/'source4-special-discovery-duo-final.png')
for port in range(2):
 if segments[port] is not None:segments[port]['end']=frame;events.append(segments[port])
events.sort(key=lambda e:(e['start'],e.get('port',0)));p=out/'source4-special-discovery-duo-inputs.json';p.write_text(json.dumps(events,indent=2)+'\n')
r={'scope':'Legitimate two-human practice scene, channel0 source3 attacks channel1 source4 then stops; no state injection','identity':identity,'input_sha256':sha(p.read_bytes()),'frames':frame,'hits':hits,'states':samples,'cast':cast,'cold_fixed_replay_verified':False};(out/'source4-special-discovery-duo-report.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({'frames':frame,'cast':cast,'hits':hits[-12:],'last':samples[-1]},indent=2))
