#!/usr/bin/env python3
"""Migrate source media, verify originals, and make lossless WebP in new paths.
No writes to wire-pairs. No upstream license claim is invented.
"""
import hashlib,json,shutil,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[2]
source=root.parent/'wire-pairs'
assets=root/'games/pairs/assets'
manifest=json.loads((source/'public/assets/manifest.json').read_text())
entries=[]
for e in manifest['files']:
    name=e['file']
    if Path(name).suffix not in ['.png','.wav','.mp3']: continue
    original=source/'public/assets'/name
    data=original.read_bytes()
    assert hashlib.sha256(data).hexdigest()==e['sha256'],name
    target=assets/'source'/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(original,target)
    output=assets/'public/pairs/v1'/name
    output=output.with_suffix('.webp') if original.suffix=='.png' else output
    output.parent.mkdir(parents=True,exist_ok=True)
    if original.suffix=='.png':subprocess.run(['cwebp','-quiet','-lossless',str(target),'-o',str(output)],check=True)
    else:shutil.copyfile(target,output)
    entries.append({**e,'sourceFile':target.relative_to(root).as_posix(),'sourceSHA256':e['sha256'],'output':output.relative_to(root).as_posix(),'urlPath':'/'+output.relative_to(assets/'public').as_posix(),'sha256':hashlib.sha256(output.read_bytes()).hexdigest(),'bytes':output.stat().st_size,'license':'Not established by upstream; public recreation, not an official Tencent asset release','conversion':'lossless WebP' if original.suffix=='.png' else 'original unchanged'})
(assets/'source-manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
shutil.copyfile(source/'ASSETS.md',assets/'source/UPSTREAM-ASSETS.md')
(assets/'manifest.json').write_text(json.dumps({'version':1,'game':'pairs','prefix':'bga/v20261002-1/pairs/v1/','entries':entries},ensure_ascii=False,indent=2)+'\n')
print('Prepared',len(entries),'verified media files')
