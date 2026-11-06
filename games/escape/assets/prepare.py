#!/usr/bin/env python3
"""Normalize approved generated art; preserve native alpha and record provenance."""
import argparse
import hashlib
import json
from collections import deque
from pathlib import Path
from PIL import Image

ROOT=Path(__file__).resolve().parents[3]
HERE=Path(__file__).resolve().parent
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def clear_white_matte(sprite):
 """Remove only the connected outer white matte in the engineer walk draft.
 Closed highlights inside the goggles/body retain their original alpha.
 """
 pixels=sprite.load();w,h=sprite.size;visited=bytearray(w*h)
 queue=deque([(x,0) for x in range(w)]+[(x,h-1) for x in range(w)]+[(0,y) for y in range(h)]+[(w-1,y) for y in range(h)])
 while queue:
  x,y=queue.popleft();index=y*w+x
  if visited[index]:continue
  visited[index]=1;r,g,b,a=pixels[x,y]
  if not (a<8 or min(r,g,b)>135 and max(r,g,b)-min(r,g,b)<55):continue
  pixels[x,y]=(r,g,b,0)
  for nx,ny in [(x-1,y),(x+1,y),(x,y-1),(x,y+1)]:
   if 0<=nx<w and 0<=ny<h and not visited[ny*w+nx]:queue.append((nx,ny))
 return sprite
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--source',type=Path,default=HERE/'source');args=parser.parse_args()
 target=HERE/'public/escape/v1';target.mkdir(parents=True,exist_ok=True)
 entries=[]
 for name in ['station-atrium','station-service','archive-hall','archive-vault']:
  source=args.source/(name+'.png');im=Image.open(source).convert('RGB')
  # The API returns a widescreen image. Crop symmetrically before resizing,
  # preserving perspective and proportions in the 3:2 runtime room frame.
  cropwidth=int(im.height*1.5);left=(im.width-cropwidth)//2
  im=im.crop((left,0,left+cropwidth,im.height)).resize((1536,1024),Image.Resampling.LANCZOS)
  out=target/(name+'.webp');im.save(out,'WEBP',quality=93,method=6)
  entries.append(dict(path='/escape/v1/'+out.name,bytes=out.stat().st_size,sha256=sha(out),sourceFile=source.name,sourceSHA256=sha(source),width=im.width,height=im.height,alpha=False))
 for name in ['characters','devices',*[f'walk-{i}' for i in range(4)]]:
  source=args.source/(name+'.png');im=Image.open(source).convert('RGBA');cell=(192,192) if name=='devices' else (96,144)
  atlas=Image.new('RGBA',(cell[0]*4,cell[1]*4));frames=[]
  for row in range(4):
   for col in range(4):
    bounds=(round(col*im.width/4),round(row*im.height/4),round((col+1)*im.width/4),round((row+1)*im.height/4))
    sprite=im.crop(bounds)
    if name=='walk-0':sprite=clear_white_matte(sprite)
    mask=sprite.getchannel('A').point(lambda a:255 if a>48 else 0);box=mask.getbbox()
    if not box:raise ValueError('empty sprite '+name+str((row,col)))
    sprite=sprite.crop(box);sprite.thumbnail((cell[0]-12,cell[1]-12),Image.Resampling.LANCZOS)
    x=col*cell[0]+(cell[0]-sprite.width)//2;y=(row+1)*cell[1]-6-sprite.height
    atlas.alpha_composite(sprite,(x,y));frames.append(dict(x=col*cell[0],y=row*cell[1],width=cell[0],height=cell[1],contentWidth=sprite.width,contentHeight=sprite.height))
  out=target/(name+'.webp');atlas.save(out,'WEBP',lossless=True,method=6)
  entries.append(dict(path='/escape/v1/'+out.name,bytes=out.stat().st_size,sha256=sha(out),sourceFile=source.name,sourceSHA256=sha(source),width=atlas.width,height=atlas.height,alpha=True,frames=frames))
 manifest=dict(format=1,model='gpt-image-2.5',mode='imagegen CLI through user-authorized current API',promptFile='prompts.jsonl',assets=entries)
 (HERE/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps(dict(files=len(entries),bytes=sum(e['bytes'] for e in entries))))
if __name__=='__main__':main()
