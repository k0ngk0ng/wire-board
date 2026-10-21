"""Full original CE09/F002 oracle; all source instructions are executed.
Cases distinguish captured DMA lineage from isolated pose/edge/capacity probes.
"""
import argparse,base64,csv,gzip,hashlib,json
from pathlib import Path
from static_sprites import SpriteROM,DIGEST
from postresult_art import POSE_BOUNDS
from verify_pass_floor_original import ROOT,Memory,MPU
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();assert hashlib.sha256(ROM).hexdigest()==DIGEST
PRG=ROM[16:131088];OUT=ROOT/'games/basketball/engine/testdata'
FRAMES=[('reward',540),('reward',660),('ending',600),('ending',1800),('ending',6000),('ending',10500),('ending',12000),('ending',13500),('reward',600),('reward',720),('ending',900),('ending',1200),('ending',3000),('ending',4800),('ending',9000),('ending',14000)]

def execute(raw,entry=0xce09):
 m=Memory(PRG,raw);c=MPU(memory=m,pc=entry);c.sp=255;c.p=32;c.stPushWord(0x6fff);pieces=0
 for steps in range(100000):
  if c.pc==0x7000:break
  if c.pc==0xd008:pieces+=1
  c.step()
 else:raise RuntimeError(('source instruction bound',hex(c.pc)))
 return bytes(m[0x200:0x300]),pieces,steps

def base():
 r=bytearray(2048);r[0x588]=7;r[0x589]=129;r[0x41d:0x429]=bytes([255]*12)
 r[0x200:0x300]=bytes((i*37+11)&255 for i in range(256));r[0x5b:0x68]=bytes([127]*13)
 return r

def actor(r,id,kind,pose,x,y,char=0,air=False,surface=0,height=0):
 r[0x3d6+id]=kind;r[0x5b+id]=pose;r[0x3e3+id]=char
 for a,value in [(0x3e7,x),(0x401,y)]:r[a+id]=value&255;r[a+13+id]=(value>>8)&255
 r[0x5f7+id]=128 if air else 0;r[0x5ea+id]=surface
 r[0xa9+id]=0;r[0xb6+id]=height&255;r[0xc3+id]=(height>>8)&255

def record(label,raw,scope='controlled isolated source call',expected=None):
 after,count,steps=execute(raw)
 if expected is not None:assert after==expected,label
 reversed_oam,_,_=execute(bytes(raw[:0x200])+after+bytes(raw[0x300:]),0xf002)
 assert reversed_oam==b''.join(after[i:i+4] for i in range(252,-1,-4))
 return dict(label=label,scope=scope,before=base64.b64encode(raw).decode(),oam=base64.b64encode(after).decode(),reversed=base64.b64encode(reversed_oam).decode(),pieces=count,instructions=steps)

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args();cases=[]
 for kind,(start,end) in POSE_BOUNDS.items():
  for pose in range((end-start)//2):
   for face in (0,128):
    for owner in range(4):
     for scene in (1,7):
      r=base();r[0x588]=scene;r[0x589]=135 if scene==1 else 129;r[0x41d]=owner
      actor(r,owner,kind,pose|face,128,120,char=(pose+owner)%8)
      cases.append(record(f'pose-{scene}-{kind}-{pose}-{face}-{owner}',r))
 for scene in (1,7):
  for kind,pose in [(17,0),(17,22),(17,23),(17,31),(17,111),(17,112),(17,113),(18,2),(19,0)]:
   for face in (0,128):
    for x,y in [(-48,120),(-1,120),(0,120),(255,120),(256,120),(128,-24),(128,-1),(128,0),(128,184),(128,185),(128,255)]:
     r=base();r[0x588]=scene;r[0x589]=135 if scene==1 else 129;r[0x41d]=2;actor(r,2,kind,pose|face,x,y,char=7)
     cases.append(record(f'edge-{scene}-{kind}-{pose}-{face}-{x}-{y}',r))
 for owner in range(4):
  for face in (0,128):
   for air,surface,height in [(False,0,0),(True,0,0),(True,0,31),(True,64,31)]:
    r=base();r[0x588]=1;r[0x589]=135;r[0x41d]=owner;actor(r,owner,18,7|face,128,120,char=7,air=air,surface=surface,height=height)
    cases.append(record(f'shadow-{owner}-{face}-{air}-{surface}-{height}',r))
 # Repeated slots are a declared controlled capacity stress, not normal actors.
 r=base()
 for owner in range(4):actor(r,owner,19,0,64+owner*40,120)
 r[0x41d:0x429]=bytes([0,1,2,3]*3);cases.append(record('capacity',r,'controlled abnormal draw-list repetition'))
 for kind,frame in FRAMES:
  path=ROOT/f'.local/nes-basket/runs/postresult-{kind}-oam'
  with (path/'frames.csv').open() as f:video=next(int(x['emulator_frame']) for x in csv.DictReader(f) if int(x['frame'])==frame)
  dma=next(json.loads(l) for l in (path/'dma-source.jsonl').read_text().splitlines() if json.loads(l)['frame']==video)
  assert not dma['partialDraw'] and dma['submittedIndex']>=0
  raw=(path/'oam-source.ram').read_bytes();offset=dma['submittedIndex']*4096;before=raw[offset:offset+2048];after=raw[offset+2048:offset+4096]
  expected=(path/f'dma-{video:05d}.bin').read_bytes();assert expected==after[0x200:0x300]
  cases.append(record(f'original-{kind}-{frame}',before,'controlled original whole-console CE09 and selected DMA',expected))
 payload=gzip.compress(json.dumps(cases,separators=(',',':')).encode(),mtime=0);file=OUT/'postresult-oam.json.gz'
 meta=dict(romSHA256=DIGEST,cases=len(cases),originalFrames=len(FRAMES),fullCE09Calls=len(cases),fullF002Calls=len(cases),pieces=sum(x['pieces'] for x in cases),capacityCases=sum(x['pieces']==64 for x in cases),fixtureSHA256=hashlib.sha256(payload).hexdigest(),skippedCalls=[],scope=__doc__)
 if args.check:assert file.read_bytes()==payload
 else:file.write_bytes(payload);file.with_name('postresult-oam-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
