#!/usr/bin/env python3
"""Native browser proof of a controlled two-member combined attack timeline."""
import hashlib,json,os,subprocess,time,uuid
from pathlib import Path
ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/stoneage/browser/combined'
OUT.mkdir(parents=True,exist_ok=True)
BASE=(ROOT/'.local/stoneage/browser/url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SESSION='sa-combo-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/sac'),AGENT_BROWSER_PROFILE=str(ROOT/'.local/sacp'),TMPDIR=str(ROOT/'.local/sact'),AGENT_BROWSER_RESTORE_SAVE='never',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[key]).mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'scope':'controlled fixture: loyal pets, melee player weapon, fixed initiative and seed; real native input and production combat/rendering'}
REPORT['sourceSHA256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [*(ROOT/'games/stoneage/web').glob('*'),*(ROOT/'games/stoneage/server').glob('*.go')] if p.is_file()}
def b(*args):
 p=subprocess.run(['agent-browser',*map(str,args),'--json'],env=ENV,cwd=ROOT,text=True,capture_output=True,timeout=60)
 assert p.returncode==0,p.stdout+p.stderr
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 option='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+option+');return {status:r.status,body:await r.json()}})()')
 assert 200<=r['status']<300,r
 return r['body']
def room():return api('/state')['room']
def command(kind,**extra):
 nonce=uuid.uuid4().hex
 for attempt in range(12):
  r=room()
  try:return api('/rooms/'+r['id'],dict(type=kind,nonce=nonce,version=r['version'],**extra))
  except AssertionError as e:
   value=e.args[0]
   if not isinstance(value,dict) or value.get('status')!=409 or value.get('body',{}).get('error')!='局面已更新，请根据最新画面重试' or attempt==11:raise
def wait(fn,seconds=90):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if fn():return
  time.sleep(.15)
 raise AssertionError('condition timed out')

try:
 b('open',BASE);api('/login',{'name':'石器验收','password':'stoneage-browser-only'})
 old=api('/state').get('room')
 if old:
  assert old['kind']=='stoneage'
  if old['status']=='playing':command('close')
  command('leave')
 api('/rooms',{'kind':'stoneage','name':'石器合击验收','capacity':2,'options':{'turnSeconds':45}})
 command('add_bot');command('ready');command('start');b('reload');wait(lambda:js('!!document.querySelector("[data-stoneage-auto-config]") && !document.querySelector(".game-loading")'))
 b('click','[data-stoneage-auto-config]');wait(lambda:room()['game']['data']['self']['loadout'] is not None and js('!document.querySelector("[data-stoneage-lock]").disabled'))
 data=room()['game']['data'];weapon=next(w for w in data['pool']['weapons'] if 1<=w['weaponType']<=3)
 b('click','[data-stoneage-tab="equipment"]')
 code='[...document.querySelectorAll(".sa-gear")].find(e=>e.querySelector("strong").textContent===NAME)'.replace('NAME',json.dumps(weapon['name']))
 js('('+code+').scrollIntoView({block:"center"})')
 point=js('(()=>{const r=('+code+').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
 b('mouse','move',*map(round,point));b('mouse','down');b('mouse','up');wait(lambda:js('!document.querySelector("[data-stoneage-lock]").disabled'))
 b('set','viewport','390','844');b('click','[data-stoneage-tab="avatars"]')
 wait(lambda:js('[...document.querySelectorAll("[data-stoneage-avatar]")].length>=8 && [...document.querySelectorAll("[data-stoneage-avatar]")].every(e=>e.querySelector("img").complete&&e.querySelector("img").naturalWidth>0)'),15)
 choices=js('[...document.querySelectorAll("[data-stoneage-avatar]")].map(e=>({id:Number(e.dataset.stoneageAvatar),loaded:e.querySelector("img").complete&&e.querySelector("img").naturalWidth>0}))')
 assert 8<=len(choices)<=12 and all(c['loaded'] for c in choices),'missing character thumbnails'
 avatar=choices[-1]['id'];b('click',f'[data-stoneage-avatar="{avatar}"]')
 assert js('Number(document.querySelector("[data-stoneage-character-preview]").dataset.stoneageCharacterPreview)')==avatar
 wait(lambda:js('!!document.querySelector("[data-stoneage-character-preview] canvas").dataset.stoneageSpriteFrame'))
 assert js('document.documentElement.scrollWidth<=innerWidth+1'),'avatar picker overflows phone'
 REPORT['appearanceChoices']=choices;b('screenshot',OUT/'avatars-mobile.png','--full')
 b('click','[data-stoneage-lock]');wait(lambda:room()['game']['data']['players'][0]['locked']);wait(lambda:room()['game']['data']['phase']=='planning')
 assert room()['game']['data']['self']['loadout']['weapon']==weapon['id']
 assert room()['game']['data']['self']['loadout']['avatar']==avatar
 b('set','viewport','390','844');b('click','[data-fullscreen-toggle]')
 js('window.__comboFrames=[];window.__comboStart=performance.now();(function sample(){const e=document.querySelector(\'.sa-actor[data-stoneage-unit="2"] [data-stoneage-bar="hp"] i\');window.__comboFrames.push({at:performance.now(),actors:[...document.querySelectorAll(\'[data-stoneage-combining=true]\')].map(e=>Number(e.dataset.stoneageUnit)),hp:e?.style.width,markers:[...document.querySelectorAll(\'[data-stoneage-current-target=true]\')].map(e=>Number(e.dataset.stoneageUnit)),motion:[...document.querySelectorAll(\'[data-stoneage-combining=true]\')].map(e=>({id:Number(e.dataset.stoneageUnit),phase:e.dataset.stoneageMotion,x:parseFloat(e.style.left)*6.4,y:parseFloat(e.style.top)*4.8}))});if(performance.now()-window.__comboStart<15000)requestAnimationFrame(sample)})()')
 b('click','[data-stoneage-command="attack"]')
 def target():
  chosen=js("(()=>{const nodes=[...document.querySelectorAll('.sa-actor')].map(e=>{const c=e.querySelector('canvas');return {id:Number(e.dataset.stoneageUnit),depth:Number(e.style.zIndex),r:c.getBoundingClientRect(),c,pixels:c.getContext('2d').getImageData(0,0,c.width,c.height).data}}).sort((a,b)=>b.depth-a.depth),target=nodes.find(e=>e.id===2),r=target.r;let fallback=null;for(let y=Math.max(1,Math.ceil(r.top));y<Math.min(innerHeight-1,r.bottom);y+=2)for(let x=Math.max(1,Math.ceil(r.left));x<Math.min(innerWidth-1,r.right);x+=2){const top=document.elementFromPoint(x,y);if(top?.closest('.sa-field-menu,.sa-actor-details,.sa-scene-log'))continue;const visible=nodes.find(n=>x>=n.r.left&&x<n.r.right&&y>=n.r.top&&y<n.r.bottom&&n.pixels[(Math.floor((y-n.r.top)*n.c.height/n.r.height)*n.c.width+Math.floor((x-n.r.left)*n.c.width/n.r.width))*4+3]>24);if(visible?.id!==2)continue;const result={point:[x,y],underlying:top?.closest('.sa-actor')?.dataset.stoneageUnit??null};if(result.underlying!=='2')return result;fallback=result}return fallback})()")
  assert chosen,'no visible character pixel'
  REPORT.setdefault('nativeTargetClicks',[]).append(chosen)
  b('mouse','move',*chosen['point']);b('mouse','down');b('mouse','up')
 target();b('wait','[data-stoneage-choices="skills"]');b('click','[data-stoneage-skill="1"]');target()
 wait(lambda:room()['game']['data']['self']['plan'] is not None,5)
 submitted=room()['game']['data']['self']['plan'];assert submitted['player']['target']==submitted['pet']['target']==2
 REPORT['submittedTargets']={'player':submitted['player']['target'],'pet':submitted['pet']['target']}
 wait(lambda:js('document.querySelectorAll("[data-stoneage-combining=true][data-stoneage-motion=attack]").length===2'),10)
 b('screenshot',OUT/'combined-fullscreen.png','--full')
 wait(lambda:room()['game']['data']['phase']=='planning' and room()['game']['data']['round']>=2,15)
 data=room()['game']['data'];events=[e for e in data['events'] if e['kind']=='combo'];assert len(events)==1 and set(events[0]['actors'])=={0,1}
 frames=js('window.__comboFrames');active=[f for f in frames if len(f['actors'])==2]
 assert active and set(active[0]['actors'])=={0,1}
 assert events[0]['target']==2 and events[0]['duration']>=1400 and events[0]['impactAt']>400
 assert all(f['markers']==[2] for f in active),'wrong unit highlighted as the attack target'
 for actor in [0,1]:
  phases={m['phase'] for f in active for m in f['motion'] if m['id']==actor}
  assert {'approach','attack','return'}<=phases,phases
  contacts=[m for f in active for m in f['motion'] if m['id']==actor and m['phase']=='attack']
  assert contacts and all(abs(((m['x']-101)**2+(m['y']-150)**2)**.5-64)<1 for m in contacts),'actor stopped near the pet instead of the selected character'
 assert any(f['hp']=='100%' for f in active) and any(f['hp']!='100%' for f in active),'combined HP did not change at shared impact'
 REPORT.update(events=events,frames=frames,errors=b('errors'));assert not REPORT['errors'].get('errors')
 before=room()['game']['data'];b('reload');wait(lambda:js('!!document.querySelector("[data-stoneage-battle]")'))
 after=room()['game']['data'];assert after['round']==before['round'] and after['players'][1]['body']['hp']==before['players'][1]['body']['hp']
 REPORT['passed']=True
finally:
 if not REPORT['passed']:
  try:REPORT['failureUI']=js('({errors:[...document.querySelectorAll("[role=alert]")].map(e=>e.textContent),avatars:[...document.querySelectorAll("[data-stoneage-avatar]")].map(e=>({id:e.dataset.stoneageAvatar,loaded:e.querySelector("img").complete,frame:e.querySelector("canvas")?.dataset.stoneageSpriteFrame}))})');b('screenshot',OUT/'failure.png','--full')
  except Exception:pass
 (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
 try:b('close')
 except Exception:pass
print(json.dumps({'passed':REPORT['passed']}))
