"""Audit BDD2's terrain-replaced action4 through original instructions, not a runtime approximation."""
import hashlib,json,collections
from pathlib import Path
from verify_pass_floor_original import ROOT,Memory,MPU,put
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
out=ROOT/'.local/basketball/rea/special-turn-alias';out.mkdir(exist_ok=True)
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
class AudioMemory(Memory):
 def __setitem__(self,a,v):
  if isinstance(a,int) and 0x4000<=a<=0x4017:list.__setitem__(self,a,v);return
  super().__setitem__(a,v)
def call(m,entry,a=0,x=0):
 cpu=MPU(memory=m,pc=entry);cpu.a=a;cpu.x=x;cpu.sp=255;cpu.stPushWord(0x6fff)
 for n in range(30000):
  if cpu.pc==0x7000:return
  cpu.step()
 raise RuntimeError(hex(cpu.pc))
audio=[]
for music in range(59,67):
 m=AudioMemory(prg,bytes(2048));m[0x8000:0xc000]=prg[:16384];call(m,0x8000,0);call(m,0x8000,music)
 for _ in range(90):call(m,0x8003)
 call(m,0x8000,30)
 for tick in range(180):call(m,0x8003);audio.append(dict(music=music,frameAfterShot=tick+1,soundWork=m[0xff]))
(out/'music-shot-work.json').write_text(json.dumps(audio)+'\n')
raw=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()
prefixes=collections.Counter(tuple(raw[n+0x100:n+0x103]) for n in range(0,len(raw),2048))
first_tiles=sorted({p[2] for p in prefixes})
assert {p[0] for p in prefixes}=={0x20,0xff}
assert all(0xc0<=p[1]<=0xdf for p in prefixes)
m=Memory(prg,base);cpu=MPU(memory=m);count=0;counterexamples=[]
def probe(x,selected,velocity,sound_work,header,column,tile):
 global count
 m[0x518]=4;m[0x55d]=selected;m[0xff]=sound_work;m[0x100]=header;m[0x101]=0xc0+column;m[0x102]=tile
 put(m,x*256+113,0x6e,0x7b,0x88);put(m,velocity*256,0x47e,0x48b);put(m,73,0x498,0x4a5)
 cpu.pc=0xbdd2;cpu.x=6;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
 for _ in range(200):
  if cpu.pc==0x7000:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 left=selected&1
 offset=(0xc0+column+256*tile) if left else (sound_work+256*header)
 target=((64 if left else 463)+offset)&65535
 crossed=(x<target and velocity<0) or (x>=target and velocity>0)
 assert m[0x518]==(12 if crossed else 4)
 assert m[0x498]+256*m[0x4a5]==(0 if crossed else 73)
 count+=1
 return m[0x518],target
# Even selection never reads column/tile; odd selection never reads audio/header.
for x in range(47,480):
 for velocity in (-8,8):
  for header in (0x20,0xff):
   for work in range(256):probe(x,8,velocity,work,header,0,0)
  for column in range(32):
   for tile in first_tiles:probe(x,7,velocity,0,0xff,column,tile)
for x in (280,400):
 # These are interior Court2 device coordinates with positive ball velocity.
 # Use actual music+shot-driver results, not invented audio scratch values.
 observed={}
 for a in audio:
  action,target=probe(x,8,8,a['soundWork'],0xff,0,0)
  observed.setdefault(action,dict(x=x,selectedHoop=8,velocityX=8,graphicsPrefix=[255,192,0],target=target,afterAction=action,audio=a))
 if len(observed)>1:counterexamples.append(list(observed.values()))
report=dict(scope='controlled original BDD2 exhaustive scalar-domain execution; component reachability evidence is not a whole-match trace',romSHA256=hashlib.sha256(rom).hexdigest(),instructionCalls=count,xRange=[47,479],graphicsHeaders=[32,255],graphicsAddressLowRange=[192,223],graphicsFirstTiles=first_tiles,observedWorldPhaseSamples=len(raw)//2048,observedPrefixes=[dict(bytes=list(k),count=v) for k,v in prefixes.items()],audioDriver='original bank0 init0, music59..66, 90 play calls, init30, 180 play calls',audioSamples=len(audio),rightFlushedThreshold='207 + original audio work byte',rightPendingThreshold='8655 + original audio work byte',leftThreshold='(64 + last upload address low + 256 * first tile) modulo65536',counterexamples=counterexamples,conclusion='Audio changes the branch at valid Court2 device X=280/400; cannot replace it with zero or a fixed special-ball offset. Retain runtime guard until the minimal original audio-work and upload-prefix sources are modeled.')
(out/'audit.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:v for k,v in report.items() if k!='observedPrefixes'},indent=2))
