#!/usr/bin/env python3
"""Real shared category filters and initial room layouts, not rule matrices."""
from pathlib import Path
import os,subprocess,json,uuid,time,shutil,urllib.request,socket,hashlib
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'.local/category-navigation';TEMP=OUT/'temporary';TEMP.mkdir(parents=True,exist_ok=True);workspace=ROOT.parent.parent if ROOT.parent.name=='.local' else ROOT;SOCKET=workspace/'.local/cat-s';SOCKET.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ,GOCACHE=str(workspace/'.local/go-build'),GOMODCACHE=str(workspace/'.local/go-mod'),GOTMPDIR=str(TEMP),TMPDIR=str(TEMP),GOTOOLCHAIN='local',AGENT_BROWSER_SESSION='categories',AGENT_BROWSER_SOCKET_DIR=str(SOCKET),AGENT_BROWSER_PROFILE=str(TEMP/'profile'),INVITE_CODE='category-local',ADMIN_USERNAME='分类验收管理员',ADMIN_PASSWORD='category-local-test',DATA_DIR=str(TEMP/'data'))
expected=['中国象棋','五子棋','升级','围棋','国际象棋','斗地主','掼蛋','军棋'];expected=sorted(expected);sourceFiles=[ROOT/'web/src/ui/game-category.tsx',ROOT/'web/src/game-api/module.ts',ROOT/'web/src/main.tsx',*[ROOT/'games'/g/'web/catalog.tsx' for g in ['gomoku','xiangqi','chess','junqi','weiqi','doudizhu','guandan','shengji']]]
sources={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sourceFiles}
report={'sourceSHA256':sources,'passed':False,'scope':'production app; real category filters and initial room layout only; no full-game rule matrix','filters':[],'rooms':[]};opened=False;process=None

def b(*a):
 p=subprocess.run(['agent-browser',*map(str,a),'--json'],cwd=ROOT,env=ENV,capture_output=True,text=True,timeout=45);assert p.returncode==0,p.stdout+p.stderr;r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(s):return b('eval',s)['result']
def api(path,body=None):
 opt='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
 r=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+opt+');return {status:r.status,body:await r.json()}})()');assert r['status'] in (200,201),r;return r['body']
def room():return api('/state')['room']
def cmd(kind,**kw):
 r=room();return api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**kw))
def wait(fn,seconds=40):
 until=time.monotonic()+seconds
 while time.monotonic()<until:
  if fn():return
  time.sleep(.1)
 raise AssertionError('condition timed out')
try:
 subprocess.run(['go','build','-o',str(TEMP/'app'),'.'],cwd=ROOT,env=ENV,check=True,timeout=180)
 with socket.socket() as probe:probe.bind(('127.0.0.1',0));port=probe.getsockname()[1]
 ENV['ADDR']='127.0.0.1:'+str(port);BASE='http://'+ENV['ADDR'];log=(TEMP/'server.log').open('w');process=subprocess.Popen([str(TEMP/'app')],cwd=ROOT,env=ENV,stdout=log,stderr=subprocess.STDOUT)
 def healthy():
  try:return urllib.request.urlopen(BASE+'/healthz',timeout=1).status==200
  except:return False
 wait(healthy);b('open',BASE);opened=True;api('/login',{'name':ENV['ADMIN_USERNAME'],'password':ENV['ADMIN_PASSWORD']});b('reload');b('wait','select[aria-label="游戏分类"]')
 for width,height in [(1440,1000),(320,760)]:
  b('set','viewport',width,height)
  assert js('document.querySelector("select[aria-label=游戏分类] option[value=poker]").textContent')=='棋牌类'
  b('select','select[aria-label="游戏分类"]','poker')
  lobby=js('[...document.querySelectorAll("select[aria-label=按游戏筛选牌桌] option")].slice(1).map(o=>o.textContent)');assert sorted(lobby)==expected,lobby
  b('select','select[aria-label="游戏分类"]','board');board=js('[...document.querySelectorAll("select[aria-label=按游戏筛选牌桌] option")].slice(1).map(o=>o.textContent)');assert not set(expected)&set(board)
  b('click','.lobby-create');b('wait','.game-choices');b('select','select[aria-label="可选游戏分类"]','poker');cards=js('[...document.querySelectorAll(".choose-games button > span:last-child")].map(e=>e.textContent.trim())');assert sorted(cards)==expected,cards
  assert not js('document.documentElement.scrollWidth>innerWidth')
  b('screenshot',OUT/('create-'+str(width)+'.png'));b('press','Escape');wait(lambda:not js('!!document.querySelector(".game-choices")'))
  b('click','.admin-nav-button');b('wait','select[aria-label="管理游戏分类"]');b('select','select[aria-label="管理游戏分类"]','poker');managed=js('[...document.querySelectorAll(".admin-game-card h3")].map(e=>e.textContent)');assert sorted(managed)==expected,managed
  b('screenshot',OUT/('admin-'+str(width)+'.png'));b('select','select[aria-label="管理游戏分类"]','board');remaining=js('[...document.querySelectorAll(".admin-game-card h3")].map(e=>e.textContent)');assert not set(expected)&set(remaining);b('click','.admin-nav-button');report['filters'].append({'width':width,'lobby':lobby,'creation':cards,'admin':managed,'removedFromBoard':True})
 # Initial native layouts at phone width: classification must not apply card-table compaction to chess games.
 for kind,count,poker in [('gomoku',2,False),('xiangqi',2,False),('chess',2,False),('junqi',2,False),('weiqi',2,False),('doudizhu',4,True),('guandan',4,True),('shengji',4,True)]:
  api('/rooms',{'kind':kind,'name':'分类布局验收','capacity':count})
  for _ in range(count-1):cmd('add_bot')
  cmd('ready');cmd('start');wait(lambda:not room().get('loading'));wait(lambda:not js('!!document.querySelector("[data-game-loading]")'))
  layout=js('document.querySelector(".room-page")?.dataset.tableLayout??null');assert (layout=='poker')==poker,(kind,layout)
  b('screenshot',OUT/(kind+'-320.png'));report['rooms'].append({'kind':kind,'layout':layout,'players':count});cmd('close');cmd('leave');wait(lambda:js('!!document.querySelector("select[aria-label=游戏分类]")'))
 assert all(hashlib.sha256((ROOT/p).read_bytes()).hexdigest()==h for p,h in sources.items())
 report['passed']=True
finally:
 if opened and not report['passed']:
  try:report['page']=js('document.body.innerText');b('screenshot',OUT/'failure.png','--full')
  except:pass
 if opened:
  try:b('close')
  except:pass
 if process:process.terminate();process.wait(timeout=10)
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');shutil.rmtree(TEMP,ignore_errors=True)
 for p in SOCKET.glob('categories.*'):p.unlink()
 if not any(SOCKET.iterdir()):SOCKET.rmdir()
