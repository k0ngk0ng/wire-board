#!/usr/bin/env python3
"""Eight real seats, native browser control of seat 2; no state injection.
Failure inventory: docs/research/bubbleparty/contact-audit-20261021.md.
"""
import argparse, ast, hashlib, http.cookiejar, json, os, shutil, subprocess, time, traceback, urllib.request, uuid
from pathlib import Path
root=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser();parser.add_argument('--touch',action='store_true');args=parser.parse_args()
run='ct'+uuid.uuid4().hex[:6];out=root/'.local/realtime-efficiency'/run;out.mkdir(parents=True)
temp=out/'temporary';sockets=root/'.local/cs'/run
env=dict(os.environ)
for k,p in {'TMPDIR':temp/'tmp','GOTMPDIR':temp/'tmp','GOCACHE':root/'.local/go-build','GOMODCACHE':root/'.local/go-mod','AGENT_BROWSER_SOCKET_DIR':sockets,'AGENT_BROWSER_PROFILE':temp/'profile'}.items():p.mkdir(parents=True,exist_ok=True);env[k]=str(p)
env.update(AGENT_BROWSER_SESSION=run,AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',WIRE_BOARD_BUBBLEPARTY_BROWSER='1',WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),GOTOOLCHAIN='local')
report={'passed':False,'scope':'production registry, HTTP/SQLite, eight real human seats; native keyboard/touch contact; no game state injection','checks':[],'screenshots':[]}
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
def native(key,hold_ms,touch=False):
 tree=ast.parse((root/'scripts/realtime/check-browser.py').read_text())
 cdp=next(ast.literal_eval(n.value) for n in tree.body if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='CDP' for t in n.targets))
 cdp=cdp.replace('options.prediction?40:160','Math.max(0,options.holdMS-80)')
 r=subprocess.run(['node','--input-type=module','-',browser('get','cdp-url')['cdpUrl'],base,json.dumps({'key':key,'holdMS':hold_ms,'prediction':False,'milliseconds':80,'touch':touch})],input=cdp,cwd=root,env=env,text=True,capture_output=True,timeout=20)
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
 made=request(host,'/rooms',{'kind':'bubbleparty','name':'敌方困泡原生触碰验收','capacity':8,'options':{'map':'community10','roundSeconds':300}})
 peers=[]
 for seat in [1,3,4,5,6,7]:
  c=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
  request(c,'/register',{'name':f'地形验收{seat}','password':'terrain-test-only','invite':'bubbleparty-browser'})
  # Seat 2 is occupied by the actual browser below, before later humans join.
  peers.append((seat,c))
 browser('open',base);opened=True;browser('set','viewport','320' if args.touch else '1280','844' if args.touch else '1000')
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
 initial=request(host,'/state')['room']['game']['data']
 report['initialPlayers']=initial['players']
 def current():return request(host,'/state')['room']['game']['data']
 def trace(name,key,ms,touch=False):
  result=native(key,ms,touch or args.touch)
  (out/(name+'.json')).write_text(json.dumps(result,ensure_ascii=False,indent=2))
  report.setdefault('inputs',[]).append({'file':name+'.json','key':key,'holdMS':ms,'touch':touch or args.touch,'players':current()['players']})
 def action(client,move='none',bomb=False):
  r=request(client,'/state')['room'];v=r['game']['data'];seat=r['you']
  body={'type':'action','version':r['version'],'nonce':uuid.uuid4().hex,'action':{'type':'input','epoch':v['epoch'],'seq':v['players'][seat]['seq']+1,'move':move,'bomb':bomb}}
  result=request(client,'/rooms/'+r['id'],body)
  report.setdefault('httpActions',[]).append({'seat':seat,'body':body,'serverAt':v['serverAt']})
  return result
 trace('red-approaches-safe-corner','ArrowRight',350)
 actor=current()['players'][2]
 assert 7.1<actor['px']<7.3 and actor['py']==6,actor
 enemy=next(c for i,c in peers if i==6)
 action(enemy,bomb=True)
 wait(lambda:current()['players'][6]['trappedUntil']>0,5)
 wait(lambda:not current()['blasts'],3)
 before=current();assert before['players'][6]['alive'] and before['players'][2]['trappedUntil']==0,before['players']
 shot('enemy-trapped-water-gone')
 trace('red-turns-into-enemy-lane','ArrowDown',175)
 trace('red-contacts-enemy','ArrowRight',160)
 after=wait(lambda:(lambda v:v if not v['players'][6]['alive'] else None)(current()),2)
 assert after['players'][2]['alive'] and after['players'][2]['eliminations']==before['players'][2]['eliminations']+1,after['players']
 event=next(e for e in reversed(after['events']) if e['type']=='eliminate' and e['seat']==6)
 assert event['value']==2,event
 check(('native 320px touch' if args.touch else 'native keyboard')+' enemy touch bursts bubble and credits actual attacker',before=before['players'],after=after['players'],event=event)
 shot('enemy-burst-credit')
 if args.touch:assert js('innerWidth===320&&document.documentElement.scrollWidth<=322')
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
