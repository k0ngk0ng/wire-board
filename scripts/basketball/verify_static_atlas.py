"""Before-export contract: complete table coverage, image identities, no cropped
limbs, selectable team/character/pose/facing/priority, source provenance.
This checks static assets, not reachability of every pose in normal gameplay.
"""
from pathlib import Path
from hashlib import sha256
import json
import argparse
from PIL import Image
from static_sprites import SpriteROM
p=argparse.ArgumentParser();p.add_argument('directory',type=Path);args=p.parse_args()
doc=json.loads((args.directory/'atlas.json').read_text())
assert doc['source']['rom_sha256']=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
assert len(doc['sheets'])==8*4*2*2
keys=set();count=0
for sheet in doc['sheets']:
    key=(sheet['team'],sheet['character'],sheet['facing'],sheet['priority']);assert key not in keys;keys.add(key)
    file=args.directory/sheet['file'];assert sha256(file.read_bytes()).hexdigest()==sheet['sha256']
    im=Image.open(file);assert im.mode=='RGBA'
    assert len(sheet['frames'])==doc['pose_counts'][sheet['team']]
    assert [f['pose'] for f in sheet['frames']]==list(range(len(sheet['frames'])))
    for frame in sheet['frames']:
        x,y,w,h=frame['rect'];tile=im.crop((x,y,x+w,y+h));assert tile.getbbox()
        assert list(tile.getbbox())==frame['bounds'];assert frame['origin']==[64,96]
        count+=1
print(json.dumps({'status':'passed','sheets':len(keys),'frames':count,'scope':'all eight static player pose tables, four selectable character head variants, two facings and OAM orders; gameplay reachability is not asserted'}))
