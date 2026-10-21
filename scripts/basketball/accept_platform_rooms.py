"""Real React Board, authenticated HTTP, browser Wasm and native keyboard/touch."""
from pathlib import Path
import json,os,subprocess,uuid
ROOT=Path(__file__).resolve().parents[2];OUT=Path(os.environ.get('BASKETBALL_BROWSER_OUTPUT',str(ROOT/'.local/basketball/platform-rooms'))).resolve();assert OUT.is_relative_to((ROOT/'.local/basketball').resolve());OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkp-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str((ROOT/'.local/basketball').resolve()/'bkn-sockets'),TMPDIR=str(ROOT/'.local/tmp'),NODE_PATH=str(ROOT/'web/node_modules'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
base=env['BASKETBALL_HTTP_URL'];checks=[];opened=False;inspector=None
def browser(*args):
 if inspector and args[0] in ('click','select','fill') and not args[1].startswith('@'):
  wait('(()=>{const e=document.querySelector('+json.dumps(args[1])+');return !!e&&!e.disabled})()')
 if inspector and args[:4]==('find','role','button','click') and '--name' in args:
  label=args[args.index('--name')+1]
  wait('(()=>{const e=[...document.querySelectorAll("button")].find(b=>b.textContent.trim()==='+json.dumps(label)+'||b.getAttribute("aria-label")==='+json.dumps(label)+');return !!e&&!e.disabled})()')
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 value=json.loads(r.stdout)
 if not value['success']:raise RuntimeError(value)
 return value['data']
def cdp(method,params={}):
 inspector.stdin.write(json.dumps(dict(method=method,params=params))+'\n');inspector.stdin.flush();r=json.loads(inspector.stdout.readline())
 if 'error'in r:raise RuntimeError(r['error'])
 return r['result']
def js(source):
 r=cdp('Runtime.evaluate',dict(expression=source,returnByValue=True,awaitPromise=True,timeout=20000))
 if r.get('exceptionDetails'):raise RuntimeError(r['exceptionDetails'])
 return r['result'].get('value')
def wait(expression):
 return js('(async()=>{for(let n=0;n<750;n++){if(await ('+expression+'))return true;await new Promise(r=>setTimeout(r,20));}throw Error('+json.dumps('Timed out: '+expression)+')})()')
def key(kind,code,key,number):cdp('Input.dispatchKeyEvent',dict(type=kind,code=code,key=key,windowsVirtualKeyCode=number))
def expect(name,value):
 assert value,name
 checks.append(name)
try:
 (OUT/'result.json').write_text('{"status":"running"}\n')
 page=base+'/'
 browser('--profile',str(OUT/'profile'),'open',page);opened=True
 endpoint=browser('get','cdp-url')['cdpUrl'];inspector=subprocess.Popen(['node',str(ROOT/'scripts/basketball/browser_cdp_session.mjs'),endpoint,page],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'cdp.stderr').open('w'),text=True);assert json.loads(inspector.stdout.readline())['ready']
 mirror=(ROOT/'scripts/basketball/browser_oss_mirror.js').read_text();cdp('Page.enable');cdp('Page.addScriptToEvaluateOnNewDocument',dict(source=mirror));js(mirror)
 wait('!!document.querySelector("input[name=name]")')
 browser('fill','input[name=name]','牌局管理员');browser('fill','input[name=password]','poker-test-password');browser('click','.auth-form button[type=submit], .auth-form button.primary')
 wait('!!document.querySelector(".lobby") || document.body.innerText.includes("创建房间")')
 js('window.platformRequests=[];const originalFetch=window.fetch;window.fetch=async(...args)=>{const response=await originalFetch(...args);if(args[1]?.method==="POST")platformRequests.push({url:args[0],body:args[1]?.body,status:response.status,data:await response.clone().json()});return response};window.platformErrors=[];window.addEventListener("error",e=>platformErrors.push(e.message));window.addEventListener("unhandledrejection",e=>platformErrors.push(String(e.reason)))')
 # Campaign uses the same production game picker and waiting room.
 browser('find','role','button','click','--name','创建房间')
 browser('fill','[aria-label="搜索可选游戏"]','热血篮球');browser('click','.choose-games button')
 browser('select','[aria-label="座位上限"]','1')
 browser('select','[aria-label="比赛形式"]','password')
 browser('fill','[aria-label="原版密码"]','020010')
 browser('fill','[data-create-step=setup] input[name=name]','热血战役准备')
 browser('click','[data-create-step=setup] button[type=submit]')
 wait('!!document.querySelector(".basketball-campaign-editor")')
 wait('[...document.querySelectorAll(".basketball-campaign-editor img")].every(i=>i.complete&&i.naturalWidth>0)')
 expect('campaign password entry uses the standard waiting room',js('!!document.querySelector(".waiting-seats")&&document.body.innerText.includes("火箭球衣")'))
 browser('select','[aria-label="装备给哪位球员"]','2')
 browser('click','[data-inventory-index="0"]')
 expect('equipment preview leaves authoritative inventory unchanged',js('fetch("/api/state").then(r=>r.json()).then(s=>s.room.options.campaign.preparation.inventory[0]===1)'))
 browser('click','[data-equip-confirm]');wait('!document.querySelector("[data-inventory-index]")')
 browser('click','[data-campaign-position="0"] [data-character="2"]')
 wait('document.querySelector(\'[data-campaign-position="0"] [data-character="2"]\')?.getAttribute("aria-pressed")==="true"')
 browser('click','[data-opponent="4"]');wait('document.querySelector(\'[data-opponent="4"]\')?.getAttribute("aria-pressed")==="true"')
 campaign=js('fetch("/api/state").then(r=>r.json()).then(s=>s.room.options)')
 expect('native campaign choices reach authenticated source preparation',campaign['campaign']['password']=='020010' and campaign['characters'][0]==2 and campaign['campaign']['preparation']['equipment'][2][0]==1 and campaign['teams']==[0,4] and campaign['court']=='court-0')
 browser('set','viewport','320','720')
 contrast=js('(()=>{const rgb=s=>s.match(/[\\d.]+/g).slice(0,3).map(Number),lum=c=>c.map(n=>{n/=255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4}).reduce((s,n,i)=>s+n*[.2126,.7152,.0722][i],0);return [...document.querySelectorAll(".basketball-campaign-editor button,.basketball-campaign-editor p,.basketball-campaign-editor h3,.basketball-campaign-editor h4,.basketball-campaign-editor label,.basketball-options small,.basketball-options p,.basketball-campaign-grades th,.basketball-campaign-grades td,.basketball-campaign-grades caption")].filter(e=>e.textContent.trim()).map(e=>{let b=e;while(b&&getComputedStyle(b).backgroundColor==="rgba(0, 0, 0, 0)")b=b.parentElement;const fg=lum(rgb(getComputedStyle(e).color)),bg=lum(rgb(getComputedStyle(b).backgroundColor));return{text:e.textContent,ratio:(Math.max(fg,bg)+.05)/(Math.min(fg,bg)+.05)}})})()')
 expect('production campaign text contrast is at least 4.5:1',all(row['ratio']>=4.5 for row in contrast))
 (OUT/'campaign-contrast.json').write_text(json.dumps(contrast,ensure_ascii=False,indent=2)+'\n')
 expect('campaign waiting room does not overflow 320px',js('document.documentElement.scrollWidth<=innerWidth'))
 browser('screenshot',str(OUT/'campaign-waiting-mobile.png'),'--full')
 browser('set','viewport','1280','900');browser('screenshot',str(OUT/'campaign-waiting-desktop.png'),'--full')
 browser('find','role','button','click','--name','离开房间');wait('!document.querySelector(".waiting-seats")')
 for mode,capacity in [('solo',1),('cooperative',2),('versus',2)]:
  browser('find','role','button','click','--name','创建房间')
  browser('fill','[aria-label="搜索可选游戏"]','热血篮球')
  expect('basketball illustrated cover in standard game picker '+mode,js('document.querySelectorAll(".choose-games button").length===1 && !!document.querySelector(".choose-games svg[role=img]")'))
  browser('click','.choose-games button')
  browser('select','[aria-label="座位上限"]',str(capacity))
  if capacity==2:
   wait('!!document.querySelector("[aria-label=比赛模式]")');browser('select','[aria-label="比赛模式"]',mode)
  browser('fill','[data-create-step=setup] input[name=name]','热血篮球 '+mode)
  browser('set','viewport','320','720')
  expect('standard creation fits mobile '+mode,js('document.documentElement.scrollWidth<=innerWidth'))
  expect('court and player choices are absent from creation '+mode,js('!document.querySelector(".basketball-preparation")'))
  browser('screenshot',str(OUT/(mode+'-create-mobile.png')))
  browser('scrollintoview','[data-create-step=setup] button[type=submit]')
  browser('click','[data-create-step=setup] button[type=submit]')
  wait('!!document.querySelector(".waiting-seats")')
  state=js('fetch("/api/state").then(r=>r.json())');room=state['room']
  expect('real platform creates exact mode and capacity '+mode,room['options']['mode']==mode and room['capacity']==capacity and len(room['seats'])==1)
  expect('standard waiting room shows declared roster '+mode,js('document.querySelectorAll(".waiting-seat").length==='+str(capacity)))
  wait('document.querySelector(".basketball-preparation") && [...document.querySelectorAll(".basketball-preparation img")].every(i=>i.complete&&i.naturalWidth>0)')
  expect('court labels remain below preview images in platform styles '+mode,js('[...document.querySelectorAll(".basketball-court-choices button")].every(b=>{const i=b.querySelector("img").getBoundingClientRect(),label=b.querySelector("span").getBoundingClientRect(),r=b.getBoundingClientRect();return label.top>=i.bottom-1 && i.left>=r.left && i.right<=r.right && label.right<=r.right})'))
  expect('preparation loads original court and player images '+mode,js('document.querySelectorAll(".basketball-preparation img").length>=15'))
  expect('all eight original courts appear only in waiting preparation '+mode,js('document.querySelectorAll(".basketball-preparation [data-court]").length===8'))
  browser('screenshot',str(OUT/(mode+'-waiting-mobile.png')),'--full')
  if mode!='versus':
   browser('find','role','button','click','--name','离开房间');wait('!document.querySelector(".waiting-seats")');continue
  expect('unfilled human seat is not mislabeled as a computer',js('[...document.querySelectorAll(".basketball-roster-player")].find(e=>e.dataset.actor==="2").querySelector(".basketball-player-owner strong").textContent==="等待玩家"'))
  browser('set','viewport','1280','900')
  browser('screenshot',str(OUT/'versus-waiting-desktop.png'),'--full')
  browser('set','viewport','320','720')
  # Change choices in the real waiting room; these are actual server actions.
  browser('scrollintoview','[data-court="court-6"]');browser('click','[data-court="court-6"]')
  wait('document.querySelector(\'[data-court="court-6"]\')?.getAttribute("aria-pressed")==="true"')
  expect('seventh court selection is saved by the preparation server',js('fetch("/api/state").then(r=>r.json()).then(s=>s.room.options.court==="court-6")'))
  browser('scrollintoview','[data-actor="2"] [data-character="1"]');browser('click','[data-actor="2"] [data-character="1"]')
  wait('document.querySelector(\'[data-actor="2"] [data-character="1"]\')?.getAttribute("aria-pressed")==="true"')
  selected=js('fetch("/api/state").then(r=>r.json()).then(s=>s.room)')
  expect('waiting selection reaches authoritative court and roster',selected['options']['court']=='court-6' and selected['options']['characters']==[0,1,1,0])
  room=selected
  # Separate authenticated HTTP session joins and readies the second human.
  import urllib.request,http.cookiejar
  peer=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
  def request(path,body=None):
   req=urllib.request.Request(base+'/api'+path,data=json.dumps(body).encode() if body is not None else None,headers={'Content-Type':'application/json'})
   with peer.open(req) as response:return json.load(response)
  request('/login',{'name':'牌局玩家1','password':'poker-test-password'})
  request('/rooms/'+room['id'],{'type':'join','version':room['version'],'nonce':'platform-room-join-001'})
  other=request('/state')['room'];request('/rooms/'+room['id'],{'type':'ready','version':other['version'],'nonce':'platform-room-ready-001'})
  wait('document.querySelectorAll(".waiting-seat")[1]?.textContent.includes("已准备")')
  browser('find','role','button','click','--name','我准备好了');wait('[...document.querySelectorAll("button")].some(b=>b.textContent.includes("开始游戏")&&!b.disabled)')
  js('[...document.querySelectorAll("button")].find(b=>b.textContent.includes("开始游戏")).scrollIntoView({block:"center"})');browser('find','role','button','click','--name','开始游戏');wait('!!document.querySelector("[data-basketball-ready=true]")')
  other=request('/state')['room'];request('/rooms/'+room['id'],{'type':'action','version':other['version'],'nonce':'platform-peer-loaded-001','action':{'type':'loaded','epoch':other['game']['data']['epoch']}})
  wait('Number(document.querySelector(".basketball-stage canvas")?.dataset.videoFrame)>=120')
  expect('versus retains original no-equipment mode',js('fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data.runtime.config.match.Mode===128 && s.room.game.data.runtime.config.match.Equipment.flat().every(i=>i===-1) && !document.querySelector(".basketball-campaign-editor"))'))
  expect('started original runtime uses prepared court and characters',js('fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data.runtime.config.match.Court===6 && JSON.stringify(s.room.game.data.runtime.config.match.Characters)==="[0,1,1,0]")'))
  expect('platform starts directly in game without original player-selection menu',js('!!document.querySelector(".basketball-stage canvas") && !!document.querySelector(".fc-play") && !document.querySelector(".basketball-error")'))
  expect('public player summaries distinguish both teams',js('document.querySelector(".players-strip").textContent.includes("A 队") && document.querySelector(".players-strip").textContent.includes("B 队")'))
  expect('phone controller remains outside the match screen',js('(()=>{const s=document.querySelector(".basketball-screen"),p=document.querySelector(".fc-play");return !s.contains(p)&&s.getBoundingClientRect().bottom<=p.getBoundingClientRect().top})()'))
  browser('screenshot',str(OUT/'versus-match-mobile.png'))
  browser('set','viewport','1280','900')
  expect('PC controller sits below the match screen',js('(()=>{const s=document.querySelector(".basketball-screen"),p=document.querySelector(".fc-play");return !s.contains(p)&&s.getBoundingClientRect().bottom<=p.getBoundingClientRect().top})()'))
  browser('screenshot',str(OUT/'versus-match-desktop.png'),'--full')
  # Observe production WebSocket packets without changing their bytes or timing.
  js('window.streamTrace=[];const originalSend=WebSocket.prototype.send;WebSocket.prototype.send=function(raw){let p;try{p=JSON.parse(raw)}catch{};if(p?.type==="input"){if(!this.basketballObserved){this.basketballObserved=true;this.addEventListener("message",e=>{try{const p=JSON.parse(e.data);if(p.type==="ack")streamTrace.push({direction:"in",at:performance.now(),packet:p})}catch{}})}streamTrace.push({direction:"out",at:performance.now(),packet:p})}return originalSend.call(this,raw)}')
  js('document.querySelector(".basketball-board").focus()')
  key('keyDown','KeyD','d',68)
  js('new Promise(r=>setTimeout(r,40))')
  key('keyUp','KeyD','d',68)
  wait('streamTrace.filter(e=>e.direction==="out").length>=2 && streamTrace.filter(e=>e.direction==="out").every(e=>streamTrace.some(a=>a.direction==="in"&&a.packet.nonce===e.packet.nonce))')
  trace=js('streamTrace')
  outgoing=[e for e in trace if e['direction']=='out']
  first=next(e for e in outgoing if e['packet']['action']['events'][-1]['buttons']==1)
  released=next(e for e in outgoing if e['at']>first['at'] and e['packet']['action']['events'][-1]['buttons']==0)
  ack=next(e for e in trace if e['direction']=='in' and e['packet']['nonce']==first['packet']['nonce'])
  expect('native D press/release uses production ordered stream',all(e['packet']['status']==200 for e in trace if e['direction']=='in'))
  if int(env.get('BASKETBALL_BROWSER_RTT_MS','0'))>=300:
   expect('release is sent before delayed first ACK',released['at']<ack['at'])
  wait('fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data.inputs[0].buttons===0)')
  (OUT/'stream-trace.json').write_text(json.dumps(dict(rttMS=int(env.get('BASKETBALL_BROWSER_RTT_MS','0')),trace=trace),indent=2)+'\n')

 expect('no application errors',js('platformErrors.length===0 && !document.querySelector(".basketball-error")'))
 (OUT/'result.json').write_text(json.dumps(dict(status='passed',checks=checks,scope='built production UI, real platform login/create/wait/join/start, independent test module registry and verified OSS-byte fixture mirrors; no production registration, OSS or full multiplayer match'),ensure_ascii=False,indent=2)+'\n')
 print(json.dumps(checks,ensure_ascii=False))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,error=repr(error)),ensure_ascii=False,indent=2)+'\n')
 if opened:
  try:
   (OUT/'failure-state.json').write_text(json.dumps(js('fetch("/api/state").then(r=>r.json()).then(state=>({body:document.body.innerText,errors:window.platformErrors,requests:window.platformRequests,state}))'),ensure_ascii=False,indent=2)+'\n')
   browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if inspector:
  inspector.stdin.close()
  try:inspector.wait(timeout=5)
  except subprocess.TimeoutExpired:inspector.terminate();inspector.wait(timeout=5)
 if opened:browser('close')
