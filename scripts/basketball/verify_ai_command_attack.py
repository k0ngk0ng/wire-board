"""Original A05D -> A63D/AD17 controlled calls and uninterrupted AI chains."""
import gzip,hashlib,itertools,json
from verify_ai_mode5_plan11 import ROOT,ROM,PRG,SOURCE,BASE,start,machine,record
from verify_pass_floor_original import put

def setup(actor,mode,entered,distances=(10,20),depth=0,actions=(0,0),seed=0x12345678):
 r=start(actor);r[0x672+actor]=mode|(128 if entered else 0);r[0x48:0x4c]=seed.to_bytes(4,'big')
 r[0x66e+actor]=17;r[0x676+actor]=9;r[0x682+actor]=5;r[0x67e+actor]=48
 base=(actor&2)^2;r[0x67a+actor]=base
 put(r,200*256+113,0x68+actor,0x75+actor,0x82+actor)
 for i,dx in enumerate(distances):
  a=base+i;put(r,(200+dx)*256+29,0x68+a,0x75+a,0x82+a);put(r,(40+depth)*256+71,0x8f+a,0x9c+a);r[0x512+a]=actions[i]|128
 return r

def main():
 calls=[]
 pairs=[(0,0),(-1,1),(-2,1),(255,256),(511,0),(-32768,0),(32767,-32768)]
 for actor,entered,distances,actions in itertools.product(range(4),[False,True],pairs,itertools.product([0,0x49,0x4a],repeat=2)):
  r=setup(actor,41,entered,distances,actions=actions)
  calls.append(record(machine(r),actor,f'select-{actor}-{entered}-{distances}-{actions}'))
 for mode,actor,entered,dx,dz,seed in itertools.product([36,37,38],range(4),[False,True],[-33,-32,-31,-16,-15,0,15,16,31,32,33,256],[0,7,8,-7,-8,127,-128],[0,0x12345678,0xffffffff]):
  r=setup(actor,mode,entered,(dx,500),depth=dz,seed=seed)
  calls.append(record(machine(r),actor,f'attack-{mode}-{actor}-{entered}-{dx}-{dz}-{seed}'))
 chains=[]
 for mode,actor,seed in itertools.product([36,37,38],range(4),[0,0x12345678,0xffffffff]):
  c=machine(setup(actor,mode,False,(0,100),seed=seed));rows=[record(c,actor,'select-command-attack'),record(c,actor,'consume-engagement')]
  chains.append(dict(mode=mode,actor=actor,seed=seed,records=rows))
 out=ROOT/'games/basketball/engine/testdata'
 p=out/'ai-command-attack.json.gz';p.write_bytes(gzip.compress((json.dumps(dict(calls=calls,chains=chains),separators=(',',':'))+'\n').encode(),mtime=0))
 meta=dict(entry='bank5 original A05D dispatcher, A63D/AD17 and subsequent mode7 A2E9',calls=len(calls),chains=len(chains),chainCalls=sum(len(c['records']) for c in chains),natural_gameplay=False,original_instructions_unchanged=True,scope='controlled dispatch calls and two-call AI handoff chains; no physics or full-match claim',romSHA256=hashlib.sha256(ROM).hexdigest(),ramSource=str(SOURCE.relative_to(ROOT)),ramSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(p.read_bytes()).hexdigest(),reusedREA='ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d')
 (out/'ai-command-attack-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
