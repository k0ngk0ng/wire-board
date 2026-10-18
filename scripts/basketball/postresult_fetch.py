"""Offline semantic tile-row extraction from original recorded raster writes.

No reference screenshots are read here. The output references static source
CHR tiles and row fragments; it contains no CPU instructions or game logic.
"""
import csv,json
from functools import lru_cache
from pathlib import Path
from PIL import Image
from static_sprites import SpriteROM
from prepare_postresult_raster_reference import state_fields

ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/nes-basket'

@lru_cache(None)
def observed(kind):
 path=BASE/f'runs/postresult-{kind}-observed'
 events=[json.loads(l) for l in (path/'events.jsonl').read_text().splitlines()]
 with (path/'frames.csv').open() as f:frames={int(r['frame']):int(r['emulator_frame']) for r in csv.DictReader(f)}
 return path,events,frames

class Rows:
 def __init__(self,fields,ppu):
  word=lambda k:int.from_bytes(fields[k],'little')
  self.v=word('RADD');self.t=word('TADD');self.fine=fields['XOFF'][0];self.toggle=fields['VTGL'][0]
  self.ctrl=fields['PPUR'][0];self.mask=fields['PPUR'][1];self.reg=fields['CMD'][0]&7;self.regs=list(fields['REGS']);self.mirror=fields['A000'][0]
  self.nt=list(ppu[0x2000:0x3000]);self.pages=[];self.mapping();self.rows=[]
 def mapping(self):
  r=self.regs;self.pages=[r[0]&254,(r[0]&254)+1,r[1]&254,(r[1]&254)+1,*r[2:6]]
 def write(self,e):
  a=e['address'];value=e['value']
  if a==0x2000:self.ctrl=value;self.t=(self.t&0xf3ff)|((value&3)<<10)
  elif a==0x2001:self.mask=value
  elif a==0x2005:
   if self.toggle:self.t=(self.t&0x8c1f)|((value&248)<<2)|((value&7)<<12)
   else:self.t=(self.t&0xffe0)|(value>>3);self.fine=value&7
   self.toggle^=1
  elif a==0x2006:
   if self.toggle:self.t=(self.t&0xff00)|value;self.v=self.t
   else:self.t=(self.t&255)|((value&63)<<8)
   self.toggle^=1
  elif a==0x8000:
   if value&0xc0:raise ValueError('Unsupported mapper bank inversion')
   self.reg=value&7
  elif a==0x8001:
   self.regs[self.reg]=value;self.mapping()
  elif a==0xa000:self.mirror=value&1
 def y_increment(self):
  if self.v&0x7000==0x7000:
   self.v^=0x7000
   if self.v&0x3e0==0x3a0:self.v^=0xba0
   elif self.v&0x3e0==0x3e0:self.v^=0x3e0
   else:self.v+=32
  else:self.v+=0x1000
 def start(self,y):self.y=y;self.fetches=[];self.segments=[];self.fixed=False
 def advance(self,lastpixel):
  # Original reference renderer retires complete eight-dot fetch groups before
  # a register change. Two fetched tile rows precede the current output group;
  # the current fine-X selects across their boundary, independently of v/t.
  end=min(34,max(0,lastpixel//8))
  while len(self.fetches)<end:
   i=len(self.fetches)
   if i>=2:
    for j in range(8):
     off=j+self.fine;prior=self.fetches[i-2+off//8]
     seg=dict(x=(i-2)*8+j,length=1,page=prior['page'],tile=prior['tile'],row=prior['row'],column=off%8,palette=prior['palette'],mask=self.mask)
     if self.segments and all(seg[k]==self.segments[-1][k] for k in ['page','tile','row','palette','mask']) and self.segments[-1]['column']+self.segments[-1]['length']==seg['column']:
      self.segments[-1]['length']+=1
     else:self.segments.append(seg)
   n=(self.v>>10)&3;base=n*1024;x=self.v&31;tile=self.nt[base+(self.v&1023)];attr=self.nt[base+960+(x>>2)+((self.v&0x380)>>4)]
   self.fetches.append(dict(page=self.pages[((self.ctrl&16)<<4|tile)>>6],tile=tile&63,row=(self.v>>12)&7,palette=(attr>>((x&2)+((self.v&0x40)>>4)))&3))
   if self.v&31==31:self.v^=0x41f
   else:self.v+=1
  if lastpixel>=268 and not self.fixed:self.y_increment();self.fixed=True
 def end(self):
  self.advance(272)
  if not self.fixed:self.y_increment();self.fixed=True
  self.rows.append(dict(y=self.y,segments=self.segments,spriteCHR=self.pages[4:].copy(),mask=self.mask))

def derive_frame(kind,frame):
 path,all_events,frames=observed(kind);absolute=frames[frame];ppu=(path/f'ppu-{absolute:05d}-ppu.bin').read_bytes();oam=(path/f'ppu-{absolute:05d}-oam.bin').read_bytes()
 fields=state_fields(BASE/f'runs/postresult-{kind}-raster-states/before-{absolute:05d}.fcs')
 remainder=int.from_bytes(fields['ICou'],'little',signed=True)
 assert remainder%16==0 and fields['DEAD']==b'\0'
 assert not(fields['PPUR'][0]&32),'Only original 8x8 sprite mode is restored'
 base=int.from_bytes(fields['TSBS'],'little')*3+remainder//16
 odd=fields['KOOK'][0];origin=base+22*341-16
 events=[dict(e,dot=e['cycles']*3) for e in all_events if e['frame']==absolute and e['kind']=='deviceWrite'];at=0;r=Rows(fields,ppu)
 while at<len(events) and events[at]['dot']<origin:r.write(events[at]);at+=1
 # Pre-render copies both scroll axes; subsequent lines restore only the
 # horizontal bits after visible tile fetches and vertical advancement.
 r.v=r.t
 for y in range(240):
  start=origin+y*341-(odd if y else 0);end=start+272-(odd if y==0 else 0);r.start(y)
  while at<len(events) and events[at]['dot']<=end:
   e=events[at];r.advance(e['dot']-start);r.write(e);at+=1
  r.end()
  horizontal=end+6
  while at<len(events) and events[at]['dot']<=horizontal:r.write(events[at]);at+=1
  r.v=(r.v&0xfbe0)|(r.t&0x41f)
  next_start=origin+(y+1)*341-odd
  while at<len(events) and events[at]['dot']<next_start:r.write(events[at]);at+=1
 return dict(format=1,kind=kind,frame=frame,emulatorFrame=absolute,clock=dict(originDot=origin,oddFrame=odd),palette=list(ppu[0x3f00:0x3f20]),oam=list(oam),rows=r.rows)

@lru_cache(None)
def art():return SpriteROM((BASE/'basketball.nes').read_bytes()),(BASE/'runs/graphics-right/nes-colors.rgb').read_bytes()

def render_frame(frame):
 rom,colors=art();im=Image.new('RGB',(256,240));palette=frame['palette'];oam=frame['oam']
 def pixel(page,tile,row,col):
  at=page*1024+tile*16+row
  return ((rom.chr[at]>>(7-col))&1)|(((rom.chr[at+8]>>(7-col))&1)<<1)
 for row in frame['rows']:
  y=row['y'];pixels=[0]*256;mask=row['mask'];sprites=[]
  for i in range(64):
   top=oam[i*4]+1
   if top<=y<top+8:sprites.append(i*4)
   if len(sprites)==8:break
  for seg in row['segments']:
   for offset in range(seg['length']):
    x=seg['x']+offset;enabled=(seg['mask']&8) and (x>=8 or seg['mask']&2)
    value=pixel(seg['page'],seg['tile'],seg['row'],seg['column']+offset) if enabled else 0
    pixels[x]=(seg['palette']*4+value) if value else 0
  for x in range(256):
   value=pixels[x]
   if mask&16 and (x>=8 or mask&4):
    for at in sprites:
     left=oam[at+3]
     if not left<=x<left+8:continue
     attr=oam[at+2];tile=oam[at+1];dx=x-left;dy=y-oam[at]-1
     sp=pixel(row['spriteCHR'][tile>>6],tile&63,7-dy if attr&128 else dy,7-dx if attr&64 else dx)
     if not sp:continue
     if not(attr&32) or value==0:value=16+(attr&3)*4+sp
     break
   color=palette[value]&63
   if mask&1:color&=48
   im.putpixel((x,y),tuple(colors[color*3:color*3+3]))
 return im
