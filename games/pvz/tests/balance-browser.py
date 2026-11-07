#!/usr/bin/env python3
"""Real authenticated room, native planting, visible wallet and producer timing.
The opt-in production browser fixture provides HTTP/WebSocket and loading gates.
"""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/pvz/balance/browser';OUT.mkdir(parents=True,exist_ok=True)
BASE=os.environ['PVZ_BROWSER_URL']
ENV=dict(os.environ,AGENT_BROWSER_SESSION='pbal'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/ab'),AGENT_BROWSER_PROFILE=str(OUT/'profile'),TMPDIR=str(OUT/'tmp'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for k in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[k]).mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'scope':'real production room and native mouse; natural wallets, visible sunflower timing; 320px viewport emulation','cases':[]}
opened=False
peer=None
only_battle=bool(os.environ.get('PVZ_BALANCE_BATTLE_ONLY'))

def b(*args,env=None):
 r=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=env or ENV,text=True,capture_output=True,timeout=60)
 assert r.returncode==0,r.stdout+r.stderr
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code,env=None):return b('eval',code,env=env)['result']
def api(path,body=None,env=None):
 opt='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opt+');return {status:r.status,body:await r.json()}})()',env=env);assert 200<=r['status']<300,r;return r['body']
def room(env=None):return api('/state',env=env)['room']
def command(kind,env=None,**extra):
 for _ in range(8):
  r=room(env=env);body=dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra)
  v=js('(async()=>{const r=await fetch('+json.dumps('/api/rooms/'+r['id'])+',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')});return {status:r.status,body:await r.json()}})()',env=env)
  if v['status']==409:continue
  assert v['status']==200,v;return v['body']
 raise AssertionError('lifecycle version did not converge')
def wait(fn,seconds=40):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if fn():return
  time.sleep(.15)
 raise AssertionError('condition timed out')
def select(kind):
 selector='[data-pvz-card='+json.dumps(kind)+']'
 if not js('document.querySelector('+json.dumps(selector)+').getAttribute("aria-pressed")==="true"'):b('click',selector)
 assert js('document.querySelector('+json.dumps(selector)+').getAttribute("aria-pressed")==="true"')
def plant(kind,row,col):
 select(kind);selector='[data-pvz-cell='+json.dumps(str(row)+':'+str(col))+']';b('scrollintoview',selector)
 rect=js('document.querySelector('+json.dumps(selector)+').getBoundingClientRect().toJSON()')
 b('mouse','move',round(rect['left']+rect['width']/2),round(rect['top']+rect['height']/2));b('mouse','down');b('mouse','up')
 wait(lambda:any(p['kind']==kind and p['owner']==0 and p['row']==row and p['col']==col for p in room()['game']['data']['plants']))
def sunflower():
 return next((p for p in room()['game']['data']['plants'] if p['owner']==0 and p['kind']=='sunflower'),None)

try:
 b('open',BASE);opened=True
 api('/register',{'name':'阳光平衡验收'+uuid.uuid4().hex[:5],'password':'pvz-balance-local','invite':'pvz-browser'});b('reload')
 for scene,n,opening in ([] if only_battle else [('day',2,500),('roofnight',4,700)]):
  b('set','viewport',1440,1000)
  api('/rooms',{'kind':'pvz','name':'阳光经济与阵营验收','capacity':n,'options':{'scene':scene,'mode':'duel' if n==2 else 'team','teams':[i%2 for i in range(n)],'roundSeconds':180,'prepSeconds':30}})
  command('add_bot')
  if n==4:
   peer=dict(ENV,AGENT_BROWSER_SESSION='pbalpeer'+uuid.uuid4().hex[:5],AGENT_BROWSER_PROFILE=str(OUT/'profile-peer'));Path(peer['AGENT_BROWSER_PROFILE']).mkdir(parents=True,exist_ok=True)
   b('open',BASE,env=peer);api('/register',{'name':'阳光队友验收'+uuid.uuid4().hex[:5],'password':'pvz-balance-local','invite':'pvz-browser'},env=peer);b('reload',env=peer)
   r=room();api('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex},env=peer)
   command('add_bot');command('ready',env=peer)
  # The waiting-room options are real production UI when editable.
  labels=js('[...document.querySelectorAll("select[aria-label=植物大战僵尸分队] option")].map(e=>e.textContent)')
  if labels:assert all('植物' in v and '僵尸' in v and '防守' not in v and '进攻' not in v for v in labels),labels
  command('ready');command('start')
  initial=room()['game']['data'];assert initial['snapshot']['rulesVersion']==3
  assert initial['players'][0]['money']==opening//(n//2),initial['players']
  b('wait','[data-pvz-board][data-ready=true]')
  if peer:b('wait','[data-pvz-board][data-ready=true]',env=peer)
  wait(lambda:not room().get('loading'))
  row,col=2,0
  if scene=='roofnight':plant('flowerpot',row,col)
  plant('sunflower',row,col)
  born=sunflower();assert born is not None
  wait(lambda:(sunflower() or {}).get('flags',{}).get('attackAt',0)>0,15)
  first=sunflower()['flags']['attackAt'];first_seconds=(first-born['bornAt'])*.05
  assert 3<=first_seconds<=6.05,first_seconds
  wait(lambda:(sunflower() or {}).get('flags',{}).get('attackAt',0)>first,25)
  second=sunflower()['flags']['attackAt'];repeat=(second-first)*.05
  assert 18<=repeat<=20.05,repeat
  wait(lambda:not any(p['owner']==0 and p['kind']=='sun' for p in room()['game']['data']['pickups']),5)
  after=room()['game']['data']
  visible=int(re.sub(r'\D','',js('document.querySelector('+json.dumps('[data-pvz-money="0"]')+').textContent')))
  assert abs(visible-after['players'][0]['money'])<=25,{'visible':visible,'authority':after['players'][0]['money']}
  case={'scene':scene,'players':n,'openingTeamSun':opening,'initialPlayers':initial['players'],'roleLabels':labels,'firstSunSeconds':first_seconds,'repeatSunSeconds':repeat,'visibleSun':visible,'authoritySun':after['players'][0]['money'],'tick':after['tick']}
  if peer:
   assert after['players'][2]['seq']==0,'manual teammate acted without input'
   credit=(after['players'][0]['money']+75-initial['players'][0]['money'])-(after['players'][2]['money']-initial['players'][2]['money'])
   assert abs(credit-50)<=1,{'productionCreditDifference':credit,'players':after['players']}
   case['productionCreditDifference']=credit
  b('screenshot',OUT/(scene+'-desktop.png'),'--full')
  b('set','viewport',320,760);b('scrollintoview','[data-pvz-board]')
  assert not js('document.documentElement.scrollWidth>innerWidth+1')
  b('screenshot',OUT/(scene+'-320.png'),'--full');REPORT['cases'].append(case)
  command('close')
  if peer:command('leave',env=peer)
  command('leave')
 if peer:
  b('close',env=peer);shutil.rmtree(OUT/'profile-peer',ignore_errors=True);peer=None
 # A real battle opening: two producers and fire on all five lanes through
 # native input, then the platform's ordinary autoplay handles continuing play.
 b('set','viewport',1440,1000)
 api('/rooms',{'kind':'pvz','name':'五路布阵与实战验收','capacity':2,'options':{'scene':'day','mode':'duel','teams':[0,1],'roundSeconds':180,'prepSeconds':10}})
 command('add_bot');command('ready');command('start')
 b('wait','[data-pvz-board][data-ready=true]');wait(lambda:not room().get('loading'))
 plant('sunflower',2,0);plant('peashooter',0,2)
 def card_ready(kind):
  d=room()['game']['data'];return d['players'][0]['cooldowns'].get(kind,0)<=d['tick']
 wait(lambda:card_ready('sunflower'),15);plant('sunflower',3,0)
 for row in range(1,5):
  wait(lambda:card_ready('peashooter'),15);plant('peashooter',row,2)
 formed=room()['game']['data']
 rows={p['row'] for p in formed['plants'] if p['owner']==0 and p['kind']=='peashooter'}
 assert len(rows)==5,'opening fire was lost before five lanes could be covered'
 assert len([p for p in formed['plants'] if p['owner']==0 and p['kind']=='sunflower'])==2
 opening_ms=(formed['tick']-formed['snapshot']['battleStartTick'])*50
 assert opening_ms<=30000,opening_ms
 b('screenshot',OUT/'five-lane-opening.png','--full')
 command('autoplay',enabled=True)
 def battle_sample_ready():
  d=room()['game']['data'];return d['round']>1 or d['phase']!='battle' or (d['tick']-d['snapshot']['battleStartTick'])*50>=60000
 wait(battle_sample_ready,60)
 live=room()['game']['data']
 b('screenshot',OUT/'live-battle.png','--full')
 REPORT['cases'].append({'kind':'native-five-lane-opening-and-live-battle','openingBattleMS':opening_ms,'openingPlants':formed['plants'],'openingWallet':formed['players'][0]['money'],'livePhase':live['phase'],'liveRound':live['round'],'liveTick':live['tick'],'liveWallet':live['players'][0]['money'],'liveZombies':len(live['zombies']),'autoplayAfterOpening':True,'rounds':live['rounds']})
 command('close');command('leave')
 REPORT['passed']=True
finally:
 if opened:
  try:
   if api('/state').get('room'):command('close');command('leave')
  except Exception:pass
  try:b('close')
  except Exception:pass
 shutil.rmtree(OUT/'profile',ignore_errors=True)
 if peer:
  try:b('close',env=peer)
  except Exception:pass
  shutil.rmtree(OUT/'profile-peer',ignore_errors=True)
 (OUT/('result-battle.json' if only_battle else 'result.json')).write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({'passed':REPORT['passed'],'cases':len(REPORT['cases'])}))
