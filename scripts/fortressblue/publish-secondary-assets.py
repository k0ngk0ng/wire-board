#!/usr/bin/env python3
"""Verify/publish immutable v5 projectile art using the tested OSS transport."""
import argparse, importlib.util, json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('classic_publisher',ROOT/'scripts/publish-classic-game-assets.py')
shared=importlib.util.module_from_spec(spec);spec.loader.exec_module(shared)
p=argparse.ArgumentParser();p.add_argument('--publish',action='store_true');p.add_argument('--dist',action='store_true');args=p.parse_args()
out=ROOT/'.local/fortress-secondary'/('oss-publish.json' if args.publish else 'preflight.json')
report={'passed':False,'mode':'publish' if args.publish else 'preflight','objects':[]}
try:
 manifest=ROOT/'games/fortressblue/assets/manifest-v5.json';m=json.loads(manifest.read_text());shared.check(len(m['entries'])==17,'expected 17 native projectile atlases')
 entries=[]
 for row in m['entries']:
  url=row['urlPath'];shared.check(url.startswith('/fortressblue/v5/') and url.endswith('.webp') and '..' not in url and len(Path(url).parts)==4,'unexpected v5 object path')
  file=ROOT/'games/fortressblue/assets/public'/url.lstrip('/');data=file.read_bytes()
  shared.verify_bytes(data,row,url);shared.media_dimensions(data,'image/webp')
  if args.dist:shared.verify_bytes((ROOT/'web/dist'/url.lstrip('/')).read_bytes(),row,'built '+url)
  entries.append({**row,'mime':'image/webp','data':data})
 shared.ensure_unique(entries)
 report.update(manifestSHA256=shared.sha(manifest.read_bytes()),objectsExpected=len(entries),bytesExpected=sum(e['bytes'] for e in entries),distVerified=args.dist)
 if args.publish:
  client=shared.OSSClient((ROOT/'config/secrets/oss-access-key-id').read_text().strip(),(ROOT/'config/secrets/oss-access-key-secret').read_text().strip())
  anchor=json.loads((ROOT/'games/snooker/assets/manifest.json').read_text())['entries'][0]
  report['anchor']=shared.verify_anchor(client,shared.PREFIX+anchor['urlPath'],anchor['sha256'])
  report['objects'],report['errors']=shared.publish_entries(client,entries,4)
  shared.check(not report['errors'] and len(report['objects'])==len(entries),'publication incomplete')
 report['passed']=True
finally:
 shared.write_report(out,report)
print(json.dumps({k:v for k,v in report.items() if k!='objects'},ensure_ascii=False))
