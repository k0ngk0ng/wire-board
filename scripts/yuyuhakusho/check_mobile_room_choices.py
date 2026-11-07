"""Native touch in the normal authenticated app; authoritative and reload proof.

Failures: create-dialog portal loses tap, ready-room selections fail or stay
optimistic only, host cannot choose bot, reload resets options, images clip,
desktop regresses. All accounts, SQLite files and hosts are disposable/local.
"""
from pathlib import Path
import argparse, hashlib, http.cookiejar, json, os, subprocess, time, urllib.request
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--label',required=True);a=p.parse_args()
OUT=ROOT/'games/yuyuhakusho/verification/mobile-room-choices'/a.label
TMP=ROOT/'.local/yuyu-mobile'/('app-'+a.label)
OUT.mkdir(parents=True,exist_ok=False);TMP.mkdir(parents=True,exist_ok=False)
common=ROOT.parents[1]if ROOT.parent.name=='worktrees'else ROOT
env=dict(os.environ,AGENT_BROWSER_SESSION='mr'+hashlib.sha256(a.label.encode()).hexdigest()[:6],AGENT_BROWSER_SOCKET_DIR=str(common/'.local/ym'),TMPDIR=str(TMP))
report={'passed':False,'checks':[],'nativeTouches':[],'scope':'Normal built application, registered Go module, SQLite/HTTP/WS, native Chromium touch; no physical-phone claim'}
jar=http.cookiejar.CookieJar();client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
def api(path,body=None):
 req=urllib.request.Request(url.rstrip('/')+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
 with client.open(req,timeout=20)as response:return json.load(response)
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=45)
 v=json.loads(r.stdout);assert r.returncode==0 and v['success'],r.stdout+r.stderr;return v['data']
def js(code):return b('eval',code)['result']
def wait(code):return b('wait','--fn',code)
def click(name):return b('find','role','button','click','--name',name,'--exact')
def check(name,value):assert value,name;report['checks'].append(name)
def tap(selector):
 js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center"});true')
 box=js('(()=>{const e=document.querySelector('+json.dumps(selector)+'),r=e.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2,hit:e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))}})()')
 assert box['hit'],(selector,box)
 endpoint=b('get','cdp-url')['cdpUrl'];steps=[{'type':'touchStart','touchPoints':[{'x':box['x'],'y':box['y']}],'wait':70},{'type':'touchEnd','touchPoints':[],'wait':100}]
 r=subprocess.run(['node',str(ROOT/'scripts/controllers/gamepad/native-touch.mjs'),endpoint,url,json.dumps(steps),'.yuyu-preparation,.yuyu-options'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=20)
 assert r.returncode==0,r.stdout+r.stderr
 report['nativeTouches'].append({'selector':selector,'point':box,'events':json.loads(r.stdout)})
def arena(value):
 wait('document.querySelector("[data-arena-picker] summary").getAttribute("aria-disabled")==="false"')
 tap('[data-arena-picker] summary');wait('document.querySelectorAll("[data-arena-option]").length===8')
 b('screenshot',str(OUT/('arena-'+str(value)+'.png')))
 tap('[data-arena-option="'+str(value)+'"] .yuyu-arena-image');wait('document.querySelector("[data-arena-picker]").dataset.arena==='+json.dumps(str(value)))
def character(seat,value):
 selector='[data-character-seat="'+str(seat)+'"]'
 wait('document.querySelector('+json.dumps(selector+' summary')+').getAttribute("aria-disabled")==="false"')
 tap(selector+' summary');wait('document.querySelectorAll("[data-character-option]").length===11')
 b('screenshot',str(OUT/('characters-'+str(seat)+'.png')))
 tap('[data-character-option="'+str(value)+'"] .yuyu-portrait')
 wait('document.querySelector('+json.dumps(selector)+').dataset.character==='+json.dumps(str(value)))
host=None
try:
 host=subprocess.Popen([str(ROOT/'.local/yuyu-mobile/app-host'),'-build',str(ROOT/'web/dist'),'-data',str(TMP/'data')],cwd=ROOT,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=(OUT/'host.log').open('w'),text=True)
 url=json.loads(host.stdout.readline())['direct'];assert url.startswith('http://127.0.0.1:')
 api('/register',{'name':'移动验收'+hashlib.sha256(a.label.encode()).hexdigest()[:6],'password':'LocalTouchAcceptance01','invite':'yuyu-local-e2e'})
 state={'cookies':[{'name':c.name,'value':c.value,'domain':c.domain,'path':c.path,'expires':c.expires or -1,'httpOnly':True,'secure':False,'sameSite':'Lax'}for c in jar],'origins':[]}
 cookie=TMP/'state.json';cookie.write_text(json.dumps(state));cookie.chmod(0o600)
 b('--state',str(cookie),'open',url);b('set','viewport','320','740');wait('!!document.querySelector(".profile-nav")')
 click('创建房间');b('find','role','button','click','--name','魔强统一战');b('wait','[aria-label=房间设置] form')
 arena(2);b('click','[aria-label=房间设置] form button.primary');b('wait','[aria-label=比赛准备]')
 check('native creation selection reached server',api('/state')['room']['options']['arena']==2)
 arena(10);character(0,8)
 options=api('/state')['room']['options'];check('own character and waiting arena persisted',options['arena']==10 and options['characters'][0]==8)
 click('添加电脑玩家');b('wait','.bot-remove');character(1,10)
 options=api('/state')['room']['options'];check('bot character persisted',options['characters']==[8,10])
 b('screenshot',str(OUT/'mobile-waiting.png'));b('reload');b('wait','[aria-label=比赛准备]')
 check('selections survive reload',api('/state')['room']['options']==options)
 check('selected thumbnails load after reload',js('[...document.querySelectorAll(".yuyu-preparation img")].every(i=>i.complete&&i.naturalWidth>0)'))
 b('set','viewport','390','844');character(0,1);arena(6)
 check('second phone width updates server',api('/state')['room']['options']['arena']==6 and api('/state')['room']['options']['characters'][0]==1)
 b('screenshot',str(OUT/'mobile-390.png'))
 b('set','viewport','1100','900');b('click','[data-arena-picker] summary');b('wait','[data-arena-option]');b('click','[data-arena-option="14"]');wait('document.querySelector("[data-arena-picker]").dataset.arena==="14"')
 check('desktop choice still persists',api('/state')['room']['options']['arena']==14)
 b('screenshot',str(OUT/'desktop.png'));report['finalOptions']=api('/state')['room']['options']
 click('我准备好了');wait('[...document.querySelectorAll("button")].some(b=>b.textContent.trim()==="开始游戏"&&!b.disabled)');click('开始游戏')
 wait('document.querySelector("[data-yuyu-board]")?.dataset.connection==="connected"');click('开启托管')
 deadline=time.monotonic()+180
 while True:
  room=api('/state')['room']
  if room['status']=='finished':break
  assert time.monotonic()<deadline,'automated real match did not finish'
  time.sleep(.5)
 b('wait','.results');heading=js('document.querySelector(".results h2").textContent')
 check('actual unranked match has a result',room['result']['unrated'] and not room['result']['rated'])
 winners=room['game']['winners'];check('actual winner remains visible',bool(winners) and all(room['seats'][seat]['name'] in heading for seat in winners) and '获胜' in heading and '练习结束' not in heading)
 check('rating explanation is separate',js('document.querySelector(".results p").textContent')=='本局不计积分，成绩已保存。')
 report['settlement']={'heading':heading,'winnerSeats':winners,'rated':False,'unrated':True}
 b('set','viewport','320','740');b('screenshot',str(OUT/'mobile-winner.png'));check('no browser runtime errors',not b('errors')['errors']);report['passed']=True
except BaseException as error:
 report['error']=repr(error)
 try:b('screenshot',str(OUT/'failure.png'))
 except Exception:pass
 raise
finally:
 report['sources']={str(f.relative_to(ROOT)):hashlib.sha256(f.read_bytes()).hexdigest()for f in [Path(__file__),ROOT/'games/yuyuhakusho/web/CharacterPicker.tsx',ROOT/'web/src/ui/results.tsx',ROOT/'web/dist/index.html']}
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 try:b('close')
 except Exception:pass
 if host:host.stdin.close();host.wait(timeout=15)
 print(json.dumps({'passed':report['passed'],'checks':len(report['checks']),'result':str(OUT/'result.json')},ensure_ascii=False))
