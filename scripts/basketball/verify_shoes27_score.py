"""Full original9035..9290 rim calls; see SHOES27_SCORE_VERIFICATION.md first."""
import base64,gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
BASE=(ROOT/'.local/nes-basket/runs/ball-pass/ram.bin').read_bytes()[:2048]
assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
def main():
 rows=[]
 for shooter,char,item,mode,team,height,side,contact in itertools.product(range(4),range(4),[27,26,255],[0,1,128],[0,3],[80,176,272],[0,1],['goal','lock','outside','underside']):
  r=bytearray(BASE);r[0x1f]=6;r[0x58b]=1;r[0x58d]=mode;r[0x586+shooter//2]=team;r[0x3e3+shooter]=char
  r[0x6e4:0x6f0]=bytes([255]*12);r[0x6e6+3*char]=item;r[0x6d1]=shooter;r[0x512+shooter]=0x1b;r[0x518]=0x8d;r[0x6cf]=13;r[0x55d]=7;r[0x53f]=6;r[0xe3]=0
  r[0x166:0x16a]=bytes([254,29,0,0]);put(r,200,0x55e,0x55f)
  for i in range(7,13):r[0x512+i]=0;r[0x626+i]=0;r[0x539+i]=i
  x=65 if side==0 else 463;r[0x519]=129;r[0x62d]=16 if contact=='lock' else 0
  put(r,x*256,0x6f,0x7c,0x89);put(r,43*256,0x96,0xa3);put(r,height*256,0xb0,0xbd,0xca)
  put(r,(x+(20 if contact=='outside' else 0))*256+77,0x6e,0x7b,0x88);put(r,46*256+99,0x95,0xa2);put(r,(height+4)*256,0xaf,0xbc,0xc9)
  put(r,256 if contact=='underside' else -256,0x4b2,0x4bf);put(r,128,0x4cc,0x4d9)
  m=Memory(PRG,r);m[0x8000:0xc000]=PRG[4*16384:5*16384];cpu=MPU(memory=m,pc=0x9035);cpu.x=6;cpu.sp=255
  sounds=[]
  for n in range(20000):
   if cpu.pc==0x9290:break
   if cpu.pc in [0xc000,0xfbcc]:sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1
   else:cpu.step()
  else:raise RuntimeError(hex(cpu.pc))
  after=bytes(m[:2048]);beforeScores=[r[0x166]+r[0x168]*256,r[0x167]+r[0x169]*256];afterScores=[after[0x166]+after[0x168]*256,after[0x167]+after[0x169]*256]
  points=(4 if height==272 else 3)+(2 if shooter<2 and item==27 else 0)
  expected=beforeScores.copy()
  if contact=='goal':expected[side^1]+=points
  assert afterScores==expected,(shooter,char,item,mode,team,height,side,contact,afterScores,expected)
  rows.append(dict(shooter=shooter,char=char,item=item,mode=mode,team=team,height=height,side=side,contact=contact,before=base64.b64encode(r).decode(),after=base64.b64encode(after).decode(),sounds=sounds))
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0);(out/'shoes27-score.json.gz').write_bytes(packed)
 meta=dict(scope='controlled original9035..9290 full rim calls; all original instructions except sound sink; no natural equipment acquisition claim',calls=len(rows),romSHA256=hashlib.sha256(ROM).hexdigest(),baseRAMSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),entry='bank4 9035',exit='bank4 9290',bonusSource='91BD..91E1, indirect equipment pointer94D8 plus slot2')
 (out/'shoes27-score-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
