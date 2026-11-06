"""Verify owned PCM command source freezes and portable originals, no ROM."""
from pathlib import Path
import hashlib,json
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'games/yuyuhakusho/verification/audio-pcm-command-go';errors=[];checked=0
for name in ['validation-v1.json','validation-controls-v2.json','validation-bus.json','validation-ports-v3.json']:
 body=json.loads((BASE/name).read_text())
 for row in body['sources']:
  target=ROOT/row.get('frozen_copy',row['path'])
  if not target.is_file()or hashlib.sha256(target.read_bytes()).hexdigest()!=row['sha256']:errors.append(name+': frozen source '+row['path'])
  checked+=1
 fixtures=body.get('fixtures',[])
 if 'fixture'in body:fixtures.append(body['fixture'])
 for row in fixtures:
  target=ROOT/row['path']
  if hashlib.sha256(target.read_bytes()).hexdigest()!=row['sha256']:errors.append(name+': original '+row['path'])
 for log in body['checks']:
  target=ROOT/log['path']
  if hashlib.sha256(target.read_bytes()).hexdigest()!=log['sha256']:errors.append(name+': log '+log['path'])
  entries=[json.loads(s)for s in target.read_text().splitlines()if s.startswith('{')]
  if any(e.get('Action')=='fail'for e in entries):errors.append(name+': failed final log')
latest=json.loads((BASE/'validation-ports-v3.json').read_text())
for row in latest['sources']:
 target=ROOT/row['path']
 if not target.is_file()or hashlib.sha256(target.read_bytes()).hexdigest()!=row['sha256']:errors.append('latest current source '+row['path'])
print(json.dumps({'pass':not errors,'frozen_source_bindings':checked,'errors':errors},ensure_ascii=False,indent=2))
if errors:raise SystemExit(1)
