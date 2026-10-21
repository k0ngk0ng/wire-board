"""Read-only re-audit of retained natural menu evidence; no new emulator run.
This proves original selection relationships, not Go whole-match coverage.
"""
import hashlib,json
from pathlib import Path
from PIL import Image,ImageChops
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/basketball/selection-names'
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
 names=json.loads((ROOT/'games/basketball/selection/names.json').read_text())
 assert digest(ROOT/'.local/nes-basket/basketball.nes')==names['romSHA256']
 for file,expected in names['sourceRecords'].items():assert digest(BASE/file)==expected,file
 courts=set();teams=set();screenshots=0
 for kind in ('courts','teams'):
  for row in names[kind]:
   src=row['source'].get('naturalMenu',row['source']);path=ROOT/src['path']
   assert json.loads((path/'capture-complete.json').read_text())==dict(status='complete')
   for file,expected in src['evidenceSHA256'].items():assert digest(path/file)==expected;screenshots+=file.endswith('.png')
   ram=(path/'ram.bin').read_bytes();f=src['selectedFrame'];state=ram[(f-1)*2048:f*2048]
   assert state[0x58b]==src['state']['court'] and list(state[0x586:0x588])==src['state']['teams']
   (courts if kind=='courts' else teams).add(row['id'])
 assert courts==teams==set(range(8))
 pairs=json.loads((BASE/'team-pairs.json').read_text())['pairs'];observed=set()
 for row in pairs:
  path=ROOT/row['path'];ram=(path/'ram.bin').read_bytes();f=row['frame'];state=ram[(f-1)*2048:f*2048]
  assert list(state[0x586:0x588])==row['state']['teams'];observed.add(tuple(state[0x586:0x588]))
  assert state[0x58d]&128,'natural versus mode expected'
 assert observed=={(a,b) for a in range(8) for b in range(8) if a!=b}
 collisions=json.loads((BASE/'member-collisions.json').read_text())['records']
 assert {row['team'] for row in collisions}==teams
 for row in collisions:
  path=ROOT/row['path'];images=[Image.open(path/f"frame-{row[k]:05d}.png").convert('RGB') for k in ('beforeFrame','rejectedFrame','movedFrame')]
  assert ImageChops.difference(images[0],images[1]).getbbox() is None
  assert ImageChops.difference(images[1],images[2]).getbbox() is not None
 result=dict(status='passed',romSHA256=names['romSHA256'],courts=sorted(courts),teams=sorted(teams),characterSlotsPerTeam=[0,1,2,3],distinctOrderedTeamPairs=len(observed),memberCollisionRoutes=len(collisions),boundScreenshots=screenshots,namesSHA256=digest(ROOT/'games/basketball/selection/names.json'),scope='retained original controller-only versus menus and 4-member selection screens; no new RAM writes, no new emulator capture, no whole-match claim',sourceRecords=names['sourceRecords'])
 out=ROOT/'.local/selection-admission';out.mkdir(parents=True,exist_ok=True)
 (out/'source-audit.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
