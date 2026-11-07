#!/usr/bin/env python3
"""Native scene choice, public-clue puzzle solving, circuits and shared victory."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid
from urllib.parse import urlsplit

ROOT=Path(__file__).resolve().parents[3]
OUT=(ROOT/os.environ.get('ESCAPE_INTERACTION_OUT','.local/escape/interaction')).resolve();assert OUT.is_relative_to(ROOT);OUT.mkdir(parents=True,exist_ok=True)
BASE=os.environ.get('ESCAPE_INTERACTION_URL','').rstrip('/') or (ROOT/'.local/escape/browser/url').read_text().strip()
USERNAME=os.environ.get('ESCAPE_INTERACTION_USER','密室体验');PASSWORD=os.environ.get('ESCAPE_INTERACTION_PASSWORD','escape-browser-password')
assert urlsplit(BASE).hostname in ('127.0.0.1','localhost') or USERNAME.startswith('密室巡检'), 'Remote acceptance requires an owned smoke-test account'
common=Path(subprocess.check_output(['git','rev-parse','--git-common-dir'],cwd=ROOT,text=True).strip())
SOCKETS=(common if common.is_absolute() else ROOT/common).resolve().parent/'.local/escape-ab'
ENV=dict(os.environ,AGENT_BROWSER_SESSION='ei'+uuid.uuid4().hex[:5],AGENT_BROWSER_SOCKET_DIR=str(SOCKETS),AGENT_BROWSER_PROFILE=str(OUT/'profile'),TMPDIR=str(OUT/'tmp'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[key]).mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'scope':'native UI selects both scenarios, reads public clues, solves coupled pump, routes shared circuits, explicitly enables autoplay and reaches natural shared result','command':'python3 -B games/escape/tests/interaction.py','cases':[],'sourceSHA256':{str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in (ROOT/'games/escape/web').glob('*') if p.is_file()}}
def b(*args):
 r=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr;d=json.loads(r.stdout);assert d['success'],d;return d['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()');assert 200<=r['status']<300,r;return r['data']
def room():return api('/state')['room']
def data():return room()['game']['data']
def command(kind,**extra):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def wait(fn,seconds=25):
 until=time.monotonic()+seconds
 while time.monotonic()<until:
  if fn():return
  time.sleep(.1)
 raise AssertionError('native interaction did not reach expected authority state')
def close_panel():
 if js('!!document.querySelector("[data-escape-dialog]")'):b('click','[data-escape-dialog] [data-controller-cancel]')
def path_to(destination):
 v=data();current=v['players'][0]['room'];rooms={r['id']:r for r in v['rooms']};queue=[[current]];seen={current}
 while queue:
  p=queue.pop(0)
  if p[-1]==destination:return p
  for door in rooms[p[-1]].get('doors',[]):
   if door['room'] not in seen:seen.add(door['room']);queue.append(p+[door['room']])
 return []
def go(destination):
 close_panel()
 for target in path_to(destination)[1:]:
  js('document.querySelector(\'button[aria-label="设施地图"]\').scrollIntoView({behavior:"instant",block:"center"})')
  b('click','button[aria-label="设施地图"]');b('wait','[data-escape-map]')
  b('find','nth',int(target[1:]),'[data-escape-map] button','click')
  wait(lambda:data()['players'][0]['room']==target)
  wait(lambda:js('document.querySelector("[data-escape-canvas]").__escapeFrame?.room')==target)
def find(kind):
 for _ in range(20):
  rooms=data()['rooms'];known=next((r for r in rooms if r.get('kind')==kind),None)
  if known:go(known['id']);return
  frontier=next((r['id'] for r in rooms if not r['known'] and path_to(r['id'])),None);assert frontier,'reachable frontier missing';go(frontier)
 raise AssertionError('room not found')
def device(kind):
 v=data();r=next(r for r in v['rooms'] if r['id']==v['players'][0]['room']);return next(d for d in r['objects'] if d['kind']==kind)
def inspect(kind):
 close_panel();d=device(kind);b('click','[data-escape-object="'+d['id']+'"]');wait(lambda:data()['players'][0].get('focus')==d['id'] and not data()['players'][0].get('goal'));return device(kind)
def control(selector):
 # Exercise the scrollable phone inspector with native touch input; never
 # force a click through a clipped panel onto the controls underneath it.
 wait(lambda:js('!!document.querySelector('+json.dumps(selector)+')&&!document.querySelector('+json.dumps(selector)+').disabled'))
 js('document.querySelector(".escape-device-panel").scrollIntoView({behavior:"instant",block:"center"})')
 for _ in range(10):
  bounds=js('''(()=>{const e=document.querySelector('''+json.dumps(selector)+'''),p=e.closest('.escape-device-panel'),r=e.getBoundingClientRect(),q=p.getBoundingClientRect(),at=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return{visible:r.top>=q.top+3&&r.bottom<=q.bottom-3&&(at===e||e.contains(at)),delta:r.top<q.top?-160:160,x:q.x+q.width/2,y:q.y+q.height/2,scroll:p.scrollTop,total:p.scrollHeight,height:p.clientHeight,target:r.toJSON(),panel:q.toJSON(),cover:at?.outerHTML.slice(0,140),overflow:getComputedStyle(p).overflowY}})()''')
  REPORT.setdefault('scrollChecks',[]).append(bounds)
  if bounds['visible']:
   before=js('document.querySelector('+json.dumps(selector)+').getAttribute("aria-pressed")')
   b('click',selector);wait(lambda:js('document.querySelector('+json.dumps(selector)+').getAttribute("aria-pressed")')!=before);return
  q=bounds['panel'];top=round(q['top']+20);bottom=round(q['bottom']-20);x=round(bounds['x'])
  start,end=(bottom,top) if bounds['delta']>0 else (top,bottom)
  process=subprocess.run(['node','scripts/browser_touch.mjs',b('get','cdp-url')['cdpUrl'],BASE,'drag',str(x),str(start),str(x),str(end)],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=20)
  assert process.returncode==0,process.stdout+process.stderr
  js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
 raise AssertionError('phone inspector control cannot be brought into view')

opened=False
try:
 b('open',BASE);opened=True;api('/login',{'name':USERNAME,'password':PASSWORD})
 existing=api('/state').get('room')
 if existing:
  assert existing['kind']=='escape' and existing['name'] in ('密室机关与合作验收','密室视觉与交互验收')
  if existing['status']=='playing':command('close')
  command('leave')
 for scenario,width,height in [('station',1440,1000),('archive',320,820)]:
  if os.environ.get('ESCAPE_INTERACTION_SCENARIO') and os.environ['ESCAPE_INTERACTION_SCENARIO']!=scenario:continue
  b('set','viewport',width,height)
  api('/rooms',{'kind':'escape','name':'密室机关与合作验收','capacity':2,'options':{'scenario':'archive' if scenario=='station' else 'station','difficulty':'easy','seed':'interaction-17'}})
  b('reload');b('wait','.escape-options');assert js('document.querySelectorAll(\'.escape-scenarios input[type=radio]\').length')==2
  b('check','input[name="escape-scenario"][value="'+scenario+'"]')
  b('find','role','button','click','--name','添加电脑玩家');wait(lambda:len(room()['seats'])==2)
  b('find','role','button','click','--name','保存并准备');wait(lambda:room()['options']['scenario']==scenario and room()['seats'][0]['ready'])
  b('find','role','button','click','--name','开始游戏');b('wait','[data-escape-board][data-ready="true"]');wait(lambda:not room().get('loading'))
  js('document.querySelector(\'button[aria-label="设施地图"]\').scrollIntoView({behavior:"instant",block:"center"})');b('click','button[aria-label="设施地图"]');b('wait','[data-escape-map]')
  plan='lift' if scenario=='station' else 'boat';label='货运升降机' if scenario=='station' else '水闸救生艇'
  b('click','.escape-map-plans button[aria-label="计划从'+label+'撤离"]');wait(lambda:data()['stats']['route']==plan);close_panel()
  find('medical');inspect('clue-pump');wait(lambda:any(c['key']=='pump' for c in data().get('journal',[])))
  values=next(c['values'] for c in data()['journal'] if c['key']=='pump')
  find('pump');inspect('pump')
  for index in range(3):
   for _ in range(4):
    old=device('pump')['dials'][index]
    if old==values[index]:break
    b('click',f'.escape-dials button:nth-child({index+1})');wait(lambda:device('pump')['dials'][index]!=old)
  b('find','role','button','click','--name','确认校准');wait(lambda:device('pump')['done'])
  b('screenshot',OUT/f'{scenario}-pump-{width}.png','--full')
  find('storage');inspect('supply');wait(lambda:device('supply')['done'])
  close_panel();js('document.querySelector(\'button[aria-label="物品栏"]\').scrollIntoView({behavior:"instant",block:"center"})');b('click','button[aria-label="物品栏"]');b('wait','[data-escape-inventory]')
  wait(lambda:data()['power']<99 and data()['supplies']['cells']>0,seconds=60)
  before=data()
  if width==320:
   point=js('''(()=>{const e=document.querySelector('[data-escape-inventory] button[aria-label="使用共享电池"]');e.scrollIntoView({behavior:"instant",block:"center"});const r=e.getBoundingClientRect();return[r.x+r.width/2,r.y+r.height/2]})()''')
   tap=subprocess.run(['node','scripts/browser_touch.mjs',b('get','cdp-url')['cdpUrl'],BASE,'tap',*map(str,point+point)],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=20);assert tap.returncode==0,tap.stdout+tap.stderr
  else:b('click','[data-escape-inventory] button[aria-label="使用共享电池"]')
  wait(lambda:data()['supplies']['cells']==before['supplies']['cells']-1)
  assert data()['power']>before['power'],'using an inventory battery did not restore team power'
  wait(lambda:js('Number(document.querySelector(\'[data-escape-item="cells"] [data-escape-item-count]\').textContent)')==data()['supplies']['cells'])
  b('screenshot',OUT/f'{scenario}-inventory-used-{width}.png','--full')
  close_panel()
  find('power');inspect('power')
  control('button[aria-label="照明供电"]');wait(lambda:'lights' not in data()['circuits'])
  extra='pump' if scenario=='station' else 'lift'
  control('button[aria-label="'+('排水供电' if extra=='pump' else '升降供电')+'"]');wait(lambda:extra in data()['circuits'])
  before=data()['circuits'];assert js('document.querySelector(\'button[aria-label="照明供电"]\').disabled'),'a third circuit should be disabled until a slot is released'
  assert data()['circuits']==before,'capacity error changed shared power'
  b('screenshot',OUT/f'{scenario}-circuits-{width}.png','--full')
  control('button[aria-label="'+('排水供电' if extra=='pump' else '升降供电')+'"]');wait(lambda:extra not in data()['circuits'])
  control('button[aria-label="照明供电"]');wait(lambda:'lights' in data()['circuits'])
  close_panel();b('find','role','button','click','--name','开启托管')
  wait(lambda:room()['game']['finished'],seconds=150)
  final=room();assert final['result']['cooperative'] and not final['result']['rated'] and len(final['game']['winners'])==2
  b('screenshot',OUT/f'{scenario}-victory-{width}.png','--full')
  REPORT['cases'].append({'scenario':scenario,'viewport':[width,height],'publicClue':values,'final':final})
  command('leave')
 REPORT['passed']=True
finally:
 if opened:
  if not REPORT['passed']:
   try:REPORT['page']=js('document.body.innerText');b('screenshot',OUT/'failure.png','--full')
   except Exception:pass
  try:b('close')
  except Exception:pass
 shutil.rmtree(OUT/'profile',ignore_errors=True)
 (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':REPORT['passed'],'cases':len(REPORT['cases'])}))
