"""Static original court and HUD art, decoded from original ROM data tables.

The court comes from row pointers, never screenshots or captured nametables.
HUD display values remain separate from its static border and original glyphs.
"""
from PIL import Image
from graphics import decode_tile

def validate_court(court):
 if type(court) is not int or not 0<=court<8:raise ValueError("Unknown original court")


def background_palette(rom,court=1,palette_id=None):
 validate_court(court)
 bank=rom.prg[65536:81920]
 def word(a):return int.from_bytes(bank[a-0x8000:a-0x8000+2],'little')
 palette_id=court+1 if palette_id is None else palette_id
 if not 1<=palette_id<=15:raise ValueError("Unknown court palette preset")
 descriptor=word(0x962f+palette_id*2) # D231 selects original court+1 palette preset.
 return bytes(bank[word(descriptor+i*2)-0x8000+j] for i in range(4) for j in range(4))


def court_tile_layers(rom,court=1):
 validate_court(court)
 bank=rom.prg[16384:32768]
 def word(a):return int.from_bytes(bank[a-0x8000:a-0x8000+2],'little')
 def read(a):return bank[a-0x8000]
 table,attributes=word(0x800c+court*2),word(0x801c+court*2)
 layers=[]
 for layer in range(2):
  tiles=[[0]*64 for _ in range(30)];palettes=[[0]*64 for _ in range(30)];attrs=[]
  for page in range(2):
   rows=word(table+layer*4+page*2);ap=word(attributes+layer*4+page*2);a=[read(ap+i) for i in range(64)];attrs.append(a)
   for y in range(30):
    row=word(rows+y*2)
    for x in range(32):
     tiles[y][page*32+x]=read(row+x)
     palettes[y][page*32+x]=(a[(y//4)*8+x//4]>>(((y%4)//2)*4+((x%4)//2)*2))&3
  layers.append(dict(tiles=tiles,palettes=palettes,attributes=attrs))
 return layers


def paint_tile(image,x,y,pattern,tile,palette,colors,group):
 for dy,row in enumerate(decode_tile(pattern[tile*16:(tile+1)*16])):
  for dx,index in enumerate(row):
   color=palette[group*4+index if index else 0]&63
   image.putpixel((x+dx,y+dy),tuple(colors[color*3:color*3+3])+(255,))


def build_court(rom,colors,court=1,palette_id=None):
 validate_court(court)
 if len(colors)!=192:raise ValueError('Expected original RGB palette')
 layers=court_tile_layers(rom,court=court);palette=background_palette(rom,court=court,palette_id=palette_id)
 # Bank1 $BEC5[court] selects the two 2 KiB background CHR banks.
 first=rom.prg[16384+0x3ec5+court];pattern=rom.chr[first*1024:(first+4)*1024]
 complete=Image.new('RGBA',(512,480))
 for layer,data in enumerate(layers):
  for y in range(30):
   for x in range(64):paint_tile(complete,x*8,layer*240+y*8,pattern,data['tiles'][y][x],palette,colors,data['palettes'][y][x])
 # The original upper 48px hold HUD template data, not world scenery.
 return complete.crop((0,48,512,480))


def build_hud_glyphs(rom,colors,court=1,palette_id=None):
 validate_court(court)
 if len(colors)!=192:raise ValueError('Expected original RGB palette')
 pattern=rom.chr[96*1024:100*1024];palette=background_palette(rom,court=court,palette_id=palette_id)
 image=Image.new('RGBA',(128,512))
 for group in range(4):
  for tile in range(256):paint_tile(image,(tile%16)*8,(group*16+tile//16)*8,pattern,tile,palette,colors,group)
 return image


def render_hud_cells(glyphs,tiles,attributes):
 if len(tiles)!=224 or len(attributes)!=64:raise ValueError('Expected seven HUD tile rows and full attributes')
 image=Image.new('RGBA',(256,56))
 for y in range(7):
  for x in range(32):
   group=(attributes[(y//4)*8+x//4]>>(((y%4)//2)*4+((x%4)//2)*2))&3
   tile=tiles[y*32+x];gx=(tile%16)*8;gy=(group*16+tile//16)*8
   image.paste(glyphs.crop((gx,gy,gx+8,gy+8)),(x*8,y*8))
 return image


def hud_dynamic_cell(x,y):
 if y==0:return 2<=x<=10 or 21<=x<=29
 if 1<=y<=3:return 1<=x<=10 or 12<=x<=19 or 21<=x<=30
 return False


def build_hud_template(rom,colors,court=1,palette_id=None):
 validate_court(court)
 layer=court_tile_layers(rom,court=court)[0]
 tiles=[layer['tiles'][y][x] for y in range(7) for x in range(32)]
 image=render_hud_cells(build_hud_glyphs(rom,colors,court=court,palette_id=palette_id),tiles,layer['attributes'][0]).crop((0,0,256,49))
 for y in range(49):
  for x in range(256):
   if hud_dynamic_cell(x//8,y//8):image.putpixel((x,y),(0,0,0,0))
 return image


def camera_source_y(encoded):
 if not 0<=encoded<=0x1ef or encoded&255>=240:raise ValueError('Camera Y is outside two 240-line pages')
 return (encoded>>8)*240+(encoded&255)


def build_court_tiles(rom,colors,court=1,palette_id=None):
 validate_court(court)
 first=rom.prg[16384+0x3ec5+court];pattern=rom.chr[first*1024:(first+4)*1024];palette=background_palette(rom,court=court,palette_id=palette_id)
 image=Image.new('RGBA',(128,512))
 for group in range(4):
  for tile in range(256):paint_tile(image,(tile%16)*8,(group*16+tile//16)*8,pattern,tile,palette,colors,group)
 return image


def stream_court_column(rom,state,column,phase=1,court=1):
 validate_court(court)
 if not 0<=column<64:raise ValueError('Court column must be within 512 pixels')
 layers=court_tile_layers(rom,court=court);slot=column&31;page=column>>5;group=slot//4
 state['columns'][slot]=column
 other_mask,current_mask=(0x33,0xcc) if column&2 else (0xcc,0x33)
 use_full=phase==0 or ((column&32)==0 and (column&2)==0) or ((column&32)!=0 and (column&2)!=0)
 for plane in range(2):
  source=layers[plane]
  for row in range(30):state['tiles'][plane][row*32+slot]=source['tiles'][row][column]
  for row in range(8):
   address=row*8+group;current=source['attributes'][page][address];other=source['attributes'][page^1][address]
   state['attributes'][plane][address]=current if use_full else ((other&other_mask)|(current&current_mask))


def initial_court_ring(rom,court=1):
 validate_court(court)
 state={'columns':[-1]*32,'tiles':[[0]*960 for _ in range(2)],'attributes':[[0]*64 for _ in range(2)]}
 for column in range(16,48):stream_court_column(rom,state,column,phase=0,court=court)
 return state


def render_court_ring(glyphs,state,scroll_x,encoded_y):
 # Two physical vertical pages, each with a 32-column horizontal ring.
 # Source images and palettes are static ROM assets; only the ring's tile and
 # attribute indices change when the original camera streams a new column.
 start_y=camera_source_y(encoded_y)
 if not 0<=scroll_x<=255 or start_y<48 or start_y+194>480:raise ValueError('Viewport leaves the original court')
 image=Image.new('RGBA',(256,194))
 cache={}
 for y in range(194):
  source_y=start_y+y;page=source_y//240;py=source_y%240;ty=py//8
  for x in range(256):
   if x<8 and y<192:image.putpixel((x,y),(0,0,0,255));continue
   px=(scroll_x+x)%256;tx=px//8;tile=state['tiles'][page][ty*32+tx]
   attr=state['attributes'][page][(ty//4)*8+tx//4];group=(attr>>(((ty%4)//2)*4+((tx%4)//2)*2))&3
   key=tile,group
   if key not in cache:
    gx=(tile%16)*8;gy=(group*16+tile//16)*8;cache[key]=glyphs.crop((gx,gy,gx+8,gy+8))
   image.putpixel((x,y),cache[key].getpixel((px%8,py%8)))
 return image
