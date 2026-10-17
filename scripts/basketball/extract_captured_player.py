"""Controlled original 50..56 calls and a continuous original computer escape chain."""
import gzip,hashlib,itertools,json
from verify_pass_floor_original import ROOT,Memory,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072]
base=(ROOT/'.local/nes-basket/runs/world-actions-cpu-long/world-actions-ram.bin').read_bytes()[:2048]
def setup(action,entered,computer,held,counter,variant):
 r=bytearray(base);actor=1
 r[0x513]=action|(128 if entered else 0);r[0x520]=128;r[0x53a]=128 if computer else 0
 r[0x643]=64;r[0x56b]=3;r[0x56c]=counter;r[0x53f]=actor if held else 6
 r[0x518]=0x82;r[0x525]=128;r[0x516]=6;r[0x523]=128;r[0x53d]=255;r[0x3da]=10
 r[0x438]=7 if variant=='animation-end' and action in (80,83) else 3 if variant=='animation-end' and action==86 else 0
 r[0x445]=0;r[0x452]=0;r[0x52d]=0xff;r[0x4ed]=255 if variant=='still' else 64
 r[0x46c]=0;r[0x5c]=128 if variant=='pose-left' else 7 if variant=='pose-seven' else 0;r[0x573]=1 if variant=='charge' else 8
 put(r,200*256+29,0x69,0x76,0x83);put(r,40*256+31,0x90,0x9d);put(r,0 if variant=='still' else 256+41,0xaa,0xb7,0xc4)
 put(r,0 if variant=='still' else 1280+33,0x479,0x486);put(r,0 if variant=='still' else 257,0x493,0x4a0)
 put(r,0 if variant=='still' else -256,0x4ad,0x4ba);put(r,0 if variant=='still' else 128,0x4c7,0x4d4);r[0x5f8]=0;r[0x6d8]=0
 return r

def run(m,actor):
 before=bytes(m[:2048]);cpu=MPU(memory=m,pc=0xe165);cpu.x=actor;sounds=[]
 for n in range(20000):
  if cpu.pc==0xe168:break
  if cpu.pc==0xfbcc:
   sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1;continue
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return dict(actor=actor,before=decode_world_action_state(before),after=decode_world_action_state(bytes(m[:2048])),context=context(before)['actions'],sounds=sounds)
rows=[]
for action,entered,computer,held,counter,variant in itertools.product(range(80,87),(False,True),(False,True),(False,True),(0,4),('still','pose-left','pose-seven','animation-end','charge')):
 m=Memory(prg,setup(action,entered,computer,held,counter,variant));rows.append(dict(action=action,variant=variant,**run(m,1)))
# A real original instruction chain, initialized once: actor then its container.
m=Memory(prg,setup(80,False,True,False,4,'still'));m[0x4ed]=255;m[0x56b]=255
chain=[]
for tick in range(220):
 player=run(m,1);aux=run(m,4);chain.append(dict(player=player,auxiliary=aux))
 if m[0x56b]==255 and (m[0x513]&127)==6 and (m[0x516]&127)==7:break
else:raise RuntimeError('computer capture chain did not escape')
requests=[]
for a,entered in itertools.product(range(25),(False,True)):
 m=Memory(prg,base);m[0x518]=a|(128 if entered else 0);m[0x525]=11;before=decode_world_action_state(bytes(m[:2048]))['ball'];cpu=MPU(memory=m,pc=0xe18b);cpu.x=6
 for n in range(1000):
  if cpu.pc==0xe1ed:break
  cpu.step()
 else:raise RuntimeError('ball request did not reach dispatch')
 requests.append(dict(before=before,after=decode_world_action_state(bytes(m[:2048]))['ball']))
falls=[]
for entered,variant in itertools.product((False,True),('still','pose-left')):
 m=Memory(prg,setup(15,entered,True,False,4,variant));m[0x56b]=1
 falls.append(run(m,1))
# Bound-motion regression: keep the old below-surface setup, which fires the
# device immediately, and separately jump from the actual 32px surface. These
# are controlled cross-court states, not naturally spawned Court2 containers.
devices=[]
for height in (0,32):
 m=Memory(prg,setup(80,False,True,False,4,'still'));m[0x4ed]=255
 run(m,1)
 m[0x58b]=2
 put(m,280*256,0x69,0x76,0x83);put(m,76*256,0x90,0x9d);put(m,height*256,0xaa,0xb7,0xc4)
 m[0x520]=84;m[0x577]=37
 calls=[]
 for tick in range(40):
  calls.append(run(m,1))
  if height==0 or (m[0x5f8]&128)==0:break
 else:raise RuntimeError('captured jump from device surface did not land')
 assert m[0x577]==0
 devices.append(dict(height=height,calls=calls))
p=ROOT/'games/basketball/engine/testdata/captured-player.json.gz';packed=gzip.compress((json.dumps(dict(calls=rows,chain=chain,requests=requests,falls=falls,devices=devices),separators=(',',':'))+'\n').encode(),mtime=0);p.write_bytes(packed)
p.with_name('captured-player-source.json').write_text(json.dumps(dict(scope='controlled original instructions; FBCC sound request sink; one-initial-state computer escape chain; cross-court device adapter cases, no natural match claim',calls=len(rows),escapeUpdates=len(chain),requests=len(requests),falls=len(falls),deviceCalls=sum(len(d['calls']) for d in devices),romSHA256=hashlib.sha256(rom).hexdigest(),baseRAMSHA256=hashlib.sha256(base).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest()),indent=2)+'\n');print(len(rows),'calls',len(chain),'escape updates',len(requests),'request cases',sum(len(d['calls']) for d in devices),'device calls')
