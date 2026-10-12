#!/usr/bin/env python3
"""Fetch every manifest URL from the actual CDN and retain repeatable evidence."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import io
import json
from pathlib import Path
import subprocess
import urllib.request
from PIL import Image

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--bundle', type=Path, required=True)
parser.add_argument('--base', required=True)
parser.add_argument('--output', type=Path, required=True)
args=parser.parse_args()
args.output.mkdir(parents=True,exist_ok=True)
manifest=json.loads((args.bundle/'manifest.json').read_text())
def verify(row):
 url=args.base.rstrip('/')+'/monopoly/v1/'+row['file']
 with urllib.request.urlopen(url,timeout=45) as response:
  body=response.read();mime=response.headers.get('Content-Type','').split(';')[0];status=response.status
 assert status==200 and mime==row['mime'],(url,status,mime)
 assert hashlib.sha256(body).hexdigest()==row['sha256'],url+' bytes differ'
 path=args.output/row['file'];path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(body)
 if mime.startswith('image/'):
  im=Image.open(io.BytesIO(body));im.load();assert list(im.size)==row['size']
 else:
  subprocess.run(['ffmpeg','-v','error','-xerror','-i',str(path),'-f','null','-'],check=True)
 return dict(row,url=url,status=status,verifiedMime=mime)
with ThreadPoolExecutor(max_workers=4) as pool: rows=list(pool.map(verify,manifest['media']))
(args.output/'result.json').write_text(json.dumps({'passed':True,'base':args.base,'files':rows},ensure_ascii=False,indent=2)+'\n')
print(f'Verified CDN bytes, MIME and decoding for {len(rows)} files.')
