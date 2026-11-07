#!/usr/bin/env python3
"""Real browser and HTTP acceptance; use only against a disposable local server."""
import argparse, hashlib, http.cookiejar, json, os, subprocess, time, urllib.request, uuid
from pathlib import Path
from urllib.parse import urlsplit
root=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--url',default='http://127.0.0.1:18099');a=p.parse_args()
assert urlsplit(a.url).hostname in ('127.0.0.1','localhost')
out=root/'.local/platform-admin-acceptance';out.mkdir(parents=True,exist_ok=True)
common=Path(subprocess.check_output(['git','rev-parse','--path-format=absolute','--git-common-dir'],cwd=root,text=True).strip()).parent
env=dict(os.environ,AGENT_BROWSER_SESSION='pa',AGENT_BROWSER_SOCKET_DIR=str(common/'.local/ab-pa'),AGENT_BROWSER_PROFILE=str(out/'profile'),TMPDIR=str(out/'tmp'))
Path(env['TMPDIR']).mkdir(exist_ok=True)
report={'passed':False,'checks':[],'measurements':[],'indexSHA256':hashlib.sha256((root/'web/dist/index.html').read_bytes()).hexdigest()}
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,text=True,capture_output=True,timeout=45)
 assert r.returncode==0,r.stdout+r.stderr
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def api(c,path,body=None):
 req=urllib.request.Request(a.url+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
 with c.open(req,timeout=20) as r:return json.load(r)
def client():return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def check(name,condition):
 assert condition,name;report['checks'].append(name)
try:
 peer=client();tag=uuid.uuid4().hex[:6];peer_name='好友'+tag
 peer_u=api(peer,'/register',{'name':peer_name,'password':'accept-test-password','invite':'page-accept-code'})
 b('open',a.url)
 check('admin login',js('fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({name:"DialogAdmin",password:"dialog-local-password"})}).then(r=>r.ok)'))
 b('reload');b('wait','.profile-nav');b('click','.profile-nav');b('wait','.profile-friends')
 me=js('fetch("/api/state").then(r=>r.json()).then(d=>d.user.id)')
 api(peer,'/players/'+me+'/friend',{'action':'request'})
 js('fetch("/api/players/'+peer_u['id']+'/friend",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({action:"accept"})})')
 b('click','.profile-heading button');b('wait','--text',peer_name)
 check('friends precede stats',js('!!(document.querySelector(".profile-friends").compareDocumentPosition(document.querySelector(".profile-stats")) & Node.DOCUMENT_POSITION_FOLLOWING)'))
 for width in (1440,320):
  b('set','viewport',str(width),'1000');check(f'profile width {width}',js('(()=>{let r=document.querySelector(".modal[aria-label=玩家页面]").getBoundingClientRect();return r.width>='+str(1000 if width==1440 else 280)+'&&document.documentElement.scrollWidth<=innerWidth})()'))
  b('screenshot',str(out/f'profile-{width}.png'))
 b('click','.modal[aria-label=玩家页面] button[aria-label=关闭]');b('set','viewport','1440','1000');b('click','.admin-nav-button');b('wait','.admin-tabs')
 check('menu order',js('[...document.querySelectorAll(".admin-tabs button")].map(e=>e.textContent.trim())')==['用户管理','游戏管理','牌桌管理','许愿管理','操作记录'])
 b('click','.admin-metrics button:nth-child(2)');check('online metric navigation',js('document.querySelector(".admin-tabs [aria-current=page]").textContent.includes("用户管理")&&document.querySelector(".admin-check input").checked'))
 b('uncheck','.admin-check input');b('fill','.admin-search input',peer_name);b('wait','--text',peer_name)
 check('user IP visible',js('document.querySelector(".admin-user-list").textContent.includes("127.0.0.1")'))
 b('find','role','button','click','--name','查看动态','--exact');b('wait','.admin-user-activity article');check('activity visible',js('document.querySelector(".admin-user-activity").textContent.includes("注册")'))
 b('screenshot',str(out/'activity-1440.png'));b('click','.modal[aria-label$=的动态] button[aria-label=关闭]')
 # Delay just the next manual refresh so pending feedback can be observed.
 js('(()=>{const f=window.fetch;window.fetch=(u,o)=>String(u)==="/api/admin"?new Promise(resolve=>setTimeout(()=>resolve(f(u,o)),1500)):f(u,o)})()')
 b('click','.admin-list-refresh');check('refresh pending',js('document.querySelector(".admin-list-refresh").disabled&&document.querySelector(".admin-list-refresh").getAttribute("aria-busy")==="true"'))
 b('wait','--fn','!document.querySelector(".admin-list-refresh").disabled')
 b('find','role','button','click','--name','游戏管理','--exact');check('no inline ordering controls',js('!document.querySelector(".admin-game-order-controls")'))
 b('find','role','button','click','--name','游戏排序','--exact');b('wait','.admin-sort-grid')
 before=js('[...document.querySelectorAll(".admin-sort-grid [data-game-id]")].map(e=>e.dataset.gameId)')
 b('drag','.admin-sort-grid [data-game-id="'+before[0]+'"]','.admin-sort-grid [data-game-id="'+before[1]+'"]')
 b('wait','--fn','document.querySelector(".admin-sort-grid article").dataset.gameId==='+json.dumps(before[1]))
 check('drag reorders covers',js('[...document.querySelectorAll(".admin-sort-grid [data-game-id]")].map(e=>e.dataset.gameId)')[0]==before[1])
 b('screenshot',str(out/'sort-1440.png'));b('find','role','button','click','--name','保存游戏顺序','--exact');b('wait','--fn','!document.querySelector(".admin-sort-grid")')
 check('sort persisted',js('fetch("/api/admin").then(r=>r.json()).then(d=>d.games[0].kind)')==before[1])
 b('find','role','button','click','--name','游戏排序','--exact');b('wait','.admin-sort-grid');b('set','viewport','320','1000');b('screenshot',str(out/'sort-320.png'))
 check('sort mobile no overflow',js('document.documentElement.scrollWidth<=innerWidth'))
 cards=js('[...document.querySelectorAll(".admin-sort-grid [data-game-id]")].map(e=>e.dataset.gameId)')
 b('focus','.admin-sort-grid article:first-child button');b('press','ArrowRight')
 b('wait','--fn','document.querySelector(".admin-sort-grid article").dataset.gameId==='+json.dumps(cards[1]))
 check('keyboard sort',js('document.querySelector(".admin-sort-grid article").dataset.gameId')==cards[1])
 # The next command performs native touch input in this owned browser.
 boxes=js('[...document.querySelectorAll(".admin-sort-grid article")].slice(0,2).map(e=>{const r=e.querySelector("button").getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})')
 endpoint=b('get','cdp-url')['cdpUrl']
 steps=[{'type':'touchStart','touchPoints':[{'x':boxes[0]['x'],'y':boxes[0]['y']}]},{'type':'touchMove','touchPoints':[{'x':boxes[1]['x'],'y':boxes[1]['y']}]},{'type':'touchEnd','touchPoints':[]}]
 r=subprocess.run(['node',str(root/'scripts/controllers/gamepad/native-touch.mjs'),endpoint,a.url,json.dumps(steps),'.admin-sort-grid'],text=True,capture_output=True,timeout=20)
 assert r.returncode==0,r.stdout+r.stderr
 b('wait','--fn','document.querySelector(".admin-sort-grid article").dataset.gameId==='+json.dumps(cards[0]))
 check('touch sort',js('document.querySelector(".admin-sort-grid article").dataset.gameId')==cards[0])
 b('find','role','button','click','--name','取消排序','--exact');b('set','viewport','1440','1000');b('find','role','button','click','--name','返回牌桌与大厅','--exact')
 js('fetch("/api/state").then(r=>r.json()).then(d=>d.room&&fetch("/api/rooms/"+d.room.id,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({type:"leave",version:d.room.version,nonce:crypto.randomUUID()})}))')
 b('reload')
 # Both waiting-room renderers consume the same seat CSS.
 for kind in js('fetch("/api/state").then(r=>r.json()).then(d=>d.availableGames)'):
  result=js('(async()=>{for(const capacity of [2,3,4,6,8]){const r=await fetch("/api/rooms",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({kind:'+json.dumps(kind)+',capacity,name:"座位布局验收"})});if(r.ok)return true}return false})()');check(kind+' create',result)
  b('reload');b('wait','.waiting-seat');h=js('document.querySelector(".waiting-seat").getBoundingClientRect().height')
  if not js('[...document.querySelectorAll("button")].some(e=>e.textContent.trim()==="添加电脑玩家")'):
   check(kind+' human-only waiting layout',kind=='contra')
   js('fetch("/api/state").then(r=>r.json()).then(d=>fetch("/api/rooms/"+d.room.id,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({type:"leave",version:d.room.version,nonce:crypto.randomUUID()})}))')
   b('reload');continue
  b('find','role','button','click','--name','添加电脑玩家','--exact');b('wait','.bot-remove')
  for width in (1440,320):
   b('set','viewport',str(width),'1000')
   report['measurements'].append({'game':kind,'width':width,'heights':js('[...document.querySelectorAll(".waiting-seat")].map(e=>e.getBoundingClientRect().height)')})
   if width==1440:check(kind+' adding bot preserves height',js('document.querySelector(".waiting-seat").getBoundingClientRect().height')==h)
   check(kind+f' equal seat heights {width}',js('(()=>{const s=[...document.querySelectorAll(".waiting-seat")].map(e=>e.getBoundingClientRect().height);return Math.max(...s)-Math.min(...s)<1})()'))
   check(kind+f' icon overlay {width}',js('getComputedStyle(document.querySelector(".bot-remove")).position==="absolute"&&document.querySelector(".bot-remove").textContent.trim()===""&&document.documentElement.scrollWidth<=innerWidth'))
   if kind in ('gomoku','splendor','guandan'):b('screenshot',str(out/f'{kind}-{width}.png'))
  b('click','.bot-remove');b('wait','--fn','!document.querySelector(".bot-remove")');check(kind+' remove icon works',True)
  js('fetch("/api/state").then(r=>r.json()).then(d=>fetch("/api/rooms/"+d.room.id,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({type:"leave",version:d.room.version,nonce:crypto.randomUUID()})}))')
  b('reload');b('set','viewport','1440','1000')
 check('no browser errors',not b('errors')['errors']);report['passed']=True
finally:
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));b('close');print(out/'result.json')
