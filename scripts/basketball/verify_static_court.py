"""Before-export contract for independent Canvas court and HUD assets."""
from pathlib import Path
from hashlib import sha256
import argparse,json
from PIL import Image
p=argparse.ArgumentParser();p.add_argument('directory',type=Path);args=p.parse_args()
m=json.loads((args.directory/'manifest.json').read_text());assert m['accepted_for_release'] is False
sizes={'court.png':(512,432),'court-upper.png':(512,192),'court-lower.png':(512,240),'court-tiles.png':(128,512),'hud-template.png':(256,49),'hud-glyphs.png':(128,512)}
for name,size in sizes.items():
 file=args.directory/name;im=Image.open(file);assert im.size==size and im.mode=='RGBA';assert sha256(file.read_bytes()).hexdigest()==m['assets'][name]['sha256']
assert m['camera']['encoded_page_height']==256 and m['camera']['physical_page_height']==240
assert m['camera']['court_source_y']==48
assert len(m['hud']['dynamic_regions'])==5
ring=json.loads((args.directory/'initial-ring.json').read_text());assert ring['columns']==list(range(32,48))+list(range(16,32))
packets=json.loads((args.directory/'stream-columns.json').read_text());assert len(packets['gameplay'])==64 and len(packets['initialization'])==64
for phase in packets.values():
 for column,packet in enumerate(phase):
  assert packet['column']==column and packet['slot']==column%32
  assert all(len(x)==30 for x in packet['tiles'])
  assert all(len(x)==8 for x in packet['attributes']) and len(packet['attribute_indices'])==8
report=json.loads((args.directory/'pixel-verification.json').read_text());assert report['frames']==83 and report['mismatches']==0
assert report['background_pixels']>3900000 and report['hud_pixels']>900000
print(json.dumps({'status':'passed','files':len(m['assets']),'pixel_frames':83,'accepted_for_release':False}))
