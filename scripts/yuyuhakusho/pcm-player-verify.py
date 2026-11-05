"""Verify the frozen standalone player source/evidence manifest without ROM."""
from pathlib import Path
import hashlib,json
ROOT=Path(__file__).resolve().parents[2]
report=ROOT/'games/yuyuhakusho/verification/audio-pcm-player/validation.json'
body=json.loads(report.read_text());errors=[]
for group in ['sources','evidence','portable_oracles']:
 for row in body[group]:
  path=ROOT/row['path']
  if not path.is_file():errors.append('missing '+row['path']);continue
  if hashlib.sha256(path.read_bytes()).hexdigest()!=row['sha256']:errors.append('SHA differs '+row['path'])
for log in body['final_logs']:
 records=[json.loads(s)for s in(ROOT/log).read_text().splitlines()if s.startswith('{')]
 failures=[r for r in records if r.get('Action')=='fail'];tops=[r for r in records if r.get('Action')=='pass'and r.get('Test')and'/'not in r['Test']]
 if failures or len(tops)!=6:errors.append('final test set differs '+log)
if body['native_player_events']!=70856 or body['native_stereo_frames']!=222258 or body['reference_frames']!=302:errors.append('coverage totals differ')
print(json.dumps({'pass':not errors,'source_files':len(body['sources']),'evidence_files':len(body['evidence']),'errors':errors},ensure_ascii=False,indent=2))
if errors:raise SystemExit(1)
