#!/usr/bin/env python3
"""CI-safe verification of immutable native primary/secondary projectile art."""
import argparse,hashlib,importlib.util,json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
def verify(dist=False):
 assets=ROOT/'games/fortressblue/assets';manifest=json.loads((assets/'manifest-v5.json').read_text())
 spec=importlib.util.spec_from_file_location('native_webp',ROOT/'scripts/fortressblue/verify-v3-assets.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
 dimensions={}
 assert manifest['version']==5 and len(manifest['entries'])==17
 for entry in manifest['entries']:
  path=entry['urlPath'];assert path.startswith('/fortressblue/v5/') and path not in dimensions
  for base in [assets/'public']+([ROOT/'web/dist'] if dist else []):
   data=(base/path.lstrip('/')).read_bytes();assert len(data)==entry['bytes'] and hashlib.sha256(data).hexdigest()==entry['sha256'],path
   dimensions[path]=module.webp_size(data)
 projectiles=json.loads((ROOT/'games/fortressblue/data/projectiles.json').read_text());effects=json.loads((ROOT/'games/fortressblue/data/projectile-effects.json').read_text())
 assert set(projectiles)=={str(n)for n in range(1,15)} and set(effects)=={'mine','homming','ion'}
 for tank in projectiles.values():assert tank['primaryStart']==0 and tank['specialStart']==32 and tank['directions']==8 and tank['animationFrames']==4 and len(tank['frames'])>=64
 for kind,count in [('mine',4),('homming',7),('ion',18)]:assert len(effects[kind]['frames'])==count
 frames=0
 for sheet in [*projectiles.values(),*effects.values()]:
  width,height=dimensions[sheet['path']];assert len(sheet['sourceSHA256'])==64
  for frame in sheet['frames']:
   assert frame['w']>0 and frame['h']>0 and frame['x']>=0 and frame['y']>=0 and frame['x']+frame['w']<=width and frame['y']+frame['h']<=height
   assert isinstance(frame['ax'],int) and isinstance(frame['ay'],int)
   frames+=1
 return {'atlases':17,'frames':frames,'bytes':sum(e['bytes']for e in manifest['entries'])}
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('--dist',action='store_true');args=p.parse_args();print(json.dumps(verify(args.dist)))
