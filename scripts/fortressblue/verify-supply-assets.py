#!/usr/bin/env python3
"""Check committed immutable supply atlases without local decoder dependencies."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
def verify(dist=False):
 manifest=json.loads((ROOT/'games/fortressblue/assets/manifest-v4.json').read_text())
 catalog=json.loads((ROOT/'games/fortressblue/data/supplies.json').read_text())
 spec=importlib.util.spec_from_file_location('v3_verify',ROOT/'scripts/fortressblue/verify-v3-assets.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
 dimensions={}
 for entry in manifest['entries']:
  for base in [ROOT/'games/fortressblue/assets/public']+([ROOT/'web/dist'] if dist else []):
   path=base/entry['urlPath'].lstrip('/');data=path.read_bytes()
   assert len(data)==entry['bytes'] and hashlib.sha256(data).hexdigest()==entry['sha256'],path
   dimensions[entry['urlPath']]=module.webp_size(data)
 assert len(manifest['entries'])==2
 for kind,count in [('crate',49),('helicopter',12)]:
  sheet=catalog[kind];assert len(sheet['frames'])==count
  width,height=dimensions[sheet['path']]
  for frame in sheet['frames']:
   assert frame['x']>=0 and frame['y']>=0 and frame['w']>0 and frame['h']>0 and frame['x']+frame['w']<=width and frame['y']+frame['h']<=height
 return {'atlases':2,'frames':61,'bytes':sum(e['bytes']for e in manifest['entries'])}
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--dist',action='store_true');args=parser.parse_args();print(json.dumps(verify(args.dist)))
