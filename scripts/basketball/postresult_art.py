"""Offline original reward/ending resources, separate from timed presentation.

Logical PPU writes and palette-independent tile pixels remain authoritative;
previews are convenience images, never captured console frames.
"""
from static_sprites import signed
from PIL import Image
from graphics import decode_tile
from result_art import render_table

PRESETS=(26,95,96,97,33,34,25,98,23)
BG_PALETTES=tuple(sorted(set(range(1,16))|{21,33,34,36}))
SPRITE_PALETTES=tuple(sorted(set(range(2,10))|{13,14,29,30,49,50,51}))
POSE_BOUNDS={17:(0x863a,0x872a),18:(0x872a,0x877e),19:(0xc410,0xc44e)}
# Source phase stores plus retained gameplay/team CHR pages. These are source
# page numbers (1 KiB), not contiguous 4 KiB sprite windows.
SPRITE_PAGES=tuple(sorted(set(range(9))|{11,13,37,38,40,56,57,59}))


def decode_background(rom,preset):
    if preset not in PRESETS:raise ValueError('Unsupported post-result background')
    descriptor=rom.word(0xd8c4+preset*2);source=rom.word(descriptor)
    base_id=rom.read(0xdb8b+preset);base=rom.word(0xd8c4+base_id*2)
    bank=rom.read(base+2);target_index=rom.read(base+6)
    if not 0<=target_index<13:raise ValueError('Unknown PPU destination selector')
    target=rom.read(0xdb71+target_index)+256*rom.read(0xdb7e+target_index)
    cursor=source;data=[]
    def read():
        nonlocal cursor
        if 0x8000<=cursor<0xc000:value=rom.prg[3*16384+cursor-0x8000]
        elif 0<=cursor<4096:value=rom.chr[bank*1024+cursor]
        else:raise ValueError('Compressed source left declared window')
        cursor+=1;return value
    for _ in range(4096):
        header=read()
        if header==0:break
        op=header>>6;count=(header&63) or 256
        if op==0:data.extend([read()]*count)
        elif op==1:data.extend(read() for _ in range(count))
        elif op==2:
            repeats=read() or 256;data.extend([read()]*(count*repeats))
        else:
            length=read() or 256;pattern=[read() for _ in range(length)];data.extend(pattern*count)
        if target+len(data)>0x3000:raise ValueError('Background write exceeds nametable window')
    else:raise ValueError('Unterminated original compressed background')
    return dict(preset=preset,descriptor=descriptor,base_preset=base_id,source=source,compressed_bank=bank,target=target,data=data,consumed=cursor-source,field_banks=[rom.read(base+3),rom.read(base+4)],palette_preset=rom.read(base+5),complete_nametable=len(data)==1024 and target%1024==0)


def palette_groups(rom,preset,sprite):
    allowed=SPRITE_PALETTES if sprite else BG_PALETTES
    if preset not in allowed:raise ValueError('Unsupported post-result palette')
    bank=rom.prg[4*16384:5*16384]
    def read(address):
        if not 0x8000<=address<0xc000:raise ValueError('Palette pointer leaves bank4')
        return bank[address-0x8000]
    def word(address):return read(address)+256*read(address+1)
    descriptor=word((0x97ab if sprite else 0x962f)+preset*2)
    return [read(word(descriptor+i*2)+j) for i in range(2 if sprite else 4) for j in range(4)]


def actor_profiles(rom,kind):
    if kind not in POSE_BOUNDS:raise ValueError('Unsupported post-result actor kind')
    start,end=POSE_BOUNDS[kind]
    if rom.word(0x8000+kind*2)!=start:raise ValueError('Actor pointer table changed')
    result=[]
    for pose in range((end-start)//2):
        descriptor=rom.word(start+pose*2);count=rom.read(descriptor)
        if not 1<=count<=64:raise ValueError('Invalid original sprite piece count')
        arrays=[rom.word(descriptor+1+2*i) for i in range(4)]
        result.append([dict(tile=rom.read(arrays[0]+i),attributes=rom.read(arrays[1]+i),x=signed(rom.read(arrays[2]+i)),y=signed(rom.read(arrays[3]+i))-23) for i in reversed(range(count))])
    return result


def index_atlas(rom,pages):
    image=Image.new('L',(512,len(pages)*8))
    for row,page in enumerate(pages):
        for tile in range(64):
            pixels=decode_tile(rom.chr[page*1024+tile*16:page*1024+(tile+1)*16])
            for y,line in enumerate(pixels):
                for x,index in enumerate(line):image.putpixel((tile*8+x,row*8+y),index)
    return image


def background_preview(rom,background,colors):
    if not background['complete_nametable']:raise ValueError('Partial write requires retained nametable and attributes')
    first,second=background['field_banks']
    pattern=rom.chr[first*1024:(first+2)*1024]+rom.chr[second*1024:(second+2)*1024]
    return render_table(background['data'],pattern,palette_groups(rom,background['palette_preset'],False),colors)
