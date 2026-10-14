#!/usr/bin/env python3
"""Verify source hashes, migrated bytes, packaged fallbacks and optional CDN."""
import argparse,hashlib,json,urllib.request,concurrent.futures
from pathlib import Path
root=Path(__file__).resolve().parents[2];parser=argparse.ArgumentParser();parser.add_argument('--dist',action='store_true');parser.add_argument('--oss',action='store_true');args=parser.parse_args()
m=json.loads((root/'games/pairs/assets/manifest.json').read_text())
assert m['prefix']=='bga/v20261002-1/pairs/v1/'
assert len(m['entries'])==67
sha=lambda b:hashlib.sha256(b).hexdigest()
def check(e):
 source=(root/e['sourceFile']).read_bytes();data=(root/e['output']).read_bytes()
 assert sha(source)==e['sourceSHA256'];assert sha(data)==e['sha256'];assert len(data)==e['bytes']
 assert e['urlPath'].startswith('/pairs/v1/') and '/llk/' not in e['urlPath']
 assert (root/e['output']).relative_to(root/'games/pairs/assets/public').as_posix()==e['urlPath'].lstrip('/')
 if args.dist:assert sha((root/'web/dist'/e['urlPath'].lstrip('/')).read_bytes())==e['sha256']
 if args.oss:
  with urllib.request.urlopen('https://cdn.ichenj.com/bga/v20261002-1'+e['urlPath'],timeout=40) as r:assert sha(r.read())==e['sha256']
 return {'path':e['urlPath'],'sha256':e['sha256'],'passed':True}
with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:results=list(pool.map(check,m['entries']))
out=root/'.local/pairs';out.mkdir(exist_ok=True,parents=True);(out/('oss-verify.json' if args.oss else 'assets-verify.json')).write_text(json.dumps({'passed':True,'dist':args.dist,'objects':results},indent=2)+'\n');print('Verified',len(results),'source/media/fallback objects')
