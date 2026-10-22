#!/usr/bin/env python3
"""See BROWSER_SCENARIOS.md; real full application, no private state injection."""
import json, os, subprocess, time, uuid, shutil, urllib.request, re
from pathlib import Path
root=Path(__file__).resolve().parents[2]
run='load'+uuid.uuid4().hex[:5]; out=root/'.local/game-loading'/run;temp=out/'temporary';temp.mkdir(parents=True)
sockets=root/'.s'/run;sockets.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SESSION=run,AGENT_BROWSER_SOCKET_DIR=str(sockets),AGENT_BROWSER_PROFILE=str(temp/'profile'),TMPDIR=str(temp),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
base=os.environ.get('LOADING_BASE','http://127.0.0.1:18189')
report={'passed':False,'games':[]};opened=False

def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],env=env,cwd=root,capture_output=True,text=True,timeout=90);assert p.returncode==0,p.stdout+p.stderr
 j=json.loads(p.stdout);assert j['success'],j;return j['data']
def js(s):return browser('eval',s)['result']
def api(p,b=None):
 opts='' if b is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(b)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+p)+opts+');return {status:r.status,body:await r.json()}})()');assert r['status'] in (200,201),r;return r['body']
def room():return api('/state').get('room')
def cmd(t,**extra):
 r=room();return api('/rooms/'+r['id'],{'type':t,'version':r['version'],'nonce':uuid.uuid4().hex,**extra})
def wait(fn,timeout=90):
 end=time.monotonic()+timeout
 while time.monotonic()<end:
  r=fn()
  if r:return r
  time.sleep(.2)
 raise AssertionError('condition timed out')
try:
 browser('open',base);opened=True
 wait(lambda:js('!!document.querySelector("input[type=password]")'))
 api('/register',{'name':'load'+uuid.uuid4().hex[:8],'password':'loading-test-123','invite':'test-invite'})
 browser('reload');wait(lambda:js('document.body.innerText.includes("创建房间")'))
 report['home']=js('performance.getEntriesByType("resource").map(x=>({url:x.name,bytes:x.decodedBodySize}))')
 metas=[{'id':p.parent.parent.name,'properties':['minPlayers: '+re.search(r'minPlayers: (\d+)',p.read_text())[1]]} for p in sorted((root/'games').glob('*/web/catalog.tsx'))]
 for index,meta in enumerate(metas):
  kind=meta['id']
  if os.environ.get('LOADING_GAMES') and kind not in os.environ['LOADING_GAMES'].split(','):continue
  count=int(next(x for x in meta['properties'] if x.startswith('minPlayers:')).split(':')[1])
  if kind=='catan':count=3
  api('/rooms',{'kind':kind,'name':'加载验收 '+kind,'capacity':count})
  for _ in range(count-1):cmd('add_bot')
  cmd('ready');cmd('start')
  wait(lambda:js('!!document.querySelector("[data-game-loading]")'),15)
  initial=room();assert not initial.get('turnDeadline'),kind
  wait(lambda:not room().get('loading'))
  wait(lambda:not js('!!document.querySelector("[data-game-loading]")'))
  r=room();assert r['status'] in ('playing','finished'),r
  assert js('document.documentElement.scrollWidth<=innerWidth+2'),kind
  browser('screenshot',str(out/(kind+'.png')))
  report['games'].append({'kind':kind,'matchId':r.get('matchId'),'offset':r.get('clockOffsetMs'),'startVersion':initial['version'],'version':r['version']})
  print('PASS',kind,flush=True)
  cmd('close');cmd('leave')
 report['passed']=True
finally:
 if opened and not report['passed']:
  try:
   report['failurePage']=js('document.body.innerText');browser('screenshot',str(out/'failure.png'),'--full')
  except:pass
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
 if opened:
  try:browser('close')
  except:pass
 shutil.rmtree(temp,ignore_errors=True);shutil.rmtree(sockets,ignore_errors=True)
