"""Execute complete original NMI/IRQ functions for typed display-program proof.

Controlled inactive upload queues and silent audio state. All instructions run;
PPU/MMC3/APU writes are recorded. CPU instruction cycles are exact; DMA hardware
stall cycles are not emulated. NMI display offsets begin after DMA/queue work at
F18F(Scene7) orF206(Scene1); IRQ effects precede audio work atselector1.
"""
import argparse,gzip,hashlib,json
from pathlib import Path
from verify_pass_floor_original import ROOT,Memory,MPU
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
OUT=ROOT/'games/basketball/engine/testdata'

class Devices(Memory):
 def __init__(self,ram):super().__init__(PRG,ram);self.device=[];self.writes=[]
 def __getitem__(self,a):
  if a==0x2002:return 0
  return super().__getitem__(a)
 def __setitem__(self,a,v):
  if isinstance(a,int):
   if a==0x8000:self.register=v&7;self.device.append((a,v));return
   if a==0x8001:
    self.device.append((a,v))
    if self.register in [6,7]:
     begin=0x8000+(self.register-6)*8192;list.__setitem__(self,slice(begin,begin+8192),self.prg[v*8192:(v+1)*8192])
    return
   if a>=0x2000:
    if 0x2000<=a<=0x2007 or 0x4000<=a<=0x4017 or a in [0xa000,0xa001,0xc000,0xc001,0xe000,0xe001]:self.device.append((a,v));return
    raise ValueError(('unexpected write',hex(a),v))
   self.writes.append((a,v))
  super().__setitem__(a,v)

def execute(context,stage=None):
 r=bytearray(2048);r[0x100]=128;r[0x4d]=128 if context['scene']==7 else 192;r[0x4e]=stage or 0;r[0x55]=6;r[0x56]=context['control'];r[0x57]=context['mask'];r[0x588]=context['scene'];r[0x589]=context['phase']
 r[0x4f:0x51]=bytes(context['backgroundCHR']);r[0x51:0x55]=bytes(context['spriteCHR']);r[0x397:0x39b]=context['cameraX'].to_bytes(2,'little')+context['cameraY'].to_bytes(2,'little');r[0x6d9]=context['stageOffset'];r[0x6da:0x6de]=bytes(context['credits'])
 # Source no-queued-text headers, no column upload, no playing song. No jumps
 # or calls are skipped, including sound-driver work in terminal IRQ selector1.
 m=Devices(r);c=MPU(memory=m,pc=0x7000);c.sp=255;c.p=0x20;c.a=0x53;c.x=0x12;c.y=0x21
 c.nmi() if stage is None else c.irq()
 origin_pc=(0xf18f if context['scene']==7 else 0xf206) if stage is None else 0xf2d4
 origin=None;allwrites=[];pcs=[]
 for i in range(100000):
  if c.pc==0x7000:break
  if c.pc==origin_pc:origin=c.processorCycles
  pc=c.pc;count=len(m.device);c.step()
  for address,value in m.device[count:]:allwrites.append(dict(pc=pc,address=address,value=value,cycle=c.processorCycles,relative=None if origin is None else c.processorCycles-origin))
  pcs.append(pc)
 else:raise RuntimeError(('function limit',hex(c.pc)))
 if origin is None:raise ValueError(('missing raster origin',origin_pc))
 # Decode only logical raster mutations; excludes PRGbank restoration, DMA,
 # upload payloads and APU writes. Header/reload times remain separate fields.
 effects=[];bank=None;address_high=None;scroll_x=True;disable=None;arm=None;latch=None
 for w in allwrites:
  if w['relative'] is None:continue
  a,v,t=w['address'],w['value'],w['relative'];kind=None;value=v
  if a==0x8000:bank=v&7
  elif a==0x8001 and bank is not None and bank<6:kind='chr'+str(bank)
  elif a==0xa000:kind='mirroring'
  elif a==0x2000:kind='control'
  elif a==0x2006:
   if address_high is None:address_high=v
   else:kind='address';value=address_high*256+v;address_high=None;scroll_x=True
  elif a==0x2005:kind='scrollX' if scroll_x else 'scrollY';scroll_x=not scroll_x
  elif a==0xc000:latch=v
  elif a==0xc001:arm=t
  elif a==0xe000 and stage==1:disable=t
  if kind:effects.append(dict(cycle=t,kind=kind,value=value))
 return dict(stage=m[0x4e],control=m[0x56],latch=latch,reloadCycle=arm,disableCycle=disable,effects=effects,instructionCycles=c.processorCycles-origin,fullFunctionInstructions=len(pcs),rawWrites=allwrites,visitedSHA256=hashlib.sha256(bytes().join(p.to_bytes(2,'little') for p in pcs)).hexdigest())

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args();cases=[]
 for scene in [1,7]:
  phases=[7,135] if scene==1 else list(range(12))+list(range(128,140))
  for phase in phases:
   for i,(x,y,offset,credits) in enumerate([(0,0,0,[0,0,0,0]),(0x19b,0x128,1,[1,127,128,255]),(0xfffe,0x1ef,254,[255,254,7,8])]):
    ctx=dict(scene=scene,phase=phase,machineControl=128 if scene==7 else 192,cameraX=x,cameraY=y,stageOffset=offset,credits=credits,backgroundCHR=[68,70] if scene==1 else [60,62],spriteCHR=[0,59,59,32] if scene==1 else [37,38,11,40],control=[0x88,0x8b,0x89][i],mask=30)
    nmi=execute(ctx);steps=[];current=nmi['stage'];ctx['control']=nmi['control']
    for _ in range(8):
     result=execute(ctx,current);steps.append(dict(inputStage=current,**result));ctx['control']=result['control']
     if current==1:break
     current=result['stage']
    else:raise ValueError('IRQ chain did not terminate')
    # Store original context, not the mutated control passed down the chain.
    ctx['control']=[0x88,0x8b,0x89][i]
    cases.append(dict(context=ctx,nmi=nmi,stages=steps))
 payload=gzip.compress(json.dumps(cases,separators=(',',':')).encode(),mtime=0);target=OUT/'postresult-display-program.json.gz'
 meta=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),cases=len(cases),irqCalls=sum(len(c['stages']) for c in cases),fixtureSHA256=hashlib.sha256(payload).hexdigest(),scope=__doc__,skippedCalls=[])
 if args.check:assert target.read_bytes()==payload
 else:target.write_bytes(payload);target.with_name('postresult-display-program-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
