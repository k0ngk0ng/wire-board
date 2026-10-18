"""Rebuild original896D opponent roster oracle; no gameplay or menu instruction patching."""
import sys,json,hashlib,itertools,gzip
from pathlib import Path
ROOT=Path.cwd();sys.path.insert(0,str(ROOT/'scripts/basketball'))
from verify_pass_floor_original import MPU
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b';prg=ROM[16:16+131072]
out=ROOT/'games/basketball/engine/testdata';source=ROOT/'.local/nes-basket/runs/campaign-audit-equipped-home-match/ram.bin';raw=source.read_bytes();BASE=raw[3409*2048:3410*2048]
def machine(r):
 m=list(r)+[0]*(65536-len(r));m[0x8000:0xc000]=prg[3*16384:4*16384];m[0xc000:]=prg[-16384:];return MPU(memory=m)
def decode(r):return dict(mode=r[0x58d],period=r[0x58c],teams=list(r[0x586:0x588]),pending=list(r[0x3cb:0x3cf]),characters=list(r[0x3e3:0x3e7]),random=int.from_bytes(bytes(r[0x48:0x4c]),'big'),phase=r[0x589],selectorTeam=r[0x6d7],enemyController=r[0x4ea],enemySelectionStatus=r[0x3d2],court=r[0x58b])
def run(r):
 cpu=machine(r);cpu.pc=0x896d;cpu.sp=255;draws=0
 for i in range(300):
  if cpu.pc in (0x89ac,0x8088):break
  if cpu.pc==0xc018:draws+=1
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return dict(before=decode(r),after=decode(cpu.memory),randomCalls=draws,stop=hex(cpu.pc)),bytes(cpu.memory[:2048])
def main():
 rows=[]
 for mode,period,team,value in itertools.product([0,1],[0,1,2,3,4,27,28],range(1,8),range(4)):
  r=bytearray(BASE);r[0x58d]=mode;r[0x58c]=period;r[0x587]=team;r[0x3cb:0x3cf]=bytes([2,3,3,0]);r[0x48:0x4c]=((value*2<<24)|0x00345678).to_bytes(4,'big');row,_=run(r);rows.append(row)
  assert row['randomCalls']==1 and row['after']['pending']==[2,3,0,[1,2,3,1][value]]
  assert row['after']['characters']==row['before']['characters']
 boundaries=[]
 for mode,period,selectionStatus,selected in itertools.product([128,129],[0,1,2,28],[0,1,128,255],[0,1]):
  r=bytearray(BASE);r[0x58d]=mode;r[0x58c]=period;r[0x3d2]=selectionStatus;r[0x4ea]=1;r[0x6d7]=selected
  row,_=run(r);boundaries.append(row);assert row['randomCalls']==0 and row['after']['pending']==row['before']['pending']
 natural=[]
 for name,frame in [('campaign-audit-first-match-confirm',769),('campaign-audit-coop-first-match',769),('campaign-audit-equipped-home-match',3410),('campaign-audit-loss-continue',11009)]:
  path=ROOT/'.local/nes-basket/runs'/name/'ram.bin'
  if not path.exists():continue
  b=path.read_bytes();r=b[(frame-1)*2048:frame*2048];next=b[frame*2048:(frame+1)*2048];row,_=run(r)
  assert row['after']['pending']==list(next[0x3cb:0x3cf]) and row['after']['random']==int.from_bytes(next[0x48:0x4c],'big')
  natural.append(dict(run=name,beforeVideoFrame=frame,nextVideoFrame=frame+1,ramSHA256=hashlib.sha256(b).hexdigest(),oracle=row,observed=decode(next)))
 result=dict(scope='controlled original896D through89AC plus read-only exact retained natural menu boundaries; no complete match rerun',romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),entry='bank3 896D',tableAddress='8986',table=list(prg[3*16384+0x8986-0x8000:3*16384+0x898a-0x8000]),calls=rows,boundaries=boundaries,natural=natural)
 payload=gzip.compress((json.dumps(result,separators=(',',':'))+'\n').encode(),mtime=0)
 (out/'campaign-roster-calls.json.gz').write_bytes(payload)
 meta=dict(scope=result['scope'],romSHA256=result['romSHA256'],baseSHA256=result['baseSHA256'],entry=result['entry'],tableAddress=result['tableAddress'],table=result['table'],normalCalls=len(rows),boundaryCalls=len(boundaries),naturalBoundaries=len(natural),fixtureSHA256=hashlib.sha256(payload).hexdigest(),sourceRuns={r['run']:r['ramSHA256'] for r in natural})
 (out/'campaign-roster-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))

if __name__=="__main__":main()
