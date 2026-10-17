"""Independent offline projection of the original font/scroll split scanline.
The 8x8 glyphs and semantic court ring supply every pixel. No screen fragment
or CPU execution is used; explicit fetch timing comes from read-only evidence.
"""
from PIL import Image
from static_court import camera_source_y

def render_hud_boundary(glyphs,ring,camera,hud,timing=None):
 timing=timing or {'addressResetFetch':5,'fineScrollResetFetch':9}
 reset=timing['addressResetFetch'];fine=timing['fineScrollResetFetch']
 if reset not in (5,6) or not 8<=fine<=10:raise ValueError('Unsupported original split timing')
 line=Image.new('RGBA',(256,1),(0,0,0,255));y=camera_source_y(camera['y'])+193;plane=y//240;row=y%240//8;fine_y=y%8;fetched=[]
 for fetch in range(34):
  if fetch>=2:
   offset=camera['x']%8 if fetch<fine else 0
   for x in range(8):
    screen=(fetch-2)*8+x
    if screen<8:continue
    position=offset+x;line.putpixel((screen,0),fetched[fetch-2+position//8][position%8])
  if fetch<reset:
   column=(camera['x']//8+fetch)%32;tile=ring['tiles'][plane][row*32+column];attribute=ring['attributes'][plane][row//4*8+column//4];palette=(attribute>>((row%4//2)*4+(column%4//2)*2))&3
   fetched.append([glyphs.getpixel((tile%16*8+x,(palette*16+tile//16)*8+fine_y)) for x in range(8)])
  else:fetched.append([hud.getpixel(((fetch-reset)*8+x,2)) for x in range(8)])
 return line
