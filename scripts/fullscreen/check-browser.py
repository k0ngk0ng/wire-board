#!/usr/bin/env python3
"""Shared full-screen browser acceptance, see SCENARIOS.md. Build web first."""
import argparse, hashlib, http.cookiejar, json, os, re, shutil, subprocess, time, traceback, urllib.request, uuid
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);parser.add_argument('--tank-only',action='store_true');parser.add_argument('--games',default='');args=parser.parse_args()
root=Path(__file__).resolve().parents[2];out=(root/args.out).resolve();assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
run='full'+uuid.uuid4().hex[:6];temp=out/'temporary';sockets=root/'.s'/run;env=dict(os.environ)
for k,v in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','npm_config_cache':root/'.local/npm','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():v.mkdir(parents=True,exist_ok=True);env[k]=str(v)
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local')
report={'passed':False,'games':{},'command':'python3 -B scripts/fullscreen/check-browser.py --out '+args.out+(' --tank-only' if args.tank_only else ''),'buildIndexSHA256':hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()}
server=log=None;opened=False

def browser(*a):
 r=subprocess.run(['agent-browser',*a,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr
 j=json.loads(r.stdout);assert j['success'],j;return j['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()');assert r['status'] in (200,201),r;return r['data']
def room():return api('/state').get('room')
def cmd(kind,**extra):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def wait(fn,seconds=20):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if server is not None and server.poll() is not None:raise AssertionError('fixture exited; inspect server.log')
  if value:=fn():return value
  time.sleep(.1)
 raise AssertionError('Timed out')
def save(name,data):(out/(name+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')

def active():return js('document.documentElement.dataset.roomFullscreen || ""')
def toggle():
 if js('!!document.querySelector(".toast.error")'):
  report.setdefault('notices',[]).append(js('document.querySelector(".toast.error").innerText'))
  browser('click','button[aria-label="关闭错误"]')
 if not active():js('window.scrollTo(0,0);true')
 browser('click','[data-fullscreen-toggle]')
def tap():
 result=subprocess.run(['node','scripts/fullscreen/tap.mjs',browser('get','cdp-url')['cdpUrl'],base,'[data-fullscreen-toggle]'],cwd=root,env=env,text=True,capture_output=True,timeout=20)
 assert result.returncode==0,result.stderr
 report.setdefault('touch',[]).append(json.loads(result.stdout))
def shot(name):browser('screenshot',str(out/(name+'.png')))
def check(name,**data):report['games'][name]=data;print('PASS',name,flush=True)
def measure():return js("""(()=>{const c=document.querySelector('[data-fortressblue-canvas]'),r=c?.getBoundingClientRect();return {width:innerWidth,height:innerHeight,overflow:document.documentElement.scrollWidth>innerWidth+1,canvas:r?{w:r.width,h:r.height,backingWidth:c.width,viewWidth:Number(c.dataset.worldViewWidth)}:null}})()""")
try:
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 wait(lambda:(out/'url').exists(),120);base=(out/'url').read_text();browser('open',base);opened=True
 api('/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'});browser('reload');browser('wait','.room-list')
 browser('set','viewport','1600','1000')
 # All registered games exercise the shared entry, native state and cleanup.
 for kind in ([] if args.tank_only else (args.games.split(',') if args.games else api('/state')['availableGames'])):
  catalog=(root/'games'/kind/'web/catalog.tsx').read_text()
  capacity=int(re.search(r'minPlayers:\s*(\d+)',catalog)[1])
  api('/rooms',{'kind':kind,'name':'全屏验收 '+kind,'capacity':capacity,'options':{}})
  browser('wait','[data-fullscreen-toggle]')
  toggle();wait(lambda:active()=='native')
  assert js('document.fullscreenElement===document.documentElement')
  browser('press','Escape');wait(lambda:not active())
  toggle();wait(lambda:active()=='native');js('document.exitFullscreen().then(()=>true)');wait(lambda:not active());toggle();wait(lambda:active()=='native');toggle();wait(lambda:not active())
  browser('set','viewport','320','740');assert not measure()['overflow'],kind;toggle();wait(lambda:bool(active()))
  assert not measure()['overflow'],kind;shot(kind+'-320')
  toggle();wait(lambda:not active());browser('set','viewport','1600','1000')
  check(kind,nativeEnter=True,escape=True,buttonExit=True,mobile320=True)
  for _ in range(capacity-1):cmd('add_bot')
  cmd('ready');toggle();wait(lambda:active()=='native');cmd('start');browser('wait','.game-main')
  wait(lambda:js('!!document.querySelector(".game-main")?.children.length&&!document.querySelector(".game-loading")'),60)
  wait(lambda:js('Array.from(document.querySelectorAll(".game-main button,.game-main input,.game-main select,.game-main [role=button],.game-main canvas")).some(e=>e.getClientRects().length&&getComputedStyle(e).visibility!=="hidden")'),30)
  # Observe actual component/controls across fullscreen, without injecting game
  # state or requiring unrelated rule combinations and complete bot matches.
  js('window.__boardBeforeFullscreen=document.querySelector(".game-main").firstElementChild;true')
  time.sleep(.4)
  assert js('window.__boardBeforeFullscreen===document.querySelector(".game-main").firstElementChild'),kind
  controls=js('(()=>{const root=document.querySelector(\'.game-main\');return [...root.querySelectorAll(\'button,input,select,[role="button"],canvas,svg[role="group"]\')].filter(e=>{const s=getComputedStyle(e);return e.getClientRects().length&&s.visibility!==\'hidden\'}).map(e=>({tag:e.tagName,name:e.getAttribute(\'aria-label\')||e.textContent?.trim().slice(0,60),disabled:!!e.disabled,width:e.getBoundingClientRect().width,height:e.getBoundingClientRect().height}))})()')
  assert controls,kind
  shot(kind+'-playing-desktop');save(kind+'-controls',controls)
  browser('set','viewport','320','740');wait(lambda:bool(active()))
  assert js('window.__boardBeforeFullscreen===document.querySelector(".game-main").firstElementChild'),kind
  shot(kind+'-playing-mobile');browser('press','Escape');wait(lambda:not active())
  browser('set','viewport','1600','1000');cmd('close');cmd('leave');wait(lambda:not js('!!document.querySelector("[data-fullscreen-toggle]")'))
  check(kind+'-playing',componentPreserved=True,desktopControls=len(controls),mobile320=True)
  browser('reload');browser('wait','.room-list')
 # Actual running tank match; canvas must resize without remounting or state reset.
 api('/rooms',{'kind':'fortressblue','name':'疯狂坦克宽屏验收','capacity':2,'options':{}})
 browser('wait','[data-fullscreen-toggle]')
 cmd('add_bot')
 cmd('ready');cmd('start');browser('wait','[data-fortressblue-tank]')
 browser('click','[data-fortressblue-tank="1"]')
 wait(lambda:room()['game']['data']['phase']=='playing',45)
 browser('set','viewport','1920','1080');time.sleep(1)
 normal=measure();js('window.__fullscreenCanvas=document.querySelector("[data-fortressblue-canvas]");true')
 toggle();wait(lambda:active()=='native');time.sleep(.7)
 wide=measure();assert wide['canvas']['w']>normal['canvas']['w']+100,(normal,wide)
 assert 1.9<wide['canvas']['w']/wide['canvas']['h']<2.1,wide
 assert js('window.__fullscreenCanvas===document.querySelector("[data-fortressblue-canvas]")')
 assert not wide['overflow'];shot('tank-desktop-fullscreen')
 # Keyboard must work immediately after clicking fullscreen, without leaving
 # focus on the fullscreen button (Space there would otherwise exit it).
 angle=js('Number(document.querySelector(".fortressblue-board").dataset.fortressblueAngle)')
 browser('press','ArrowUp')
 wait(lambda:js('Number(document.querySelector(".fortressblue-board").dataset.fortressblueAngle)')==angle+1)
 browser('press','q');assert js('document.querySelector("[data-fortressblue-weapon=special]").getAttribute("aria-pressed")==="true"')
 browser('press','e');assert js('document.querySelector("[data-fortressblue-weapon=primary]").getAttribute("aria-pressed")==="true"')
 before_shot=room()['game']['data'].get('lastShot',{}).get('id',0)
 shot_input=subprocess.run(['node','scripts/fullscreen/tap.mjs',browser('get','cdp-url')['cdpUrl'],base,'[data-fortressblue-canvas]','shot'],cwd=root,env=env,text=True,capture_output=True,timeout=20);assert shot_input.returncode==0,shot_input.stderr
 wait(lambda:room()['game']['data'].get('lastShot',{}).get('id',0)>before_shot)
 assert active()=='native'
 check('tank-keyboard-fullscreen',angle=True,weapon=True,shot=True)

 browser('find','role','button','click','--name','玩法速查');browser('wait','[role="dialog"]');shot('fullscreen-rules')
 browser('click','[role="dialog"] button[aria-label="关闭"]')
 toggle();wait(lambda:not active());time.sleep(.4);restored=measure();assert abs(restored['canvas']['w']-normal['canvas']['w'])<2
 # Unsupported and rejected native API are labelled capability fault probes.
 js('window.__nativeRequest=Element.prototype.requestFullscreen;window.__webkitRequest=Element.prototype.webkitRequestFullscreen;Element.prototype.requestFullscreen=undefined;Element.prototype.webkitRequestFullscreen=undefined;true')
 for w,h in [(320,740),(844,390)]:
  browser('set','viewport',str(w),str(h));tap();wait(lambda:active()=='page')
  assert not measure()['overflow'];shot('tank-page-'+str(w));tap();wait(lambda:not active())
 browser('set','viewport','1600','1000')
 js('Element.prototype.requestFullscreen=()=>Promise.reject(new Error("acceptance denied"));true')
 toggle();wait(lambda:active()=='page');js('window.__escapeForwarded=false;window.addEventListener("keydown",e=>{if(e.key==="Escape")window.__escapeForwarded=true},{once:true});true');browser('press','Escape');wait(lambda:not active());assert js('window.__escapeForwarded'), 'Escape must also cancel game input'
 toggle();wait(lambda:active()=='page');cmd('close');cmd('leave');wait(lambda:not active())
 js('Element.prototype.requestFullscreen=window.__nativeRequest;Element.prototype.webkitRequestFullscreen=window.__webkitRequest;true')
 check('tank-widescreen',normal=normal,wide=wide,restored=restored,unsupportedFallback=True,rejectedFallback=True,leaveCleanup=True)
 # Playing poker uses a separate compact platform header; assert actual hit
 # targets, not merely the presence of the button in the DOM.
 for kind in ['doudizhu','guandan','shengji']:
  capacity=4
  api('/rooms',{'kind':kind,'name':'全屏扑克牌桌','capacity':capacity,'options':{}})
  for _ in range(capacity-1):cmd('add_bot')
  cmd('ready');cmd('start');browser('wait','.poker-board')
  browser('set','viewport','320','740');time.sleep(.3)
  tap();wait(lambda:bool(active()));assert not measure()['overflow'];shot(kind+'-playing-320')
  geometry=js("(()=>{const b=document.querySelector('[data-fullscreen-toggle]'),r=b.getBoundingClientRect();return {button:{x:r.x,y:r.y,w:r.width,h:r.height},clear:[...document.querySelectorAll('.compact-room-button,.poker-table-toolbar,.table-chat')].filter(e=>e.getClientRects().length).every(e=>{const q=e.getBoundingClientRect();return q.right<=r.left||q.left>=r.right||q.bottom<=r.top||q.top>=r.bottom})}})()")
  assert geometry['clear'],geometry
  tap();wait(lambda:not active());cmd('close');cmd('leave');check(kind+'-playing',**geometry)
 # Native entry can finish after navigating away. Its late promise must not
 # restore either the old room layout or a fullscreen element.
 browser('set','viewport','1600','1000')
 api('/rooms',{'kind':'fortressblue','name':'全屏异步退出验收','capacity':2,'options':{}})
 browser('wait','[data-fullscreen-toggle]')
 js('Element.prototype.requestFullscreen=function(){return window.__nativeRequest.call(this).then(()=>new Promise(resolve=>window.__finishFullscreen=resolve))};true')
 toggle();wait(lambda:active()=='native');wait(lambda:js('typeof window.__finishFullscreen==="function"'))
 cmd('leave');wait(lambda:not active());js('window.__finishFullscreen();true');time.sleep(.3)
 assert js('!document.fullscreenElement&&!document.documentElement.dataset.roomFullscreen')
 check('late-native-completion',cleaned=True)
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc();print(report['error'],flush=True)
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=20)
  except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
  if server.returncode!=0:report['passed']=False
 if log:log.close()
 for path in [temp,sockets,out/'data']:
  if path.exists():shutil.rmtree(path)
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 report['sources']={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'scripts/fullscreen/check-browser.py',root/'scripts/fullscreen/tap.mjs',root/'web/src/ui/compact-room-panel.tsx',root/'web/src/ui/room-fullscreen.tsx',root/'web/src/ui/room-fullscreen.css',root/'web/src/main.tsx',root/'games/fortressblue/web/style.css',root/'games/fortressblue/web/Board.tsx'] if p.exists()}
 report['scope']='All registered game rooms: shared fullscreen only; actual tank match: canvas/layout; Chromium desktop and mobile viewport. Capability overrides test fallback, not physical iOS.'
 save('report',report)
raise SystemExit(0 if report['passed'] else 1)
