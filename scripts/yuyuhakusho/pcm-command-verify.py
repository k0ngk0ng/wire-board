"""Verify the frozen command research ledger and its unchanged source bindings."""
from pathlib import Path
import argparse,hashlib,json
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--native',action='store_true',help='also verify preserved workspace original captures');a=p.parse_args();report=ROOT/'games/yuyuhakusho/verification/audio-pcm-command/validation-address-v3-research.json';body=json.loads(report.read_text());errors=[]
for key in ['sources','evidence']+(['native_artifacts']if a.native else[]):
 for row in body[key]:
  path=ROOT/row['path']
  if not path.is_file():errors.append('missing '+row['path']);continue
  if hashlib.sha256(path.read_bytes()).hexdigest()!=row['sha256']:errors.append('SHA differs '+row['path'])
for item in body['checks']:
 result=json.loads((ROOT/item['path']).read_text())
 if result['pass']!=item['expected_pass']:errors.append('outcome differs '+item['path'])
if body['accepted_cases']!=23 or body['ordered_events']!=519256 or body['reference_frames']!=1274 or body['stereo_pcm_frames']!=937609:errors.append('coverage counts differ')
print(json.dumps({'pass':not errors,'sources':len(body['sources']),'evidence':len(body['evidence']),'native_checked':a.native,'errors':errors},ensure_ascii=False,indent=2))
if errors:raise SystemExit(1)
