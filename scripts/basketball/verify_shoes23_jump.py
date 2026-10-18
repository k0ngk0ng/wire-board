"""Complete unchanged-ROM E165 jump calls; failure inventory in SHOES23_JUMP_VERIFICATION.md."""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
BASE=(ROOT/'.local/nes-basket/runs/ball-pass/ram.bin').read_bytes()[:2048]
assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
ACTIONS=[4,5,13,20,21,36,37,52,53,68,69]
def initial(action=4,item=23,mode=0,team=0,char=0,actor=0,phase=0,stage=0,court=1,heading=64):
 r=bytearray(BASE);r[0x58b]=court;r[0x58d]=mode;r[0x586+actor//2]=team;r[0x3e3+actor]=char
 r[0x6e4:0x6f0]=bytes([255]*12);r[0x6e6+3*char]=item;r[0x540:0x546]=bytes(range(7,13));r[0x569]=255
 r[0x3da:0x3dc]=bytes([12,11]);r[0x53d:0x53f]=bytes([255,255]);r[0x53f]=actor if action in [20,21,68,69] else 6
 r[0x539+actor]=0;r[0x512+actor]=action|(128 if stage else 0);r[0x51f+actor]=128;r[0x52c+actor]=255
 r[0x576+actor]=phase&255;r[0x437+actor]=1 if stage==2 else 0;r[0x444+actor]=1 if stage>=2 else 0;r[0x451+actor]=0
 r[0x642+actor]=128|(actor^1) if action in [68,69] else 64;r[0x4ec+actor]=heading;r[0x46b+actor]=0
 r[0x4e0+actor]=0;r[0x4e4+actor]=0;r[0x5f7+actor]=128 if stage>=2 else 0
 if action in [36,37]:r[0x53d]=actor
 if action in [52,53]:r[0x540]=actor
 if action in [68,69]:r[0x569]=actor;r[0x642+(actor^1)]=64;r[0x5f7+(actor^1)]=128 if stage>=2 else 0
 put(r,200*256+17,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+33,0x8f+actor,0x9c+actor);put(r,40*256+19 if stage>=2 else 0,0xa9+actor,0xb6+actor,0xc3+actor)
 for lo,hi,value in [(0x478,0x485,513),(0x492,0x49f,33),(0x4ac,0x4b9,1024 if stage>=2 else 0),(0x4c6,0x4d3,173 if stage>=2 else 0)]:put(r,value,lo+actor,hi+actor)
 return r
def call(m,actor,label):
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe165);cpu.x=actor;sounds=[];shoe=[];motion=[]
 for count in range(30000):
  if cpu.pc==0xe168:break
  if cpu.pc==0xec31:shoe.append(cpu.x)
  if cpu.pc==0xeeff:motion.append(cpu.x)
  if cpu.pc==0xfbcc:sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1
  else:cpu.step()
 else:raise RuntimeError((label,hex(cpu.pc)))
 return dict(label=label,actor=actor,before=decode_world_action_state(before),after=decode_world_action_state(bytes(m[:2048])),context=context(before)['actions'],shoes=[before[0x6e6+3*before[0x3e3+i]] for i in range(4)],sounds=sounds,shoeCalls=shoe,motionObjects=motion)
def main():
 rows=[]
 for action,item,mode,team,char,stage,phase in itertools.product(ACTIONS,[23,255],[0,128],[0,1],[0,3],[0,1,2,3],[0,-32,-1]):
  if stage and phase:continue
  r=initial(action,item,mode,team,char,stage=stage,phase=phase)
  rows.append(call(Memory(PRG,r),0,f'{action:02x}-{item}-{mode}-{team}-{char}-{stage}-{phase}'))
 for action,actor,char,heading,computer in itertools.product(ACTIONS,range(4),range(4),[0,192,255],[False,True]):
  r=initial(action,23,0,0,char,actor,phase=-1,heading=heading);r[0x539+actor]=128 if computer else 0;rows.append(call(Memory(PRG,r),actor,f'rotation-{action:02x}-{actor}-{char}-{heading}-{computer}'))
 chains=[]
 for action,item,court,phase in itertools.product(ACTIONS,[23,255],[0,1,5],[0,-32]):
  m=Memory(PRG,initial(action,item,court=court,phase=phase));chain=[]
  for tick in range(20):
   if m[0x512]&127 not in ACTIONS:break
   chain.append(call(m,0,f'chain-{action:02x}-{item}-{court}-{phase}-{tick}'))
  chains.append(chain)
 dest=ROOT/'games/basketball/engine/testdata';raw=gzip.compress(json.dumps(dict(calls=rows,chains=chains),separators=(',',':')).encode(),mtime=0)
 (dest/'shoes23-jump.json.gz').write_bytes(raw)
 meta=dict(scope='controlled complete original E165..E168 calls and once-initialized continuous chains; FBCC sound sink only; no natural-gameplay claim',romSHA256=hashlib.sha256(ROM).hexdigest(),baseRAMSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(raw).hexdigest(),calls=len(rows),chains=len(chains),chainCalls=sum(map(len,chains)),actions=ACTIONS)
 (dest/'shoes23-jump-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
