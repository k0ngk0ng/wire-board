"""Original shared landing entry and complete 8BBD calls, before implementation."""
import gzip,hashlib,itertools,json
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory
from verify_pass_floor_original import MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def main():
 rows=[]
 for actor,action,entered,flag in itertools.product(range(4),[6,22,38,54,70],[False,True],[0,1,128,255]):
  r=bytearray(BASE)
  for addr,value in {0x512+actor:action|(128 if entered else 0),0x554+actor:flag,0x51f+actor:128,0x46b+actor:0,0x4ec+actor:64,0x437+actor:0,0x444+actor:0,0x451+actor:0,0x52c+actor:255,0x5f7+actor:0,0x642+actor:64,0x4e0+actor:0,0x4e4+actor:0,0x58b:1}.items():r[addr]=value
  put(r,200*256+17,0x68+actor,0x75+actor,0x82+actor);put(r,40*256+33,0x8f+actor,0x9c+actor);put(r,0,0xa9+actor,0xb6+actor,0xc3+actor)
  for lo,hi in [(0x478,0x485),(0x492,0x49f),(0x4ac,0x4b9),(0x4c6,0x4d3)]:put(r,0,lo+actor,hi+actor)
  m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=0x8bbd);cpu.x=actor;cpu.sp=255;cpu.stPushWord(0x6fff)
  mid=None
  for step in range(20000):
   if cpu.pc==0x8bda:mid=bytes(m[:2048])
   if cpu.pc==0x7000:break
   cpu.step()
  else:raise RuntimeError(hex(cpu.pc))
  assert mid is not None
  rows.append(dict(actor=actor,action=action,entered=entered,beforeFlag=flag,preparedFlag=mid[0x554+actor],afterFlag=m[0x554+actor],before=decode_world_action_state(r),after=decode_world_action_state(bytes(m[:2048])),context=context(r)['actions']))
 out=ROOT/'games/basketball/engine/testdata';payload=(json.dumps(rows,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);(out/'landing-special-clear.json.gz').write_bytes(packed)
 before=(ROOT/'.local/basketball/rea/special-shot-clear/runtime-before.json').read_bytes()
 assert hashlib.sha256(before).hexdigest()=='f1dfb112e822033c27680fc9ab5e36f9bd5fc2390f334c9b995569ddd7b7eba9'
 (out/'landing-special-runtime-before.json.gz').write_bytes(gzip.compress(before,mtime=0))
 source=(ROOT/'.local/nes-basket/runs/ball-rest-team5-aligned/world-after.ram').read_bytes()
 assert hashlib.sha256(source).hexdigest()=='93fd388aab383aa7a82c7bff46981266027a080cc26b8cf2146297765d9dc8a5'
 expected=[decode_world_action_state(source[i*2048:(i+1)*2048]) for i in range(1013,1073)]
 (out/'landing-special-runtime-worlds.json.gz').write_bytes(gzip.compress(json.dumps(expected,separators=(',',':')).encode(),mtime=0))
 (out/'landing-special-clear-source.json').write_text(json.dumps(dict(runtimeBeforeSHA256=hashlib.sha256(before).hexdigest(),runtimeWorlds=dict(sourceSHA256=hashlib.sha256(source).hexdigest(),firstIndex=1013,count=60,scope='one controlled semantic origin followed by unmodified original whole-machine play'),scope='controlled original complete shared landing handler calls; all five dispatcher aliases',records=len(rows),entry='bank6 8BBD',write='8BD7 STA 0554,X',romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),aliases=[6,22,38,54,70]),indent=2)+'\n');print(len(rows))
if __name__=='__main__':main()
