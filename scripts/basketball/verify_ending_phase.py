"""Unchanged Scene7 phase calls, stopping BEFORE BF35 actors; see failure ledger."""
import base64,gzip,hashlib,json,itertools
from pathlib import Path
from verify_pass_floor_original import Memory,MPU
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'games/basketball/engine/testdata'
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
SOURCE=ROOT/'.local/postresult-source-map'
RAW=gzip.decompress((SOURCE/'ending-None.ram.gz').read_bytes())
TRACE=json.loads(gzip.decompress((SOURCE/'ending-None.json.gz').read_bytes()))
class Mem(Memory):
 def __init__(self,*args):super().__init__(*args);self.mirror=[]
 def __setitem__(self,a,v):
  if a==0xa000:self.mirror.append(v);list.__setitem__(self,a,v);return
  super().__setitem__(a,v)
def run(label,r):
 m=Mem(PRG,r);m[0x8000:0xc000]=PRG[3*16384:4*16384]
 c=MPU(memory=m,pc=0xb995);c.sp=255;c.p=0x20;c.stPushWord(0x6fff)
 events=[];path=[];rng=0;mirrors=0
 for count in range(20000):
  if c.pc in (0xbf35,0xf79d):break
  path.append(c.pc)
  if c.pc==0xf9c3:rng+=1
  if c.pc in (0xc000,0xc006,0xc021,0xc024):
   if c.pc==0xc000:events.append(dict(kind='sound',value=c.a))
   if c.pc==0xc006:events.append(dict(kind='chr',chrPages=list(m[0x51:0x55])))
   if c.pc==0xc021:events.append(dict(kind='fade',value=m[0x301],delay=m[0x302]))
   if c.pc==0xc024:events.append(dict(kind='background',value=m[0x3a0]))
   c.pc=(c.stPopWord()+1)&65535
  else:c.step()
  if len(m.mirror)>mirrors:
   events.append(dict(kind='mirroring',value=m.mirror[-1]));mirrors=len(m.mirror)
 else:raise RuntimeError((label,hex(c.pc)))
 assert rng==0
 return dict(label=label,before=base64.b64encode(r).decode(),after=base64.b64encode(bytes(m[:2048])).decode(),requests=events,reset=c.pc==0xf79d,path=path,rngCalls=rng)
def main():
 rows=[];selected=[]
 # Sample source trajectory phase changes, entry/exit, every timer byte boundary,
 # and every phase5 actor2 action change; retain exact original initial RAM.
 for i,t in enumerate(TRACE):
  b=t['before'];a=t['after'];prior=TRACE[i-1]['before'] if i else None
  if not b['entered'] or b['phase']!=a['phase'] or b['timer']&255 in (0,1,254,255) or b['phase'] in (0,2,4,6,7,9,10) or (prior and b['actions'][2]!=prior['actions'][2]):
   r=bytearray(RAW[i*2048:(i+1)*2048]);r[4]=t['pressed'];rows.append(run(f'trajectory-{i}',r));selected.append(i)
 base=bytearray(RAW[:2048])
 # Distinct sentinel bytes expose accidental clears; no actor instructions run.
 for phase,entered in itertools.product(range(11),[False,True]):
  for timer in [0,1,255,256,0x36f,0x370,0xe07,0xe08,0xf7f,0xf80,0x10bf,0x10c0,65535]:
   if phase==5 and timer>0xf80 and timer!=65535:continue
   r=bytearray(base);r[0x589]=phase|(128 if entered else 0);r[0x588]=7;r[0x58c]=28
   r[0x6d7:0x6d9]=timer.to_bytes(2,'little');r[0x6d9:0x6df]=bytes([17,1,0,3,255,127])
   r[0x6ab:0x6ae]=bytes([19,29,39]);r[0x301:0x303]=bytes([7,9]);r[0x305:0x307]=bytes([13,17]);r[0x30b:0x32e]=bytes(range(35))
   r[0x6ca:0x6d2]=bytes([1,2,3,4,5,6,7,8]);r[4]=0
   for a in range(4):
    r[0x512+a]=0xde if a==2 else 0x80+a;r[0x437+a]=19+a;r[0x444+a]=23+a;r[0x5b+a]=9+a
    r[0x478+a]=1+a;r[0x485+a]=2+a;r[0x492+a]=3+a;r[0x49f+a]=4+a
   rows.append(run(f'phase-{phase}-{entered}-timer-{timer}',r))
 for period in [0,1,27,28,29,255]:
  r=bytearray(base);r[0x589]=0;r[0x58c]=period;rows.append(run(f'period-{period}',r))
 for entered,pressed in itertools.product([False,True],[0,1,2,4,8,16,32,64,128,144,255]):
  r=bytearray(base);r[0x589]=9|(128 if entered else 0);r[4]=pressed;rows.append(run(f'confirm-{entered}-{pressed}',r))
 for action,entered,timer,offset in itertools.product(range(12,24),[False,True],[0,254,255,0x3ff,0x4ff,0x5ff,0xeff,0xf7f],[0,1,255]):
  r=bytearray(base);r[0x589]=0x85;r[0x514]=action|(128 if entered else 0);r[0x6d7:0x6d9]=timer.to_bytes(2,'little');r[0x6d9]=offset;r[0x446]=31;r[0x439]=23
  rows.append(run(f'phase5-{action}-{entered}-{timer}-{offset}',r))
 for credits,timer in itertools.product([[0,0,0,0,0],[1,1,1,1,127],[255,255,255,255,128],[2,3,0,4,255]],[0,0x10bf,0x10c0,65535]):
  r=bytearray(base);r[0x589]=0x88;r[0x6da:0x6df]=bytes(credits);r[0x6d7:0x6d9]=timer.to_bytes(2,'little');rows.append(run(f'credits-{credits}-{timer}',r))
 packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);(OUT/'ending-phase.json.gz').write_bytes(packed)
 meta=dict(calls=len(rows),romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),trajectoryRAMSHA256=hashlib.sha256(RAW).hexdigest(),sampledTrajectoryCalls=selected,scope='declared controlled original phase calls; stop BF35 before actor tail or F79D before reset',omissions={'C000':'audio request','C006':'CHR request','C021':'fade request; no NMI waits','C024':'background request','BF35':'actor tail; explicit component boundary','outer':'FD0A text and input/video scheduling omitted'},rngCalls=0)
 (OUT/'ending-phase-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps({k:v for k,v in meta.items()if k!='sampledTrajectoryCalls'}))
if __name__=='__main__':main()
