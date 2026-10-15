#!/usr/bin/env python3
"""Research contact sheet made from the verified CC0 Racing Pack, not game UI.
Run with Pillow: python make-preview.py --racing-zip PATH [--font CHINESE_FONT]
"""
import argparse, colorsys, hashlib, json
from io import BytesIO
from pathlib import Path
from zipfile import ZipFile
from PIL import Image, ImageDraw, ImageFont

root=Path(__file__).resolve().parents[3]
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--racing-zip',type=Path,default=root/'.local/racing-research/images/downloads/kenney-racing-pack.zip')
p.add_argument('--font',default='/System/Library/Fonts/STHeiti Medium.ttc')
a=p.parse_args()
out=Path(__file__).resolve().parent
W,H=1320,1000
im=Image.new('RGB',(W,H),'#f3f0e6');d=ImageDraw.Draw(im)
font=lambda size:ImageFont.truetype(a.font,size)
used=[]
with ZipFile(a.racing_zip) as z:
 def sprite(name):
  raw=z.read(name);used.append({'member':name,'sha256':hashlib.sha256(raw).hexdigest()})
  return Image.open(BytesIO(raw)).convert('RGBA')
 def place(image,box,angle=0):
  image=image.rotate(angle,expand=True,resample=Image.Resampling.BICUBIC)
  image.thumbnail((box[2]-box[0],box[3]-box[1]),Image.Resampling.LANCZOS)
  im.paste(image,(round((box[0]+box[2]-image.width)/2),round((box[1]+box[3]-image.height)/2)),image)
 def label(x,y,text,size=21,color='#254d40'):
  d.text((x,y),text,font=font(size),fill=color)
 def card(x,y,w,h):
  d.rounded_rectangle((x,y,x+w,y+h),radius=20,fill='#fffdf5',outline='#c8d2bf',width=2)
 label(42,30,'赛车素材实样',38)
 label(42,85,'Kenney · CC0 俯视卡通素材；仅供选材，非游戏截图',22,'#52634f')
 label(42,140,'车辆与六个席位',26)
 names=['蓝','橙红','绿','黄','黑','紫（改色示例）']
 colors=['blue','red','green','yellow','black','blue']
 for i,(color,name) in enumerate(zip(colors,names)):
  x=42+i*207;card(x,185,194,225)
  car=sprite(f'PNG/Cars/car_{color}_1.png')
  if i==5:
   pixels=car.load()
   for yy in range(car.height):
    for xx in range(car.width):
     r,g,b,alpha=pixels[xx,yy];h,s,v=colorsys.rgb_to_hsv(r/255,g/255,b/255)
     if alpha and .48<h<.66 and s>.3:
      rr,gg,bb=colorsys.hsv_to_rgb(.77,s*.82,v)
      pixels[xx,yy]=(round(rr*255),round(gg*255),round(bb*255),alpha)
  place(car,(x+20,201,x+174,355),-12)
  label(x+15,371,f'{i+1:02d}  {name}',20)
 label(42,440,'道路拼块：宽弯、连续弯、发卡弯可分别设计',25)
 tiles=[('PNG/Tiles/Asphalt road/road_asphalt02.png','沥青直道'),('PNG/Tiles/Asphalt road/road_asphalt07.png','弯道拼块'),('PNG/Tiles/Asphalt road/road_asphalt42.png','起终点标记'),('PNG/Tiles/Asphalt road/road_asphalt68.png','斜向道路')]
 # The exact dirt/sand names are discovered from the original ZIP listing.
 for token,title in [('road_dirt02.png','泥地弯道'),('road_sand02.png','沙地弯道')]:
  member=next(n for n in z.namelist() if n.endswith('/'+token));tiles.append((member,title))
 for i,(name,title) in enumerate(tiles):
  x=42+i*207;card(x,485,194,195);place(sprite(name),(x+28,500,x+166,638));label(x+20,647,title,20)
 label(42,710,'场景与效果素材',25)
 objects=[('tree_large.png','树木'),('cone_straight.png','路锥'),('barrier_red_race.png','护栏'),('skidmark_long_1.png','轮胎印'),('tribune_full.png','观众看台'),('lights.png','发车灯')]
 for i,(name,title) in enumerate(objects):
  x=42+i*207;card(x,755,194,170);place(sprite('PNG/Objects/'+name),(x+24,765,x+170,872));label(x+20,893,title,20)
 label(42,950,'车辆底层性能统一；第六种配色为基于 CC0 车图的衍生示例。封面另绘原创 SVG。',19,'#52634f')
file=out/'material-board.png';im.save(file,optimize=True)
(out/'material-board.sources.json').write_text(json.dumps({'status':'research-preview-not-game','sourceZip':str(a.racing_zip.relative_to(root)) if a.racing_zip.is_relative_to(root) else str(a.racing_zip),'sourceSha256':hashlib.sha256(a.racing_zip.read_bytes()).hexdigest(),'output':file.name,'outputSha256':hashlib.sha256(file.read_bytes()).hexdigest(),'license':'CC0-1.0','derived':'sixth car recolored from blue; no gameplay stats changed or implemented','members':used},ensure_ascii=False,indent=2)+'\n')
print(file)
