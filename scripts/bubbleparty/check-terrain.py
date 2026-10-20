#!/usr/bin/env python3
"""Eight real seats, native browser control of seat 2; no state injection.
Failure inventory: docs/research/realtime-efficiency/MOTION.md.
"""
import ast, hashlib, http.cookiejar, json, os, shutil, subprocess, time, traceback, urllib.request, uuid
from pathlib import Path
root=Path(__file__).resolve().parents[2]
run='tree'+uuid.uuid4().hex[:6];out=root/'.local/realtime-efficiency'/run;out.mkdir(parents=True)
temp=out/'temporary';sockets=root/'.local/tree-sockets'/run
env=dict(os.environ)
for k,p in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():p.mkdir(parents=True,exist_ok=True);env[k]=str(p)
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local')
report={'passed':False,'scope':'production registry, HTTP/SQLite, eight real human seats; native key events; no coordinate injection','checks':[],'screenshots':[]}
report['sources']={str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for folder in ['games/bubbleparty/server','games/bubbleparty/web'] for p in (root/folder).glob('*') if p.is_file()}
report['avatarManifestSHA256']=hashlib.sha256((root/'games/bubbleparty/assets/manifest-v3.json').read_bytes()).hexdigest()
report['distIndexSHA256']=hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()
server=log=None;opened=False

def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45)
 assert r.returncode==0,r.stderr+r.stdout
 result=json.loads(r.stdout);assert result['success'],result;return result['data']
def js(code):return browser('eval',code)['result']
def api(path,body=None):
 opts='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opts+');return {status:r.status,data:await r.json()}})()')
 assert r['status'] in [200,201],r
 return r['data']
def request(client,path,body=None):
 req=urllib.request.Request(base+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
 with client.open(req,timeout=10) as r:return json.load(r)
def command(client,kind):
 r=request(client,'/state')['room'];return request(client,'/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
def wait(fn,seconds=10):
 until=time.monotonic()+seconds
 while time.monotonic()<until:
  r=fn()
  if r:return r
  time.sleep(.05)
 raise AssertionError('condition timed out')
def room():return api('/state')['room']
def check(name,**detail):report['checks'].append(dict(name=name,**detail));print('PASS',name,flush=True)
def shot(name):
 if js('!!document.querySelector("[role=dialog]")'):
  wait(lambda:js('(()=>{const e=document.querySelector("[role=dialog]");return e&&[e,e.parentElement].every(n=>Number(getComputedStyle(n).opacity)>.99&&n.getAnimations().every(a=>a.playState==="finished"))})()'))
  layout=js('(()=>{const e=document.querySelector("[role=dialog]"),r=e.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:innerWidth,height:innerHeight,opacity:getComputedStyle(e).opacity,background:getComputedStyle(e).backgroundColor,color:getComputedStyle(e).color,buttons:[...e.querySelectorAll("[data-bubbleparty-avatar]")].map(b=>{const r=b.getBoundingClientRect(),hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return {visible:r.x>=0&&r.right<=innerWidth&&r.y>=0&&r.bottom<=innerHeight,hit:hit===b||b.contains(hit)}})}})()')
  assert layout['left']>=0 and layout['right']<=layout['width'] and layout['top']>=0 and layout['bottom']<=layout['height'],layout
  assert all(b['visible'] and b['hit'] for b in layout['buttons']),layout
  report.setdefault('dialogLayouts',[]).append({'screenshot':name,**layout})
 browser('screenshot' ,str(out/(name+'.png')),'--full');report['screenshots'].append(name+'.png')
def native(key,hold_ms):
 tree=ast.parse((root/'scripts/realtime/check-browser.py').read_text())
 cdp=next(ast.literal_eval(n.value) for n in tree.body if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='CDP' for t in n.targets))
 cdp=cdp.replace('options.prediction?40:160','Math.max(0,options.holdMS-80)')
 r=subprocess.run(['node','--input-type=module','-',browser('get','cdp-url')['cdpUrl'],base,json.dumps({'key':key,'holdMS':hold_ms,'prediction':False,'milliseconds':700})],input=cdp,cwd=root,env=env,text=True,capture_output=True,timeout=20)
 assert r.returncode==0,r.stdout+r.stderr
 return json.loads(r.stdout)
def pose():
 r=room();p=r['game']['data']['players'][2]
 drawn=js("(()=>{const c=document.querySelector('[data-bubbleparty-canvas]');return {x:Number(c.dataset.renderX),y:Number(c.dataset.renderY)}})()")
 assert abs(drawn['x']-p['px'])<.015 and abs(drawn['y']-p['py'])<.015,(p,drawn)
 return r,p,drawn
try:
 log=(out/'server.log').open('w');server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 wait(lambda:(out/'url').exists(),120);base=(out/'url').read_text()
 host=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
 request(host,'/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'})
 made=request(host,'/rooms',{'kind':'bubbleparty','name':'树木碰撞与人物落格验收','capacity':8,'options':{'map':'community10','roundSeconds':300}})
 peers=[]
 for seat in [1,3,4,5,6,7]:
  c=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
  request(c,'/register',{'name':f'地形验收{seat}','password':'terrain-test-only','invite':'bubbleparty-browser'})
  # Seat 2 is occupied by the actual browser below, before later humans join.
  peers.append((seat,c))
 browser('open',base);opened=True;browser('set','viewport','1280','1000')
 api('/register',{'name':'原生键盘地形验收','password':'terrain-test-only','invite':'bubbleparty-browser'})
 for seat in range(1,8):
  r=request(host,'/state')['room'];body={'type':'join','version':r['version'],'nonce':uuid.uuid4().hex}
  if seat==2:api('/rooms/'+made['id'],body)
  else:request(next(c for i,c in peers if i==seat),'/rooms/'+made['id'],body)
 for _,c in peers:command(c,'ready')
 r=room();api('/rooms/'+r['id'],{'type':'ready','version':r['version'],'nonce':uuid.uuid4().hex})
 command(host,'ready');command(host,'start');browser('reload');browser('wait','.game-layout')
 wait(lambda:room()['game']['data']['phase']=='playing')
 assert room()['you']==2
 wait(lambda:js('performance.getEntriesByType("resource").filter(e=>e.name.includes("/bubbleparty/v3/avatars/")&&e.responseEnd>0).length>=48'))
 initial=room()['game']['data'];original_player=initial['players'][2]
 browser('click','[data-bubbleparty-avatar-open]');browser('wait','[role=dialog][aria-label="选择人物"]')
 assert js('document.querySelectorAll("[data-bubbleparty-avatar]").length')==4
 shot('red-avatar-picker-desktop')
 browser('click','[data-bubbleparty-avatar="3"]')
 wait(lambda:room()['game']['data']['players'][2].get('avatar')==3)
 wait(lambda:js('!document.querySelector("[role=dialog][aria-label=选择人物]")'))
 chosen=room()['game']['data']['players'][2]
 for field in ['team','px','py','speedMs','maxBombs','range']:assert chosen[field]==original_player[field],(field,chosen,original_player)
 browser('set','viewport','320','844');browser('click','[data-bubbleparty-avatar-open]');shot('red-avatar-picker-320')
 browser('click','[data-bubbleparty-avatar="1"]');wait(lambda:room()['game']['data']['players'][2].get('avatar')==1)
 browser('reload');browser('wait','.game-layout')
 assert room()['game']['data']['players'][2]['avatar']==1
 check('four original appearances selectable, fixed red team and refresh persistence',player=room()['game']['data']['players'][2])
 browser('set','viewport','1280','1000')
 (out/'pine-keyboard-network.json').write_text(json.dumps(native('ArrowLeft',500),ensure_ascii=False,indent=2))
 r,p,drawn=pose();v=r['game']['data']
 assert p['x']==5 and p['px']>=4.78-.000001 and v['cells'][6*15+4]==1 and v['tiles'][6*15+4]==3,(p,v['cells'][94],v['tiles'][94])
 check('native movement stops at pine and drawn pose reconciles',player=p,drawn=drawn)
 shot('pine-collision-desktop')
 (out/'herb-keyboard-network.json').write_text(json.dumps(native('ArrowDown',200),ensure_ascii=False,indent=2))
 r,p,drawn=pose();v=r['game']['data']
 assert p['y']==7 and v['cells'][7*15+5]==0 and v['tiles'][7*15+5]==2,(p,v['cells'][110],v['tiles'][110])
 check('native movement enters passable fruit herb',player=p,drawn=drawn)
 shot('herb-occlusion-desktop')
 browser('press','Space')
 v=wait(lambda:(lambda v:v if v['bombs'] else None)(room()['game']['data']))
 bomb=next(b for b in v['bombs'] if b['seat']==2);p=v['players'][2]
 assert [bomb['x'],bomb['y']]==[p['x'],p['y']],(bomb,p)
 check('bomb placement matches actual player grid',bomb=bomb,player=p)
 browser('set','viewport','320','844');browser('scrollintoview','[data-bubbleparty-canvas]')
 assert js('innerWidth===320&&document.documentElement.scrollWidth<=322')
 shot('actual-grid-and-no-number-badges-320')
 # Use another real account in the blue team; no seat or state is forged.
 api('/logout',{});api('/login',{'name':'地形验收6','password':'terrain-test-only'});browser('reload');browser('wait','.game-layout')
 assert room()['you']==6 and room()['game']['data']['players'][6]['team']==1
 browser('set','viewport','320','844');browser('click','[data-bubbleparty-avatar-open]');shot('blue-avatar-picker-320')
 for choice in [0,1,2,3,0]:
  if not js('!!document.querySelector("[role=dialog][aria-label=选择人物]")'):browser('click','[data-bubbleparty-avatar-open]')
  browser('click',f'[data-bubbleparty-avatar="{choice}"]')
  wait(lambda:room()['game']['data']['players'][6].get('avatar')==choice)
  assert room()['game']['data']['players'][6]['team']==1
 check('all four choices retain blue team')
 browser('set','viewport','1280','1000');native('ArrowLeft',120);native('ArrowRight',60)
 shot('blue-avatar-facing-right')
 browser('press','Space')
 trapped=wait(lambda:(lambda p:p if p['trappedUntil']>0 else None)(room()['game']['data']['players'][6]),6)
 shot('blue-avatar-trapped')
 browser('click','[data-bubbleparty-avatar-open]');browser('wait','[role=dialog][aria-label=选择人物]')
 dead=wait(lambda:(lambda p:p if not p['alive'] else None)(room()['game']['data']['players'][6]),8)
 wait(lambda:js('!document.querySelector("[role=dialog][aria-label=选择人物]")'),2)
 shot('blue-avatar-death')
 assert trapped['team']==dead['team']==1 and trapped['avatar']==dead['avatar']==0
 check('blue appearance retained through right-facing movement, trapped and death',trapped=trapped,dead=dead)
 assert not browser('errors')['errors'],browser('errors')
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc();print(report['error'],flush=True)
 if opened:
  try:shot('failure')
  except BaseException:pass
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:
  (out/'stop').write_text('stop')
  try:server.wait(timeout=20)
  except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
  if server.returncode:report['passed']=False
 if log:log.close()
 for p in [temp,sockets,out/'data']:
  if p.exists():shutil.rmtree(p)
 for name in ['url','stop']:(out/name).unlink(missing_ok=True)
 (out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(out/'report.json',flush=True)
 if not report['passed']:raise SystemExit(1)
