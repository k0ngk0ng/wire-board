"""Actual native latch versus data-only synthesized-address admission."""
from pathlib import Path
import argparse,gzip,json
from reference import ROOT,sha
p=argparse.ArgumentParser();p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);p.add_argument('--data-only',action='store_true');a=p.parse_args();source=a.capture.resolve();out=a.out.resolve();assert source.is_relative_to(ROOT)and out.is_relative_to(ROOT);body=json.loads(gzip.decompress(source.read_bytes()));address=0;rows=[]
for f in body['frames']:
 for e in body['events']:
  if e['frame']!=f['frame']:continue
  if not a.data_only and e['kind']==11:address=e['data']
  elif a.data_only and e['kind']in[1,2,9]:address=e['address']&255
  # The actual YM reset deliberately preserves address, in both models.
 expected=f['native_ym_control']['address'];rows.append({'frame':f['frame'],'actual':address,'original':expected,'equal':address==expected})
report={'pass':all(r['equal']for r in rows),'data_only_synthesized_address':a.data_only,'native_path':str(source.relative_to(ROOT)),'native_sha256':sha(source.read_bytes()),'rows':rows,'checker_sha256':sha(Path(__file__).read_bytes()),'scope':'A typed address-latch projection of actual port calls; no full chip inference from this one field'};out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
if not report['pass']:raise SystemExit(1)
