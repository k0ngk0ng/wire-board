"""Export ball/hoop/prop sheets from original ROM tables, no runtime ROM needed."""
from pathlib import Path
from hashlib import sha256
import argparse,json
from PIL import Image,ImageDraw
from static_sprites import SpriteROM,DIGEST
from static_objects import render_object,object_palette_side
p=argparse.ArgumentParser();p.add_argument('--rom',type=Path,default=Path('.local/nes-basket/basketball.nes'));p.add_argument('--colors',type=Path,default=Path('.local/nes-basket/runs/graphics-right/nes-colors.rgb'));p.add_argument('--output',type=Path,default=Path('.local/basketball/static-objects'));args=p.parse_args()
rom=SpriteROM(args.rom.read_bytes());colors=args.colors.read_bytes();args.output.mkdir(parents=True,exist_ok=True)
keys=[(kind,pose,bank) for kind in range(8,15) for pose in range(rom.pose_count(kind)) for bank in ([rom.ball_chr_bank(pose)] if kind==8 else range(32,36))]
doc={'accepted_for_release':False,'source':{'rom_sha256':DIGEST,'metasprite_bank':2,'decoder':'fixed $D079/$D0C6','ball_chr_selector':'fixed $F049/$F061','attribute_mask':'fixed $CEAB..$CEC0/$D199','palette_table':'bank4 $97AB','rgb_sha256':sha256(colors).hexdigest()},'pose_counts':[rom.pose_count(k) for k in range(8,15)],'ball_chr_banks':[rom.ball_chr_bank(p) for p in range(40)],'selection':{'sheet':'team palette + packed pose facing + OAM priority','frame':'kind + pose + shared fourth sprite CHR bank','palette_side':'0 selects home team, 1 selects away team; special ball action >=12 clears side bit for source seats 0/1, retains it for seats 2/3','ball_hidden_pose_127':'no sprite; select shared CHR page 32','scene_0x86':'retain previous CHR page; do not call ordinary ball selector'},'sheets':[],'limitations':['All kinds 8..14 use one team palette pair per pose; no runtime ROM or palette emulation is necessary.','Only seven previously captured original object images have pixel comparisons.','Unobserved ball effects, object variants and team colors have static table evidence, not natural gameplay image validation.','Gameplay draw mode only; menu/cutscene priority modifications are outside these sheets.','Shadow sprites are separate and are not included.']}
for team in range(8):
 for facing in (0,128):
  for reverse in (False,True):
   order='reverse' if reverse else 'forward';filename=f'palette-{team}-facing-{facing}-{order}.png';sheet=Image.new('RGBA',(16*128,((len(keys)+15)//16)*128));frames=[]
   for ordinal,(kind,pose,bank) in enumerate(keys):
    pages=[96,97,98,99,0,team+1,team+1,bank];palette=bytes(16)+rom.team_palette(team)*2
    tile=render_object(rom,kind,pose|facing,pages,palette,colors,reverse=reverse,size=(128,128),origin=(64,96));x=(ordinal%16)*128;y=(ordinal//16)*128;sheet.paste(tile,(x,y))
    frames.append({'kind':kind,'pose':pose,'chr_bank':bank,'palette_side':object_palette_side(rom,kind,pose),'rect':[x,y,128,128],'origin':[64,96],'bounds':list(tile.getbbox())})
   file=args.output/filename;sheet.save(file);doc['sheets'].append({'team_palette':team,'facing':facing,'priority':order,'file':filename,'sha256':sha256(file.read_bytes()).hexdigest(),'frames':frames})
(args.output/'atlas.json').write_text(json.dumps(doc,indent=2)+'\n')
preview=Image.new('RGB',(16*96,((len(keys)+15)//16)*104),'#dce6d8');draw=ImageDraw.Draw(preview)
for i,(kind,pose,bank) in enumerate(keys):
 tile=render_object(rom,kind,pose,[96,97,98,99,0,1,2,bank],bytes(16)+rom.team_palette(0)+rom.team_palette(1),colors,size=(128,128),origin=(64,96))
 tile=tile.resize((96,96),Image.Resampling.NEAREST);x=i%16*96;y=i//16*104;preview.paste(tile,(x,y),tile);draw.text((x+2,y+90),f'{kind}:{pose}/{bank}',fill='#263d31')
preview.save(args.output/'object-preview.png')
print(json.dumps({'sheets':len(doc['sheets']),'frames':len(keys)*len(doc['sheets']),'pose_counts':doc['pose_counts']}))
