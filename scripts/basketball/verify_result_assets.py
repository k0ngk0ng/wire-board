"""Before-export contract: separate original result scenes, score glyphs and kind17."""
from pathlib import Path
from hashlib import sha256
import argparse,json
from PIL import Image
p=argparse.ArgumentParser();p.add_argument('directory',type=Path);args=p.parse_args();m=json.loads((args.directory/'manifest.json').read_text())
assert m['accepted_for_release'] is False and len(m['courts'])==8 and len(m['actorProfiles'])==120
assets=0
for court in m['courts']:
 assert court['presets']==[court['court']+101,99]
 variants=court.get('paletteVariants',[]);assert [v['paletteID'] for v in variants]==([9,10] if court['court']==4 else [])
 for variant in [court,*variants]:
  for role,size in [('background',(256,240)),('hud',(256,48)),('digits',(160,24)),('hudTiles',(128,512))]:
   record=variant[role];assets+=1;file=args.directory/record['file'];assert sha256(file.read_bytes()).hexdigest()==record['sha256'];im=Image.open(file);assert im.size==size and im.mode=='RGBA'
assert assets==40
print(json.dumps({'status':'passed','courts':8,'actor_poses':120,'assets':assets,'accepted_for_release':False}))
