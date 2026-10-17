"""Original full injury49 handler, including support writes before motion."""
import gzip,hashlib,itertools,json
from verify_ball18 import ROOT,ROM,PRG,BASE,SoundMemory,MPU,put
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def initial(actor,entered,support,held,assisted):
 r=bytearray(BASE)
 parent=actor^1 if support=='player' else 4
 for addr,value in {0x512+actor:0x49|(128 if entered else 0),0x51f+actor:128,0x46b+actor:0,0x437+actor:0,0x444+actor:0,0x451+actor:0,0x52c+actor:255,0x5f7+actor:128,0x642+actor:0xc0|parent,0x550+actor:255,0x570:parent if assisted else 255,0x53f:actor if held else 6,0x53d:255,0x53e:255,0x56b:255,0x58b:1}.items():r[addr]=value
 # Full original object state: a grounded, free Kind12 prop; variants change
 # only its actual original flags. The player-support case uses its ROM kind.
 r[0x3da]=12;r[0x646]=128 if support=='attached' else 0;r[0x5fb]=128 if support=='airborne' else 0
 r[0x516]=0x80;r[0x4f0]=255
 if support=='other-kind':r[0x3da]=14
 for i in range(6):r[0x540+i]=7+i
 for addrs,v in [([0x68+actor,0x75+actor,0x82+actor],200*256+17),([0x8f+actor,0x9c+actor],40*256+33),([0xa9+actor,0xb6+actor,0xc3+actor],60*256+11),([0x478+actor,0x485+actor],-641),([0x492+actor,0x49f+actor],73),([0x4ac+actor,0x4b9+actor],-123),([0x4c6+actor,0x4d3+actor],96)]:put(r,v,*addrs)
 return r

def decode(r):
 s=decode_world_action_state(r)
 for i,a in enumerate(s["auxiliary"]):a["kind"]=r[0x3da+i]
 return s

def run(r,actor):
 m=SoundMemory(PRG,r);cpu=MPU(memory=m,pc=0x9673);cpu.x=actor;cpu.sp=255;cpu.stPushWord(0x6fff);prepared=None;beforeMotion=None;sounds=[]
 for _ in range(20000):
  if cpu.pc==0x96f1:prepared=bytes(m[:2048])
  if cpu.pc==0x96f4:beforeMotion=bytes(m[:2048])
  if cpu.pc==0xfbcc:sounds.append(cpu.a)
  if cpu.pc==0x7000:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 assert beforeMotion is not None
 # Entered handler branches to shared animation at9670; it still reaches96f4.
 if prepared is None:prepared=bytes(r)
 return dict(actor=actor,before=decode(r),prepared=decode(prepared),beforeMotion=decode(beforeMotion),after=decode(bytes(m[:2048])),context=context(r)['actions'],sounds=sounds)

def main():
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 rows=[]
 for actor,entered,support,held,assisted in itertools.product(range(4),[False,True],['player','grounded','airborne','attached','other-kind'],[False,True],[False,True]):
  r=initial(actor,entered,support,held,assisted)
  rows.append(dict(label=f'{actor}-{entered}-{support}-{held}-{assisted}',**run(r,actor)))
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);(out/'hurt-rider.json.gz').write_bytes(packed)
 before=(ROOT/'.local/basketball/roster-rotation-special-balls-20261018/case-11/before.json').read_bytes();(out/'hurt-rider-runtime-before.json.gz').write_bytes(gzip.compress(before,mtime=0))
 meta=dict(scope='controlled original9673 full calls with attachment; exact retained native runtime failure separately',records=len(rows),entry='bank6 9673',prepared='96F1',beforeMotion='96F4',romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),runtimeBeforeSHA256=hashlib.sha256(before).hexdigest())
 (out/'hurt-rider-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
