#!/usr/bin/env python3
"""Pack original supply sprites; never overwrite previously published versions."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('fortress_codec',ROOT/'scripts/fortressblue/prepare-original-assets.py')
codec=importlib.util.module_from_spec(spec);spec.loader.exec_module(codec)
from PIL import Image
OUT=ROOT/'games/fortressblue/assets/public/fortressblue/v4'
OUT.mkdir(parents=True,exist_ok=True)
manifest=[];catalog={}
for name,source_name in [('crate','item'),('helicopter','transfer')]:
 source=ROOT/f'.local/fortressblue-original/downloaded/{source_name}.i16'
 frames=codec.frames(source)
 # Native Item includes coin/gift variants; preserve all 49 frames for audit.
 width=max(f['image'].width for f in frames);height=max(f['image'].height for f in frames)
 columns=8;rows=(len(frames)+columns-1)//columns
 atlas=Image.new('RGBA',(width*columns,height*rows));metadata=[]
 for i,frame in enumerate(frames):
  image=frame['image'];x=(i%columns)*width;y=(i//columns)*height;atlas.paste(image,(x,y))
  metadata.append(dict(x=x,y=y,w=image.width,h=image.height,ax=-frame['ox'],ay=-frame['oy']))
 path=OUT/f'{name}.webp'
 import io
 output=io.BytesIO();atlas.save(output,format='WEBP',lossless=True,method=6);data=output.getvalue()
 if path.exists():assert path.read_bytes()==data,'Immutable media exists with different bytes'
 else:path.write_bytes(data)
 url=f'/fortressblue/v4/{name}.webp'
 manifest.append(dict(urlPath=url,bytes=len(data),sha256=hashlib.sha256(data).hexdigest()))
 catalog[name]=dict(path=url,frames=metadata,source=str(source.relative_to(ROOT)),sourceSHA256=hashlib.sha256(source.read_bytes()).hexdigest())
(ROOT/'games/fortressblue/data/supplies.json').write_text(json.dumps(catalog,ensure_ascii=False,indent=2)+'\n')
(ROOT/'games/fortressblue/assets/manifest-v4.json').write_text(json.dumps(dict(version=4,license='Original Fortress assets; rights remain with original owners; extraction authorized in this session.',conversion='Original I16 decoder; native rectangles/anchors, visible RGBA and alpha preserved; lossless WebP may normalize fully transparent RGB.',entries=manifest),ensure_ascii=False,indent=2)+'\n')
print(json.dumps(manifest,indent=2))
