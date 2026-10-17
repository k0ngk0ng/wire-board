"""Offline semantic decoder of original bank-2 metasprites.

Source: fixed $D079/$D0C6, $CF30..$CFFF. No CPU/PPU emulation,
no OAM replay, and no screenshot-dependent pose geometry.
"""
from hashlib import sha256
from PIL import Image
from graphics import decode_tile

DIGEST='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'

class SpriteROM:
    def __init__(self,data):
        if len(data)!=262160 or sha256(data).hexdigest()!=DIGEST:
            raise ValueError('Expected the pinned original basketball ROM')
        self.prg=data[16:131088];self.chr=data[131088:]
        self.bank=self.prg[32768:49152]
    def read(self,address):
        if 0x8000<=address<0xc000:return self.bank[address-0x8000]
        if 0xc000<=address<=0xffff:return self.prg[-16384+address-0xc000]
        raise ValueError('Metasprite pointer is outside original PRG')
    def word(self,address):return self.read(address)+256*self.read(address+1)
    def pose_count(self,kind):
        if not 0<=kind<19:raise ValueError('Unsupported sprite kind')
        end=self.word(0x8002+2*kind) if kind<18 else 0x877e
        return (end-self.word(0x8000+2*kind))//2
    def pieces(self,kind,pose):
        if not 0<=pose<self.pose_count(kind):raise ValueError('Pose exceeds its kind table')
        address=self.word(self.word(0x8000+2*kind)+pose*2)
        count=self.read(address)
        if not 1<=count<=64:raise ValueError('Invalid original sprite piece count')
        arrays=[self.word(address+1+2*i) for i in range(4)]
        return [tuple(self.read(p+i) for p in arrays) for i in reversed(range(count))]
    def ball_chr_bank(self,packed_pose):
        # Fixed $F049, after the world draw list is built. Hidden pose 127
        # chooses ordinary page 32. Scene 0x86 retains the previous page and
        # must be handled by the caller rather than inventing a new selection.
        if not 0<=packed_pose<=255:raise ValueError('Invalid packed ball pose')
        pose=packed_pose&127
        if pose==127:pose=0
        if pose>=40:raise ValueError('Ball pose exceeds the original CHR selector')
        return self.read(0xf061+pose)
    def team_palette(self,team):
        if not 0<=team<8:raise ValueError('Unknown team')
        bank=self.prg[65536:81920]
        read=lambda a:bank[a-0x8000]
        word=lambda a:read(a)+256*read(a+1)
        table=word(0x97ab+(team+2)*2)
        # $9526 copies two four-color sprite palettes per team. Universal
        # background entries are normalized later and never render sprites.
        return bytes(read(word(table+2*i)+j) for i in range(2) for j in range(4))
    def player_chr_pages(self,home,away):
        if not 0<=home<8 or not 0<=away<8:raise ValueError('Unknown team')
        return [0,home+1,away+1,32]
    def identify_chr_pages(self,pattern):
        if len(pattern)!=8192:raise ValueError('Expected complete captured CHR mapping')
        pages=[]
        for offset in range(0,8192,1024):
            matches=[i for i in range(128) if self.chr[i*1024:(i+1)*1024]==pattern[offset:offset+1024]]
            if len(matches)!=1:raise ValueError('CHR page does not identify uniquely')
            pages.append(matches[0])
        return pages


def signed(value):return value-256 if value>=128 else value


def render_player(rom,kind,packed_pose,character,owner,chr_pages,palette,colors,reverse=False,size=(64,64),origin=(32,56)):
    if not 0<=owner<4 or not 0<=character<5:raise ValueError('Unsupported player/character slot')
    if len(chr_pages)!=8 or any(not 0<=p<128 for p in chr_pages):raise ValueError('Expected eight CHR page numbers')
    if len(palette)!=32 or len(colors)!=192:raise ValueError('Incomplete palette')
    pattern=b''.join(rom.chr[p*1024:(p+1)*1024] for p in chr_pages)
    pieces=rom.pieces(kind,packed_pose&127)
    if reverse:pieces=pieces[::-1]
    image=Image.new('RGBA',size)
    # OAM's first piece wins overlapping pixels. Paint in reverse order.
    for tile,attr,x,y in pieces[::-1]:
        if tile<4:
            table=rom.word(0xd12d+tile*2)
            tile=(tile+rom.read(table+character+(5 if owner>=2 else 0)))&255
        elif tile&0xc0==0x40:
            tile=(tile+(0x40 if owner>=2 else 0))&255
        if attr&3<2:attr=(attr+(2 if owner>=2 else 0))&255
        x=signed(x)
        if not packed_pose&128:
            x=-x-8;attr^=0x40
        pixels=decode_tile(pattern[4096+tile*16:4096+(tile+1)*16])
        if attr&128:pixels=pixels[::-1]
        if attr&64:pixels=[row[::-1] for row in pixels]
        for dy,row in enumerate(pixels):
            for dx,index in enumerate(row):
                if not index:continue
                px=origin[0]+x+dx;py=origin[1]+signed(y)-23+dy
                if not (0<=px<size[0] and 0<=py<size[1]):raise ValueError('Original pose exceeds fixed canvas')
                color=palette[16+(attr&3)*4+index]&63
                image.putpixel((px,py),tuple(colors[color*3:color*3+3])+(255,))
    return image
