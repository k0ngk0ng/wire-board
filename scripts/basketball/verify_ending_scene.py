"""Logical FD0A -> FC83 composition; actual capture and declared CPU kernel.

Whole-console resource calls are labelled before comparison, never silently
repaired. The continuous kernel consumes upload header highs externally once
per call, resets the renderer draw list, and copies camera at its boundary.
"""
import argparse,base64,gzip,hashlib,json
from verify_ending_presentation import ROOT,OUT,ROM,PRG,Mem,execute
CAPTURE=ROOT/'.local/nes-basket/runs/postresult-ending-observed'
class TextMemory(Mem):
 def __init__(self,*args):self.headers=[];super().__init__(*args)
 def __setitem__(self,a,v):
  if isinstance(a,int) and a in (0x30b,0x32e):self.headers.append(a)
  super().__setitem__(a,v)
def upload(r,slot):
 a=0x30b+slot*35
 return dict(address=r[a]*256+r[a+1],length=r[a+2],data=list(r[a+3:a+35]))
def queued_requests(headers,raw):
 return [dict(kind='text-upload',slot=i,upload=upload(raw,i))for i in range(2)if 0x30b+i*35 in headers]
def capture_rows():
 raw=(CAPTURE/'boundaries.ram').read_bytes();records=[];current=None
 def snapshot(e):return raw[e['raw']*2048:(e['raw']+1)*2048]
 for line in (CAPTURE/'events.jsonl').open():
  e=json.loads(line)
  if e['kind']=='textBegin' and e['scene']==7:
   assert current is None
   current=dict(frame=e['frame'],cycles=e['cycles'],before=snapshot(e),external=[],textHeaders=[],sounds=[])
  elif current is not None:
   if e['kind']=='textEnd':current['afterText']=snapshot(e)
   elif e['kind']=='endingBegin':current['beforePhase']=snapshot(e)
   elif e['kind']=='queueHeader' and 'afterText' not in current:current['textHeaders'].append(e['address'])
   elif e['kind'] in ('fadeBegin','backgroundBegin'):current['external'].append(dict(kind=e['kind'],frame=e['frame'],cycles=e['cycles']))
   elif e['kind']=='sound':current['sounds'].append(e['value'])
   elif e['kind'] in ('endingEnd','reset'):
    current['reset']=e['kind']=='reset';current['after']=b'' if current['reset'] else snapshot(e)
    current['endFrame']=e['frame'];current['endCycles']=e['cycles'];records.append(current);current=None
 assert current is None and len(records)==13370
 for r in records:
  r['textRequests']=queued_requests(r['textHeaders'],r['afterText'])
  for k in ('before','afterText','beforePhase','after'):r[k]=base64.b64encode(r[k]).decode()
 return records

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args()
 actual=capture_rows();initial=base64.b64decode(actual[0]['before']);m=TextMemory(PRG,initial);calls=[]
 for tick in range(14000):
  # Declared NMI queue consumption: only header high bytes becomeFF. Preserve
  # address low/length/data; no expected state or counter is reloaded.
  m[0x30b]=255;m[0x32e]=255
  m[0x397:0x39b]=m[0x39b:0x39f];m[0x41c]=0;m[0x41d:0x429]=[255]*12
  m[4]=16 if m[0x589]&127==9 and m[0x589]&128 else 0
  m.headers=[];text=execute(m,0xfd0a,3);assert not text['requests'] and not text['actors'] and text['rngCalls']==0
  text_after=bytes(m[:2048]);requests=queued_requests(m.headers,text_after)
  phase=execute(m,0xfc83,3);assert phase['rngCalls']==0
  requests += [dict(kind='presentation',presentation=e)for e in phase['requests']]
  calls.append(dict(pressed=m[4],after=base64.b64encode(bytes(m[:2048])).decode(),requests=requests,reset=phase['reset'],textInstructions=text['instructions'],phaseInstructions=phase['instructions'],actors=phase['actors'],rngCalls=0))
  if phase['reset']:break
 else:raise RuntimeError('ending scene failed to reset')
 assert len(calls)==12745
 data=dict(initial=base64.b64encode(initial).decode(),calls=calls,observed=actual)
 packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0);path=OUT/'ending-scene.json.gz'
 meta=dict(calls=len(calls),observedCalls=len(actual),observedResourceBoundaryCalls=sum(bool(r['external'])for r in actual),observedResetCalls=sum(r['reset']for r in actual),romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),captureEventSHA256=hashlib.sha256((CAPTURE/'events.jsonl').read_bytes()).hexdigest(),captureRAMSHA256=hashlib.sha256((CAPTURE/'boundaries.ram').read_bytes()).hexdigest(),scope=__doc__,kernelExternalBoundary='both upload header highbytes030B/032E=FF; camera display copy; D063 draw finalization; C000/C006/C021/C024 emitted as requests; no hidden RAM reload',capturedBoundary='whole-console textBeginFD0A to endingEndBF3F; resource waits separately labelled; reset has no finalRAM snapshot')
 if args.check:assert path.read_bytes()==packed
 else:path.write_bytes(packed);(OUT/'ending-scene-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
