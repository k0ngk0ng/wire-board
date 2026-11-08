#!/usr/bin/env python3
"""Record stationary bow scatter and consecutive melee through the real renderer."""
import json
import hashlib
import os
from pathlib import Path
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/stoneage/browser/sequence'
BASE=(ROOT/'.local/stoneage/browser/url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SESSION='stoneage-'+uuid.uuid4().hex[:6],
    AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/sqb'),
    AGENT_BROWSER_PROFILE=str(ROOT/'.local/sqp'),TMPDIR=str(ROOT/'.local/sqt'),
    AGENT_BROWSER_SCREENSHOT_DIR=str(OUT),
    AGENT_BROWSER_RESTORE_SAVE='never',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for k in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[k]).mkdir(parents=True,exist_ok=True)
def b(*args):
 p=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=60)
 assert p.returncode==0,p.stdout+p.stderr
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()')
 assert 200<=r['status']<300,r;return r['body']
def room():return api('/state')['room']
def command(kind,**extra):
 nonce=uuid.uuid4().hex
 for attempt in range(12):
  r=room()
  try:return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=nonce,**extra))
  except AssertionError as e:
   value=e.args[0]
   if not isinstance(value,dict) or value.get('status')!=409 or value.get('body',{}).get('error')!='局面已更新，请根据最新画面重试' or attempt==11:raise

def actor_point(unit):
 selector=f'.sa-actor[data-stoneage-unit="{unit}"]'
 js('document.querySelector('+json.dumps(selector)+').scrollIntoView({behavior:"instant",block:"center"})')
 point=js("(()=>{const wanted=UNIT,nodes=[...document.querySelectorAll('.sa-actor')].map(e=>{const c=e.querySelector('canvas');return {id:Number(e.dataset.stoneageUnit),depth:Number(e.style.zIndex),r:c.getBoundingClientRect(),c,pixels:c.getContext('2d').getImageData(0,0,c.width,c.height).data}}).sort((a,b)=>b.depth-a.depth),r=nodes.find(n=>n.id===wanted).r,points=[];for(const y of [.65,.5,.8,.35])for(const x of [.5,.65,.35])points.push([r.left+r.width*x,r.top+r.height*y]);for(let y=Math.max(3,Math.ceil(r.top));y<Math.min(innerHeight-3,r.bottom);y+=3)for(let x=Math.max(3,Math.ceil(r.left));x<Math.min(innerWidth-3,r.right);x+=3)points.push([x,y]);function visible(x,y){return nodes.find(n=>x>=n.r.left&&x<n.r.right&&y>=n.r.top&&y<n.r.bottom&&n.pixels[(Math.floor((y-n.r.top)*n.c.height/n.r.height)*n.c.width+Math.floor((x-n.r.left)*n.c.width/n.r.width))*4+3]>100)?.id}for(const [x,y] of points){if(x<3||y<3||x>=innerWidth-3||y>=innerHeight-3||document.elementFromPoint(x,y)?.closest('.sa-field-menu,.sa-actor-details,.sa-scene-log,.sa-scene-log-button'))continue;if([[-2,-2],[0,0],[2,2],[-2,2],[2,-2]].every(([dx,dy])=>visible(x+dx,y+dy)===wanted))return[x,y]}return null})()".replace('UNIT',str(unit)))
 assert point,('actor has no reachable pointer target',unit)
 return point

def click_actor(unit):
 point=[round(v) for v in actor_point(unit)];b('mouse','move',*point);b('mouse','down');b('mouse','up')

def wait(fn,seconds=40):
 until=time.monotonic()+seconds
 while time.monotonic()<until:
  if fn():return
  time.sleep(.15)
 raise AssertionError('condition timed out')
CASE=OUT;CASE.mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'scope':'controlled production-module fixture; 100 loyalty, 10000 HP, fixed seed, stone defenders; authenticated HTTP loadout; native combat clicks and production 1.5x timeline'}
REPORT['sourceSHA256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [*(ROOT/'games/stoneage/web').glob('*'),*(ROOT/'games/stoneage/server').glob('*.go'),ROOT/'games/stoneage/motion/metadata.json',ROOT/'games/stoneage/server/elite_source.json'] if p.is_file()}
REPORT['distIndexSHA256']=hashlib.sha256((ROOT/'web/dist/index.html').read_bytes()).hexdigest()
try:
 b('open',BASE);api('/login',{'name':'石器验收','password':'stoneage-browser-only'})
 old=api('/state').get('room')
 if old:
  assert old['kind']=='stoneage'
  if old['status']=='playing':command('close')
  command('leave')
 api('/rooms',{'kind':'stoneage','name':'石器连续与远程验收','capacity':2,'options':{'turnSeconds':45}})
 command('add_bot');command('ready');command('start');b('reload')
 wait(lambda:js('!!document.querySelector("[data-stoneage-auto-config]")&&!document.querySelector(".game-loading")'),90)
 b('click','[data-stoneage-auto-config]');wait(lambda:room()['game']['data']['self']['loadout'] is not None)
 data=room()['game']['data'];loadout=data['self']['loadout'];loadout['skills']=[1,2,10,31]
 loadout['weapon']=next(w['id'] for w in data['pool']['weapons'] if w['weaponType']==4)
 command('action',action={'type':'lock','session':data['session'],'window':data['window'],'loadout':loadout});b('reload')
 wait(lambda:room()['game']['data']['phase']=='planning' and js('!!document.querySelector("[data-stoneage=planning] [data-stoneage-command=attack]:not(:disabled)")'),90)
 b('set','viewport','1440','1000');b('click','[data-fullscreen-toggle]')
 js('window.__sequenceFrames=[];window.__sequenceStart=performance.now();(function sample(){window.__sequenceFrames.push({at:Date.now(),actors:[...document.querySelectorAll(".sa-actor")].map(e=>({id:Number(e.dataset.stoneageUnit),phase:e.dataset.stoneageMotion,x:parseFloat(e.style.left)*6.4,y:parseFloat(e.style.top)*4.8,hp:e.querySelector("[data-stoneage-bar=hp] i").style.width})),projectiles:[...document.querySelectorAll("[data-stoneage-projectile]")].map(e=>({kind:e.dataset.stoneageProjectile,target:Number(e.dataset.stoneageProjectileTarget),phase:e.dataset.stoneageProjectilePhase,x:Number(e.dataset.stoneageProjectileX),y:Number(e.dataset.stoneageProjectileY)}))});if(performance.now()-window.__sequenceStart<18000)requestAnimationFrame(sample)})()')
 b('click','[data-stoneage-command="attack"]');b('wait','button[aria-label="取消选择目标"]');click_actor(2)
 b('wait','[data-stoneage-skill="10"]');b('click','[data-stoneage-skill="10"]');b('wait','button[aria-label="取消选择目标"]');click_actor(2)
 wait(lambda:room()['game']['data']['phase']=='resolving',5)
 data=room()['game']['data'];REPORT['events']=data['events'];REPORT['playback']=data['playback']
 wait(lambda:js('!!document.querySelector("[data-stoneage-projectile=arrow]")'),15)
 b('screenshot',CASE/'arrow-flight.png','--full')
 wait(lambda:room()['game']['data']['phase']=='planning' and room()['game']['data']['round']==2,30)
 frames=js('window.__sequenceFrames');REPORT['frames']=frames
 bow=[e for e in data['events'] if e['actor']==0 and e['kind'] in ['attack','miss']]
 dual=[e for e in data['events'] if e['actor']==1 and e['kind'] in ['attack','miss']]
 assert len(bow)>=2 and len({e['target'] for e in bow})>=2,('fixture did not exercise scatter',bow)
 assert len(dual)==2 and all(e['target']==2 for e in dual),('continuous attack changed target',dual)
 assert all(abs(a['x']-517)<.1 and abs(a['y']-384)<.1 for f in frames for a in f['actors'] if a['id']==0),'bow user left formation'
 phases=[next(a for a in f['actors'] if a['id']==1).get('phase') for f in frames]
 runs=[p for i,p in enumerate(phases) if i==0 or phases[i-1]!=p]
 assert runs.count('approach')==1 and runs.count('return')==1,('continuous attack approached/returned between hits',runs)
 for e in bow:
  visible=[(f,p) for f in frames for p in f['projectiles'] if p['target']==e['target'] and p['kind']=='arrow' and p['phase']=='flight']
  assert len(visible)>=4 and max(p['x'] for f,p in visible)-min(p['x'] for f,p in visible)>30,'arrow did not visibly fly'
  impact=data['playback']['startedAt']+e['at']+e['impactAt']
  before=[f for f,p in visible if f['at']<impact-50];assert before
  event_index=data['events'].index(e)
  prior=data['events'][event_index-1]['players'] if event_index else data['playback']['before']
  expected=prior[e['target']//2]['pet' if e['target']%2 else 'body']
  before_hp=[float(next(a['hp'].rstrip('%') for a in f['actors'] if a['id']==e['target'])) for f in before]
  assert all(abs(hp-expected['hp']/expected['maxHp']*100)<.001 for hp in before_hp),'HP changed ahead of arrow arrival'
 assert data['playback']['duration']<11000,'controlled round still has excessive pauses'
 REPORT['checks']={'scatterTargets':[e['target'] for e in bow],'dualTargets':[e['target'] for e in dual],'petPhaseRuns':runs,'roundDuration':data['playback']['duration'],'nativeArrowFrames':sum(len(f['projectiles']) for f in frames)}

 # Double charge prepares for two complete rounds, then releases once.
 charge_round=room()['game']['data']['round'];charge_history=[]
 wait(lambda:js('!!document.querySelector("[data-stoneage=planning] [data-stoneage-command=guard]:not(:disabled)")'))
 b('click','[data-stoneage-command="guard"]');b('wait','[data-stoneage-skill="31"]');b('click','[data-stoneage-skill="31"]');b('wait','button[aria-label="取消选择目标"]');click_actor(2)
 for step in range(3):
  wait(lambda:(current_phase:=room()['game']['data'])['phase']=='resolving' or current_phase['round']==charge_round+step+1,5)
  current=room()['game']['data'];charge_history.append(current)
  strikes=[e for e in current['events'] if e['actor']==1 and e['kind'] in ['attack','miss']]
  assert len(strikes)==(1 if step==2 else 0),'double charge did not wait two rounds and strike once'
  wait(lambda:room()['game']['data']['phase']=='planning' and room()['game']['data']['round']==charge_round+step+1,30)
  wait(lambda:js('!!document.querySelector("[data-stoneage=planning] [data-stoneage-command=guard]:not(:disabled)")'))
  if step<2:
   assert room()['game']['data']['players'][0]['pet']['charge']['remaining']==1-step
   if step==0:b('screenshot',CASE/'double-charge.png','--full')
   b('click','[data-stoneage-command="guard"]')
   wait(lambda:room()['game']['data']['players'][0]['submitted'],5)
   assert not js('!!document.querySelector("[data-stoneage-choices=skills]")'),'charging pet still required another command'
 assert not room()['game']['data']['players'][0]['pet'].get('charge'),'charge remained after release'
 REPORT['chargeHistory']=charge_history
 REPORT['checks']['chargeAttackCounts']=[sum(e['actor']==1 and e['kind'] in ['attack','miss'] for e in turn['events']) for turn in charge_history]
 b('set','viewport','320','740');b('screenshot',CASE/'mobile-after.png','--full')
 before=room()['game']['data'];b('reload');wait(lambda:js('!!document.querySelector("[data-stoneage-battle]")'),90)
 after=room()['game']['data'];assert after['round']==before['round'] and after['players'][1]['body']['hp']==before['players'][1]['body']['hp']
 REPORT['errors']=b('errors');assert not REPORT['errors'].get('errors');REPORT['passed']=True
finally:
 if not REPORT['passed']:
  try:b('screenshot',CASE/'failure.png','--full')
  except Exception:pass
 (CASE/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
 try:b('close')
 except Exception:pass
print(json.dumps({'passed':REPORT['passed'],'checks':REPORT.get('checks')}))
