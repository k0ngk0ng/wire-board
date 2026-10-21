#!/usr/bin/env python3
"""Recover original mount sheets, item icons and ten red/blue rider poses.
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
public=out/'public/bubbleparty/v4/mounts';public.mkdir(parents=True,exist_ok=True)
manifest={'version':'bubbleparty/v4/mounts','source':{'url':'https://archive.org/download/crazy_arcade_setups/2002.03.13./Crazy%20Arcade.zip','sha256':sha(raw),'indexSha256':sha(idx),'objectTable':lookup['ObjectTable/ObjectTable'],'itemAttr':lookup['Attr/ItemAttr']},'generator':'scripts/bubbleparty/prepare-mounts.py','generatorSha256':sha(Path(__file__).read_bytes()),'licenseStatus':'Original asset redistribution permission unverified','entries':[],'animations':[]}
catalog={'version':'bubbleparty/v4/mounts','tickMS':30,'timingNote':'Ship speed: 40 original pixels per grid / pixelsPerTick * 30ms engine update. Animation uses original weights at 30ms.','anchorNote':'Original frame offsets retained; common original character/ship canvas aligns to same ground point. Project ground placement uses width/2, height-4.','mounts':[],'riders':[]}
attrs={};b=member('Attr/ItemAttr');p=8
while p<len(b):
 start=p;n=struct.unpack_from('<I',b,p)[0];p+=4;name=b[p:p+n].decode('ascii');p+=n
 fields=list(struct.unpack_from('<10i',b,p));p+=40
 n=struct.unpack_from('<I',b,p)[0];p+=4;model=b[p:p+n].decode('ascii');p+=n
 tail=list(struct.unpack_from('<4i',b,p));p+=16
 if model.startswith('Ship/'):attrs[model]={'sourceItem':name,'fieldsRaw':fields,'tailRaw':tail,'offsetInItemAttr':start,'length':p-start}
assert p==len(b) and len(attrs)==8

def save(file,w,h,pixels,sources,frames):
 encoded=png(public/file,w,h,pixels)
 manifest['entries'].append({'file':file,'urlPath':'/bubbleparty/v4/mounts/'+file,'sha256':sha(encoded),'bytes':len(encoded),'width':w,'height':h,'sources':[lookup[n] for n in sources],'framesRaw':frames})
 return file

def clip_frames(frames):
 return [{'x':f[0],'y':f[1],'w':f[2]-f[0],'h':f[3]-f[1],'ax':(f[2]-f[0])/2,'ay':f[3]-f[1]-4} for f in frames]
ids=['airship','owl','turtle','turtle-fast','airship-broken','octopus','octopus-fast','railcar']
labels=['飞碟','猫头鹰','乌龟','快乌龟','故障飞碟','章鱼','快章鱼','矿车']
for i in range(8):
 model=f'Ship/Ship{i:02}';a,origin=animations(model)
 item={'id':ids[i],'name':labels[i],'model':model,**attrs[model],'pixelsPerTick':attrs[model]['tailRaw'][0],'clips':{}}
 manifest['animations'].append({'source':origin,'states':a})
 for d,state in {'left':'00 Move Left','up':'01 Move Up','right':'02 Move Right','down':'03 Move Down'}.items():
  source=a[state]['resource'];w,h,f,pixels=decode(source)
  item['clips'][d]={'file':save(f'{ids[i]}-{d}.png',w,h,pixels,[source],f),'frames':clip_frames(f),'sequence':a[state]['sequence']}
 w,h,f,pixels=decode(item['sourceItem'])
 item['icon']={'file':save(f'{ids[i]}-icon.png',w,h,pixels,[item['sourceItem']],f),'frames':clip_frames(f)}
 catalog['mounts'].append(item)
for i in range(10):
 a,origin=animations(f'Character/C{i:02}');item={'id':i,'clips':{}}
 for d,state in {'left':'04 Ride Left','up':'05 Ride Up','right':'06 Ride Right','down':'07 Ride Down'}.items():
  source=a[state]['resource'];w,h,f,adjust=decode(source+'LayerAdjust');sw,sh,sf,static=decode(source+'LayerStatic')
  assert (sw,sh)==(w,h) and [v[:6] for v in f]==[v[:6] for v in sf]
  clip={'frames':clip_frames(f),'sequence':a[state]['sequence']}
  for team in ['red','blue']:
   pixels=bytearray(static)
   for pos in range(0,len(adjust),4):
    if adjust[pos+3]:pixels[pos:pos+4]=adjust[pos:pos+4] if team=='red' else bytes((adjust[pos+2],adjust[pos+1],adjust[pos],adjust[pos+3]))
   clip[team]=save(f'rider-{i}-{d}-{team}.png',w,h,pixels,[source+'LayerStatic',source+'LayerAdjust'],f)
  item['clips'][d]=clip
 catalog['riders'].append(item)
(out/'mounts-v4.json').write_text(json.dumps(catalog,ensure_ascii=False,indent=2)+'\n')
(out/'manifest-mounts-v4.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'mounts':len(catalog['mounts']),'files':len(manifest['entries']),'bytes':sum(e['bytes'] for e in manifest['entries'])}))
