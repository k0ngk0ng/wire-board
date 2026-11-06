"""Autonomous command address stream versus retained original YM latch."""
from pathlib import Path
import argparse,gzip,hashlib,json,runpy
from reference import ROOT,sha
Device=runpy.run_path(str(ROOT/'scripts/yuyuhakusho/pcm-command-prototype.py'))['Device']
p=argparse.ArgumentParser();p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT);rows=[]
for path in sorted(capture.glob('*.json.gz')):
 body=json.loads(gzip.decompress(path.read_bytes()));events=Device(body).run();packets={};address=0;trace=[]
 for event in events:
  if event['kind']=='address':packets.setdefault(event['start']//896040,[]).append(event)
 for frame in body['frames']:
  for event in packets.get(frame['frame'],[]):address=event['value']
  expected=frame['native_ym_control']['address'];assert address==expected,(path.name,frame['frame'],address,expected);trace.append(address)
 rows.append({'name':body['case']['name'],'frames':len(trace),'all_model_address_latches_equal':True,'native_sha256':sha(path.read_bytes()),'address_trace_sha256':sha(json.dumps(trace,separators=(',',':')).encode())})
report={'pass':True,'scope':'At every frame boundary the autonomous address stream reproduces the original retained YM address; reset preserves that latch','cases':rows,'producer_sha256':sha(Path(__file__).read_bytes()),'model_sha256':sha((ROOT/'scripts/yuyuhakusho/pcm-command-prototype.py').read_bytes())};out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'pass':True,'cases':len(rows),'frames':sum(r['frames']for r in rows)}))
