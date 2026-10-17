"""Original result-screen presets and big-score art, decoded offline from ROM.
DBF8's data stream is in temporary background CHR pages, not a screenshot.
"""
from PIL import Image,ImageDraw
from static_court import paint_tile


def decode_background(rom,preset):
 if preset not in [99,*range(101,109)]:raise ValueError('Unsupported result background preset')
 descriptor=rom.word(0xd8c4+preset*2);source=rom.word(descriptor)
 base_preset=rom.read(0xdb8b+preset);base=rom.word(0xd8c4+base_preset*2)
 compressed_bank=rom.read(base+2);target_index=rom.read(base+6)
 target=rom.read(0xdb71+target_index)+256*rom.read(0xdb7e+target_index)
 cursor=source;output=[]
 def read():
  nonlocal cursor
  if cursor>=0x8000:
   value=rom.prg[3*16384+cursor-0x8000]
  elif cursor<4096:
   value=rom.chr[compressed_bank*1024+cursor]
  else:raise ValueError('Result source left its mapped background CHR window')
  cursor+=1;return value
 for _ in range(4096):
  header=read()
  if header==0:break
  op=header>>6;count=header&63;count=count or 256
  if op==0:output.extend([read()]*count)
  elif op==1:output.extend(read() for _ in range(count))
  elif op==2:
   repeats=read() or 256;value=read();output.extend([value]*(count*repeats))
  else:
   length=read() or 256;pattern=[read() for _ in range(length)];output.extend(pattern*count)
  if len(output)>16384:raise ValueError('Result decompression exceeds PPU capacity')
 else:raise ValueError('Unterminated result background')
 return {'preset':preset,'descriptor':descriptor,'source':source,'compressed_bank':compressed_bank,'target':target,'data':output,'consumed':cursor-source,'field_banks':[rom.read(base+3),rom.read(base+4)],'palette_preset':rom.read(base+5)}


def palette_for_court(rom,court,palette_id=None):
 palette_id=court+1 if palette_id is None else palette_id
 if not 1<=palette_id<=15:raise ValueError("Unknown original court palette")
 bank=rom.prg[65536:81920]
 def word(a):return int.from_bytes(bank[a-0x8000:a-0x8000+2],'little')
 table=word(0x962f+palette_id*2)
 return bytes(bank[word(table+i*2)-0x8000+j] for i in range(4) for j in range(4))


def render_table(data,pattern,palette,colors):
 if len(data)!=1024:raise ValueError('Expected one complete result nametable')
 image=Image.new('RGBA',(256,240))
 for y in range(30):
  for x in range(32):
   attr=data[960+(y//4)*8+x//4];group=(attr>>(((y%4)//2)*4+((x%4)//2)*2))&3
   paint_tile(image,x*8,y*8,pattern,data[y*32+x],palette,colors,group)
 return image


def build_result_images(rom,court,colors,palette_id=None):
 if not 0<=court<8:raise ValueError('Unknown original court')
 field=decode_background(rom,court+101);score=decode_background(rom,99);palette=palette_for_court(rom,court,palette_id=palette_id)
 bank4=rom.prg[65536:81920];field_bank=bank4[0x3ba3+court]
 background=render_table(field['data'],rom.chr[field_bank*1024:(field_bank+4)*1024],palette,colors)
 hud_pattern=rom.chr[96*1024:98*1024]+rom.chr[14*1024:16*1024]
 if len(score['data'])!=192:raise ValueError('Result HUD must write exactly six tile rows')
 # Preset 99 replaces six rows only; the normal sky tile has a blank first
 # HUD scanline. Browser composition retains the live ring for that last line.
 hud_data=score['data']+[1]*(960-len(score['data']))+[0]*64
 template=render_table(hud_data,hud_pattern,palette,colors).crop((0,0,256,49))
 for x in [64,144]:ImageDraw.Draw(template).rectangle((x,8,x+47,31),fill=(0,0,0,0))
 digits=Image.new('RGBA',(160,24));digit_bases=list(bank4[0x3d82:0x3d8c])
 for digit,base in enumerate(digit_bases):
  for row in range(3):
   for col in range(2):paint_tile(digits,digit*16+col*8,row*8,hud_pattern,base+row*16+col,palette,colors,0)
 return {'background':background,'hud':template,'digits':digits,'field_source':field,'hud_source':score,'digit_bases':digit_bases}


def compose_result_background(images,digits,camera):
 if camera!={'x':0,'y':296}:raise ValueError('Result camera differs from restored original layout')
 if len(digits)!=2 or any(len(side)!=3 or any(not 0<=d<10 for d in side) for side in digits):raise ValueError('Expected two original three-digit score caches')
 image=Image.new('RGBA',(256,240),(0,0,0,255));image.paste(images['background'].crop((0,40,256,234)),(0,0));ImageDraw.Draw(image).rectangle((0,0,7,191),fill=(0,0,0,255))
 hud=images['hud'].crop((0,3,256,49));image.paste(hud,(0,194),hud)
 for side,values in enumerate(digits):
  for column,digit in enumerate(values):image.paste(images['digits'].crop((digit*16,0,digit*16+16,24)),(64+side*80+column*16,199))
 return image
