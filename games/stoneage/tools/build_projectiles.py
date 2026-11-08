#!/usr/bin/env python3
"""Index existing native arrow/stone bitmaps; never copy or upload images."""
import argparse,hashlib,json
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--source',required=True);args=parser.parse_args()
root=Path(args.source).resolve();assets=root/'client/web/assets/original';manifest=json.loads((assets/'manifest.json').read_text());result={'basePath':'assets','bitmaps':{},'resources':{}}
for logical in [*range(25630,25646),*range(25650,25666),25785,25786]:
 f=manifest['bitmaps'][str(logical)];data=(assets/f['file']).read_bytes()
 result['bitmaps'][str(logical)]={k:f[k] for k in ['file','width','height','xoffset','yoffset']}
 result['resources'][f['file']]={'bytes':len(data),'sha256':hashlib.sha256(data).hexdigest()}
output=Path(__file__).resolve().parents[1]/'web/projectile-art.json';output.write_text(json.dumps(result,ensure_ascii=False,separators=(',',':'))+'\n')
