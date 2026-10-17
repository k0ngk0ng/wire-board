"""Source09 carried-body 9C4E paired-motion calls; test before world wiring."""
import gzip,hashlib,itertools,json
from verify_attack_handlers import ROOT,ROM,PRG,start,machine
from verify_pass_floor_original import put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context
rows=[]
for noLink,action,actor,support,court,entered,negative,gravity in itertools.product((False,True),(9,11),range(4),range(6),(0,1,2),(False,True),(False,True),(0,113)):
 if support==actor or (noLink and action==9):continue
 r=start(actor);r[0x3da:0x3dc]=bytes([(10,11),(12,11),(12,13)][court]);r[0x58b]=court;r[0x569]=actor;r[0x642+actor]=128|support;r[0x642+support]=64
 r[0x512+actor]=action|(128 if entered else 0);r[0x51f+actor]=128;r[0x550+actor]=255 if noLink else actor^1
 r[0x437+actor]=3;r[0x444+actor]=0;r[0x451+actor]=0;r[0x52c+actor]=11 if action==9 else 61
 r[0x4ec+actor]=192 if negative else 64;r[0x6d7:0x6e4]=bytes(13)
 for i in [actor,support]:
  put(r,240*256+37,0x68+i,0x75+i,0x82+i);put(r,40*256+29,0x8f+i,0x9c+i)
  put(r,(64 if i==actor else 32)*256+31,0xa9+i,0xb6+i,0xc3+i)
  r[0x5f7+i]=128
 put(r,-517 if negative else 517,0x478+actor,0x485+actor);put(r,-127 if negative else 127,0x492+actor,0x49f+actor)
 put(r,513,0x4ac+actor,0x4b9+actor);put(r,gravity,0x4c6+actor,0x4d3+actor)
 cpu=machine(r);cpu.pc=0xe165;cpu.x=actor;before=bytes(cpu.memory[:2048]);sounds=[]
 for steps in range(20000):
  if cpu.pc==0xe168:break
  if cpu.pc==0xfbcc:sounds.append(cpu.a);cpu.pc=cpu.stPopWord()+1
  else:cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 rows.append(dict(actor=actor,before=decode_world_action_state(before),after=decode_world_action_state(bytes(cpu.memory[:2048])),context=context(before)['actions'],sounds=sounds))
p=ROOT/'games/basketball/engine/testdata/carried-assist.json.gz';b=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0);p.write_bytes(b)
p.with_name('carried-assist-source.json').write_text(json.dumps(dict(records=len(rows),entry='E165/E18B ->9852 ->9C4E ->9DCF/EEFF/EEFF/9E06',romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(b).hexdigest(),scope='isolated source09 carried-body and attached0B world calls; supports0..5 excluding self; courts0/1/2; not natural full-game trace'),indent=2)+'\n');print(len(rows))
