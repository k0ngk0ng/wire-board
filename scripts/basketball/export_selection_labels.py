"""Build compact UI labels from verified name metadata; no gameplay allowlist."""
from pathlib import Path
import argparse,hashlib,json
ROOT=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--check',action='store_true');args=parser.parse_args()
source=ROOT/'games/basketball/selection/names.json';data=json.loads(source.read_text())
assert data['authority']=='label-evidence-not-gameplay-allowlist'
assert [c['id'] for c in data['courts']]==list(range(8))
assert [t['id'] for t in data['teams']]==list(range(8))
for item in data['courts']+data['teams']:
 assert item['nameVerified'] and item['suggestedChineseLabel']
for team in data['teams']:
 assert [c['character'] for c in team['characters']]==list(range(4))
 assert all(c['nameVerified'] and c['team']==team['id'] and c['suggestedChineseLabel'] for c in team['characters'])
labels={'schemaVersion':1,'namesSHA256':hashlib.sha256(source.read_bytes()).hexdigest(),'translationAuthority':'editorial-not-official','courts':[c['suggestedChineseLabel'] for c in data['courts']],'teams':[{'name':t['suggestedChineseLabel'],'characters':[c['suggestedChineseLabel'] for c in t['characters']]}for t in data['teams']]}
# These identities are distinct in the original menu, despite sharing ID zero.
assert labels['courts'][0]=='尼亚加拉' and labels['teams'][0]['name']=='热血'
encoded=json.dumps(labels,ensure_ascii=False,indent=2)+'\n';target=ROOT/'games/basketball/selection/labels.json'
if args.check:assert target.read_text()==encoded,'UI labels differ from verified name metadata'
else:target.write_text(encoded)
print(json.dumps({'status':'passed','courts':8,'teams':8,'characters':32,'sourceSHA256':labels['namesSHA256'],'labelsSHA256':hashlib.sha256(encoded.encode()).hexdigest()}))
