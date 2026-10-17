#!/usr/bin/env python3
"""Real HTTP/browser checks for stable Splendor turns and deliberate logout.
Failures: own/opponent turns change bank/board/market/wallet geometry; hidden
actions remain focusable/clickable; logout stays in the header, appears on a
peer profile, fires when closing a profile, or fails to clear authentication.
"""
import argparse, http.cookiejar, json, os, shutil, subprocess, time, uuid
from pathlib import Path
from urllib.parse import urlsplit
import urllib.request
root=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--url',default='http://127.0.0.1:18099');p.add_argument('--baseline',action='store_true');a=p.parse_args()
assert urlsplit(a.url).hostname in ('localhost','127.0.0.1')
run=uuid.uuid4().hex[:8];out=root/'.local'/('bank-layout-'+run);out.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(root/'.local/ab3'),AGENT_BROWSER_SESSION='bank-'+run,AGENT_BROWSER_PROFILE=str(out/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad',TMPDIR=str(root/'.local/tmp'))
report={'passed':False,'widths':[],'issues':[]}
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,text=True,capture_output=True,timeout=30);assert not r.returncode,r.stdout+r.stderr
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def client():return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def api(c,path,body=None):
 with c.open(urllib.request.Request(a.url+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'}),timeout=15) as r:return json.load(r)
def issue(ok,message):
 if not ok:report['issues'].append(message)
 if not a.baseline:assert ok,message
def click_profile(self=True):
 selector='header button[aria-label="我的战绩与好友"]' if self else '.players-strip [data-player-seat="'+str(1-me)+'"] .player-link'
 js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center",inline:"center"})')
 b('click',selector);b('wait','.profile-page')
def close_profile():
 b('click','.modal[aria-label=玩家页面] button[aria-label=关闭]');b('wait','--fn','!document.querySelector(".modal[aria-label=玩家页面]")')
try:
 admin=client();api(admin,'/login',{'name':'LayoutAdmin','password':'layout-local-password'});api(admin,'/admin/games/splendor',{'enabled':True})
 b('open',a.url)
 for width in [1440,851,850,390,320]:
  clients=[client(),client()];names=['布局'+run+str(width)+str(i) for i in range(2)];password='layout-test-only'
  for i in range(2):api(clients[i],'/register',dict(name=names[i],password=password,invite='layout-accept-code'))
  def room(i=0):return api(clients[i],'/state')['room']
  def command(i,kind,**extra):
   r=room(i);return api(clients[i],'/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
  api(clients[0],'/rooms',dict(kind='splendor',capacity=2,name='回合布局验收'))
  r=room();api(clients[1],'/rooms/'+r['id'],dict(type='join',version=r['version'],nonce=uuid.uuid4().hex))
  command(0,'ready');command(1,'ready');command(0,'start');me=room()['you']
  assert js('fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(dict(name=names[0],password=password))+')}).then(r=>r.ok)')
  b('set','viewport',str(width),'900');b('reload');b('wait','.splendor-board');time.sleep(.2)
  issue(not js('!!document.querySelector("header button[aria-label=退出登录]")'),f'{width}: logout still exposed in header')
  js('''(()=>{window.bankFrames=[];window.bankSampling=true;const sample=()=>{if(!window.bankSampling)return;const box=s=>{const e=document.querySelector(s),r=e?.getBoundingClientRect();return r?{y:r.y+scrollY,h:r.height}:null};const actions=document.querySelector('.bank-actions');window.bankFrames.push({board:box('.splendor-board'),bank:box('.gem-bank'),market:box('.market'),wallet:box('.personal-area'),actions:box('.bank-actions'),visible:!!actions&&getComputedStyle(actions).visibility!=='hidden',hiddenSafe:!actions||!actions.inert||[...actions.querySelectorAll('button')].every(e=>e.disabled)});window.bankRAF=requestAnimationFrame(sample)};sample()})()''')
  for step in range(4):
   r=room();g=r['game'];actor=next(i for i in range(2) if room(i)['you']==g['turn'])
   take=[0]*6
   for color in sorted(range(5),key=lambda i:g['splendor']['bank'][i],reverse=True)[:3]:
    if g['splendor']['bank'][color]:take[color]=1
   if actor==0:
    for color,n in enumerate(take):
     if n:b('click',f'.token-button[data-bank-color="{color}"]')
    b('click','.bank-actions .primary')
   else:command(actor,'action',action=dict(type='take',tokens=take))
   b('wait','--fn','document.querySelector(".bank-actions") ? (getComputedStyle(document.querySelector(".bank-actions")).visibility !== "hidden") === '+json.dumps(g['turn']!=me)+' : '+json.dumps(g['turn']==me))
   time.sleep(.15)
   b('screenshot',str(out/f'{width}-step-{step}.png'))
  frames=js('window.bankSampling=false;cancelAnimationFrame(window.bankRAF);window.bankFrames')
  row={'width':width,'frames':frames,'ranges':{}}
  report['widths'].append(row)
  for item,prop in [('board','h'),('board','y'),('bank','h'),('market','y'),('wallet','y')]:
   values=[f[item][prop] for f in frames];extent=max(values)-min(values);row['ranges'][item+'.'+prop]=extent
   issue(extent<.5,f'{width}: {item}.{prop} changed by {extent}px')
  issue({f['visible'] for f in frames}=={True,False},f'{width}: both turns not sampled')
  issue(all(f['hiddenSafe'] for f in frames),f'{width}: hidden actions not disabled')
  click_profile();issue(js('!!document.querySelector(".modal button[aria-label=退出登录]")'),f'{width}: own profile lacks logout');b('screenshot',str(out/f'{width}-own-profile.png'));close_profile()
  issue(js('fetch("/api/state").then(r=>r.status)')==200,f'{width}: closing profile logged out')
  click_profile(False);issue(not js('!!document.querySelector(".modal button[aria-label=退出登录]")'),f'{width}: peer profile contains logout');close_profile()
  if not a.baseline and width==320:
   click_profile();b('click','.modal button[aria-label=退出登录]');b('wait','.auth-page')
   issue(js('fetch("/api/state").then(r=>r.status)')==401,'Logout did not clear session')
  command(0,'close');command(0,'leave');command(1,'leave')
 report['passed']=not report['issues'];assert not b('errors')['errors']
except Exception as e:
 report['error']=str(e)
 try:
  report['snapshot']=b('snapshot','-i');b('screenshot',str(out/'failure.png'))
 except Exception:pass
 raise
finally:
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));b('close');shutil.rmtree(out/'profile',ignore_errors=True);print(out/'result.json',flush=True)
