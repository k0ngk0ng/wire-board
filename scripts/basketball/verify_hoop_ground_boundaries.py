"""Original 30/32/39/3B boundary calls plus airborne complete action stages."""
import gzip,hashlib,itertools,json
from verify_hoop_air_throw import ROOT,ROM,run,world,put,context,encode_world_probe

def main():
 origin=ROOT/'games/basketball/engine/testdata/hoop-air-throw-runtime.json.gz';snapshot=json.loads(gzip.decompress(origin.read_bytes()))
 raw=(ROOT/'.local/nes-basket/runs/court-4-reference/ram.bin').read_bytes()[1500*2048:1501*2048]
 seed=bytearray(raw);patch,_=encode_world_probe(snapshot)
 for a,v in patch.items():seed[a]=v
 seed[:8]=bytes(8);base,_,_,_,_=run(seed,0,0xd314,0xd345)
 calls=[];stages=[]
 def build(action,entered,air,court,left,computer):
  r=bytearray(base)
  for a,v in {0x512:action|(128 if entered else 0),0x51f:128,0x437:0,0x444:0,0x451:0,0x52c:255,0x46b:128 if left else 0,0x4ec:192 if left else 64,0x5f7:128 if air else 0,0x642:0,0x539:128 if computer else 0,0x569:255,0x58b:court,0x4e0:0}.items():r[a]=v
  put(r,(2 if left else 510)*256+73,0x68,0x75,0x82);put(r,45*256+19,0x8f,0x9c);put(r,60*256 if air else 0,0xa9,0xb6,0xc3)
  put(r,-768 if left else 768,0x478,0x485);put(r,17,0x492,0x49f);put(r,0,0x4ac,0x4b9);put(r,256 if air else 0,0x4c6,0x4d3)
  r[0x540:0x546]=bytes([15]*6);r[0x545]=0
  return r
 for args in itertools.product([0x30,0x32,0x39,0x3b],[False,True],[False,True],[1,4],[False,True],[False,True]):
  r=build(*args);final,_,sounds,_,_=run(r,0)
  calls.append(dict(before=world(r),after=world(final),context=context(r)['actions'],sounds=sounds))
 for action,court in itertools.product([0x30,0x32,0x39,0x3b],[1,4]):
  r=build(action,False,True,court,False,False);put(r,300*256+73,0x68,0x75,0x82)
  final,order,sounds,_,_=run(r,0,0xd345,0xd348)
  stages.append(dict(before=world(r),after=world(final),context=context(r)['actions'],sounds=sounds,calls=order))
 out=ROOT/'games/basketball/engine/testdata';payload=gzip.compress((json.dumps(dict(calls=calls,stages=stages),separators=(',',':'))+'\n').encode(),mtime=0);(out/'hoop-ground-boundaries.json.gz').write_bytes(payload)
 (out/'hoop-ground-boundaries-source.json').write_text(json.dumps(dict(romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(payload).hexdigest(),calls=len(calls),wholeStages=len(stages),scope='controlled original unmodified handlers30/32/39/3B, ground/air and left/right edge with human/computer, Court1 and4; complete airborne action stages',naturalGameplay=False),indent=2)+'\n')
 print('ground boundary original calls',len(calls),'stages',len(stages))
if __name__=='__main__':main()
