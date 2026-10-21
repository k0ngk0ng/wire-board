"""Original entered dunk after another actor reuses shared target byte056E.
No patched original instructions; complete E165..E168 includes source motion.
Entry with a non-hoop target is intentionally not asserted as legal.
"""
import gzip,hashlib,itertools,json
from pathlib import Path
from verify_dunk_variants import ROOT,ROM,setup,call

def main():
 rows=[];chains=[]
 for actor,action,target,cursor in itertools.product(range(4),[0x58,0x59,0x5b],range(7),range(5)):
  cpu=setup(actor,action,7,False);m=cpu.memory
  m[0x512+actor]|=128;m[0x56e]=target;m[0x444+actor]=cursor
  m[0x437+actor]=[2,1,254,254,254][cursor] if action==0x59 else [4,5,254,254,254][cursor]
  m[0x451+actor]=0
  rows.append(call(cpu,actor))
 for actor,action,target in itertools.product(range(4),[0x58,0x59,0x5b],range(4)):
  cpu=setup(actor,action,7,False);first=call(cpu,actor)
  # Explicit source boundary: another action may overwrite056E after entry.
  cpu.memory[0x56e]=target;chain=[]
  for tick in range(160):
   chain.append(call(cpu,actor))
   if cpu.memory[0x512+actor]&127 not in (0x58,0x59,0x5b):break
  else:raise AssertionError('source chain did not land')
  chains.append(chain)
 payload=gzip.compress(json.dumps(dict(calls=rows,chains=chains),separators=(',',':')).encode(),mtime=0)
 out=ROOT/'games/basketball/engine/testdata';(out/'dunk-shared-target.json.gz').write_bytes(payload)
 source=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),calls=len(rows),chains=len(chains),chainCalls=sum(map(len,chains)),entry='fixed E165..E168 / bank6 8E9D',scope=__doc__,fixtureSHA256=hashlib.sha256(payload).hexdigest(),skippedCalls=['FBCC sound sink only'],targetAliases='0577+target0..2 = actor1..3 jumpPhase; target3..6 = actor0..3 attackVariant')
 (out/'dunk-shared-target-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))
if __name__=='__main__':main()
