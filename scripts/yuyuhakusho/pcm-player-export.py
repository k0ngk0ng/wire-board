"""Portable original-only player events; never derive expected values from model."""
from pathlib import Path
import argparse,gzip,json,runpy
from reference import ROOT,sha
# This accessor only classifies original events; it does not instantiate a model.
access=runpy.run_path(str(ROOT/'scripts/yuyuhakusho/pcm-player-oracle.py'))['original']
p=argparse.ArgumentParser();p.add_argument('--capture',required=True,type=Path);p.add_argument('--out',required=True,type=Path);p.add_argument('--only-edges',action='store_true');a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT)
cases=[]
for path in sorted(capture.glob('*.json.gz')):
 body=json.loads(gzip.decompress(path.read_bytes()))
 if a.only_edges and body['case']['name'] not in ['dual-min69','single-rate000','single-rate001','single-rate020','single-rate255']:continue
 events=access(body);samples=[None if s is None else{'id':s['id'],'encoded_hex':s['encoded_hex'],'rate':s['rate']}for s in body['samples']]
 encoded_pcs={0x12b:0,0x1a4:0,0x2a2:0,0x4dd:0,0x2bb:1,0x502:1};counts=[0,0];fetched=[0,0];dummy=0;acc=[128,128]
 for e in body['events']:
  if e['kind']==4 and e['pc']in encoded_pcs:counts[encoded_pcs[e['pc']]]+=1
  if e['kind']==7 and e['address']in [0x1fc1,0x1fc5]:acc[(e['address']-0x1fc1)//4]=e['data']
 for e in events:
  if e['kind']=='bank':fetched[e['slot']]+=1
 lastdac=next(e for e in reversed(body['events'])if e['kind']==1)
 if body['case']['mode']=='single':
  fetched[0]+=128;acc[0]=lastdac['data'];dummy=(body['frames'][-1]['registers']['iy']-body['samples'][0]['source']['z80_header'])&65535
 phase=0 if lastdac['pc']==0x2e1 else 1
 final={'consumed':counts,'fetched':fetched,'accumulator':acc,'dummy':dummy,'prefetch_slot':phase,'time':events[-1]['cycle']+49*15}
 cases.append({'name':body['case']['name'],'mode':body['case']['mode'],'samples':samples,'events':events,'final':final,'native_path':str(path.relative_to(ROOT)),'native_sha256':sha(path.read_bytes())})
raw=(json.dumps({'version':1,'scope':'Declared-private initialized original player; continuous original event times and output bytes, no external bus holds','cases':cases},separators=(',',':'))+'\n').encode();out.parent.mkdir(parents=True,exist_ok=True);out.write_bytes(gzip.compress(raw,mtime=0))
print(json.dumps({'cases':len(cases),'events':sum(len(c['events'])for c in cases),'raw_bytes':len(raw),'gzip_bytes':out.stat().st_size,'sha256':sha(out.read_bytes()),'raw_sha256':sha(raw)}))
