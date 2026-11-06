"""World IRQ entry oracle with declared original bus and line feedback."""
import argparse,gzip,json,re
from pathlib import Path
from reference import ROOT,sha
from extract_battle_prefix_fixture import context,clock

def flags(e):return {'TransferActive':bool(e['world_flags']&1),'WorldActive':bool(e['world_flags']&128),'MissedWorld':bool(e['world_flags']&64)}
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();src=a.capture.resolve();out=a.out.resolve();assert src.is_relative_to(ROOT)and out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True);d=json.loads(gzip.decompress(src.read_bytes()));program=bytes.fromhex(json.loads((ROOT/'games/yuyuhakusho/engine/testdata/world-frame-irq.json').read_text())['hex']);table=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k/m68kops.h';entries=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',table.read_text());cases=[];costs={}
 for index,row in enumerate(d['cases']):
  events=[dict(zip(d['fields'],e))for e in row['events']];cpu=[e for e in events if e['kind']in[1,10]];first,last=cpu[0],cpu[-1];aux=[];bus=[];io=[]
  for e,nxt in zip(cpu,cpu[1:]):
   pc=e['pc'];raw=program[pc-0xfff000:pc-0xfff000+4];op=int.from_bytes(raw[:2],'big');entry=[r for r in entries if op&int(r[1],16)==int(r[2],16)][-1];handler=entry[0];cost=int(entry[3]);internal=0
   if re.match(r'm68k_op_b(eq|ne|pl|gt|hi)_',handler):
    length=2 if op&255 else 4
    if nxt['pc']==pc+length:cost+=-2 if length==2 else 2
   if handler=='m68k_op_dbf_16':cost=10 if nxt['pc']!=pc+4 else 14
   if handler=='m68k_op_movem_32_re_pd':internal=int.from_bytes(raw[2:4],'big').bit_count()*8;cost+=internal
   at=e['cycle']+(14 if e['cycle']>=e['refresh']else 0);after=at+cost*7+(7 if pc==0xfff0ba else 0);delta=nxt['cycle']-after;assert delta>=0,(index,hex(pc),delta)
   if pc in[0xfff0b0,0xfff0cc]:
    bus.append({'At':at,'Asserted':pc==0xfff0b0,'Delay':delta if pc==0xfff0b0 else 0,'GrantedBefore':e['bus_state']==3,'FrontierAfter':nxt['aux_cycle']});delta=0 if pc==0xfff0b0 else delta
   if nxt['line']!=e['line']:
    assert nxt['line']==e['line']+3420;aux.append({'Target':e['line']+3420,'Delay':delta,'Held':nxt['bus_state']==3})
   else:assert delta==0,(index,hex(pc),'unpriced same-line effect',delta)
   costs[hex(pc)]={'handler':handler,'base':int(entry[3]),'internal':internal}
  for e in events:
   if e['kind']not in[2,4]:continue
   address=e['address'];x={'At':e['cycle'],'Write':e['kind']==4,'Width':e['width'],'Value':e['value']}
   if address&0xffff00==0xa11100:x.update(Kind='bus-control',Value=e['value']&1)
   elif address==0xa01fff:x['Kind']='transfer-gate'
   elif address&0x1c==8:x['Kind']='vertical'
   elif address&0x1c==0:x['Kind']='palette-data'
   elif e['width']==4:x['Kind']='palette-address'
   elif e['value']>>8==0x81:x['Kind']='display'
   else:raise AssertionError(e)
   io.append(x)
  cases.append({'ID':('natural-'+str(row['frame'])if 'private'not in row else 'private-'+str(index)),'Clock':clock(first),'Context':context(first),'Flags':flags(first),'DisplayReady':bool(first['flags']&128),'Pause':first['pause'],'Blocked':bool(first['blocked']),'Held':first['bus_state']==3,'Frontier':first['aux_cycle'],'CPU':[{'Clock':clock(e),'Context':context(e),'Flags':flags(e)}for e in cpu[:-1]],'IO':io,'Auxiliary':aux,'Bus':bus,'End':{'Clock':clock(last),'Context':context(last),'Flags':flags(last),'Blocked':bool(last['blocked']),'Held':last['bus_state']==3}})
 raw=(json.dumps({'version':1,'cases':cases},separators=(',',':'))+'\n').encode();p=out/'portable-world-entry.json.gz';p.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(cases),'cpu_steps':sum(len(c['CPU'])for c in cases),'io':sum(len(c['IO'])for c in cases),'costs':costs,'fixture_sha256':sha(p.read_bytes()),'scope':'Given actual world IRQ entry and observed bus/line feedback. Not whole-world or independent background timing.','sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [src,table,Path(__file__)]}};(out/'export.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in ['pass','cases','cpu_steps','io']}))
if __name__=='__main__':main()
