from pathlib import Path
import importlib.util,json,hashlib,io
ROOT=Path(__file__).resolve().parents[2];spec=importlib.util.spec_from_file_location('codec',ROOT/'scripts/fortressblue/prepare-original-assets.py');codec=importlib.util.module_from_spec(spec);spec.loader.exec_module(codec)
from PIL import Image
out=ROOT/'games/fortressblue/assets/public/fortressblue/v5';out.mkdir(parents=True,exist_ok=True)
metadata={};entries=[]
for tank in range(1,15):
 source=ROOT/f'.local/fortressblue-original/downloaded/bullet{tank:02}.i16';frames=codec.frames(source);w=max(f['image'].width for f in frames);h=max(f['image'].height for f in frames);cols=8;atlas=Image.new('RGBA',(cols*w,((len(frames)+cols-1)//cols)*h));rects=[]
 for i,frame in enumerate(frames):
  im=frame['image'];x=(i%cols)*w;y=(i//cols)*h;atlas.paste(im,(x,y));rects.append(dict(x=x,y=y,w=im.width,h=im.height,ax=-frame['ox'],ay=-frame['oy']))
 buf=io.BytesIO();atlas.save(buf,format='WEBP',lossless=True,method=6,exact=True);raw=buf.getvalue();p=out/f'projectile-{tank}.webp'
 if p.exists():assert p.read_bytes()==raw,'Immutable atlas mismatch'
 else:p.write_bytes(raw)
 path=f'/fortressblue/v5/{p.name}';entries.append(dict(urlPath=path,bytes=len(raw),sha256=hashlib.sha256(raw).hexdigest()));metadata[str(tank)]=dict(path=path,primaryStart=0,specialStart=32,directions=8,animationFrames=4,frames=rects,source=str(source.relative_to(ROOT)),sourceSHA256=hashlib.sha256(source.read_bytes()).hexdigest())
(ROOT/'games/fortressblue/data/projectiles.json').write_text(json.dumps(metadata,ensure_ascii=False,separators=(',',':'))+'\n')
(ROOT/'games/fortressblue/assets/manifest-v5.json').write_text(json.dumps(dict(version=5,license='原版素材，权利属于原权利方；本会话授权提取。',conversion='Native I16 main images, original rectangles and anchors; lossless exact-RGBA WebP atlas, no resizing/recoloring/mirroring.',entries=entries),ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'entries':len(entries),'bytes':sum(e['bytes']for e in entries),'frames':sum(len(t['frames'])for t in metadata.values())}))
effects={}
for name in ['mine','homming','ion']:
 source=ROOT/f'.local/fortressblue-original/downloaded/{name}.i16';frames=codec.frames(source);w=max(f['image'].width for f in frames);h=max(f['image'].height for f in frames);cols=min(8,max(1,2048//w));atlas=Image.new('RGBA',(cols*w,((len(frames)+cols-1)//cols)*h));rects=[]
 for i,frame in enumerate(frames):
  im=frame['image'];x=(i%cols)*w;y=(i//cols)*h;atlas.paste(im,(x,y));rects.append(dict(x=x,y=y,w=im.width,h=im.height,ax=-frame['ox'],ay=-frame['oy']))
 buf=io.BytesIO();atlas.save(buf,format='WEBP',lossless=True,method=6,exact=True);raw=buf.getvalue();p=out/f'{name}.webp'
 if p.exists():assert p.read_bytes()==raw,'Immutable atlas mismatch'
 else:p.write_bytes(raw)
 path=f'/fortressblue/v5/{p.name}';entries.append(dict(urlPath=path,bytes=len(raw),sha256=hashlib.sha256(raw).hexdigest()));effects[name]=dict(path=path,frames=rects,source=str(source.relative_to(ROOT)),sourceSHA256=hashlib.sha256(source.read_bytes()).hexdigest())
(ROOT/'games/fortressblue/data/projectile-effects.json').write_text(json.dumps(effects,ensure_ascii=False,separators=(',',':'))+'\n')
(ROOT/'games/fortressblue/assets/manifest-v5.json').write_text(json.dumps(dict(version=5,license='原版素材，权利属于原权利方；本会话授权提取。',conversion='Native I16 main images, original rectangles and anchors; lossless exact-RGBA WebP atlas, no resizing/recoloring/mirroring.',entries=entries),ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'entries':len(entries),'bytes':sum(e['bytes']for e in entries),'effects':{k:len(v['frames'])for k,v in effects.items()}}))
