"""Whole original World wrapper oracle; PCs/cost decomposition remain offline."""
import argparse,gzip,json,re
from pathlib import Path
from reference import ROOT,sha
F=896040;L=3420
def signed(v):return v if v<0x80000000 else v-0x100000000
def flags(e):return {'TransferActive':bool(e['world_flags']&1),'WorldActive':bool(e['world_flags']&128),'MissedWorld':bool(e['world_flags']&64)}
def epoch(e,first):return(e['frame_index']-first)*F
def clock(e,first):return {'cycle':epoch(e,first)+e['cycle'],'refresh':epoch(e,first)+signed(e['refresh'])}
def context(e,first):
 phase=3 if e['line']else 1 if e['cycle']<788 else 2
 return {'FrameOrigin':epoch(e,first),'Video':{'Phase':phase,'Line':e['line']//L,'Counter':e['counter'],'Reload':e['reload'],'HorizontalEnabled':bool(e['reg0']&16),'VerticalEnabled':bool(e['reg1']&32),'DisplayEnabled':bool(e['reg1']&64),'HorizontalPending':bool(e['hpending']),'VerticalPending':bool(e['vpending'])},'Pin':e['pin'],'Mask':e['mask'],'Phase':e['phase']//2}
def transfer(words):
 assert len(words)==8 and [w>>8 for w in words[:6]]==[0x93,0x94,0x95,0x96,0x97,0x8f]
 code=(words[6]>>14)&3|(words[7]>>2)&0x3c;target={1:1,3:2,5:3}[code&15];assert code&0x20
 return {'Words':((words[0]&255)|((words[1]&255)<<8))or 65536,'SourceWord':(words[2]&255)|((words[3]&255)<<8)|((words[4]&255)<<16),'Step':words[5]&255,'Target':target,'Address':(words[6]&0x3fff)|((words[7]&3)<<14)}
def boundaries(e,nxt,first):
 origin=epoch(e,first);line=e['line']//L;phase=1 if line==0 and e['cycle']<788 else 2
 end_origin=epoch(nxt,first);end_line=nxt['line']//L;end_phase=1 if end_line==0 and nxt['cycle']<788 else 2;out=[]
 while(origin,line,phase)!=(end_origin,end_line,end_phase):
  if line==0 and phase==1:out.append(origin+788);phase=2
  else:
   out.append(origin+(line+1)*L);line+=1
   if line==262:origin+=F;line=0;phase=1
  assert len(out)<600,('invalid original video progression',e,nxt)
 return out
def dma_end(e,at,desc,first):
 origin=epoch(e,first);line=e['line']//L;remaining=desc['Words'];rate=102 if desc['Target']==1 else 198;continued=False
 while True:
  boundary=37*L if line<37 else(line+1)*L;cycles=boundary-at;count=((cycles&0xffffffff)*rate&0xffffffff)//L
  if remaining<count:count=remaining;cycles=count*L//rate
  remaining-=count;end=at+cycles
  if not remaining:return origin+end,continued
  continued=True;line+=1
  if line==262:origin+=F;line=0
  at=line*L
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True)
 src=ROOT/'games/yuyuhakusho/verification/world-dma/native-world-dma.json.gz';d=json.loads(gzip.decompress(src.read_bytes()));rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();tablepath=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k/m68kops.h';entries=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',tablepath.read_text());cases=[];costs={}
 for index,row in enumerate(d['cases']):
  events=[dict(zip(d['fields'],e))for e in row['events']];cpu=[e for e in events if e['kind']in[1,10]];first,last=cpu[0],cpu[-1];origin_frame=first['frame_index'];transfers=[transfer(row['transfers'][i:i+8])for i in range(0,len(row['transfers']),8)];item=0;aux=[];bus=[];io=[]
  for e,nxt in zip(cpu,cpu[1:]):
   pc=e['pc'];raw=rom[pc:pc+4];op=int.from_bytes(raw[:2],'big');entry=[x for x in entries if op&int(x[1],16)==int(x[2],16)][-1];handler=entry[0];cost=int(entry[3]);internal=0
   if re.match(r'm68k_op_b(eq|ne|pl|gt|hi)_',handler):
    length=2 if op&255 else 4
    if nxt['pc']==pc+length:cost+=-2 if length==2 else 2
   if handler in['m68k_op_movem_32_re_pd','m68k_op_movem_32_er_pi']:internal=int.from_bytes(raw[2:4],'big').bit_count()*8;cost+=internal
   at=e['cycle']+(14 if signed(e['cycle'])>=signed(e['refresh'])else 0);after=epoch(e,origin_frame)+at+cost*7
   if pc in[0x32ec,0x3338,0x2e6c]:after+=7
   if pc in[0x688,0x3026]:
    desc=transfers[item]if pc==0x688 else {'Words':64,'Target':2}
    if pc==0x688:item+=1
    end,continued=dma_end(e,at,desc,origin_frame);after=end+(0 if continued else cost*7)
   if pc==0x68e and nxt['pc']==0x2e5e:
    phase=(at//7)%10;after+=(50 if phase==0 else 60-phase)*7
   delta=clock(nxt,origin_frame)['cycle']-after;assert delta>=0,(index,hex(pc),handler,delta,e,nxt)
   targets=boundaries(e,nxt,origin_frame)
   if pc in[0x32e2,0x32fe,0x332e,0x334a,0x2e62,0x2e7e]:
    bus.append({'At':epoch(e,origin_frame)+at,'Asserted':pc in[0x32e2,0x332e,0x2e62],'Delay':delta,'HeldBefore':e['bus_state']&3==3,'FrontierAfter':epoch(nxt,origin_frame)+signed(nxt['aux_cycle'])});delta=0
   for i,target in enumerate(targets):aux.append({'Target':target,'Delay':delta if i==0 else 0})
   if not targets:assert delta==0,('unpriced same-batch cost',index,hex(pc),handler,delta)
   costs[hex(pc)]={'handler':handler,'base':int(entry[3]),'internal':internal}
  for e in events:
   if e['kind']not in[2,4]:continue
   address=e['address'];x={'At':epoch(e,origin_frame)+e['cycle'],'Write':e['kind']==4,'Width':e['width'],'Value':e['value']}
   if address&0xffff00==0xa11100:x.update(Kind='bus-control',Value=e['value']&1)
   elif address==0xa01fff:x['Kind']='transfer-gate'
   elif e['kind']==2:x.update(Kind='vdp-status',Value=e['value']&0x88)
   elif address&0x1c==0:x['Kind']='palette-data'
   else:x['Kind']='vdp-control'
   io.append(x)
  # These layout values are the actual battle configuration used by046a;
  # derived IRQ/display/reload bits remain with their canonical live owners.
  layout={'Planes':[0xc000,0xe000],'Window':0xd000,'Sprites':0xf400,'HScroll':0xf000,'PlaneSize':1,'WindowHorizontal':0,'WindowVertical':6,'CounterBase':0}
  world={'Palette':{'Flags':first['flags']},'SceneMode':52,'SceneRequest':0,'VideoMode':2,'InterruptMode':4,'ScrollMode':3}
  cases.append({'ID':('private-'+str(index)if 'private'in row else'natural-'+str(row['frame'])),'Clock':clock(first,origin_frame),'Context':{'IRQ':context(first,origin_frame),'DMAWindows':None},'Graphics':{'Pending':transfers,'UploadPending':bool(first['transfer_flag']&128),'CachedDisplay':bool(first['control_word']&64),'SavedColors':[first['saved_color'],0],'PreviousVideoMode':2,'Layout':layout},'World':world,'Flags':flags(first),'Pause':first['pause'],'Held':first['bus_state']&3==3,'Blocked':bool(first['blocked']),'Frontier':signed(first['aux_cycle']),'CPU':[{'Clock':clock(e,origin_frame),'Context':context(e,origin_frame),'Flags':flags(e)}for e in cpu[:-1]],'IO':io,'Auxiliary':aux,'Bus':bus,'End':{'Clock':clock(last,origin_frame),'Context':context(last,origin_frame),'Flags':flags(last),'Blocked':bool(last['blocked']),'Held':last['bus_state']&3==3,'AwaitingLogic':last['pc']==0x3a2e,'BusyReturned':last['pc']==0xfff136,'QueueConsumed':last['transfer_end']==0xd400 and not bool(first['world_flags']&128),'Frames':last['frame_index']-first['frame_index']}})
 raw=(json.dumps({'version':1,'cases':cases},separators=(',',':'))+'\n').encode();fixture=out/'portable-world-clock.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(cases),'cpu_steps':sum(len(c['CPU'])for c in cases),'io':sum(len(c['IO'])for c in cases),'auxiliary_inputs':sum(len(c['Auxiliary'])for c in cases),'scope':'Actual whole wrapper CPU/IO oracle, with observed per-batch auxiliary and bus feedback declared external. IRQ phases use original frame-relative coordinates; clocks/refresh exported absolute. No runtime PC/time program.','costs':costs,'fixture_sha256':sha(fixture.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [src,rompath,tablepath,Path(__file__)]}};(out/'export.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in ['pass','cases','cpu_steps','io','auxiliary_inputs']}))
if __name__=='__main__':main()
