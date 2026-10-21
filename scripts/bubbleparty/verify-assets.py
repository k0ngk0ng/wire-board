#!/usr/bin/env python3
"""Validate immutable source assets and built same-origin fallbacks, v2-v4.
Failure inventory: missing v4 manifest, invalid SHA/bytes/dimensions/frames,
unsafe paths, duplicate objects and media silently omitted from web/dist.
"""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys

ROOT=Path(__file__).resolve().parents[2]

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--dist',action='store_true');args=parser.parse_args()
 subprocess.run([sys.executable,'-B',str(ROOT/'scripts/bubbleparty/check-assets.py')],cwd=ROOT,check=True)
 spec=importlib.util.spec_from_file_location('classic_assets',ROOT/'scripts/publish-classic-game-assets.py')
 module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
 entries,bindings=module.load_entries(ROOT,module.BUBBLE_MANIFESTS,dist=args.dist)
 count=0
 if args.dist:
  for filename in ['manifest-v2.json','manifest-v3.json']:
   manifest=json.loads((ROOT/'games/bubbleparty/assets'/filename).read_text())
   for entry in manifest['entries']:
    file=ROOT/'web/dist'/entry['urlPath'].lstrip('/');data=file.read_bytes()
    if hashlib.sha256(data).hexdigest()!=entry['sha256']:raise ValueError('built fallback SHA mismatch: '+str(file))
    if 'bytes'in entry and len(data)!=entry['bytes']:raise ValueError('built fallback bytes mismatch: '+str(file))
    count+=1
  count+=len(entries)
 print(json.dumps({'passed':True,'dist':args.dist,'packagedFiles':count,'v4Files':len(entries),'v4Manifests':bindings},ensure_ascii=False))

if __name__=='__main__':main()
