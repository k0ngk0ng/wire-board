"""Portable original prefix oracle + explicitly external per-batch delay inputs."""
import argparse,gzip,json,re
from pathlib import Path
from reference import ROOT,sha

def video(e):return {'Phase':3,'Line':e['line']//3420,'Counter':e['counter'],'Reload':e['reload'],'HorizontalEnabled':bool(e['reg0']&16),'VerticalEnabled':bool(e['reg1']&32),'DisplayEnabled':bool(e['reg1']&64),'HorizontalPending':bool(e['hpending']),'VerticalPending':bool(e['vpending'])}
def context(e):return {'FrameOrigin':0,'Video':video(e),'Pin':e['pin'],'Mask':e['mask'],'Phase':e['phase']//2}
def clock(e):return {'cycle':e['cycle'],'refresh':e['refresh']}
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True)
 src=ROOT/'games/yuyuhakusho/verification/battle-sample-irq/native-battle-prefix.json.gz';data=json.loads(gzip.decompress(src.read_bytes()));rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();irqpath=ROOT/'games/yuyuhakusho/engine/testdata/world-frame-irq.json';irq=bytes.fromhex(json.loads(irqpath.read_text())['hex']);tablepath=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k/m68kops.h';entries=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',tablepath.read_text());cases=[];costs={}
 for index,row in enumerate(data['cases']):
  events=[dict(zip(data['fields'],e))for e in row['events']];first,last=events[0],events[-1];cpu=[e for e in events if e['kind']in[1,10]];aux=[];observed=[]
  for e,nxt in zip(cpu,cpu[1:]):
   pc=e['pc'];raw=irq[pc-0xfff000:pc-0xfff000+4]if pc>=0xfff000 else rom[pc:pc+4];opcode=int.from_bytes(raw[:2],'big');entry=[r for r in entries if opcode&int(r[1],16)==int(r[2],16)][-1];handler=entry[0];base=int(entry[3]);cost=base;internal=0
   if re.search(r'm68k_op_(asl|lsr)_\d+_s',handler):cost+=2*((((opcode>>9)-1)&7)+1)
   if re.match(r'm68k_op_b(hi|eq|ne|pl|gt|ge|le|cs|cc)_',handler):
    length=2 if opcode&255 else 4
    if nxt['pc']==pc+length:cost+=-2 if length==2 else 2
   if handler.startswith('m68k_op_db'):cost=10 if nxt['pc']!=pc+4 else 14 if handler=='m68k_op_dbf_16'else 12
   if handler=='m68k_op_movem_32_re_pd':internal=int.from_bytes(raw[2:4],'big').bit_count()*8;cost+=internal
   at=e['cycle']+(14 if e['cycle']>=e['refresh']else 0);after=at+cost*7;delta=nxt['cycle']-after
   assert delta>=0,(index,hex(pc),handler,delta)
   if nxt['line']!=e['line']:
    assert nxt['line']==e['line']+3420,(index,hex(pc),'multi-line gap');aux.append({'Target':e['line']+3420,'Delay':delta})
   else:assert delta==0,(index,hex(pc),'unpriced same-line effect',delta)
   costs.setdefault(hex(pc),{'source_pc':pc,'opcode':opcode,'handler':handler,'base_cycles':base,'internal_cycles':internal,'observed_cpu_cycles':set()})['observed_cpu_cycles'].add(cost)
   observed.append({'Clock':clock(e),'Context':context(e)})
  io=[]
  for e in events:
   if e['kind']not in[2,4]:continue
   address=e['address'];item={'At':e['cycle'],'Value':e['value'],'Width':e['width'],'Port':0,'Control':False,'Write':e['kind']==4}
   if address>=0xa10000 and address<0xa10010:
    offset=address&15;item.update(Kind='controller',Port=0 if offset in[3,9]else 1,Control=offset>=9)
   elif address&0x1c==8:item['Kind']='vertical'
   elif address&0x1c==0:item['Kind']='palette-data'
   elif e['width']==4:item['Kind']='palette-address'
   elif e['value']>>8==0x8a:item['Kind']='reload'
   elif e['value']>>8==0x81:item['Kind']='display'
   else:raise AssertionError(('unexpected IO',e))
   io.append(item)
  ports=[]
  for p in range(2):ports.append({'Count':first['ports'+str(p)],'Held':[first['held'+str(p*4+i)]if i<first['ports'+str(p)]else 0 for i in range(4)]})
  cases.append({'ID':('private-'+str(index)if 'private'in row else 'natural-'+str(row['frame'])),'Clock':clock(first),'Context':context(first),'Input':{'DisplayReady':bool(first['flags']&128),'RestoreColor':first['saved_color'],'Controllers':{'Ports':ports}},'CPU':observed,'IO':io,'Auxiliary':aux,'End':{'Clock':clock(last),'Context':context(last)}})
 for c in costs.values():c['observed_cpu_cycles']=sorted(c['observed_cpu_cycles'])
 raw=(json.dumps({'version':1,'cases':cases},separators=(',',':'))+'\n').encode();fixture=out/'portable-battle-prefix.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(cases),'cpu_steps':sum(len(c['CPU'])for c in cases),'io_operations':sum(len(c['IO'])for c in cases),'auxiliary_inputs':sum(len(c['Auxiliary'])for c in cases),'scope':'Clock/VDP prefix oracle with explicitly observed external line-end main delay feedback. Does not prove independent PCM/DMA feedback. Source PCs/opcodes are provenance only.','costs':costs,'fixture_sha256':sha(fixture.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [src,rompath,irqpath,tablepath,Path(__file__)]}};(out/'portable-export.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in ['pass','cases','cpu_steps','io_operations','auxiliary_inputs']}))
if __name__=='__main__':main()
