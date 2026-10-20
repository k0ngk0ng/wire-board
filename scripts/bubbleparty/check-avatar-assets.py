#!/usr/bin/env python3
"""Failure inventory before asset preparation: missing variant/team/state;
mutable paths, wrong SHA, invalid frame rectangles, lost alpha, blue copied
from red, and a right-facing sprite reverting to its red static layer.
"""
import hashlib,json,struct,zlib
from pathlib import Path
root=Path(__file__).resolve().parents[2]
manifest=json.loads((root/'games/bubbleparty/assets/manifest-v3.json').read_text())
catalog=json.loads((root/'games/bubbleparty/assets/avatars-v3.json').read_text())
assert manifest['version']=='bubbleparty/v3/avatars'
assert hashlib.sha256((root/manifest['generator']).read_bytes()).hexdigest()==manifest['generatorSha256']
assert len(catalog['avatars'])==4 and {a['id'] for a in catalog['avatars']}==set(range(4))
entries={e['urlPath']:e for e in manifest['entries']}
def read_png(path):
 raw=path.read_bytes();assert raw[:8]==b'\x89PNG\r\n\x1a\n';pos=8;compressed=b''
 while pos<len(raw):
  n=struct.unpack_from('>I',raw,pos)[0];kind=raw[pos+4:pos+8];data=raw[pos+8:pos+8+n]
  assert zlib.crc32(kind+data)&0xffffffff==struct.unpack_from('>I',raw,pos+8+n)[0]
  if kind==b'IHDR':w,h,depth,color,_,_,_=struct.unpack('>IIBBBBB',data);assert (depth,color)==(8,6)
  if kind==b'IDAT':compressed+=data
  pos+=n+12
 decoded=zlib.decompress(compressed);assert len(decoded)==h*(w*4+1)
 # The retained original-resource decoder uses canonical unfiltered RGBA PNG.
 assert all(decoded[y*(w*4+1)]==0 for y in range(h))
 return w,h,b''.join(decoded[y*(w*4+1)+1:(y+1)*(w*4+1)] for y in range(h))
for avatar in catalog['avatars']:
 assert avatar['name'] and avatar['id'] in range(4)
 for key,clip in avatar['clips'].items():
  images=[]
  for team in ['red','blue']:
   url='/bubbleparty/v3/avatars/'+clip[team];entry=entries[url]
   p=root/'games/bubbleparty/assets/public'/url.lstrip('/')
   assert hashlib.sha256(p.read_bytes()).hexdigest()==entry['sha256']
   w,h,rgba=read_png(p);images.append(rgba)
   for f in clip['frames']:assert 0<=f['x']<f['x']+f['w']<=w and 0<=f['y']<f['y']+f['h']<=h
   assert all(0<=i<len(clip['frames']) for i,_ in clip['sequence'])
  red,blue=images;assert len(red)==len(blue)
  changed=0
  for i in range(0,len(red),4):
   assert red[i+3]==blue[i+3]
   if red[i:i+4]!=blue[i:i+4]:
    changed+=1;assert blue[i:i+4]==bytes((red[i+2],red[i+1],red[i],red[i+3]))
  assert changed>=100,(avatar['id'],key,changed)
assert len(entries)==48
print(json.dumps({'passed':True,'avatars':4,'teamVariants':8,'spriteSheets':48,'source':manifest['source']['sha256']},ensure_ascii=False))
