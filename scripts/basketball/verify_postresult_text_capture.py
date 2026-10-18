"""Corroborate isolated source text/message writes against declared controlled
whole-console captures. NMI consumption of pending header high bytes is reported
separately; it is never patched into either input or expected source execution.
"""
import hashlib,json,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];sys.path.insert(0,str(ROOT/'scripts/basketball'))
from verify_postresult_text import execute
OUT=ROOT/'.local/postresult-text';OUT.mkdir(parents=True,exist_ok=True)
results=[]
for name in ['postresult-ending-observed','postresult-reward-observed']:
 p=ROOT/'.local/nes-basket/runs'/name;events=p/'events.jsonl';ram=p/'boundaries.ram';active={};counts={'text':0,'message':0};consumed=[];errors=[];queues=[]
 with ram.open('rb')as f:
  def read(e):f.seek(e['raw']*2048);r=f.read(2048);assert len(r)==2048;return r
  for line in events.open():
   e=json.loads(line);k=e['kind']
   if k in ['textBegin','messageBegin']:
    kind='text'if k=='textBegin'else'message';assert kind not in active;active[kind]=(e,read(e));queues=[]
   elif k=='queueHeader':queues.append(e)
   elif k in ['textEnd','messageEnd']:
    kind='text'if k=='textEnd'else'message';start,before=active.pop(kind);observed=read(e)
    expected,meta=execute(before,0xa496 if kind=='text'else 0xbe6e,0x7000 if kind=='text'else 0xbebc)
    watched=list(range(0x6ab,0x6b8))+list(range(0x304,0x351))if kind=='text'else list(range(0x351,0x397))
    diffs=[dict(address=f'{a:04X}',expected=expected[a],observed=observed[a])for a in watched if expected[a]!=observed[a]]
    header={0x30b,0x32e}if kind=='text'else{0x351,0x374}
    valid=[d for d in diffs if int(d['address'],16)in header and d['observed']==255 and any(q['address']==int(d['address'],16)and q['value']==255 for q in queues)]
    if valid:consumed.append(dict(kind=kind,frame=start['frame'],exitFrame=e['frame'],headers=valid))
    invalid=[d for d in diffs if d not in valid]
    if invalid:errors.append(dict(kind=kind,frame=start['frame'],exitFrame=e['frame'],differences=invalid))
    counts[kind]+=1
 incomplete=[dict(kind=k,frame=v[0]['frame'],raw=v[0]['raw'],reason='capture ends before matching return; excluded from comparison')for k,v in active.items()]
 result=dict(incomplete=incomplete,run=name,eventsSHA256=hashlib.file_digest(events.open('rb'),'sha256').hexdigest(),ramSHA256=hashlib.file_digest(ram.open('rb'),'sha256').hexdigest(),calls=counts,nmiConsumedHeaders=consumed,errors=errors)
 results.append(result)
(OUT/'whole-console-corroboration.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps([dict(run=r['run'],calls=r['calls'],nmiConsumedCases=len(r['nmiConsumedHeaders']),errors=len(r['errors']),incomplete=r['incomplete'],firstErrors=r['errors'][:2])for r in results]))
assert not any(r['errors']for r in results)
