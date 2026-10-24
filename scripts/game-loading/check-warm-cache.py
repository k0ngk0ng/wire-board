#!/usr/bin/env python3
"""Two natural starts per game in the production app; verify warm request reuse."""
from pathlib import Path
import os,subprocess,json,uuid,time,shutil,urllib.request,socket,hashlib
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/game-loading/warm';TEMP=OUT/'temporary';TEMP.mkdir(parents=True,exist_ok=True)
workspace=ROOT.parent.parent if ROOT.parent.name=='.local' else ROOT
SOCKET=workspace/'.local/warm-s';SOCKET.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ,GOCACHE=str(workspace/'.local/go-build'),GOMODCACHE=str(workspace/'.local/go-mod'),GOTMPDIR=str(TEMP),TMPDIR=str(TEMP),GOTOOLCHAIN='local',AGENT_BROWSER_SESSION='warm-cache',AGENT_BROWSER_SOCKET_DIR=str(SOCKET),AGENT_BROWSER_PROFILE=str(TEMP/'profile'),INVITE_CODE='warm-cache-local',DATA_DIR=str(TEMP/'data'))
sources={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [ROOT/'web/src/game-api/load-assets.ts',ROOT/'web/src/game-api/load-cache.ts',ROOT/'web/src/game-api/loading.tsx']}
report={'sourceSHA256':sources,'passed':False,'scope':'production application, normal local accounts and real room starts; same browser document','games':[]};process=None;opened=False

def b(*args):
 p=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=ENV,capture_output=True,text=True,check=True,timeout=40);d=json.loads(p.stdout);assert d['success'],d;return d['data']
def js(s):return b('eval',s)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,body:await r.json()}})()');assert r['status'] in (200,201),r;return r['body']
def room():return api('/state')['room']
def cmd(kind,**kw):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**kw))
def wait(fn,seconds=35):
 until=time.monotonic()+seconds
 while time.monotonic()<until:
  if fn():return
  time.sleep(.1)
 raise AssertionError('condition timed out')
try:
 subprocess.run(['go','build','-o',str(TEMP/'app'),'.'],cwd=ROOT,env=ENV,check=True,timeout=180)
 with socket.socket() as probe:probe.bind(('127.0.0.1',0));port=probe.getsockname()[1]
 ENV['ADDR']='127.0.0.1:'+str(port);url='http://'+ENV['ADDR'];log=(TEMP/'server.log').open('w');process=subprocess.Popen([str(TEMP/'app')],cwd=ROOT,env=ENV,stdout=log,stderr=subprocess.STDOUT)
 def health():
  try:return urllib.request.urlopen(url+'/healthz',timeout=1).status==200
  except:return False
 wait(health)
 with urllib.request.urlopen(url+'/game-load-manifest.json') as r:manifest=json.load(r)
 b('open',url);opened=True;api('/register',{'name':'缓存验收'+uuid.uuid4().hex[:6],'password':'cache-local-test-only','invite':ENV['INVITE_CODE']});b('reload');b('wait','--fn','document.body.innerText.includes("创建房间")')
 for kind in os.environ.get('WARM_GAMES','gomoku,conquest').split(','):
  files=[x['url'] for x in manifest['games'][kind]];runs=[]
  for attempt in range(2):
   js('performance.clearResourceTimings();window.__loadLabels=[];window.__loadObserver?.disconnect();window.__loadObserver=new MutationObserver(()=>{const p=document.querySelector(".game-loading [role=status]");if(p&&!window.__loadLabels.includes(p.textContent))window.__loadLabels.push(p.textContent)});window.__loadObserver.observe(document.body,{subtree:true,childList:true,characterData:true})')
   api('/rooms',{'kind':kind,'name':'缓存验收 '+kind,'capacity':2});cmd('add_bot');cmd('ready');cmd('start')
   wait(lambda:not room().get('loading'));wait(lambda:not js('!!document.querySelector("[data-game-loading]")'))
   resources=js('performance.getEntriesByType("resource").map(r=>({name:r.name,initiatorType:r.initiatorType,transferSize:r.transferSize,decodedBodySize:r.decodedBodySize}))');programs=[r for r in resources if any(r['name'].endswith(f) for f in files)];labels=js('window.__loadLabels');r=room();assert r['status'] in ('playing','finished')
   if attempt==0:assert programs,(kind,'cold program preparation absent')
   else:assert not programs,(kind,'warm program was re-read',programs)
   runs.append({'matchId':r.get('matchId'),'programRequests':programs,'labels':labels,'loadingGateCompleted':not r.get('loading')})
   b('screenshot',OUT/(kind+('-warm.png' if attempt else '-cold.png')));cmd('close');cmd('leave');wait(lambda:js('document.body.innerText.includes("创建房间")'))
  assert runs[0]['matchId']!=runs[1]['matchId'];report['games'].append({'kind':kind,'runs':runs});print('PASS cold/warm',kind,flush=True)
 assert all(hashlib.sha256((ROOT/p).read_bytes()).hexdigest()==h for p,h in sources.items())
 report['build']=manifest['build'];report['passed']=True
finally:
 if opened:
  try:b('close')
  except:pass
 if process:
  process.terminate();process.wait(timeout=10)
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 shutil.rmtree(TEMP,ignore_errors=True)
 for p in SOCKET.glob('warm-cache.*'):p.unlink()
 if not any(SOCKET.iterdir()):SOCKET.rmdir()
