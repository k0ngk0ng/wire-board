"""Verify original court package identities, independent art and all hashes."""
import json
from pathlib import Path
from hashlib import sha256
from PIL import Image
ROOT=Path(__file__).resolve().parents[2];root=ROOT/'.local/basketball/courts';top=json.loads((root/'manifest.json').read_text());assert not top['accepted_for_release'];images=0;jsons=0;identities=[]
for rec in top['courts']:
 p=root/rec['manifest'];assert sha256(p.read_bytes()).hexdigest()==rec['sha256'];m=json.loads(p.read_text());assert m['court']==rec['court'] and not m['accepted_for_release'];d=p.parent
 for name,a in m['assets'].items():
  f=d/name;assert sha256(f.read_bytes()).hexdigest()==a['sha256']
  if f.suffix=='.png':
   im=Image.open(f);assert list(im.size)==a['size'] and im.mode=='RGBA';images+=1
  else:json.loads(f.read_text());jsons+=1
 identities.append(m['assets'][f'court-{m["court"]+1}.png']['sha256'])
assert len(set(identities))==8 and images==60 and jsons==16
report={'status':'passed','scope':'eight independent original court static assets and exported palette variants; not complete frame equivalence','courts':8,'distinct_base_courts':8,'images':images,'json_assets':jsons,'accepted_for_release':False};(root/'asset-verification.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
