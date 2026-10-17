"""Offline asset decoding only. This module is not a runtime emulator."""
from PIL import Image


def decode_tile(data):
    if len(data)!=16:
        raise ValueError('A NES 2bpp tile requires exactly 16 bytes')
    return [[((data[y]>>(7-x))&1)|(((data[y+8]>>(7-x))&1)<<1)
             for x in range(8)] for y in range(8)]


def sprite_pixels(pattern,tile,attributes,tall,bank):
    if len(pattern)!=8192:
        raise ValueError('A captured pattern table must contain 8192 bytes')
    start=((tile&1)*4096+(tile&0xfe)*16) if tall else bank*4096+tile*16
    pixels=decode_tile(pattern[start:start+16])
    if tall:
        pixels+=decode_tile(pattern[start+16:start+32])
    if attributes&0x80:pixels=pixels[::-1]
    if attributes&0x40:pixels=[row[::-1] for row in pixels]
    return pixels


def render_sprites(ppu,oam,ctrl,colors,scanline_limit=8,clip_left=True):
    if len(ppu)!=0x4000 or len(oam)!=256 or len(colors)!=192:
        raise ValueError('Expected complete PPU/OAM/RGB captures')
    image=Image.new('RGBA',(256,240),(0,0,0,0))
    tall=bool(ctrl&0x20);height=16 if tall else 8
    layers=[]
    for i in range(64):
        y,tile,attributes,x=oam[i*4:i*4+4]
        y+=1
        layers.append((x,y,attributes,sprite_pixels(ppu[:8192],tile,attributes,tall,(ctrl>>3)&1)))
    for y in range(240):
        visible=[s for s in layers if s[1]<=y<s[1]+height][:scanline_limit]
        for x0,y0,attributes,pixels in visible[::-1]:
            for dx,index in enumerate(pixels[y-y0]):
                x=x0+dx
                if index and (8 if clip_left else 0)<=x<256:
                    color=ppu[0x3f10+(attributes&3)*4+index]&0x3f
                    rgb=tuple(colors[color*3:color*3+3])
                    image.putpixel((x,y),rgb+(255,))
    return image


def render_background(ppu,colors,bank=0):
    """Decode the two horizontal nametables, excluding screen-space scrolling."""
    if len(ppu)!=0x4000 or len(colors)!=192:raise ValueError('Incomplete PPU/palette')
    image=Image.new('RGB',(512,240))
    for table in range(2):
        base=0x2000+table*0x400
        for ty in range(30):
            for tx in range(32):
                tile=ppu[base+ty*32+tx]
                start=bank*4096+tile*16
                pixels=decode_tile(ppu[start:start+16])
                attr=ppu[base+0x3c0+(ty//4)*8+tx//4]
                shift=((ty%4)//2)*4+((tx%4)//2)*2
                palette=(attr>>shift)&3
                for dy,row in enumerate(pixels):
                    for dx,index in enumerate(row):
                        color=ppu[0x3f00+(palette*4+index if index else 0)]&0x3f
                        image.putpixel((table*256+tx*8+dx,ty*8+dy),tuple(colors[color*3:color*3+3]))
    return image


def render_scene(field_ppu,hud_ppu,oam,ctrl,colors,scroll):
    """Offline reconstruction of the observed New York field/HUD split.

    Split coordinates are measured from the original raster, not new game rules.
    Unsupported sprite/background priority must fail instead of hiding a mismatch.
    """
    if len(scroll)!=2:raise ValueError('Expected horizontal and vertical scroll')
    for i in range(64):
        y,tile,attr,x=oam[i*4:i*4+4]
        if y<239 and attr&0x20:
            raise ValueError('Background-priority sprite requires additional reconstruction')
    mapped=bytearray(field_ppu)
    mapped[0x2000:0x2400]=field_ppu[0x2800:0x2c00]
    mapped[0x2400:0x2800]=field_ppu[0x2c00:0x3000]
    field=render_background(mapped,colors,0)
    hud=render_background(hud_ppu,colors,0)
    image=Image.new('RGB',(256,240))
    sx,sy=scroll
    for y in range(240):
        for x in range(256):
            color=field.getpixel(((x+sx)%256,(y+sy)%240)) if y<194 else hud.getpixel((x,y-191))
            if x<8 and y<192:color=(0,0,0)
            image.putpixel((x,y),color)
    sprites=render_sprites(hud_ppu,oam,ctrl,colors)
    image.paste(sprites,(0,0),sprites)
    return image
