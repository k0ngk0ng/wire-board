#!/usr/bin/env python3
"""Production app: native admin sorting, peer RTT and quiet realtime recovery."""
from pathlib import Path
import asyncio,threading,subprocess,os,json,time,socket,uuid,shutil,urllib.request
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'.local/near-order/browser';TEMP=OUT/'temporary';SOCKET=ROOT/'.local/order-net-sockets'
for p in [OUT,TEMP,SOCKET]:p.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(TEMP),TMPDIR=str(TEMP),GOTOOLCHAIN='local',INVITE_CODE='order-network-local',ADMIN_USERNAME='排序验收管理员',ADMIN_PASSWORD='order-network-test-only',DATA_DIR=str(TEMP/'data'))
report={'passed':False,'scope':'production app, real HTTP/WS and TCP propagation delay; native desktop and 320px touch; no physical-device claim','cases':[]};process=None;opened=[]

class Proxy:
 def __init__(self,backend,delay=0):
  self.backend=backend;self.delay=delay;self.offline=False;self.writers=set();self.loop=asyncio.new_event_loop();self.ready=threading.Event()
  threading.Thread(target=self.run,daemon=True).start();self.ready.wait(5)
 def run(self):
  asyncio.set_event_loop(self.loop)
  self.server=self.loop.run_until_complete(asyncio.start_server(self.connect,'127.0.0.1',0));self.port=self.server.sockets[0].getsockname()[1];self.ready.set();self.loop.run_forever()
 async def pipe(self,reader,writer):
  queue=asyncio.Queue(128)
  async def deliver():
   while True:
    item=await queue.get()
    if item is None:return
    data,due=item;await asyncio.sleep(max(0,due-self.loop.time()));writer.write(data);await writer.drain()
  task=asyncio.create_task(deliver())
  try:
   while data:=await reader.read(32768):await queue.put((data,self.loop.time()+self.delay/2000))
   await queue.put(None);await task
  finally:task.cancel();writer.close()
 async def connect(self,reader,writer):
  if self.offline:writer.close();return
  remote=None
  try:
   upstream,remote=await asyncio.open_connection('127.0.0.1',self.backend);self.writers.update([writer,remote]);await asyncio.gather(self.pipe(reader,remote),self.pipe(upstream,writer))
  except (ConnectionError,asyncio.CancelledError):pass
  finally:
   for w in [writer,remote]:
    if w:w.close();self.writers.discard(w)
 def change(self,offline=None,delay=None):
  def apply():
   if delay is not None:self.delay=delay
   if offline is not None:
    self.offline=offline
    if offline:
     for w in list(self.writers):w.close()
  self.loop.call_soon_threadsafe(apply)
 def stop(self):
  async def finish():
   self.offline=True;self.server.close();await self.server.wait_closed()
   for w in list(self.writers):w.close()
   tasks=[t for t in asyncio.all_tasks() if t is not asyncio.current_task()]
   for task in tasks:task.cancel()
   await asyncio.gather(*tasks,return_exceptions=True)
  try:asyncio.run_coroutine_threadsafe(finish(),self.loop).result(5)
  finally:self.loop.call_soon_threadsafe(self.loop.stop)

def b(session,*args):
 env=dict(ENV,AGENT_BROWSER_SESSION='order-'+session,AGENT_BROWSER_SOCKET_DIR=str(SOCKET),AGENT_BROWSER_PROFILE=str(TEMP/('profile-'+session)))
 p=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=45);assert p.returncode==0,p.stdout+p.stderr;r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(session,code):return b(session,'eval',code)['result']
def api(session,path,body=None):
 options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js(session,'(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,data:await r.json()}})()');assert r['status'] in [200,201],r;return r['data']
def room(session='main'):return api(session,'/state')['room']
def cmd(session,kind,**values):
 r=room(session);return api(session,'/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**values))
def wait(fn,seconds=40):
 until=time.monotonic()+seconds
 while time.monotonic()<until:
  if fn():return
  time.sleep(.1)
 raise AssertionError('condition timed out')
def ids(session='main'):return js(session,'[...document.querySelectorAll(".admin-game-card")].map(e=>e.dataset.gameId)')
def native(session,selector,key=None):
 ws=b(session,'get','cdp-url');ws=ws if isinstance(ws,str) else ws.get('url',ws.get('cdpUrl',ws.get('value')))
 code=r'''const [url,page,selector,key]=process.argv.slice(1),ws=new WebSocket(url);let n=0;const pending=new Map();await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});ws.onmessage=({data})=>{const m=JSON.parse(data),p=pending.get(m.id);if(p){pending.delete(m.id);m.error?p[1](m.error):p[0](m.result)}};const call=(method,params={},sessionId)=>new Promise((r,j)=>{const id=++n;pending.set(id,[r,j]);ws.send(JSON.stringify({id,method,params,sessionId}))});try{const t=(await call('Target.getTargets')).targetInfos.find(t=>t.type==='page'&&t.url.startsWith(page));const {sessionId}=await call('Target.attachToTarget',{targetId:t.targetId,flatten:true});if(key){const defs={k:['KeyK',75],ArrowRight:['ArrowRight',39]};await call('Input.dispatchKeyEvent',{type:'rawKeyDown',key,code:defs[key][0],windowsVirtualKeyCode:defs[key][1]},sessionId);await new Promise(r=>setTimeout(r,80));await call('Input.dispatchKeyEvent',{type:'keyUp',key,code:defs[key][0],windowsVirtualKeyCode:defs[key][1]},sessionId);}else{const r=await call('Runtime.evaluate',{expression:'document.querySelector('+JSON.stringify(selector)+').getBoundingClientRect().toJSON()',returnByValue:true},sessionId);const q=r.result.value;await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:1},sessionId);await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:q.x+q.width/2,y:q.y+q.height/2,id:0,radiusX:4,radiusY:4,force:1}]},sessionId);await call('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]},sessionId)}}finally{ws.close()}'''
 p=subprocess.run(['node','--input-type=module','-e',code,ws,BASES[session],selector,key or ''],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=25);assert p.returncode==0,p.stderr+p.stdout

proxies=[]
try:
 if (TEMP/'data').exists():shutil.rmtree(TEMP/'data')
 subprocess.run(['go','build','-o',str(TEMP/'app'),'.'],cwd=ROOT,env=ENV,check=True,timeout=180)
 with socket.socket() as s:s.bind(('127.0.0.1',0));port=s.getsockname()[1]
 ENV['ADDR']='127.0.0.1:'+str(port);log=(TEMP/'server.log').open('w');process=subprocess.Popen([str(TEMP/'app')],cwd=ROOT,env=ENV,stdout=log,stderr=subprocess.STDOUT)
 def healthy():
  try:return urllib.request.urlopen('http://'+ENV['ADDR']+'/healthz',timeout=1).status==200
  except:return False
 wait(healthy);proxies=[Proxy(port),Proxy(port,250)];BASES=dict(zip(['main','peer'],['http://127.0.0.1:'+str(p.port) for p in proxies]))
 for session in ['main','peer']:b(session,'open',BASES[session]);opened.append(session)
 api('main','/login',{'name':ENV['ADMIN_USERNAME'],'password':ENV['ADMIN_PASSWORD']});api('peer','/register',{'name':'网络验收玩家','password':'order-network-test-only','invite':ENV['INVITE_CODE']})
 for session in opened:b(session,'reload');b(session,'wait','.lobby-create')
 b('main','set','viewport',1440,1000);b('main','click','.admin-nav-button');b('main','wait','.admin-game-card')
 initial=api('main','/admin')['games'];names={g['kind']:g['name'] for g in initial};before=[g['kind'] for g in initial]
 b('main','select','select[aria-label="管理游戏分类"]','sports');visible=ids();assert len(visible)>=2
 moved=visible[-1];up='button[aria-label='+json.dumps('上移'+names[moved],ensure_ascii=False)+']';down='button[aria-label='+json.dumps('下移'+names[moved],ensure_ascii=False)+']'
 b('main','click',up);b('main','click',down);disabled=js('main','(()=>{const e=document.querySelector("button[aria-label=保存游戏顺序]"),s=getComputedStyle(e);return {disabled:e.disabled,color:s.color,background:s.backgroundColor,opacity:s.opacity}})()')
 assert disabled['disabled'] and disabled['opacity']=='1',disabled
 b('main','click',up);draft=ids();assert draft[-2]==moved;time.sleep(5.5);assert ids()==draft,'automatic refresh erased draft'
 b('main','click','button[aria-label="保存游戏顺序"]');wait(lambda:js('main','document.body.innerText.includes("游戏顺序已保存")'))
 b('main','select','select[aria-label="管理游戏分类"]','all');ordered=ids();outside=[x for x in before if x not in visible];assert [ordered.index(x) for x in outside]==[before.index(x) for x in outside]
 js('main','window.__sortDrag=[];["dragstart","dragover","drop","dragend"].forEach(t=>document.addEventListener(t,e=>window.__sortDrag.push({type:t,target:e.target.closest("[data-game-id]")?.dataset.gameId,trusted:e.isTrusted})));true')
 b('main','drag',f'[data-game-id="{ordered[0]}"]',f'[data-game-id="{ordered[2]}"]');wait(lambda:ids()!=ordered);drag_trace=js('main','window.__sortDrag');assert any(e['type']=='drop' and e['trusted'] for e in drag_trace);b('main','click','button[aria-label="保存游戏顺序"]');wait(lambda:not js('main','!!document.querySelector("button[aria-label=保存游戏顺序]")'));b('main','screenshot',OUT/'admin-desktop.png')
 b('main','set','viewport',320,760);first=ids()[0];selector='button[aria-label='+json.dumps('下移'+names[first],ensure_ascii=False)+']';b('main','scrollintoview',selector);native('main',selector);wait(lambda:ids()[1]==first)
 b('main','scrollintoview','button[aria-label="保存游戏顺序"]');native('main','button[aria-label="保存游戏顺序"]');wait(lambda:not js('main','!!document.querySelector("button[aria-label=保存游戏顺序]")'));saved=ids();assert not js('main','document.documentElement.scrollWidth>innerWidth');b('main','screenshot',OUT/'admin-mobile.png')
 hidden=next(x for x in reversed(saved) if x not in ['tabletennis','battlecity']);api('main','/admin/games/'+hidden,{'enabled':False});b('main','reload');b('main','wait','.admin-game-card');assert ids()==saved
 b('main','click','.admin-nav-button');b('main','click','.lobby-create');b('main','wait','.choose-games');cards=js('main','[...document.querySelectorAll(".choose-games button > span:last-child")].map(e=>e.textContent.trim())');assert cards==[names[x] for x in saved if x!=hidden],cards;b('main','screenshot',OUT/'ordered-create-mobile.png');b('main','press','Escape')
 report['cases'].append({'type':'sorting','nativeDesktopDrag':True,'dragEvents':drag_trace,'nativeMobileTouch':True,'filteredSlotsPreserved':True,'draftSurvivedRefresh':True,'disabledSaveStyle':disabled,'saved':saved,'hiddenPreserved':hidden,'playerCards':cards})
 # Two true clients share a normal room. RTT comes from protocol ping/pong.
 api('main','/rooms',{'kind':'tabletennis','name':'网络信号验收','capacity':2});r=room();api('peer','/rooms/'+r['id'],dict(type='join',version=r['version'],nonce=uuid.uuid4().hex))
 wait(lambda:all(isinstance(p.get('network',{}).get('rttMs'),(float,int)) for p in room()['seats']))
 seats=room()['seats'];assert seats[1]['network']['rttMs']>seats[0]['network']['rttMs']+100,seats
 b('main','scrollintoview','.waiting-seat');signals=js('main','[...document.querySelectorAll(".waiting-seat .network-signal")].map(e=>({text:e.textContent,bars:e.dataset.networkBars,rect:e.getBoundingClientRect().toJSON()}))');assert len(signals)==2 and int(signals[0]['bars'])>int(signals[1]['bars']),signals;b('main','screenshot',OUT/'peer-signals-mobile.png')
 cmd('main','ready');cmd('peer','ready');cmd('main','start');wait(lambda:not room().get('loading'));wait(lambda:js('main','document.querySelector("[data-tabletennis-board]")?.dataset.ready==="true"'))
 wait(lambda:room()['game']['data']['phase']=='serve');state=room()['game']['data'];actor=state['expected'];session=['main','peer'][actor];proxy=proxies[actor];proxy.change(delay=250);time.sleep(.3);b(session,'focus','.tt-scene canvas');native(session,'.tt-scene canvas','k');proxy.change(offline=True);time.sleep(1)
 assert not js(session,'!!document.querySelector("button[aria-label=关闭错误]")');assert not js(session,'document.body.innerText.includes("实时连接已断开")');b(session,'screenshot',OUT/'quiet-reconnect.png')
 proxy.change(offline=False,delay=0);time.sleep(.2)
 def restored():
  try:return api(session,'/state')['room']['game']['data']['epoch']==state['epoch']
  except AssertionError as e:
   if 'Failed to fetch' not in str(e):raise
   return False
 wait(restored);wait(lambda:js(session,'document.querySelector(".topbar .network-signal")?.dataset.networkBars!=="0"'))
 assert not js(session,'!!document.querySelector("button[aria-label=关闭错误]")');assert all(not p.get('autoPlay') for p in room()['seats']);report['cases'].append({'type':'network','measuredPeers':[s['network'] for s in seats],'signals':signals,'quietRecovery':True,'epochPreserved':True,'autoplay':False})
 cmd('main','close');cmd('main','leave');cmd('peer','leave')
 # Battlecity used to require a manual resync click after any transport error.
 api('main','/rooms',{'kind':'battlecity','name':'坦克重连验收','capacity':2});cmd('main','add_bot');cmd('main','ready');cmd('main','start');wait(lambda:not room().get('loading'));wait(lambda:room()['game']['data']['phase']=='playing')
 b('main','scrollintoview','canvas');b('main','focus','canvas');proxies[0].change(delay=250);time.sleep(.3);native('main','canvas','ArrowRight');proxies[0].change(offline=True);time.sleep(1)
 assert not js('main','document.body.innerText.includes("操作同步失败")');assert not js('main','!!document.querySelector("button[aria-label=关闭错误]")')
 proxies[0].change(offline=False,delay=0);time.sleep(.3);wait(lambda:js('main','document.querySelector(".topbar .network-signal")?.dataset.networkBars!=="0"'));time.sleep(.3)
 before_seq=room()['game']['data']['tanks'][0]['seq'];b('main','focus','canvas');native('main','canvas','ArrowRight');wait(lambda:room()['game']['data']['tanks'][0]['seq']>before_seq);b('main','screenshot',OUT/'battlecity-recovered.png');report['cases'].append({'type':'battlecity','recoveredWithoutManualResync':True});cmd('main','close');cmd('main','leave')
 report['passed']=True
finally:
 if opened and not report['passed']:
  try:report['page']=js('main','document.body.innerText');report['dragTrace']=js('main','window.__sortDrag??[]');b('main','screenshot',OUT/'failure.png','--full')
  except:pass
 for session in opened:
  try:b(session,'close')
  except:pass
 for proxy in proxies:proxy.stop()
 if process:process.terminate();process.wait(timeout=15)
 for session in opened:shutil.rmtree(TEMP/('profile-'+session),ignore_errors=True)
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'cases':len(report['cases'])}))
