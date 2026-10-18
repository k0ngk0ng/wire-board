"""Continuous original ending kernel with C033 motion AND C00C projection.

Declared caller boundary: displayed camera copy and renderer D063 draw-list
finalization. No FD0A text/render/NMI/video scheduling or outer RNG is executed.
"""
import argparse,base64,gzip,hashlib,json
from verify_ending_phase import ROOT,OUT,ROM,PRG,Mem,MPU
OMIT={0xc000:'audio request',0xc006:'CHR request',0xc021:'fade request; no waits',0xc024:'background request',0xc036:'message upload request'}
def execute(m,entry,bank):
 m[0x8000:0xc000]=PRG[bank*16384:(bank+1)*16384];c=MPU(memory=m,pc=entry);c.sp=255;c.p=0x20;c.stPushWord(0x6fff)
 events=[];actors=[];rng=0;projection=0;mirrors=len(m.mirror)
 for count in range(40000):
  if c.pc in (0x7000,0xf79d):break
  if c.pc==0xf9c3:rng+=1
  if c.pc==0xc033:actors.append(c.x)
  if c.pc==0xc00c:projection+=1
  if c.pc in OMIT:
   if c.pc==0xc000:events.append(dict(kind='sound',value=c.a))
   if c.pc==0xc006:events.append(dict(kind='chr',chrPages=list(m[0x51:0x55])))
   if c.pc==0xc021:events.append(dict(kind='fade',value=m[0x301],delay=m[0x302]))
   if c.pc==0xc024:events.append(dict(kind='background',value=m[0x3a0]))
   if c.pc==0xc036:raise RuntimeError('unexpected message upload in ending kernel')
   c.pc=(c.stPopWord()+1)&65535
  else:c.step()
  if len(m.mirror)>mirrors:events.append(dict(kind='mirroring',value=m.mirror[-1]));mirrors=len(m.mirror)
 else:raise RuntimeError(('instruction bound',hex(c.pc)))
 return dict(requests=events,actors=actors,rngCalls=rng,projectionCalls=projection,instructions=count,reset=c.pc==0xf79d)
def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args()
 source=OUT/'campaign-settlement-cases.json.gz';cases=json.loads(gzip.decompress(source.read_bytes()))['cases']
 row=next(r for r in cases if r['nextStage']=='complete' and r['inventoryShape']=='empty')
 raw=base64.b64decode(row['phases'][1]['rawAfter']);m=Mem(PRG,raw);tail=execute(m,0xbc96,4)
 assert tail['actors']==[0,1,2,3] and tail['projectionCalls']==4
 initial=bytes(m[:2048]);calls=[];phases=[]
 for tick in range(14000):
  # Exact documented caller boundary, not execution of the full CE09 renderer.
  m[0x397:0x39b]=m[0x39b:0x39f];m[0x41c]=0;m[0x41d:0x429]=[255]*12
  phase=m[0x589]&127
  if not phases or phases[-1]['phase']!=phase:phases.append(dict(tick=tick,phase=phase))
  m[4]=16 if phase==9 and m[0x589]&128 else 0
  result=execute(m,0xb995,3)
  assert result['rngCalls']==0
  assert result['actors']==([] if result['reset'] else [0,1,2,3])
  assert result['projectionCalls']==len(result['actors'])
  calls.append(dict(pressed=m[4],after=base64.b64encode(bytes(m[:2048])).decode(),**result))
  if result['reset']:break
 else:raise RuntimeError('ending did not reset')
 assert len(calls)==12745,(len(calls),phases)
 data=dict(confirmationBefore=base64.b64encode(raw).decode(),confirmationTail=tail,initial=base64.b64encode(initial).decode(),calls=calls)
 payload=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0);path=OUT/'ending-presentation.json.gz'
 meta=dict(calls=len(calls),actorCalls=sum(len(r['actors']) for r in calls),projectionCalls=sum(r['projectionCalls'] for r in calls),rngCalls=0,phases=phases,romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(payload).hexdigest(),settlementCase=row['label'],settlementSourceSHA256=hashlib.sha256(source.read_bytes()).hexdigest(),omittedCalls={f'{k:04X}':v for k,v in OMIT.items()},callerBoundary='direct camera copy039B..039E to0397..039A; exact draw-list clear041D..0428 and count041C (D063 finalization); no source RAM reload',scope=__doc__)
 if args.check:assert path.read_bytes()==payload
 else:path.write_bytes(payload);(OUT/'ending-presentation-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
