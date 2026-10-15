#!/usr/bin/env python3
"""Verify Weiqi media hashes, signatures, local fallback and optional decoding.
Failure cases: absent/empty/mislabelled files; stale dist; corrupt media; license
not retained. Written before preparing the first Weiqi media package.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

root=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--dist',action='store_true');p.add_argument('--decode',action='store_true');args=p.parse_args()
manifest=json.loads((root/'games/weiqi/assets/sources.json').read_text())
source=root/'games/weiqi/assets/public/weiqi/v1'
assert (root/manifest['audioSource']['licenseEvidence']).is_file()
checks=[]
for item in manifest['files']:
    data=(source/item['file']).read_bytes();assert len(data)==item['bytes'] and hashlib.sha256(data).hexdigest()==item['sha256'],item
    assert data[:4]==b'RIFF' and data[8:12]==(b'WEBP' if item['file'].endswith('.webp') else b'WAVE'),item
    if args.dist:assert (root/'web/dist/weiqi/v1'/item['file']).read_bytes()==data,item
    if args.decode:subprocess.run(['ffmpeg','-v','error','-i',str(source/item['file']),'-f','null','-'],check=True,capture_output=True)
    checks.append(item['file'])
assert len(checks)==9
out=root/'.local/weiqi/assets';out.mkdir(parents=True,exist_ok=True)
(out/'verification.json').write_text(json.dumps({'passed':True,'files':checks,'dist':args.dist,'decoded':args.decode,'command':'python3 -B scripts/weiqi/verify-assets.py'+(' --dist' if args.dist else '')+(' --decode' if args.decode else '')},indent=2)+'\n')
print('PASS Weiqi assets:',len(checks))
