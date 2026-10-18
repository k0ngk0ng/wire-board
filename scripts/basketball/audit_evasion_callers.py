"""Bounded literal-store/alias inventory, source tables and observed callers."""
import hashlib,json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
def main():
 rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();prg=rom[16:16+131072];out=ROOT/'.local/basketball/rea/evasion-direct';direct=[];adjacent=[];modeWrites=[];calls=[]
 for bank in range(8):
  b=prg[bank*16384:(bank+1)*16384];start=0xc000 if bank==7 else 0x8000
  for i in range(len(b)-2):
   opcode=b[i];base=b[i+1]+256*b[i+2]
   if opcode in [0x8d,0x9d,0x99,0x8c,0x8e,0xee,0xfe,0xce,0xde] and base in [0x672,0x676,0x67a,0x67e,0x682,0x683,0x684,0x685]:
    entry=dict(bank=bank,address=hex(start+i),opcode=hex(opcode),base=hex(base),index=('X' if opcode in [0x9d,0xfe,0xde] else 'Y' if opcode==0x99 else None))
    if base>=0x682:direct.append(entry)
    else:
     entry['indicesToParameterBytes']=[hex(v-base) for v in range(0x682,0x686)];adjacent.append(entry)
   if opcode in [0x20,0x4c] and base==0xb7cd:calls.append(dict(bank=bank,address=hex(start+i),opcode=hex(opcode)))
   if i+4<len(b) and b[i]==0xa9 and b[i+1] in [26,154] and b[i+2] in [0x8d,0x9d,0x99] and b[i+3:i+5]==bytes([0x72,6]):modeWrites.append(dict(bank=bank,address=hex(start+i),value=b[i+1]))
 b=prg[5*16384:6*16384];word=lambda a:b[a-0x8000]+256*b[a-0x8000+1]
 table=[list(b[word(0xba5c+2*i)-0x8000:word(0xba5c+2*i)-0x8000+12]) for i in range(8)]
 observed=json.loads((out/'caller-capture.json').read_text())
 result=dict(scope='literal instruction-byte candidates across 8 PRG banks plus 18000-frame unchanged reference; candidates are not all proven code',romSHA256=hashlib.sha256(rom).hexdigest(),directParameterStoreCandidates=direct,adjacentBaseStoreCandidates=adjacent,literalB7CDCallCandidates=calls,literalMode26WriteCandidates=modeWrites,sourceRandomChoices=table,sourceRandomChoiceSet=sorted(set(v for row in table for v in row)),observedWrites=observed['writes'],observedEvasionPlans=sorted(set(c['parameter'] for c in observed['evasionCalls'])),observedDirectPlans=observed['remainingPlanCalls'],observedParameterWritesOfRemainingValues=[r for r in observed['writeGroups'] if r['value'] in [6,7,8,9,10,12,13]],limits=['Indexed base references do not prove their runtime index; adjacent aliases remain explicit candidates.','Observed parameter stores in this reference used X1..3; that is not an all-configuration index proof.','Indirect stores, computed control transfers and RAM initialization are not statically exhausted by this literal scan.','Shared parameter values in shot/pursuit/command modes are not evasion26 entry evidence.','No natural entry into the seven restored direct plans was established.'])
 (out/'caller-audit.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({k:v for k,v in result.items() if k not in ['directParameterStoreCandidates','adjacentBaseStoreCandidates','sourceRandomChoices','observedParameterWritesOfRemainingValues']}))
if __name__=='__main__':main()
