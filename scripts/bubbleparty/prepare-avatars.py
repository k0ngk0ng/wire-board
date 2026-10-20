#!/usr/bin/env python3
"""Recover four reviewed original characters, with fixed red/blue team palettes.
Failure inventory and evidence: docs/research/realtime-efficiency/MOTION.md and
 docs/research/bubbleparty/geometry-original-evidence.md. No binary execution.
"""
import argparse,hashlib,json,struct,zlib
from pathlib import Path
root=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--client-dir',type=Path,required=True);p.add_argument('--out',type=Path,default=root/'games/bubbleparty/assets');args=p.parse_args()
out=args.out.resolve();assert out.is_relative_to(root)
raw=(args.client_dir/'Fx.idd').read_bytes();idx=(args.client_dir/'Fx.idx').read_bytes()
sha=lambda b:hashlib.sha256(b).hexdigest()
assert sha(raw)=='3554b3cda62313366d07776c48c203ddc95092b18b753b4d0986fab19ed664b2'
assert sha(idx)=='6be28522015c5a4b0f4b8ab86a989819d1b0b83f676566d4b99cc4c8d15aba7c'
lookup={};cursor=4;last_end=0
for _ in range(struct.unpack_from('<I',idx)[0]):
 n=struct.unpack_from('<I',idx,cursor)[0];cursor+=4;assert 0<n<1024 and cursor+n+8<=len(idx)
 name=idx[cursor:cursor+n].rstrip(b'\0').decode('cp949',errors='replace');cursor+=n
 assert name not in lookup
 offset,size=struct.unpack_from('<II',idx,cursor);cursor+=8
 assert last_end<=offset<=offset+size<=len(raw);last_end=offset+size
 lookup[name]={'name':name,'offset':offset,'size':size,'sha256':sha(raw[offset:offset+size])}
assert cursor==len(idx)
def member(name):
 r=lookup[name];return raw[r['offset']:r['offset']+r['size']]
def color(word):
 r,g,b=(word>>11)&31,(word>>5)&63,word&31
 return bytes(((r<<3)|(r>>2),(g<<2)|(g>>4),(b<<3)|(b>>2),255))
def decode(name):
 b=member(name);n,fmt,w,h=struct.unpack_from('<4I',b)
 assert fmt==1 and 0<n<100 and 0<w<2048 and 0<h<2048
 frames=[list(struct.unpack_from('<7i',b,16+28*i)) for i in range(n)]
 assert all(0<=f[0]<f[2]<=w and 0<=f[1]<f[3]<=h for f in frames)
 pos=16+28*n;length=struct.unpack_from('<I',b,pos)[0];pos+=4;assert len(b)-pos==length
 rgba=bytearray()
 while pos<len(b):
  op=b[pos];pos+=1;kind,count=op>>6,op&63;assert kind<3 and count>0
  if kind==0:
   assert pos+count*2<=len(b)
   for i in range(count):rgba.extend(color(struct.unpack_from('<H',b,pos+i*2)[0]))
   pos+=count*2
  elif kind==1:
   assert pos+2<=len(b);rgba.extend(color(struct.unpack_from('<H',b,pos)[0])*count);pos+=2
  else:rgba.extend(bytes(4)*count)
  assert len(rgba)<=w*h*4
 assert pos==len(b) and len(rgba)==w*h*4
 return w,h,frames,bytes(rgba)
def png(path,w,h,rgba):
 def chunk(kind,data):return struct.pack('>I',len(data))+kind+data+struct.pack('>I',zlib.crc32(kind+data)&0xffffffff)
 scan=b''.join(b'\0'+rgba[y*w*4:(y+1)*w*4] for y in range(h))
 encoded=b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',w,h,8,6,0,0,0))+chunk(b'IDAT',zlib.compress(scan))+chunk(b'IEND',b'')
 path.write_bytes(encoded);return encoded
# ObjectTable provides actual animation-resource references and frame order.
table=member('ObjectTable/ObjectTable');cursor=4;objects=[]
for _ in range(struct.unpack_from('<I',table)[0]):
 n=struct.unpack_from('<I',table,cursor)[0];cursor+=4;assert 0<n<200
 name=table[cursor:cursor+n].decode('ascii');cursor+=n
 offset=struct.unpack_from('<i',table,cursor)[0];cursor+=4;objects.append((name,offset))
def animations(character):
 i=next(i for i,(name,offset) in enumerate(objects) if name==character)
 begin=objects[i][1];end=next(o for _,o in objects[i+1:] if o>=0);b=table[begin:end];pos=4;result={}
 for _ in range(struct.unpack_from('<I',b)[0]):
  names=[]
  for _ in range(2):
   n=struct.unpack_from('<I',b,pos)[0];pos+=4;assert 0<n<200 and pos+n<=len(b)
   names.append(b[pos:pos+n].decode('ascii'));pos+=n
  n=struct.unpack_from('<I',b,pos)[0];pos+=4;assert 0<n<1000 and pos+n*8<=len(b)
  pairs=[list(struct.unpack_from('<ii',b,pos+i*8)) for i in range(n)];pos+=n*8
  result[names[0]]={'resource':names[1],'sequence':pairs}
 assert pos==len(b)
 return result,{'name':character,'offsetInObjectTable':begin,'size':len(b),'sha256':sha(b)}
public=out/'public/bubbleparty/v3/avatars';public.mkdir(parents=True,exist_ok=True)
manifest={'version':'bubbleparty/v3/avatars','source':{'url':'https://archive.org/download/crazy_arcade_setups/2002.03.13./Crazy%20Arcade.zip','sha256':sha(raw),'indexSha256':sha(idx),'objectTable':lookup['ObjectTable/ObjectTable'],'reaEvidence':'ev_0811e32c3a8d2fb2fef08c77afdc64ef07612a8357e91110e329b039e7936136'},'generator':'scripts/bubbleparty/prepare-avatars.py','generatorSha256':sha(Path(__file__).read_bytes()),'palette':'Static below Adjust; blue swaps R/B of Adjust only. Project palette, not a claimed original engine algorithm.','licenseStatus':'Original asset redistribution permission unverified','entries':[],'animations':[]}
catalog={'version':'bubbleparty/v3/avatars','tickMS':20,'timingNote':'20ms per raw animation weight is a project presentation choice; original time unit unverified.','anchorNote':'Frame center X, height minus four Y is a project ground-plane anchor; raw original offsets retained in manifest.','avatars':[]}
labels=['泡泡帽','飞行帽','熊耳帽','条纹帽']
states={'up':'21 Walk Up','down':'23 Walk Down','left':'20 Walk Left','right':'22 Walk Right','idle':'00 Blink','death':'01 Die'}
for avatar in range(4):
 character=f'Character/C{avatar:02d}';anims,origin=animations(character)
 manifest['animations'].append({'source':origin,'states':{name:anims[name] for name in [*states.values(),'25 Locked In']}})
 item={'id':avatar,'name':labels[avatar],'clips':{}}
 for key,state in states.items():
  resource=anims[state]['resource'];a=decode(resource+'LayerAdjust');s=decode(resource+'LayerStatic')
  assert a[:2]==s[:2] and [f[:6] for f in a[2]]==[f[:6] for f in s[2]]
  w,h,frames,adjust=a;static=s[3]
  clip={'frames':[{'x':f[0],'y':f[1],'w':f[2]-f[0],'h':f[3]-f[1],'ax':(f[2]-f[0])/2,'ay':f[3]-f[1]-4} for f in frames],'sequence':anims[state]['sequence']}
  for team in ['red','blue']:
   pixels=bytearray(static)
   for i in range(0,len(adjust),4):
    if adjust[i+3]:pixels[i:i+4]=adjust[i:i+4] if team=='red' else bytes((adjust[i+2],adjust[i+1],adjust[i],adjust[i+3]))
   filename=f'avatar-{avatar}-{key}-{team}.png';encoded=png(public/filename,w,h,bytes(pixels));clip[team]=filename
   manifest['entries'].append({'urlPath':'/bubbleparty/v3/avatars/'+filename,'sha256':sha(encoded),'bytes':len(encoded),'width':w,'height':h,'avatar':avatar,'team':team,'state':key,'sourceLayers':[lookup[resource+'LayerStatic'],lookup[resource+'LayerAdjust']],'framesRaw':frames})
  item['clips'][key]=clip
 item['lockedFrames']=[i for i,_ in anims['25 Locked In']['sequence']]
 catalog['avatars'].append(item)
(out/'manifest-v3.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
(out/'avatars-v3.json').write_text(json.dumps(catalog,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'avatars':len(catalog['avatars']),'files':len(manifest['entries']),'bytes':sum(e['bytes'] for e in manifest['entries'])}))
