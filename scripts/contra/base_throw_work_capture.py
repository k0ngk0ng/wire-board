"""Original weapon phase -> actual Base roller consumer, declared boundary inputs.

Not a natural campaign run: each case installs semantic projectile/enemy context
at the real $8584 entry, observes all following native code through E54F return,
and restores the prior non-stack RAM. No result or inherited output is injected.
"""
from pathlib import Path
import gzip,hashlib,json,os,shutil,sys
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-base-throw-work/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

ENEMY=[0x30a,0x324,0x33e,0x358,0x4b8,0x4c8,0x4d8,0x4e8,0x4f8,0x508,0x518,0x528,0x538,0x548,0x558,0x568,0x578,0x588,0x598,0x5a8,0x5b8,0x5c8,0x5d8,0x5e8]
EF=['Sprite','Y','X','SpriteAttributes','Routine','FractionY','FractionX','VelocityYHi','VelocityYLo','VelocityXHi','VelocityXLo','Kind','AnimationDelay','VariableA','AttackDelay','Frame','HP','ScoreCollision','StateWidth','Attributes','D0','D1','D2','D3']
WF=['Sprite','Attributes','Kind','YFraction','XFraction','Y','X','VYLo','VXLo','VYHi','VXHi','Timer','Aim','Routine','Owner','RapidOrSpread','DistanceOrSpread','CenterX','CenterY','CenterXFraction','CenterYFraction']

def values(v,names):
 d=dict(v)
 for key in ['VX','VY','VelocityX','VelocityY']:
  if key in d:d[key+'Lo']=d[key]&255;d[key+'Hi']=(d[key]>>8)&255
 for i,n in enumerate(v.get('Data',[])):d['D'+str(i)]=n
 return [d[n] for n in names]

def cases():
 f=json.loads(gzip.decompress((ROOT/'games/contra/engine/testdata/session-base2-throw-entry.json.gz').read_bytes()))
 a=f['before']['World']['Active'];w=a['Combat']['World'];base={}
 for slot,p in enumerate(a['Combat']['Weapons']['Projectiles']):
  for i,v in enumerate(values(p,WF)):base[str(0x368+i*16+slot)]=v
 for slot,p in enumerate(a['Enemies']['Slots']):
  for address,v in zip(ENEMY,values(p,EF)):base[str(address+slot)]=v
 inv=json.loads((ROOT/'docs/research/nes-contra/evidence/enemies-all-stage-data.json').read_text())['stages'][3]
 simple={0x1a:(w['Frame']+1)&255,0x2f:w['WeaponStrength'],0x30:3,0x31:0,0x40:1,0x41:0,0x64:2,0x65:0,0x68:0,0x6a:a['Combat']['ScreenCleared'],0x8e:1,0xfc:232,0xfd:0,0xff:176,0x21:0,0x23:0,0x80:int(inv['specificDispatch'],16)&255,0x81:int(inv['specificDispatch'],16)>>8}
 for i,n in enumerate(['CoresRemaining','PlatingDestroyed','AttackRounds','RedSoldierCreated','GrenadeLauncher']):simple[0x86+i]=a['Enemies']['Base'][n]
 for i,p in enumerate(a['Combat']['Players']):
  for address,key in [(0x90,'State'),(0x38,'GameOver'),(0xae,'SpawnInvincible'),(0xb0,'Invincible'),(0xaa,'Weapon'),(0x334,'X'),(0x31a,'Y')]:simple[address+i]=(p['Motion']['position']>>8)&255 if key=='Y'else p[key]
  simple[0xf1+i]=64 if i==0 else 0;simple[0xf5+i]=64 if i==0 else 0
 for address,v in simple.items():base[str(address)]=v
 for i,v in enumerate(a['Terrain']['Supertiles']):base[str(0x600+i)]=v
 for i,v in enumerate(a['Combat']['Terrain']):base[str(0x680+i)]=v
 out=[]
 def add(name,writes):out.append({'name':name,'writes':writes})
 for seed in [7,165]:add('recorded-entry-'+str(seed),base|{'10':seed})
 for kind in range(1,6):
  for routine in range(3):
   writes=dict(base)
   for at in range(0x368,0x4b8):writes[str(at)]=0
   # Canonical valid sample, one real native weapon dispatch; both upper kind
   # bits and impact/laser delays are exercised without fabricating output.
   p=dict(Sprite=30,Attributes=0,Kind=kind|128,X=136,Y=124,XFraction=72,YFraction=0,VX=-68,VY=-256,Timer=10,Aim=4,Routine=routine,Owner=0,RapidOrSpread=0,DistanceOrSpread=0,CenterX=136,CenterY=124 if kind==3 else 0,CenterXFraction=0,CenterYFraction=0)
   for i,v in enumerate(values(p,WF)):writes[str(0x368+i*16+3)]=v
   writes['10']=165;add(f'kind{kind}-routine{routine}',writes)
 writes=dict(base)
 for at in range(0x368,0x4b8):writes[str(at)]=0
 writes['10']=77;add('no-projectile-retains-entry',writes)
 writes=dict(base)
 for address,v in zip(ENEMY,values(a['Enemies']['Slots'][13],EF)):writes[str(address+15)]=v
 writes['10']=165;add('collision-height-overrides-dispatch',writes)
 return out

def producer_cases():
 base=cases()[0]['writes'];out=[]
 shapes=[
  ('soldier-wave',dict(Kind=25,Routine=2,AnimationDelay=1,Attributes=1,Data=[2,0,0,0])),
  ('roller-wave',dict(Kind=26,Routine=2,AnimationDelay=1,Attributes=0,Data=[0,0,0,0])),
  ('grenade-image-count',dict(Kind=18,Routine=2,Y=96,AttackDelay=253,Data=[128,0,0,0])),
  ('large-explosion-image-family',dict(Kind=2,Routine=7,AnimationDelay=1,Frame=255,StateWidth=137)),
  ('roller-explosion-image-family',dict(Kind=17,Routine=4,AnimationDelay=1,Frame=255)),
  ('launcher-lane',dict(Kind=23,Routine=2,Y=109,AnimationDelay=1,Data=[0,0,0,0])),
  ('wall-shot-velocity',dict(Kind=19,Routine=4,AttackDelay=1,StateWidth=159)),
  ('door-opening-explosion',dict(Kind=20,Routine=9,Sprite=0,Y=104,AnimationDelay=1,Data=[0,0,0,0])),
 ]
 for name,change in shapes:
  enemy=dict(Sprite=1,Y=80,X=128,SpriteAttributes=0,Routine=2,FractionY=0,FractionX=0,VelocityY=0,VelocityX=0,Kind=25,AnimationDelay=0,VariableA=0,AttackDelay=0,Frame=0,HP=241,ScoreCollision=0,StateWidth=129,Attributes=0,Data=[0,0,0,0]);enemy.update(change)
  writes=dict(base)
  for address,v in zip(ENEMY,values(enemy,EF)):writes[str(address+15)]=v
  out.append({'name':name,'writes':writes})
 return out

HOOK=r'''
local cases=CASES;local index=0;local active=nil;local exits={}
local log=assert(io.open(out..'/throw-work.jsonl','w'))
local function read(a)return memory.readbyte(a)end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1 end
local function snap()return hex(0,2048)end
local function arm(pc)
 if exits[pc]then return end;exits[pc]=true
 memory.registerexec(pc,1,function()
  if not active or active.exit~=pc then return end
  local p=active;active=nil
  log:write(string.format('{"case":%d,"beforeWeapon":"%s","beforeEnemy":"%s","afterEnemy":"%s","writes":[%s],"throws":[%s]}\n',p.index,p.before,p.enemy,snap(),table.concat(p.writes,','),table.concat(p.throws,',')))
  for a,v in pairs(p.restore)do memory.writebyte(a,v)end
 end)
end
memory.registerexec(0x8584,1,function()
 if read(0x8000)~=0x1c or index>=#cases then return end
 assert(not active,'overlapping capture');index=index+1
 local p={index=index,restore={},writes={},throws={}}
 for a=0,2047 do if a<256 or a>=512 then p.restore[a]=read(a)end end
 for a,v in pairs(cases[index].writes)do memory.writebyte(tonumber(a),v)end
 p.before=snap();active=p
end)
memory.registerwrite(0x0a,1,function(a,n,v)
 if active then active.writes[#active.writes+1]=string.format('{"pc":%d,"bank":%d,"slot":%d,"value":%d}',memory.getregister('pc'),read(0x8000),read(0x83),v)end
end)
memory.registerexec(0xe54f,1,function()
 if active then active.enemy=snap();active.exit=ret();arm(active.exit)end
end)
memory.registerexec(0x972e,1,function()
 if active and read(0x8000)==0x10 then active.throws[#active.throws+1]=string.format('{"creator":%d,"child":%d,"attribute":%d}',read(0x83),memory.getregister('x'),read(0x0a))end
end)
function finish_probe()assert(not active,'unfinished native phase');assert(index==#cases,'missing cases');log:close()end
'''

def main():
 label=sys.argv[1]if len(sys.argv)>1 else'v1';extra=len(sys.argv)>2 and sys.argv[2]=='producers';declared=cases()+producer_cases() if extra else cases()
 (BASE/'states').mkdir(parents=True,exist_ok=True)
 src=ROOT/'.local/contra-campaign/states/stage-4-v2.fcs';dst=BASE/'states/base4.fcs'
 if dst.exists():assert dst.read_bytes()==src.read_bytes()
 else:shutil.copyfile(src,dst)
 encoded='{'+','.join('{writes={'+','.join('['+k+']='+str(v)for k,v in c['writes'].items())+'}}'for c in declared)+'}'
 spec={'name':'base-throw-work-'+label,'load_state':'base4','frames':len(declared)+4,'segments':[],'captures':[],'hooks_lua':HOOK.replace('CASES',encoded),'timeout_seconds':90}
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 rows=[json.loads(line)for line in(out/'throw-work.jsonl').read_text().splitlines()];assert len(rows)==len(declared)
 data={'source_is_original_execution':True,'interventions':__doc__,'source':json.loads((out/'source.json').read_text()),'cases':declared,'records':rows}
 target=ROOT/('games/contra/engine/testdata/original-base-throw-work-producers.json.gz' if extra else 'games/contra/engine/testdata/original-base-throw-work.json.gz')
 if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows,'independent native repeat differs'
 else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
 print(json.dumps({'cases':len(rows),'entryValues':[int(r['beforeEnemy'][20:22],16)for r in rows],'throws':[r['throws']for r in rows],'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
