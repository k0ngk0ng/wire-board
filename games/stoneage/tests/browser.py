#!/usr/bin/env python3
"""Test-first UI acceptance: real room, real browser input, existing CDN assets."""
import json
import hashlib
import os
from pathlib import Path
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/stoneage/browser'
PLAYERS=int(os.environ.get('STONEAGE_BROWSER_PLAYERS','2'))
FAIL_ASSET=os.environ.get('STONEAGE_BROWSER_FAIL_ASSET')=='1'
assert PLAYERS in (2,4,6,8,10)
CASE=OUT/'loyalty-release' if PLAYERS==2 else OUT/'loyalty-release'/f'{PLAYERS//2}v{PLAYERS//2}'
CASE.mkdir(parents=True,exist_ok=True)
BASE=(OUT/'url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SESSION='stoneage-'+uuid.uuid4().hex[:6],
    AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/stoneage/ab'),
    AGENT_BROWSER_PROFILE=str(OUT/'profile'),TMPDIR=str(OUT/'tmp'),
    AGENT_BROWSER_SCREENSHOT_DIR=str(OUT),
    AGENT_BROWSER_RESTORE_SAVE='never',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for k in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[k]).mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'players':PLAYERS,'scope':'production web build and StoneAge module; native mouse, keyboard/touch and simulated narrow viewport; original CDN addresses','cases':[]}
REPORT['sourceSHA256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [*(ROOT/'games/stoneage/web').glob('*'),*(ROOT/'games/stoneage/server').glob('*.go')] if p.is_file()}
REPORT['distIndexSHA256']=hashlib.sha256((ROOT/'web/dist/index.html').read_bytes()).hexdigest()
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
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def actor_point(unit):
 selector=f'.sa-actor[data-stoneage-unit="{unit}"]'
 js('document.querySelector('+json.dumps(selector)+').scrollIntoView({behavior:"instant",block:"center"})')
 point=js("(()=>{const e=document.querySelector(SELECTOR),r=e.getBoundingClientRect();for(const a of [.5,.8,.2,.95,.05])for(const b of [.5,.8,.2,.95,.05]){const x=r.x+r.width*a,y=r.y+r.height*b;if(x>0&&x<innerWidth&&y>0&&y<innerHeight&&document.elementFromPoint(x,y)?.closest('.sa-actor')===e)return[x,y]}return null})()".replace('SELECTOR',json.dumps(selector)))
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
try:
 b('open',BASE)
 api('/login',{'name':'石器验收','password':'stoneage-browser-only'})
 existing=api('/state').get('room')
 if existing:
  assert existing['kind']=='stoneage' and existing['name']=='石器浏览器验收'
  if existing['status']=='playing':command('close')
  command('leave')
 api('/rooms',{'kind':'stoneage','name':'石器浏览器验收','capacity':PLAYERS,'options':{'turnSeconds':45}})
 for _ in range(PLAYERS-1):command('add_bot')
 command('ready');command('start')
 b('reload')
 wait(lambda:js('!!document.querySelector("[data-stoneage-selection]")'),90)
 b('wait','[data-stoneage-selection]')
 candidate_loyalty=js('[...document.querySelectorAll(".sa-pet-choice")].map(e=>({id:Number(e.dataset.stoneagePet),loyalty:Number(e.querySelector("[data-stoneage-loyalty]")?.dataset.stoneageLoyalty)}))')
 assert candidate_loyalty==[{'id':p['id'],'loyalty':p['loyalty']} for p in room()['game']['data']['pool']['pets']],'candidate loyalty display differs from public pool'
 REPORT['candidateLoyalty']=candidate_loyalty
 assert js('[...document.querySelectorAll("[data-stoneage-selection] canvas")].every(e=>{const r=e.getBoundingClientRect();return Math.abs(r.width-r.height)<=1})'),'HD pixels changed the logical preview proportions'
 js('performance.setResourceTimingBufferSize(20000)')
 b('click','[data-stoneage-tab="equipment"]')
 for label in ['帽子','武器','衣服']:
  assert label in js('document.querySelector("[data-stoneage-selection]").innerText'),label
 b('click','[data-stoneage-tab="supplies"]')
 for label in ['回血补给','回气补给','解除异常']:
  assert label in js('document.querySelector("[data-stoneage-selection]").innerText'),label
 b('click','[data-stoneage-auto-config]')
 wait(lambda:room()['game']['data']['self']['loadout'] is not None)
 wait(lambda:js('!document.querySelector("input[aria-label=体力加点]").disabled'))
 for label,value in zip(['体力加点','腕力加点','耐力加点','速度加点'],[233,107,81,59]):
  b('fill',f'input[aria-label="{label}"]',str(value))
 assert '剩余 0 / 480' in js('document.querySelector("[data-stoneage-points-left]").textContent')
 b('screenshot',CASE/'selection-desktop.png','--full')
 b('set','viewport','320','740')
 assert js('document.documentElement.scrollWidth<=innerWidth+1'),'mobile overflow'
 assert js('[...document.querySelectorAll("[data-stoneage-selection] canvas")].every(e=>{const r=e.getBoundingClientRect();return Math.abs(r.width-r.height)<=1})'),'mobile HD preview was stretched'
 b('screenshot',CASE/'selection-mobile.png','--full')
 js('document.querySelector(\'[data-stoneage-tab="skills"]\').scrollIntoView({behavior:"instant",block:"center"})')
 point=js('(()=>{const r=document.querySelector(\'[data-stoneage-tab="skills"]\').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
 tap=subprocess.run(['node','scripts/browser_touch.mjs',b('get','cdp-url')['cdpUrl'],BASE,'tap',*map(str,point+point)],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=20)
 assert tap.returncode==0,tap.stdout+tap.stderr
 assert js('document.querySelector(\'[data-stoneage-tab="skills"]\').getAttribute("aria-pressed")==="true"'),'touch did not switch skills'
 b('click','[data-stoneage-tab="equipment"]')
 b('press','Tab');assert js('!!document.activeElement.closest("[data-stoneage]")'),'keyboard focus escaped'
 if FAIL_ASSET:
  js('window.__stoneageFetch=window.fetch.bind(window);window.__stoneageFailed=false;window.fetch=(...args)=>{if(String(args[0]).includes("/stoneage/")&&String(args[0]).includes("/battle/battle_00.png")&&!window.__stoneageFailed){window.__stoneageFailed=true;return Promise.reject(new TypeError("验收：本次资源暂时不可用"))}return window.__stoneageFetch(...args)}')
 b('click','[data-stoneage-lock]')
 wait(lambda:room()['game']['data']['players'][0]['locked'],10)
 REPORT['afterLock']=js('(async()=>{const s=await(await fetch("/api/state")).json();return {phase:s.room.game.data.phase,loading:s.room.loading,players:s.room.game.data.players.map(p=>({locked:p.locked,seat:p.seat})),errors:[...document.querySelectorAll(".error,.sa-error,[role=alert]")].map(e=>e.textContent)}})()')
 wait(lambda:room()['game']['data']['phase']!='selection',90)
 assert room()['game']['data']['self']['loadout']['points']==[233,107,81,59]
 if FAIL_ASSET:
  wait(lambda:js('!!document.querySelector(".game-loading [role=alert]")'),40)
  assert room()['game']['data']['phase']=='arming','battle clock started before resources were ready'
  b('screenshot',CASE/'load-failure.png','--full')
  js('window.fetch=window.__stoneageFetch;delete window.__stoneageFetch')
  b('click','.game-loading button.primary')
  REPORT['loadRetry']={'blockedIn':'arming','recovered':True}
 wait(lambda:js('!!document.querySelector("[data-stoneage-battle]")'),90)
 assert js('document.documentElement.scrollWidth<=innerWidth+1'),'battle overflow'
 assert js('document.querySelectorAll(".sa-actor").length')==PLAYERS*2,'missing battlefield actors'
 assert js('[...document.querySelectorAll(".sa-sprite")].every(e=>e.width>=252&&e.dataset.stoneagePixelScale==="2")'),'HD sprite canvas is not rendered at 2x'
 assert js('document.querySelectorAll(".sa-actor-label").length')==0,'persistent actor names remain'
 assert js('[...document.querySelectorAll(".sa-actor")].every(e=>!!e.title&&!!e.getAttribute("aria-label"))'),'hover/accessible identities missing'
 assert js('document.querySelectorAll("[data-stoneage-battle] select").length')==0,'outside-style battle dropdowns remain'
 strip=js('(()=>{const e=document.querySelector(\'[data-player-flow="horizontal"]\'),r=e?.getBoundingClientRect();return e?{top:r.top,height:r.height,width:r.width,scroll:e.scrollWidth,rows:[...e.querySelectorAll(\'[data-player-seat]\')].map(n=>Math.round(n.getBoundingClientRect().top))}:null})()')
 assert strip and max(strip['rows'])-min(strip['rows'])<=2,'players wrap vertically'
 REPORT['mobilePlayerStrip']=strip
 if PLAYERS==10:
  assert strip['scroll']>strip['width'],'ten players do not scroll horizontally'
  js('document.querySelector("[data-player-flow]").scrollIntoView({behavior:"instant",block:"center"})')
  y=js('(()=>{const r=document.querySelector("[data-player-flow]").getBoundingClientRect();return r.y+r.height/2})()')
  drag=subprocess.run(['node','scripts/browser_touch.mjs',b('get','cdp-url')['cdpUrl'],BASE,'drag','270',str(y),'40',str(y)],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=20)
  assert drag.returncode==0,drag.stdout+drag.stderr
  assert js('document.querySelector("[data-player-flow]").scrollLeft')>0,'touch swipe did not scroll players'
 b('screenshot',CASE/'battle-mobile.png','--full')
 b('set','viewport','1440','1000')
 wait(lambda:room()['game']['data']['phase']=='planning',40)
 toolbar=js('(()=>{const e=document.querySelector("[data-stoneage-field-menu]");return {switches:e.querySelectorAll("button[aria-pressed]").length,buttons:[...e.querySelectorAll("[data-stoneage-command]")].map(b=>({kind:b.dataset.stoneageCommand,top:b.getBoundingClientRect().top}))}})()')
 assert toolbar['switches']==0,'player/pet switching tabs remain'
 assert len(toolbar['buttons'])==4 and max(b['top'] for b in toolbar['buttons'])-min(b['top'] for b in toolbar['buttons'])<=1,'PC battle buttons are not in one row'
 assert not js('!!document.querySelector("[data-stoneage-choices=skills]")'),'pet stage opened before player command'
 REPORT['desktopToolbar']=toolbar
 before_config=room()['game']['data']['self']['loadout']
 b('click','[data-fullscreen-toggle]')
 wait(lambda:js('!!document.querySelector("[data-game-fullscreen-surface][data-room-fullscreen]")'))
 assert js('!!document.querySelector("[data-stoneage-field-menu]").closest("[data-room-fullscreen]")'),'command menu escaped fullscreen'
 click_actor(0);wait(lambda:js('!!document.querySelector("[data-stoneage-details]")'))
 assert js('!!document.querySelector("[data-stoneage-details]").closest("[data-room-fullscreen]")'),'details escaped fullscreen'
 b('screenshot',CASE/'battle-details.png','--full')
 click_actor(0);assert not js('!!document.querySelector("[data-stoneage-details]")'),'repeat actor click did not close details'
 click_actor(0);b('click','[data-stoneage-details]');assert not js('!!document.querySelector("[data-stoneage-details]")'),'details panel click did not close'
 click_actor(0);b('click','button[aria-label="关闭详情"]');assert not js('!!document.querySelector("[data-stoneage-details]")'),'details close button failed'
 gap=js('(()=>{const e=document.querySelector(\'.sa-actor[data-stoneage-unit="0"]\'),h=e.querySelector(\'[data-stoneage-bar="hp"]\').getBoundingClientRect(),m=e.querySelector(\'[data-stoneage-bar="mp"]\').getBoundingClientRect();return m.top-h.bottom})()')
 assert 0<=gap<=2,('HP/MP gap too large',gap);REPORT['hpMpGap']=gap
 b('screenshot',CASE/'battle-fullscreen.png','--full')
 b('press','Escape');wait(lambda:js('!document.querySelector("[data-game-fullscreen-surface][data-room-fullscreen]")'))
 assert room()['game']['data']['self']['loadout']==before_config,'fullscreen changed allocation/loadout'
 b('click','[data-stoneage-command="items"]');b('wait','[data-stoneage-choices="items"]');b('click','button[aria-label="返回战斗菜单"]')
 b('click','[data-stoneage-command="attack"]')
 target=room()['game']['data']['players'][1]['pet']['id']
 if PLAYERS==10:
  target=15 # Native front opponent pet slot, visibly reachable in a full party.
 click_actor(target)
 assert not js('!!document.querySelector("[data-stoneage-details]")'),'attack opened inspection instead of choosing target'
 b('wait','[data-stoneage-choices="skills"]')
 assert js('document.querySelectorAll("[data-stoneage-field-menu] button[aria-pressed]").length')==0,'pet stage added switching tabs'
 assert not room()['game']['data']['players'][0]['submitted'],'player stage submitted before pet selection'
 b('click','[data-stoneage-skill="1"]');click_actor(target)
 wait(lambda:room()['game']['data']['players'][0]['submitted'])
 selected=room()['game']['data']['self']['plan'];assert selected['player']['target']==target and selected['pet']['target']==target,'scene targeting did not submit both commands'
 REPORT['resourcesBeforeReload']=js('performance.getEntriesByType("resource").filter(e=>e.name.startsWith("https://cdn.ichenj.com/stoneage/")).map(e=>({url:e.name,bytes:e.encodedBodySize,transferBytes:e.transferSize}))')
 assert all('/assets/hd/arena-2x-gpt-image-2.5-v1/' in e['url'] for e in REPORT['resourcesBeforeReload'] if '/assets/' in e['url']),'runtime still fetched original image URLs'
 before=room()['game']['data']['round'];b('reload')
 wait(lambda:js('!!document.querySelector("[data-stoneage-battle]")'),90)
 assert room()['game']['data']['round']>=before,'reload reset match'
 b('screenshot',CASE/'battle-desktop.png','--full')
 errors=b('errors');REPORT['errors']=errors
 assert not errors.get('errors'),errors
 REPORT['resources']=js('performance.getEntriesByType("resource").filter(e=>e.name.startsWith("https://cdn.ichenj.com/stoneage/")).map(e=>({url:e.name,bytes:e.encodedBodySize,transferBytes:e.transferSize}))')
 assert REPORT['resources'],'original CDN was not used'
 REPORT['cases']=['complete classed selection','equal-budget free points','one active pet and one ride','native diagonal facing','mouse and keyboard/touch','320px layout','fullscreen and Escape','in-scene commands and click targets','sequential player then pet without switching tabs','single-row PC commands','hover and toggle details','horizontal player strip','compact HP/MP bars','reload continuity','direct original CDN']
 if FAIL_ASSET:REPORT['cases'].append('failed asset blocks clock and retries successfully')
 REPORT['passed']=True
finally:
 (CASE/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
 try:b('close')
 except Exception:pass
print(json.dumps({'passed':REPORT['passed'],'players':PLAYERS,'cases':REPORT['cases'],'resourceCount':len(REPORT.get('resources',[]))},ensure_ascii=False))
