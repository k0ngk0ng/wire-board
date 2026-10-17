"""Check exported frame identity, transparency, anchoring and coverage.

Written before the exporter: missing ownership, empty frames, cropped limbs,
mixed character frames, and untraceable exported images must fail or be listed
as explicitly skipped captures, never silently presented as complete animation.
"""
from pathlib import Path
from hashlib import sha256
import argparse,json
from PIL import Image

p=argparse.ArgumentParser();p.add_argument('directory',type=Path);args=p.parse_args()
doc=json.loads((args.directory/'atlas.json').read_text());ids=set();primary_pose_keys={}
for entry in doc['frames']:
    assert entry['id'] not in ids;ids.add(entry['id'])
    file=args.directory/entry['file'];assert sha256(file.read_bytes()).hexdigest()==entry['sha256']
    im=Image.open(file);assert im.mode=='RGBA' and im.size==(64,64)
    assert im.getbbox() and entry['origin']==[32,56]
    assert entry['sources'] and all(s['object']==entry['object'] for s in entry['sources'])
    for source in entry['sources']:
        assert source['priority'] in ['forward','reverse']
        if entry['object']==0:
            key=(source['state']['ram005b'],source['priority'])
            assert primary_pose_keys.get(key,entry['id'])==entry['id'],f'A pose/priority pair does not select a unique image: {key}'
            primary_pose_keys[key]=entry['id']
assert {0,1,2,3,6}<=set(f['object'] for f in doc['frames'])
assert len([f for f in doc['frames'] if f['object']==0])>=6,'Expected actual changing player poses'
assert all(s['reason'] for s in doc['skipped'])
print(json.dumps({'status':'passed','unique_frames':len(doc['frames']),'sources':sum(len(f['sources']) for f in doc['frames']),'scope':'sampled character/ball poses only; all actions and characters not yet covered'}))
