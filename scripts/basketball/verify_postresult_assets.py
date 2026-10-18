"""Pre-implementation source oracle and exported-resource contract.
Failure inventory: games/basketball/POSTRESULT_ART_VERIFICATION.md.
No gameplay, outer schedule, text, IRQ, fades or earned victory claimed.
"""
import argparse, hashlib, json
from pathlib import Path
from PIL import Image
from static_sprites import SpriteROM, DIGEST
from verify_pass_floor_original import Memory, MPU

ROOT=Path(__file__).resolve().parents[2]
PRESETS=(26,95,96,97,33,34,25,98,23)
BG_PALETTES=tuple(sorted(set(range(1,16))|{21,33,34,36,6}))
SPRITE_PALETTES=tuple(sorted(set(range(2,10))|{13,14,29,30,49,50,51}))
KINDS={17:120,18:42,19:31}

class PPUAccess(Memory):
    """Only the read-buffer/address/upload boundary; no frame simulation."""
    def __init__(self, rom):
        super().__init__(rom.prg,bytes(2048));self.rom=rom;self.ppu=bytearray(16384)
        self.address=0;self.high=True;self.buffer=0;self.writes=[]
    def __getitem__(self,a):
        if a==0x2002:self.high=True;return 0
        if a==0x2007:
            old=self.buffer;self.buffer=self.ppu[self.address&0x3fff];self.address=(self.address+1)&0x3fff;return old
        return super().__getitem__(a)
    def __setitem__(self,a,v):
        if a==0x2006:
            self.address=((v&63)<<8)|(self.address&255) if self.high else (self.address&0xff00)|v;self.high=not self.high;return
        if a==0x2007:
            self.writes.append([self.address,v]);self.ppu[self.address&0x3fff]=v;self.address=(self.address+1)&0x3fff;return
        if a in (0x2000,0x2001):return
        super().__setitem__(a,v)

def execute(m,entry,stop=None,limit=200000):
    c=MPU(memory=m,pc=entry);c.sp=255;c.p=32;c.stPushWord(0x6fff)
    for n in range(limit):
        if c.pc in (0x7000,stop):return n
        if c.pc==0xfa0b:
            m.ppu[:4096]=m.rom.chr[m[0x4f]*1024:(m[0x4f]+4)*1024]
            c.pc=(c.stPopWord()+1)&65535
        else:c.step()
    raise AssertionError(('source instruction bound',hex(c.pc)))

def background_oracle(rom,preset):
    m=PPUAccess(rom);m[0x8000:0xc000]=rom.prg[3*16384:4*16384];m[0x3a0]=preset
    execute(m,0xdc15,0xdc96)
    addresses=[a for a,_ in m.writes];assert addresses==list(range(addresses[0],addresses[0]+len(addresses)))
    return dict(target=addresses[0],data=[v for _,v in m.writes],field_banks=[m[0x25],m[0x26]],palette_preset=m[0x27])

def profiles_oracle(rom,kind,count):
    profiles=[]
    for pose in range(count):
        m=Memory(rom.prg,bytes(2048));m[0x8000:0xc000]=rom.prg[2*16384:3*16384];m[0x3d6]=kind;m[0x5b]=pose
        execute(m,0xd079);pieces=[]
        while True:
            execute(m,0xd0c6)
            signed=lambda x:x-256 if x>=128 else x
            pieces.append(dict(tile=m[0x19],attributes=m[0x1a],x=signed(m[0x1b]),y=signed(m[0x18])-23))
            if m[0x27]==0:break
            m[0x27]-=1
        profiles.append(pieces)
    return profiles

def palettes_oracle(rom,bg,sprite0,sprite1):
    m=Memory(rom.prg,bytes(2048));m[0x8000:0xc000]=rom.prg[4*16384:5*16384]
    m[0x303]=0x40;m[0x304]=bg;m[0x305]=sprite0;m[0x306]=sprite1
    execute(m,0x9526,0x9619)
    return list(m[0x377:0x397])

def main():
    p=argparse.ArgumentParser();p.add_argument('directory',type=Path);args=p.parse_args()
    # Import after parsing so the initial expected red is explicit and repeatable.
    from postresult_art import decode_background,actor_profiles,palette_groups
    rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes())
    m=json.loads((args.directory/'manifest.json').read_text())
    assert m['source']['rom_sha256']==DIGEST and m['accepted_for_release'] is False
    assert [b['preset'] for b in m['backgrounds']]==list(PRESETS)
    for b in m['backgrounds']:
        expected=background_oracle(rom,b['preset']);got=decode_background(rom,b['preset'])
        for key,value in expected.items():assert b[key]==got[key]==value,(b['preset'],key)
    for kind,count in KINDS.items():
        expected=profiles_oracle(rom,kind,count)
        assert m['actorProfiles'][str(kind)]==actor_profiles(rom,kind)==expected
    palette_calls=0
    for bg in BG_PALETTES:
        for spr in SPRITE_PALETTES:
            raw_bg=palette_groups(rom,bg,False);raw_spr=palette_groups(rom,spr,True)
            expected=palettes_oracle(rom,bg,spr,spr);palette_calls+=1
            assert m['backgroundPalettes'][str(bg)]==raw_bg
            assert m['spritePalettes'][str(spr)]==raw_spr
            got=raw_bg+raw_spr*2
            for i in range(4,32,4):got[i]=got[0]
            assert got==expected,(bg,spr)
    for record in m['assets']:
        file=args.directory/record['file'];assert hashlib.sha256(file.read_bytes()).hexdigest()==record['sha256']
        im=Image.open(file);assert list(im.size)==record['size'] and im.mode==record['mode']
    atlas=Image.open(args.directory/m['indexAtlas']['file'])
    for row,page in enumerate(m['indexAtlas']['pages']):
        for tile in range(64):
            raw=rom.chr[page*1024+tile*16:page*1024+tile*16+16]
            for y in range(8):
                for x in range(8):
                    expected=((raw[y]>>(7-x))&1)|(((raw[y+8]>>(7-x))&1)<<1)
                    assert atlas.getpixel((tile*8+x,row*8+y))==expected
    result=dict(status='passed',backgrounds=len(PRESETS),poses=sum(KINDS.values()),palette_calls=palette_calls,assets=len(m['assets']),accepted_for_release=False)
    print(json.dumps(result))
if __name__=='__main__':main()
