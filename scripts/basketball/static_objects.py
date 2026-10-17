"""Original ball, hoop and prop metasprites for offline asset export.

Kinds 8..14 use the common bank-2 decoder but no character head substitutions.
All their tiles live on the fourth sprite CHR page, shared with the ball.
"""
from PIL import Image
from graphics import decode_tile
from static_sprites import signed


def object_palette_side(rom,kind,pose,attribute_mask=0xff):
 if not 8<=kind<=14:raise ValueError('Expected a ball/hoop/auxiliary sprite kind')
 if attribute_mask not in (0xfd,0xff):raise ValueError('Unsupported original attribute mask')
 sides={int(bool(attr&attribute_mask&2)) for _,attr,_,_ in rom.pieces(kind,pose)}
 if len(sides)!=1:raise ValueError('Mixed-team object palette needs a separate exporter')
 return sides.pop()


def render_object(rom,kind,packed_pose,chr_pages,palette,colors,reverse=False,attribute_mask=0xff,size=(64,64),origin=(32,56)):
 if not 8<=kind<=14:raise ValueError('Expected a ball/hoop/auxiliary sprite kind')
 if attribute_mask not in (0xfd,0xff):raise ValueError('Unsupported original attribute mask')
 if len(chr_pages)!=8 or any(not 0<=p<128 for p in chr_pages):raise ValueError('Expected eight CHR page numbers')
 if len(palette)!=32 or len(colors)!=192:raise ValueError('Incomplete palette')
 if not 0<=packed_pose<=255:raise ValueError('Invalid packed object pose')
 pieces=rom.pieces(kind,packed_pose&127)
 if reverse:pieces=pieces[::-1]
 pattern=b''.join(rom.chr[p*1024:(p+1)*1024] for p in chr_pages)
 image=Image.new('RGBA',size)
 for tile,attr,x,y in pieces[::-1]:
  attr&=attribute_mask;x=signed(x)
  if not packed_pose&128:x=-x-8;attr^=64
  pixels=decode_tile(pattern[4096+tile*16:4096+(tile+1)*16])
  if attr&128:pixels=pixels[::-1]
  if attr&64:pixels=[row[::-1] for row in pixels]
  for dy,row in enumerate(pixels):
   for dx,index in enumerate(row):
    if not index:continue
    px=origin[0]+x+dx;py=origin[1]+signed(y)-23+dy
    if not 0<=px<size[0] or not 0<=py<size[1]:raise ValueError('Original object exceeds canvas')
    color=palette[16+(attr&3)*4+index]&63
    image.putpixel((px,py),tuple(colors[color*3:color*3+3])+(255,))
 return image
