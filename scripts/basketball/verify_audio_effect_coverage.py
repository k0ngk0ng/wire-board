"""Bounded audit of restored literal requests and the original attack sound table.

This is not a static proof of every dynamically selected engine request.
"""
from pathlib import Path
import hashlib,json,re
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/audio/export'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def main():
 catalog=json.loads((ROOT/'games/basketball/assets/audio-catalog.json').read_text())
 playback=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())
 original=json.loads((OUT/'manifest.json').read_text())
 effects={x['id']:x['wav'] for x in catalog['effects']}
 effects.update({int(i):v['wav'] for i,v in playback['effects'].items()})
 supported=set(effects)|{int(i) for i in playback['music']}|{x['id'] for x in catalog['controls']}
 profile=ROOT/'games/basketball/engine/attack_profiles.go'
 table=profile.read_text().split('var originalAttackSounds = [9][32]uint8{',1)[1].split('\n}',1)[0]
 attack=[int(x) for x in re.findall(r'\d+',table)];assert len(attack)==288
 assert not set(attack)-set(effects),f'missing attack assets: {set(attack)-set(effects)}'
 requests=[]
 for path in sorted((ROOT/'games/basketball/engine').glob('*.go')):
  if path.name.endswith('_test.go'):continue
  for line_num,line in enumerate(path.read_text().splitlines(),1):
   if not any(word in line for word in ['Sound','sounds','sound ']):continue
   values=[]
   for match in re.finditer(r'append\([^,]+,\s*(0x[0-9a-f]+|\d+)\)',line):values.append(int(match[1],0))
   for match in re.finditer(r'(?:SoundIDs:|SoundIDs =)\s*\[\]uint8\{([\d, ]+)\}',line):values.extend(int(x) for x in re.findall(r'\d+',match[1]))
   for match in re.finditer(r'CombatSound\{[^}]+ID: (\d+)',line):values.append(int(match[1]))
   for ident in values:
    assert ident in supported,f'{path.name}:{line_num} missing sound {ident}'
    requests.append(dict(file=path.name,line=line_num,id=ident))
 assets=[]
 originals={e['id']:e for e in original['sounds']}
 for ident,file in sorted(effects.items()):
  actual=sha(OUT/file)
  expected=playback['effects'][str(ident)]['sha256'] if str(ident) in playback['effects'] else originals[ident]['sha256'][file]
  assert actual==expected,file
  assets.append(dict(id=ident,file=file,sha256=actual))
 assert {1,23,48}<={r['id'] for r in requests}
 assert catalog['accepted_for_release'] is playback['accepted_for_release'] is False
 result=dict(status='passed',attack_entries=len(attack),attack_ids=sorted(set(attack)),literal_requests=requests,effects=assets,source_table_sha256=sha(profile),scope='9 x 32 original attack sound table and explicit SoundIDs/CombatSound literals; not arbitrary dynamic data-flow or playback mixing',accepted_for_release=False)
 target=ROOT/'.local/basketball/audio-resume/coverage.json';target.parent.mkdir(parents=True,exist_ok=True);target.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(dict(status='passed',effects=len(assets),attack_entries=len(attack),literal_requests=len(requests))))
if __name__=='__main__':main()
