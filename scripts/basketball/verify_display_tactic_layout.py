"""Original B074 glyph upload and AFF3 cache oracle; optional fixture export."""
import sys,json,hashlib,gzip,argparse
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'scripts/basketball'))
from verify_ai_mode5_plan11 import ROM,PRG,BASE,start,call,MPU
from verify_pass_floor_original import Memory
class RecordedMemory(Memory):
 def __init__(self,r):
  super().__init__(PRG,r);self.writes=[]
 def __setitem__(self,a,v):
  if isinstance(a,int):self.writes.append([a,v])
  super().__setitem__(a,v)
def run(r,entry):
 m=RecordedMemory(r);cpu=MPU(memory=m);call(cpu,entry,0,4)
 return m
rows=[]
for slot in [1,3]:
 for main in range(13):
  for reply in [0,1,2]:
   r=start(0);r[0x3ce]=slot;r[0x6c6:0x6ca]=bytes([0]*4);index=(slot//2)*2;r[0x6c6+index]=main;r[0x6c7+index]=reply;r[0x3c7:0x3cb]=bytes([255,0,255,0]);r[0x166:0x16a]=bytes([17,23,0,0])
   m=run(r,0xb074)
   rows.append(dict(slot=slot,main=main,reply=reply,upperUpload=dict(address=m[0x30b]*256+m[0x30c],length=m[0x30d],glyphs=m[0x30e:0x317]),lowerUpload=dict(address=m[0x32e]*256+m[0x32f],length=m[0x330],glyphs=m[0x331:0x33a]),upperWrites=[[hex(a),v] for a,v in m.writes if 0x30e<=a<0x31b],lowerWrites=[[hex(a),v] for a,v in m.writes if 0x331<=a<0x33e],cacheBefore=list(r[0x3c7:0x3cb]),cacheAfter=list(m[0x3c7:0x3cb]),cacheWrites=[[hex(a),v] for a,v in m.writes if 0x3c7<=a<0x3cb]))
r=start(0);r[0x300]=0;r[0x3ce]=0;r[0x3c7:0x3cb]=bytes([255,0,255,0]);r[0x6c6:0x6ca]=bytes([4,1,7,2]);r[0x6c2:0x6c6]=bytes([3,0,0,200]);r[0x3cd]=0
m=RecordedMemory(r);cpu=MPU(memory=m);sequence=[]
for i in range(12):
 m.writes=[];before=dict(turn=m[0x3ce],cache=list(m[0x3c7:0x3cb]));call(cpu,0xaff3,0,4)
 sequence.append(dict(before=before,after=dict(turn=m[0x3ce],cache=list(m[0x3c7:0x3cb])),cacheWrites=[[hex(a),v] for a,v in m.writes if 0x3c7<=a<0x3cb]))
out=ROOT/'.local/basketball/rea/hud-command-layout'
out.mkdir(parents=True,exist_ok=True)
(out/'panel-calls.json').write_text(json.dumps(rows,indent=2)+'\n');(out/'scheduler-chain.json').write_text(json.dumps(sequence,indent=2)+'\n')
assert all(not r['cacheWrites'] and r['cacheBefore']==r['cacheAfter'] for r in rows)
assert all(r['upperUpload']['length']==9 and r['lowerUpload']['length']==9 and r['upperUpload']['glyphs'][6]==0 and r['lowerUpload']['glyphs'][6]==0 for r in rows)
assert all(not r['cacheWrites'] and r['before']['cache']==r['after']['cache'] for r in sequence)
meta=dict(scope='isolated original B074 panel and continuous AFF3 scheduler execution, no instruction modifications and no PPU/NMI emulation',romSHA256=hashlib.sha256(ROM).hexdigest(),panelCalls=len(rows),schedulerCalls=len(sequence),tableOffsets=list(PRG[4*16384+0xb17a-0x8000:4*16384+0xb17e-0x8000]),cacheWriteCount=0,slots=[s['after']['turn'] for s in sequence],outputs={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in [out/'panel-calls.json',out/'scheduler-chain.json']})
(out/'result.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))

parser=argparse.ArgumentParser();parser.add_argument('--write-fixture',action='store_true');args=parser.parse_args()
if args.write_fixture:
 target=ROOT/'games/basketball/engine/testdata/display-tactic-layout.json.gz'
 packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);target.write_bytes(packed)
 meta['fixtureSHA256']=hashlib.sha256(packed).hexdigest()
 target.with_name('display-tactic-layout-source.json').write_text(json.dumps(meta,indent=2)+'\n')
