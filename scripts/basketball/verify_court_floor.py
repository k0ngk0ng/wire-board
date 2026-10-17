"""Configured original floor/side/device instruction oracle, not natural play."""
import gzip,hashlib,json
from pathlib import Path
from py65.devices.mpu6502 import MPU
from verify_court_profiles import ROOT,DATA,image,profiles,EXPECTED

EXITS=0x7ff0

def w(m,a,v):m[a]=v&255;m[a+13]=(v>>8)&255

def point(m,i,x,d,h,travel=False):
 if travel:
  for a,shift,v in [(0x58e,0,x),(0x59b,8,x),(0x5a8,16,x),(0x5b5,0,d),(0x5c2,8,d)]:m[a+i]=(v>>shift)&255
 else:
  for a,shift,v in [(0x68,0,x),(0x75,8,x),(0x82,16,x),(0x8f,0,d),(0x9c,8,d),(0xa9,0,h),(0xb6,8,h),(0xc3,16,h)]:m[a+i]=(v>>shift)&255

def initial(base,c,obj,x,y,nx,ny,height,previous,variant):
 m=base.copy();m[0x58b]=c;m[0x48:0x4c]=[0x12,0x34,0x56,0x78]
 for i in range(13):
  point(m,i,(200+7*i)*256+13,40*256+17,0);point(m,i,(200+7*i)*256+19,40*256+23,0,True)
  m[0x512+i]=128;m[0x51f+i]=128;m[0x539+i]=128 if i<4 else 255;m[0x3d6+i]=[0,0,1,1,12,11,8,9,9,9,9,9,9][i];m[0x642+i]=0
 m[0x53f]=6;m[0x550:0x554]=[255]*4;m[0x576:0x57a]=[7]*4
 point(m,obj,x*256+31,y*256+47,height);point(m,obj,nx*256+63,ny*256+79,0,True)
 w(m,0x5cf+obj,previous);w(m,0x478+obj,513);w(m,0x492+obj,-257)
 vz,g=[(-257,61),(0,0),(257,0),(-513,256),(0,256),(0,0),(257,256),(-1,1)][variant%8]
 w(m,0x4ac+obj,vz);w(m,0x4c6+obj,g)
 m[0x5f7+obj]=[128,0,192,64,128,255,0,128][variant%8]
 actions=[0,14,2,18,34,50,82,5,1,6,7,10,13,66,3,14]
 m[0x512+obj]=128|actions[variant%len(actions)]
 if variant&1:m[0x539]=0
 if variant%5==0:m[0x53f]=obj
 if variant%7==0:m[0x53d]=obj
 if variant%11==0:m[0x53e]=obj
 if c==4:m[0x6d8]=variant&1
 # Linked/riding contacts exercise 89AE and New York's shared-object effects.
 if variant%4==0:
  rider=(obj+1)%4;m[0x642+rider]=128|obj;m[0x512+rider]=128|66
  if obj<4:m[0x550+obj]=rider
  if obj in (4,5):m[0x550+rider]=(rider+1)%4
 return m

def main():
 base=image();maps=profiles(base);raw=bytearray();rows=[];coverage={};scenarios=[]
 for c,grid in enumerate(maps):
  candidates={}
  for y in range(16,min(88,len(grid)*8),8):
   for x in range(8,504,8):
    for dx,dy in [(8,0),(-8,0),(0,8),(0,-8),(8,8),(-8,-8)]:
     nx,ny=x+dx,y+dy
     if not(0<=nx<512 and 0<=ny<len(grid)*8):continue
     key=(grid[y//8][x//8],grid[ny//8][nx//8],grid[y//8][nx//8],grid[ny//8][x//8],dx,dy,x>=256)
     candidates.setdefault(key,(x,y,nx,ny))
  # Exact original platform/device entrance and auxiliary-wrap boundaries.
  for x,y in [(104,79),(111,79),(192,79),(199,79),(256,79),(263,79),(352,71),(447,71),(119,79),(392,79),(264,72),(303,72),(376,72),(407,72),(144,80),(151,80),(152,80),(159,80),(120,72),(183,72),(272,72),(335,72),(55,56),(56,56),(471,56),(472,71),(135,73),(136,73),(391,73),(392,73)]:
   if y<len(grid)*8:candidates[('special',x,y)]=(x,y,x,y)
  for x,y,nx,ny in candidates.values():
   old=grid[y//8][x//8]&248;floor=grid[ny//8][nx//8]&248
   cases=[(floor*256+17,floor),(floor*256,floor),(floor*256-1,floor),(max(old,floor)*256+1,max(old,floor)),(-1,0),((floor+1)*256,0),(256*256+1,256),(floor*256-513,max(old,floor)+1),(old*256-1,max(0,floor-1))]
   for obj in range(13):
    for v,(height,previous) in enumerate(cases):scenarios.append((c,obj,x,y,nx,ny,height,previous,v+(obj%2)*8))
 for index,args in enumerate(scenarios):
  c,obj,*_=args;m=initial(base,*args);before=bytes(m[:2048]);cpu=MPU(memory=m);cpu.pc=0x8249;cpu.x=obj;cpu.sp=0xfd;m[0x1fe]=(EXITS-1)&255;m[0x1ff]=(EXITS-1)>>8;sounds=[]
  for n in range(10000):
   if cpu.pc==EXITS:break
   coverage[cpu.pc]=coverage.get(cpu.pc,0)+1
   if cpu.pc==0xc000:
    sounds.append(cpu.a);cpu.pc=((cpu.stPop()|(cpu.stPop()<<8))+1)&65535;continue
   cpu.step()
  else:raise AssertionError((index,args,hex(cpu.pc)))
  raw.extend(before);raw.extend(bytes(m[:2048]));rows.append(dict(frame=index,object=obj,sounds=sounds))
 p=DATA/'court-floor-machines.ram.gz';p.write_bytes(gzip.compress(raw,mtime=0));p.with_name('court-floor-machines.json').write_text(json.dumps(rows,separators=(',',':'))+'\n')
 branches={hex(a):coverage.get(a,0) for a in [0x8249,0x8277,0x82ec,0x831e,0x839e,0x83cc,0x840b,0x8458,0x846e,0x84d6,0x85b5,0x84e4,0x863c,0x86af,0x873c,0x8857,0x88ad,0x88c6,0x89ae,0x89e6]}
 meta=dict(records=len(rows),source_is_original_execution=True,natural_gameplay=False,method='isolated py65 original 8249 with original bank4/fixed bank7; only C000 audio request sink intercepted; configured synthetic floor/side/device inputs',rom_sha256=EXPECTED,fixture_sha256=hashlib.sha256(p.read_bytes()).hexdigest(),branches=branches)
 p.with_name('court-floor-machines-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
