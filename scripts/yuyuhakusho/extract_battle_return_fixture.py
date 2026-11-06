"""Portable complete return oracle; no source PCs are runtime inputs."""
import argparse,gzip,json
from pathlib import Path
from reference import ROOT,sha

def clock(e):return {'cycle':e['cycle'],'refresh':e['refresh']}
def context(e):return {'FrameOrigin':0,'Video':{'Phase':3,'Line':e['line']//3420,'Counter':e['counter'],'Reload':e['reload'],'HorizontalEnabled':bool(e['reg0']&16),'VerticalEnabled':bool(e['reg1']&32),'DisplayEnabled':bool(e['reg1']&64),'HorizontalPending':bool(e['hpending']),'VerticalPending':bool(e['vpending'])},'Pin':e['pin'],'Mask':e['mask'],'Phase':e['phase']//2}
def entry_delay(at):
 phase=(at//7)%10;return (50 if phase==0 else 60-phase)*7

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);p.add_argument('--private',type=Path,default=ROOT/'games/yuyuhakusho/verification/battle-irq-return/native-battle-return.json.gz');p.add_argument('--natural',type=Path,default=ROOT/'games/yuyuhakusho/verification/battle-irq-return/native-battle-return-natural.json.gz');a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True);cases=[];bindings={}
 for kind,path in [('private',a.private.resolve()),('natural',a.natural.resolve())]:
  d=json.loads(gzip.decompress(path.read_bytes()));bindings[str(path.relative_to(ROOT))]=sha(path.read_bytes())
  for index,row in enumerate(d['cases']):
   es=[dict(zip(d['fields'],e))for e in row['events']];first,last=es[0],es[-1];saved=row['input']['mask']if kind=='private'else last['mask'];cpu=[e for e in es if e['kind']in[1,10,11]];aux=[]
   for e,nxt in zip(cpu,cpu[1:]):
    pc=e['pc'];at=e['cycle']+(14 if e['cycle']>=e['refresh']else 0)
    costs={0xfff096:124,0xfff000:20,0xfff004:16,0xfff00a:10,0xfff09c:20,0xfff0a4:16,0xfff09a:20,0xfff0a8:20};assert pc in costs,hex(pc);after=at+costs[pc]*7
    rte=pc in[0xfff09a,0xfff0a8];immediate=rte and e['pin']>saved
    if immediate:after+=entry_delay(at)
    if kind=='private'and rte and not immediate and nxt['pc']==0xfff000:after+=entry_delay(after)
    delta=nxt['cycle']-after;assert delta>=0,(kind,index,hex(pc),delta)
    if nxt['line']!=e['line']:
     assert nxt['line']==e['line']+3420
     aux.append({'Target':e['line']+3420,'Delay':delta})
    else:assert delta==0,(kind,index,hex(pc),delta)
   io=[]
   for e in es:
    if e['kind']in[2,4]:assert e['address']&0x1c==4 and e['value']==0x8adf;io.append({'At':e['cycle'],'Kind':'reload','Write':True,'Value':e['value'],'Width':2})
   cases.append({'ID':kind+'-'+str(index),'Clock':clock(first),'Context':context(first),'CallerMask':saved,'CPU':[{'Clock':clock(e),'Context':context(e)}for e in cpu[:-1]],'IO':io,'Auxiliary':aux,'End':{'Clock':clock(last),'Context':context(last)},'PendingWorld':last['kind']==11})
 raw=(json.dumps({'version':1,'cases':cases},separators=(',',':'))+'\n').encode();p=out/'portable-battle-return.json.gz';p.write_bytes(gzip.compress(raw,mtime=0));r={'pass':True,'cases':len(cases),'private':1872,'natural':23,'pending_world':sum(c['PendingWorld']for c in cases),'cpu_steps':sum(len(c['CPU'])for c in cases),'external_aux_inputs':sum(len(c['Auxiliary'])for c in cases),'scope':'Private declared zero background or natural observed line-delay feedback; return ordering and pin/mask/video outcomes. No in-ack hook, CPU/stack/program in production.','fixture_sha256':sha(p.read_bytes()),'sources':bindings,'extractor_sha256':sha(Path(__file__).read_bytes())};(out/'export.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps(r))
if __name__=='__main__':main()
