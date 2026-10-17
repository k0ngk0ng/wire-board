"""Export original player art from static ROM tables, independent of recordings.
The ROM is an offline input only; consumers load PNG sheets and JSON metadata.
"""
from pathlib import Path
from hashlib import sha256
import argparse,json
from PIL import Image
from static_sprites import SpriteROM,DIGEST,render_player
p=argparse.ArgumentParser()
p.add_argument('--rom',type=Path,default=Path('.local/nes-basket/basketball.nes'))
p.add_argument('--colors',type=Path,default=Path('.local/nes-basket/runs/graphics-right/nes-colors.rgb'))
p.add_argument('--output',type=Path,default=Path('.local/basketball/static-atlas'))
args=p.parse_args();rom=SpriteROM(args.rom.read_bytes());colors=args.colors.read_bytes()
args.output.mkdir(parents=True,exist_ok=True)
doc={'accepted_for_release':False,'source':{'rom_sha256':DIGEST,'metasprite_bank':2,'kind_pointer_table':'0x8000','decoder':'fixed 0xD079 / 0xD0C6','head_mapping':'fixed 0xCF30 / 0xD12D','chr_setup':'bank 3 0x82E5..0x8303','palette_setup':'bank 4 0x9526 / 0x97AB','rgb_sha256':sha256(colors).hexdigest()},'pose_counts':[rom.pose_count(i) for i in range(8)],'sheets':[], 'limitations':['Static pose availability does not establish all gameplay actions or transitions.','Character selection metadata/names require separate menu recovery.','Head variants 0..3 exported; extra table entry 4 is not claimed to be a selectable character.','Player-only sheets exclude shadows, ball, hoops and court art.','Current RGB appearance uses the verified original capture renderer palette.']}
for team in range(8):
    palette=bytes(16)+rom.team_palette(team)*2
    pages=[96,97,98,99]+rom.player_chr_pages(team,team)
    count=rom.pose_count(team)
    for character in range(4):
        for facing in (0,128):
            for reverse in (False,True):
                order='reverse' if reverse else 'forward'
                filename=f'team-{team}-character-{character}-facing-{facing}-{order}.png'
                sheet=Image.new('RGBA',(16*128,((count+15)//16)*128));frames=[]
                for pose in range(count):
                    tile=render_player(rom,team,pose|facing,character,0,pages,palette,colors,reverse=reverse,size=(128,128),origin=(64,96))
                    x=(pose%16)*128;y=(pose//16)*128;sheet.paste(tile,(x,y))
                    frames.append({'pose':pose,'rect':[x,y,128,128],'origin':[64,96],'bounds':list(tile.getbbox())})
                file=args.output/filename;sheet.save(file)
                doc['sheets'].append({'team':team,'character':character,'facing':facing,'priority':order,'file':filename,'sha256':sha256(file.read_bytes()).hexdigest(),'frames':frames})
(args.output/'atlas.json').write_text(json.dumps(doc,ensure_ascii=False,indent=2)+'\n')
preview=Image.new('RGB',(8*192,4*128),'#dce6d8')
for team in range(8):
    for character in range(4):
        palette=bytes(16)+rom.team_palette(team)*2;pages=[96,97,98,99]+rom.player_chr_pages(team,team)
        tile=render_player(rom,team,0,character,0,pages,palette,colors)
        crop=tile.crop((16,16,48,56)).resize((96,120),Image.Resampling.NEAREST)
        preview.paste(crop,(team*192+48,character*128),crop)
preview.save(args.output/'team-preview.png')
print(json.dumps({'sheets':len(doc['sheets']),'poses_per_team':doc['pose_counts'],'frames':sum(len(x['frames']) for x in doc['sheets'])}))
