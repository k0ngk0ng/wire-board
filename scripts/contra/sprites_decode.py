"""Source-bound Japanese meta-sprite decoder and NES sprite pixel projection.

Independent data interpretation for asset recovery; not a CPU/PPU emulator and
not the shipping game renderer. SpriteBank follows REA $9284..$936D semantics.
"""
from functools import lru_cache
from PIL import Image


def signed8(value):
    return value-256 if value&128 else value


class SpriteBank:
    def __init__(self, prg):
        self.prg=prg
        assert len(prg)==131072

    def word(self,address):
        assert 0x8000<=address<0xbfff
        return int.from_bytes(self.prg[address:address+2],'little')

    @lru_cache(maxsize=255)
    def sprite(self,code):
        assert 1<=code<=255
        pointer=self.word(0x93da+(code-1)*2)
        assert 0x95d8<=pointer<0xc000
        header=self.prg[pointer]
        if header==0:
            return {'code':code,'address':pointer,'kind':'blank','parts':[]}
        if header==0xfe:
            return {'code':code,'address':pointer,'kind':'compact','parts':[
                {'dy':-8,'tile':self.prg[pointer+1],'attr':self.prg[pointer+2],'dx':-4,'recoil':False}]}
        assert header<64,(code,header)
        cursor=pointer+1;recoil=True;parts=[];visited=set()
        while len(parts)<header:
            assert 0x95d8<=cursor<0xbffd
            if self.prg[cursor]==0x80:
                assert cursor not in visited,'cyclic shared sprite'
                visited.add(cursor)
                cursor=self.word(cursor+1)
                recoil=False
            y,tile,attr,x=self.prg[cursor:cursor+4]
            parts.append({'dy':signed8(y),'tile':tile,'attr':attr,'dx':signed8(x),'recoil':recoil})
            cursor+=4
        return {'code':code,'address':pointer,'kind':'regular','parts':parts}

    def compose(self,row,before):
        sprite=self.sprite(row['code']);buffer=bytearray(before)
        offset,remaining=row['offset'],row['remaining']
        flags,x,y=row['flags'],row['x'],row['y']
        assert len(buffer)==256 and offset%4==0 and row['stride'] in (4,196)
        for part in sprite['parts']:
            if sprite['kind']=='compact':
                py=(y-8)&255;attribute=part['attr']|flags;px=x-4
            else:
                effects=flags&0xc8
                if not part['recoil']:effects&=0xf7
                dy=(part['dy']+(1 if effects&8 else 0))&255
                if effects&128:dy=(0xf0-dy)&255
                py=(y+dy)&255
                mask=0xfc if flags&4 else 0xff
                merged=flags&(0x23 if flags&4 else 0x20)
                attribute=((part['attr']&mask)|merged)^effects
                dx=part['dx']&255
                if effects&64:dx=(0xf8-dx)&255
                px=x+signed8(dx)
            # The original writes these bytes before testing horizontal clipping.
            # A clipped trailing part leaves a partial slot for the outer filler.
            buffer[offset:offset+3]=bytes([py,part['tile'],attribute])
            if 0<=px<256:
                buffer[offset+3]=px
                remaining=(remaining-1)&255
                offset=(offset+row['stride'])&255
        return bytes(buffer),offset,remaining


def render_oam(oam,ppu,colors,ctrl,mask):
    """Project actual DMA using current CHR/palette, NES priority and scanline cap.

    The foreground mask excludes behind-background pixels; this is deliberately
    not a whole PPU renderer. Background occlusion remains the future renderer's
    responsibility. Color conversion comes from the recording emulator palette.
    """
    assert len(oam)==256 and len(ppu)==0x4000 and len(colors)==192
    image=Image.new('RGBA',(256,240),(0,0,0,0));front=bytearray(256*240)
    if not mask&16:return image,front
    height=16 if ctrl&32 else 8
    for scanline in range(240):
        candidates=[]
        for i in range(64):
            y,tile,attr,x=oam[i*4:i*4+4]
            top=y+1
            if top<=scanline<top+height:
                candidates.append((y,tile,attr,x))
                if len(candidates)==8:break
        for y,tile,attr,x in reversed(candidates):
            row=scanline-y-1
            if attr&128:row=height-1-row
            if height==16:
                address=(tile&1)*4096+(tile&254)*16+(row//8)*16+row%8
            else:
                address=(4096 if ctrl&8 else 0)+tile*16+row
            lo,hi=ppu[address],ppu[address+8]
            for col in range(8):
                xx=x+col
                if xx>=256 or xx<8 and not mask&4:continue
                shift=col if attr&64 else 7-col
                value=((lo>>shift)&1)|(((hi>>shift)&1)<<1)
                if value==0:continue
                color=ppu[0x3f10+(attr&3)*4+value]&63
                rgb=tuple(colors[color*3:color*3+3])
                image.putpixel((xx,scanline),rgb+(255,))
                front[scanline*256+xx]=0 if attr&32 else 1
    return image,front
