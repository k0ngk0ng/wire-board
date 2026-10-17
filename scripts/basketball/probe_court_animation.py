"""Original instructions for bounded animation branches absent in natural play.
Sound and palette peripheral entries are observed commands; original RNG executes.
"""
import gzip,hashlib,json,sys
from pathlib import Path
from capture_court_animation import ROOT,state,patches,scores
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
class Trace(list):
 active=False
 addresses=[]
 def __setitem__(self,k,v):
  super().__setitem__(k,v)
  if self.active and isinstance(k,int) and k in (0x30b,0x32e,0x351):self.addresses.append(k)
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];m=Trace([0]*65536);m[0x8000:0xc000]=prg[4*16384:5*16384];m[0xc000:]=prg[-16384:];cpu=MPU(memory=m);rows=[]
configurations={0:[[i]+[0]*8 for i in range(4)],1:[[0]*9],2:[[side,128,128,t,t,0,0,0,0] for side in (0,1) for t in (0,1,2)],3:[[flags,t]+[0]*7 for flags,t in [(0,0),(128,0),(1,8),(1,1)]],4:[[f,d]+[0]*7 for f in range(3) for d in (0,1)],5:[[side,128,128,t,t,0,0,0,0] for side in (0,1) for t in (0,1,2)],6:[[0,2,4,6,0,2,0,0,0],[128,132,134,128,132,134,1,1,0],[128,132,134,128,132,134,0x1d,0x3d,0],[128,132,134,128,132,134,0x6d,0x6d,128]],7:[[0]*9]}
for court in (2,5):
 configurations[court]+=[[side,0,0,t,t,0,0,0,0] for side in (0,1) for t in (0,1,2)]
# Force each jackpot symbol using original per-reel permutations, not desired
# scores. Progress 0x3D reaches the original third-reel stop/award branch.
permutations=[list(m[0xa3da:0xa3df]),list(m[0xa3df:0xa3e4]),list(m[0xa3e4:0xa3e9])]
for symbol in range(4):
 frames=[128+2*p.index(symbol) for p in permutations]
 for side in (0,1):
  configurations[6].append((frames+[128]*3 if side==0 else [128]*3+frames)+([0x3d,0] if side==0 else [0,0x3d])+[0])
for court,configs in configurations.items():
 for config in configs:
  for camera in range(256):
   m.active=False;m[:0x800]=[0]*0x800;m[0x58b]=court;m[0x589]=1;m[0x300]=17;m[0x397]=camera;m[0x6d7:0x6e0]=config;m[0x48:0x4c]=[0x12,0x34,0x56,0x78];m[0x166:0x16a]=[4,20,0,0]
   m[0x540:0x546]=[7,8,9,10,11,12];m[0x62d:0x633]=[1,1,0,0,0,0]
   before=bytes(m[:0x800]);cpu.pc=0x9b95;cpu.sp=0xfd;cpu.stPushWord(0x6fff);m.addresses=[];m.active=True;sounds=[];palette=False
   for tick in range(20000):
    if cpu.pc==0x7000:break
    if cpu.pc in (0xc000,0xfcec):
     if cpu.pc==0xc000:sounds.append(cpu.a)
     else:palette=True
     cpu.pc=cpu.stPopWord()+1;continue
    cpu.step()
   else:raise RuntimeError((court,camera,hex(cpu.pc)))
   after=bytes(m[:0x800]);m.active=False
   rows.append(dict(court=court,frame=0,before=state(before,court),context=dict(court=court,active=True,videoFrame=17,cameraX=camera,random=int.from_bytes(before[0x48:0x4c],'big'),scores=scores(before),hoopOwners=list(before[0x540:0x546]),hoopLocks=list(before[0x62d:0x633])),after=dict(state=state(after,court),patches=patches(after,m.addresses),palette=palette,paletteID=after[0x304] if palette else 0,sounds=sounds,random=int.from_bytes(after[0x48:0x4c],'big'),scores=scores(after))))
p=ROOT/'games/basketball/engine/testdata/court-animation-probes.json.gz';p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0))
p.with_name('court-animation-probes-source.json').write_text(json.dumps(dict(scope='isolated original 6502 routine execution, observed sound/palette peripheral commands; no natural-gameplay claim',entry='bank4:9B95',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n');print(len(rows))
