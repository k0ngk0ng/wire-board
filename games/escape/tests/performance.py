#!/usr/bin/env python3
"""Real fast/slow peers, native keyboard/touch, visible draw and wire artifacts."""
import concurrent.futures
import json
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid
from PIL import Image, ImageChops

ROOT=Path(__file__).resolve().parents[3]
COMMON=Path(subprocess.check_output(['git','rev-parse','--git-common-dir'],cwd=ROOT,text=True).strip())
SOCKETS=(COMMON if COMMON.is_absolute() else ROOT/COMMON).resolve().parent/'.local/escape-ab'
OUT=ROOT/'.local/escape/performance';OUT.mkdir(parents=True,exist_ok=True)
BASE=(ROOT/'.local/escape/browser/url').read_text().strip()
class Browser:
 def __init__(self,name,rtt=0):
  self.name=name;self.env=dict(os.environ,AGENT_BROWSER_SESSION='en'+uuid.uuid4().hex[:5],AGENT_BROWSER_SOCKET_DIR=str(SOCKETS),AGENT_BROWSER_PROFILE=str(OUT/(name+'-profile')),TMPDIR=str(OUT/'tmp'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
  for key in ['AGENT_BROWSER_PROFILE','TMPDIR','AGENT_BROWSER_SOCKET_DIR']:Path(self.env[key]).mkdir(parents=True,exist_ok=True)
  self.b('open',BASE)
  if rtt:self.b('set','headers',json.dumps({'X-Escape-Test-RTT':str(rtt)}))
 def b(self,*args):
  r=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=self.env,text=True,capture_output=True,timeout=50);assert r.returncode==0,r.stdout+r.stderr;d=json.loads(r.stdout);assert d['success'],d;return d['data']
 def js(self,code):return self.b('eval',code)['result']
 def api(self,path,body=None):
  opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
  r=self.js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()');assert 200<=r['status']<300,r;return r['data']
 def room(self):return self.api('/state').get('room')
 def command(self,kind,**extra):
  r=self.room();return self.api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
 def measure(self,name,mode):
  target=OUT/name;target.mkdir(exist_ok=True)
  r=subprocess.run(['node','games/escape/tests/native-input.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,json.dumps({'mode':mode,'out':str(target),'samples':5})],cwd=ROOT,env=self.env,text=True,capture_output=True,timeout=50);assert r.returncode==0,r.stdout+r.stderr
  d=json.loads(r.stdout);gaps=sorted(d['frame']['gaps']);d['drawFPS']=1000/(sum(gaps)/len(gaps));d['visibleFPS']=d['presentation']['fps'];displayed=sorted(d['presentation']['gaps']);d['gapP95']=displayed[min(len(displayed)-1,int(len(displayed)*.95))]
  if mode not in ('observe','gamepad'):
   assert len(d['native'])==(2 if mode=='reconnect' else 5) and all(x.get('trusted') and not x.get('failed') and 'latencyMs'in x for x in d['native']),d['native']
   latencies=sorted(x['presentedLatencyMs'] for x in d['native']);d['latencyP95']=latencies[-1];d['drawLatencyP95']=max(x['latencyMs'] for x in d['native']);assert d['latencyP95']<=80,d['native']
   before,after=Image.open(target/'before.png').convert('RGB'),Image.open(target/'after.png').convert('RGB');assert ImageChops.difference(before,after).getbbox(),'native action produced no visible pixels'
   if mode=='reconnect':
    fault=d['reconnect'];assert fault['closed']>=1 and d['connectionClosures']>=1
    quiet=[x for x in d['inputs'] if fault['bootstrapAt']<x['at']<fault['freshAt']]
    assert not any(x['packet']['action'].get('dx') or x['packet']['action'].get('dy') for x in quiet),'old held input replayed after recovery'
    assert sum(x is not None for x in d['ackMs'])>=2
   else:assert d['inputPackets']>=10 and all(x is not None for x in d['ackMs']),d['ackMs']
  if mode=='gamepad':
   assert d['virtualGamepad']['after']['serverX']>d['virtualGamepad']['before']['serverX']+5,'virtual standard gamepad did not move authority'
   assert any(0<abs(x['packet']['action'].get('dx',0))<.99 for x in d['inputs']),'analog axis was flattened to a digital key'
   assert not d['uiError'],'analog jitter overflowed input queue'
   assert d['inputs'][-1]['packet']['action'].get('dx')==0,'gamepad neutral did not release'
  assert d['visibleFPS']>=50,d['visibleFPS'];assert not d['overflow'];assert d['frameBytes']['count']>=10
  assert d['frameBytes']['median']<6000,d['frameBytes']
  return d
 def close(self):
  try:self.b('close')
  finally:shutil.rmtree(OUT/(self.name+'-profile'),ignore_errors=True)

def wait(fn,seconds=30):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if fn():return
  time.sleep(.1)
 raise AssertionError('production condition timed out')

report={'passed':False,'command':'.local/image-tools/bin/python -B games/escape/tests/performance.py','scope':'two authenticated humans, real WebSockets, ordered per-client 400ms TCP RTT, native Chromium keyboard/touch and actual Canvas draws','cases':{}}
report['sourceSHA256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [*(ROOT/'games/escape/web').glob('*'),ROOT/'games/escape/controls.json',ROOT/'games/escape/assets/manifest.json'] if p.is_file()}
report['distIndexSHA256']=hashlib.sha256((ROOT/'web/dist/index.html').read_bytes()).hexdigest()
fast=slow=None
try:
 fast=Browser('fast')
 try:fast.api('/register',{'name':'性能探索者','password':'escape-browser-password','invite':'escape-browser'})
 except AssertionError:fast.api('/login',{'name':'性能探索者','password':'escape-browser-password'})
 previous=fast.room()
 if previous:
  assert previous['kind']=='escape'
  if previous['status']=='playing':fast.command('close')
  fast.command('leave')
 slow=Browser('slow',400)
 try:slow.api('/register',{'name':'慢端探索者','password':'escape-browser-password','invite':'escape-browser'})
 except AssertionError:slow.api('/login',{'name':'慢端探索者','password':'escape-browser-password'})
 previous=slow.room()
 if previous:slow.command('leave')
 fast.b('set','viewport',1440,1000);slow.b('set','viewport',1280,1000)
 r=fast.api('/rooms',{'kind':'escape','name':'输入与慢连接验收','capacity':2,'options':{'scenario':'station','difficulty':'easy','seed':'performance-17'}})
 slow.api('/rooms/'+r['id'],{'type':'join','version':fast.room()['version'],'nonce':uuid.uuid4().hex})
 fast.command('ready');slow.command('ready');fast.command('start')
 fast.b('reload');slow.b('reload');fast.b('wait','[data-escape-board][data-ready="true"]');slow.b('wait','[data-escape-board][data-ready="true"]')
 wait(lambda:not fast.room().get('loading'))
 with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
  a=pool.submit(fast.measure,'fast-keyboard','keyboard');b=pool.submit(slow.measure,'slow-keyboard','keyboard')
  report['cases']['fast-keyboard']=a.result();report['cases']['slow-keyboard']=b.result()
 delayed=report['cases']['slow-keyboard'];assert max(delayed['ackMs'])>=350,'configured latency did not affect the real connection'
 assert any(abs(x['first']['x']-x['before']['x'])>.08 and abs(x['first']['serverX']-x['before']['serverX'])<.01 for x in delayed['native']),'slow client did not visibly predict before authority'
 slow.b('set','viewport',320,820)
 report['cases']['slow-touch']=slow.measure('slow-touch','touch')
 report['cases']['slow-virtual-gamepad']=slow.measure('slow-virtual-gamepad','gamepad')
 slow.b('set','viewport',1280,1000)
 with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
  a=pool.submit(fast.measure,'fast-during-peer-outage','observe');b=pool.submit(slow.measure,'slow-reconnect','reconnect')
  report['cases']['fast-during-peer-outage']=a.result();report['cases']['slow-reconnect']=b.result()
 healthy=report['cases']['fast-during-peer-outage'];assert healthy['connectionClosures']==0 and healthy['serverProgress']['to']-healthy['serverProgress']['from']>=3500,'slow connection fault stopped the healthy peer'
 fast.command('close');fast.command('leave');slow.command('leave')
 # Reach a real four-role room through ordinary roster/actions, then gather
 # computer partners rather than injecting actor positions for a render test.
 fast.b('set','viewport',1440,1000);slow.b('set','viewport',1280,1000)
 r=fast.api('/rooms',{'kind':'escape','name':'四人密集画面验收','capacity':4,'options':{'scenario':'station','difficulty':'easy','seed':'dense-11'}})
 slow.api('/rooms/'+r['id'],{'type':'join','version':fast.room()['version'],'nonce':uuid.uuid4().hex})
 fast.command('add_bot');fast.command('add_bot');fast.command('ready');slow.command('ready');fast.command('start')
 fast.b('reload');slow.b('reload');fast.b('wait','[data-escape-board][data-ready="true"]');slow.b('wait','[data-escape-board][data-ready="true"]');wait(lambda:not fast.room().get('loading'))
 data=fast.room()['game']['data'];fast.command('action',action={'type':'ping','epoch':data['epoch'],'seq':data['players'][0]['seq']+1})
 wait(lambda:all(p['room']=='r4' for p in fast.room()['game']['data']['players']),seconds=18)
 with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
  a=pool.submit(fast.measure,'dense-four-fast','observe');b=pool.submit(slow.measure,'dense-four-slow','keyboard')
  report['cases']['dense-four-fast']=a.result();report['cases']['dense-four-slow']=b.result()
 fast.command('close');fast.command('leave');slow.command('leave')
 report['passed']=True
finally:
 for b in [fast,slow]:
  if b:
   if not report['passed']:
    try:
     (OUT/(b.name+'-failure.json')).write_text(json.dumps({'page':b.js('document.body.innerText'),'errors':b.b('errors')},ensure_ascii=False,indent=2)+'\n')
     b.b('screenshot',OUT/(b.name+'-failure.png'),'--full')
    except Exception:pass
   try:b.close()
   except Exception:pass
 (OUT/'summary.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({'passed':report['passed'],'cases':{k:{x:v[x] for x in ['visibleFPS','gapP95','latencyP95','ackMs','frameBytes'] if x in v}for k,v in report['cases'].items()}},ensure_ascii=False))
