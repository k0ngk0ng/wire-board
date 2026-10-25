#!/usr/bin/env python3
"""Real table-tennis rooms, production UI and native mouse/touch input."""
from pathlib import Path
import os,subprocess,json,time,uuid,shutil,hashlib
ROOT=Path(__file__).resolve().parents[3];HERE=Path(__file__).resolve().parent;OUT=ROOT/'.local/tabletennis/acceptance'/os.environ.get('TT_BROWSER_REPORT','current');FIXTURE=ROOT/'.local/tabletennis/browser';workspace=ROOT.parent.parent if ROOT.parent.name=='.local' else ROOT;SOCKET=workspace/'.local/tt-live'
for p in [OUT,FIXTURE,SOCKET,OUT/'tmp']:p.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ,AGENT_BROWSER_SESSION='tt-live',AGENT_BROWSER_SOCKET_DIR=str(SOCKET),AGENT_BROWSER_PROFILE=str(OUT/'profile'),TMPDIR=str(OUT/'tmp'),GOTMPDIR=str(OUT/'tmp'),GOCACHE=str(workspace/'.local/go-build'),GOMODCACHE=str(workspace/'.local/go-mod'),GOTOOLCHAIN='local',WIRE_BOARD_TABLETENNIS_BROWSER='1')
report={'passed':False,'scope':'production module, normal local rooms, native Chromium mouse/touch; mobile viewport simulation','cases':[]};opened=False;server=None;host_opened=False
seat=int(os.environ.get('TT_BROWSER_SEAT','0'))
sources=sorted((ROOT/'games/tabletennis').rglob('*'))+sorted((ROOT/'web/dist').rglob('*'))
report['sha256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sources if p.is_file()}

def b(*a):
 r=subprocess.run(['agent-browser',*map(str,a),'--json'],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr;j=json.loads(r.stdout);assert j['success'],j;return j['data']
def js(s):return b('eval',s)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,body:await r.json()}})()');assert r['status'] in (200,201),r;return r['body']
def room():return api('/state')['room']
def hb(*a):
 env=dict(ENV,AGENT_BROWSER_SESSION='tt-peer',AGENT_BROWSER_PROFILE=str(OUT/'peer-profile'))
 r=subprocess.run(['agent-browser',*map(str,a),'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr;j=json.loads(r.stdout);assert j['success'],j;return j['data']
def hapi(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=hb('eval','(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,body:await r.json()}})()')['result'];assert r['status'] in (200,201),r;return r['body']
def hcmd(kind,**kw):
 r=hapi('/state')['room'];return hapi('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**kw))
def cmd(kind,**kw):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**kw))
def wait(fn,seconds=30):
 until=time.monotonic()+seconds
 while time.monotonic()<until:
  if fn():return
  time.sleep(.1)
 raise AssertionError('condition timed out')
def visual_audit():
 return js('''(()=>{const rgb=s=>(s.match(/[\\d.]+/g)||[]).slice(0,3).map(Number),lum=c=>c.map(x=>{x/=255;return x<=.04045?x/12.92:((x+.055)/1.055)**2.4}).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll('.tt-score strong,.tt-score small,.tt-score-center b,.tt-status span,.tt-status b')].map(e=>{let p=e,bg;while(p){bg=getComputedStyle(p).backgroundColor;if(bg!=='rgba(0, 0, 0, 0)'&&bg!=='transparent')break;p=p.parentElement;}const fg=getComputedStyle(e).color,a=lum(rgb(fg)),b=lum(rgb(bg));const range=document.createRange();range.selectNodeContents(e);return {text:e.textContent,foreground:fg,background:bg,contrast:(Math.max(a,b)+.05)/(Math.min(a,b)+.05),rect:range.getBoundingClientRect().toJSON()};});})()''')
try:
 for n in ['url','stop','network-delay']:(FIXTURE/n).unlink(missing_ok=True)
 log=(OUT/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=31m','-run','^TestTableTennisBrowserFixture$','./games/tabletennis/tests'],cwd=ROOT,env=ENV,stdout=log,stderr=subprocess.STDOUT)
 wait(lambda:(FIXTURE/'url').exists(),180);BASE=(FIXTURE/'url').read_text().strip();b('open',BASE);opened=True
 api('/register',{'name':'乒乓验收'+uuid.uuid4().hex[:6],'password':'tabletennis-local-only','invite':'tabletennis-browser'});b('reload');b('wait','--fn','document.body.innerText.includes("创建房间")')
 if seat:
  hb('open',BASE);host_opened=True;hapi('/register',{'name':'双打同场'+uuid.uuid4().hex[:6],'password':'tabletennis-local-only','invite':'tabletennis-browser'});hb('reload');hb('wait','--fn','document.body.innerText.includes("创建房间")')
 for court,n,width,height in [('club',2,1440,1000),('arena',4,390,844),('heritage',4,320,760)]:
  if os.environ.get('TT_BROWSER_COURT') and os.environ['TT_BROWSER_COURT']!=court:continue
  if seat>=n:continue
  b('set','viewport',width,height);creator=hapi if seat else api;hostcmd=hcmd if seat else cmd
  creator('/rooms',{'kind':'tabletennis','name':'乒乓实战验收','capacity':n,'options':{'court':court,'difficulty':'easy','bestOf':1}})
  if seat:
   for _ in range(seat-1):hostcmd('add_bot')
   r=hapi('/state')['room'];api('/rooms/'+r['id'],dict(type='join',version=r['version'],nonce=uuid.uuid4().hex));hostcmd('ready')
  for _ in range(n-1-seat if seat else n-1):hostcmd('add_bot')
  cmd('ready');(FIXTURE/'network-delay').write_text(os.environ.get('TT_BROWSER_NETWORK',os.environ.get('TT_BROWSER_LATENCY','0')));time.sleep(.06);hostcmd('start');wait(lambda:not room().get('loading'));wait(lambda:js('document.querySelector("[data-tabletennis-board]")?.dataset.ready==="true"'))
  assert room()['you']==seat
  if seat:hostcmd('autoplay',enabled=True)
  time.sleep(.25)
  if width<650:wait(lambda:js('(()=>{const r=document.querySelector(".tt-scene canvas").getBoundingClientRect();return r.top>=0&&r.bottom<=innerHeight+2})()'),3)
  entry=js('({height:innerHeight,canvas:document.querySelector(".tt-scene canvas").getBoundingClientRect().toJSON()})')
  if width<650:assert entry['canvas']['top']>=0 and entry['canvas']['bottom']<=entry['height']+2,entry
  b('scrollintoview','.tt-board');meta=js('({width:innerWidth,overflow:document.documentElement.scrollWidth>innerWidth,buttons:document.querySelectorAll(".tt-board button").length,frame:document.querySelector(".tt-scene canvas").__tableTennisFrame,rect:document.querySelector(".tt-scene canvas").getBoundingClientRect().toJSON()})');assert not meta['overflow'] and meta['buttons']==0 and meta['frame']['rackets']==n,meta
  if os.environ.get('TT_BROWSER_HUD'):
   wait(lambda:js('document.querySelector(".tt-scene canvas").__tableTennisFrame.phase==="serve"&&document.querySelector(".tt-scene canvas").__tableTennisFrame.inputEnabled'))
   wait(lambda:js('!!document.querySelector("[data-tabletennis-serve-clock]")'))
   clock_geometry=js('(()=>{const e=document.querySelector("[data-tabletennis-serve-clock]"),r=document.createRange();r.selectNodeContents(e);return {rect:r.getBoundingClientRect().toJSON(),text:e.textContent,width:innerWidth,height:innerHeight};})()');cr=clock_geometry['rect'];assert cr['width']>0 and cr['left']>=0 and cr['right']<=clock_geometry['width'] and cr['top']>=0 and cr['bottom']<=clock_geometry['height'],clock_geometry
   meta['serveClock']=clock_geometry;meta['serveAudit']=visual_audit();assert all(v['contrast']>=4.5 for v in meta['serveAudit']);b('screenshot',OUT/(court+'-serve-clock.png'))
  ws=b('get','cdp-url');ws=ws if isinstance(ws,str) else ws.get('url',ws.get('cdpUrl',ws.get('value')))
  native=[]
  report['currentCase']={'court':court,'metadata':meta}
  peer_motion=None
  if os.environ.get('TT_BROWSER_PEER_MOTION'):
   assert seat,'peer movement requires a second real browser'
   hostcmd('autoplay',enabled=False);hb('set','viewport',1440,1000);hb('scrollintoview','.tt-board');time.sleep(.2)
   js('window.__ttPeerFrames=[];window.__ttPeerSampling=true;(function collect(){if(!window.__ttPeerSampling)return;window.__ttPeerFrames.push(document.querySelector(".tt-scene canvas").__tableTennisFrame);requestAnimationFrame(collect)})();true')
   peer_ws=hb('get','cdp-url');peer_ws=peer_ws if isinstance(peer_ws,str) else peer_ws.get('url',peer_ws.get('cdpUrl',peer_ws.get('value')))
   r=subprocess.run(['node',str(HERE/'native-follow.mjs'),peer_ws,BASE,'{}'],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=40);assert r.returncode==0,r.stderr+r.stdout
   time.sleep(.2);samples=js('window.__ttPeerSampling=false;window.__ttPeerFrames');positions=[next(p for p in f['remoteRackets'] if p['seat']==0) for f in samples]
   assert len({p['motionSeq'] for p in positions})>=5,positions
   assert max(p['x'] for p in positions)-min(p['x'] for p in positions)>20 and max(p['y'] for p in positions)-min(p['y'] for p in positions)>20
   peer_motion={'sender':json.loads(r.stdout),'observerFrames':samples};b('screenshot',OUT/(court+'-peer-motion.png'));hostcmd('autoplay',enabled=True)
  if os.environ.get('TT_BROWSER_FOLLOW'):
   r=subprocess.run(['node',str(HERE/'native-follow.mjs'),ws,BASE,json.dumps({'touch':width<650})],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=40);assert r.returncode==0,r.stderr+r.stdout;native.append(json.loads(r.stdout))
  strokes=[{'kind':kind,'touch':width<650} for kind in ['drive','chop']]
  if os.environ.get('TT_BROWSER_SPEED'):strokes=[{'kind':'drive','touch':width<650,'serveOnly':True,'stepDelay':delay} for delay in [60,8]]
  if os.environ.get('TT_BROWSER_LOW'):strokes=[{'kind':'drive','touch':width<650} for _ in range(6)]
  for stroke_options in strokes:
   r=subprocess.run(['node',str(HERE/'native-strike.mjs'),ws,BASE,json.dumps(stroke_options)],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=40)
   if r.returncode:raise RuntimeError(r.stderr+r.stdout)
   native.append(json.loads(r.stdout))
  if os.environ.get('TT_BROWSER_LONG'):
   r=subprocess.run(['node',str(HERE/'native-long.mjs'),ws,BASE,json.dumps({'touch':width<650})],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=85);assert r.returncode==0,r.stderr+r.stdout;native.append(json.loads(r.stdout))
  if os.environ.get('TT_BROWSER_SPEED'):
   hits=[v for v in native if 'stroke' in v];assert hits[1]['stroke']['power']>hits[0]['stroke']['power']+.1,hits
   assert hits[1]['swingDuration']<hits[0]['swingDuration'],hits
  if os.environ.get('TT_BROWSER_LOW'):
   low=[v for v in native if v.get('before',{}).get('phase')=='rally' and v.get('launch') and v['launch']['initial']['y']<1.05 and v['launch']['receiveAt']>0];assert low,'no native low-ball return reached the opposite table'
  if os.environ.get('TT_BROWSER_MOVEMENT'):
   r=subprocess.run(['node',str(HERE/'native-movement.mjs'),ws,BASE,str(width<650).lower()],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=40);assert r.returncode==0,r.stderr+r.stdout;native.append(json.loads(r.stdout))
  b('screenshot',OUT/(court+'-'+str(width)+'.png'))
  assert not js('!!document.querySelector("button[aria-label=关闭错误]")'),'unexpected platform action error'
  audit=visual_audit();assert all(v['contrast']>=4.5 for v in audit),audit
  (FIXTURE/'network-delay').write_text('0');time.sleep(.3)
  js('window.__ttFullscreenCanvas=document.querySelector(".tt-scene canvas");true');b('click','button[aria-label="全屏"]');wait(lambda:js('!!document.querySelector("[data-room-fullscreen]")'));full=js('({rect:document.querySelector(".tt-board").getBoundingClientRect().toJSON(),canvas:document.querySelector(".tt-scene canvas").getBoundingClientRect().toJSON(),height:innerHeight,sameCanvas:window.__ttFullscreenCanvas===document.querySelector(".tt-scene canvas")})');b('screenshot',OUT/(court+'-fullscreen.png'));assert full['sameCanvas'] and full['canvas']['height']>200 and full['rect']['bottom']<=full['height']+2,full
  if os.environ.get('TT_BROWSER_FOLLOW'):
   r=subprocess.run(['node',str(HERE/'native-follow.mjs'),ws,BASE,json.dumps({'touch':width<650})],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=40);assert r.returncode==0,r.stderr+r.stdout;full['follow']=json.loads(r.stdout)
  if width>=650:
   r=subprocess.run(['node',str(HERE/'native-strike.mjs'),ws,BASE,json.dumps({'kind':'drive','keyboard':True})],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=40)
   if r.returncode:raise RuntimeError(r.stderr+r.stdout)
   native.append(json.loads(r.stdout))
  b('click','button[aria-label="退出全屏"]');wait(lambda:not js('!!document.querySelector("[data-room-fullscreen]")'));report['cases'].append({'court':court,'players':n,'viewport':[width,height],'metadata':meta,'entry':entry,'native':native,'fullscreen':full,'networkDelayMS':int(os.environ.get('TT_BROWSER_LATENCY','0'))})
  if os.environ.get('TT_BROWSER_RECOVERY'):
   before=room()['game']['data'];(FIXTURE/'network-delay').write_text('{"offline":true}');time.sleep(1.4)
   stale=js('document.querySelector(".tt-scene canvas").__tableTennisFrame');assert not stale['inputEnabled'] and not stale['gestureDown'],stale
   (FIXTURE/'network-delay').write_text('0')
   def reconnected():
    try:return room()['game']['data']['serverAt']>before['serverAt']
    except AssertionError as e:
     if 'Failed to fetch' not in str(e):raise
     return False
   wait(reconnected);after=room();assert after['game']['data']['epoch']==before['epoch'] and not after['seats'][seat].get('autoPlay'),after
   b('reload');wait(lambda:js('document.querySelector("[data-tabletennis-board]")?.dataset.ready==="true"'));assert not room().get('loading')
   r=subprocess.run(['node',str(HERE/'native-strike.mjs'),ws,BASE,json.dumps({'kind':'drive','touch':width<650})],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=40);assert r.returncode==0,r.stderr+r.stdout
   report['cases'][-1]['recovery']={'stale':stale,'epochPreserved':True,'autoplay':False,'reloadDoesNotPause':True,'native':json.loads(r.stdout)}
  if os.environ.get('TT_BROWSER_FINISH'):
   cmd('autoplay',enabled=True);wait(lambda:room()['game']['data']['phase']=='finished',480)
   wait(lambda:js('!!document.querySelector("[role=dialog]")'));b('screenshot',OUT/(court+'-result.png'))
   report['cases'][-1]['naturalResult']=room()['result']
   b('click','[role="dialog"] button[aria-label="关闭"]');time.sleep(.4);assert not js('!!document.querySelector("[role=dialog]")')
   b('click','button.result-reopen');wait(lambda:js('!!document.querySelector("[role=dialog]")'));report['cases'][-1]['resultReopened']=True
   b('click','[role="dialog"] button[aria-label="关闭"]')
  if os.environ.get('TT_BROWSER_SERVICE'):
   cmd('autoplay',enabled=True);serves={};initial=room()['game']['data'];target=sum(initial['score'])+6
   def service_progress():
    state=room()['game']['data'];total=sum(state['score'])
    if state['phase']=='serve':
     turn=10+total-20 if min(state['score'])>=10 else total//2
     assert state['expected']==state['order'][turn%len(state['order'])],state
     serves[total]={'total':total,'score':state['score'],'server':state['expected'],'order':state['order'],'ballWithinService':1 if min(state['score'])>=10 else total%2+1}
    return total>=target
   wait(service_progress,100);assert len(serves)>=4,serves;report['cases'][-1]['serviceRotation']=list(serves.values())
  report['cases'][-1]['seat']=seat
  if peer_motion:report['cases'][-1]['peerMotion']=peer_motion
  report['cases'][-1]['visualAudit']=audit
  js('window.__retiredTTCourt=document.querySelector(".tt-scene canvas");true')
  if room()['status']=='playing':hostcmd('close')
  cmd('leave')
  if seat:hostcmd('leave')
  wait(lambda:js('document.body.innerText.includes("创建房间")'));wait(lambda:js('window.__retiredTTCourt.getContext("webgl2").isContextLost()'));report['cases'][-1]['contextReleased']=True
 report['passed']=True
finally:
 if opened and not report['passed']:
  try:report['page']=js('document.body.innerText');report['failureGeometry']=js('({scrollY,viewport:[innerWidth,innerHeight],canvas:document.querySelector(".tt-scene canvas")?.getBoundingClientRect().toJSON()})');b('screenshot',OUT/'failure.png','--full')
  except:pass
 if opened:
  try:b('close')
  except:pass
 if host_opened:
  try:hb('close')
  except:pass
 if server:
  (FIXTURE/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 for n in ['url','stop','network-delay']:(FIXTURE/n).unlink(missing_ok=True)
 shutil.rmtree(OUT/'profile',ignore_errors=True)
 shutil.rmtree(OUT/'peer-profile',ignore_errors=True)
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'cases':len(report['cases'])}))
