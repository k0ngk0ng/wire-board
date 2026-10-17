"""Isolate original actor dispatch with controlled ownership; not a natural match capture."""
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
 def __setitem__(self,address,value):
  if isinstance(address,int) and address>=0x8000:
   if address==0x8000:self.bank_select=value&7
   if address==0x8001 and self.bank_select in (6,7):
    start=0x8000+(self.bank_select-6)*8192
    super().__setitem__(slice(start,start+8192),prg[(value%16)*8192:(value%16+1)*8192])
   return
  super().__setitem__(address,value)
source=ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin';base=source.read_bytes()[:2048]
rows=[]
for action in range(7):
 for computer in (False,True):
  for entered in (False,True):
   for variant in ('normal','charge','jump-skip')+(tuple(f'tail{cursor}-{clock}' for cursor,ticks in [(3,15),(4,255),(5,1),(6,255)] for clock in (0,ticks-1)) if action==2 else ()):
    mem=Memory();mem[:2048]=base;actor=1
    mem[0x53f]=actor;mem[0x539+actor]=128 if computer else 0
    mem[0x512+actor]=action|(128 if entered else 0);mem[0x51f+actor]=128
    mem[0x642+actor]=64;mem[0x6d7+actor]=0;mem[0x4ec+actor]=64
    mem[0x437+actor]=0;mem[0x444+actor]=0;mem[0x451+actor]=0;mem[0x52c+actor]=255
    mem[0x572+actor]=1 if variant=='charge' else 8;mem[0x576+actor]=255 if variant=='jump-skip' else 0
    mem[0x478+actor]=0;mem[0x485+actor]=4;mem[0x492+actor]=0;mem[0x49f+actor]=1
    mem[0x4ac+actor]=0;mem[0x4b9+actor]=0;mem[0x4c6+actor]=0;mem[0x4d3+actor]=0
    mem[0x68+actor]=33;mem[0x75+actor]=200;mem[0x82+actor]=0
    mem[0x8f+actor]=55;mem[0x9c+actor]=40;mem[0xa9+actor]=0;mem[0xb6+actor]=0;mem[0xc3+actor]=0;mem[0x5f7+actor]=0
    if variant.startswith('tail'):
     cursor,clock=map(int,variant[4:].split('-'));mem[0x444+actor]=cursor;mem[0x437+actor]=clock
    before=bytes(mem[:2048]);cpu=MPU(memory=mem,pc=0xe165);cpu.x=actor
    for steps in range(20000):
     if cpu.pc==0xe168:break
     cpu.step()
    else:raise RuntimeError((action,computer,entered,variant,hex(cpu.pc)))
    after=bytes(mem[:2048]);rows.append(dict(action=action,computer=computer,entered=entered,variant=variant,steps=steps,actor=actor,before=decode_world_action_state(before),after=decode_world_action_state(after),context=context(before)['actions']))
out=ROOT/'games/basketball/engine/testdata/possessed-free-actions.json.gz';payload=(json.dumps(rows,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out.write_bytes(packed)
out.with_name('possessed-free-actions-source.json').write_text(json.dumps(dict(scope='isolated original instruction execution with controlled RAM, not natural game trace',records=len(rows),romSHA256=hashlib.sha256(rom).hexdigest(),baseRAMSHA256=hashlib.sha256(base).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),boundary='fixed E165..E168 single actor; original MMC3 bank selection'),indent=2)+'\n')
print(len(rows),'original isolated action calls')

fixed=prg[-16384:]
def read(a):return fixed[a-0xc000]
offsets=[]
for action in range(106):
 index=read(0xecf5+action);pointer=read(0xed65+index*2)+256*read(0xed66+index*2)
 offsets.append(dict(action=action,index=index,address=pointer,bytes=[read(pointer+i) for i in range(4)]))
p=ROOT/'games/basketball/engine/testdata/held-pose-offsets.json';p.write_text(json.dumps(offsets,indent=2)+'\n')
p.with_name('held-pose-offsets-source.json').write_text(json.dumps(dict(scope='original fixed-bank action selectors and integer pose offsets; independent of player handler support',romSHA256=hashlib.sha256(rom).hexdigest(),selectors='$ECF5..$ED5E',pointers='$ED65',records=len(offsets),fixtureSHA256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n')
