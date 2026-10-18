"""Unchanged original C033 actors and declared isolated command-handler probes.

Controlled states are not naturally earned trajectories. Full actor calls retain
initialization, script, motion and C00C projection; only C000 audio is intercepted.
Direct handler probes declare RAM operand data and stop before next script fetch.
"""
import argparse,base64,gzip,hashlib,json,subprocess
from pathlib import Path
from verify_pass_floor_original import Memory,MPU
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'games/basketball/engine/testdata'
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088];BANK=PRG[3*16384:4*16384]
SHA='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
assert hashlib.sha256(ROM).hexdigest()==SHA
LENGTH={0xf0:4,0xf1:6,0xf2:4,0xf3:4,0xf4:2,0xf5:2,0xf6:2,0xf7:4,0xf8:2,0xf9:4,0xfa:4,0xfb:4,0xfc:2,0xfd:2,0xfe:4,0xff:4}
NAMES=['Position','Velocity','Height','HeightVelocity','Character','Text','Sound','Palettes','Action','OtherAction','Repeat','Camera','Phase','DisplayMask','OtherAction','OtherAction']
def word(a):return int.from_bytes(BANK[a-0x8000:a-0x8000+2],'little')
def program(action):
 start=word(0x97c2+2*action);pc=start;commands=[]
 for _ in range(256):
  op=BANK[pc-0x8000];n=LENGTH.get(op,2);data=list(BANK[pc-0x8000:pc-0x8000+n]);commands.append(dict(address=pc,slot=(pc-start)//2,data=data))
  pc+=n
  if op==0xf8:return dict(action=action,address=start,commands=commands)
 raise RuntimeError(('script has no action terminator',action))
ACTIONS={0,*range(12,17),*range(0x51,0x66),11,18,20,36}
while True:
 added={c['data'][1 if c['data'][0]==0xf8 else 2]&127 for a in ACTIONS for c in program(a)['commands']if c['data'][0]in(0xf8,0xf9,0xfe,0xff)}-ACTIONS
 if not added:break
 ACTIONS|=added
PROGRAMS={a:program(a)for a in sorted(ACTIONS)}
BASE=bytearray(gzip.decompress((OUT/'match-result.ram.gz').read_bytes())[:2048])
def put(r,v,*addresses):
 for i,a in enumerate(addresses):r[a]=(v>>(8*i))&255

def seed(actor,action,slot,mirror,repeat=0,elapsed=0):
 r=bytearray(BASE);r[0x588]=1 if action>=0x60 else 7;r[0x589]=0x87 if action>=0x60 else 0x83
 r[0x41c]=0;r[0x41d:0x42a]=bytes([255]*13)
 r[0x397:0x39f]=bytes([128,0,0,0,128,0,0,0]);r[0x6ab]=0x35;r[0x6ad]=7;r[0x305:0x307]=bytes([3,4]);r[0x304]=0x1e
 for i in range(4):
  r[0x512+i]=0x80;r[0x5b+i]=127;r[0x437+i]=0;r[0x444+i]=0;r[0x6ca+i]=0
  put(r,(100+i*8)*256+91,0x68+i,0x75+i,0x82+i);put(r,(50+i)*256+113,0x8f+i,0x9c+i);put(r,37,0xa9+i,0xb6+i,0xc3+i)
  put(r,-257,0x478+i,0x485+i);put(r,73,0x492+i,0x49f+i);put(r,55,0x4ac+i,0x4b9+i);put(r,9,0x4c6+i,0x4d3+i)
 r[0x512+actor]=0x80|action;r[0x5b+actor]=0x80 if mirror else 0;r[0x444+actor]=slot;r[0x437+actor]=elapsed;r[0x6ca+actor]=repeat
 return r

def call(raw,actor,entry=0x9598,stop=0x7000):
 m=Memory(PRG,raw);m[0x8000:0xc000]=BANK;c=MPU(memory=m,pc=entry);c.x=actor;c.sp=255;c.p=0x20;c.stPushWord(0x6fff);sounds=[];commands=[]
 for n in range(20000):
  if c.pc==stop:return bytes(m[:2048]),dict(sounds=sounds,commands=commands,instructions=n)
  if c.pc==0x95ed:commands.append(m[0x28]+256*m[0x29]+c.y)
  if c.pc==0xc000:
   sounds.append(c.a);c.pc=(c.stPopWord()+1)&65535
  else:c.step()
 raise RuntimeError(('call limit',hex(c.pc),actor,entry))

def record(label,r,actor,**kwargs):
 after,details=call(r,actor,**kwargs)
 return dict(label=label,actor=actor,before=base64.b64encode(r).decode(),after=base64.b64encode(after).decode(),**details)

def rows():
 records=[]
 # Put the previously unsupported reward continuation first for a meaningful RED.
 ordered=[0x62,0x63,0x64]+[a for a in PROGRAMS if a not in(0x62,0x63,0x64)]
 for action in ordered:
  # Cross-actor scripts retain their original ending/reward actor roles.
  # Arbitrary actor reassignment can self-target F9 and execute operand slots;
  # that synthetic choreography is outside the supported scene domain.
  for cmd in PROGRAMS[action]['commands']:
   for mirror in [False,True]:
    actor={**{a:2 for a in range(12,17)},0x51:0,0x52:1,0x53:2,0x54:3,0x55:0,0x56:1,0x57:2,0x58:3,0x5c:0,0x5d:1,0x5e:2,0x5f:0,0x62:0,0x63:1,0x64:2}.get(action,0)
    r=seed(actor,action,cmd['slot'],mirror)
    records.append(record(f'action-{action:02x}-slot-{cmd["slot"]}-mirror-{int(mirror)}',r,actor))
    if cmd['data'][0]==0xfa:
     for rep in [0x80,0xff]:
      r=seed(actor,action,cmd['slot'],mirror,rep);records.append(record(f'repeat-{action:02x}-{cmd["slot"]}-{int(mirror)}-{rep}',r,actor))
  for elapsed in [1,255]:
   r=seed(0,action,0,False,0x91,elapsed);records.append(record(f'countdown-{action:02x}-{elapsed}',r,0))
  r=seed(0,action,7,True,0xa1,77);r[0x512]=action;records.append(record(f'entry-{action:02x}',r,0))
 # Original per-tick actor order, retaining cross-actor effects and draw insertions.
 for actions in [(0x62,0x63,0x64,0),(0x55,0x56,0x57,0),(0x5c,0x5d,0x5e,0)]:
  r=seed(0,actions[0],0,False)
  for i,a in enumerate(actions):r[0x512+i]=a
  for tick in range(300 if actions[0]==0x62 else 1800):
   r[0x41c]=0;r[0x41d:0x42a]=bytes([255]*13)
   for actor in range(3 if actions[0]==0x62 else 4):
    before=bytes(r);after,info=call(r,actor);r=bytearray(after)
    # Store every command-bearing call, first call, and countdown edge. All calls
    # still execute to obtain a continuous source-only actor trajectory.
    if info['commands'] or before[0x437+actor]<=1 or before[0x512+actor]<128:
     records.append(dict(label=f'chain-{actions[0]:02x}-{tick}-{actor}',actor=actor,before=base64.b64encode(before).decode(),after=base64.b64encode(after).decode(),**info))
 return records

def handlers():
 args={0xf0:[0xff,0xfe,0xf1],0xf1:[0xff,0x80,0x80,0xff,0],0xf2:[0xfe,0xfc,0],0xf3:[0xff,0xf8,0xf0],0xf4:[3],0xf5:[36],0xf6:[8],0xf7:[31,47,0],0xf8:[0x60],0xf9:[3,0x58,0],0xfa:[0,0xfe,0],0xfb:[0xff,0xfc,0],0xfc:[0x89],0xfd:[0xe1],0xfe:[2,0x0c,0],0xff:[1,0x63,0]}
 records=[]
 for op,operands in args.items():
  for variant in range(4):
   r=seed(0,0x62,1,False,[0,0x80,0xfe,0xff][variant]);r[0x28:0x2a]=bytes([0x80,1]);r[0x18]=0
   r[0x180:0x180+1+len(operands)]=bytes([op,*operands]);r[0x39b:0x39f]=bytes([255,255,[0,1,0xf0,0xff][variant],1])
   row=record(f'handler-{op:02x}-{variant}',r,0,entry=word(0x95f4+2*(op&15)),stop=0x95bf if op==0xf8 else 0x95d4)
   row.update(opcode=op,operands=operands);records.append(row)
 return records

def emit_tables(check=False):
 lines=['// Code generated by scripts/basketball/verify_result_actor_extension.py --emit-tables; DO NOT EDIT.','// Bank 3 original menu/result scripts, decoded into semantic commands.','package engine','','var originalResultActorPrograms = map[uint8]map[uint8]resultActorCommand{']
 for a,p in PROGRAMS.items():
  lines.append(f'\t0x{a:02x}: {{ // source {p["address"]:04X}')
  for c in p['commands']:
   op=c['data'][0];kind='resultPose'if op<0xf0 else'result'+NAMES[op-0xf0]
   data=c['data']if op<0xf0 else c['data'][1:]
   lines.append(f'\t\t{c["slot"]}: {{Kind: {kind}, Args: [5]uint8{{'+', '.join(f'0x{x:02x}'for x in data)+'}},')
  lines.append('\t},')
 lines.append('}')
 data=subprocess.check_output(['gofmt'],input=('\n'.join(lines)+'\n').encode())
 path=ROOT/'games/basketball/engine/result_actor_tables.go'
 if check:assert path.read_bytes()==data,'result actor semantic tables changed'
 else:path.write_bytes(data)

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');p.add_argument('--emit-tables',action='store_true');a=p.parse_args()
 if a.emit_tables:emit_tables(a.check);return
 payload=dict(actors=rows(),handlers=handlers());data=gzip.compress((json.dumps(payload,separators=(',',':'))+'\n').encode(),mtime=0);path=OUT/'result-actor-extension.json.gz'
 if a.check:assert path.read_bytes()==data,'original actor fixtures changed'
 else:path.write_bytes(data)
 meta=dict(romSHA256=SHA,actorCalls=len(payload['actors']),handlerCalls=len(payload['handlers']),actions=sorted(PROGRAMS),fixtureSHA256=hashlib.sha256(data).hexdigest(),scope='controlled source C033 complete actor calls and declared isolated original handler calls; not natural menu/ending histories',actorCall=dict(entry='bank3 9598',stop='RTS sentinel7000',omitted={'C000':'audio driver request only; ordered IDs retained'},projection='C00C/CD3C/D0D9 executed unchanged; no projection bypass'),handlerCall=dict(operands='declared RAM0180 operand bytes, original ROM instructions unchanged',cursor='already incremented one slot by original dispatcher',stops='95D4 before next fetch, or95BF after self-action change'),sourceREA=['ev_f0ce6b1dbcabba1133dc39a0adf0773ec895a3f8489b27b31893fe9bf045a4e8','ev_93cedbb127d764b97aff2143de421dba36068fe4f8fb1f7be5cfbe0251cbfe78'])
 if not a.check:(OUT/'result-actor-extension-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
