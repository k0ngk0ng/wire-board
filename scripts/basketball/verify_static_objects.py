"""Before-export checks: exact key coverage, immutable PNGs, no empty/cropped
objects, explicit palette ownership, and ball-pose CHR provenance.
"""
from pathlib import Path
from hashlib import sha256
import argparse,json
from PIL import Image
p=argparse.ArgumentParser();p.add_argument('directory',type=Path);args=p.parse_args()
doc=json.loads((args.directory/'atlas.json').read_text());assert doc['pose_counts']==[40,6,4,1,3,2,4]
assert len(doc['sheets'])==32
frames=0
for sheet in doc['sheets']:
 file=args.directory/sheet['file'];assert sha256(file.read_bytes()).hexdigest()==sheet['sha256'];im=Image.open(file);assert im.mode=='RGBA'
 keys=set()
 for f in sheet['frames']:
  key=(f['kind'],f['pose'],f['chr_bank']);assert key not in keys;keys.add(key)
  assert 8<=f['kind']<=14 and 0<=f['pose']<doc['pose_counts'][f['kind']-8]
  assert f['palette_side'] in (0,1) and f['origin']==[64,96]
  if f['kind']==8:assert f['chr_bank']==doc['ball_chr_banks'][f['pose']]
  else:assert f['chr_bank'] in (32,33,34,35)
  x,y,w,h=f['rect'];tile=im.crop((x,y,x+w,y+h));assert tile.getbbox() and list(tile.getbbox())==f['bounds'];frames+=1
 assert len(keys)==120
assert frames==3840
print(json.dumps({'status':'passed','sheets':32,'frames':frames,'scope':'static kind 8..14 pose tables and relevant shared CHR pages; not action reachability'}))
