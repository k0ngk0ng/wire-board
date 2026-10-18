"""Original direct aliases and B383 dispatcher for shot plans11/12/13."""
import gzip,hashlib,itertools,json
from verify_ai_command_pass import ROOT,ROM,BASE,Mem,MPU,control,view
ENTRIES={11:0xb564,12:0xb58e,13:0xb58e}

def setup(actor,plan,negative,zone,action,entered,human,character,seed):
 r=bytearray(BASE)
 for address,value in [(0x512,action|(128 if entered else 0)),(0x52c,255),(0x4ec,32),(0x46b,128),(0x672,134),(0x676,9),(0x66e,39),(0x682,plan|(128 if negative else 0)),(0x67e,37),(0x67a,2),(0x619,0),(0x61d,3),(0x66a,zone),(0x3e3,character),(0x572,4),(0x576,0)]:r[address+actor]=value
 r[0x539:0x53d]=bytes([128]*4)
 if human:r[0x539+(actor^1)]=0
 r[0x58c]=0;r[0x58b]=1;r[0x586:0x588]=bytes([0,1]);r[0x53f]=actor
 r[0x48:0x4c]=seed.to_bytes(4,'big')
 return r

def run(r,actor,direct,caller=False):
 plan=r[0x682+actor]&127;m=Mem(r);cpu=MPU(memory=m,pc=0xa2ab if caller else ENTRIES[plan] if direct else 0xb383);cpu.x=actor;cpu.p=0x20;cpu.sp=255;cpu.stPushWord(0x6fff)
 for steps in range(3000):
  if cpu.pc==0x7000:break
  cpu.step()
 else:raise RuntimeError(hex(cpu.pc))
 v=view(r,actor);v['TeamHasHuman']=(r[0x539+(actor&2)]&r[0x53a+(actor&2)]&128)==0
 ctx=dict(Character=r[0x3e3+actor],Court=r[0x58b],JumpVelocity=1792,JumpPhase=0,ShotSkill=3)
 av=view(m,actor)
 after=dict(Control=control(m,actor),Action=av['Action'],ActionEntered=av['ActionEntered'],Heading=av['Heading'],FacingLeft=av['FacingLeft'],Random=av['Random'],BindHoop=-1)
 shot={}
 for address,flag,field in [(0x52c,'ChangeNextAction','NextAction'),(0x572,'ChangeShotCycle','ShotCycle'),(0x576,'ChangeJumpPhase','JumpPhase')]:
  if address+actor in m.writes:shot[flag]=True;shot[field]=m[address+actor] if field=='NextAction' else (m[address+actor] if m[address+actor]<128 else m[address+actor]-256)
 if shot:after['ShotPlan']=shot
 return dict(direct=direct,caller=caller,plan=plan,before=control(r,actor),view=v,context=ctx,after=after)

def selector_audit():
 rows=[];image=Mem(BASE)
 for team,region,value in itertools.product(range(8),range(2),range(256)):
  first=(value&127)<<1;second=((value&1)^(value>>7))<<1;seed=(first<<24)|(second<<16)|0x3456
  r=bytearray(BASE);r[0x586]=team;r[0x66a]=0x50 if region else 0x32;r[0x48:0x4c]=seed.to_bytes(4,'big')
  m=Mem(r);cpu=MPU(memory=m,pc=0xb6d0);cpu.x=0;cpu.p=0x20;cpu.sp=255;cpu.stPushWord(0x6fff);seen=False
  for n in range(1000):
   if cpu.pc==0xb6f9:assert cpu.a==value;seen=True
   if cpu.pc==0x7000:break
   cpu.step()
  assert cpu.pc==0x7000 and seen
  rows.append(dict(team=team,region=region,randomByte=value,seed=seed,afterRandom=int.from_bytes(bytes(m[0x48:0x4c]),'big'),plan=m[0x682]))
 sites={hex(t):[hex(i) for i in range(0x8000,0xbffe) if image[i] in (0x20,0x4c) and image[i+1]==t&255 and image[i+2]==t>>8] for t in [0xb383,0xb564,0xb58e,0xb6d0]}
 writes=[dict(address=hex(i),opcode=hex(image[i])) for i in range(0x8000,0xbffe) if image[i] in (0x9d,0x99,0x8d,0xfe,0xde,0xee,0xce) and image[i+1:i+3]==[0x82,0x06]]
 return dict(selectorCalls=rows,selectorPlans=sorted(set(x['plan'] for x in rows)),choiceBytes=list(image[0xb711:0xb71a]),dispatchEntries={str(p):hex(image[0xb38b+2*p]+256*image[0xb38c+2*p]) for p in range(16)},directCallByteCandidates=sites,parameterWriteByteCandidates=writes,scope='B6D0 exhausts all returned random bytes for8 teams and2 zone regions; byte-pattern candidates require code-context review and do not prove all indirect callers')

def main():
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 rows=[]
 for actor,zone,action,entered,negative,direct in itertools.product(range(4),[0,0x44,0x45,0x50,0x62],[0x10,0x12,0x14,0x16,0x19],[False,True],[False,True],[False,True]):
  rows.append(run(setup(actor,11,negative,zone,action,entered,False,0,0x12345678),actor,direct))
 for actor,plan,zone,action,entered,human,character,seed,negative,direct in itertools.product(range(4),[12,13],[0x50,0x60,0x62,0x65],[0x10,0x12,0x14,0x16],[False,True],[False,True],[0,1],[0,0x12345678],[False,True],[False,True]):
  rows.append(run(setup(actor,plan,negative,zone,action,entered,human,character,seed),actor,direct))
 audit=selector_audit()
 selections={}
 for r in audit['selectorCalls']:
  if r['plan']==12:selections.setdefault((r['team'],r['region']),[]).append(r)
 for (team,region),choices in selections.items():
  for actor,action,entered,choice in itertools.product(range(4),[0x10,0x12,0x14,0x16],[False,True],[choices[0],choices[-1]]):
   r=setup(actor,13,False,0x50,action,entered,False,actor%2,choice['seed']);r[0x672+actor]=6;r[0x586+actor//2]=team
   rows.append(run(r,actor,False,True))
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);(out/'ai-shot-plans11-13.json.gz').write_bytes(packed)
 auditpacked=gzip.compress((json.dumps(audit,separators=(',',':'))+'\n').encode(),mtime=0);(out/'ai-shot-selector-audit.json.gz').write_bytes(auditpacked)
 meta=dict(records=len(rows),callerCalls=sum(r["caller"] for r in rows),entries=ENTRIES,scope='isolated original direct aliases and B383; controlled setup is not a natural producer',romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest(),selectorSHA256=hashlib.sha256(auditpacked).hexdigest(),selectorCalls=len(audit['selectorCalls']),selectorPlans=audit['selectorPlans'])
 (out/'ai-shot-plans11-13-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
