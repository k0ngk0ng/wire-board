#!/usr/bin/env python3
"""Targeted real UI feedback checks using explicitly labelled starting hands.
Exercises response ranking, preserving a held bomb, cycling, turn reset,
reload reset and actually playing the recommended pair through the UI.
No production debug routes; only the opt-in Go test fixture changes starting hands.
"""
import json,os,subprocess,time,uuid,urllib.request,http.cookiejar,shutil
from pathlib import Path
root=Path(__file__).resolve().parents[2];layout=os.environ.get('POKER_HINT_LAYOUT')=='1';out=root/('.local/poker-layout/live' if layout else '.local/poker-hints/browser');fixture=root/'.local/poker/browser';out.mkdir(parents=True,exist_ok=True);fixture.mkdir(parents=True,exist_ok=True)
env=dict(os.environ)
for key,rel in {'AGENT_BROWSER_SOCKET_DIR':'.local/ab-hints','AGENT_BROWSER_PROFILE':'.local/poker-hints/profile','TMPDIR':'.local/tmp','GOTMPDIR':'.local/tmp','GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','npm_config_cache':'.local/npm'}.items():
 p=root/rel;p.mkdir(parents=True,exist_ok=True);env[key]=str(p)
env.update(WIRE_BOARD_POKER_BROWSER='1',WIRE_BOARD_POKER_HINTS='1',AGENT_BROWSER_SESSION='pf-'+uuid.uuid4().hex[:6],AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',GOTOOLCHAIN='local')
if layout:
 env['AGENT_BROWSER_PROFILE']=str(out/'temporary/profile');env['TMPDIR']=str(out/'temporary/tmp');Path(env['TMPDIR']).mkdir(parents=True,exist_ok=True)
report={'passed':False,'scope':'labelled deterministic opening hands; real UI/HTTP and production actions','checks':[]};opened=False;server=None;handle=None

def b(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=40)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return b('eval',code)['result']
def api(path,body=None):
 opt='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('fetch('+json.dumps('/api'+path)+opt+').then(async r=>({status:r.status,body:await r.json()}))');assert 200<=r['status']<300,r;return r['body']
def command(kind):
 r=api('/state')['room'];return api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
def track_layout():
 js('''(()=>{window.__layoutRange={};window.__layoutEpoch=(window.__layoutEpoch??0)+1;const epoch=window.__layoutEpoch;function trackPoker(){if(epoch!==window.__layoutEpoch)return;for(const s of ['.poker-board','.poker-arena','.poker-actions','.poker-hand']){const r=document.querySelector(s)?.getBoundingClientRect();if(!r)continue;for(const [k,v] of Object.entries({x:r.x+scrollX,y:r.y+scrollY,width:r.width,height:r.height})){const key=s+k,a=window.__layoutRange[key]??[v,v];window.__layoutRange[key]=[Math.min(a[0],v),Math.max(a[1],v)];}}requestAnimationFrame(trackPoker)};trackPoker();})()''')
def shot(name):
 if layout:
  geometry=js('window.__layoutRange');report.setdefault('geometry',[]).append({'name':name,'range':geometry});assert geometry and all(v[1]-v[0]<.75 for v in geometry.values()),geometry
 b('wait','--fn','[...document.querySelectorAll(".poker-new-play")].every(e=>e.getAnimations().every(a=>a.playState!=="running"))')
 b('wait','--fn','[...document.querySelectorAll(".poker-card img")].every(e=>e.complete&&e.naturalWidth>0)')
 assert not b('errors')['errors'],b('errors');assert js('document.documentElement.scrollWidth<=innerWidth+2'),name
 assert js('document.querySelectorAll(".poker-index").length')==0
 assert js('document.querySelectorAll(".poker-card-mark").length')==0,'card corners must not carry Chinese badges'
 assert js('[...document.querySelectorAll(".poker-card[data-card-face] img")].filter(e=>+e.parentElement.dataset.cardFace>=0).every(e=>e.src.includes("/poker/v4/"))'),'latest integrated faces required'
 # Every played row must show each rank's exposed corner, including wrapped rows.
 b('scrollintoview','.poker-arena')
 hits=js('''[...document.querySelectorAll('.poker-play .poker-strip-slot')].map(e=>{const r=e.getBoundingClientRect(),x=r.left+6,y=r.top+10;return {id:e.querySelector('[data-card-face]').dataset.cardFace,hit:e.contains(document.elementFromPoint(x,y)),x,y}})''')
 assert all(h['hit'] for h in hits),hits
 b('screenshot',str(out/(name+'.png')),'--full');report['checks'].append(name);print('PASS',name,flush=True)
try:
 for name in ['url','stop']:(fixture/name).unlink(missing_ok=True)
 with (out/'build.log').open('w') as f:subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT,check=True)
 handle=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=21m','-run','^TestPokerBrowserFixture$','./internal/gametest'],cwd=root,env=env,stdout=handle,stderr=subprocess.STDOUT)
 start=time.monotonic()
 while not (fixture/'url').exists():
  if server.poll() is not None or time.monotonic()-start>90:raise RuntimeError('fixture failed')
  time.sleep(.1)
 base=(fixture/'url').read_text();b('open',base);opened=True;api('/login',{'name':'扑克验收管理员','password':'poker-browser-test-only'})
 for game in ['doudizhu','guandan']:
  api('/admin/games/'+game,{'enabled':True});r=api('/rooms',{'kind':game,'name':'牌面与报牌验收','capacity':4,'options':{'turnSeconds':120}});peers=[]
  for p in range(1,4):
   jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
   def req(path,body=None,client=client):
    request=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
    with client.open(request,timeout=20) as response:return json.load(response)
   req('/register',{'name':game+'反馈玩家'+str(p),'password':'poker-browser-test-only','invite':'poker-browser'})
   req('/rooms/'+r['id'],{'type':'join','version':api('/state')['room']['version'],'nonce':uuid.uuid4().hex})
   rr=req('/state')['room'];req('/rooms/'+r['id'],{'type':'ready','version':rr['version'],'nonce':uuid.uuid4().hex});peers.append(req)
  command('ready');command('start')
  gate=api('/state')['room'].get('loading')
  if gate:
   for index,req in enumerate(peers):
    for kind in ['load_begin','load_ready']:
     rr=req('/state')['room'];req('/rooms/'+rr['id'],{'type':kind,'version':rr['version'],'nonce':uuid.uuid4().hex,'loadToken':gate['token'],'loadBuild':gate['build'],'loadClient':'hint-peer-'+str(index)})
  b('reload');b('wait','.poker-board');b('wait','--fn','!document.querySelector("[data-game-loading]")');b('set','viewport','320','900')
  if js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true"'):b('find','role','button','click','--name','取消静音','--exact')
  if layout:track_layout()
  initial=api('/state')['room'];assert sorted(initial['game']['data']['hand'])==[6,7,19,20,32,45]
  def hint(expected):
   b('find','role','button','click','--name','提示','--exact')
   picked=js('[...document.querySelectorAll(".poker-hand [aria-pressed=true]")].map(e=>+e.dataset.card).sort((a,b)=>a-b)')
   assert picked==sorted(expected),(game,picked,expected)
  hint([7,20]);shot(game+'-pair9-first')
  hint([6,19]);hint([6,19,32,45]);hint([7,20]);hint([6,19])
  assert api('/state')['room']['version']==initial['version'],'hint submitted an action'
  # Pass a complete circle, then have the opponent lead pair 6 in a new window.
  b('find','role','button','click','--name','不出','--exact')
  for req in peers[:2]:
   rr=req('/state')['room'];req('/rooms/'+rr['id'],{'type':'action','version':rr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'pass','window':rr['game']['data']['window']}})
  req=peers[2];rr=req('/state')['room'];req('/rooms/'+rr['id'],{'type':'action','version':rr['version'],'nonce':uuid.uuid4().hex,'action':{'type':'play','cards':[4,17],'window':rr['game']['data']['window']}})
  b('wait','--fn','!document.querySelector(".poker-actions button").disabled')
  hint([7,20]);shot(game+'-new-turn-pair9')
  hint([6,19]);
  if layout:js('window.__layoutEpoch++')
  b('reload');b('wait','.poker-board');b('wait','--fn','!document.querySelector("[data-game-loading]")');hint([7,20])
  if layout:track_layout()
  shot(game+'-reload-pair9')
  b('find','role','button','click','--name','出牌（2）','--exact')
  b('wait','--fn','document.querySelectorAll(".poker-hand button[data-card]").length===4')
  after=api('/state')['room']['game']['data'];assert sorted(after['hand'])==[6,19,32,45]
  assert sorted(after['lastPlay']['cards'])==[7,20]
  if layout:
   shot(game+'-played-kept-bomb');js('window.__layoutEpoch++')
  b('set','viewport','1280','900')
  if layout:
   js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))');track_layout()
  shot(game+'-desktop-played-kept-bomb')
  command('close');command('leave');b('wait','.room-list')
 report['passed']=True
except Exception as e:
 import traceback;traceback.print_exc();report['error']=repr(e)
 if opened:
  try:b('screenshot',str(out/'failure.png'),'--full');report['snapshot']=b('snapshot','-i');report['state']=api('/state')
  except Exception:pass
finally:
 if opened:
  try:b('close')
  except Exception:pass
 if server:
  (fixture/'stop').write_text('stop')
  try:server.wait(timeout=15)
  except subprocess.TimeoutExpired:server.terminate();server.wait(timeout=10)
 if handle:handle.close()
 if layout:shutil.rmtree(out/'temporary',ignore_errors=True)
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('Artifact:',out/'result.json',flush=True)
 if not report['passed']:raise SystemExit(1)
