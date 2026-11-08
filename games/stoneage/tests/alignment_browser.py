#!/usr/bin/env python3
"""Measure real StoneAge card/menu columns before fixing layout regressions."""
import json,os,subprocess,time,uuid
from pathlib import Path

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/stoneage/browser/alignment'
OUT.mkdir(parents=True,exist_ok=True)
BASE=(ROOT/'.local/stoneage/browser/url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SESSION='sa-align-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/saa'),AGENT_BROWSER_PROFILE=str(ROOT/'.local/saap'),TMPDIR=str(ROOT/'.local/saat'),AGENT_BROWSER_RESTORE_SAVE='never',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[key]).mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'scope':'real local server and production web build; native inputs and measured rendered geometry','measurements':[]}
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
def cards(tab,width):
 b('click',f'[data-stoneage-tab="{tab}"]')
 result=js('''(()=>{const rows=[...document.querySelectorAll('.sa-gear,.sa-skill,.sa-supply')];return rows.map(e=>{const r=e.getBoundingClientRect(),icon=e.querySelector(':scope > img,:scope > svg,:scope > .sa-skill-icon'),body=e.querySelector(e.matches('.sa-skill')?':scope > span:nth-child(2)':':scope > span'),i=icon.getBoundingClientRect(),t=body.getBoundingClientRect();return {name:body.querySelector('strong').textContent,icon:i.x-r.x,text:t.x-r.x,textTop:t.y-r.y,iconTop:i.y-r.y,iconWidth:i.width,cardWidth:r.width,bodyRight:t.right-r.x,children:[...body.children].map(n=>n.getBoundingClientRect().x-t.x)}})})()''')
 assert result,(tab,width)
 REPORT['measurements'].append({'tab':tab,'width':width,'rows':result})
 for row in result:
  assert row['icon']<=17 and row['text']<=70 and row['textTop']<=17 and row['iconTop']<=17,('content centered by text length',tab,width,row)
  assert row['bodyRight']<=row['cardWidth']-8 and all(abs(x)<=1 for x in row['children']),('text column overflow or misalignment',tab,width,row)
 assert max(r['text'] for r in result)-min(r['text'] for r in result)<=1.1,('inconsistent text columns',tab,width,result)
 assert js('document.documentElement.scrollWidth<=innerWidth+1'),('page overflow',tab,width)
def menu(kind,width):
 rows=js('''[...document.querySelectorAll('.sa-field-choices > div > button')].map(e=>{const r=e.getBoundingClientRect();return {title:e.querySelector('span').textContent,left:e.querySelector('span').getBoundingClientRect().x-r.x,children:[...e.children].map(c=>({left:c.getBoundingClientRect().x-r.x,right:c.getBoundingClientRect().right-r.x})),width:r.width}})''')
 assert rows,(kind,width)
 REPORT['measurements'].append({'menu':kind,'width':width,'rows':rows})
 for row in rows:
  assert row['left']<=10 and all(abs(c['left']-row['left'])<=1 and c['right']<=row['width'] for c in row['children']),('menu text has inconsistent left edge',kind,width,row)
def players(width):
 rows=js('''[...document.querySelectorAll('[data-player-seat]')].map(e=>{const r=e.getBoundingClientRect();return [...e.querySelectorAll('.sa-player-summary > span')].slice(0,3).map(n=>n.getBoundingClientRect().top-r.top)})''')
 assert len(rows)==10 and all(len(r)==3 for r in rows)
 assert all(max(r[i] for r in rows)-min(r[i] for r in rows)<=1 for i in range(3)),('public player resource rows drift',width,rows)
 REPORT['measurements'].append({'players':rows,'width':width})
try:
 b('open',BASE);api('/login',{'name':'石器验收','password':'stoneage-browser-only'})
 old=api('/state').get('room')
 if old:
  assert old['kind']=='stoneage' and old['name'] in ['石器浏览器验收','石器对齐验收']
  if old['status']=='playing':command('close')
  command('leave')
 api('/rooms',{'kind':'stoneage','name':'石器对齐验收','capacity':10,'options':{'turnSeconds':45,'preparationSeconds':300}})
 for _ in range(9):command('add_bot')
 command('ready');command('start');b('reload');wait(lambda:js('!!document.querySelector("[data-stoneage-selection]") && !document.querySelector(".game-loading")'))
 for width,height in [(1440,1000),(768,1000),(390,844),(320,740)]:
  b('set','viewport',width,height)
  for tab in ['equipment','skills','supplies']:cards(tab,width)
  for tab in ['pets','rides']:
   b('click',f'[data-stoneage-tab="{tab}"]')
   assert js('document.documentElement.scrollWidth<=innerWidth+1')
  b('click','[data-stoneage-tab="equipment"]');b('screenshot',OUT/f'equipment-{width}.png','--full')
 b('click','[data-stoneage-auto-config]');wait(lambda:room()['game']['data']['self']['loadout'] is not None and js('!document.querySelector("[data-stoneage-lock]").disabled'))
 b('click','[data-stoneage-lock]');wait(lambda:room()['game']['data']['players'][0]['locked']);wait(lambda:room()['game']['data']['phase']=='planning')
 for width,height in [(1440,1000),(768,1000),(390,844),(320,740)]:
  b('set','viewport',width,height);players(width)
  for kind in ['items','magic']:
   b('click',f'[data-stoneage-command="{kind}"]');b('wait',f'[data-stoneage-choices="{kind}"]');menu(kind,width);b('click','button[aria-label="返回战斗菜单"]')
 b('click','[data-stoneage-command="guard"]');b('wait','[data-stoneage-choices="skills"]')
 for width,height in [(1440,1000),(390,844),(320,740)]:
  b('set','viewport',width,height);menu('skills',width);b('screenshot',OUT/f'skills-{width}.png','--full')
 b('click','[data-fullscreen-toggle]');wait(lambda:js('!!document.querySelector("[data-room-fullscreen]")'));menu('skills-fullscreen',320);b('screenshot',OUT/'skills-fullscreen-320.png','--full');b('press','Escape')
 REPORT['errors']=b('errors');assert not REPORT['errors'].get('errors')
 REPORT['passed']=True
finally:
 (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
 try:b('close')
 except Exception:pass
print(json.dumps({'passed':REPORT['passed'],'measurements':len(REPORT['measurements'])}))
