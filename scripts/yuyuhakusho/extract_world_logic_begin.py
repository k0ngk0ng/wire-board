"""Portable actual3a2e prelude, normalization and nested-VINT oracle."""
import argparse,gzip,json,re
from pathlib import Path
from reference import ROOT,sha
from extract_world_clock import signed,clock,context,flags,epoch,boundaries
def corrected_scene(row):
 scene=dict(row['scene'])
 if 'VideoMode'not in scene:
  window=bytes.fromhex(row['before']['clock']['window']);scene['VideoMode']=scene['SceneMode'];scene['SceneMode']=int.from_bytes(window[48:50],'big');scene['SceneRequest']=int.from_bytes(window[50:52],'big')
 return scene
def world(s,scene):
 return dict(Clock=s['clock']['state'],Controllers=s['controllers'],RandomSeed=s['seed'],SlowGate=s['slow_gate'],ResetEnabled=s['reset_enabled'],Palette=s['palette'],**scene)
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();src=a.capture.resolve();out=a.out.resolve();assert src.is_relative_to(ROOT)and out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True);d=json.loads(gzip.decompress(src.read_bytes()));rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();tablepath=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k/m68kops.h';table=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',tablepath.read_text());cases=[];costs={};total_aux=0
 for index,row in enumerate(d['cases']):
  scene=corrected_scene(row)
  for key in ['graphics_before','graphics_after']:
   if 'PreviousVideoMode'not in row[key]:row[key]['PreviousVideoMode']=row['scene']['SceneRequest']
  events=[dict(zip(d['fields'],e))for e in row['events']];cpu=[e for e in events if e['kind']in[1,10]];first,last=cpu[0],cpu[-1];origin=first['frame_index'];feedback={};last_cpu=None;sync_delays={}
  for e in events:
   if e['kind']==1:last_cpu=e
   if e['kind']==20:
    assert last_cpu is not None and e['frame_index']==last_cpu['frame_index']and e['line']==last_cpu['line']
    if e['address']:
     key=(last_cpu['frame_index'],last_cpu['cycle'],last_cpu['pc']);sync_delays[key]=sync_delays.get(key,0)+e['value']
    else:
     target=epoch(e,origin)+last_cpu['batch'];assert last_cpu['batch']in[788,last_cpu['line']+3420]
     feedback[target]=feedback.get(target,0)+e['value'];total_aux+=e['value']
  aux=[];bus=[];io=[]
  for e,nxt in zip(cpu,cpu[1:]):
   pc=e['pc'];raw=rom[pc:pc+4];op=int.from_bytes(raw[:2],'big');entry=[r for r in table if op&int(r[1],16)==int(r[2],16)][-1];handler=entry[0];cost=int(entry[3]);internal=0
   if re.match(r'm68k_op_b(eq|ne|pl|gt|hi|cs|cc|mi)_',handler):
    size=2 if op&255 else 4
    if nxt['pc']==pc+size:cost+=-2 if size==2 else 2
   if re.search(r'm68k_op_(asl|lsr|lsl)_\d+_s',handler):cost+=2*((((op>>9)-1)&7)+1)
   if handler in['m68k_op_movem_32_re_pd','m68k_op_movem_32_er_pi']:internal=int.from_bytes(raw[2:4],'big').bit_count()*8;cost+=internal
   if handler=='m68k_op_divu_16_d':
    c=row['before']['clock']['state'];amplitude=c['amplitude_request']or c['amplitude'];duration=c['duration_request'];assert duration;cost=136-2*(amplitude//(2*duration)).bit_count();internal=cost
   at=e['cycle']+(14 if signed(e['cycle'])>=signed(e['refresh'])else 0);after=epoch(e,origin)+at+cost*7
   if pc in[0x2e6c,0x2fa0]:after+=7
   if pc==0x3026:after=epoch(e,origin)+at+(64*3420//198)+cost*7
   targets=boundaries(e,nxt,origin);batch_delay=sum(feedback.get(t,0)for t in targets);after+=batch_delay
   key=(e['frame_index'],e['cycle'],pc);request_delay=sync_delays.get(key,0);after+=request_delay
   if nxt['pc']==0x2e5e:
    relative=after-epoch(nxt,origin);phase=(relative//7)%10;after+=(50 if phase==0 else 60-phase)*7
   delta=clock(nxt,origin)['cycle']-after;assert delta==0,('unpriced frame-prefix cost',index,hex(pc),handler,delta,e,nxt)
   for target in targets:aux.append({'Target':target,'Delay':feedback.get(target,0)})
   if pc in[0x2e62,0x2e7e,0x2f96,0x2fb2]:bus.append({'At':epoch(e,origin)+at,'Asserted':pc in[0x2e62,0x2f96],'Delay':request_delay,'HeldBefore':e['bus_state']&3==3,'FrontierAfter':epoch(nxt,origin)+signed(nxt['aux_cycle'])})
   costs[hex(pc)]={'handler':handler,'base':int(entry[3]),'internal':internal}
  assert set(feedback).issubset({x['Target']for x in aux})
  for e in events:
   if e['kind']not in[2,4]:continue
   address=e['address'];x={'At':epoch(e,origin)+e['cycle'],'Write':e['kind']==4,'Width':e['width'],'Value':e['value']}
   if address&0xffff00==0xa11100:x.update(Kind='bus-control',Value=e['value']&1)
   elif address==0xa01fff:x['Kind']='transfer-gate'
   elif e['kind']==2:x.update(Kind='vdp-status',Value=e['value']&0x88)
   elif address&0x1c==0:x['Kind']='palette-data'
   else:x['Kind']='vdp-control'
   io.append(x)
  dma=None
  if first['dma_end']and first['dma_end']>signed(first['aux_cycle']):dma=[{'Begin':0,'End':first['dma_end'],'Mode':first['dma_type'],'Remaining':first['dma_remaining']}]
  cases.append({'ID':f"natural-{row['frame']}-{index}",'Clock':clock(first,origin),'Context':{'IRQ':context(first,origin),'DMAWindows':dma,'NormalizedDevices':row['devices']},'Graphics':row['graphics_before'],'Flags':flags(first),'World':world(row['before'],scene),'Input':row['normalization'],'Held':first['bus_state']&3==3,'Blocked':bool(first['blocked']),'Frontier':signed(first['aux_cycle']),'CPU':[{'Clock':clock(e,origin),'Context':context(e,origin),'Flags':flags(e)}for e in cpu[:-1]],'IO':io,'Auxiliary':aux,'Bus':bus,'End':{'Clock':clock(last,origin),'Context':context(last,origin),'Flags':flags(last),'World':world(row['after'],scene),'Graphics':row['graphics_after'],'Frames':last['frame_index']-first['frame_index']}})
 raw=(json.dumps({'Version':1,'Cases':cases},separators=(',',':'))+'\n').encode();fixture=out/'portable-logic-begin.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(cases),'steps':sum(len(c['CPU'])for c in cases),'io':sum(len(c['IO'])for c in cases),'auxiliary_inputs':sum(len(c['Auxiliary'])for c in cases),'actual_auxiliary_delay':total_aux,'cross_frame_cases':sum(c['End']['Frames']for c in cases),'costs':costs,'fixture_sha256':sha(fixture.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [src,rompath,tablepath,Path(__file__)]},'scope':'Actual logical frame prelude through3a5a; original bank contention measured separately and explicitly external to main CPU oracle. Native snapshots and event bytes unchanged.'};(out/'export.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in ['pass','cases','steps','io','auxiliary_inputs','actual_auxiliary_delay','cross_frame_cases']}),flush=True)
if __name__=='__main__':main()
