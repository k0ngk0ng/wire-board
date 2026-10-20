#!/usr/bin/env python3
"""CI entrypoint: validate both immutable source assets and built local fallbacks.
Failure cases: v3 omitted by the old v2-only check; missing or altered files in
web/dist; build packaging silently drops native avatar sheets.
"""
import argparse,hashlib,json,runpy
from pathlib import Path
root=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--dist',action='store_true');args=p.parse_args()
runpy.run_path(str(root/'scripts/bubbleparty/check-assets.py'),run_name='__main__')
count=0
if args.dist:
 for filename in ['manifest-v2.json','manifest-v3.json']:
  manifest=json.loads((root/'games/bubbleparty/assets'/filename).read_text())
  for entry in manifest['entries']:
   file=root/'web/dist'/entry['urlPath'].lstrip('/')
   data=file.read_bytes();assert hashlib.sha256(data).hexdigest()==entry['sha256'],str(file)
   count+=1
print(json.dumps({'passed':True,'dist':args.dist,'packagedFiles':count}))
