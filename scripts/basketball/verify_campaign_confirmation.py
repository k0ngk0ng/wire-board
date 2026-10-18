"""Selection cleanup and later roster/grade commit from four retained menus.

The two segments execute unchanged original instructions independently. Their
intervening menu/tactics waits are omitted, so their RNG origins stay separate.
"""
import base64,hashlib,json
from pathlib import Path
from verify_pass_floor_original import MPU,Memory

ROOT=Path(__file__).resolve().parents[2]
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
PRG=ROM[16:16+131072]
CASES=[
 ('campaign-audit-first-match-confirm',769,1612),
 ('campaign-audit-coop-first-match',769,1565),
 ('campaign-audit-equipped-home-match',3410,4194),
 ('campaign-audit-loss-continue',11009,11086),
]

class TrackedMemory(Memory):
 def __init__(self,r):
  self.writes=[];self.pc=0;self.track=False
  super().__init__(PRG,r)
  self[0x8000:0xc000]=PRG[3*16384:4*16384]
 def __setitem__(self,address,value):
  if self.track and isinstance(address,int):
   self.writes.append(dict(pc=self.pc,address=address,before=self[address],after=value,space='mapper-bus' if address in (0x8000,0x8001) else 'RAM'))
  super().__setitem__(address,value)

def machine(r):return MPU(memory=TrackedMemory(r))

def execute(r,entry,stop):
 cpu=machine(r);cpu.pc=entry;cpu.sp=255;cpu.memory.track=True
 randomCalls=gradeCalls=0;actorCalls=[]
 for instructions in range(30000):
  if cpu.pc==stop:break
  if cpu.pc==0xc018:randomCalls+=1
  if cpu.pc==0xacbb:gradeCalls+=1
  if cpu.pc==0xc033:actorCalls.append(cpu.x)
  cpu.memory.pc=cpu.pc
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 return bytes(cpu.memory[:2048]),dict(randomCalls=randomCalls,gradeCalls=gradeCalls,actorCalls=actorCalls,instructions=instructions),cpu.memory.writes

def random_state(r):return int.from_bytes(bytes(r[0x48:0x4c]),'big')
def signed(v):return v if v<128 else v-256

def selection(r):
 return dict(mode=r[0x58d],pending=list(r[0x3cb:0x3cf]),objectKind5=r[0x3db],menuScratch=list(r[0x6d7:0x6e0]),random=random_state(r))

def committed(r):
 return dict(characters=list(r[0x3e3:0x3e7]),grades=[dict(run=r[0x615+i],jump=r[0x619+i],shot=r[0x61d+i],recovery=r[0x621+i],attack=r[0x625+i]) for i in range(4)],equipment=[[signed(v) for v in r[0x6e4+3*i:0x6e7+3*i]] for i in range(4)],mode=r[0x58d],teams=list(r[0x586:0x588]),court=r[0x58b],period=r[0x58c],winBits=list(r[0x6d2:0x6d4]),rewardSet=r[0x6d4],inventory=[signed(v) for v in r[0xd3:0xe3]])

def state_write_fields(writes):
 fields={0x588:'scene',0x589:'phase',0x407:'projectedY.ball.lowByte'}
 for i in range(4):
  fields[0x3e3+i]=f'characters[{i}]'
  for address,name in [(0x615,'run'),(0x619,'jump'),(0x61d,'shot'),(0x621,'recovery'),(0x625,'attack')]:fields[address+i]=f'grades[{i}].{name}'
 for i in range(6):
  for address,name in [(0x512,'actionByte'),(0x485,'velocityX.integerByte'),(0x82,'positionX.highByte'),(0x6d7,'courtOrMenuFlag')]:fields[address+i]=f'objects[{i}].{name}'
 for i in range(8):fields[0x6ca+i]=f'006CA..006D1[{i}]'
 result=[]
 for address in sorted(set(w['address'] for w in writes)):
  if address in fields:result.append(dict(address=address,field=fields[address],writes=[w for w in writes if w['address']==address]))
 return result

def frame(raw,n):return raw[(n-1)*2048:n*2048]

def main():
 romSHA=hashlib.sha256(ROM).hexdigest()
 assert romSHA=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 rows=[]
 for name,selectionFrame,commitFrame in CASES:
  source=ROOT/'.local/nes-basket/runs'/name/'ram.bin';raw=source.read_bytes()
  selectionBefore=frame(raw,selectionFrame)
  selectionAfter,selectionCounts,_=execute(selectionBefore,0x896d,0x8088)
  assert selection(selectionAfter)==selection(frame(raw,selectionFrame+1))
  assert selectionAfter[0x589]==frame(raw,selectionFrame+1)[0x589]==31
  assert selectionCounts['randomCalls']==1
  commitBefore=frame(raw,commitFrame)
  commitAfter,commitCounts,commitWrites=execute(commitBefore,0xb2ef,0xb32d)
  tailAfter,tailCounts,tailWrites=execute(commitBefore,0xb2ef,0xb337)
  assert tailCounts['actorCalls']==[0,1,2,3] and tailCounts['randomCalls']==0 and tailCounts['gradeCalls']==1
  observed=frame(raw,commitFrame+1)
  assert committed(commitAfter)==committed(observed)
  tailActorAddresses=[address+i for i in range(4) for address in [0x512,0x5b,0x437,0x444,0x451,0x68,0x75,0x82,0x8f,0x9c,0xa9,0xb6,0xc3,0x478,0x485,0x492,0x49f,0x4ac,0x4b9,0x4c6,0x4d3,0x3e7,0x3f4,0x401,0x40e]]
  assert all(tailAfter[address]==observed[address] for address in tailActorAddresses)
  assert commitAfter[0x3e3:0x3e7]==commitBefore[0x3cb:0x3cf]==selectionAfter[0x3cb:0x3cf]
  assert commitCounts['randomCalls']==0 and commitCounts['gradeCalls']==1
  assert random_state(commitBefore)==random_state(commitAfter)
  rows.append(dict(run=name,sourceSHA256=hashlib.sha256(raw).hexdigest(),selectionBefore=selection(selectionBefore),selectionAfter=selection(selectionAfter),selectionPhaseBefore=selectionBefore[0x589],selectionPhaseAfter=selectionAfter[0x589],selectionBeforeFrame=selectionFrame,selectionObservedAfterFrame=selectionFrame+1,selectionRandomCalls=selectionCounts['randomCalls'],commit=committed(commitAfter),commitTailRawAfter=base64.b64encode(tailAfter).decode(),commitTailStop="bank3:B337 before RTS, after four C033 actor updates",commitTailActorCalls=tailCounts["actorCalls"],commitTailRandomCalls=tailCounts["randomCalls"],commitTailObservedActorBytesMatched=len(tailActorAddresses),commitTailWrites=tailWrites,commitRawBefore=base64.b64encode(commitBefore).decode(),commitRawAfter=base64.b64encode(commitAfter).decode(),commitStop="bank3:B32D before four stateful menu/result actor updates",commitWrites=commitWrites,commitWrittenAddresses=sorted(set(w['address'] for w in commitWrites)),commitStateWriteFields=state_write_fields(commitWrites),commitBeforeFrame=commitFrame,commitObservedAfterFrame=commitFrame+1,commitPending=list(commitBefore[0x3cb:0x3cf]),commitRandomBefore=random_state(commitBefore),commitRandomAfter=random_state(commitAfter),commitObservedAfterRandom=random_state(observed),commitRandomCalls=commitCounts['randomCalls'],commitGradeCalls=commitCounts['gradeCalls'],omittedInterveningVideoFrames=commitFrame-(selectionFrame+1),interveningMenuWaitsAndTacticsOmitted=True))
 out=ROOT/'games/basketball/engine/testdata';payload=(json.dumps(rows,indent=2)+'\n').encode();(out/'campaign-confirmation.json').write_bytes(payload)
 metadata=dict(records=len(rows),romSHA256=romSHA,fixtureSHA256=hashlib.sha256(payload).hexdigest(),scope='four paired independent original instruction segments from retained ordinary menus; intervening waits/tactics omitted, not one continuous RNG transaction',selectionEntry='bank3 896D',selectionStop='8088 before preview rendering',commitEntry='bank3 B2EF',commitStop='B32D before four stateful menu/result actor updates; includes B2FF..B328 commit cleanup',commitTailStop='B337 before RTS after four C033/9598 actor initialization-animation-motion-projection updates',mapper='original MMC3 PRG register writes execute through existing Memory; no C033 or C00C bypass',rawRAMScope='2048-byte input snapshot from retained prior video frame, then unchanged original CPU execution; SP=255 for isolated probe; stack/work writes retained, not claimed as exact live-PC original RAM',writeTraceScope='ordered writes including same-value stores; state_write_fields names recognized persistent commit fields while complete trace also includes CPU stack and work bytes',equipmentIndexing='by character slot0..3, not live actor seat; three signed item slots each',gradesIndexing='by committed actor seat0..3',inventoryIndexing='16 signed item entries at00D3..00E2',sourceRuns={r['run']:r['sourceSHA256'] for r in rows},naturalEvidence=['solo Mode0 Period0','cooperative Mode1 Period0','password020010 equipped home Mode0 Period2','halftime confirmation Mode0 Period1'],rngLimit='selection consumes one draw and matches its adjacent retained frame; commit probe starts directly atB2EF from its retained prior-frame RAM and consumes zero; intervening menu waits and caller main-loop timing are omitted, so neither selectionAfter.random==commitRandomBefore nor commitRandomAfter==commitObservedAfterRandom is claimed')
 (out/'campaign-confirmation-source.json').write_text(json.dumps(metadata,indent=2)+'\n')
 print(json.dumps(dict(records=len(rows),fixtureSHA256=metadata['fixtureSHA256'],rows=[dict(run=r['run'],mode=r['commit']['mode'],period=r['commit']['period'],characters=r['commit']['characters'],selectionRandom=r['selectionAfter']['random'],commitRandom=r['commitRandomBefore'],omittedFrames=r['omittedInterveningVideoFrames']) for r in rows])))

if __name__=='__main__':main()
