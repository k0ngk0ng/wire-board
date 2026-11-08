#!/usr/bin/env python3
"""Real admin UI and HTTP: live game identity, deleted-table activity and archives."""
import hashlib,http.cookiejar,json,os,subprocess,time,uuid,urllib.request
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];BASE='http://127.0.0.1:18099';OUT=ROOT/'.local/admin-room-history/browser';OUT.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ,AGENT_BROWSER_SESSION='arh'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/ahrb'),AGENT_BROWSER_PROFILE=str(ROOT/'.local/ahrp'),TMPDIR=str(ROOT/'.local/ahrt'),AGENT_BROWSER_SCREENSHOT_DIR=str(OUT),AGENT_BROWSER_RESTORE_SAVE='never',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for k in ['AGENT_BROWSER_SOCKET_DIR','AGENT_BROWSER_PROFILE','TMPDIR']:Path(ENV[k]).mkdir(parents=True,exist_ok=True)
REPORT={'passed':False,'scope':'production server/routes, independent SQLite, real admin browser input, desktop and 320px','sourceSHA256':{str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [ROOT/'internal/server/admin_room_history.go',ROOT/'internal/server/access_activity.go',ROOT/'web/src/admin/room-history.tsx',ROOT/'web/src/admin/user-activity.tsx']}}
def client():return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def api(c,path,body=None):
 with c.open(urllib.request.Request(BASE+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'}),timeout=25) as r:return json.load(r)
def command(c,kind):
 room=api(c,'/state')['room'];return api(c,'/rooms/'+room['id'],{'type':kind,'version':room['version'],'nonce':uuid.uuid4().hex})
def b(*args):
 p=subprocess.run(['agent-browser',*map(str,args),'--json'],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=60);assert p.returncode==0,p.stdout+p.stderr
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return b('eval',code)['result']
def wait(fn,seconds=30):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if fn():return
  time.sleep(.1)
 raise AssertionError('admin UI condition timed out')
owner=client();name='牌桌记录验收'+uuid.uuid4().hex[:6]
try:
 uid=api(owner,'/register',{'name':name,'password':'history-browser-test-only','invite':'page-accept-code'})['id'];tables=[]
 for kind,title in [('splendor','删除后仍可查询%_'),('carcassonne','另一场游戏记录')]:
  api(owner,'/rooms',{'kind':kind,'name':title,'capacity':2,'password':'history-private-room-only'});rid=api(owner,'/state')['room']['id']
  for action in ['add_bot','ready','start','close','leave']:command(owner,action)
  tables.append({'id':rid,'kind':kind,'name':title})
 api(owner,'/rooms',{'kind':'carcassonne','name':'正在等待的卡卡颂','capacity':2})
 b('open',BASE);assert js('(async()=>{const r=await fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({name:"DialogAdmin",password:"dialog-local-password"})});return r.ok})()')
 b('reload');b('wait','.admin-nav-button');b('click','.admin-nav-button');b('wait','.admin-tabs');b('find','role','button','click','--name','用户管理','--exact');b('fill','.admin-search input',name)
 wait(lambda:js('document.querySelector(".admin-users")?.innerText.includes("卡卡颂")||[...document.querySelectorAll(".admin-user-card")].some(e=>e.innerText.includes("卡卡颂"))'))
 b('find','role','button','click','--name','查看动态','--exact');b('wait','.user-activity-modal')
 wait(lambda:js('document.querySelector(".user-activity-modal").innerText.includes("璀璨宝石")&&document.querySelector(".user-activity-modal").innerText.includes("卡卡颂")'))
 b('screenshot',OUT/'activity-desktop.png','--full')
 js('[...document.querySelectorAll(".admin-user-activity article")].find(e=>e.innerText.includes("删除后仍可查询%_")).dataset.historyAccept="yes"')
 b('click','[data-history-accept=yes] button');b('wait','.admin-history-modal')
 wait(lambda:js('document.querySelectorAll(".admin-history-modal [data-admin-history-id]").length===1'))
 assert js('document.querySelector(".admin-history-modal").innerText.includes("璀璨宝石")&&document.querySelector(".admin-history-modal").innerText.includes("删除后仍可查询%_")')
 b('screenshot',OUT/'activity-linked-history.png','--full');b('click','.admin-history-modal button[aria-label="关闭"]')
 b('find','role','button','click','--name','牌桌管理','--exact');b('find','role','button','click','--name','历史牌桌','--exact')
 wait(lambda:js('document.querySelectorAll("[data-admin-history-id]").length===2'))
 b('select','select[aria-label="历史牌桌游戏"]','carcassonne');wait(lambda:js('document.querySelectorAll("[data-admin-history-id]").length===1&&document.querySelector("[data-admin-history-id]").dataset.adminHistoryKind==="carcassonne"'))
 b('select','select[aria-label="历史牌桌游戏"]','');b('fill','input[aria-label="搜索历史牌桌"]','%_')
 wait(lambda:js('document.querySelectorAll("[data-admin-history-id]").length===1&&document.querySelector("[data-admin-history-id]").innerText.includes("删除后仍可查询%_")'))
 REPORT['tables']=tables;REPORT['widths']=[]
 for width in [1440,320]:
  b('set','viewport',width,900);assert js('document.documentElement.scrollWidth<=innerWidth+1'),'admin history overflows viewport'
  b('scrollintoview','[data-admin-room-history]');b('screenshot',OUT/f'history-{width}.png','--full')
  REPORT['widths'].append({'width':width,'visibleHistory':js('document.querySelectorAll("[data-admin-history-id]").length'),'overflow':js('document.documentElement.scrollWidth-innerWidth')})
 b('fill','input[aria-label="搜索历史牌桌"]','没有这样的牌桌');wait(lambda:js('document.querySelectorAll("[data-admin-history-id]").length===0&&document.querySelector(".admin-history").innerText.includes("暂无符合条件")'))
 REPORT['errors']=b('errors');assert not REPORT['errors'].get('errors');REPORT['passed']=True
finally:
 try:command(owner,'leave')
 except Exception:pass
 if not REPORT['passed']:
  try:b('screenshot',OUT/'failure.png','--full')
  except Exception:pass
 (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
 try:b('close')
 except Exception:pass
print(json.dumps({'passed':REPORT['passed'],'widths':REPORT.get('widths')}))
