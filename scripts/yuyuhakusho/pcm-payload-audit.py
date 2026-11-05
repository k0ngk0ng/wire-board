"""Read-only audit of the frozen PCM resource sources and portable evidence."""
from pathlib import Path
import hashlib,json,re

ROOT=Path(__file__).resolve().parents[2]
path=ROOT/'games/yuyuhakusho/verification/pcm-payload/validation.json'
body=json.loads(path.read_bytes());errors=[]
for group in ['source_sha256','evidence_sha256']:
    for name,want in body[group].items():
        p=(ROOT/name).resolve()
        if not p.is_relative_to(ROOT)or not p.is_file():errors.append('missing or outside workspace: '+name)
        elif hashlib.sha256(p.read_bytes()).hexdigest()!=want:errors.append('SHA differs: '+name)
proofs=[]
for name in body['final_logs']:
    p=ROOT/name
    if p.suffix!='.jsonl':continue
    rows=[json.loads(line)for line in p.read_text().splitlines()if line.startswith('{')]
    tops=[r for r in rows if r.get('Action')=='pass'and r.get('Test')and'/'not in r['Test']]
    subs=[r for r in rows if r.get('Action')=='pass'and'/'in r.get('Test','')]
    proof=re.findall(r'PCM_PAYLOAD_PROOF rows=129 encoded=437940 decoded=875880 sha256=([a-f0-9]{64})',p.read_text())
    if any(r.get('Action')=='fail'for r in rows)or len(tops)!=6 or len(subs)!=26 or len(proof)!=1:errors.append('final suite differs: '+name)
    proofs+=proof
if len(proofs)!=2 or len(set(proofs))!=1 or proofs[0]!=body['full_payload_proof_sha256']:errors.append('native/Wasm complete bytes differ')
if body['world_audio_integrated']or body['full_audio_goal_complete']:errors.append('scope exceeds this resource layer')
print(json.dumps({'pass':not errors,'sources':len(body['source_sha256']),'evidence':len(body['evidence_sha256']),'validation_sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'errors':errors},ensure_ascii=False,indent=2))
if errors:raise SystemExit(1)
