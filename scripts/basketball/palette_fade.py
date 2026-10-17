"""Original $9526 full-group NES palette clamp and an exact RGB quotient LUT."""
def fade_nes_index(index,level):
 if type(index) is not int or not 0<=index<64:raise ValueError('Unknown NES color index')
 if type(level) is not int or not 0<=level<=4:raise ValueError('Unknown original palette level')
 if level==0:return 15
 threshold=level*16
 return (index&15)+threshold-16 if index>=threshold else index

def build_palette_fade_lut(colors):
 if len(colors)!=192:raise ValueError('Expected original 64-color RGB palette')
 entries={}
 for index in range(64):
  rgb=tuple(colors[index*3:index*3+3]);levels=[list(colors[fade_nes_index(index,level)*3:fade_nes_index(index,level)*3+3]) for level in range(5)]
  if rgb in entries and entries[rgb]!=levels:raise ValueError('RGB aliases do not preserve the original index clamp')
  entries[rgb]=levels
 return [{'rgb':list(rgb),'levels':levels} for rgb,levels in entries.items()]
