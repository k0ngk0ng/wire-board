"""Recover independent, foot-anchored character/ball frames from draw ownership.

Uses original OAM construction's X register to identify owners. It never cuts
characters out of composed screenshots and never invents unseen pose pixels.
"""
from pathlib import Path
from hashlib import sha256
import argparse,json
from PIL import Image,ImageDraw
from graphics import render_sprites

p=argparse.ArgumentParser();p.add_argument('--captures',type=Path,default=Path('.local/nes-basket/runs'));p.add_argument('--output',type=Path,default=Path('.local/basketball/atlas'))
args=p.parse_args();args.output.mkdir(parents=True,exist_ok=True)
frames={};skipped=[]
for directory in sorted(args.captures.glob('graphics-*')):
    if not (directory/'nes-colors.rgb').exists():continue
    colors=(directory/'nes-colors.rgb').read_bytes()
    for info in sorted(directory.glob('ppu-*-objects.json')):
        stem=info.name.removesuffix('-objects.json')
        states=json.loads(info.read_text());ppu=(directory/(stem+'-ppu.bin')).read_bytes()
        oam=(directory/(stem+'-oam.bin')).read_bytes();owners=(directory/(stem+'-owners.bin')).read_bytes()
        ctrl=(directory/(stem+'-ctrl.bin')).read_bytes()[0]
        scroll_file=directory/(stem+'-scroll.bin')
        if not scroll_file.exists():continue
        order_file=directory/(stem+'-order.bin')
        if not order_file.exists():continue
        priority='reverse' if order_file.read_bytes()[0] else 'forward'
        sx,sy=scroll_file.read_bytes()
        for owner in sorted(set(owners)-{255}):
            if str(owner) not in states:continue
            state=states[str(owner)]
            indices=[i for i in range(64) if owners[i]==owner and oam[i*4]<239 and oam[i*4+1]!=0x2f]
            if not indices:continue
            source={'capture':directory.name,'frame':int(stem.split('-')[1]),'object':owner,'state':state,'sprite_indices':indices,'priority':priority}
            # Select the on-screen representative of the horizontal wrap.
            anchor_x=(state['x']//256-sx)%256
            z=state['z'];z=z-(1<<24) if z&(1<<23) else z
            anchor_y=248-sy-state['y']//256-z//256
            if anchor_x<24 or anchor_x>232 or anchor_y<48 or anchor_y>232:
                skipped.append({**source,'reason':'pose may be clipped by screen edge'});continue
            own=bytearray(oam)
            for i in range(64):
                if i not in indices:own[i*4]=255
            layer=render_sprites(ppu,own,ctrl,colors,scanline_limit=64,clip_left=False)
            bbox=layer.getbbox()
            if bbox is None:continue
            left,top=anchor_x-32,anchor_y-56
            if bbox[0]<left or bbox[1]<top or bbox[2]>left+64 or bbox[3]>top+64:
                skipped.append({**source,'reason':'observed pieces exceed foot-anchored frame; pose needs investigation'});continue
            sprite=layer.crop((left,top,left+64,top+64))
            identity=sha256(bytes([owner])+sprite.tobytes()).hexdigest()
            if identity not in frames:
                filename=f'object-{owner}-{identity[:16]}.png';file=args.output/filename;sprite.save(file)
                frames[identity]={'id':identity,'object':owner,'file':filename,'sha256':sha256(file.read_bytes()).hexdigest(),'origin':[32,56],'sources':[]}
            frames[identity]['sources'].append(source)
doc={'status':'partial','method':'original draw routine ownership, palette and CHR; no screenshot cutouts','frames':list(frames.values()),'skipped':skipped,
     'limitations':['only sampled New York scene and characters','shadow separated by observed tile 0x2f; not yet a complete shadow atlas','full action and AI coverage remains incomplete']}
(args.output/'atlas.json').write_text(json.dumps(doc,ensure_ascii=False,indent=2)+'\n')
primary={}
for entry in doc['frames']:
    if entry['object']!=0:continue
    for source in entry['sources']:
        key=f"{source['state']['ram005b']}:{source['priority']}"
        value={'file':entry['file'],'sha256':entry['sha256'],'origin':entry['origin']}
        if key in primary:assert primary[key]==value,'Ambiguous primary-player pose'
        primary[key]=value
(args.output/'kunio-frames.json').write_text(json.dumps({'character':'kunio','status':'partial','key':'packed pose/facing byte:OAM priority direction','frames':primary},indent=2)+'\n')
ordered=sorted(doc['frames'],key=lambda x:(x['object'],x['id']))
cols=8;rows=(len(ordered)+cols-1)//cols
sheet=Image.new('RGB',(cols*128,rows*142),'#dce6d8');draw=ImageDraw.Draw(sheet)
for i,entry in enumerate(ordered):
    x=(i%cols)*128;y=(i//cols)*142;im=Image.open(args.output/entry['file']).resize((128,128),Image.Resampling.NEAREST)
    sheet.paste(im,(x,y+14),im);draw.text((x+4,y+2),f"object {entry['object']} / {i+1}",fill='#263d31')
sheet.save(args.output/'contact-sheet.png')
print(json.dumps({'frames':len(frames),'objects':sorted({x['object'] for x in frames.values()}),'skipped_partial_poses':len(skipped)}))
