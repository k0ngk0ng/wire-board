"""Source-only scheduler oracle. No production scheduler or CPU interpreter added.

Runs existing source probe machinery for bounded fade/background instructions;
compares source wait rules with retained whole-console event boundaries.
"""
import argparse,hashlib,json,math
from collections import Counter
from pathlib import Path
from verify_pass_floor_original import Memory,MPU
from static_sprites import SpriteROM
from py65.disassembler import Disassembler
from postresult_art import decode_background,PRESETS
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/postresult-scheduler'
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'

class ResourceMemory(Memory):
 def __init__(self,ram,chr_bank=0):
  self.ppu_address=0;self.latch=0;self.buffer=0;self.ppu_writes=[];self.chr_bank=chr_bank
  super().__init__(PRG,ram)
 def __getitem__(self,a):
  if isinstance(a,int) and a==0x2002:self.latch=0;return 0
  if isinstance(a,int) and a==0x2007:
   result=self.buffer;addr=self.ppu_address
   assert 0<=addr<4096,('source read outside compressed CHR',addr)
   self.buffer=ROM[131088+self.chr_bank*1024+addr];self.ppu_address+=1;return result
  return super().__getitem__(a)
 def __setitem__(self,a,v):
  if isinstance(a,int):
   if a==0x2006:
    if self.latch==0:self.ppu_address=(v&63)<<8
    else:self.ppu_address=(self.ppu_address&0xff00)|v
    self.latch^=1;list.__setitem__(self,a,v);return
   if a==0x2007:self.ppu_writes.append((self.ppu_address,v));self.ppu_address+=1;list.__setitem__(self,a,v);return
   if a==0x8001 and self.register<6:list.__setitem__(self,a,v);return
   if 0x2000<=a<=0x2007 or a==0xa000:list.__setitem__(self,a,v);return
  super().__setitem__(a,v)

def background_work():
 rom=SpriteROM(ROM);rows=[]
 for preset in PRESETS:
  spec=decode_background(rom,preset);r=bytearray(2048);r[0x3a0]=preset;r[0x56]=0x80;r[0x57]=0x1e
  m=ResourceMemory(r,spec['compressed_bank']);m[0x8000:0xc000]=PRG[3*16384:4*16384]
  c=MPU(memory=m,pc=0xdc15);c.sp=255;c.p=0x20;c.stPush(3)
  reads=0;writes=0;pcs=Counter()
  for n in range(1000000):
   if c.pc==0xdcb0:break
   pcs[c.pc]+=1;reads+=c.pc==0xdd34;writes+=c.pc==0xdd61;c.step()
  else:raise AssertionError('background source did not finish')
  assert m.ppu_writes==[(spec['target']+i,v)for i,v in enumerate(spec['data'])]
  # NMI-disabled interval only; handoff/outer waits remain separate events.
  rows.append(dict(preset=preset,source=spec['source'],compressedBank=spec['compressed_bank'],consumed=spec['consumed'],outputBytes=len(spec['data']),cycles=c.processorCycles,instructions=n,readCalls=reads,writeCalls=writes,coarseWorkFrames=math.ceil(c.processorCycles/29780.5),sourcePCCounts={f'{k:04X}':v for k,v in sorted(pcs.items())}))
 return rows

def fade_probes():
 rows=[]
 for scene,phase in [(1,7),(7,0),(7,8),(7,9)]:
  for delay in [0,1,2,3]:
   for flags in [0,128]:
    r=bytearray(2048);r[0x588]=scene;r[0x589]=phase;r[0x301]=flags;r[0x302]=delay;r[0x304]=9;r[0x305]=2;r[0x306]=3;r[0x4c]=77
    m=ResourceMemory(r);m[0x8000:0xc000]=PRG[4*16384:5*16384]
    c=MPU(memory=m,pc=0x94df);c.sp=255;c.p=0x20;c.stPushWord(0x6fff)
    ticks=0;commits=[];resets=[]
    for n in range(200000):
     if c.pc==0x7000:break
     if c.pc==0x961e and m[0x374]==0x3f:
      ticks+=1;m[0x374]=255;m[0x4c]=(m[0x4c]+1)&255;commits.append(dict(tick=ticks,level=m[0x303]//16,counter=m[0x4c]))
     if c.pc==0x9625:resets.append(dict(tick=ticks,prior=m[0x4c]))
     if c.pc==0x9627 and m[0x4c]<delay:ticks+=1;m[0x4c]=(m[0x4c]+1)&255
     c.step()
    else:raise AssertionError('fade source did not return')
    expected=1 if delay==0 else 4*(delay+1)
    assert ticks==expected and m[0x4c]==delay,(scene,phase,delay,flags,ticks,m[0x4c])
    assert [x['level']for x in commits]==([4]if delay==0 else [3,2,1,0]if flags&128 else [1,2,3,4])
    rows.append(dict(scene=scene,phase=phase,delay=delay,flags=flags,videoTicks=ticks,exitCounter=m[0x4c],commits=commits,resets=resets))
 return rows

def source_queue_commit(m):
 c=MPU(memory=m,pc=0xcc1a);c.sp=255;c.p=0x20;c.stPushWord(0x6fff)
 before=len(m.ppu_writes)
 for n in range(10000):
  if c.pc==0x7000:break
  c.step()
 else:raise AssertionError('source pending-queue consumer did not return')
 return dict(cycles=c.processorCycles,writes=len(m.ppu_writes)-before)

def queue_probes():
 rows=[]
 for mask in range(16):
  for length in [0,1,32]:
   r=bytearray(2048);r[0x4c]=73
   for slot in range(4):
    a=0x30b+slot*35;r[a]=0x23 if mask&(1<<slot) else 255;r[a+1]=slot*32;r[a+2]=length;r[a+3:a+35]=bytes(range(32));r[0x307+slot]=slot&1
   m=ResourceMemory(r);result=source_queue_commit(m)
   for slot in range(4):
    a=0x30b+slot*35
    assert m[a]==255 and m[a+1:a+35]==list(r[a+1:a+35])
    assert m[0x307+slot]==(0 if mask&(1<<slot)else r[0x307+slot])
   assert m[0x4c]==73 and result['writes']==mask.bit_count()*length
   rows.append(dict(mask=mask,length=length,**result))
 return rows

def message_probes():
 rows=[]
 for message in [28,30,29]:
  for counter in [0,1,2,255]:
   r=bytearray(2048);r[0x588]=1;r[0x589]=0x87;r[0x6f7]=message;r[0x4c]=counter
   for a in (0x30b,0x32e,0x351,0x374):r[a]=255
   m=ResourceMemory(r);m[0x8000:0xc000]=PRG[4*16384:5*16384]
   c=MPU(memory=m,pc=0xbe6e);c.sp=255;c.p=0x20;c.stPushWord(0x6fff);nmis=0
   for n in range(10000):
    if c.pc==0x7000:break
    if c.pc==0xbebe and nmis==0:
     assert m[0x351]==32 and m[0x374]==32
     source_queue_commit(m);m[0x4c]=(m[0x4c]+1)&255;nmis+=1
    c.step()
   else:raise AssertionError('source message did not return')
   assert nmis==1 and m[0x4c]==(counter+1)&255 and m[0x351]==m[0x374]==255
   rows.append(dict(message=message,entryCounter=counter,exitCounter=m[0x4c],normalNMIs=nmis))
 return rows


def observed(kind,bg):
 p=ROOT/f'.local/nes-basket/runs/postresult-{kind}-observed';events=[json.loads(l)for l in (p/'events.jsonl').open()]
 intervals=[];dispatch=None;wait=None;fades=[];messages=[];backgrounds=[];openings={};unmatched=[]
 for e in events:
  k=e['kind']
  if k=='schedule':
   if dispatch and wait:
    expected_scene=1 if kind=='reward'else 7
    if dispatch['scene']==expected_scene and (kind!='reward'or dispatch['phase']&127==7):
     threshold=2 if wait['scene']==1 else 0
     remaining=max(0,threshold-wait['videoCounter'])+1
     assert e['frame']-wait['frame']==remaining,(kind,dispatch,wait,e)
     assert e['videoCounter']==(max(threshold,wait['videoCounter'])+1)&255
     intervals.append(dict(start=dispatch,wait=wait,next=e,remainingTicks=remaining,totalVideoFrames=e['frame']-dispatch['frame']))
   dispatch=None;wait=None
  elif k in ('scene1','scene7'):dispatch=e
  elif k=='scheduleWait':wait=e
  if k in ('fadeBegin','messageBegin','backgroundBegin'):openings[k]=e
  if k in ('fadeEnd','messageEnd','backgroundEnd'):
   begin=k.replace('End','Begin');a=openings.pop(begin,None)
   if a is None:unmatched.append(e);continue
   if a['scene']!=(1 if kind=='reward'else 7) or kind=='reward'and a['phase']&127!=7:continue
   span=dict(start=a,end=e,videoFrames=e['frame']-a['frame'],cycles=e['cycles']-a['cycles'])
   if begin=='fadeBegin':
    d=a['fade'][1];assert span['videoFrames']==(1 if d==0 else 4*(1+d));assert e['videoCounter']==d
    q=[x for x in events if a['cycles']<=x['cycles']<=e['cycles'] and x['kind']=='queueHeader'and x['address']==0x374]
    sets=[x for x in q if x['value']==63];consumes=[x for x in q if x['value']==255]
    assert len(sets)==len(consumes)==(1 if d==0 else 4)
    assert all(y['frame']==x['frame']+1 for x,y in zip(sets,consumes))
    assert all(y['frame']-x['frame']==d+1 for x,y in zip(sets,sets[1:]))
    span['queueWrites']=q;fades.append(span)
   elif begin=='messageBegin':assert span['videoFrames']==1;messages.append(span)
   else:backgrounds.append(span)
 presets=[26]if kind=='reward'else [95,96,97,33,34,25,98,23]
 assert len(backgrounds)==len(presets)
 for span,preset in zip(backgrounds,presets):
  work=next(r for r in bg if r['preset']==preset)
  span['preset']=preset;span['disabledWorkCycles']=work['cycles']
  span['sourceCoarseTicks']=3+work['coarseWorkFrames']
  assert span['videoFrames']==span['sourceCoarseTicks'],(kind,preset,span)
  # Two special handoff NMIs do not increment4C. Normal entry/exit ones do.
  assert span['end']['videoCounter']==(span['start']['videoCounter']+2)&255
 return dict(kind=kind,intervals=intervals,cadenceCounts=dict(Counter(r['totalVideoFrames']for r in intervals)),fades=fades,messages=messages,backgrounds=backgrounds,unmatched=unmatched,eventSHA256=hashlib.sha256((p/'events.jsonl').read_bytes()).hexdigest())

def source_instructions():
 m=ResourceMemory(bytes(2048));m[0x8000:0xc000]=PRG[4*16384:5*16384];d=Disassembler(MPU(memory=m));lines=[]
 for start,end in [(0xc000,0xc03f),(0xca54,0xca8b),(0xcaa4,0xcab0),(0xcae3,0xcb06),(0x94df,0x962f),(0xbe6e,0xbec3),(0xf831,0xf845),(0xf089,0xf0ad),(0xcc1a,0xcd3c),(0xdbf8,0xdc8e),(0xdc96,0xdd7a)]:
  lines.append(f'bank4 visible + fixed7 [{start:04X},{end:04X})');pc=start
  while pc<end:
   n,line=d.instruction_at(pc);lines.append(f'{pc:04X} '+bytes(m[pc:pc+n]).hex(' ').ljust(9)+' '+line);pc+=n
 lines.append('CB06 cadence data: '+bytes(m[0xcb06:0xcb0e]).hex(' '))
 lines.append('DC8E background dispatcher data: '+bytes(m[0xdc8e:0xdc96]).hex(' '))
 return ('\n'.join(lines)+'\n').encode()


def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args();OUT.mkdir(exist_ok=True)
 instructions=source_instructions();instruction_path=OUT/'instructions.txt'
 if args.check:assert instruction_path.read_bytes()==instructions
 else:instruction_path.write_bytes(instructions)
 bg=background_work();fades=fade_probes();queues=queue_probes();messages=message_probes();runs=[observed(k,bg)for k in ('reward','ending')]
 data=dict(instructionsSHA256=hashlib.sha256(instructions).hexdigest(),romSHA256=hashlib.sha256(ROM).hexdigest(),backgroundWork=bg,controlledFadeCalls=fades,controlledQueueCalls=queues,controlledMessageCalls=messages,observed=runs,scope=__doc__)
 encoded=(json.dumps(data,indent=2)+'\n').encode();path=OUT/'oracle.json'
 if args.check:assert path.read_bytes()==encoded
 else:path.write_bytes(encoded)
 print(json.dumps(dict(status='passed',oracleSHA256=hashlib.sha256(encoded).hexdigest(),fadeProbeCalls=len(fades),queueProbeCalls=len(queues),messageProbeCalls=len(messages),observed=[dict(kind=r['kind'],dispatchIntervals=len(r['intervals']),cadence=r['cadenceCounts'],fades=len(r['fades']),messages=len(r['messages']),backgrounds=len(r['backgrounds']))for r in runs],backgroundWork=[{k:v for k,v in r.items()if k!='sourcePCCounts'}for r in bg])))
if __name__=='__main__':main()
