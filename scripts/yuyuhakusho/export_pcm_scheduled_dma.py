"""Portable actual PCM/physical-port ledger plus explicit live VDP DMA progress."""
import argparse,gzip,json,subprocess,sys
from pathlib import Path
from reference import ROOT,sha
F=896040
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True)
 source=capture/'native-transfer-gate.json.gz';native=json.loads(gzip.decompress(source.read_bytes()));base=out/'base.json.gz'
 result=subprocess.run([sys.executable,'-B',str(ROOT/'scripts/yuyuhakusho/export_pcm_transfer_gate.py'),'--capture',str(capture),'--out',str(base)],cwd=ROOT,text=True,capture_output=True,check=True);(out/'base-export.log').write_text(result.stdout+result.stderr)
 data=json.loads(gzip.decompress(base.read_bytes()));assert len(data['Cases'])==len(native['cases']);waits=0;events=0
 for c,body in zip(data['Cases'],native['cases']):
  assert c['Name']==body['case']['name'];c['Steps']=body['steps']
  for tr,raw in zip(c['Transactions'],body['transactions']):tr['MainDelay']=raw['main_delay']
  for f,row in enumerate(c['Frames']):
   original=row['Events'];cursor=0;physical=[]
   for e in body['events']:
    if e['frame']!=f or e['kind']not in[1,2,3,9,10,11]:continue
    if e['kind']==11:
     physical.append({'Cycle':f*F+e['cycles_before'],'Kind':'ym-address','Value':e['data'],'PayloadID':'','Offset':0,'Bus':None,'Register':0,'Port':e['address']})
    else:
     x=dict(original[cursor]);cursor+=1;x['Port']=3 if x['Kind']=='pan'else 1+2*(x['Register']>>8)if x['Kind']=='ym-write'else 1 if x['Kind']in['dac','enable']else 0;physical.append(x)
     if x['Bus']and x['Bus']['SetWaitFlag']:waits+=1
   assert cursor==len(original);row['Events']=physical;row['DMAWaiting']=body['frames'][f]['dma_waiting'];row['Frontier']=body['frames'][f]['frontier'];events+=len(physical)
 data['Version']=4;raw=(json.dumps(data,separators=(',',':'))+'\n').encode();fixture=out/'portable-scheduled-dma.json.gz';fixture.write_bytes(gzip.compress(raw,mtime=0));report={'pass':True,'cases':len(data['Cases']),'frames':sum(len(c['Frames'])for c in data['Cases']),'events':events,'wait_latches':waits,'waiting_progress_steps':sum(s['Kind']=='dma'and s['WaitingBefore']for c in data['Cases']for s in c['Steps']),'fixture_sha256':sha(fixture.read_bytes()),'native_sha256':sha(source.read_bytes()),'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [Path(__file__),ROOT/'scripts/yuyuhakusho/export_pcm_transfer_gate.py']},'scope':'Whole original encoded PCM streams, actual physical YM ports, complete PCM/DAC DSP and live native VDP progress. Native DMA progress supplied as caller effects; main VDP pricing proved separately.'};(out/'export.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report),flush=True)
if __name__=='__main__':main()
