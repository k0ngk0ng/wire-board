#!/usr/bin/env python3
"""Read back every published poker object; check bytes, MIME and media signature."""
import concurrent.futures,hashlib,json,urllib.request
from pathlib import Path
root=Path(__file__).resolve().parents[2]
manifest=json.loads((root/'assets/poker/manifest.json').read_text());base='https://cdn.ichenj.com/bga/v20261002-1'
def check(e):
 path=e['output'].removeprefix('web/public/');url=base+'/'+path
 with urllib.request.urlopen(url,timeout=45) as r:data=r.read();mime=r.headers.get_content_type()
 digest=hashlib.sha256(data).hexdigest();assert digest==e['sha256'],url
 expected='audio/ogg' if path.endswith('.ogg') else 'image/webp';assert mime==expected,(url,mime)
 assert (data[:4]==b'OggS') if path.endswith('.ogg') else (data[:4]==b'RIFF' and data[8:12]==b'WEBP'),url
 return dict(url=url,sha256=digest,mime=mime,bytes=len(data),passed=True)
with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:results=list(pool.map(check,manifest['entries']))
out=root/'.local/poker/oss-readback.json';out.write_text(json.dumps({'passed':True,'objects':results},indent=2)+'\n');print('Verified',len(results),'objects:',out)
