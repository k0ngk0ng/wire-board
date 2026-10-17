"""Independent, bounded original-instruction oracle; not natural gameplay.

Run from the repository root with workspace py65 on PYTHONPATH. No live
emulator state is read or changed. Go production never loads this ROM/CPU.
"""
import copy,gzip,hashlib,json,sys
from pathlib import Path
from py65.devices.mpu6502 import MPU
ROOT=Path(__file__).resolve().parents[2]
ROM=ROOT/'.local/nes-basket/basketball.nes'
DATA=ROOT/'games/basketball/engine/testdata'
EXPECTED='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
ROW_COUNTS=[10,12,10,11,10,11,9,11]

def image():
 raw=ROM.read_bytes();assert hashlib.sha256(raw).hexdigest()==EXPECTED
 memory=[0]*65536;memory[0x8000:0xc000]=raw[16+4*16384:16+5*16384];memory[0xc000:]=raw[16+7*16384:16+8*16384]
 return memory

def word(m,a):return m[a]|m[a+1]<<8

def profiles(m):
 result=[]
 for c,rows in enumerate(ROW_COUNTS):
  table=word(m,0x8b1f+c*2);pages=[word(m,table+i*2) for i in range(2)]
  result.append([[m[word(m,pages[x//32]+2*y)+x%32] for x in range(64)] for y in range(rows)])
 return result

def setpos(m,p,x,old):
 bases=(0x68,0x75,0x82,0x8f,0x9c) if old else (0x58e,0x59b,0x5a8,0x5b5,0x5c2)
 for a,shift,key in [(bases[0],0,'x'),(bases[1],8,'x'),(bases[2],16,'x'),(bases[3],0,'depth'),(bases[4],8,'depth')]:m[a+x]=(p[key]>>shift)&255
 if old:
  for a,shift in [(0xa9,0),(0xb6,8),(0xc3,16)]:m[a+x]=(p['height']>>shift)&255

def execute(m,x,entry,stop):
 cpu=MPU(memory=m);cpu.x=x;cpu.pc=entry
 for n in range(500):
  if cpu.pc==stop:return
  cpu.step()
 raise AssertionError((hex(cpu.pc),hex(entry)))

def probe(base,c,s):
 m=base.copy();x=3;m[0x58b]=c;setpos(m,s['position'],x,True);setpos(m,s['candidate'],x,False)
 m[0x5f7+x]=s['flags'];m[0x512+x]=s['action'];m[0x492+x]=s['velocityDepth']&255;m[0x49f+x]=(s['velocityDepth']>>8)&255
 execute(m,x,0x89fb,0x8b1e)
 result=copy.deepcopy(s)
 result['position']['depth']=m[0x8f+x]|m[0x9c+x]<<8
 result['candidate']['depth']=m[0x5b5+x]|m[0x5c2+x]<<8
 result['flags']=m[0x5f7+x];v=m[0x492+x]|m[0x49f+x]<<8;result['velocityDepth']=v if v<32768 else v-65536
 low=s['position']['x']//256&255;nextlow=s['candidate']['x']//256&255
 samples=[m[word(m,p)+idx] for p,idx in [(0x34,low>>3),(0x2c,nextlow>>3),(0x2e,nextlow>>3),(0x32,low>>3)]]
 assert m[0x604+x]==samples[0] and m[0x5ea+x]==samples[0]&248
 return {'state':result,'samples':samples}

def boundary(base,c,o,s):
 m=base.copy();m[0x58b]=c;setpos(m,s['body']['position'],o,False)
 v=s['body']['velocity']
 for a,key in [(0x478,'x'),(0x492,'depth'),(0x4ac,'height')]:m[a+o]=v[key]&255;m[a+13+o]=(v[key]>>8)&255
 m[0x539+o]=s['control'];m[0x512+o]=s['action']|(128 if s['entered'] else 0)
 execute(m,o,0x8021,0x80c6)
 result=copy.deepcopy(s);p=result['body']['position'];p['x']=m[0x58e+o]|m[0x59b+o]<<8|m[0x5a8+o]<<16
 if p['x']&0x800000:p['x']-=0x1000000
 p['depth']=m[0x5b5+o]|m[0x5c2+o]<<8
 for a,key in [(0x478,'x'),(0x492,'depth'),(0x4ac,'height')]:
  vv=m[a+o]|m[a+13+o]<<8;result['body']['velocity'][key]=vv if vv<32768 else vv-65536
 result['action']=m[0x512+o]&127;result['entered']=m[0x512+o]>=128
 return result

def main():
 base=image();maps=profiles(base)
 if '--emit-profile' in sys.argv:
  path=ROOT/'games/basketball/engine/court_profile_data.go'
  lines=['// Generated from original bank-4 row tables; see COURT_TERRAIN_VERIFICATION.md.','package engine','','var originalCourtSurfaceCells = [8][][64]uint8{']
  for rows in maps:
   lines.append('{');lines.extend('{'+','.join(map(str,row))+'},' for row in rows);lines.append('},')
  lines.append('}');path.write_text('\n'.join(lines)+'\n');return
 queries=[]
 for c,rows in enumerate(maps):
  for y,row in enumerate(rows):
   for x,cell in enumerate(row):
    p=dict(x=(x*8+7)*256+19,depth=(y*8+7)*256+23,height=0)
    q=dict(x=((x+1)%64*8)*256+7,depth=((y+1)%len(rows)*8)*256+11,height=19)
    s=dict(position=p,candidate=q,flags=255,action=0,velocityDepth=-257)
    queries.append(dict(court=c,state=s,want=probe(base,c,s)))
 for c in [5,7]:
  for x in range(512):
   for y in [71,79]:
    for h in [0,71,72,79,80,119,120,127,128,223,224,255]:
     p=dict(x=x*256+31,depth=y*256+63,height=h*256+17)
     q=dict(x=((x+9)%512)*256+47,depth=((y+3)%80)*256+71,height=33)
     s=dict(position=p,candidate=q,flags=128 if h&1 else 255,action=14 if x&1 else 0,velocityDepth=-257)
     queries.append(dict(court=c,state=s,want=probe(base,c,s)))
 # Full signed height uses only the low integer byte in the original edge probe.
 for c in [5,7]:
  for h in [-65536,-1,65536,65536+79*256]:
   p=dict(x=105*256+7,depth=79*256+9,height=h)
   s=dict(position=p,candidate=copy.deepcopy(p),flags=255,action=142,velocityDepth=-2)
   queries.append(dict(court=c,state=s,want=probe(base,c,s)))
 bounds=[]
 for c in range(8):
  maxdepth=base[0x802f+c]
  for o in range(13):
   left=base[0x8225+o];right=256+base[0x8232+o]
   for xi,x in enumerate([-1,0,left-1,left,left+1,255,256,right-1,right,right+1,511,512]):
    for yi,y in enumerate([0,15,16,17,maxdepth-1,maxdepth,maxdepth+1,255]):
     for control in [0,128]:
      s=dict(body=dict(position=dict(x=x*256+13,depth=y*256+17,height=-9),velocity=dict(x=-273,depth=514,height=-777),gravity=61),action=[2,18,34,50,66,82,4][(xi+yi)%7],entered=True,control=control)
      bounds.append(dict(court=c,object=o,state=s,want=boundary(base,c,o,s)))
 result=dict(probes=queries,bounds=bounds)
 path=DATA/'court-profiles.json.gz';path.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0))
 meta=dict(rom_sha256=EXPECTED,source_is_original_execution=True,natural_gameplay=False,method='isolated py65 original bank4 89FB..8B1E and 8021..80C6; artificial boundary inputs; no live RAM injection',probes=len(queries),bounds=len(bounds),rows=ROW_COUNTS,fixture_sha256=hashlib.sha256(path.read_bytes()).hexdigest())
 path.with_name('court-profiles-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
