"""Original 3E actor calls with controlled RAM, preserving pre-motion and final boundaries."""
import gzip,hashlib,json,sys
from pathlib import Path
from extract_world_actions import ROOT,decode_world_action_state
from extract_world_preparation import context
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
class Memory(list):
 def __init__(self):
  super().__init__([0]*65536);self.bank_select=0
  self[0x8000:0xc000]=prg[6*16384:7*16384];self[0xc000:]=prg[-16384:]
 def __setitem__(self,a,v):
  if isinstance(a,int) and a>=0x8000:
   if a==0x8000:self.bank_select=v&7
   if a==0x8001 and self.bank_select in (6,7):
    start=0x8000+(self.bank_select-6)*8192;super().__setitem__(slice(start,start+8192),prg[(v%16)*8192:(v%16+1)*8192])
   return
  super().__setitem__(a,v)
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
rows=[]
for court in (1,2):
 for entered in (False,True):
  for variant in ['low-even','low-odd','middle','high','attached-left','attached-right','aux-owned','human-up','computer-cycle','equipment2','equipment4','special-negative','landing-request','release-char0','release-char0zero','release-char1']:
   m=Memory();m[:2048]=base;actor=1
   m[0x58b]=court;m[0x3da]=12;m[0x3db]=13 if court==2 else 11
   m[0x512+actor]=62|(128 if entered else 0);m[0x51f+actor]=0x90 if variant=='landing-request' else 0x80
   m[0x642+actor]=0xc0 if variant.startswith('attached') else 0x40
   m[0x539+actor]=0x80 if variant=='computer-cycle' else 0
   m[0x53f]=actor;m[0x53d]=actor if variant=='aux-owned' else 255;m[0x53e]=255
   m[0x48:0x4c]=[0x12,0x34,0x56,0x79 if variant=='low-odd' else 0x78]
   m[0x437+actor]=0;m[0x444+actor]=0;m[0x451+actor]=0;m[0x52c+actor]=255
   m[0x5b+actor]=5;m[0x46b+actor]=0;m[0x46b]=128 if variant=='attached-left' else 0
   m[0x554+actor]=128 if variant=='special-negative' else 1;m[0x572+actor]=8
   m[0x58d]=0 if variant.startswith('equipment') or variant=='special-negative' else 128
   m[0x586]=0;m[0x3e3+actor]=1;m[0x6e8]=int(variant.removeprefix('equipment')) if variant.startswith('equipment') else 255
   m[0x4e0+actor]=8 if variant=='human-up' else 0;m[0x55d]=7;m[0x6f0]=3 if variant=='high' else 1
   for j in (0,actor):
    m[0x68+j]=33;m[0x75+j]=24;m[0x82+j]=1;m[0x8f+j]=17;m[0x9c+j]=76 if court==2 else 40
    h=56 if variant=='middle' else 152 if variant=='high' else 0
    m[0xa9+j]=0;m[0xb6+j]=h;m[0xc3+j]=0;m[0x5f7+j]=128 if h else 0
    for a in (0x478,0x485,0x492,0x49f,0x4ac,0x4b9,0x4c6,0x4d3):m[a+j]=0
    m[0x6d7+j]=0
   if variant.startswith('release-'):
    m[0x53f]=0;m[0x512]=0x9b;m[0x58d]=0;m[0x3e3+actor]=0 if variant!='release-char1' else 1
    m[0x6e4]=255;m[0x6e7]=255;m[0x554]=0 if variant=='release-char0zero' else 3
   before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe165);cpu.x=actor;sounds=[];middle=None
   for steps in range(20000):
    if cpu.pc==0x9e99 and middle is None:middle=bytes(m[:2048])
    if cpu.pc==0xfbcc:sounds.append(cpu.a)
    if cpu.pc==0xe168:break
    cpu.step()
   else:raise RuntimeError((court,entered,variant,hex(cpu.pc)))
   assert middle is not None
   after=bytes(m[:2048]);c=context(before)['actions'];c['actorHeldButtons']=list(before[0x4e0:0x4e4])
   rows.append(dict(court=court,entered=entered,variant=variant,actor=actor,before=decode_world_action_state(before),prepared=decode_world_action_state(middle),after=decode_world_action_state(after),context=c,held=list(before[0x4e0:0x4e4]),sounds=sounds))
out=ROOT/'games/basketball/engine/testdata/launched-player.json.gz';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);out.write_bytes(packed)
out.with_name('launched-player-source.json').write_text(json.dumps(dict(scope='controlled original E165..E168 execution; pre-animation boundary 9E99; not natural gameplay',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),baseRAMSHA256=hashlib.sha256(base).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest()),indent=2)+'\n');print(len(rows),'original launched player calls')
